package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"media-upload-system/api"
	"media-upload-system/config"
	"media-upload-system/model"
	"media-upload-system/storage"
	"media-upload-system/strapi"
	"media-upload-system/tmdb"
	"media-upload-system/upload"
	"media-upload-system/worker"
)

// Variables globales
var (
	cfg           *config.Config
	db            *storage.Database
	workerPool    *worker.Pool
	uploadMgr     *upload.Uploader // Utilisation du type Uploader au lieu de Manager
	apiClient     *api.Client
	discordClient *api.DiscordWebhook
	strapiClient  *strapi.StrapiClient
	tmdbClient    *tmdb.TMDBClient
)

// Gestionnaire de webhook
func webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier la méthode HTTP
	if r.Method != http.MethodPost {
		http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
		return
	}

	// Lire le corps de la requête
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Erreur lors de la lecture du corps de la requête: %v", err)
		http.Error(w, "Erreur lors de la lecture de la requête", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Journaliser le payload brut pour le débogage
	log.Printf("Payload reçu: %s", string(body))

	// Décoder le JSON
	var webhook model.RadarrWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		log.Printf("Erreur lors du décodage JSON: %v", err)
		http.Error(w, "JSON invalide", http.StatusBadRequest)
		return
	}

	// Traiter l'événement
	switch webhook.EventType {
	case "Download":
		handleDownloadEvent(&webhook)
	default:
		log.Printf("Type d'événement ignoré: %s", webhook.EventType)
	}

	// Répondre avec succès
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Webhook reçu avec succès",
	})
}

// Traiter un événement de téléchargement
func handleDownloadEvent(webhook *model.RadarrWebhook) {
	if webhook.Movie != nil && webhook.MovieFile != nil {
		log.Printf("=== FILM TÉLÉCHARGÉ ===")
		log.Printf("Titre: %s (%d)", webhook.Movie.Title, webhook.Movie.Year)
		log.Printf("TMDB ID: %d", webhook.Movie.TmdbId)
		log.Printf("IMDB ID: %s", webhook.Movie.ImdbId)
		log.Printf("Chemin du dossier: %s", webhook.Movie.FolderPath)
		log.Printf("Chemin du fichier: %s", webhook.MovieFile.Path)
		log.Printf("Taille: %d octets (%.2f GB)", webhook.MovieFile.Size, float64(webhook.MovieFile.Size)/(1024*1024*1024))
		log.Printf("Qualité: %s", webhook.MovieFile.Quality)

		if webhook.MovieFile.MediaInfo != nil {
			log.Printf("Résolution: %dx%d", webhook.MovieFile.MediaInfo.Width, webhook.MovieFile.MediaInfo.Height)
			log.Printf("Codec vidéo: %s", webhook.MovieFile.MediaInfo.VideoCodec)
			log.Printf("Codec audio: %s (%.1f canaux)", webhook.MovieFile.MediaInfo.AudioCodec, webhook.MovieFile.MediaInfo.AudioChannels)
			log.Printf("Langues audio: %v", webhook.MovieFile.MediaInfo.AudioLanguages)
			if len(webhook.MovieFile.MediaInfo.Subtitles) > 0 {
				log.Printf("Sous-titres: %v", webhook.MovieFile.MediaInfo.Subtitles)
			}
		}

		if webhook.Release != nil {
			log.Printf("Titre de la release: %s", webhook.Release.ReleaseTitle)
			log.Printf("Indexeur: %s", webhook.Release.Indexer)
		}

		// Vérifier si le film a déjà été uploadé
		existingUpload, err := db.CheckExistingUpload(webhook.Movie.TmdbId, storage.TypeMovie, nil, nil)
		if err != nil {
			log.Printf("Erreur lors de la vérification d'un upload existant: %v", err)
		}

		if existingUpload != nil && existingUpload.UploadStatus == storage.StatusCompleted {
			log.Printf("Le film a déjà été uploadé: %s (ID: %d)", existingUpload.Title, existingUpload.ID)
			return
		}

		// Créer un nouvel upload
		newUpload := &storage.Upload{
			TmdbID:       webhook.Movie.TmdbId,
			Title:        webhook.Movie.Title,
			Type:         storage.TypeMovie,
			FilePath:     webhook.MovieFile.Path,
			UploadStatus: storage.StatusPending,
		}

		// Ajouter l'upload à la base de données
		uploadID, err := db.AddUpload(newUpload)
		if err != nil {
			log.Printf("Erreur lors de l'ajout de l'upload à la base de données: %v", err)
			return
		}

		log.Printf("Upload ajouté à la base de données avec l'ID: %d", uploadID)

		// Ajouter la tâche au pool de workers
		workerPool.AddTask(func() error {
			return processMovieUpload(uploadID, webhook.Movie.TmdbId, webhook.Movie.Title, webhook.MovieFile.Path)
		})

	} else if webhook.Series != nil && webhook.Episodes != nil {
		log.Printf("=== SÉRIE TÉLÉCHARGÉE ===")
		log.Printf("Titre: %s (%d)", webhook.Series.Title, webhook.Series.Year)
		log.Printf("TMDB ID: %d", webhook.Series.TmdbId)
		log.Printf("IMDB ID: %s", webhook.Series.ImdbId)
		log.Printf("Chemin: %s", webhook.Series.Path)

		log.Printf("Épisodes:")
		for _, episode := range webhook.Episodes {
			log.Printf("  - S%02dE%02d: %s", episode.SeasonNumber, episode.EpisodeNumber, episode.Title)

			// Vérifier si l'épisode a déjà été uploadé
			season := episode.SeasonNumber
			episodeNum := episode.EpisodeNumber

			existingUpload, err := db.CheckExistingUpload(webhook.Series.TmdbId, storage.TypeSeries, &season, &episodeNum)
			if err != nil {
				log.Printf("Erreur lors de la vérification d'un upload existant: %v", err)
				continue
			}

			if existingUpload != nil && existingUpload.UploadStatus == storage.StatusCompleted {
				log.Printf("L'épisode a déjà été uploadé: %s S%02dE%02d (ID: %d)", existingUpload.Title, *existingUpload.Season, *existingUpload.Episode, existingUpload.ID)
				continue
			}

			// Créer un nouvel upload
			newUpload := &storage.Upload{
				TmdbID:       webhook.Series.TmdbId,
				Title:        webhook.Series.Title,
				Type:         storage.TypeSeries,
				Season:       &season,
				Episode:      &episodeNum,
				FilePath:     fmt.Sprintf("%s/Season %d/%s", webhook.Series.Path, season, episode.Title),
				UploadStatus: storage.StatusPending,
			}

			// Ajouter l'upload à la base de données
			uploadID, err := db.AddUpload(newUpload)
			if err != nil {
				log.Printf("Erreur lors de l'ajout de l'upload à la base de données: %v", err)
				continue
			}

			log.Printf("Upload ajouté à la base de données avec l'ID: %d", uploadID)

			// Ajouter la tâche au pool de workers
			workerPool.AddTask(func() error {
				return processEpisodeUpload(uploadID, webhook.Series.TmdbId, webhook.Series.Title, newUpload.FilePath, season, episodeNum)
			})
		}
	} else {
		log.Printf("Événement de téléchargement sans média identifiable")
	}
}

// Add this function to limit memory usage during uploads
func limitMemoryUsage(filePath string, uploadFunc func(string, string) (*upload.UploadResult, error), title string) (*upload.UploadResult, error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'obtention des informations du fichier: %w", err)
	}

	fileSize := fileInfo.Size()

	// For very large files (over 4GB), use a streaming approach
	if fileSize > 4*1024*1024*1024 {
		log.Printf("Fichier volumineux détecté (%d GB), utilisation du mode économie de mémoire", fileSize/(1024*1024*1024))

		// Set GOMAXPROCS to limit CPU usage during heavy uploads
		originalMaxProcs := runtime.GOMAXPROCS(0)
		runtime.GOMAXPROCS(2) // Limit to 2 cores during upload

		// Force garbage collection before starting the upload
		runtime.GC()

		// Execute the upload
		result, err := uploadFunc(filePath, title)

		// Restore original GOMAXPROCS
		runtime.GOMAXPROCS(originalMaxProcs)

		// Force garbage collection after upload
		runtime.GC()

		return result, err
	}

	// For smaller files, just use the normal upload function
	return uploadFunc(filePath, title)
}

// Modify the processMovieUpload function to use our memory-limiting function
func processMovieUpload(uploadID int64, tmdbID int, title, filePath string) error {
	log.Printf("Traitement de l'upload du film: %s (ID: %d)", title, uploadID)

	// Mettre à jour le statut
	if err := db.UpdateUploadStatus(uploadID, storage.StatusUploading); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
	}

	// Créer une liste pour stocker les résultats d'upload
	var results []*upload.UploadResult
	var mutex sync.Mutex
	var wg sync.WaitGroup
	var errors []error

	// Uploader vers MixDrop si activé
	if cfg.Uploaders.MixDrop.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("Démarrage de l'upload vers mixdrop pour le film %s (ID: %d)", title, uploadID)

			mixdropUploader := upload.NewMixDropUploader(
				cfg.Uploaders.MixDrop.Email,
				cfg.Uploaders.MixDrop.ApiKey,
				cfg.Uploaders.MixDrop.Enabled,
			)

			// Utiliser notre fonction de limitation de mémoire
			result, err := limitMemoryUsage(filePath, mixdropUploader.UploadFile, title)

			if err != nil {
				log.Printf("Erreur lors de l'upload vers MixDrop: %v", err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
			} else {
				log.Printf("Upload vers mixdrop terminé avec succès pour le film %s (ID: %d)", title, uploadID)
				mutex.Lock()
				results = append(results, result)
				mutex.Unlock()
			}
		}()
	}

	// Uploader vers Netu si activé
	if cfg.Uploaders.Netu.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Printf("Démarrage de l'upload vers netu pour le film %s (ID: %d)", title, uploadID)

			netuUploader := upload.NewNetuUploader(
				cfg.Uploaders.Netu.ApiKey,
				cfg.Uploaders.Netu.Enabled,
			)

			// Utiliser notre fonction de limitation de mémoire
			result, err := limitMemoryUsage(filePath, netuUploader.UploadFile, title)

			if err != nil {
				log.Printf("Erreur lors de l'upload vers Netu: %v", err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
			} else {
				log.Printf("Upload vers netu terminé avec succès pour le film %s (ID: %d)", title, uploadID)
				mutex.Lock()
				results = append(results, result)
				mutex.Unlock()
			}
		}()
	}

	// Attendre que tous les uploads soient terminés
	wg.Wait()

	// Vérifier si au moins un upload a réussi
	success := len(results) > 0
	var discordLinks []api.HostedLink

	for _, result := range results {
		if result.Success {
			// Ajouter le lien à la base de données
			link := storage.HostedLink{
				Hoster:   result.Hoster,
				FileCode: result.FileCode,
				URL:      result.URL,
				Embed:    result.Embed,
			}

			if err := db.AddUploadLink(uploadID, link); err != nil {
				log.Printf("Erreur lors de l'ajout du lien à la base de données: %v", err)
			}

			// Ajouter le lien pour Discord
			discordLinks = append(discordLinks, api.HostedLink{
				Hoster:   result.Hoster,
				URL:      result.URL,
				Embed:    result.Embed,
				FileCode: result.FileCode,
			})
		}
	}

	// Mettre à jour le statut
	if success {
		if err := db.UpdateUploadStatus(uploadID, storage.StatusCompleted); err != nil {
			return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
		}
	} else {
		if err := db.UpdateUploadStatus(uploadID, storage.StatusFailed); err != nil {
			return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
		}
		return fmt.Errorf("aucun upload n'a réussi")
	}

	// Récupérer les informations du film depuis TMDB
	movie, err := api.FetchTMDBMovie(tmdbID, cfg.TMDB.ApiKey)
	if err != nil {
		log.Printf("Erreur lors de la récupération des informations du film: %v", err)
		// Continuer même en cas d'erreur
	}

	// Notifier Discord
	if movie != nil && len(discordLinks) > 0 {
		if err := discordClient.NotifyUpload(movie, discordLinks); err != nil {
			log.Printf("Erreur lors de la notification à Discord: %v", err)
			// Continuer même en cas d'erreur
		}
	} else {
		// Notification simplifiée si on n'a pas pu récupérer les infos du film
		log.Printf("Envoi d'une notification Discord simplifiée")

		// Créer un embed simple
		embed := api.DiscordEmbed{
			Title:     fmt.Sprintf("🎬 Nouveau film disponible: %s", title),
			Color:     3447003, // Bleu Discord
			Timestamp: time.Now().Format(time.RFC3339),
			Footer: &api.DiscordEmbedFooter{
				Text: "Media Upload System",
			},
		}

		// Ajouter les liens
		var linksField api.DiscordEmbedField
		linksField.Name = "🔗 Liens"

		for _, link := range discordLinks {
			linksField.Value += fmt.Sprintf("**%s**: [Regarder](%s) | [Embed](%s)\n",
				link.Hoster, link.URL, link.Embed)
		}

		embed.Fields = append(embed.Fields, linksField)

		// Créer le payload
		payload := api.DiscordWebhookPayload{
			Username:  "Media Upload Bot",
			AvatarURL: "https://cdn-icons-png.flaticon.com/512/2503/2503508.png",
			Embeds:    []api.DiscordEmbed{embed},
		}

		// Sérialiser le payload
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Erreur lors de la sérialisation du payload: %v", err)
		} else {
			// Envoyer la requête
			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			resp, err := client.Post(discordClient.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
			if err != nil {
				log.Printf("Erreur lors de l'envoi de la requête: %v", err)
			} else {
				defer resp.Body.Close()

				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					log.Printf("Discord a retourné un code non-2xx: %d", resp.StatusCode)
				} else {
					log.Printf("Notification Discord simplifiée envoyée avec succès")
				}
			}
		}
	}

	// Si Strapi est activé, envoyer les données à Strapi
	if cfg.Strapi.Enabled && len(discordLinks) > 0 {
		log.Printf("Strapi est activé, envoi des données pour %d liens", len(discordLinks))

		// Créer la fiche une seule fois
		log.Printf("Création de la fiche dans Strapi pour le film %s (TMDB ID: %d)", title, tmdbID)
		ficheID, err := strapiClient.CreateFiche(title, tmdbID)
		if err != nil {
			log.Printf("ERREUR lors de la création de la fiche Strapi: %v", err)
			// Continuer malgré l'erreur
		} else {
			log.Printf("Fiche créée dans Strapi avec l'ID: %s", ficheID)

			// Envoyer tous les liens d'embed à Strapi
			for _, link := range discordLinks {
				log.Printf("Envoi du lien d'embed à Strapi pour %s: %s", link.Hoster, link.Embed)
				embedLinkID, err := strapiClient.CreateLink(ficheID, link.Embed)
				if err != nil {
					log.Printf("ERREUR lors de l'envoi du lien à Strapi: %v", err)
					// Continuer malgré l'erreur
				} else {
					log.Printf("Lien d'embed créé dans Strapi avec l'ID: %s", embedLinkID)
				}
			}
		}
	} else {
		if !cfg.Strapi.Enabled {
			log.Printf("Strapi est désactivé, aucune donnée envoyée")
		} else if len(discordLinks) == 0 {
			log.Printf("Aucun lien disponible pour Strapi")
		}
	}

	log.Printf("Upload du film terminé avec succès: %s (ID: %d)", title, uploadID)
	return nil
}

// Traiter l'upload d'un épisode
func processEpisodeUpload(uploadID int64, tmdbID int, title, filePath string, season, episode int) error {
	log.Printf("Traitement de l'upload de l'épisode: %s S%02dE%02d (ID: %d)", title, season, episode, uploadID)

	// Mettre à jour le statut
	if err := db.UpdateUploadStatus(uploadID, storage.StatusUploading); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
	}

	// Créer une liste pour stocker les résultats d'upload
	var results []*upload.UploadResult
	var mutex sync.Mutex
	var wg sync.WaitGroup
	var errors []error

	episodeTitle := fmt.Sprintf("%s S%02dE%02d", title, season, episode)

	// Uploader vers MixDrop si activé
	if cfg.Uploaders.MixDrop.Enabled {
		wg.Add(1)
		workerPool.AddTask(func() error {
			defer wg.Done()
			log.Printf("Démarrage de l'upload vers mixdrop pour l'épisode %s (ID: %d)", episodeTitle, uploadID)

			mixdropUploader := upload.NewMixDropUploader(
				cfg.Uploaders.MixDrop.Email,
				cfg.Uploaders.MixDrop.ApiKey,
				cfg.Uploaders.MixDrop.Enabled,
			)

			uploadFunc := func(filePath string, title string) (*upload.UploadResult, error) {
				return mixdropUploader.UploadFile(filePath, title)
			}

			result, err := limitMemoryUsage(filePath, uploadFunc, episodeTitle)

			if err != nil {
				log.Printf("Erreur lors de l'upload vers MixDrop: %v", err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
				return err
			} else {
				log.Printf("Upload vers mixdrop terminé avec succès pour l'épisode %s (ID: %d)", episodeTitle, uploadID)
				mutex.Lock()
				results = append(results, result)
				mutex.Unlock()
				return nil
			}
		})
	}

	// Uploader vers Netu si activé
	if cfg.Uploaders.Netu.Enabled {
		wg.Add(1)
		workerPool.AddTask(func() error {
			defer wg.Done()
			log.Printf("Démarrage de l'upload vers netu pour l'épisode %s (ID: %d)", episodeTitle, uploadID)

			netuUploader := upload.NewNetuUploader(
				cfg.Uploaders.Netu.ApiKey,
				cfg.Uploaders.Netu.Enabled,
			)

			uploadFunc := func(filePath string, title string) (*upload.UploadResult, error) {
				return netuUploader.UploadFile(filePath, title)
			}

			result, err := limitMemoryUsage(filePath, uploadFunc, episodeTitle)

			if err != nil {
				log.Printf("Erreur lors de l'upload vers Netu: %v", err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
				return err
			} else {
				log.Printf("Upload vers netu terminé avec succès pour l'épisode %s (ID: %d)", episodeTitle, uploadID)
				mutex.Lock()
				results = append(results, result)
				mutex.Unlock()
				return nil
			}
		})
	}

	// Attendre que tous les uploads soient terminés
	wg.Wait()

	// Vérifier si au moins un upload a réussi
	success := len(results) > 0
	var discordLinks []api.HostedLink

	for _, result := range results {
		if result.Success {
			// Ajouter le lien à la base de données
			link := storage.HostedLink{
				Hoster:   result.Hoster,
				FileCode: result.FileCode,
				URL:      result.URL,
				Embed:    result.Embed,
			}

			if err := db.AddUploadLink(uploadID, link); err != nil {
				log.Printf("Erreur lors de l'ajout du lien à la base de données: %v", err)
			}

			// Ajouter le lien pour Discord
			discordLinks = append(discordLinks, api.HostedLink{
				Hoster:   result.Hoster,
				URL:      result.URL,
				Embed:    result.Embed,
				FileCode: result.FileCode,
			})
		}
	}

	// Mettre à jour le statut
	if success {
		if err := db.UpdateUploadStatus(uploadID, storage.StatusCompleted); err != nil {
			return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
		}
	} else {
		if err := db.UpdateUploadStatus(uploadID, storage.StatusFailed); err != nil {
			return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
		}
		return fmt.Errorf("aucun upload n'a réussi")
	}

	// Notifier Discord
	if len(discordLinks) > 0 {
		if err := discordClient.NotifyEpisodeUpload(title, tmdbID, season, episode, discordLinks); err != nil {
			log.Printf("Erreur lors de la notification à Discord: %v", err)
			// Continuer même en cas d'erreur
		}
	}

	// Si Strapi est activé, envoyer les données à Strapi
	if cfg.Strapi.Enabled && len(discordLinks) > 0 {
		// Créer un titre formaté pour l'épisode
		episodeTitle := fmt.Sprintf("%s - S%02dE%02d", title, season, episode)

		// Créer la fiche de série une seule fois
		log.Printf("Création de la fiche dans Strapi pour l'épisode %s (TMDB ID: %d)", episodeTitle, tmdbID)
		ficheID, err := strapiClient.CreateFiche(episodeTitle, tmdbID)
		if err != nil {
			log.Printf("ERREUR lors de la création de la fiche Strapi: %v", err)
			// Continuer malgré l'erreur
		} else {
			log.Printf("Fiche créée dans Strapi avec l'ID: %s", ficheID)

			// Envoyer tous les liens d'embed à Strapi
			for _, link := range discordLinks {
				log.Printf("Envoi du lien d'embed à Strapi pour %s: %s", link.Hoster, link.Embed)
				embedLinkID, err := strapiClient.CreateLink(ficheID, link.Embed)
				if err != nil {
					log.Printf("ERREUR lors de l'envoi du lien à Strapi: %v", err)
					// Continuer malgré l'erreur
				} else {
					log.Printf("Lien d'embed créé dans Strapi avec l'ID: %s", embedLinkID)
				}
			}
		}
	}

	log.Printf("Upload de l'épisode terminé avec succès: %s S%02dE%02d (ID: %d)", title, season, episode, uploadID)
	return nil
}

func main() {
	// Analyser les arguments de la ligne de commande
	configPath := flag.String("config", "config.json", "Chemin vers le fichier de configuration")
	createConfig := flag.Bool("create-config", false, "Créer un fichier de configuration par défaut")
	// Suppression des variables non utilisées
	// watchMode := flag.Bool("watch", false, "Activer le mode surveillance")
	// cronMode := flag.Bool("cron", false, "Activer le mode cron")
	flag.Parse()

	// Créer un fichier de configuration par défaut si demandé
	if *createConfig {
		if err := config.CreateDefaultConfig(*configPath); err != nil {
			log.Fatalf("Erreur lors de la création du fichier de configuration: %v", err)
		}
		log.Printf("Fichier de configuration créé: %s", *configPath)
		return
	}

	// Charger la configuration
	var err error
	cfg, err = config.LoadConfig(*configPath)
	if err != nil {
		log.Printf("Erreur lors du chargement de la configuration: %v", err)
		log.Printf("Création d'un fichier de configuration par défaut...")

		// Créer le répertoire si nécessaire
		dir := filepath.Dir(*configPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatalf("Erreur lors de la création du répertoire: %v", err)
			}
		}

		// Créer le fichier de configuration
		if err := config.CreateDefaultConfig(*configPath); err != nil {
			log.Fatalf("Erreur lors de la création du fichier de configuration: %v", err)
		}

		// Recharger la configuration
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			log.Fatalf("Erreur lors du rechargement de la configuration: %v", err)
		}
	}

	// Configurer le logger
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Initialiser la base de données
	db, err = storage.NewDatabase(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Erreur lors de l'initialisation de la base de données: %v", err)
	}
	defer db.Close()

	// Initialiser le pool de workers
	workerPool = worker.NewPool(cfg.Workers.MaxConcurrent)
	workerPool.Start()
	defer workerPool.Stop()

	// Nous n'initialisons plus le gestionnaire d'uploads ici
	// car nous créons directement les uploaders dans les fonctions processMovieUpload et processEpisodeUpload

	// Initialiser le client API
	apiClient = api.NewClient(cfg.API.Endpoint)

	// Initialiser le client Discord
	discordClient = api.NewDiscordWebhook(cfg.Discord.WebhookURL)

	// Initialiser le client Strapi si activé
	if cfg.Strapi.Enabled {
		log.Printf("Initialisation du client Strapi avec l'URL: %s", cfg.Strapi.BaseURL)
		strapiClient = strapi.NewStrapiClient(cfg.Strapi.BaseURL, cfg.Strapi.Username, cfg.Strapi.Password)

		// Tester la connexion à Strapi
		if err := strapiClient.Login(); err != nil {
			log.Printf("AVERTISSEMENT: Impossible de se connecter à Strapi: %v", err)
			log.Printf("Les uploads vers Strapi seront désactivés")
			cfg.Strapi.Enabled = false
		} else {
			log.Printf("Connexion à Strapi réussie, le client est prêt")
		}
	} else {
		log.Printf("Strapi est désactivé dans la configuration")
	}

	// Initialiser le client TMDB
	tmdbClient = tmdb.NewTMDBClient(cfg.TMDB.ApiKey)

	// Définir les routes
	http.HandleFunc("/webhook", webhookHandler)

	// Démarrer le serveur
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Démarrage du serveur sur %s...", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		log.Fatalf("Erreur lors du démarrage du serveur: %v", err)
	}
}

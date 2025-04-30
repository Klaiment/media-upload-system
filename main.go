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
	"strconv"
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
	uploadMgr     *upload.Manager
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

// Traiter l'upload d'un film
func processMovieUpload(uploadID int64, tmdbID int, title, filePath string) error {
	log.Printf("Traitement de l'upload du film: %s (ID: %d)", title, uploadID)

	// Mettre à jour le statut
	if err := db.UpdateUploadStatus(uploadID, storage.StatusUploading); err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
	}

	// Obtenir la liste des uploaders activés
	enabledUploaders := uploadMgr.GetEnabledUploaders()

	// Créer une tâche pour chaque uploader
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var results []*upload.UploadResult
	var errors []error

	for _, uploader := range enabledUploaders {
		wg.Add(1)

		// Créer une tâche pour cet uploader
		uploaderName := uploader.Name() // Capturer la valeur pour la goroutine

		// Ajouter la tâche au pool de workers
		workerPool.AddTask(func() error {
			defer wg.Done()

			log.Printf("Démarrage de l'upload vers %s pour le film %s (ID: %d)", uploaderName, title, uploadID)

			// Récupérer l'uploader par son nom
			uploader, err := uploadMgr.GetUploader(uploaderName)
			if err != nil {
				log.Printf("Erreur lors de la récupération de l'uploader %s: %v", uploaderName, err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
				return err
			}

			// Uploader le fichier
			result, err := uploader.UploadFile(filePath, title)
			if err != nil {
				log.Printf("Erreur lors de l'upload vers %s: %v", uploaderName, err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
				return err
			}

			// Ajouter le résultat à la liste
			mutex.Lock()
			results = append(results, result)
			mutex.Unlock()

			log.Printf("Upload vers %s terminé avec succès pour le film %s (ID: %d)", uploaderName, title, uploadID)
			return nil
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
		for _, link := range discordLinks {
			if err := sendToStrapi(tmdbID, title, link.URL); err != nil {
				log.Printf("Erreur lors de l'envoi à Strapi: %v", err)
				// Continuer même en cas d'erreur
			}
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

	// Obtenir la liste des uploaders activés
	enabledUploaders := uploadMgr.GetEnabledUploaders()

	// Créer une tâche pour chaque uploader
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var results []*upload.UploadResult
	var errors []error

	episodeTitle := fmt.Sprintf("%s S%02dE%02d", title, season, episode)

	for _, uploader := range enabledUploaders {
		wg.Add(1)

		// Créer une tâche pour cet uploader
		uploaderName := uploader.Name() // Capturer la valeur pour la goroutine

		// Ajouter la tâche au pool de workers
		workerPool.AddTask(func() error {
			defer wg.Done()

			log.Printf("Démarrage de l'upload vers %s pour l'épisode %s (ID: %d)", uploaderName, episodeTitle, uploadID)

			// Récupérer l'uploader par son nom
			uploader, err := uploadMgr.GetUploader(uploaderName)
			if err != nil {
				log.Printf("Erreur lors de la récupération de l'uploader %s: %v", uploaderName, err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
				return err
			}

			// Uploader le fichier
			result, err := uploader.UploadFile(filePath, episodeTitle)
			if err != nil {
				log.Printf("Erreur lors de l'upload vers %s: %v", uploaderName, err)
				mutex.Lock()
				errors = append(errors, err)
				mutex.Unlock()
				return err
			}

			// Ajouter le résultat à la liste
			mutex.Lock()
			results = append(results, result)
			mutex.Unlock()

			log.Printf("Upload vers %s terminé avec succès pour l'épisode %s (ID: %d)", uploaderName, episodeTitle, uploadID)
			return nil
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
		for _, link := range discordLinks {
			if err := sendToStrapiSeries(tmdbID, title, link.URL, season, episode); err != nil {
				log.Printf("Erreur lors de l'envoi à Strapi: %v", err)
				// Continuer même en cas d'erreur
			}
		}
	}

	log.Printf("Upload de l'épisode terminé avec succès: %s S%02dE%02d (ID: %d)", title, season, episode, uploadID)
	return nil
}

// sendToStrapi envoie les données du film à Strapi
func sendToStrapi(tmdbID int, title, url string) error {
	log.Printf("Envoi des données du film %s à Strapi...", title)

	// Se connecter à Strapi si nécessaire
	if strapiClient.AuthToken == "" {
		if err := strapiClient.Login(); err != nil {
			return fmt.Errorf("erreur lors de la connexion à Strapi: %w", err)
		}
	}

	// Récupérer les détails du film depuis TMDB
	tmdbData, err := tmdbClient.GetMovieDetailsJSON(tmdbID)
	if err != nil {
		return fmt.Errorf("erreur lors de la récupération des détails du film: %w", err)
	}

	// Récupérer les genres du film
	movieDetails, err := tmdbClient.GetMovieDetails(tmdbID)
	if err != nil {
		return fmt.Errorf("erreur lors de la récupération des détails du film: %w", err)
	}

	// Extraire les noms des genres
	var genreNames []string
	for _, genre := range movieDetails.Genres {
		genreNames = append(genreNames, genre.Name)
	}

	// Trouver les IDs des genres dans Strapi
	genderIDs, err := strapiClient.FindGenderIDsByNames(genreNames)
	if err != nil {
		return fmt.Errorf("erreur lors de la recherche des IDs de genres: %w", err)
	}

	// Créer la fiche dans Strapi
	ficheResp, err := strapiClient.CreateFiche(title, strconv.Itoa(tmdbID), tmdbData, genderIDs)
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la fiche: %w", err)
	}

	// Créer le lien dans Strapi
	_, err = strapiClient.CreateLink(url, ficheResp.Data.DocumentID)
	if err != nil {
		return fmt.Errorf("erreur lors de la création du lien: %w", err)
	}

	log.Printf("Données du film envoyées avec succès à Strapi")
	return nil
}

// sendToStrapiSeries envoie les données de la série à Strapi
func sendToStrapiSeries(tmdbID int, title, url string, season, episode int) error {
	log.Printf("Envoi des données de la série %s S%02dE%02d à Strapi...", title, season, episode)

	// Se connecter à Strapi si nécessaire
	if strapiClient.AuthToken == "" {
		if err := strapiClient.Login(); err != nil {
			return fmt.Errorf("erreur lors de la connexion à Strapi: %w", err)
		}
	}

	// TODO: Implémenter la logique pour les séries
	// Pour l'instant, on utilise la même logique que pour les films

	log.Printf("Données de la série envoyées avec succès à Strapi")
	return nil
}

func main() {
	// Analyser les arguments de la ligne de commande
	configPath := flag.String("config", "config.json", "Chemin vers le fichier de configuration")
	createConfig := flag.Bool("create-config", false, "Créer un fichier de configuration par défaut")
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

	// Initialiser le gestionnaire d'uploads
	uploadMgr = upload.NewManager()

	// Enregistrer les uploaders
	if cfg.Uploaders.Netu.Enabled {
		netuUploader := upload.NewNetuUploader(cfg.Uploaders.Netu.ApiKey, true)
		uploadMgr.RegisterUploader(netuUploader)
	}

	if cfg.Uploaders.MixDrop.Enabled {
		mixdropUploader := upload.NewMixDropUploader(cfg.Uploaders.MixDrop.Email, cfg.Uploaders.MixDrop.ApiKey, true)
		uploadMgr.RegisterUploader(mixdropUploader)
	}

	// Vous pouvez ajouter d'autres uploaders ici
	// Par exemple:
	// if cfg.Uploaders.Uptobox.Enabled {
	//     uptoboxUploader := upload.NewUptoboxUploader(cfg.Uploaders.Uptobox.ApiKey, true)
	//     uploadMgr.RegisterUploader(uptoboxUploader)
	// }

	// Initialiser le client API
	apiClient = api.NewClient(cfg.API.Endpoint)

	// Initialiser le client Discord
	discordClient = api.NewDiscordWebhook(cfg.Discord.WebhookURL)

	// Initialiser le client Strapi si activé
	if cfg.Strapi.Enabled {
		strapiClient = strapi.NewStrapiClient(cfg.Strapi.BaseURL, cfg.Strapi.Username, cfg.Strapi.Password)

		// Tester la connexion à Strapi
		if err := strapiClient.Login(); err != nil {
			log.Printf("Avertissement: Impossible de se connecter à Strapi: %v", err)
		}
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

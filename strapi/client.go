package strapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"media-upload-system/storage"
	"media-upload-system/tmdb"
)

// StrapiClient représente un client pour l'API Strapi
type StrapiClient struct {
	BaseURL    string
	Username   string
	Password   string
	Token      string
	HTTPClient *http.Client
	TMDBClient *tmdb.TMDBClient
	DB         *storage.Database
}

// LoginResponse représente la réponse de l'API Strapi pour la connexion
type LoginResponse struct {
	JWT string `json:"jwt"`
}

// Genre représente un genre dans Strapi
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// GenreResponse représente la réponse de l'API Strapi pour les genres
type GenreResponse struct {
	Data []struct {
		ID         int `json:"id"`
		Attributes struct {
			Name string `json:"name"`
		} `json:"attributes"`
	} `json:"data"`
}

// FicheResponse représente la réponse de l'API Strapi pour la création d'une fiche
type FicheResponse struct {
	Data struct {
		ID int `json:"documentId"`
	} `json:"data"`
}

// FicheSearchResponse représente la réponse de l'API Strapi pour la recherche de fiches
type FicheSearchResponse struct {
	Data []struct {
		ID int `json:"id"`
	} `json:"data"`
}

// LinkResponse représente la réponse de l'API Strapi pour la création d'un lien
type LinkResponse struct {
	Data struct {
		ID int `json:"id"`
	} `json:"data"`
}

// ErrorResponse représente une réponse d'erreur de l'API Strapi
type ErrorResponse struct {
	Data  interface{} `json:"data"`
	Error struct {
		Status  int    `json:"status"`
		Name    string `json:"name"`
		Message string `json:"message"`
		Details struct {
			Errors []struct {
				Path    []string `json:"path"`
				Message string   `json:"message"`
				Name    string   `json:"name"`
				Value   string   `json:"value"`
			} `json:"errors"`
		} `json:"details"`
	} `json:"error"`
}

// NewStrapiClient crée un nouveau client Strapi
func NewStrapiClient(baseURL, username, password string, tmdbClient *tmdb.TMDBClient, db *storage.Database) *StrapiClient {
	return &StrapiClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		TMDBClient: tmdbClient,
		DB:         db,
	}
}

// Login se connecte à l'API Strapi et obtient un token JWT
func (c *StrapiClient) Login() error {
	// Préparer les données de connexion
	data := map[string]string{
		"identifier": c.Username,
		"password":   c.Password,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation des données de connexion: %w", err)
	}

	// Créer la requête
	req, err := http.NewRequest("POST", c.BaseURL+"/api/auth/local", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Envoyer la requête
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Vérifier si la requête a réussi
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("erreur lors de la connexion: code %d, réponse: %s", resp.StatusCode, string(body))
	}

	// Décoder la réponse JSON
	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Stocker le token
	c.Token = loginResp.JWT

	return nil
}

// GetGenres récupère la liste des genres depuis Strapi
func (c *StrapiClient) GetGenres() ([]Genre, error) {
	// Vérifier si le token est disponible
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return nil, fmt.Errorf("erreur lors de la connexion: %w", err)
		}
	}

	// Créer la requête
	req, err := http.NewRequest("GET", c.BaseURL+"/api/genres", nil)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Envoyer la requête
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Vérifier si la requête a réussi
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("erreur lors de la récupération des genres: code %d, réponse: %s", resp.StatusCode, string(body))
	}

	// Décoder la réponse JSON
	var genreResp GenreResponse
	if err := json.Unmarshal(body, &genreResp); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Convertir la réponse en liste de genres
	genres := make([]Genre, 0, len(genreResp.Data))
	for _, g := range genreResp.Data {
		genres = append(genres, Genre{
			ID:   g.ID,
			Name: g.Attributes.Name,
		})
	}

	return genres, nil
}

// CreateSlug crée un slug à partir d'un titre
func CreateSlug(title string) string {
	// Convertir en minuscules
	slug := strings.ToLower(title)

	// Remplacer les espaces par des tirets
	slug = strings.ReplaceAll(slug, " ", "-")

	// Supprimer les caractères spéciaux
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, slug)

	// Supprimer les tirets multiples
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Supprimer les tirets au début et à la fin
	slug = strings.Trim(slug, "-")

	return slug
}

// SearchFicheByTMDBID recherche une fiche par son ID TMDB
func (c *StrapiClient) SearchFicheByTMDBID(tmdbID int) (int, error) {
	// Vérifier si le token est disponible
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return 0, fmt.Errorf("erreur lors de la connexion: %w", err)
		}
	}

	// Créer l'URL avec le filtre TMDB ID
	apiURL := fmt.Sprintf("%s/api/fiches?filters[tmdb_id][$eq]=%d", c.BaseURL, tmdbID)

	// Créer la requête
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Envoyer la requête
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Vérifier si la requête a réussi
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("erreur lors de la recherche de la fiche: code %d, réponse: %s", resp.StatusCode, string(body))
	}

	// Décoder la réponse JSON
	var searchResp FicheSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return 0, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Vérifier si une fiche a été trouvée
	if len(searchResp.Data) == 0 {
		return 0, nil // Aucune fiche trouvée
	}

	// Retourner l'ID de la première fiche trouvée
	return searchResp.Data[0].ID, nil
}

// CreateFiche crée une nouvelle fiche dans Strapi
func (c *StrapiClient) CreateFiche(title string, tmdbID int) (string, error) {
	// Vérifier si le token est disponible
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return "", fmt.Errorf("erreur lors de la connexion: %w", err)
		}
	}

	// Vérifier si la fiche existe déjà
	existingID, err := c.SearchFicheByTMDBID(tmdbID)
	if err != nil {
		log.Printf("Erreur lors de la recherche de la fiche existante: %v", err)
		// Continuer malgré l'erreur
	}

	if existingID > 0 {
		log.Printf("Fiche déjà existante avec l'ID: %d", existingID)
		return fmt.Sprintf("%d", existingID), nil
	}

	// Créer un slug à partir du titre
	slug := CreateSlug(title)

	// Récupérer les données TMDB
	tmdbData, err := c.TMDBClient.GetMovieDetailsJSON(tmdbID)
	if err != nil {
		log.Printf("Erreur lors de la récupération des données TMDB: %v", err)
		// Continuer malgré l'erreur, on utilisera juste le titre et l'ID
	}

	// Préparer les données de la fiche
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"title":   title, // Valeur par défaut
			"slug":    slug,
			"tmdb_id": fmt.Sprintf("%d", tmdbID),
		},
	}

	// Ajouter les données TMDB si disponibles
	if tmdbData != "" {
		var tmdbJSON map[string]interface{}
		if err := json.Unmarshal([]byte(tmdbData), &tmdbJSON); err == nil {
			// Utiliser le titre de TMDB si disponible
			if tmdbTitle, ok := tmdbJSON["title"].(string); ok && tmdbTitle != "" {
				data["data"].(map[string]interface{})["title"] = tmdbTitle
				// Recréer le slug avec le nouveau titre
				slug = CreateSlug(tmdbTitle)
				data["data"].(map[string]interface{})["slug"] = slug
			}

			// Ajouter les données TMDB complètes
			data["data"].(map[string]interface{})["tmdb_data"] = tmdbJSON
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la sérialisation des données de la fiche: %w", err)
	}

	// Créer la requête
	req, err := http.NewRequest("POST", c.BaseURL+"/api/fiches", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Envoyer la requête
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Vérifier si la requête a réussi
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Essayer de décoder la réponse d'erreur
		var errorResp ErrorResponse
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil {
			// Si c'est une erreur d'unicité sur le slug, essayer avec un slug modifié
			if errorResp.Error.Status == 400 && len(errorResp.Error.Details.Errors) > 0 &&
				errorResp.Error.Details.Errors[0].Path[0] == "slug" &&
				errorResp.Error.Details.Errors[0].Message == "This attribute must be unique" {

				// Ajouter un timestamp au slug pour le rendre unique
				uniqueSlug := fmt.Sprintf("%s-%d", slug, time.Now().Unix())

				// Mettre à jour les données avec le nouveau slug
				data["data"].(map[string]interface{})["slug"] = uniqueSlug

				jsonData, err = json.Marshal(data)
				if err != nil {
					return "", fmt.Errorf("erreur lors de la sérialisation des données de la fiche avec slug unique: %w", err)
				}

				// Créer une nouvelle requête
				req, err = http.NewRequest("POST", c.BaseURL+"/api/fiches", bytes.NewBuffer(jsonData))
				if err != nil {
					return "", fmt.Errorf("erreur lors de la création de la requête avec slug unique: %w", err)
				}

				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+c.Token)

				// Envoyer la requête
				resp, err = c.HTTPClient.Do(req)
				if err != nil {
					return "", fmt.Errorf("erreur lors de l'envoi de la requête avec slug unique: %w", err)
				}
				defer resp.Body.Close()

				// Lire la réponse
				body, err = io.ReadAll(resp.Body)
				if err != nil {
					return "", fmt.Errorf("erreur lors de la lecture de la réponse avec slug unique: %w", err)
				}

				// Vérifier si la requête a réussi
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
					return "", fmt.Errorf("erreur lors de la création de la fiche avec slug unique: code %d, réponse: %s", resp.StatusCode, string(body))
				}
			} else {
				return "", fmt.Errorf("erreur lors de la création de la fiche: code %d, réponse: %s", resp.StatusCode, string(body))
			}
		} else {
			return "", fmt.Errorf("erreur lors de la création de la fiche: code %d, réponse: %s", resp.StatusCode, string(body))
		}
	}

	// Décoder la réponse JSON
	var ficheResp FicheResponse
	if err := json.Unmarshal(body, &ficheResp); err != nil {
		return "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Retourner l'ID de la fiche créée
	return fmt.Sprintf("%d", ficheResp.Data.ID), nil
}

// Fonction pour vérifier si un lien existe déjà pour une fiche
func (c *StrapiClient) CheckLinkExists(ficheID, embedURL string) (bool, error) {
	// Vérifier si le token est disponible
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return false, fmt.Errorf("erreur lors de la connexion: %w", err)
		}
	}

	// Créer l'URL avec les filtres
	apiURL := fmt.Sprintf("%s/api/links?filters[fiche][id][$eq]=%s&filters[link][$eq]=%s",
		c.BaseURL, ficheID, url.QueryEscape(embedURL))

	// Créer la requête
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Envoyer la requête
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Vérifier si la requête a réussi
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("erreur lors de la vérification du lien: code %d, réponse: %s", resp.StatusCode, string(body))
	}

	// Décoder la réponse JSON
	var response struct {
		Data []struct {
			ID int `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return false, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Si on a des résultats, le lien existe déjà
	return len(response.Data) > 0, nil
}

// CreateLink crée un nouveau lien dans Strapi
func (c *StrapiClient) CreateLink(ficheID, embedURL string) (string, error) {
	// Vérifier si le token est disponible
	if c.Token == "" {
		if err := c.Login(); err != nil {
			return "", fmt.Errorf("erreur lors de la connexion: %w", err)
		}
	}

	// Vérifier si le lien existe déjà
	exists, err := c.CheckLinkExists(ficheID, embedURL)
	if err != nil {
		log.Printf("Erreur lors de la vérification du lien existant: %v", err)
		// Continuer malgré l'erreur
	}

	if exists {
		log.Printf("Le lien existe déjà pour la fiche %s: %s", ficheID, embedURL)
		return "exists", nil
	}

	// Préparer les données du lien
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"link":  embedURL,
			"fiche": ficheID,
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la sérialisation des données du lien: %w", err)
	}

	// Créer la requête
	req, err := http.NewRequest("POST", c.BaseURL+"/api/links", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Envoyer la requête
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Vérifier si la requête a réussi
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("erreur lors de la création du lien: code %d, réponse: %s", resp.StatusCode, string(body))
	}

	// Décoder la réponse JSON
	var linkResp LinkResponse
	if err := json.Unmarshal(body, &linkResp); err != nil {
		return "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Retourner l'ID du lien créé
	return fmt.Sprintf("%d", linkResp.Data.ID), nil
}

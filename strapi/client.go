package strapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// StrapiClient gère les interactions avec l'API Strapi
type StrapiClient struct {
	BaseURL   string
	AuthToken string
	Username  string
	Password  string
}

// LoginResponse représente la réponse de l'API lors de la connexion
type LoginResponse struct {
	JWT  string `json:"jwt"`
	User struct {
		ID         int    `json:"id"`
		DocumentID string `json:"documentId"`
		Username   string `json:"username"`
		Email      string `json:"email"`
	} `json:"user"`
}

// GenderResponse représente la réponse de l'API pour les genres
type GenderResponse struct {
	Data []struct {
		ID         int    `json:"id"`
		DocumentID string `json:"documentId"`
		Name       string `json:"name"`
		Slug       string `json:"slug"`
	} `json:"data"`
	Meta struct {
		Pagination struct {
			Page      int `json:"page"`
			PageSize  int `json:"pageSize"`
			PageCount int `json:"pageCount"`
			Total     int `json:"total"`
		} `json:"pagination"`
	} `json:"meta"`
}

// CreateFicheRequest représente la requête pour créer une nouvelle fiche
type CreateFicheRequest struct {
	Data struct {
		Title      string   `json:"title"`
		Categories []string `json:"categories"`
		Genders    []string `json:"genders"`
		TmdbID     string   `json:"tmdb_id"`
		TmdbData   string   `json:"tmdb_data"`
		Links      []string `json:"links"`
		Slug       string   `json:"slug"`
		Slider     bool     `json:"slider"`
	} `json:"data"`
}

// CreateLinkRequest représente la requête pour créer un nouveau lien
type CreateLinkRequest struct {
	Data struct {
		Link  string `json:"link"`
		Fiche string `json:"fiche"`
	} `json:"data"`
}

// CreateLinkResponse représente la réponse de l'API lors de la création d'un lien
type CreateLinkResponse struct {
	Data struct {
		ID         int    `json:"id"`
		DocumentID string `json:"documentId"`
		Link       string `json:"link"`
	} `json:"data"`
}

// CreateFicheResponse représente la réponse de l'API lors de la création d'une fiche
type CreateFicheResponse struct {
	Data struct {
		ID         int    `json:"id"`
		DocumentID string `json:"documentId"`
		Title      string `json:"title"`
		Slug       string `json:"slug"`
	} `json:"data"`
}

// NewStrapiClient crée un nouveau client Strapi
func NewStrapiClient(baseURL, username, password string) *StrapiClient {
	return &StrapiClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
	}
}

// Login se connecte à l'API Strapi et récupère un token JWT
func (s *StrapiClient) Login() error {
	log.Printf("Connexion à l'API Strapi...")

	// Préparer les données de connexion
	loginData := map[string]string{
		"identifier": s.Username,
		"password":   s.Password,
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation des données de connexion: %w", err)
	}

	// Créer la requête
	req, err := http.NewRequest("POST", s.BaseURL+"/api/auth/local", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Envoyer la requête
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("erreur lors de la connexion: code %d", resp.StatusCode)
	}

	// Décoder la réponse
	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("erreur lors du décodage de la réponse: %w", err)
	}

	// Stocker le token
	s.AuthToken = loginResp.JWT
	log.Printf("Connexion réussie à l'API Strapi (utilisateur: %s)", loginResp.User.Username)

	return nil
}

// GetGenders récupère la liste des genres disponibles
func (s *StrapiClient) GetGenders() (*GenderResponse, error) {
	log.Printf("Récupération des genres depuis l'API Strapi...")

	// Créer la requête
	req, err := http.NewRequest("GET", s.BaseURL+"/api/genders", nil)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.AuthToken)

	// Envoyer la requête
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("erreur lors de la récupération des genres: code %d", resp.StatusCode)
	}

	// Décoder la réponse
	var genderResp GenderResponse
	if err := json.NewDecoder(resp.Body).Decode(&genderResp); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse: %w", err)
	}

	log.Printf("Récupération de %d genres depuis l'API Strapi", len(genderResp.Data))

	return &genderResp, nil
}

// CreateFiche crée une nouvelle fiche dans Strapi
func (s *StrapiClient) CreateFiche(title, tmdbID, tmdbData string, genderIDs []string) (*CreateFicheResponse, error) {
	log.Printf("Création d'une nouvelle fiche pour %s (TMDB ID: %s)...", title, tmdbID)

	// Préparer les données
	var req CreateFicheRequest
	req.Data.Title = title
	req.Data.TmdbID = tmdbID
	req.Data.TmdbData = tmdbData
	req.Data.Genders = genderIDs
	req.Data.Categories = []string{}
	req.Data.Links = []string{}
	req.Data.Slider = false

	// Générer le slug à partir du titre
	req.Data.Slug = generateSlug(title)

	// Sérialiser en JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la sérialisation des données: %w", err)
	}

	// Créer la requête
	httpReq, err := http.NewRequest("POST", s.BaseURL+"/api/fiches", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.AuthToken)

	// Envoyer la requête
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("erreur lors de la création de la fiche: code %d", resp.StatusCode)
	}

	// Décoder la réponse
	var ficheResp CreateFicheResponse
	if err := json.NewDecoder(resp.Body).Decode(&ficheResp); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse: %w", err)
	}

	log.Printf("Fiche créée avec succès (ID: %s)", ficheResp.Data.DocumentID)

	return &ficheResp, nil
}

// CreateLink crée un nouveau lien dans Strapi
func (s *StrapiClient) CreateLink(link, ficheDocumentID string) (*CreateLinkResponse, error) {
	log.Printf("Création d'un nouveau lien pour la fiche %s...", ficheDocumentID)

	// Préparer les données
	var req CreateLinkRequest
	req.Data.Link = link
	req.Data.Fiche = ficheDocumentID

	// Sérialiser en JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la sérialisation des données: %w", err)
	}

	// Créer la requête
	httpReq, err := http.NewRequest("POST", s.BaseURL+"/api/links", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.AuthToken)

	// Envoyer la requête
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("erreur lors de la création du lien: code %d", resp.StatusCode)
	}

	// Décoder la réponse
	var linkResp CreateLinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&linkResp); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse: %w", err)
	}

	log.Printf("Lien créé avec succès (ID: %s)", linkResp.Data.DocumentID)

	return &linkResp, nil
}

// FindGenderIDsByNames trouve les IDs des genres par leurs noms
func (s *StrapiClient) FindGenderIDsByNames(genreNames []string) ([]string, error) {
	// Récupérer tous les genres
	genders, err := s.GetGenders()
	if err != nil {
		return nil, err
	}

	// Créer une map pour faciliter la recherche
	genderMap := make(map[string]string)
	for _, gender := range genders.Data {
		genderMap[strings.ToLower(gender.Name)] = gender.DocumentID
	}

	// Trouver les IDs correspondants
	var genderIDs []string
	for _, name := range genreNames {
		if id, ok := genderMap[strings.ToLower(name)]; ok {
			genderIDs = append(genderIDs, id)
		} else {
			log.Printf("Genre non trouvé: %s", name)
		}
	}

	return genderIDs, nil
}

// generateSlug génère un slug à partir d'un titre
func generateSlug(title string) string {
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

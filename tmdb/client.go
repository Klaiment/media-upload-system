package tmdb

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// TMDBClient gère les interactions avec l'API TMDB
type TMDBClient struct {
	ApiKey string
}

// MovieDetails représente les détails d'un film
type MovieDetails struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Overview   string `json:"overview"`
	PosterPath string `json:"poster_path"`
	Genres     []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	ReleaseDate string `json:"release_date"`
}

// NewTMDBClient crée un nouveau client TMDB
func NewTMDBClient(apiKey string) *TMDBClient {
	return &TMDBClient{
		ApiKey: apiKey,
	}
}

// GetMovieDetails récupère les détails d'un film par son ID
func (t *TMDBClient) GetMovieDetails(tmdbID int) (*MovieDetails, error) {
	log.Printf("Récupération des détails du film TMDB ID: %d", tmdbID)

	url := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=fr-FR", tmdbID, t.ApiKey)

	// Créer un client HTTP avec timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("l'API a retourné un code non-200: %d", resp.StatusCode)
	}

	var movie MovieDetails
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	log.Printf("Détails du film récupérés: %s (%s)", movie.Title, movie.ReleaseDate)

	return &movie, nil
}

// GetMovieDetailsJSON récupère les détails d'un film au format JSON brut
func (t *TMDBClient) GetMovieDetailsJSON(tmdbID int) (string, error) {
	log.Printf("Récupération des détails complets du film TMDB ID: %d", tmdbID)

	url := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&language=fr-FR&append_to_response=credits,similar,recommendations", tmdbID, t.ApiKey)

	// Créer un client HTTP avec timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("l'API a retourné un code non-200: %d", resp.StatusCode)
	}

	// Lire le corps de la réponse en tant que JSON brut
	var rawData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		return "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Convertir en JSON
	jsonData, err := json.Marshal(rawData)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la sérialisation en JSON: %w", err)
	}

	return string(jsonData), nil
}

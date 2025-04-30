package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NotifyPayload représente le payload à envoyer à l'API
type NotifyPayload struct {
	TmdbID int      `json:"tmdbId"`
	Title  string   `json:"title"`
	Type   string   `json:"type"`
	Links  []Link   `json:"links"`
}

// Link représente un lien d'hébergement
type Link struct {
	Hoster   string `json:"hoster"`
	Link     string `json:"link"`
	FileCode string `json:"fileCode"`
	Season   *int   `json:"season,omitempty"`
	Episode  *int   `json:"episode,omitempty"`
}

// Client représente un client pour notifier l'API
type Client struct {
	Endpoint string
}

// NewClient crée un nouveau client API
func NewClient(endpoint string) *Client {
	return &Client{
		Endpoint: endpoint,
	}
}

// NotifyUpload notifie l'API d'un upload terminé
func (c *Client) NotifyUpload(payload *NotifyPayload) error {
	// Sérialiser le payload en JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation du payload: %w", err)
	}

	// Créer un client HTTP avec timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Envoyer la requête POST
	resp, err := client.Post(c.Endpoint, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Vérifier le code de statut
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("l'API a retourné un code non-2xx: %d", resp.StatusCode)
	}

	return nil
}
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// DiscordWebhook représente le client pour envoyer des notifications à Discord
type DiscordWebhook struct {
	WebhookURL string
}

// DiscordEmbed représente un embed Discord
type DiscordEmbed struct {
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Timestamp   string                 `json:"timestamp,omitempty"`
	Footer      *DiscordEmbedFooter    `json:"footer,omitempty"`
	Thumbnail   *DiscordEmbedThumbnail `json:"thumbnail,omitempty"`
	Image       *DiscordEmbedImage     `json:"image,omitempty"`
	Author      *DiscordEmbedAuthor    `json:"author,omitempty"`
	Fields      []DiscordEmbedField    `json:"fields,omitempty"`
}

// DiscordEmbedFooter représente le footer d'un embed Discord
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordEmbedThumbnail représente la miniature d'un embed Discord
type DiscordEmbedThumbnail struct {
	URL string `json:"url"`
}

// DiscordEmbedImage représente l'image d'un embed Discord
type DiscordEmbedImage struct {
	URL string `json:"url"`
}

// DiscordEmbedAuthor représente l'auteur d'un embed Discord
type DiscordEmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordEmbedField représente un champ d'un embed Discord
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordWebhookPayload représente le payload à envoyer au webhook Discord
type DiscordWebhookPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// TMDBMovie représente les informations d'un film depuis l'API TMDB
type TMDBMovie struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	ReleaseDate      string  `json:"release_date"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	Runtime          int     `json:"runtime"`
	ImdbID           string  `json:"imdb_id"`
	OriginalLanguage string  `json:"original_language"`
	Genres           []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Credits struct {
		Cast []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"cast"`
		Crew []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Job  string `json:"job"`
		} `json:"crew"`
	} `json:"credits"`
}

// NewDiscordWebhook crée un nouveau client pour le webhook Discord
func NewDiscordWebhook(webhookURL string) *DiscordWebhook {
	return &DiscordWebhook{
		WebhookURL: webhookURL,
	}
}

// FetchTMDBMovie récupère les informations d'un film depuis l'API TMDB
func FetchTMDBMovie(tmdbID int, apiKey string) (*TMDBMovie, error) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?api_key=%s&append_to_response=credits,similar,recommendations&language=fr-FR", tmdbID, apiKey)
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la requête à l'API TMDB: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("l'API TMDB a retourné un code non-200: %d", resp.StatusCode)
	}
	
	var movie TMDBMovie
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}
	
	return &movie, nil
}

// NotifyUpload envoie une notification d'upload à Discord
func (d *DiscordWebhook) NotifyUpload(movie *TMDBMovie, links []HostedLink) error {
	// Créer l'embed
	embed := DiscordEmbed{
		Title:       fmt.Sprintf("🎬 Nouveau film disponible: %s (%s)", movie.Title, movie.ReleaseDate[:4]),
		Description: movie.Overview,
		Color:       3447003, // Bleu Discord
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: "Media Upload System",
		},
	}
	
	// Ajouter l'image du film
	if movie.PosterPath != "" {
		embed.Thumbnail = &DiscordEmbedThumbnail{
			URL: fmt.Sprintf("https://image.tmdb.org/t/p/w342%s", movie.PosterPath),
		}
	}
	
	if movie.BackdropPath != "" {
		embed.Image = &DiscordEmbedImage{
			URL: fmt.Sprintf("https://image.tmdb.org/t/p/w780%s", movie.BackdropPath),
		}
	}
	
	// Ajouter les informations du film
	var genres string
	for i, genre := range movie.Genres {
		if i > 0 {
			genres += ", "
		}
		genres += genre.Name
	}
	
	// Trouver le réalisateur
	var director string
	for _, crew := range movie.Credits.Crew {
		if crew.Job == "Director" {
			if director != "" {
				director += ", "
			}
			director += crew.Name
		}
	}
	
	// Ajouter les acteurs principaux (max 3)
	var cast string
	for i, actor := range movie.Credits.Cast {
		if i >= 3 {
			break
		}
		if i > 0 {
			cast += ", "
		}
		cast += actor.Name
	}
	
	// Ajouter les champs
	embed.Fields = []DiscordEmbedField{
		{
			Name:   "📊 Note",
			Value:  fmt.Sprintf("%.1f/10", movie.VoteAverage),
			Inline: true,
		},
		{
			Name:   "⏱️ Durée",
			Value:  fmt.Sprintf("%d min", movie.Runtime),
			Inline: true,
		},
		{
			Name:   "🎭 Genres",
			Value:  genres,
			Inline: true,
		},
	}
	
	if director != "" {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "🎬 Réalisateur",
			Value:  director,
			Inline: true,
		})
	}
	
	if cast != "" {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "🎭 Acteurs",
			Value:  cast,
			Inline: true,
		})
	}
	
	// Ajouter les liens
	var linksField DiscordEmbedField
	linksField.Name = "🔗 Liens"
	
	for _, link := range links {
		linksField.Value += fmt.Sprintf("**%s**: [Regarder](%s) | [Embed](%s)\n", 
			link.Hoster, link.URL, link.Embed)
	}
	
	embed.Fields = append(embed.Fields, linksField)
	
	// Créer le payload
	payload := DiscordWebhookPayload{
		Username:  "Media Upload Bot",
		AvatarURL: "https://cdn-icons-png.flaticon.com/512/2503/2503508.png",
		Embeds:    []DiscordEmbed{embed},
	}
	
	// Sérialiser le payload
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation du payload: %w", err)
	}
	
	// Envoyer la requête
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Post(d.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord a retourné un code non-2xx: %d", resp.StatusCode)
	}
	
	log.Printf("Notification Discord envoyée avec succès")
	return nil
}

// NotifyEpisodeUpload envoie une notification d'upload d'épisode à Discord
func (d *DiscordWebhook) NotifyEpisodeUpload(title string, tmdbID, season, episode int, links []HostedLink) error {
	// Créer l'embed
	embed := DiscordEmbed{
		Title:     fmt.Sprintf("📺 Nouvel épisode disponible: %s S%02dE%02d", title, season, episode),
		Color:     15105570, // Orange
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: "Media Upload System",
		},
	}
	
	// Ajouter les liens
	var linksField DiscordEmbedField
	linksField.Name = "🔗 Liens"
	
	for _, link := range links {
		linksField.Value += fmt.Sprintf("**%s**: [Regarder](%s) | [Embed](%s)\n", 
			link.Hoster, link.URL, link.Embed)
	}
	
	embed.Fields = append(embed.Fields, linksField)
	
	// Créer le payload
	payload := DiscordWebhookPayload{
		Username:  "Media Upload Bot",
		AvatarURL: "https://cdn-icons-png.flaticon.com/512/2503/2503508.png",
		Embeds:    []DiscordEmbed{embed},
	}
	
	// Sérialiser le payload
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation du payload: %w", err)
	}
	
	// Envoyer la requête
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Post(d.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord a retourné un code non-2xx: %d", resp.StatusCode)
	}
	
	log.Printf("Notification Discord envoyée avec succès")
	return nil
}

// HostedLink représente un lien d'hébergement pour Discord
type HostedLink struct {
	Hoster   string
	URL      string
	Embed    string
	FileCode string
}
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config représente la configuration globale de l'application
type Config struct {
	Server struct {
		Port int    `json:"port"`
		Host string `json:"host"`
	} `json:"server"`

	Workers struct {
		MaxConcurrent int `json:"maxConcurrent"`
	} `json:"workers"`

	Uploaders struct {
		Netu struct {
			Enabled bool   `json:"enabled"`
			ApiKey  string `json:"apiKey"`
		} `json:"netu"`
		MixDrop struct {
			Enabled bool   `json:"enabled"`
			Email   string `json:"email"`
			ApiKey  string `json:"apiKey"`
		} `json:"mixdrop"`
		// Vous pouvez ajouter d'autres uploaders ici
	} `json:"uploaders"`

	Database struct {
		Path string `json:"path"`
	} `json:"database"`

	API struct {
		Endpoint string `json:"endpoint"`
	} `json:"api"`

	Discord struct {
		WebhookURL string `json:"webhookUrl"`
	} `json:"discord"`

	TMDB struct {
		ApiKey string `json:"apiKey"`
	} `json:"tmdb"`

	Strapi struct {
		Enabled  bool   `json:"enabled"`
		BaseURL  string `json:"baseUrl"`
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"strapi"`
}

// LoadConfig charge la configuration depuis un fichier JSON
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'ouverture du fichier de configuration: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage du fichier de configuration: %w", err)
	}

	// Valeurs par défaut
	if config.Server.Port == 0 {
		config.Server.Port = 3005
	}
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}
	if config.Workers.MaxConcurrent == 0 {
		config.Workers.MaxConcurrent = 10
	}
	if config.Database.Path == "" {
		config.Database.Path = "./uploads.db"
	}

	return &config, nil
}

// CreateDefaultConfig crée un fichier de configuration par défaut
func CreateDefaultConfig(path string) error {
	config := Config{}

	// Serveur
	config.Server.Port = 3005
	config.Server.Host = "0.0.0.0"

	// Workers
	config.Workers.MaxConcurrent = 10

	// Uploaders
	config.Uploaders.Netu.Enabled = true
	config.Uploaders.Netu.ApiKey = "d81d2161e383e64533b3e0015bfa6b9a"

	config.Uploaders.MixDrop.Enabled = false
	config.Uploaders.MixDrop.Email = "your-email@example.com"
	config.Uploaders.MixDrop.ApiKey = "your-mixdrop-api-key"

	// Database
	config.Database.Path = "./uploads.db"

	// API
	config.API.Endpoint = "https://your-site.com/api/media"

	// Discord
	config.Discord.WebhookURL = "https://discord.com/api/webhooks/your-webhook-url"

	// TMDB
	config.TMDB.ApiKey = "3e104b3f1e36e8c494a6f5e0f7f67e0d"

	// Strapi
	config.Strapi.Enabled = true
	//config.Strapi.BaseURL = "https://api.streameo.me"
	config.Strapi.BaseURL = "https://api.streameo.me"
	config.Strapi.Username = "admin"
	config.Strapi.Password = "Clement123!"

	// Sérialiser en JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("erreur lors de la sérialisation de la configuration: %w", err)
	}

	// Écrire dans le fichier
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("erreur lors de l'écriture du fichier de configuration: %w", err)
	}

	return nil
}

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
		// Ajoutez d'autres hébergeurs ici
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
	
	config.Uploaders.MixDrop.Enabled = true
	config.Uploaders.MixDrop.Email = "streameo@proton.me"
	config.Uploaders.MixDrop.ApiKey = "bX5CPdwaeQQ5DBJUX"
	
	// Database
	config.Database.Path = "./uploads.db"
	
	// API
	config.API.Endpoint = "https://your-site.com/api/media"
	
	// Discord
	config.Discord.WebhookURL = "https://discordapp.com/api/webhooks/1367197008187752522/rzjd_SKy8LkV1YAOkDys7y-XVi9GSQtvfHEmRdalE7k1VA4iozt0mFUTrSB0sugWwZvY"
	
	// TMDB
	config.TMDB.ApiKey = "bfdc88d1d7b360fca90425956dcb6951"
	
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
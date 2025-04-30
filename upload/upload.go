package upload

import (
	"fmt"
)

// Uploader définit l'interface pour les uploaders de fichiers
type Uploader interface {
	Name() string
	IsEnabled() bool
	UploadFile(filePath, title string) (*UploadResult, error)
}

// UploadResult représente le résultat d'un upload
type UploadResult struct {
	Success  bool
	Hoster   string
	FileCode string
	URL      string
	Embed    string // URL d'embed pour les lecteurs vidéo
}

// Manager gère les différents uploaders
type Manager struct {
	uploaders map[string]Uploader
}

// NewManager crée un nouveau gestionnaire d'uploaders
func NewManager() *Manager {
	return &Manager{
		uploaders: make(map[string]Uploader),
	}
}

// RegisterUploader enregistre un uploader
func (m *Manager) RegisterUploader(uploader Uploader) {
	m.uploaders[uploader.Name()] = uploader
}

// GetUploader récupère un uploader par son nom
func (m *Manager) GetUploader(name string) (Uploader, error) {
	uploader, ok := m.uploaders[name]
	if !ok {
		return nil, fmt.Errorf("uploader non trouvé: %s", name)
	}
	return uploader, nil
}

// GetEnabledUploaders récupère la liste des uploaders activés
func (m *Manager) GetEnabledUploaders() []Uploader {
	var enabledUploaders []Uploader
	for _, uploader := range m.uploaders {
		if uploader.IsEnabled() {
			enabledUploaders = append(enabledUploaders, uploader)
		}
	}
	return enabledUploaders
}

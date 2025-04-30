package upload

import (
	"fmt"
	"log"
)

// UploadResult représente le résultat d'un upload
type UploadResult struct {
	Hoster    string
	FileCode  string
	URL       string
	Embed     string
	Success   bool
	Error     error
}

// Uploader est l'interface que tous les hébergeurs doivent implémenter
type Uploader interface {
	Name() string
	UploadFile(filePath, title string) (*UploadResult, error)
	IsEnabled() bool
}

// Manager gère les uploads vers différents hébergeurs
type Manager struct {
	uploaders []Uploader
}

// NewManager crée un nouveau gestionnaire d'uploads
func NewManager() *Manager {
	return &Manager{
		uploaders: []Uploader{},
	}
}

// RegisterUploader enregistre un nouvel uploader
func (m *Manager) RegisterUploader(uploader Uploader) {
	m.uploaders = append(m.uploaders, uploader)
	log.Printf("Uploader enregistré: %s", uploader.Name())
}

// UploadToAll upload un fichier vers tous les hébergeurs enregistrés et activés
func (m *Manager) UploadToAll(filePath, title string) []*UploadResult {
	results := make([]*UploadResult, 0)
	
	for _, uploader := range m.uploaders {
		if !uploader.IsEnabled() {
			log.Printf("Uploader %s désactivé, ignoré", uploader.Name())
			continue
		}
		
		log.Printf("Upload vers %s: %s", uploader.Name(), filePath)
		result, err := uploader.UploadFile(filePath, title)
		
		if err != nil {
			log.Printf("Erreur lors de l'upload vers %s: %v", uploader.Name(), err)
			results = append(results, &UploadResult{
				Hoster:  uploader.Name(),
				Success: false,
				Error:   err,
			})
		} else {
			log.Printf("Upload réussi vers %s: %s", uploader.Name(), result.URL)
			results = append(results, result)
		}
	}
	
	return results
}

// GetEnabledUploaders retourne la liste des uploaders activés
func (m *Manager) GetEnabledUploaders() []Uploader {
	enabled := make([]Uploader, 0)
	
	for _, uploader := range m.uploaders {
		if uploader.IsEnabled() {
			enabled = append(enabled, uploader)
		}
	}
	
	return enabled
}

// GetUploader retourne un uploader par son nom
func (m *Manager) GetUploader(name string) (Uploader, error) {
	for _, uploader := range m.uploaders {
		if uploader.Name() == name {
			return uploader, nil
		}
	}
	
	return nil, fmt.Errorf("uploader non trouvé: %s", name)
}
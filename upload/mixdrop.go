package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// MixDropUploader gère les uploads vers MixDrop
type MixDropUploader struct {
	Email   string
	ApiKey  string
	Enabled bool
}

// MixDropResponse représente la réponse de l'API MixDrop
type MixDropResponse struct {
	Success bool `json:"success"`
	Result  struct {
		FileRef string `json:"fileref"`
		Title   string `json:"title"`
		Status  string `json:"status"`
	} `json:"result"`
}

// NewMixDropUploader crée un nouvel uploader MixDrop
func NewMixDropUploader(email, apiKey string, enabled bool) *MixDropUploader {
	return &MixDropUploader{
		Email:   email,
		ApiKey:  apiKey,
		Enabled: enabled,
	}
}

// Name retourne le nom de l'uploader
func (m *MixDropUploader) Name() string {
	return "mixdrop"
}

// IsEnabled indique si l'uploader est activé
func (m *MixDropUploader) IsEnabled() bool {
	return m.Enabled
}

// UploadFile upload un fichier vers MixDrop
func (m *MixDropUploader) UploadFile(filePath, title string) (*UploadResult, error) {
	log.Printf("Upload du fichier %s vers MixDrop...", filePath)

	// Vérifier si le fichier existe
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("le fichier n'existe pas: %s", filePath)
	}

	// Ouvrir le fichier
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'ouverture du fichier: %w", err)
	}
	defer file.Close()

	// Créer un buffer pour le corps de la requête
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Ajouter les champs du formulaire
	if err := writer.WriteField("email", m.Email); err != nil {
		return nil, fmt.Errorf("erreur lors de l'ajout du champ email: %w", err)
	}

	if err := writer.WriteField("key", m.ApiKey); err != nil {
		return nil, fmt.Errorf("erreur lors de l'ajout du champ key: %w", err)
	}

	if err := writer.WriteField("title", title); err != nil {
		return nil, fmt.Errorf("erreur lors de l'ajout du champ title: %w", err)
	}

	// Ajouter le fichier
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du champ file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("erreur lors de la copie du fichier: %w", err)
	}

	// Fermer le writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("erreur lors de la fermeture du writer: %w", err)
	}

	// Créer la requête
	log.Printf("Envoi de la requête à MixDrop...")
	req, err := http.NewRequest("POST", "https://ul.mixdrop.ag/api", &requestBody)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Envoyer la requête
	client := &http.Client{
		Timeout: 24 * time.Hour, // Timeout très long pour les gros fichiers
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Décoder la réponse JSON
	var response MixDropResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w, réponse: %s", err, string(body))
	}

	// Vérifier si l'upload a réussi
	if !response.Success {
		return nil, fmt.Errorf("l'upload a échoué: %s", string(body))
	}

	log.Printf("Fichier uploadé avec succès sur MixDrop, fileref: %s", response.Result.FileRef)

	// Construire les URLs avec le nouveau domaine mixdrop.ag
	directURL := fmt.Sprintf("https://mixdrop.ag/e/%s", response.Result.FileRef)
	embedURL := fmt.Sprintf("https://mixdrop.ag/e/%s", response.Result.FileRef)

	// Créer le résultat
	result := &UploadResult{
		Success:  true,
		Hoster:   "mixdrop",
		FileCode: response.Result.FileRef,
		URL:      directURL,
		Embed:    embedURL,
	}

	return result, nil
}

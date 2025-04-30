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
		FileRef  string `json:"fileref"`
		URL      string `json:"url"`
		EmbedURL string `json:"embedurl"`
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

	// Vérifier que le fichier existe
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
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Ajouter les paramètres du formulaire
	if err := writer.WriteField("email", m.Email); err != nil {
		return nil, fmt.Errorf("erreur lors de l'écriture du champ email: %w", err)
	}

	if err := writer.WriteField("key", m.ApiKey); err != nil {
		return nil, fmt.Errorf("erreur lors de l'écriture du champ key: %w", err)
	}

	// Ajouter le fichier
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création du champ de fichier: %w", err)
	}

	// Copier le fichier dans le formulaire
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("erreur lors de la copie du fichier: %w", err)
	}

	// Fermer le writer pour finaliser le formulaire
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("erreur lors de la fermeture du writer: %w", err)
	}

	// Créer la requête HTTP
	req, err := http.NewRequest("POST", "https://ul.mixdrop.ag/api", body)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	// Définir les headers
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Créer un client HTTP avec timeout plus long pour l'upload
	client := &http.Client{
		Timeout: 3 * time.Hour, // Timeout très long pour les gros fichiers
	}

	// Envoyer la requête
	log.Printf("Envoi de la requête à MixDrop...")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MixDrop a retourné un code non-200: %d", resp.StatusCode)
	}

	// Décoder la réponse
	var result MixDropResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("l'upload a échoué")
	}

	log.Printf("Fichier uploadé avec succès sur MixDrop, fileref: %s", result.Result.FileRef)

	return &UploadResult{
		Hoster:   "mixdrop",
		FileCode: result.Result.FileRef,
		URL:      result.Result.URL,
		Embed:    result.Result.EmbedURL,
		Success:  true,
	}, nil
}
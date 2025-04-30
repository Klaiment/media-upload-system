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
	"strconv"
	"time"
)

// NetuUploader gère les uploads vers Netu.tv
type NetuUploader struct {
	ApiKey  string
	Enabled bool
}

// NetuUploadServerResponse représente la réponse de l'API pour obtenir le serveur d'upload
type NetuUploadServerResponse struct {
	Status  int  `json:"status"`
	Success bool `json:"success"`
	Result  struct {
		URL string `json:"url"`
	} `json:"result"`
}

// NetuUploadResponse représente la réponse de l'API après l'upload du fichier
type NetuUploadResponse struct {
	Status  int  `json:"status"`
	Success bool `json:"success"`
	Result  struct {
		FileCode string `json:"file_code"`
	} `json:"result"`
}

// NetuUploadFileResponse représente la réponse du serveur d'upload
type NetuUploadFileResponse struct {
	Status   int   `json:"status"`
	Success  bool  `json:"success"`
	TimeHash int64 `json:"time_hash"`
}

// NewNetuUploader crée un nouvel uploader Netu.tv
func NewNetuUploader(apiKey string, enabled bool) *NetuUploader {
	return &NetuUploader{
		ApiKey:  apiKey,
		Enabled: enabled,
	}
}

// Name retourne le nom de l'uploader
func (n *NetuUploader) Name() string {
	return "netu"
}

// IsEnabled indique si l'uploader est activé
func (n *NetuUploader) IsEnabled() bool {
	return n.Enabled
}

// UploadFile upload un fichier vers Netu.tv
func (n *NetuUploader) UploadFile(filePath, title string) (*UploadResult, error) {
	// Étape 1: Obtenir le serveur d'upload
	log.Printf("Étape 1: Obtention du serveur d'upload...")
	uploadServer, err := n.getUploadServer()
	if err != nil {
		return nil, fmt.Errorf("échec de l'obtention du serveur d'upload: %w", err)
	}

	// Étape 2: Uploader le fichier sur le serveur
	log.Printf("Étape 2: Upload du fichier sur le serveur %s...", uploadServer)
	timeHash, err := n.uploadToServer(filePath, uploadServer)
	if err != nil {
		return nil, fmt.Errorf("échec de l'upload du fichier: %w", err)
	}

	// Étape 3: Finaliser l'upload
	log.Printf("Étape 3: Finalisation de l'upload...")
	fileCode, err := n.finalizeUpload(filepath.Base(filePath), title, timeHash)
	if err != nil {
		return nil, fmt.Errorf("échec de la finalisation de l'upload: %w", err)
	}

	log.Printf("Upload terminé avec succès, code du fichier: %s", fileCode)

	// Construire les URLs avec le domaine player1.streameo.me pour le player
	directURL := fmt.Sprintf("https://player1.streameo.me/watch/%s", fileCode)
	embedURL := fmt.Sprintf("https://player1.streameo.me/embed/%s", fileCode)

	// Créer le résultat
	result := &UploadResult{
		Success:  true,
		Hoster:   "netu",
		FileCode: fileCode,
		URL:      directURL,
		Embed:    embedURL,
	}

	return result, nil
}

// getUploadServer obtient l'URL du serveur d'upload
func (n *NetuUploader) getUploadServer() (string, error) {
	// Utiliser netu.tv pour les appels API
	url := fmt.Sprintf("https://netu.tv/api/file/upload_server?key=%s", n.ApiKey)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("le serveur a retourné un code non-200: %d", resp.StatusCode)
	}

	var response NetuUploadServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	if !response.Success {
		return "", fmt.Errorf("l'API a retourné une erreur: %d", response.Status)
	}

	return response.Result.URL, nil
}

// uploadToServer upload le fichier sur le serveur
func (n *NetuUploader) uploadToServer(filePath, serverURL string) (string, error) {
	// Vérifier si le fichier existe
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("le fichier n'existe pas: %s", filePath)
	}

	// Ouvrir le fichier
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("erreur lors de l'ouverture du fichier: %w", err)
	}
	defer file.Close()

	// Créer un buffer pour le corps de la requête
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Ajouter le fichier
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("erreur lors de la création du champ file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("erreur lors de la copie du fichier: %w", err)
	}

	// Fermer le writer
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("erreur lors de la fermeture du writer: %w", err)
	}

	// Créer la requête
	req, err := http.NewRequest("POST", serverURL, &requestBody)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la création de la requête: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Envoyer la requête
	client := &http.Client{
		Timeout: 24 * time.Hour, // Timeout très long pour les gros fichiers
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	// Lire la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	// Décoder la réponse JSON
	var response NetuUploadFileResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w, réponse: %s", err, string(body))
	}

	// Vérifier si l'upload a réussi
	if !response.Success {
		return "", fmt.Errorf("l'upload a échoué: %d", response.Status)
	}

	// Convertir le time_hash en string
	timeHash := strconv.FormatInt(response.TimeHash, 10)

	return timeHash, nil
}

// finalizeUpload finalise l'upload et obtient le code du fichier
func (n *NetuUploader) finalizeUpload(filename, title, timeHash string) (string, error) {
	// Utiliser netu.tv pour les appels API
	url := fmt.Sprintf("https://netu.tv/api/file/create?key=%s&name=%s&description=%s&time_hash=%s", n.ApiKey, filename, title, timeHash)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("le serveur a retourné un code non-200: %d", resp.StatusCode)
	}

	var response NetuUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	if !response.Success {
		return "", fmt.Errorf("l'API a retourné une erreur: %d", response.Status)
	}

	return response.Result.FileCode, nil
}

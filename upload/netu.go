package upload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
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

// NetuUploadServerResponse représente la réponse de l'API pour obtenir un serveur d'upload
type NetuUploadServerResponse struct {
	Msg    string `json:"msg"`
	Status int    `json:"status"`
	Result struct {
		UploadServer string `json:"upload_server"`
		ServerID     string `json:"server_id"`
		Hash         string `json:"hash"`
		TimeHash     int64  `json:"time_hash"`
		UserID       string `json:"userid"`
		KeyHash      string `json:"key_hash"`
	} `json:"result"`
}

// NetuUploadResponse représente la réponse après l'upload du fichier
type NetuUploadResponse struct {
	Success  string `json:"success"`
	FileName string `json:"file_name"`
}

// NetuFinalizeResponse représente la réponse finale après l'ajout du fichier
type NetuFinalizeResponse struct {
	Msg    string `json:"msg"`
	Status int    `json:"status"`
	Result struct {
		FileCode      string `json:"file_code"`
		FolderID      string `json:"folder_id"`
		FileCodeEmbed string `json:"file_code_embed"`
	} `json:"result"`
}

// NewNetuUploader crée un nouvel uploader Netu
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

// UploadFile upload un fichier vers Netu.tv en suivant les 3 étapes
func (n *NetuUploader) UploadFile(filePath, title string) (*UploadResult, error) {
	// Vérifier que le fichier existe
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("le fichier n'existe pas: %s", filePath)
	}

	// Étape 1: Obtenir l'URL d'upload
	uploadServer, err := n.getUploadServer()
	if err != nil {
		return nil, fmt.Errorf("échec de l'obtention du serveur d'upload: %w", err)
	}

	// Étape 2: Uploader le fichier
	uploadResult, err := n.uploadToServer(uploadServer, filePath)
	if err != nil {
		return nil, fmt.Errorf("échec de l'upload du fichier: %w", err)
	}

	// Étape 3: Finaliser l'upload
	finalResult, err := n.finalizeUpload(
		title,
		uploadServer.Result.UploadServer,
		uploadServer.Result.ServerID,
		uploadResult.FileName,
	)
	if err != nil {
		return nil, fmt.Errorf("échec de la finalisation de l'upload: %w", err)
	}

	directURL := fmt.Sprintf("https://player1.streameo.me/watch/%s", finalResult.Result.FileCode)
	embedURL := fmt.Sprintf("https://player1.streameo.me/embed/%s", finalResult.Result.FileCode)

	// Créer le résultat
	result := &UploadResult{
		Success:  true,
		Hoster:   "netu",
		FileCode: finalResult.Result.FileCode,
		URL:      directURL,
		Embed:    embedURL,
	}

	return result, nil
}

// getUploadServer obtient l'URL du serveur d'upload (étape 1)
func (n *NetuUploader) getUploadServer() (*NetuUploadServerResponse, error) {
	log.Printf("Étape 1: Obtention du serveur d'upload...")

	apiURL := fmt.Sprintf("https://netu.tv/api/file/upload_server?key=%s", n.ApiKey)

	// Créer un client HTTP avec timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("l'API a retourné un code non-200: %d", resp.StatusCode)
	}

	var result NetuUploadServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("l'API a retourné une erreur: %s", result.Msg)
	}

	log.Printf("Serveur d'upload obtenu: %s", result.Result.UploadServer)
	return &result, nil
}

// uploadToServer upload le fichier vers le serveur d'upload (étape 2)
func (n *NetuUploader) uploadToServer(serverInfo *NetuUploadServerResponse, filePath string) (*NetuUploadResponse, error) {
	log.Printf("Étape 2: Upload du fichier %s vers le serveur...", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'ouverture du fichier: %w", err)
	}
	defer file.Close()

	// Créer un buffer pour le corps de la requête
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Ajouter les paramètres du formulaire
	params := map[string]string{
		"hash":      serverInfo.Result.Hash,
		"time_hash": strconv.FormatInt(serverInfo.Result.TimeHash, 10),
		"userid":    serverInfo.Result.UserID,
		"key_hash":  serverInfo.Result.KeyHash,
		"upload":    "1",
	}

	for key, val := range params {
		if err := writer.WriteField(key, val); err != nil {
			return nil, fmt.Errorf("erreur lors de l'écriture du champ %s: %w", key, err)
		}
	}

	// Ajouter le fichier
	part, err := writer.CreateFormFile("Filedata", filepath.Base(filePath))
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
	req, err := http.NewRequest("POST", serverInfo.Result.UploadServer, body)
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
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'envoi de la requête: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("le serveur d'upload a retourné un code non-200: %d", resp.StatusCode)
	}

	// Décoder la réponse
	var result NetuUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	if result.Success != "yes" {
		return nil, fmt.Errorf("l'upload a échoué")
	}

	log.Printf("Fichier uploadé avec succès, file_name: %s", result.FileName)
	return &result, nil
}

// finalizeUpload finalise l'upload en envoyant les informations au serveur principal (étape 3)
func (n *NetuUploader) finalizeUpload(name, server, serverID, fileName string) (*NetuFinalizeResponse, error) {
	log.Printf("Étape 3: Finalisation de l'upload...")

	// Encoder les paramètres dans l'URL
	params := url.Values{}
	params.Add("key", n.ApiKey)
	params.Add("name", name)
	params.Add("server", server)
	params.Add("server_id", serverID)
	params.Add("file_name", fileName)

	// Construire l'URL
	finalizeURL := fmt.Sprintf("https://netu.tv/api/file/add?%s", params.Encode())

	// Créer un client HTTP avec timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Envoyer la requête POST
	resp, err := client.Post(finalizeURL, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("l'API a retourné un code non-200: %d", resp.StatusCode)
	}

	// Décoder la réponse
	var result NetuFinalizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	if result.Status != 200 {
		return nil, fmt.Errorf("l'API a retourné une erreur: %s", result.Msg)
	}

	log.Printf("Upload finalisé avec succès, file_code: %s", result.Result.FileCode)
	return &result, nil
}

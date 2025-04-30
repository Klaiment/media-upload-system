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

// NetuUploadServerResponse représente la réponse de l'API pour obtenir le serveur d'upload
type NetuUploadServerResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		UploadServer string `json:"upload_server"`
		ServerID     string `json:"server_id"`
		Hash         string `json:"hash"`
		TimeHash     int64  `json:"time_hash"`
		UserID       string `json:"userid"`
		KeyHash      string `json:"key_hash"`
	} `json:"result"`
}

// NetuUploadResponse représente la réponse de l'API après l'upload du fichier
type NetuUploadResponse struct {
	Status int    `json:"status"`
	Msg    string `json:"msg"`
	Result struct {
		FileCode      string `json:"file_code"`
		FolderID      string `json:"folder_id"`
		FileCodeEmbed string `json:"file_code_embed"`
	} `json:"result"`
}

// NetuUploadFileResponse représente la réponse du serveur d'upload
type NetuUploadFileResponse struct {
	Success  string `json:"success"`
	FileName string `json:"file_name"`
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
	serverInfo, err := n.getUploadServer()
	if err != nil {
		return nil, fmt.Errorf("échec de l'obtention du serveur d'upload: %w", err)
	}

	// Étape 2: Uploader le fichier sur le serveur
	log.Printf("Étape 2: Upload du fichier sur le serveur %s...", serverInfo.UploadServer)
	fileName, err := n.uploadToServer(filePath, serverInfo)
	if err != nil {
		return nil, fmt.Errorf("échec de l'upload du fichier: %w", err)
	}

	// Étape 3: Finaliser l'upload
	log.Printf("Étape 3: Finalisation de l'upload...")
	fileCode, fileCodeEmbed, err := n.finalizeUpload(fileName, title, serverInfo)
	if err != nil {
		return nil, fmt.Errorf("échec de la finalisation de l'upload: %w", err)
	}

	log.Printf("Upload terminé avec succès, code du fichier: %s, code embed: %s", fileCode, fileCodeEmbed)

	// Construire les URLs avec le nouveau format pour l'embed
	directURL := fmt.Sprintf("https://player1.streameo.me/e/%s", fileCodeEmbed)
	embedURL := fmt.Sprintf("https://player1.streameo.me/e/%s", fileCodeEmbed)

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

// Structure pour stocker les informations du serveur d'upload
type ServerInfo struct {
	UploadServer string
	ServerID     string
	Hash         string
	TimeHash     int64
	UserID       string
	KeyHash      string
}

// getUploadServer obtient l'URL du serveur d'upload
func (n *NetuUploader) getUploadServer() (*ServerInfo, error) {
	// Utiliser netu.tv pour les appels API
	apiURL := fmt.Sprintf("https://netu.tv/api/file/upload_server?key=%s", n.ApiKey)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("le serveur a retourné un code non-200: %d", resp.StatusCode)
	}

	// Lire le corps de la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	log.Printf("Réponse du serveur pour upload_server: %s", string(body))

	// Décoder la réponse JSON
	var response NetuUploadServerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Vérifier si le statut est 200 (OK)
	if response.Status != 200 {
		return nil, fmt.Errorf("l'API a retourné un statut non-200: %d", response.Status)
	}

	// Créer et retourner les informations du serveur
	serverInfo := &ServerInfo{
		UploadServer: response.Result.UploadServer,
		ServerID:     response.Result.ServerID,
		Hash:         response.Result.Hash,
		TimeHash:     response.Result.TimeHash,
		UserID:       response.Result.UserID,
		KeyHash:      response.Result.KeyHash,
	}

	return serverInfo, nil
}

// uploadToServer upload le fichier sur le serveur
func (n *NetuUploader) uploadToServer(filePath string, serverInfo *ServerInfo) (string, error) {
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

	// Ajouter les champs du formulaire
	if err := writer.WriteField("hash", serverInfo.Hash); err != nil {
		return "", fmt.Errorf("erreur lors de l'ajout du champ hash: %w", err)
	}
	if err := writer.WriteField("time_hash", strconv.FormatInt(serverInfo.TimeHash, 10)); err != nil {
		return "", fmt.Errorf("erreur lors de l'ajout du champ time_hash: %w", err)
	}
	if err := writer.WriteField("userid", serverInfo.UserID); err != nil {
		return "", fmt.Errorf("erreur lors de l'ajout du champ userid: %w", err)
	}
	if err := writer.WriteField("key_hash", serverInfo.KeyHash); err != nil {
		return "", fmt.Errorf("erreur lors de l'ajout du champ key_hash: %w", err)
	}
	if err := writer.WriteField("upload", "1"); err != nil {
		return "", fmt.Errorf("erreur lors de l'ajout du champ upload: %w", err)
	}

	// Ajouter le fichier
	part, err := writer.CreateFormFile("Filedata", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("erreur lors de la création du champ Filedata: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("erreur lors de la copie du fichier: %w", err)
	}

	// Fermer le writer
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("erreur lors de la fermeture du writer: %w", err)
	}

	// Créer la requête
	req, err := http.NewRequest("POST", serverInfo.UploadServer, &requestBody)
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

	log.Printf("Réponse du serveur pour l'upload: %s", string(body))

	// Décoder la réponse JSON
	var response NetuUploadFileResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w, réponse: %s", err, string(body))
	}

	// Vérifier si l'upload a réussi
	if response.Success != "yes" {
		return "", fmt.Errorf("l'upload a échoué, success: %s", response.Success)
	}

	// Vérifier si le nom du fichier est vide
	if response.FileName == "" {
		return "", fmt.Errorf("le serveur a retourné un nom de fichier vide")
	}

	return response.FileName, nil
}

// finalizeUpload finalise l'upload et obtient le code du fichier
func (n *NetuUploader) finalizeUpload(fileName, title string, serverInfo *ServerInfo) (string, string, error) {
	// Encoder les paramètres pour l'URL
	params := url.Values{}
	params.Add("key", n.ApiKey)
	params.Add("name", title)
	params.Add("server", serverInfo.ServerID) // Utiliser server_id au lieu de l'URL complète
	params.Add("file_name", fileName)

	// Utiliser netu.tv pour les appels API
	apiURL := "https://netu.tv/api/file/add?" + params.Encode()

	log.Printf("URL de finalisation: %s", apiURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return "", "", fmt.Errorf("erreur lors de la requête HTTP: %w", err)
	}
	defer resp.Body.Close()

	// Lire le corps de la réponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("erreur lors de la lecture de la réponse: %w", err)
	}

	log.Printf("Réponse du serveur pour finaliser l'upload: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("le serveur a retourné un code non-200: %d, réponse: %s", resp.StatusCode, string(body))
	}

	// Décoder la réponse JSON
	var response NetuUploadResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", "", fmt.Errorf("erreur lors du décodage de la réponse JSON: %w", err)
	}

	// Vérifier si le statut est 200 (OK)
	if response.Status != 200 {
		return "", "", fmt.Errorf("l'API a retourné un statut non-200: %d", response.Status)
	}

	// Vérifier si le code du fichier est vide
	if response.Result.FileCode == "" {
		return "", "", fmt.Errorf("l'API a retourné un code de fichier vide")
	}

	// Vérifier si le code embed du fichier est vide
	if response.Result.FileCodeEmbed == "" {
		return "", "", fmt.Errorf("l'API a retourné un code embed vide")
	}

	return response.Result.FileCode, response.Result.FileCodeEmbed, nil
}

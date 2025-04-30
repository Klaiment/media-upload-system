package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// UploadStatus représente le statut d'un upload
type UploadStatus string

const (
	StatusPending   UploadStatus = "pending"
	StatusUploading UploadStatus = "uploading"
	StatusCompleted UploadStatus = "completed"
	StatusFailed    UploadStatus = "failed"
)

// MediaType représente le type de média
type MediaType string

const (
	TypeMovie  MediaType = "movie"
	TypeSeries MediaType = "series"
)

// HostedLink représente un lien d'hébergement
type HostedLink struct {
	Hoster    string `json:"hoster"`
	FileCode  string `json:"fileCode"`
	URL       string `json:"url"`
	Embed     string `json:"embed"`
	CreatedAt string `json:"createdAt"`
}

// Upload représente un upload dans la base de données
type Upload struct {
	ID           int64
	TmdbID       int
	Title        string
	Type         MediaType
	Season       *int
	Episode      *int
	FilePath     string
	Links        []HostedLink
	UploadStatus UploadStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Database gère la base de données SQLite
type Database struct {
	db *sql.DB
}

// NewDatabase crée une nouvelle instance de la base de données
func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'ouverture de la base de données: %w", err)
	}

	// Vérifier la connexion
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erreur lors du ping de la base de données: %w", err)
	}

	// Créer l'instance
	database := &Database{
		db: db,
	}

	// Initialiser la base de données
	if err := database.Initialize(); err != nil {
		return nil, fmt.Errorf("erreur lors de l'initialisation de la base de données: %w", err)
	}

	return database, nil
}

// Close ferme la connexion à la base de données
func (d *Database) Close() error {
	return d.db.Close()
}

// Initialize initialise la base de données
func (d *Database) Initialize() error {
	// Créer la table uploads
	query := `
	CREATE TABLE IF NOT EXISTS uploads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tmdb_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		type TEXT NOT NULL,
		season INTEGER,
		episode INTEGER,
		file_path TEXT NOT NULL,
		links TEXT DEFAULT '[]',
		upload_status TEXT NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_tmdb_type ON uploads(tmdb_id, type);
	`

	_, err := d.db.Exec(query)
	if err != nil {
		return fmt.Errorf("erreur lors de la création de la table: %w", err)
	}

	log.Printf("Base de données initialisée avec succès")
	return nil
}

// AddUpload ajoute un nouvel upload à la base de données
func (d *Database) AddUpload(upload *Upload) (int64, error) {
	query := `
	INSERT INTO uploads (
		tmdb_id, title, type, season, episode, file_path, upload_status
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.Exec(
		query,
		upload.TmdbID,
		upload.Title,
		upload.Type,
		upload.Season,
		upload.Episode,
		upload.FilePath,
		upload.UploadStatus,
	)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de l'ajout de l'upload: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("erreur lors de la récupération de l'ID: %w", err)
	}

	return id, nil
}

// UpdateUploadStatus met à jour le statut d'un upload
func (d *Database) UpdateUploadStatus(id int64, status UploadStatus) error {
	query := `
	UPDATE uploads
	SET upload_status = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := d.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
	}

	return nil
}

// AddUploadLink ajoute un lien d'hébergement à un upload
func (d *Database) AddUploadLink(id int64, link HostedLink) error {
	// Récupérer les liens existants
	query := `
	SELECT links
	FROM uploads
	WHERE id = ?
	`

	var linksJSON string
	err := d.db.QueryRow(query, id).Scan(&linksJSON)
	if err != nil {
		return fmt.Errorf("erreur lors de la récupération des liens: %w", err)
	}

	// Décoder les liens existants
	var links []HostedLink
	if err := json.Unmarshal([]byte(linksJSON), &links); err != nil {
		// Si le JSON est invalide, initialiser un tableau vide
		links = []HostedLink{}
	}

	// Ajouter le nouveau lien
	link.CreatedAt = time.Now().Format(time.RFC3339)
	links = append(links, link)

	// Encoder les liens mis à jour
	updatedLinksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("erreur lors de l'encodage des liens: %w", err)
	}

	// Mettre à jour la base de données
	updateQuery := `
	UPDATE uploads
	SET links = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err = d.db.Exec(updateQuery, string(updatedLinksJSON), id)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour des liens: %w", err)
	}

	return nil
}

// GetUpload récupère un upload par son ID
func (d *Database) GetUpload(id int64) (*Upload, error) {
	query := `
	SELECT id, tmdb_id, title, type, season, episode, file_path, links, upload_status, created_at, updated_at
	FROM uploads
	WHERE id = ?
	`

	row := d.db.QueryRow(query, id)

	var upload Upload
	var createdAt, updatedAt string
	var season, episode sql.NullInt64
	var linksJSON string

	err := row.Scan(
		&upload.ID,
		&upload.TmdbID,
		&upload.Title,
		&upload.Type,
		&season,
		&episode,
		&upload.FilePath,
		&linksJSON,
		&upload.UploadStatus,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("erreur lors de la récupération de l'upload: %w", err)
	}

	// Convertir les valeurs nullables
	if season.Valid {
		seasonVal := int(season.Int64)
		upload.Season = &seasonVal
	}
	if episode.Valid {
		episodeVal := int(episode.Int64)
		upload.Episode = &episodeVal
	}

	// Décoder les liens
	if err := json.Unmarshal([]byte(linksJSON), &upload.Links); err != nil {
		upload.Links = []HostedLink{}
	}

	// Convertir les dates
	upload.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	upload.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &upload, nil
}

// GetPendingUploads récupère les uploads en attente
func (d *Database) GetPendingUploads() ([]*Upload, error) {
	query := `
	SELECT id, tmdb_id, title, type, season, episode, file_path, links, upload_status, created_at, updated_at
	FROM uploads
	WHERE upload_status = ?
	ORDER BY created_at ASC
	`

	rows, err := d.db.Query(query, StatusPending)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des uploads en attente: %w", err)
	}
	defer rows.Close()

	var uploads []*Upload
	for rows.Next() {
		var upload Upload
		var createdAt, updatedAt string
		var season, episode sql.NullInt64
		var linksJSON string

		err := rows.Scan(
			&upload.ID,
			&upload.TmdbID,
			&upload.Title,
			&upload.Type,
			&season,
			&episode,
			&upload.FilePath,
			&linksJSON,
			&upload.UploadStatus,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("erreur lors du scan d'un upload: %w", err)
		}

		// Convertir les valeurs nullables
		if season.Valid {
			seasonVal := int(season.Int64)
			upload.Season = &seasonVal
		}
		if episode.Valid {
			episodeVal := int(episode.Int64)
			upload.Episode = &episodeVal
		}

		// Décoder les liens
		if err := json.Unmarshal([]byte(linksJSON), &upload.Links); err != nil {
			upload.Links = []HostedLink{}
		}

		// Convertir les dates
		upload.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		upload.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		uploads = append(uploads, &upload)
	}

	return uploads, nil
}

// CheckExistingUpload vérifie si un upload existe déjà
func (d *Database) CheckExistingUpload(tmdbID int, mediaType MediaType, season, episode *int) (*Upload, error) {
	var query string
	var args []interface{}

	if mediaType == TypeMovie {
		query = `
		SELECT id, tmdb_id, title, type, season, episode, file_path, links, upload_status, created_at, updated_at
		FROM uploads
		WHERE tmdb_id = ? AND type = ? AND season IS NULL AND episode IS NULL
		ORDER BY created_at DESC
		LIMIT 1
		`
		args = []interface{}{tmdbID, mediaType}
	} else {
		query = `
		SELECT id, tmdb_id, title, type, season, episode, file_path, links, upload_status, created_at, updated_at
		FROM uploads
		WHERE tmdb_id = ? AND type = ? AND season = ? AND episode = ?
		ORDER BY created_at DESC
		LIMIT 1
		`
		args = []interface{}{tmdbID, mediaType, season, episode}
	}

	row := d.db.QueryRow(query, args...)

	var upload Upload
	var createdAt, updatedAt string
	var dbSeason, dbEpisode sql.NullInt64
	var linksJSON string

	err := row.Scan(
		&upload.ID,
		&upload.TmdbID,
		&upload.Title,
		&upload.Type,
		&dbSeason,
		&dbEpisode,
		&upload.FilePath,
		&linksJSON,
		&upload.UploadStatus,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("erreur lors de la vérification d'un upload existant: %w", err)
	}

	// Convertir les valeurs nullables
	if dbSeason.Valid {
		seasonVal := int(dbSeason.Int64)
		upload.Season = &seasonVal
	}
	if dbEpisode.Valid {
		episodeVal := int(dbEpisode.Int64)
		upload.Episode = &episodeVal
	}

	// Décoder les liens
	if err := json.Unmarshal([]byte(linksJSON), &upload.Links); err != nil {
		upload.Links = []HostedLink{}
	}

	// Convertir les dates
	upload.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	upload.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &upload, nil
}
package storage

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Types de médias
const (
	TypeMovie  = "movie"
	TypeSeries = "series"
)

// Statuts d'upload
const (
	StatusPending   = "pending"
	StatusUploading = "uploading"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Database représente une connexion à la base de données
type Database struct {
	db *sql.DB
}

// Upload représente un upload en cours ou terminé
type Upload struct {
	ID           int64
	TmdbID       int
	Title        string
	Type         string
	Season       *int
	Episode      *int
	FilePath     string
	UploadStatus string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HostedLink représente un lien vers un fichier hébergé
type HostedLink struct {
	ID        int64
	UploadID  int64
	Hoster    string
	FileCode  string
	URL       string
	Embed     string
	CreatedAt time.Time
}

// NewDatabase crée une nouvelle instance de la base de données
func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Activer les foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return nil, err
	}

	database := &Database{
		db: db,
	}

	// Initialiser les tables
	if err := database.InitTables(); err != nil {
		return nil, err
	}

	// Initialiser la table de queue
	if err := database.InitQueueTable(); err != nil {
		return nil, err
	}

	return database, nil
}

// Close ferme la connexion à la base de données
func (db *Database) Close() error {
	return db.db.Close()
}

// InitTables initialise les tables de la base de données
func (db *Database) InitTables() error {
	// Table des uploads
	_, err := db.db.Exec(`
		CREATE TABLE IF NOT EXISTS uploads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tmdb_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			type TEXT NOT NULL,
			season INTEGER,
			episode INTEGER,
			file_path TEXT NOT NULL,
			upload_status TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Table des liens hébergés
	_, err = db.db.Exec(`
		CREATE TABLE IF NOT EXISTS hosted_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			upload_id INTEGER NOT NULL,
			hoster TEXT NOT NULL,
			file_code TEXT NOT NULL,
			url TEXT NOT NULL,
			embed TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (upload_id) REFERENCES uploads (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

// AddUpload ajoute un nouvel upload à la base de données
func (db *Database) AddUpload(upload *Upload) (int64, error) {
	query := `
		INSERT INTO uploads (
			tmdb_id, title, type, season, episode, file_path, upload_status, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	result, err := db.db.Exec(
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
		return 0, err
	}

	return result.LastInsertId()
}

// GetUpload récupère un upload par son ID
func (db *Database) GetUpload(id int64) (*Upload, error) {
	query := `
		SELECT id, tmdb_id, title, type, season, episode, file_path, upload_status, created_at, updated_at
		FROM uploads
		WHERE id = ?
	`

	var upload Upload
	var createdAt, updatedAt string
	var season, episode sql.NullInt64

	err := db.db.QueryRow(query, id).Scan(
		&upload.ID,
		&upload.TmdbID,
		&upload.Title,
		&upload.Type,
		&season,
		&episode,
		&upload.FilePath,
		&upload.UploadStatus,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if season.Valid {
		s := int(season.Int64)
		upload.Season = &s
	}

	if episode.Valid {
		e := int(episode.Int64)
		upload.Episode = &e
	}

	upload.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	upload.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &upload, nil
}

// UpdateUploadStatus met à jour le statut d'un upload
func (db *Database) UpdateUploadStatus(id int64, status string) error {
	query := `
		UPDATE uploads
		SET upload_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.db.Exec(query, status, id)
	return err
}

// AddUploadLink ajoute un lien hébergé à un upload
func (db *Database) AddUploadLink(uploadID int64, link HostedLink) error {
	query := `
		INSERT INTO hosted_links (
			upload_id, hoster, file_code, url, embed
		) VALUES (?, ?, ?, ?, ?)
	`

	_, err := db.db.Exec(
		query,
		uploadID,
		link.Hoster,
		link.FileCode,
		link.URL,
		link.Embed,
	)
	return err
}

// GetUploadLinks récupère tous les liens hébergés pour un upload
func (db *Database) GetUploadLinks(uploadID int64) ([]HostedLink, error) {
	query := `
		SELECT id, upload_id, hoster, file_code, url, embed, created_at
		FROM hosted_links
		WHERE upload_id = ?
		ORDER BY created_at DESC
	`

	rows, err := db.db.Query(query, uploadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []HostedLink
	for rows.Next() {
		var link HostedLink
		var createdAt string

		err := rows.Scan(
			&link.ID,
			&link.UploadID,
			&link.Hoster,
			&link.FileCode,
			&link.URL,
			&link.Embed,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		link.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		links = append(links, link)
	}

	return links, nil
}

// CheckExistingUpload vérifie si un média a déjà été uploadé
func (db *Database) CheckExistingUpload(tmdbID int, mediaType string, season, episode *int) (*Upload, error) {
	var query string
	var args []interface{}

	if mediaType == TypeMovie {
		query = `
			SELECT id, tmdb_id, title, type, season, episode, file_path, upload_status, created_at, updated_at
			FROM uploads
			WHERE tmdb_id = ? AND type = ?
			ORDER BY created_at DESC
			LIMIT 1
		`
		args = []interface{}{tmdbID, mediaType}
	} else if mediaType == TypeSeries && season != nil && episode != nil {
		query = `
			SELECT id, tmdb_id, title, type, season, episode, file_path, upload_status, created_at, updated_at
			FROM uploads
			WHERE tmdb_id = ? AND type = ? AND season = ? AND episode = ?
			ORDER BY created_at DESC
			LIMIT 1
		`
		args = []interface{}{tmdbID, mediaType, *season, *episode}
	} else {
		return nil, fmt.Errorf("type de média ou paramètres invalides")
	}

	var upload Upload
	var createdAt, updatedAt string
	var dbSeason, dbEpisode sql.NullInt64

	err := db.db.QueryRow(query, args...).Scan(
		&upload.ID,
		&upload.TmdbID,
		&upload.Title,
		&upload.Type,
		&dbSeason,
		&dbEpisode,
		&upload.FilePath,
		&upload.UploadStatus,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if dbSeason.Valid {
		s := int(dbSeason.Int64)
		upload.Season = &s
	}

	if dbEpisode.Valid {
		e := int(dbEpisode.Int64)
		upload.Episode = &e
	}

	upload.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	upload.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return &upload, nil
}

// GetPendingUploads récupère tous les uploads en attente
func (db *Database) GetPendingUploads() ([]*Upload, error) {
	query := `
		SELECT id, tmdb_id, title, type, season, episode, file_path, upload_status, created_at, updated_at
		FROM uploads
		WHERE upload_status = ?
		ORDER BY created_at ASC
	`

	rows, err := db.db.Query(query, StatusPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var uploads []*Upload
	for rows.Next() {
		var upload Upload
		var createdAt, updatedAt string
		var season, episode sql.NullInt64

		err := rows.Scan(
			&upload.ID,
			&upload.TmdbID,
			&upload.Title,
			&upload.Type,
			&season,
			&episode,
			&upload.FilePath,
			&upload.UploadStatus,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		if season.Valid {
			s := int(season.Int64)
			upload.Season = &s
		}

		if episode.Valid {
			e := int(episode.Int64)
			upload.Episode = &e
		}

		upload.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		upload.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		uploads = append(uploads, &upload)
	}

	return uploads, nil
}

// InitQueueTable initialise la table de queue dans la base de données
func (db *Database) InitQueueTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		payload TEXT NOT NULL,
		status TEXT NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		processed_at TIMESTAMP
	)`

	_, err := db.db.Exec(query)
	return err
}

// QueueStatus représente l'état d'une tâche dans la queue
type QueueStatus string

const (
	QueueStatusPending    QueueStatus = "pending"
	QueueStatusProcessing QueueStatus = "processing"
	QueueStatusCompleted  QueueStatus = "completed"
	QueueStatusFailed     QueueStatus = "failed"
)

// QueueItem représente une tâche dans la queue
type QueueItem struct {
	ID          int64       `json:"id"`
	Type        string      `json:"type"`
	Payload     string      `json:"payload"`
	Status      QueueStatus `json:"status"`
	Attempts    int         `json:"attempts"`
	MaxAttempts int         `json:"max_attempts"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	ProcessedAt *time.Time  `json:"processed_at"`
}

// TaskPayload représente les données d'une tâche
type TaskPayload struct {
	UploadID int64  `json:"upload_id"`
	TmdbID   int    `json:"tmdb_id"`
	Title    string `json:"title"`
	FilePath string `json:"file_path"`
	Season   *int   `json:"season,omitempty"`
	Episode  *int   `json:"episode,omitempty"`
}

// AddToQueue ajoute une tâche à la queue
func (db *Database) AddToQueue(taskType string, payload interface{}, maxAttempts int) (int64, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de la sérialisation du payload: %w", err)
	}

	query := `
	INSERT INTO queue (type, payload, status, max_attempts, updated_at)
	VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	result, err := db.db.Exec(query, taskType, string(payloadJSON), QueueStatusPending, maxAttempts)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de l'ajout à la queue: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("erreur lors de la récupération de l'ID: %w", err)
	}

	return id, nil
}

// GetNextQueueItem récupère la prochaine tâche à traiter
func (db *Database) GetNextQueueItem() (*QueueItem, error) {
	query := `
	SELECT id, type, payload, status, attempts, max_attempts, created_at, updated_at, processed_at
	FROM queue
	WHERE status = ? OR (status = ? AND attempts < max_attempts)
	ORDER BY created_at ASC
	LIMIT 1
	`

	var item QueueItem
	var createdAt, updatedAt string
	var processedAt sql.NullString

	err := db.db.QueryRow(query, QueueStatusPending, QueueStatusFailed).Scan(
		&item.ID,
		&item.Type,
		&item.Payload,
		&item.Status,
		&item.Attempts,
		&item.MaxAttempts,
		&createdAt,
		&updatedAt,
		&processedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération de la tâche: %w", err)
	}

	item.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	item.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	if processedAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", processedAt.String)
		item.ProcessedAt = &t
	}

	return &item, nil
}

// UpdateQueueItemStatus met à jour le statut d'une tâche
func (db *Database) UpdateQueueItemStatus(id int64, status QueueStatus) error {
	query := `
	UPDATE queue
	SET status = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := db.db.Exec(query, status, id)
	if err != nil {
		return fmt.Errorf("erreur lors de la mise à jour du statut: %w", err)
	}

	return nil
}

// MarkQueueItemProcessing marque une tâche comme étant en cours de traitement
func (db *Database) MarkQueueItemProcessing(id int64) error {
	query := `
	UPDATE queue
	SET status = ?, attempts = attempts + 1, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := db.db.Exec(query, QueueStatusProcessing, id)
	if err != nil {
		return fmt.Errorf("erreur lors du marquage de la tâche: %w", err)
	}

	return nil
}

// MarkQueueItemCompleted marque une tâche comme terminée
func (db *Database) MarkQueueItemCompleted(id int64) error {
	query := `
	UPDATE queue
	SET status = ?, processed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := db.db.Exec(query, QueueStatusCompleted, id)
	if err != nil {
		return fmt.Errorf("erreur lors du marquage de la tâche: %w", err)
	}

	return nil
}

// MarkQueueItemFailed marque une tâche comme échouée
func (db *Database) MarkQueueItemFailed(id int64) error {
	query := `
	UPDATE queue
	SET status = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?
	`

	_, err := db.db.Exec(query, QueueStatusFailed, id)
	if err != nil {
		return fmt.Errorf("erreur lors du marquage de la tâche: %w", err)
	}

	return nil
}

// GetPendingQueueItems récupère toutes les tâches en attente ou en cours
func (db *Database) GetPendingQueueItems() ([]*QueueItem, error) {
	query := `
	SELECT id, type, payload, status, attempts, max_attempts, created_at, updated_at, processed_at
	FROM queue
	WHERE status IN (?, ?, ?)
	ORDER BY created_at ASC
	`

	rows, err := db.db.Query(query, QueueStatusPending, QueueStatusProcessing, QueueStatusFailed)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la récupération des tâches: %w", err)
	}
	defer rows.Close()

	var items []*QueueItem

	for rows.Next() {
		var item QueueItem
		var createdAt, updatedAt string
		var processedAt sql.NullString

		err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Payload,
			&item.Status,
			&item.Attempts,
			&item.MaxAttempts,
			&createdAt,
			&updatedAt,
			&processedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("erreur lors de la lecture de la tâche: %w", err)
		}

		item.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		item.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

		if processedAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", processedAt.String)
			item.ProcessedAt = &t
		}

		items = append(items, &item)
	}

	return items, nil
}

// ResetStuckQueueItems réinitialise les tâches bloquées en traitement
func (db *Database) ResetStuckQueueItems() (int, error) {
	// Considère comme "bloquées" les tâches en traitement depuis plus de 30 minutes
	thirtyMinutesAgo := time.Now().Add(-30 * time.Minute).Format("2006-01-02 15:04:05")

	query := `
	UPDATE queue
	SET status = ?
	WHERE status = ? AND updated_at < ?
	`

	result, err := db.db.Exec(query, QueueStatusPending, QueueStatusProcessing, thirtyMinutesAgo)
	if err != nil {
		return 0, fmt.Errorf("erreur lors de la réinitialisation des tâches bloquées: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("erreur lors du comptage des tâches réinitialisées: %w", err)
	}

	return int(count), nil
}

// CleanupOldCompletedItems supprime les tâches terminées anciennes
func (db *Database) CleanupOldCompletedItems(days int) (int, error) {
	cutoffDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	query := `
	DELETE FROM queue
	WHERE status = ? AND processed_at < ?
	`

	result, err := db.db.Exec(query, QueueStatusCompleted, cutoffDate)
	if err != nil {
		return 0, fmt.Errorf("erreur lors du nettoyage des tâches anciennes: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("erreur lors du comptage des tâches supprimées: %w", err)
	}

	return int(count), nil
}

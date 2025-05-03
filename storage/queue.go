package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

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

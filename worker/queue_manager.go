package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"media-upload-system/storage"
)

// QueueManager gère la queue de tâches
type QueueManager struct {
	db            *storage.Database
	pool          *Pool
	isRunning     bool
	stopChan      chan struct{}
	wg            sync.WaitGroup
	pollInterval  time.Duration
	cleanupPeriod time.Duration
	handlers      map[string]func(payload []byte) error
}

// NewQueueManager crée un nouveau gestionnaire de queue
func NewQueueManager(db *storage.Database, pool *Pool) *QueueManager {
	return &QueueManager{
		db:            db,
		pool:          pool,
		isRunning:     false,
		stopChan:      make(chan struct{}),
		pollInterval:  5 * time.Second,
		cleanupPeriod: 1 * time.Hour,
		handlers:      make(map[string]func(payload []byte) error),
	}
}

// RegisterHandler enregistre un gestionnaire pour un type de tâche
func (qm *QueueManager) RegisterHandler(taskType string, handler func(payload []byte) error) {
	qm.handlers[taskType] = handler
}

// Start démarre le gestionnaire de queue
func (qm *QueueManager) Start() {
	if qm.isRunning {
		return
	}

	qm.isRunning = true
	qm.wg.Add(1)

	// Réinitialiser les tâches bloquées au démarrage
	count, err := qm.db.ResetStuckQueueItems()
	if err != nil {
		log.Printf("Erreur lors de la réinitialisation des tâches bloquées: %v", err)
	} else if count > 0 {
		log.Printf("Réinitialisation de %d tâches bloquées", count)
	}

	// Démarrer la boucle principale
	go qm.processLoop()

	// Démarrer la boucle de nettoyage
	go qm.cleanupLoop()

	log.Printf("Gestionnaire de queue démarré")
}

// Stop arrête le gestionnaire de queue
func (qm *QueueManager) Stop() {
	if !qm.isRunning {
		return
	}

	qm.isRunning = false
	close(qm.stopChan)
	qm.wg.Wait()
	log.Printf("Gestionnaire de queue arrêté")
}

// AddTask ajoute une tâche à la queue
func (qm *QueueManager) AddTask(taskType string, payload interface{}, maxAttempts int) (int64, error) {
	return qm.db.AddToQueue(taskType, payload, maxAttempts)
}

// processLoop est la boucle principale de traitement des tâches
func (qm *QueueManager) processLoop() {
	defer qm.wg.Done()

	ticker := time.NewTicker(qm.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-qm.stopChan:
			return
		case <-ticker.C:
			qm.processNextTask()
		}
	}
}

// cleanupLoop est la boucle de nettoyage des tâches anciennes
func (qm *QueueManager) cleanupLoop() {
	ticker := time.NewTicker(qm.cleanupPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-qm.stopChan:
			return
		case <-ticker.C:
			// Nettoyer les tâches terminées de plus de 7 jours
			count, err := qm.db.CleanupOldCompletedItems(7)
			if err != nil {
				log.Printf("Erreur lors du nettoyage des tâches anciennes: %v", err)
			} else if count > 0 {
				log.Printf("Nettoyage de %d tâches anciennes", count)
			}
		}
	}
}

// processNextTask traite la prochaine tâche dans la queue
func (qm *QueueManager) processNextTask() {
	item, err := qm.db.GetNextQueueItem()
	if err != nil {
		log.Printf("Erreur lors de la récupération de la prochaine tâche: %v", err)
		return
	}

	if item == nil {
		// Aucune tâche à traiter
		return
	}

	// Marquer la tâche comme étant en cours de traitement
	if err := qm.db.MarkQueueItemProcessing(item.ID); err != nil {
		log.Printf("Erreur lors du marquage de la tâche %d: %v", item.ID, err)
		return
	}

	log.Printf("Traitement de la tâche %d de type %s (tentative %d/%d)",
		item.ID, item.Type, item.Attempts, item.MaxAttempts)

	// Trouver le gestionnaire pour ce type de tâche
	handler, exists := qm.handlers[item.Type]
	if !exists {
		log.Printf("Aucun gestionnaire trouvé pour le type de tâche %s", item.Type)
		qm.db.MarkQueueItemFailed(item.ID)
		return
	}

	// Exécuter le gestionnaire dans le pool de workers
	qm.pool.AddTask(func() error {
		err := handler([]byte(item.Payload))
		if err != nil {
			log.Printf("Erreur lors du traitement de la tâche %d: %v", item.ID, err)
			qm.db.MarkQueueItemFailed(item.ID)
			return err
		}

		// Marquer la tâche comme terminée
		if err := qm.db.MarkQueueItemCompleted(item.ID); err != nil {
			log.Printf("Erreur lors du marquage de la tâche %d comme terminée: %v", item.ID, err)
			return err
		}

		log.Printf("Tâche %d terminée avec succès", item.ID)
		return nil
	})
}

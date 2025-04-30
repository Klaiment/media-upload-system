package worker

import (
	"log"
	"sync"
)

// Task représente une tâche à exécuter
type Task func() error

// Pool représente un pool de workers
type Pool struct {
	tasks        chan Task
	wg           sync.WaitGroup
	maxWorkers   int
	activeWorkers int
	mu           sync.Mutex
}

// NewPool crée un nouveau pool de workers
func NewPool(maxWorkers int) *Pool {
	return &Pool{
		tasks:      make(chan Task, 100), // Buffer de 100 tâches
		maxWorkers: maxWorkers,
	}
}

// Start démarre le pool de workers
func (p *Pool) Start() {
	log.Printf("Démarrage du pool de workers avec %d workers maximum", p.maxWorkers)
	
	// Démarrer les workers
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop arrête le pool de workers
func (p *Pool) Stop() {
	close(p.tasks)
	p.wg.Wait()
	log.Printf("Pool de workers arrêté")
}

// AddTask ajoute une tâche au pool
func (p *Pool) AddTask(task Task) {
	p.tasks <- task
}

// GetActiveWorkers retourne le nombre de workers actifs
func (p *Pool) GetActiveWorkers() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activeWorkers
}

// worker est la goroutine qui exécute les tâches
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	
	log.Printf("Worker %d démarré", id)
	
	for task := range p.tasks {
		// Incrémenter le compteur de workers actifs
		p.mu.Lock()
		p.activeWorkers++
		p.mu.Unlock()
		
		log.Printf("Worker %d: démarrage d'une tâche (workers actifs: %d/%d)", id, p.GetActiveWorkers(), p.maxWorkers)
		
		// Exécuter la tâche
		err := task()
		if err != nil {
			log.Printf("Worker %d: erreur lors de l'exécution de la tâche: %v", id, err)
		} else {
			log.Printf("Worker %d: tâche terminée avec succès", id)
		}
		
		// Décrémenter le compteur de workers actifs
		p.mu.Lock()
		p.activeWorkers--
		p.mu.Unlock()
	}
	
	log.Printf("Worker %d arrêté", id)
}
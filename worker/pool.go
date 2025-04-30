package worker

import (
	"log"
	"sync"
)

// Task représente une tâche à exécuter
type Task func() error

// Pool représente un pool de workers
type Pool struct {
	tasks       chan Task
	wg          sync.WaitGroup
	maxWorkers  int
	activeCount int
	mutex       sync.Mutex
}

// NewPool crée un nouveau pool de workers
func NewPool(maxWorkers int) *Pool {
	return &Pool{
		tasks:      make(chan Task, 100),
		maxWorkers: maxWorkers,
	}
}

// Start démarre les workers
func (p *Pool) Start() {
	log.Printf("Démarrage du pool de workers avec %d workers maximum", p.maxWorkers)

	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop arrête les workers
func (p *Pool) Stop() {
	close(p.tasks)
	p.wg.Wait()
	log.Printf("Pool de workers arrêté")
}

// AddTask ajoute une tâche au pool
func (p *Pool) AddTask(task Task) {
	p.tasks <- task
}

// AddTaskWithCallback ajoute une tâche au pool avec un callback
func (p *Pool) AddTaskWithCallback(task Task, callback func(error)) {
	p.tasks <- func() error {
		err := task()
		callback(err)
		return err
	}
}

// RunParallelTasks exécute plusieurs tâches en parallèle et attend qu'elles soient toutes terminées
func (p *Pool) RunParallelTasks(tasks []Task) []error {
	var wg sync.WaitGroup
	errors := make([]error, len(tasks))

	for i, task := range tasks {
		wg.Add(1)

		// Capture les variables pour la goroutine
		taskIndex := i
		taskFunc := task

		p.AddTaskWithCallback(taskFunc, func(err error) {
			errors[taskIndex] = err
			wg.Done()
		})
	}

	wg.Wait()
	return errors
}

// GetActiveCount retourne le nombre de workers actifs
func (p *Pool) GetActiveCount() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.activeCount
}

// worker représente un worker
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	log.Printf("Worker %d démarré", id)

	for task := range p.tasks {
		p.mutex.Lock()
		p.activeCount++
		log.Printf("Worker %d: démarrage d'une tâche (workers actifs: %d/%d)", id, p.activeCount, p.maxWorkers)
		p.mutex.Unlock()

		err := task()

		p.mutex.Lock()
		p.activeCount--
		p.mutex.Unlock()

		if err != nil {
			log.Printf("Worker %d: erreur lors de l'exécution de la tâche: %v", id, err)
		} else {
			log.Printf("Worker %d: tâche terminée avec succès", id)
		}
	}

	log.Printf("Worker %d arrêté", id)
}

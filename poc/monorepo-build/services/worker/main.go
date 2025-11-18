package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"monorepo-build/shared/models"
	"monorepo-build/shared/utils"
)

const (
	DefaultWorkers      = 4
	DefaultQueueSize    = 100
	ShutdownTimeout     = 30 * time.Second
	HealthCheckInterval = 10 * time.Second
)

// WorkerPool manages background job processing
type WorkerPool struct {
	ctx        context.Context
	cancel     context.CancelFunc
	jobs       chan Job
	results    chan JobResult
	workers    []*Worker
	numWorkers int
	wg         sync.WaitGroup
	stats      *Statistics
}

// Job represents a unit of work
type Job struct {
	ID        string
	Type      string
	Data      interface{}
	Priority  int
	CreatedAt time.Time
}

// JobResult represents the outcome of job processing
type JobResult struct {
	JobID     string
	Success   bool
	Error     error
	Duration  time.Duration
	Timestamp time.Time
}

// Statistics tracks worker pool metrics
type Statistics struct {
	mu              sync.RWMutex
	TotalProcessed  int64
	TotalSucceeded  int64
	TotalFailed     int64
	AverageDuration time.Duration
	StartTime       time.Time
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(numWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		ctx:        ctx,
		cancel:     cancel,
		jobs:       make(chan Job, DefaultQueueSize),
		results:    make(chan JobResult, DefaultQueueSize),
		numWorkers: numWorkers,
		workers:    make([]*Worker, 0, numWorkers),
		stats: &Statistics{
			StartTime: time.Now(),
		},
	}
}

// Start initializes and starts all workers
func (wp *WorkerPool) Start() {
	log.Printf("Starting worker pool with %d workers", wp.numWorkers)

	for i := 0; i < wp.numWorkers; i++ {
		worker := NewWorker(i, wp.jobs, wp.results, wp.ctx)
		wp.workers = append(wp.workers, worker)
		wp.wg.Add(1)
		go func(w *Worker) {
			defer wp.wg.Done()
			w.Start()
		}(worker)
	}

	// Start result collector
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		wp.collectResults()
	}()

	// Start health monitor
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		wp.healthMonitor()
	}()
}

// Submit adds a job to the queue
func (wp *WorkerPool) Submit(job Job) error {
	select {
	case wp.jobs <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Shutdown gracefully stops the worker pool
func (wp *WorkerPool) Shutdown() error {
	log.Println("Shutting down worker pool...")
	close(wp.jobs)
	wp.cancel()

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Worker pool shut down gracefully")
		return nil
	case <-time.After(ShutdownTimeout):
		return fmt.Errorf("worker pool shutdown timed out")
	}
}

// collectResults processes job results
func (wp *WorkerPool) collectResults() {
	for result := range wp.results {
		wp.stats.mu.Lock()
		wp.stats.TotalProcessed++
		if result.Success {
			wp.stats.TotalSucceeded++
		} else {
			wp.stats.TotalFailed++
			log.Printf("Job %s failed: %v", result.JobID, result.Error)
		}
		// Update average duration
		wp.stats.AverageDuration = (wp.stats.AverageDuration*time.Duration(wp.stats.TotalProcessed-1) + result.Duration) / time.Duration(wp.stats.TotalProcessed)
		wp.stats.mu.Unlock()
	}
}

// healthMonitor periodically logs worker pool statistics
func (wp *WorkerPool) healthMonitor() {
	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wp.logStats()
		case <-wp.ctx.Done():
			return
		}
	}
}

// logStats logs current statistics
func (wp *WorkerPool) logStats() {
	wp.stats.mu.RLock()
	defer wp.stats.mu.RUnlock()

	uptime := time.Since(wp.stats.StartTime)
	log.Printf("Worker Stats - Processed: %d, Success: %d, Failed: %d, Avg Duration: %v, Uptime: %v",
		wp.stats.TotalProcessed,
		wp.stats.TotalSucceeded,
		wp.stats.TotalFailed,
		wp.stats.AverageDuration,
		uptime)
}

func main() {
	numWorkers := DefaultWorkers
	pool := NewWorkerPool(numWorkers)
	pool.Start()

	// Simulate job submission
	go func() {
		for i := 0; i < 20; i++ {
			user := &models.User{
				ID:        utils.GenerateID("user"),
				Email:     fmt.Sprintf("user%d@example.com", i),
				Name:      fmt.Sprintf("User %d", i),
				Role:      "member",
				CreatedAt: time.Now(),
			}

			job := Job{
				ID:        utils.GenerateID("job"),
				Type:      "process_user",
				Data:      user,
				Priority:  1,
				CreatedAt: time.Now(),
			}

			if err := pool.Submit(job); err != nil {
				log.Printf("Failed to submit job: %v", err)
			}

			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if err := pool.Shutdown(); err != nil {
		log.Fatalf("Worker pool shutdown error: %v", err)
	}

	pool.logStats()
	log.Println("Worker application exited")
}

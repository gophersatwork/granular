package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"monorepo-build/shared/models"
	"monorepo-build/shared/utils"
)

// Worker processes jobs from the queue
type Worker struct {
	id      int
	jobs    <-chan Job
	results chan<- JobResult
	ctx     context.Context
}

// NewWorker creates a new worker
func NewWorker(id int, jobs <-chan Job, results chan<- JobResult, ctx context.Context) *Worker {
	return &Worker{
		id:      id,
		jobs:    jobs,
		results: results,
		ctx:     ctx,
	}
}

// Start begins processing jobs
func (w *Worker) Start() {
	log.Printf("Worker %d started", w.id)
	for {
		select {
		case job, ok := <-w.jobs:
			if !ok {
				log.Printf("Worker %d stopped (channel closed)", w.id)
				return
			}
			w.processJob(job)
		case <-w.ctx.Done():
			log.Printf("Worker %d stopped (context cancelled)", w.id)
			return
		}
	}
}

// processJob handles a single job
func (w *Worker) processJob(job Job) {
	start := time.Now()
	result := JobResult{
		JobID:     job.ID,
		Timestamp: start,
	}

	log.Printf("Worker %d processing job %s (type: %s)", w.id, job.ID, job.Type)

	var err error
	switch job.Type {
	case "process_user":
		err = w.processUser(job)
	case "send_email":
		err = w.sendEmail(job)
	case "generate_report":
		err = w.generateReport(job)
	case "cleanup":
		err = w.cleanup(job)
	default:
		err = fmt.Errorf("unknown job type: %s", job.Type)
	}

	result.Duration = time.Since(start)
	result.Success = err == nil
	result.Error = err

	select {
	case w.results <- result:
	case <-w.ctx.Done():
		return
	}
}

// processUser handles user processing jobs
func (w *Worker) processUser(job Job) error {
	user, ok := job.Data.(*models.User)
	if !ok {
		return fmt.Errorf("invalid user data")
	}

	// Validate user
	if err := user.Validate(); err != nil {
		return fmt.Errorf("user validation failed: %w", err)
	}

	// Simulate complex processing
	time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

	// Update metadata
	user.UpdateMetadata("processed_by", fmt.Sprintf("worker_%d", w.id))
	user.UpdateMetadata("processed_at", time.Now().Format(time.RFC3339))
	user.UpdateMetadata("checksum", utils.ComputeHash(user.Email))

	// Simulate additional work
	for i := 0; i < 100; i++ {
		_ = utils.GenerateID("temp")
	}

	log.Printf("Worker %d processed user: %s", w.id, user.Email)
	return nil
}

// sendEmail handles email sending jobs
func (w *Worker) sendEmail(job Job) error {
	data, ok := job.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid email data")
	}

	to, _ := data["to"].(string)
	subject, _ := data["subject"].(string)
	body, _ := data["body"].(string)

	// Simulate email sending
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

	log.Printf("Worker %d sent email to %s: %s", w.id, to, subject)

	// Simulate some processing
	_ = utils.ComputeHash(body)

	return nil
}

// generateReport handles report generation jobs
func (w *Worker) generateReport(job Job) error {
	reportType, _ := job.Data.(string)

	// Simulate complex report generation
	time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)

	// Generate dummy data
	data := make(map[string]interface{})
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("metric_%d", i)
		data[key] = rand.Float64() * 100
	}

	jsonData, err := utils.FormatJSON(data)
	if err != nil {
		return err
	}

	log.Printf("Worker %d generated %s report (%d bytes)", w.id, reportType, len(jsonData))
	return nil
}

// cleanup handles cleanup jobs
func (w *Worker) cleanup(job Job) error {
	// Simulate cleanup operations
	time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)

	log.Printf("Worker %d performed cleanup", w.id)
	return nil
}

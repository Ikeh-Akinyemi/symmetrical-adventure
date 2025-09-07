package worker

import (
	"encoding/json"
	"errors"
	"gusto-webhook-guide/internal/models"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const maxRetries = 5
const retryDelay = 10 * time.Second

// Pool manages a pool of workers and a job queue.
type Pool struct {
	JobQueue         chan models.Job
	wg               sync.WaitGroup
	logger           *slog.Logger
	idempotencyStore *IdempotencyStore
}

// NewPool creates a new worker pool.
func NewPool(maxQueueSize, numWorkers int, logger *slog.Logger, store *IdempotencyStore) *Pool {
	return &Pool{
		JobQueue:         make(chan models.Job, maxQueueSize),
		logger:           logger,
		idempotencyStore: store,
	}
}

// Start launches the worker goroutines.
func (p *Pool) Start(numWorkers int) {
	for i := 1; i <= numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop waits for all workers to finish processing.
func (p *Pool) Stop() {
	p.logger.Info("Stopping worker pool... Closing job queue.")
	close(p.JobQueue) // Signal workers to stop by closing the channel.
	p.wg.Wait()
	p.logger.Info("All workers have stopped.")
}

// worker is the background goroutine that processes jobs from the queue.
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	p.logger.Info("Worker started", "worker_id", id)

	for job := range p.JobQueue {
		var event models.WebhookEvent // Corrected type
		if err := json.Unmarshal(job.Payload, &event); err != nil {
			p.logger.Error("Worker failed to unmarshal job payload", "worker_id", id, "error", err)
			continue // Discard unparseable job.
		}

		logger := p.logger.With("worker_id", id, "event_uuid", event.UUID, "attempt", job.Attempts+1)

		if p.idempotencyStore.Has(event.UUID) {
			logger.Warn("Duplicate webhook event detected and ignored")
			continue
		}

		err := p.processEvent(event)

		if err == nil {
			logger.Info("Event processed successfully")
			p.idempotencyStore.Set(event.UUID)
		} else {
			var permanentErr *ErrPermanent
			var transientErr *ErrTransient

			if errors.As(err, &permanentErr) {
				logger.Error("Event failed with permanent error, will not be retried", "error", err)
				p.idempotencyStore.Set(event.UUID)
			} else if errors.As(err, &transientErr) {
				job.Attempts++
				if job.Attempts < maxRetries {
					logger.Warn("Event failed with transient error, re-queuing for another attempt", "error", err, "delay", retryDelay)
					go func(j models.Job) { 
						time.Sleep(retryDelay)
						p.JobQueue <- j
					}(job)
				} else {
					logger.Error("CRITICAL: Job failed after max retries, moving to dead-letter queue (simulated)", "error", err)
					p.idempotencyStore.Set(event.UUID) // Mark as processed to prevent Gusto retries.
				}
			} else {
				logger.Error("Event failed with an unknown error", "error", err)
			}
		}
	}
}

// processEvent simulates making an API call back to Gusto and handling the structured error response.
func (p *Pool) processEvent(event models.WebhookEvent) error { // Corrected type
	p.logger.Info("Worker processing event", "event_uuid", event.UUID, "event_type", event.EventType)

	errorCategory := ""
	if strings.Contains(event.EventType, "company.updated") {
		errorCategory = "server_error"
	}
	if strings.Contains(event.EventType, "company.deleted") {
		errorCategory = "invalid_attribute_value"
	}

	if errorCategory != "" {
		err := errors.New("simulated Gusto API error")
		switch errorCategory {
		case "server_error", "rate_limit_error", "system_error":
			return &ErrTransient{Err: err}
		default:
			return &ErrPermanent{Err: err}
		}
	}

	return nil
}
package worker

import (
	"encoding/json"
	"errors"
	"gusto-webhook-guide/internal/webhooks"
	"log/slog"
	"strings"
	"sync"
)

// Pool manages a pool of workers and a job queue.
type Pool struct {
	JobQueue         chan []byte
	wg               sync.WaitGroup
	logger           *slog.Logger
	idempotencyStore *IdempotencyStore
}

// NewPool creates a new worker pool.
func NewPool(maxQueueSize, numWorkers int, logger *slog.Logger, store *IdempotencyStore) *Pool {
	return &Pool{
		JobQueue:         make(chan []byte, maxQueueSize),
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
	p.wg.Wait()       // Wait for all workers to exit.
	p.logger.Info("All workers have stopped.")
}

// worker is the background goroutine that processes jobs from the queue.
func (p *Pool) worker(id int) {
	defer p.wg.Done()
	p.logger.Info("Worker started", "worker_id", id)

	for jobPayload := range p.JobQueue {
		var event webhooks.WebhookEvent
		if err := json.Unmarshal(jobPayload, &event); err != nil {
			p.logger.Error("Worker failed to unmarshal job payload", "worker_id", id, "error", err)
			continue
		}

		logger := p.logger.With("worker_id", id, "event_uuid", event.UUID)

		// 1. Check for duplicates first.
		if p.idempotencyStore.Has(event.UUID) {
			logger.Warn("Duplicate webhook event detected and ignored")
			continue
		}

		// 2. Process the event and handle errors.
		err := p.processEvent(event)

		// 3. Decide whether to mark the event as processed based on the error type.
		if err == nil {
			logger.Info("Event processed successfully")
			p.idempotencyStore.Set(event.UUID)
		} else {
			var permanentErr *ErrPermanent
			var transientErr *ErrTransient

			if errors.As(err, &permanentErr) {
				logger.Error("Event failed with permanent error, will not be retried", "error", err)
				// Mark as processed to prevent Gusto's retries from causing repeated failures.
				p.idempotencyStore.Set(event.UUID)
			} else if errors.As(err, &transientErr) {
				logger.Warn("Event failed with transient error, will be retried by Gusto", "error", err)
				// DO NOT mark as processed. This allows Gusto's retry to be processed as a new event.
			} else {
				logger.Error("Event failed with an unknown error", "error", err)
				// To be safe, we treat unknown errors as transient and allow a retry.
			}
		}
	}
}

// processEvent simulates the actual work of handling a webhook event.
// It returns different error types based on the eventType to demonstrate handling.
func (p *Pool) processEvent(event webhooks.WebhookEvent) error {
	p.logger.Info("Worker processing event", "event_uuid", event.UUID, "event_type", event.EventType)

	// Simulate different outcomes based on event type for demonstration.
	if strings.Contains(event.EventType, "company--created") {
		// Simulate success
		return nil
	}

	if strings.Contains(event.EventType, "company--updated") {
		// Simulate a temporary, retryable error (e.g., downstream service is down).
		return &ErrTransient{Err: errors.New("failed to connect to downstream service")}
	}

	if strings.Contains(event.EventType, "company--deleted") {
		// Simulate a permanent, non-retryable error (e.g., invalid data).
		return &ErrPermanent{Err: errors.New("company has an invalid configuration")}
	}

	// Default to success for any other event types.
	return nil
}

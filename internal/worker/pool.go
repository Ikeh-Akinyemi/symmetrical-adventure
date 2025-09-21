package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"gusto-webhook-guide/internal/models"
	"io"
	"log/slog"
	"net/http"
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

// GustoAPIErrorResponse defines the structure of a Gusto API error.
type GustoAPIErrorResponse struct {
	Errors []struct {
		Category string `json:"category"`
		Message  string `json:"message"`
	} `json:"errors"`
}

// processEvent makes a real API call back to Gusto and handles the response.
func (p *Pool) processEvent(event models.WebhookEvent) error {
	p.logger.Info("Worker processing event", "event_uuid", event.UUID, "event_type", event.EventType)

	// We'll use the 'company.updated' event to trigger a real API call.
	if strings.Contains(event.EventType, "company.updated") {
		// 1. Get the company-specific access token.
		accessToken := "supply-access-token-here"

		// 2. Make an API call to get company details.
		companyURL := fmt.Sprintf("https://api.gusto-demo.com/v1/companies/%s", event.ResourceUUID)
		req, _ := http.NewRequest("GET", companyURL, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			// A client-side error (e.g., DNS, timeout) is a transient failure.
			return &ErrTransient{Err: fmt.Errorf("http client error: %w", err)}
		}
		defer resp.Body.Close()

		// 3. Handle the API response.
		if resp.StatusCode >= 400 {
			// This is an API error from Gusto. Parse the error response.
			bodyBytes, _ := io.ReadAll(resp.Body)
			var gustoError GustoAPIErrorResponse
			if err := json.Unmarshal(bodyBytes, &gustoError); err != nil {
				// If we can't parse the error, treat it as transient.
				return &ErrTransient{Err: fmt.Errorf("failed to parse Gusto error response: %w", err)}
			}

			if len(gustoError.Errors) > 0 {
				errorCategory := gustoError.Errors[0].Category
				apiErr := fmt.Errorf("Gusto API error: %s", gustoError.Errors[0].Message)

				// Use the 'category' from the JSON error to classify the failure.
				switch errorCategory {
				case "server_error", "rate_limit_error", "system_error":
					return &ErrTransient{Err: apiErr}
				default:
					// Treat all others (validation, auth, etc.) as permanent.
					return &ErrPermanent{Err: apiErr}
				}
			}
		}

		// If status code is 2xx, the API call was successful.
		p.logger.Info("Successfully fetched company details after webhook event.")
	}

	// For all other event types, we do nothing.
	return nil
}
package webhooks

import (
	"gusto-webhook-guide/internal/contextkeys"
	"log/slog"
	"net/http"
)

// Handler now includes a channel to send jobs to the worker pool.
type Handler struct {
	Logger   *slog.Logger
	JobQueue chan<- []byte // Write-only channel
}

// NewHandler is updated to accept the job queue channel.
func NewHandler(logger *slog.Logger, jobQueue chan<- []byte) *Handler {
	return &Handler{
		Logger:   logger,
		JobQueue: jobQueue,
	}
}

// HandleWebhook now queues the event for background processing.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Retrieve the raw request body from the context, which was placed there
	// by our security middleware.
	bodyBytes, ok := r.Context().Value(contextkeys.RequestBodyKey).([]byte)
	if !ok || len(bodyBytes) == 0 {
		h.Logger.Error("Could not retrieve request body from context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Send the job to the worker pool. This is a non-blocking operation
	// as long as the job queue channel is not full.
	select {
	case h.JobQueue <- bodyBytes:
		h.Logger.Info("Webhook event successfully queued for processing")
		// Respond immediately with 202 Accepted to signal receipt.
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Event accepted for processing.\n"))
	default:
		// This case is hit if the job queue is full.
		h.Logger.Error("Job queue is full. Rejecting webhook event.")
		http.Error(w, "Server busy. Please try again later.", http.StatusServiceUnavailable)
	}
}

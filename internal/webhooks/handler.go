package webhooks

import (
	"encoding/json"
	"gusto-webhook-guide/internal/contextkeys"
	"gusto-webhook-guide/internal/models"
	"log/slog"
	"net/http"
)

// Handler contains dependencies for the webhook HTTP handlers.
type Handler struct {
	Logger   *slog.Logger
	JobQueue chan<- models.Job // Corrected type
}

// NewHandler creates a new instance of the webhook Handler.
func NewHandler(logger *slog.Logger, jobQueue chan<- models.Job) *Handler {
	return &Handler{
		Logger:   logger,
		JobQueue: jobQueue,
	}
}

// HandleWebhook is the final, correct version that handles both verification and events.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	bodyBytes, ok := r.Context().Value(contextkeys.RequestBodyKey).([]byte)
	if !ok {
		h.Logger.Error("Could not retrieve request body from context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if token, isVerification := payload["verification_token"]; isVerification {
		h.Logger.Info("✅ Received verification payload from Gusto. Use the token and UUID from the logs to complete verification.",
			"verification_token", token,
			"webhook_subscription_uuid", payload["webhook_subscription_uuid"],
		)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Verification payload acknowledged.\n"))
		return
	}

	if _, isEvent := payload["event_type"]; isEvent {
		// Create a new job with 0 initial attempts.
		job := models.Job{
			Payload:  bodyBytes,
			Attempts: 0,
		}
		select {
		case h.JobQueue <- job:
			h.Logger.Info("Webhook event successfully queued for processing")
			w.WriteHeader(http.StatusAccepted)
		default:
			h.Logger.Error("Job queue is full. Rejecting webhook event.")
			http.Error(w, "Server busy.", http.StatusServiceUnavailable)
		}
		return
	}

	h.Logger.Warn("Received webhook with unknown payload format", "body", string(bodyBytes))
	http.Error(w, "Unknown request format", http.StatusBadRequest)
}
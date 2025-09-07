package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// Handler contains dependencies for the setup handler.
type Handler struct {
	Logger   *slog.Logger
	APIToken string
}

// HandleWebhookSetup now ONLY creates the webhook subscription.
func (h *Handler) HandleWebhookSetup(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		URL string `json:"webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	webhookURL := requestBody.URL
	if webhookURL == "" {
		http.Error(w, "webhook_url is required", http.StatusBadRequest)
		return
	}

	h.Logger.Info("Step 1: Kicking off webhook subscription creation...", "url", webhookURL)

	createURL := "https://api.gusto-demo.com/v1/webhook_subscriptions"
	createBody := fmt.Sprintf(`{"url": "%s", "subscription_types": ["Company"]}`, webhookURL)
	req, _ := http.NewRequest("POST", createURL, bytes.NewBufferString(createBody))
	req.Header.Set("Authorization", "Bearer "+h.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating subscription: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		msg := fmt.Sprintf("Failed to create subscription. Status: %s, Body: %s", resp.Status, string(bodyBytes))
		http.Error(w, msg, resp.StatusCode)
		return
	}

	var createResp struct {
		UUID string `json:"uuid"`
	}
	json.Unmarshal(bodyBytes, &createResp)

	h.Logger.Info("✅ Subscription created. Gusto is now sending the verification payload to your /webhooks endpoint. Check the logs below.", "uuid", createResp.UUID)
	fmt.Fprintf(w, "Subscription created with UUID: %s. Check your server logs for the verification token from Gusto.", createResp.UUID)
}
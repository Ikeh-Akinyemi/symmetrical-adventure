package webhooks

import "encoding/json"

// Resource represents the primary object related to the webhook event.
type Resource struct {
	Type string `json:"type"`
	UUID string `json:"uuid"`
}

// WebhookEvent represents the structure of an incoming webhook from Gusto.
// The Payload is kept as json.RawMessage because its structure varies
// depending on the eventType.
type WebhookEvent struct {
	UUID      string          `json:"uuid"`
	EventType string          `json:"eventType"`
	Resource  Resource        `json:"resource"`
	Payload   json.RawMessage `json:"payload"`
}

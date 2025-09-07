package models

import "encoding/json"

// WebhookEvent represents the structure of an incoming webhook from Gusto.
type WebhookEvent struct {
	UUID         string          `json:"uuid"`
	EventType    string          `json:"event_type"`
	ResourceType string          `json:"resource_type"`
	ResourceUUID string          `json:"resource_uuid"`
	EntityType   string          `json:"entity_type"`
	EntityUUID   string          `json:"entity_uuid"`
	Payload      json.RawMessage `json:"payload"`
}

// Job wraps the raw event payload and includes a retry counter.
type Job struct {
	Payload  []byte
	Attempts int
}
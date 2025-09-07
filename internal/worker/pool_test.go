package worker

import (
	"encoding/json"
	"gusto-webhook-guide/internal/models"
	"io"
	"log/slog"
	"testing"
)

func TestWorkerLogic(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	testCases := []struct {
		name                   string
		initialStoreState      map[string]bool
		jobPayload             models.WebhookEvent 
		expectedFinalStoreKeys []string
	}{
		{
			name:              "Success Case - Event processed and UUID stored",
			initialStoreState: map[string]bool{},
			jobPayload: models.WebhookEvent{
				UUID:      "success-uuid-123",
				EventType: "company.created",
			},
			expectedFinalStoreKeys: []string{"success-uuid-123"},
		},
		{
			name:              "Transient Error Case - Event fails, UUID is NOT stored",
			initialStoreState: map[string]bool{},
			jobPayload: models.WebhookEvent{
				UUID:      "transient-uuid-456",
				EventType: "company.updated",
			},
			expectedFinalStoreKeys: []string{},
		},
		{
			name:              "Permanent Error Case - Event fails, UUID IS stored",
			initialStoreState: map[string]bool{},
			jobPayload: models.WebhookEvent{
				UUID:      "permanent-uuid-789",
				EventType: "company.deleted",
			},
			expectedFinalStoreKeys: []string{"permanent-uuid-789"},
		},
		{
			name: "Duplicate Event Case - Event is ignored, store is unchanged",
			initialStoreState: map[string]bool{
				"duplicate-uuid-abc": true,
			},
			jobPayload: models.WebhookEvent{
				UUID:      "duplicate-uuid-abc",
				EventType: "company.created",
			},
			expectedFinalStoreKeys: []string{"duplicate-uuid-abc"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			idempotencyStore := NewIdempotencyStore()
			for key := range tc.initialStoreState {
				idempotencyStore.Set(key)
			}

			pool := NewPool(1, 1, logger, idempotencyStore)
			payloadBytes, _ := json.Marshal(tc.jobPayload)
			job := models.Job{Payload: payloadBytes, Attempts: 0}

			pool.Start(1)
			pool.JobQueue <- job
			close(pool.JobQueue)
			pool.wg.Wait()

			idempotencyStore.mu.Lock()
			defer idempotencyStore.mu.Unlock()

			if len(idempotencyStore.store) != len(tc.expectedFinalStoreKeys) {
				t.Errorf("incorrect number of keys in store: got %d, want %d", len(idempotencyStore.store), len(tc.expectedFinalStoreKeys))
			}

			for _, key := range tc.expectedFinalStoreKeys {
				if _, found := idempotencyStore.store[key]; !found {
					t.Errorf("expected key %q not found in store", key)
				}
			}
		})
	}

	t.Run("Failure - Unparseable JSON", func(t *testing.T) {
		idempotencyStore := NewIdempotencyStore()
		pool := NewPool(1, 1, logger, idempotencyStore)
		job := models.Job{Payload: []byte(`{"invalid-json`), Attempts: 0}

		pool.Start(1)
		pool.JobQueue <- job
		close(pool.JobQueue)
		pool.wg.Wait()

		if len(idempotencyStore.store) != 0 {
			t.Errorf("store should be empty after unparseable JSON, but has %d keys", len(idempotencyStore.store))
		}
	})
}
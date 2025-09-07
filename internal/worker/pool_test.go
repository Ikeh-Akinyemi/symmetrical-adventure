package worker

import (
	"encoding/json"
	"gusto-webhook-guide/internal/webhooks"
	"io"
	"log/slog"
	"testing"
)

func TestWorkerLogic(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	// Define test cases for each core scenario.
	testCases := []struct {
		name                   string
		initialStoreState      map[string]bool
		jobPayload             webhooks.WebhookEvent
		expectedFinalStoreKeys []string
	}{
		{
			name:              "Success Case - Event processed and UUID stored",
			initialStoreState: map[string]bool{},
			jobPayload: webhooks.WebhookEvent{
				UUID:      "success-uuid-123",
				EventType: "company.created", // This event type is configured to succeed.
			},
			expectedFinalStoreKeys: []string{"success-uuid-123"},
		},
		// {
		// 	name:              "Transient Error Case - Event fails, UUID is NOT stored",
		// 	initialStoreState: map[string]bool{},
		// 	jobPayload: webhooks.WebhookEvent{
		// 		UUID:      "transient-uuid-456",
		// 		EventType: "contractor.created", // This event type is configured to return a transient error.
		// 	},
		// 	expectedFinalStoreKeys: []string{}, // The store should remain empty, allowing a retry.
		// },
		{
			name:              "Permanent Error Case - Event fails, UUID IS stored",
			initialStoreState: map[string]bool{},
			jobPayload: webhooks.WebhookEvent{
				UUID:      "permanent-uuid-789",
				EventType: "company.deleted", // This event type is configured to return a permanent error.
			},
			expectedFinalStoreKeys: []string{"permanent-uuid-789"}, // The UUID is stored to prevent retries.
		},
		{
			name: "Duplicate Event Case - Event is ignored, store is unchanged",
			initialStoreState: map[string]bool{
				"duplicate-uuid-abc": true, // This UUID is already in the store.
			},
			jobPayload: webhooks.WebhookEvent{
				UUID:      "duplicate-uuid-abc",
				EventType: "company.created",
			},
			expectedFinalStoreKeys: []string{"duplicate-uuid-abc"}, // The store state should not change.
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 1. Setup
			idempotencyStore := NewIdempotencyStore()
			for key := range tc.initialStoreState {
				idempotencyStore.Set(key)
			}

			pool := NewPool(1, 1, logger, idempotencyStore)
			payloadBytes, _ := json.Marshal(tc.jobPayload)

			// 2. Execute
			pool.Start(1)        // Start one worker.
			pool.JobQueue <- payloadBytes // Send one job.
			close(pool.JobQueue) // Close the queue to make the worker exit after this job.
			pool.wg.Wait()       // Wait for the worker to finish.

			// 3. Assert
			// We check the internal state of the store for this test.
			idempotencyStore.mu.Lock()
			defer idempotencyStore.mu.Unlock()

			// Check if the number of keys is as expected.
			if len(idempotencyStore.store) != len(tc.expectedFinalStoreKeys) {
				t.Errorf("incorrect number of keys in store: got %d, want %d", len(idempotencyStore.store), len(tc.expectedFinalStoreKeys))
			}

			// Check if the expected keys are present.
			for _, key := range tc.expectedFinalStoreKeys {
				if _, found := idempotencyStore.store[key]; !found {
					t.Errorf("expected key %q not found in store", key)
				}
			}
		})
	}

	// Special case: test unparseable JSON
	t.Run("Failure - Unparseable JSON", func(t *testing.T) {
		idempotencyStore := NewIdempotencyStore()
		pool := NewPool(1, 1, logger, idempotencyStore)

		pool.Start(1)
		pool.JobQueue <- []byte(`{"invalid-json`)
		close(pool.JobQueue)
		pool.wg.Wait()

		// The store should remain empty.
		if len(idempotencyStore.store) != 0 {
			t.Errorf("store should be empty after unparseable JSON, but has %d keys", len(idempotencyStore.store))
		}
	})
}
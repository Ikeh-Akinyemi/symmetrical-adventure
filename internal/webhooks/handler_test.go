package webhooks

import (
	"bytes"
	"context"
	"gusto-webhook-guide/internal/contextkeys"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleWebhook(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil)) // Suppress logs during tests.

	// Define test cases for each logical path in the handler.
	testCases := []struct {
		name               string
		requestBody        []byte
		jobQueueCapacity   int // To test the "queue full" scenario.
		setBodyInContext   bool
		expectedStatusCode int
		expectJobQueued    bool
	}{
		{
			name:               "Success - Verification Payload",
			requestBody:        []byte(`{"verification_token": "abc", "webhook_subscription_uuid": "xyz"}`),
			jobQueueCapacity:   1,
			setBodyInContext:   true,
			expectedStatusCode: http.StatusOK,
			expectJobQueued:    false,
		},
		{
			name:               "Success - Event Payload",
			requestBody:        []byte(`{"event_type": "company.created", "uuid": "123"}`),
			jobQueueCapacity:   1,
			setBodyInContext:   true,
			expectedStatusCode: http.StatusAccepted,
			expectJobQueued:    true,
		},
		{
			name:               "Failure - Unknown Payload Format",
			requestBody:        []byte(`{"some_other_key": "some_value"}`),
			jobQueueCapacity:   1,
			setBodyInContext:   true,
			expectedStatusCode: http.StatusBadRequest,
			expectJobQueued:    false,
		},
		{
			name:               "Failure - Invalid JSON",
			requestBody:        []byte(`{"invalid-json`),
			jobQueueCapacity:   1,
			setBodyInContext:   true,
			expectedStatusCode: http.StatusBadRequest,
			expectJobQueued:    false,
		},
		{
			name:               "Failure - Job Queue Full",
			requestBody:        []byte(`{"event_type": "company.created", "uuid": "123"}`),
			jobQueueCapacity:   0, // A zero-capacity (unbuffered) channel will always be full.
			setBodyInContext:   true,
			expectedStatusCode: http.StatusServiceUnavailable,
			expectJobQueued:    false,
		},
		{
			name:               "Failure - Missing Body in Context",
			requestBody:        []byte(`{}`),
			jobQueueCapacity:   1,
			setBodyInContext:   false, // Simulate a middleware failure.
			expectedStatusCode: http.StatusInternalServerError,
			expectJobQueued:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a job queue with the specified capacity for the test.
			jobQueue := make(chan []byte, tc.jobQueueCapacity)
			handler := NewHandler(logger, jobQueue)

			// Create the request and response recorder.
			req := httptest.NewRequest("POST", "/webhooks", bytes.NewReader(tc.requestBody))
			rr := httptest.NewRecorder()

			// Set the body in the context if the test case requires it.
			if tc.setBodyInContext {
				ctx := context.WithValue(req.Context(), contextkeys.RequestBodyKey, tc.requestBody)
				req = req.WithContext(ctx)
			}

			// Call the handler.
			handler.HandleWebhook(rr, req)

			// Assert the status code.
			if status := rr.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}

			// Assert whether a job was queued or not.
			var jobWasQueued bool
			select {
			case <-jobQueue:
				jobWasQueued = true
			default:
				jobWasQueued = false
			}

			if jobWasQueued != tc.expectJobQueued {
				t.Errorf("job queuing expectation failed: got %v want %v", jobWasQueued, tc.expectJobQueued)
			}
		})
	}
}
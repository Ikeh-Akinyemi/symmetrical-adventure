package webhooks

import (
	"bytes"
	"context"
	"gusto-webhook-guide/internal/contextkeys"
	"gusto-webhook-guide/internal/models"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleWebhook(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	testCases := []struct {
		name               string
		requestBody        []byte
		jobQueueCapacity   int
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
			jobQueueCapacity:   0,
			setBodyInContext:   true,
			expectedStatusCode: http.StatusServiceUnavailable,
			expectJobQueued:    false,
		},
		{
			name:               "Failure - Missing Body in Context",
			requestBody:        []byte(`{}`),
			jobQueueCapacity:   1,
			setBodyInContext:   false,
			expectedStatusCode: http.StatusInternalServerError,
			expectJobQueued:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jobQueue := make(chan models.Job, tc.jobQueueCapacity)
			handler := NewHandler(logger, jobQueue)

			req := httptest.NewRequest("POST", "/webhooks", bytes.NewReader(tc.requestBody))
			rr := httptest.NewRecorder()

			if tc.setBodyInContext {
				ctx := context.WithValue(req.Context(), contextkeys.RequestBodyKey, tc.requestBody)
				req = req.WithContext(ctx)
			}

			handler.HandleWebhook(rr, req)

			if status := rr.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}

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
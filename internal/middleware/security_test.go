package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"gusto-webhook-guide/internal/contextkeys"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestVerifySignature uses a table-driven approach to test the middleware.
func TestVerifySignature(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil)) // Suppress logs during tests.
	const testPayload = `{"event":"test"}`

	testCases := []struct {
		name               string
		secret             string // The secret to initialize the middleware with.
		signatureHeader    string // The signature to send in the request header.
		expectedStatusCode int
		expectBodyInCtx    bool
	}{
		{
			name:               "Success - Valid Signature",
			secret:             "test-secret",
			signatureHeader:    calculateHmac("test-secret", testPayload),
			expectedStatusCode: http.StatusOK,
			expectBodyInCtx:    true,
		},
		{
			name:               "Failure - Invalid Signature",
			secret:             "test-secret",
			signatureHeader:    "invalid-signature",
			expectedStatusCode: http.StatusForbidden,
			expectBodyInCtx:    false,
		},
		{
			name:               "Failure - Missing Signature Header",
			secret:             "test-secret",
			signatureHeader:    "",
			expectedStatusCode: http.StatusForbidden,
			expectBodyInCtx:    false,
		},
		{
			name:               "Success - Setup Mode with Empty Secret",
			secret:             "", // The crucial test case for our setup flow.
			signatureHeader:    "any-value-is-ignored",
			expectedStatusCode: http.StatusOK,
			expectBodyInCtx:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a dummy next handler that will be called if the middleware passes.
			// This handler also checks if the request body was correctly passed in the context.
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.expectBodyInCtx {
					bodyFromCtx, ok := r.Context().Value(contextkeys.RequestBodyKey).([]byte)
					if !ok || string(bodyFromCtx) != testPayload {
						t.Errorf("request body not found or incorrect in context")
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				}
				w.WriteHeader(http.StatusOK)
			})

			// Create the request and response recorder.
			req := httptest.NewRequest("POST", "/webhooks", bytes.NewBufferString(testPayload))
			if tc.signatureHeader != "" {
				req.Header.Set("X-Gusto-Signature", tc.signatureHeader)
			}
			rr := httptest.NewRecorder()

			// Create the middleware handler to test.
			handlerToTest := VerifySignature(logger, tc.secret)(nextHandler)
			handlerToTest.ServeHTTP(rr, req)

			// Assert the final status code.
			if status := rr.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}
		})
	}
}

// calculateHmac is a helper function to generate a valid HMAC-SHA256 signature for testing.
func calculateHmac(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

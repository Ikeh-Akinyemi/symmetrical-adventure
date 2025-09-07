package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	// Setup a dummy handler that will be called if middleware succeeds
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Use a discard logger to keep test output clean
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	secret := "test-secret"

	// Define test cases
	testCases := []struct {
		name               string
		payload            string
		signatureHeader    string
		expectedStatusCode int
	}{
		{
			name:               "Valid Signature",
			payload:            `{"event":"test"}`,
			signatureHeader:    calculateHmac("test-secret", `{"event":"test"}`),
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Invalid Signature",
			payload:            `{"event":"test"}`,
			signatureHeader:    "invalid-signature",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "Missing Signature Header",
			payload:            `{"event":"test"}`,
			signatureHeader:    "",
			expectedStatusCode: http.StatusForbidden,
		},
		{
			name:               "Empty Body with Valid Signature",
			payload:            "",
			signatureHeader:    calculateHmac("test-secret", ""),
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a request with the test case's payload
			req := httptest.NewRequest("POST", "/webhooks", bytes.NewBufferString(tc.payload))
			if tc.signatureHeader != "" {
				req.Header.Set("X-Gusto-Signature", tc.signatureHeader)
			}

			// Create a ResponseRecorder to record the response
			rr := httptest.NewRecorder()

			// Create the middleware handler and serve the request
			handlerToTest := VerifySignature(logger, secret)(nextHandler)
			handlerToTest.ServeHTTP(rr, req)

			// Check if the status code is what we expect
			if status := rr.Code; status != tc.expectedStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatusCode)
			}
		})
	}
}

// Helper function to calculate the expected HMAC for test cases
func calculateHmac(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
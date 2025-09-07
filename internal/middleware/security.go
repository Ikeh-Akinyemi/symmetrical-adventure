package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"gusto-webhook-guide/internal/contextkeys"
	"io"
	"log/slog"
	"net/http"
)

// VerifySignature is a middleware to validate the X-Gusto-Signature header.
func VerifySignature(logger *slog.Logger, secret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				logger.Error("Failed to read request body", "error", err)
				http.Error(w, "Cannot read request body", http.StatusInternalServerError)
				return
			}
			r.Body.Close()

			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			ctx := context.WithValue(r.Context(), contextkeys.RequestBodyKey, bodyBytes)
			r = r.WithContext(ctx) // Update the request with the new context.

			// This is a special ping from Gusto during the webhook subscription process.
			// The docs say it may contain a specific value, but in practice, it often has
			// a signature that can't be verified yet. For setup, we need to allow this
			// initial request through without a full HMAC check if our secret isn't set yet.
			if secret == "" {
				logger.Warn("Signature verification is running with an empty secret. Allowing request for setup purposes.")
				next.ServeHTTP(w, r)
				return
			}

			gustoSignature := r.Header.Get("X-Gusto-Signature")
			if gustoSignature == "" {
				http.Error(w, "Missing X-Gusto-Signature header", http.StatusForbidden)
				return
			}

			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			expectedSignature := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(gustoSignature), []byte(expectedSignature)) {
				logger.Warn(
					"Invalid signature received",
					"received_signature", gustoSignature,
					"expected_signature", expectedSignature,
				)
				http.Error(w, "Invalid signature", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
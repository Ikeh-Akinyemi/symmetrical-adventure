package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"

	"gusto-webhook-guide/internal/contextkeys"
)

// VerifySignature is a Chi middleware to validate the X-Gusto-Signature header.
func VerifySignature(logger *slog.Logger, secret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the signature from the request header.
			gustoSignature := r.Header.Get("X-Gusto-Signature")
			if gustoSignature == "" {
				logger.Warn("Missing X-Gusto-Signature header")
				http.Error(w, "Missing X-Gusto-Signature header", http.StatusForbidden)
				return
			}

			// Read the entire request body.
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				logger.Error("Failed to read request body", "error", err)
				http.Error(w, "Cannot read request body", http.StatusInternalServerError)
				return
			}
			// It's important to close the original body.
			r.Body.Close()

			// Restore the body so the next handler can read it.
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// Compute the expected HMAC-SHA256 signature.
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write(bodyBytes)
			expectedSignature := hex.EncodeToString(mac.Sum(nil))

			// Compare the signatures in constant time to prevent timing attacks.
			if !hmac.Equal([]byte(gustoSignature), []byte(expectedSignature)) {
				logger.Warn(
					"Invalid signature received",
					"received_signature", gustoSignature,
					"expected_signature", expectedSignature,
				)
				http.Error(w, "Invalid signature", http.StatusForbidden)
				return
			}

			// If the signature is valid, proceed to the next handler.
			ctx := context.WithValue(r.Context(), contextkeys.RequestBodyKey, bodyBytes)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

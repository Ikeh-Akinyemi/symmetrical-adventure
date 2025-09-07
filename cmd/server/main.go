package main

import (
	"context"
	"errors"
	"gusto-webhook-guide/internal/middleware"
	"gusto-webhook-guide/internal/setup"
	"gusto-webhook-guide/internal/webhooks"
	"gusto-webhook-guide/internal/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Initialize a structured JSON logger.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Load environment variables from a .env file for local development.
	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, continuing with environment variables")
	}

	// Get server port from environment variables, with a default value.
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080" // Default port if not specified.
	}
	serverAddr := ":" + port

	// Read the API token needed for the setup endpoint.
	apiToken := os.Getenv("GUSTO_API_TOKEN")
	if apiToken == "" {
		logger.Warn("GUSTO_API_TOKEN not set. The /admin/setup-webhook endpoint will not work.")
	}

	// Read the verification token, which acts as our signing secret for incoming webhooks.
	verificationToken := os.Getenv("GUSTO_VERIFICATION_TOKEN")
	if verificationToken == "" {
		logger.Warn("GUSTO_VERIFICATION_TOKEN is not set. Webhook signature verification will fail.")
	}

	// Create the idempotency store.
	idempotencyStore := worker.NewIdempotencyStore()

	// Create and start the worker pool.
	const maxQueueSize = 100
	const numWorkers = 5
	workerPool := worker.NewPool(maxQueueSize, numWorkers, logger, idempotencyStore)
	workerPool.Start(numWorkers)

	// --- Router Setup ---
	router := chi.NewRouter()

	// --- Webhook Routes ---
	webhookHandler := webhooks.NewHandler(logger, workerPool.JobQueue)
	router.Route("/webhooks", func(r chi.Router) {
		r.Use(middleware.VerifySignature(logger, verificationToken))
		r.Post("/", webhookHandler.HandleWebhook)
	})

	// --- Admin Route for Setup ---
	setupHandler := &setup.Handler{
		Logger:   logger,
		APIToken: apiToken,
	}
	router.Post("/admin/setup-webhook", setupHandler.HandleWebhookSetup)

	// Create and configure the HTTP server.
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	// Start the server in a goroutine so it doesn't block.
	go func() {
		logger.Info("Server starting", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for an interrupt signal to gracefully shut down the server.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Server shutting down...")

	// Stop the worker pool and wait for jobs to finish.
	workerPool.Stop()

	// Create a context with a timeout to allow existing requests to finish.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt to gracefully shut down the server.
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited gracefully")
}
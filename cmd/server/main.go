package main

import (
	"context"
	"errors"
	"gusto-webhook-guide/internal/middleware"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found, continuing with environment variables")
	}

	// Read the signing secret from the environment.
	// This is a critical configuration; the application should not start without it.
	signingSecret := os.Getenv("GUSTO_SIGNING_SECRET")
	if signingSecret == "" {
		logger.Error("GUSTO_SIGNING_SECRET is not set. Application cannot start.")
		os.Exit(1)
	}

	// 1. Create the idempotency store.
	idempotencyStore := worker.NewIdempotencyStore()

	// 2. Create and start the worker pool.
	// We'll configure it with a queue size of 100 and 5 concurrent workers.
	const maxQueueSize = 100
	const numWorkers = 5
	workerPool := worker.NewPool(maxQueueSize, numWorkers, logger, idempotencyStore)
	workerPool.Start(numWorkers)

	// 3. Instantiate our webhook handler, passing it the worker pool's job queue.
	webhookHandler := webhooks.NewHandler(logger, workerPool.JobQueue)

	router := chi.NewRouter()
	router.Route("/webhooks", func(r chi.Router) {
		r.Use(middleware.VerifySignature(logger, signingSecret))
		r.Post("/", webhookHandler.HandleWebhook)
	})

	server := &http.Server{
		Addr:    ":" + os.Getenv("SERVER_PORT"),
		Handler: router,
	}

	go func() {
		logger.Info("Server starting", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Server shutting down...")

	// Stop the worker pool. This will block until all workers are done.
	workerPool.Stop()

	// Shut down the HTTP server.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exited gracefully")
}

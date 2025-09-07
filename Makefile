# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOLINT=golangci-lint
GORUN=$(GOCMD) run

# Binary name
BINARY_NAME=gusto-webhook-handler
BINARY_DIR=./bin

all: help

build: ## Compile the application binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/server/main.go

run: ## Run the application locally
	@echo "Starting the server..."
	$(GORUN) ./cmd/server/main.go

test: ## Run all unit tests
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

lint: ## Lint the codebase using golangci-lint
	@echo "Linting code..."
	@# Ensure golangci-lint is installed: https://golangci-lint.run/usage/install/
	$(GOLINT) run

clean: ## Remove build artifacts
	@echo "Cleaning up..."
	@rm -rf $(BINARY_DIR)

help: ## Display this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: all build run test lint clean help
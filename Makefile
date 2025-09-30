.PHONY: help build run test clean docker-build docker-up docker-down setup

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: ## Run initial setup script
	@bash setup.sh

build: ## Build the Go application
	@echo "Building application..."
	@go build -o bin/injection-tracker ./cmd/server

run: ## Run the application locally
	@echo "Running application..."
	@go run ./cmd/server/main.go

test: ## Run all tests
	@echo "Running all tests..."
	@go test -v -race -cover ./...

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	@go test -v -race -cover ./internal/auth ./internal/repository

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@go test -v -race -cover ./internal/handlers

test-security: ## Run security tests
	@echo "Running security tests..."
	@go test -v -race -cover -run TestSecurity ./...
	@go test -v -race -cover ./internal/middleware

test-inventory: ## Run critical inventory transaction tests
	@echo "Running inventory transaction tests..."
	@go test -v -race -cover ./internal/repository -run TestInventory

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-coverage-detailed: ## Run tests with detailed coverage by package
	@echo "Running tests with detailed coverage..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo ""
	@echo "Coverage report generated: coverage.html"

test-benchmark: ## Run benchmark tests
	@echo "Running benchmark tests..."
	@go test -bench=. -benchmem -run=^$$ ./...

test-all: ## Run all tests including security and benchmarks
	@echo "Running comprehensive test suite..."
	@go test -v -race -cover ./...
	@echo ""
	@echo "Running security tests..."
	@go test -v -race -run TestSecurity ./...
	@echo ""
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem -run=^$$ ./...
	@echo ""
	@echo "All tests completed!"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/ coverage.out coverage.html
	@go clean

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker-compose build

docker-up: ## Start application with Docker Compose
	@echo "Starting application..."
	@docker-compose up -d
	@echo "Application started!"
	@echo "Access at: http://localhost:8080"

docker-down: ## Stop application
	@echo "Stopping application..."
	@docker-compose down

docker-logs: ## View Docker logs
	@docker-compose logs -f

docker-rebuild: ## Rebuild and restart Docker containers
	@echo "Rebuilding containers..."
	@docker-compose down
	@docker-compose build --no-cache
	@docker-compose up -d

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

migrate: ## Run database migrations
	@echo "Running migrations..."
	@go run ./cmd/server/main.go migrate

backup: ## Create database backup
	@echo "Creating backup..."
	@mkdir -p backups
	@cp data/tracker.db backups/tracker-$(shell date +%Y%m%d-%H%M%S).db
	@echo "Backup created in backups/"

dev: ## Run in development mode with auto-reload (requires air)
	@echo "Starting development server..."
	@air

security-check: ## Run security checks
	@echo "Running security checks..."
	@gosec -quiet ./...

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/cosmtrek/air@latest
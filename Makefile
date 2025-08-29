# FastFill Backend Makefile

.PHONY: help run build test clean docker-build docker-run lint fmt deps

# Default target
help: ## Show this help message
	@echo "FastFill Backend - Available Commands:"
	@echo
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

# Development commands
run: ## Run the development server
	go run cmd/server/main.go

build: ## Build the production binary
	go build -o server cmd/server/main.go

build-prod: ## Build production binary with optimizations
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o server cmd/server/main.go

test: ## Run tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -cover ./...

clean: ## Clean build artifacts
	rm -f server
	go clean
	go mod tidy

# Code quality
fmt: ## Format code
	go fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

# Dependencies
deps: ## Download and verify dependencies
	go mod download
	go mod verify
	go mod tidy

deps-update: ## Update all dependencies
	go get -u ./...
	go mod tidy

# Docker commands
docker-build: ## Build Docker image
	docker build -t fastfill-backend .

docker-run: ## Run with Docker
	docker run -p 8080:8080 --env-file .env fastfill-backend

# Database commands
db-migrate: ## Run database migrations (auto-migration on startup)
	@echo "Database migrations are handled automatically by GORM on server startup"

# Development helpers
dev: ## Run in development mode with live reload (requires air)
	air

install-tools: ## Install development tools
	go install github.com/cosmtrek/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
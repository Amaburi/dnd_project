.PHONY: all build test lint run migrate seed clean deps

# Variables
BINARY_NAME=dnd-campaign-manager
PORT=8080

# Build
build:
	go build -o $(BINARY_NAME) ./cmd/server

# Run development server.
# .env is sourced into the environment so config.yaml's ${MONGODB_URI},
# ${DEEPSEEK_API_KEY} and ${REDIS_PASSWORD} placeholders resolve.
run: build
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; ./$(BINARY_NAME)

# Run from source without building a binary
run-dev:
	@set -a; if [ -f .env ]; then . ./.env; fi; set +a; go run ./cmd/server

# Download dependencies
deps:
	go mod download
	go mod tidy

# Testing
test:
	go test -v ./...
	go test -v ./test/unit/...

# Unit tests only
test-unit:
	go test -v ./internal/... ./pkg/...

# Integration tests
test-integration:
	go test -v ./test/integration/...

# Code coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Linting
lint:
	golangci-lint run ./...

# Security check
security:
	go list -m all | nancy go.sum

# Database migrations
migrate:
	go run ./cmd/migrator/main.go up

# Seed data
seed:
	go run ./cmd/seed/main.go

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out

# Setup development environment
setup: deps
	cp configs/config.yaml config.yaml
	@echo "Edit config.yaml with your configuration"

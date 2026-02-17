.PHONY: all build test lint run migrate seed clean deps

# Variables
BINARY_NAME=dnd-campaign-manager
PORT=8080

# Build
build:
	go build -o $(BINARY_NAME) ./cmd/server

# Run development server
run: build
	CONFIG_PATH=./configs/config.yaml ./$(BINARY_NAME)

# Run with custom config
run-dev:
	CONFIG_PATH=./configs/config.yaml PORT=$(PORT) go run ./cmd/server

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

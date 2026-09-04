.PHONY: all build test test-unit test-integration test-all coverage coverage-all lint run run-dev demo clean deps setup security

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

# Unit tests. Offline: no database, no AI provider, nothing skipped.
test:
	go test ./...

# Integration tests against a real MongoDB.
#
# They sit behind a build tag rather than skipping themselves, so `make test`
# never quietly passes over them and this target either exercises a database or
# fails saying why. Point it at anything: a local mongod, a container someone
# else started, or a scratch Atlas cluster.
#
#   MONGODB_TEST_URI=mongodb://localhost:27017 make test-integration
MONGODB_TEST_URI ?=
test-integration:
	@test -n "$(MONGODB_TEST_URI)" || \
		(echo "MONGODB_TEST_URI is not set, e.g. MONGODB_TEST_URI=mongodb://localhost:27017 make test-integration"; exit 1)
	MONGODB_TEST_URI=$(MONGODB_TEST_URI) go test -tags=integration -count=1 ./...

# Everything, which is what CI should run.
test-all: test test-integration

# Code coverage for the offline suite.
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out

# Coverage including the integration tests, which is the number that reflects
# the handlers and repositories. -coverpkg attributes coverage to the packages
# being exercised rather than only the one holding the test.
coverage-all:
	@test -n "$(MONGODB_TEST_URI)" || \
		(echo "MONGODB_TEST_URI is not set"; exit 1)
	MONGODB_TEST_URI=$(MONGODB_TEST_URI) go test -tags=integration -count=1 \
		-coverpkg=./internal/... -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

# Linting
lint:
	golangci-lint run ./...

# Security check
security:
	go list -m all | nancy go.sum

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out

# Run the offline demo of one full turn: parse, resolve, narrate.
demo:
	go run ./examples

# Setup development environment
setup: deps
	cp .env.example .env
	@echo "Edit .env with your MONGODB_URI and GROQ_API_KEY"

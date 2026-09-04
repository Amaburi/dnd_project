# Project Structure & Dependencies

> **This document is aspirational.** It describes the intended layout, not what exists.
> Several directories below (`pkg/`, `internal/application/`, `test/`, `cmd/migrator`,
> `cmd/seed`) have not been created, and there is no `docker-compose.yml` in the repo.
> Check the tree before assuming a package is there. Redis and WebSocket entries were
> removed on 2026-09-04 — see ARCHITECTURE.md §0.

## Overview

This document outlines the recommended Go project structure for the AI D&D Campaign Manager, including package organization, dependencies, and build configuration.

---

## Directory Structure

```
dnd-project/
├── cmd/
│   └── server/
│       └── main.go                    # Application entry point
│
├── internal/
│   ├── api/
│   │   ├── server.go                  # HTTP server setup
│   │   ├── router.go                  # Route definitions
│   │   ├── handlers/
│   │   │   ├── campaign_handler.go
│   │   │   ├── character_handler.go
│   │   │   ├── session_handler.go
│   │   │   ├── combat_handler.go
│   │   │   └── ai_handler.go
│   │   └── middleware/
│   │       ├── logger.go
│   │       ├── recovery.go
│   │       └── rate_limiter.go
│   │
│   ├── application/
│   │   ├── campaign_service.go
│   │   ├── character_service.go
│   │   ├── session_service.go
│   │   ├── combat_service.go
│   │   └── ai_dm_service.go
│   │
│   ├── domain/
│   │   ├── models/
│   │   │   ├── campaign.go
│   │   │   ├── character.go
│   │   │   ├── session.go
│   │   │   ├── combat.go
│   │   │   └── dice.go
│   │   ├── services/
│   │   │   ├── dice_roller.go
│   │   │   ├── rules_engine.go
│   │   │   └── state_manager.go
│   │   └── events/
│   │       └── event_bus.go
│   │
│   ├── infrastructure/
│   │   ├── database/
│   │   │   ├── mongodb/
│   │   │   │   ├── client.go
│   │   │   │   ├── campaign_repo.go
│   │   │   │   ├── character_repo.go
│   │   │   │   ├── session_repo.go
│   │   │   │   └── combat_repo.go
│   │   │   └── migrations/
│   │   │       └── *.go
│   │   ├── ai/
│   │   │   ├── deepseek_client.go
│   │   │   ├── prompt_builder.go
│   │   │   └── context_manager.go
│   │   └── config/
│   │       └── config.go
│   │
│   └── utils/
│       ├── errors.go
│       ├── validation.go
│       ├── uuid.go
│       └── tokenizer.go
│
├── pkg/
│   ├── dice/
│   │   ├── roller.go
│   │   ├── parser.go
│   │   └── probability.go
│   │
│   ├── dnd5e/
│   │   ├── abilities.go
│   │   ├── skills.go
│   │   ├── classes.go
│   │   ├── races.go
│   │   └── spells.go
│   │
│   └── templates/
│       ├── system_prompts.go
│       └── npc_prompts.go
│
├── configs/
│   ├── config.yaml
│   └── Dockerfile
│
├── scripts/
│   ├── migrate.sh
│   ├── seed.sh
│   └── test.sh
│
├── docs/
│   ├── ARCHITECTURE.md
│   ├── DATA_MODELS.md
│   ├── API_DESIGN.md
│   ├── AI_INTEGRATION.md
│   ├── GAME_ENGINE.md
│   └── AI_CONTEXT_PROMPTS.md
│
├── test/
│   ├── unit/
│   ├── integration/
│   └── fixtures/
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Go Module Configuration

### go.mod

```go
module github.com/dnd-campaign/manager

go 1.21

require (
    // Web Framework
    github.com/gin-gonic/gin v1.9.1
    
    // MongoDB Driver
    go.mongodb.org/mongo-driver v1.13.1
    
    
    // Configuration
    github.com/spf13/viper v1.18.2
    
    // HTTP Client
    github.com/imroc/req/v3 v3.27.2
    
    // UUID
    github.com/google/uuid v1.5.0
    
    // Validation
    github.com/go-playground/validator/v10 v10.16.0
    
    // Logging
    github.com/rs/zerolog v1.31.0
    
    // Testing
    github.com/stretchr/testify v1.8.4
    github.com/testcontainers/testcontainers-go v0.26.0
)

require (
    // Development tools
    github.com/golangci/golangci-lint v1.55.2
    github.com/sonatype-nexus-community/nancy v1.0.1
)
```

---

## Dependencies Overview

### Core Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| **gin** | v1.9.1 | HTTP web framework |
| **mongo-driver** | v1.13.1 | MongoDB driver |
| **viper** | v1.18.2 | Configuration management |
| **google/uuid** | v1.5.0 | UUID generation |
| **zerolog** | v1.31.0 | Structured logging |
| **go-playground/validator** | v10.16.0 | Request validation |
| **testify** | v1.8.4 | Testing assertions |

### AI Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| **deepseek-sdk** | latest | DeepSeek API client |
| **anthropic-sdk** | latest | Optional: Claude API backup |

---

## Package Descriptions

### internal/api

**Purpose**: HTTP API layer handling incoming requests and responses

```
internal/api/
├── server.go          # Server initialization
├── router.go          # Route registration
├── handlers/          # Request handlers
│   ├── campaign.go   # Campaign CRUD
│   ├── character.go  # Character management
│   ├── session.go    # Session management
│   ├── combat.go     # Combat resolution
│   └── ai.go         # AI DM interactions
└── middleware/       # HTTP middleware
    ├── logger.go
    ├── recovery.go
    └── rate_limiter.go
```

### internal/application

**Purpose**: Business logic layer coordinating services

```
internal/application/
├── campaign_service.go   # Campaign business logic
├── character_service.go # Character management
├── session_service.go   # Session orchestration
├── combat_service.go    # Combat flow control
└── ai_dm_service.go     # AI DM orchestration
```

### internal/domain

**Purpose**: Core domain models and business rules

```
internal/domain/
├── models/       # Domain entities
├── services/    # Business rule implementations
└── events/      # Event handling
```

### internal/infrastructure

**Purpose**: External integrations and infrastructure

```
internal/infrastructure/
├── database/    # Data access layer
├── ai/         # AI service integration
└── config/     # Configuration loading
```

### pkg/dice

**Purpose**: Reusable dice rolling utilities

### pkg/dnd5e

**Purpose**: D&D 5e rules reference data

---

## Build Configuration

### Makefile

```makefile
.PHONY: all build test lint run migrate seed clean

# Variables
BINARY_NAME=dnd-campaign-manager
PORT=8080

# Build
build:
    go build -o $(BINARY_NAME) ./cmd/server

# Run
run: build
    ./$(BINARY_NAME)

# Run with custom port
run-dev:
    PORT=$(PORT) go run ./cmd/server

# Testing
test:
    go test -v ./...
    go test -v ./test/integration/...

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
```

---

## Configuration Management

### config.yaml

```yaml
# Application Configuration
app:
  host: "0.0.0.0"
  port: 8080
  environment: "development"
  debug: true

# MongoDB Configuration
mongodb:
  host: "localhost"
  port: 27017
  username: "dnd_user"
  password: "${MONGODB_PASSWORD}"
  database: "dnd_campaigns"
  auth_source: "admin"
  max_pool_size: 100
  min_pool_size: 10

# AI provider: any OpenAI-compatible /chat/completions endpoint
ai:
  provider: "groq"
  api_key: "${GROQ_API_KEY}"
  base_url: "https://api.groq.com/openai/v1"
  model: "llama-3.3-70b-versatile"
  timeout: "10s"
  max_retries: 3

# Dice Settings
dice:
  default_sides: 20
  enable_advantage: true
  enable_fudge: false

# Combat Settings
combat:
  auto initiative: true
  round_duration_seconds: 60
  max_turn_time_seconds: 120

# Logging
logging:
  level: "debug"
  format: "json"
  output: "stdout"

# Rate Limiting
rate_limit:
  requests_per_minute: 60
  burst: 10
  ai_requests_per_hour: 100
```

### Environment Variables

```bash
# Required
export GROQ_API_KEY="your-api-key"     # or DEEPSEEK_API_KEY / OPENAI_API_KEY / AI_API_KEY
export MONGODB_URI="mongodb+srv://..."

# Optional
export APP_ENV="production"
export PORT="8080"
```

---

## Dockerfile

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Production image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/pkg ./pkg

EXPOSE 8080

ENV PORT=8080
ENV APP_ENV=production

CMD ["./server"]
```

---

## Docker Compose

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - APP_ENV=development
      - MONGODB_HOST=mongodb
    depends_on:
      - mongodb
    volumes:
      - ./configs:/app/configs:ro

  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"
    environment:
      - MONGO_INITDB_DATABASE=dnd_campaigns
      - MONGO_INITDB_ROOT_USERNAME=admin
      - MONGO_INITDB_ROOT_PASSWORD=${MONGO_PASSWORD}
    volumes:
      - mongodb_data:/data/db
      - ./scripts/mongo-init.js:/docker-entrypoint-initdb.d/init.js:ro

volumes:
  mongodb_data:
```

---

## Testing Strategy

### Test Organization

```
test/
├── unit/                          # Unit tests
│   ├── dice/
│   │   ├── roller_test.go
│   │   └── parser_test.go
│   ├── rules/
│   │   ├── combat_test.go
│   │   └── ability_check_test.go
│   └── services/
│       └── ai_dm_test.go
│
├── integration/                   # Integration tests
│   ├── api/
│   │   ├── campaign_api_test.go
│   │   └── character_api_test.go
│   ├── database/
│   │   └── mongodb_test.go
│   └── ai/
│       └── deepseek_test.go
│
├── fixtures/                      # Test data
│   ├── campaigns/
│   ├── characters/
│   └── sessions/
│
└── testutils/                     # Test utilities
    ├── test_server.go
    ├── test_helpers.go
    └── test_data.go
```

### Unit Test Example

```go
package dice_test

import (
    "testing"
    "github.com/dnd-campaign/pkg/dice"
    "github.com/stretchr/testify/assert"
)

func TestRoller_Roll(t *testing.T) {
    roller := dice.NewRoller()
    
    t.Run("simple d20 roll", func(t *testing.T) {
        result := roller.Roll(dice.DiceExpression{
            Dice: []dice.Die{{Sides: 20, Quantity: 1}},
        })
        
        assert.Equal(t, 1, len(result.DiceResults))
        assert.InRange(t, result.DiceResults[0], 1, 20)
    })
    
    t.Run("roll with modifier", func(t *testing.T) {
        result := roller.Roll(dice.DiceExpression{
            Dice:     []dice.Die{{Sides: 20, Quantity: 1}},
            Modifier: 5,
        })
        
        assert.Equal(t, 5, result.Modifier)
        assert.Equal(t, result.DiceResults[0]+5, result.Total)
    })
}
```

### Integration Test Example

```go
package api_test

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/dnd-campaign/internal/api/handlers"
    "github.com/dnd-campaign/internal/infrastructure/database"
    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
)

func TestCampaignAPI_Create(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    // Setup
    db := database.NewTestDB(t)
    handler := handlers.NewCampaignHandler(db)
    router := gin.New()
    router.POST("/campaigns", handler.Create)
    
    // Test
    campaign := handlers.CreateCampaignRequest{
        Title:       "Test Campaign",
        Description: "A test campaign",
    }
    
    body, _ := json.Marshal(campaign)
    req := httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewBuffer(body))
    req.Header.Set("Content-Type", "application/json")
    
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    
    // Assert
    assert.Equal(t, http.StatusCreated, w.Code)
    
    var response handlers.CampaignResponse
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.NotEmpty(t, response.CampaignID)
    assert.Equal(t, "Test Campaign", response.Title)
}
```

---

## Continuous Integration

### GitHub Actions Workflow

```yaml
name: CI/CD

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
          
      - name: Cache Go modules
        uses: actions/cache@v4
        with:
          path: |
            ~/.cache/go-build
            ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          
      - name: Run Unit Tests
        run: make test-unit
        
      - name: Run Integration Tests
        run: make test-integration
        
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Run Linter
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
          args: --timeout 5m

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Security Audit
        run: make security
```

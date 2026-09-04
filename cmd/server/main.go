package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dnd-campaign/manager/internal/api"
	"github.com/dnd-campaign/manager/internal/api/handlers"
	"github.com/dnd-campaign/manager/internal/api/middleware"
	"github.com/dnd-campaign/manager/internal/application/memory"
	"github.com/dnd-campaign/manager/internal/application/turn"
	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/rules"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
	"github.com/dnd-campaign/manager/internal/infrastructure/config"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/rs/zerolog"
)

func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	logger.Info().Msg("Starting D&D Campaign Manager...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize MongoDB client
	mongoClient, err := mongodb.NewClient(mongodb.Config{
		URI:            cfg.MongoDB.URI,
		Database:       cfg.MongoDB.Database,
		Username:       cfg.MongoDB.Username,
		Password:       cfg.MongoDB.Password,
		AuthSource:     cfg.MongoDB.AuthSource,
		MaxPoolSize:    cfg.MongoDB.MaxPoolSize,
		MinPoolSize:    cfg.MongoDB.MinPoolSize,
		ConnectTimeout: cfg.MongoDB.ConnectTimeout,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to MongoDB")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Close(ctx); err != nil {
			logger.Error().Err(err).Msg("Error closing MongoDB connection")
		}
	}()

	logger.Info().Msg("Connected to MongoDB")

	// Initialize collections and indexes
	ctx := context.Background()
	if err := mongoClient.InitializeCollections(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize collections")
	}
	if err := mongoClient.CreateIndexes(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create indexes")
	}

	// Initialize repositories
	campaignRepo := mongodb.NewCampaignRepository(mongoClient)
	characterRepo := mongodb.NewCharacterRepository(mongoClient)
	monsterRepo := mongodb.NewMonsterRepository(mongoClient)
	sessionRepo := mongodb.NewSessionRepository(mongoClient)
	// The event repository writes through the session repository so a logged
	// roll or AI call also lands in the session's running totals.
	eventRepo := mongodb.NewStoryEventRepository(mongoClient, sessionRepo)
	encounterRepo := mongodb.NewEncounterRepository(mongoClient)

	// One roller for the process: seeded from the OS, shared by everything
	// that needs randomness.
	roller := dice.New()

	// The AI service and rules engine behind the turn endpoint. A missing key
	// is fatal here rather than a surprise on the first player action.
	aiService, err := ai.NewService(ai.ClientConfig{
		Provider:   cfg.AI.Provider,
		APIKey:     cfg.AI.APIKey,
		BaseURL:    cfg.AI.BaseURL,
		Model:      cfg.AI.Model,
		Timeout:    cfg.AI.Timeout,
		MaxRetries: cfg.AI.MaxRetries,
		Pricing: ai.Pricing{
			PromptUSDPerMillion:     cfg.AI.Pricing.PromptUSDPerMillion,
			CompletionUSDPerMillion: cfg.AI.Pricing.CompletionUSDPerMillion,
		},
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create the AI service")
	}
	defer aiService.Close()

	turnService := turn.NewService(
		characterRepo, monsterRepo, sessionRepo, eventRepo,
		aiService, rules.NewEngine(roller),
	)

	// The campaign's memory. NewService already gave the turn a budgeted view
	// of recent events; this replaces it with one that also carries the rolling
	// summary, so a campaign does not forget its first session once the log
	// outgrows a context window.
	campaignMemory := memory.New(eventRepo, campaignRepo, aiService)
	campaignMemory.Budget = memory.Budget{
		MaxTokens: cfg.Memory.MaxTokens,
		MinRecent: cfg.Memory.MinRecent,
	}
	campaignMemory.Window = cfg.Memory.Window
	campaignMemory.CompactAfter = cfg.Memory.CompactAfter
	campaignMemory.Retain = cfg.Memory.Retain
	turnService.Memory = campaignMemory
	turnService.Compactor = campaignMemory

	// Initialize handlers. Each needs the other's repository: campaigns cascade
	// their characters on delete, characters resolve their campaign by _id.
	campaignHandler := handlers.NewCampaignHandler(
		campaignRepo, characterRepo, monsterRepo, sessionRepo, eventRepo, encounterRepo)
	characterHandler := handlers.NewCharacterHandler(characterRepo, campaignRepo)
	monsterHandler := handlers.NewMonsterHandler(monsterRepo, campaignRepo)
	sessionHandler := handlers.NewSessionHandler(sessionRepo, eventRepo, campaignRepo)
	actionHandler := handlers.NewActionHandler(turnService, campaignRepo)
	combatHandler := handlers.NewCombatHandler(
		encounterRepo, characterRepo, monsterRepo, sessionRepo, campaignRepo, roller)
	diceHandler := handlers.NewDiceHandler(roller)

	// Create API server
	server := api.NewServer(api.ServerConfig{
		Host:         cfg.App.Host,
		Port:         cfg.App.Port,
		Debug:        cfg.App.Debug,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		Logger:       logger,
		CORS: middleware.CORSConfig{
			AllowedOrigins:   cfg.CORS.AllowedOrigins,
			AllowCredentials: cfg.CORS.AllowCredentials,
		},
		RateLimit: middleware.RateLimitConfig{
			RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			Burst:             cfg.RateLimit.Burst,
		},
	}, campaignHandler, characterHandler, monsterHandler, sessionHandler, actionHandler, combatHandler,
		diceHandler)

	// Start server in goroutine
	go func() {
		logger.Info().Str("host", cfg.App.Host).Int("port", cfg.App.Port).Msg("Starting HTTP server")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server exited")
}

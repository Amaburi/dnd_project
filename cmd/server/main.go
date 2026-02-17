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
		ConnectTimeout: 10 * time.Second,
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

	// Initialize handlers
	campaignHandler := handlers.NewCampaignHandler(campaignRepo)
	characterHandler := handlers.NewCharacterHandler(characterRepo)

	// Create API server
	server := api.NewServer(api.ServerConfig{
		Port:         cfg.App.Port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, campaignHandler, characterHandler)

	// Start server in goroutine
	go func() {
		logger.Info().Int("port", cfg.App.Port).Msg("Starting HTTP server")
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

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dnd-campaign/manager/internal/api/handlers"
	"github.com/gin-gonic/gin"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Host         string
	Port         int
	Debug        bool
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Server wraps the HTTP server
type Server struct {
	router           *gin.Engine
	server           *http.Server
	config           ServerConfig
	campaignHandler  *handlers.CampaignHandler
	characterHandler *handlers.CharacterHandler
}

// NewServer creates a new API server
func NewServer(cfg ServerConfig, campaignHandler *handlers.CampaignHandler, characterHandler *handlers.CharacterHandler) *Server {
	// Honour the configured environment instead of pinning release mode, so
	// app.debug actually changes anything.
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	srv := &Server{
		router: router,
		server: &http.Server{
			Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		config:           cfg,
		campaignHandler:  campaignHandler,
		characterHandler: characterHandler,
	}

	srv.setupMiddleware()
	srv.setupRoutes()

	return srv
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Add recovery middleware
	s.router.Use(gin.Recovery())

	// Add logger middleware (custom)
	s.router.Use(gin.Logger())
}

// setupRoutes configures API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().UTC(),
		})
	})

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Campaign routes
		v1.POST("/campaigns", s.campaignHandler.CreateCampaign)
		v1.GET("/campaigns", s.campaignHandler.ListCampaigns)
		v1.GET("/campaigns/:id", s.campaignHandler.GetCampaign)
		v1.PUT("/campaigns/:id", s.campaignHandler.UpdateCampaign)
		v1.DELETE("/campaigns/:id", s.campaignHandler.DeleteCampaign)

		// Character routes
		v1.POST("/campaigns/:id/characters", s.characterHandler.CreateCharacter)
		v1.GET("/campaigns/:id/characters", s.characterHandler.ListCharacters)
		v1.GET("/campaigns/:id/characters/:char_id", s.characterHandler.GetCharacter)
		v1.PUT("/campaigns/:id/characters/:char_id", s.characterHandler.UpdateCharacter)
		v1.DELETE("/campaigns/:id/characters/:char_id", s.characterHandler.DeleteCharacter)
	}
}

// Start begins serving requests
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

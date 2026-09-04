package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dnd-campaign/manager/internal/api/handlers"
	"github.com/dnd-campaign/manager/internal/api/middleware"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Host         string
	Port         int
	Debug        bool
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Logger receives one structured line per request. The zero value is a
	// silent logger, which is what tests want.
	Logger zerolog.Logger

	CORS      middleware.CORSConfig
	RateLimit middleware.RateLimitConfig
}

// Server wraps the HTTP server
type Server struct {
	router           *gin.Engine
	server           *http.Server
	config           ServerConfig
	campaignHandler  *handlers.CampaignHandler
	characterHandler *handlers.CharacterHandler
	monsterHandler   *handlers.MonsterHandler
	sessionHandler   *handlers.SessionHandler
	actionHandler    *handlers.ActionHandler
	combatHandler    *handlers.CombatHandler
	diceHandler      *handlers.DiceHandler
}

// NewServer creates a new API server
func NewServer(
	cfg ServerConfig,
	campaignHandler *handlers.CampaignHandler,
	characterHandler *handlers.CharacterHandler,
	monsterHandler *handlers.MonsterHandler,
	sessionHandler *handlers.SessionHandler,
	actionHandler *handlers.ActionHandler,
	combatHandler *handlers.CombatHandler,
	diceHandler *handlers.DiceHandler,
) *Server {
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
		monsterHandler:   monsterHandler,
		sessionHandler:   sessionHandler,
		actionHandler:    actionHandler,
		combatHandler:    combatHandler,
		diceHandler:      diceHandler,
	}

	srv.setupMiddleware()
	srv.setupRoutes()

	return srv
}

// setupMiddleware configures middleware.
//
// Order matters and is deliberate: the request id comes first so everything
// after it can log one, recovery wraps the handlers so a panic is still
// answered in JSON, and CORS sits ahead of the rate limiter so a browser
// preflight is answered rather than counted against the budget.
func (s *Server) setupMiddleware() {
	s.router.Use(
		middleware.RequestID(),
		middleware.Logger(s.config.Logger),
		middleware.Recovery(s.config.Logger),
		middleware.ErrorHandler(s.config.Logger),
		middleware.CORS(s.config.CORS),
		middleware.RateLimit(s.config.RateLimit),
	)
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

		// Monster statblock routes
		v1.POST("/campaigns/:id/monsters", s.monsterHandler.CreateMonster)
		v1.GET("/campaigns/:id/monsters", s.monsterHandler.ListMonsters)
		v1.POST("/campaigns/:id/monsters/seed", s.monsterHandler.SeedMonsters)
		v1.GET("/campaigns/:id/monsters/:monster_id", s.monsterHandler.GetMonster)
		v1.PUT("/campaigns/:id/monsters/:monster_id", s.monsterHandler.UpdateMonster)
		v1.DELETE("/campaigns/:id/monsters/:monster_id", s.monsterHandler.DeleteMonster)

		// Session routes. "active" is registered before ":session_id" is
		// reached by any request that could match both.
		v1.POST("/campaigns/:id/sessions", s.sessionHandler.CreateSession)
		v1.GET("/campaigns/:id/sessions", s.sessionHandler.ListSessions)
		v1.GET("/campaigns/:id/sessions/active", s.sessionHandler.GetActiveSession)
		v1.GET("/campaigns/:id/sessions/:session_id", s.sessionHandler.GetSession)
		v1.PUT("/campaigns/:id/sessions/:session_id", s.sessionHandler.UpdateSession)
		v1.DELETE("/campaigns/:id/sessions/:session_id", s.sessionHandler.DeleteSession)
		v1.POST("/campaigns/:id/sessions/:session_id/start", s.sessionHandler.StartSession)
		v1.POST("/campaigns/:id/sessions/:session_id/end", s.sessionHandler.EndSession)

		// Story events: the append-only log of what happened.
		v1.POST("/campaigns/:id/sessions/:session_id/events", s.sessionHandler.AppendEvent)
		v1.GET("/campaigns/:id/sessions/:session_id/events", s.sessionHandler.ListEvents)
		v1.GET("/campaigns/:id/events/recent", s.sessionHandler.RecentEvents)

		// One player action, end to end: parse, resolve, persist, narrate, log.
		v1.POST("/campaigns/:id/actions", s.actionHandler.TakeAction)

		// Combat encounters: initiative, turn order and the fight's own log.
		v1.POST("/campaigns/:id/encounters", s.combatHandler.CreateEncounter)
		v1.GET("/campaigns/:id/encounters", s.combatHandler.ListEncounters)
		v1.GET("/campaigns/:id/encounters/active", s.combatHandler.GetActiveEncounter)
		v1.GET("/campaigns/:id/encounters/:encounter_id", s.combatHandler.GetEncounter)
		v1.DELETE("/campaigns/:id/encounters/:encounter_id", s.combatHandler.DeleteEncounter)
		v1.GET("/campaigns/:id/encounters/:encounter_id/stats", s.combatHandler.EncounterStats)
		v1.POST("/campaigns/:id/encounters/:encounter_id/combatants", s.combatHandler.AddCombatant)
		v1.POST("/campaigns/:id/encounters/:encounter_id/initiative", s.combatHandler.RollInitiative)
		v1.POST("/campaigns/:id/encounters/:encounter_id/next-turn", s.combatHandler.NextTurn)
		v1.POST("/campaigns/:id/encounters/:encounter_id/end", s.combatHandler.EndEncounter)

		// Dice. Not campaign-scoped: a roll is not campaign state, and making
		// the client name a campaign to roll a d20 would be ceremony for
		// nothing. The probability routes roll nothing at all.
		v1.POST("/dice/roll", s.diceHandler.Roll)
		v1.POST("/dice/d20", s.diceHandler.RollD20)
		v1.POST("/dice/damage", s.diceHandler.RollDamage)
		v1.GET("/dice/probability", s.diceHandler.Probability)
		v1.POST("/dice/probability/check", s.diceHandler.CheckProbability)
		v1.POST("/dice/probability/attack", s.diceHandler.AttackProbability)
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

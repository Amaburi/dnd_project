package api

import (
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/api/handlers"
)

// Registering the route table is where a wildcard conflict shows up, and it
// panics at startup rather than failing a request -- so it needs a test that
// builds the real server.
//
// The specific risk here is "/campaigns/:id/sessions/active" sitting beside
// "/campaigns/:id/sessions/:session_id": a static segment and a parameter at
// the same position.
func TestRoutesRegisterWithoutConflict(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("registering routes panicked: %v", r)
		}
	}()

	srv := NewServer(
		ServerConfig{Host: "127.0.0.1", Port: 0},
		&handlers.CampaignHandler{},
		&handlers.CharacterHandler{},
		&handlers.MonsterHandler{},
		&handlers.SessionHandler{},
		&handlers.ActionHandler{},
		&handlers.CombatHandler{},
	)

	routes := srv.router.Routes()
	if len(routes) == 0 {
		t.Fatal("no routes registered")
	}

	registered := make(map[string]bool, len(routes))
	for _, r := range routes {
		registered[r.Method+" "+r.Path] = true
	}

	// Every route the UI depends on, named explicitly so a rename is caught.
	for _, want := range []string{
		"GET /health",

		"POST /api/v1/campaigns",
		"GET /api/v1/campaigns",
		"GET /api/v1/campaigns/:id",
		"PUT /api/v1/campaigns/:id",
		"DELETE /api/v1/campaigns/:id",

		"POST /api/v1/campaigns/:id/characters",
		"GET /api/v1/campaigns/:id/characters",
		"GET /api/v1/campaigns/:id/characters/:char_id",
		"PUT /api/v1/campaigns/:id/characters/:char_id",
		"DELETE /api/v1/campaigns/:id/characters/:char_id",

		"POST /api/v1/campaigns/:id/monsters",
		"GET /api/v1/campaigns/:id/monsters",
		"POST /api/v1/campaigns/:id/monsters/seed",
		"GET /api/v1/campaigns/:id/monsters/:monster_id",
		"PUT /api/v1/campaigns/:id/monsters/:monster_id",
		"DELETE /api/v1/campaigns/:id/monsters/:monster_id",

		"POST /api/v1/campaigns/:id/sessions",
		"GET /api/v1/campaigns/:id/sessions",
		"GET /api/v1/campaigns/:id/sessions/active",
		"GET /api/v1/campaigns/:id/sessions/:session_id",
		"PUT /api/v1/campaigns/:id/sessions/:session_id",
		"DELETE /api/v1/campaigns/:id/sessions/:session_id",
		"POST /api/v1/campaigns/:id/sessions/:session_id/start",
		"POST /api/v1/campaigns/:id/sessions/:session_id/end",

		"POST /api/v1/campaigns/:id/sessions/:session_id/events",
		"GET /api/v1/campaigns/:id/sessions/:session_id/events",
		"GET /api/v1/campaigns/:id/events/recent",

		"POST /api/v1/campaigns/:id/actions",

		"POST /api/v1/campaigns/:id/encounters",
		"GET /api/v1/campaigns/:id/encounters",
		"GET /api/v1/campaigns/:id/encounters/active",
		"GET /api/v1/campaigns/:id/encounters/:encounter_id",
		"DELETE /api/v1/campaigns/:id/encounters/:encounter_id",
		"GET /api/v1/campaigns/:id/encounters/:encounter_id/stats",
		"POST /api/v1/campaigns/:id/encounters/:encounter_id/combatants",
		"POST /api/v1/campaigns/:id/encounters/:encounter_id/initiative",
		"POST /api/v1/campaigns/:id/encounters/:encounter_id/next-turn",
		"POST /api/v1/campaigns/:id/encounters/:encounter_id/end",
	} {
		if !registered[want] {
			t.Errorf("route not registered: %s", want)
		}
	}
}

// Everything under the API lives beneath one version prefix, so a client can
// be pointed at a new one wholesale.
func TestEveryAPIRouteIsVersioned(t *testing.T) {
	srv := NewServer(
		ServerConfig{Host: "127.0.0.1", Port: 0},
		&handlers.CampaignHandler{},
		&handlers.CharacterHandler{},
		&handlers.MonsterHandler{},
		&handlers.SessionHandler{},
		&handlers.ActionHandler{},
		&handlers.CombatHandler{},
	)

	for _, r := range srv.router.Routes() {
		if r.Path == "/health" {
			continue
		}
		if !strings.HasPrefix(r.Path, "/api/v1/") {
			t.Errorf("route %s %s is outside the version prefix", r.Method, r.Path)
		}
	}
}

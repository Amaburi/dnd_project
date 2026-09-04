//go:build integration

// End-to-end tests over HTTP against a real MongoDB and a stubbed model.
//
// The handlers and repositories are the two packages a unit test cannot reach
// without a database, which is why their coverage sat near zero. These drive
// the actual routes so the wiring, the scoping, the status codes and the
// cascades are exercised the way a client meets them.
//
//	MONGODB_TEST_URI=mongodb://localhost:27017 go test -tags=integration ./...
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/dnd-campaign/manager/internal/api/handlers"
	"github.com/dnd-campaign/manager/internal/application/turn"
	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/rules"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type harness struct {
	server *Server
	stub   *ai.StubClient
}

// newHarness builds the real server against a scratch database and a stub
// model, so a test exercises every layer except the provider.
func newHarness(t *testing.T, replies ...string) *harness {
	t.Helper()

	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Fatal("MONGODB_TEST_URI is not set; integration tests need a database " +
			"(e.g. MONGODB_TEST_URI=mongodb://localhost:27017)")
	}

	client, err := mongodb.NewClient(mongodb.Config{
		URI: uri, Database: "dnd_http_test_" + primitive.NewObjectID().Hex(),
		MaxPoolSize: 10, MinPoolSize: 1, ConnectTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("connecting to %s: %v", uri, err)
	}

	ctx := context.Background()
	if err := client.InitializeCollections(ctx); err != nil {
		t.Fatalf("initialising collections: %v", err)
	}
	if err := client.CreateIndexes(ctx); err != nil {
		t.Fatalf("creating indexes: %v", err)
	}
	t.Cleanup(func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Database().Drop(dropCtx)
		_ = client.Close(dropCtx)
	})

	campaigns := mongodb.NewCampaignRepository(client)
	characters := mongodb.NewCharacterRepository(client)
	monsters := mongodb.NewMonsterRepository(client)
	sessions := mongodb.NewSessionRepository(client)
	events := mongodb.NewStoryEventRepository(client, sessions)
	encounters := mongodb.NewEncounterRepository(client)

	narrator, stub := ai.NewStubService(replies...)
	roller := dice.NewSeeded(1337)
	turns := turn.NewService(characters, monsters, sessions, events, narrator, rules.NewEngine(roller))

	server := NewServer(
		ServerConfig{Host: "127.0.0.1", Port: 0},
		handlers.NewCampaignHandler(campaigns, characters, monsters, sessions, events, encounters),
		handlers.NewCharacterHandler(characters, campaigns),
		handlers.NewMonsterHandler(monsters, campaigns),
		handlers.NewSessionHandler(sessions, events, campaigns),
		handlers.NewActionHandler(turns, campaigns),
		handlers.NewCombatHandler(encounters, characters, monsters, sessions, campaigns, roller),
	)

	return &harness{server: server, stub: stub}
}

// do runs a request through the router and decodes the response.
func (h *harness) do(t *testing.T, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding request: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	h.server.router.ServeHTTP(rec, req)

	decoded := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	}
	return rec, decoded
}

// doList is do for endpoints that return an array.
func (h *harness) doList(t *testing.T, method, path string) (*httptest.ResponseRecorder, []any) {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	h.server.router.ServeHTTP(rec, req)

	var decoded []any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	}
	return rec, decoded
}

func (h *harness) campaign(t *testing.T) string {
	t.Helper()
	rec, body := h.do(t, http.MethodPost, "/api/v1/campaigns", map[string]any{
		"title": "The Sunken Crypt", "created_by": "dm1",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating a campaign returned %d: %s", rec.Code, rec.Body)
	}
	return body["id"].(string)
}

func TestHealthCheck(t *testing.T) {
	h := newHarness(t)

	rec, body := h.do(t, http.MethodGet, "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body["status"] != "healthy" {
		t.Errorf("body = %v", body)
	}
}

// The status codes a client depends on: a caller's mistake is a 400, a missing
// document a 404, and an unexpected failure never leaks driver detail.
func TestErrorStatusCodes(t *testing.T) {
	h := newHarness(t)

	rec, _ := h.do(t, http.MethodPost, "/api/v1/campaigns", map[string]any{"title": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a campaign with no title returned %d, want 400", rec.Code)
	}

	rec, _ = h.do(t, http.MethodGet, "/api/v1/campaigns/not-an-object-id", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a malformed id returned %d, want 400", rec.Code)
	}

	rec, _ = h.do(t, http.MethodGet, "/api/v1/campaigns/"+primitive.NewObjectID().Hex(), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("a missing campaign returned %d, want 404", rec.Code)
	}

	rec, _ = h.doList(t, http.MethodGet, "/api/v1/campaigns")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("listing without user_id returned %d, want 400", rec.Code)
	}
}

func TestCampaignCRUDOverHTTP(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(t)

	rec, body := h.do(t, http.MethodGet, "/api/v1/campaigns/"+id, nil)
	if rec.Code != http.StatusOK || body["title"] != "The Sunken Crypt" {
		t.Fatalf("get returned %d: %v", rec.Code, body)
	}

	// A PUT carrying only the changed field must not blank the rest.
	rec, updated := h.do(t, http.MethodPut, "/api/v1/campaigns/"+id, map[string]any{"title": "Renamed"})
	if rec.Code != http.StatusOK {
		t.Fatalf("update returned %d: %s", rec.Code, rec.Body)
	}
	if updated["title"] != "Renamed" {
		t.Errorf("title = %v", updated["title"])
	}
	if updated["campaign_id"] == "" || updated["created_by"] != "dm1" {
		t.Errorf("the update blanked immutable fields: %v", updated)
	}

	rec, list := h.doList(t, http.MethodGet, "/api/v1/campaigns?user_id=dm1")
	if rec.Code != http.StatusOK || len(list) != 1 {
		t.Errorf("list returned %d with %d items", rec.Code, len(list))
	}

	rec, _ = h.do(t, http.MethodDelete, "/api/v1/campaigns/"+id, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete returned %d, want 204", rec.Code)
	}
}

func characterBody() map[string]any {
	return map[string]any{
		"name": "Thistle", "type": "player",
		"basic_info": map[string]any{
			"race": "halfling", "subrace": "lightfoot", "background": "criminal",
			"classes": []map[string]any{{"class": "rogue", "subclass": "thief", "level": 3}},
		},
		"ability_scores": map[string]any{
			"strength": 10, "dexterity": 17, "constitution": 14,
			"intelligence": 12, "wisdom": 13, "charisma": 11,
		},
		"skills": map[string]any{
			"deception": "proficient", "stealth": "expertise",
			"acrobatics": "proficient", "investigation": "proficient",
		},
	}
}

// A character must be invisible through another campaign's URL: that scoping
// is only observable through the routes.
func TestCharacterScopingOverHTTP(t *testing.T) {
	h := newHarness(t)
	one := h.campaign(t)

	rec, other := h.do(t, http.MethodPost, "/api/v1/campaigns", map[string]any{
		"title": "Elsewhere", "created_by": "dm1",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating the second campaign: %s", rec.Body)
	}
	otherID := other["id"].(string)

	rec, created := h.do(t, http.MethodPost, "/api/v1/campaigns/"+one+"/characters", characterBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating a character returned %d: %s", rec.Code, rec.Body)
	}
	charID := created["id"].(string)

	rec, _ = h.do(t, http.MethodGet, "/api/v1/campaigns/"+one+"/characters/"+charID, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("reading through its own campaign returned %d", rec.Code)
	}

	rec, _ = h.do(t, http.MethodGet, "/api/v1/campaigns/"+otherID+"/characters/"+charID, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("reading through an unrelated campaign returned %d, want 404", rec.Code)
	}
	rec, _ = h.do(t, http.MethodDelete, "/api/v1/campaigns/"+otherID+"/characters/"+charID, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("deleting through an unrelated campaign returned %d, want 404", rec.Code)
	}
}

// An illegal sheet is a 400 with an explanation, not a 500.
func TestCharacterValidationOverHTTP(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(t)

	body := characterBody()
	body["basic_info"].(map[string]any)["classes"] = []map[string]any{
		{"class": "rogue", "subclass": "champion", "level": 3},
	}

	rec, response := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/characters", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an illegal sheet returned %d, want 400: %s", rec.Code, rec.Body)
	}
	if response["error"] == nil {
		t.Error("no explanation was returned")
	}
}

func TestMonsterSeedAndFilterOverHTTP(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(t)

	rec, body := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/monsters/seed", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("seeding returned %d: %s", rec.Code, rec.Body)
	}
	if body["seeded"].(float64) < 10 {
		t.Errorf("seeded %v monsters", body["seeded"])
	}

	rec, all := h.doList(t, http.MethodGet, "/api/v1/campaigns/"+id+"/monsters")
	if rec.Code != http.StatusOK || len(all) < 10 {
		t.Fatalf("listing returned %d with %d monsters", rec.Code, len(all))
	}

	rec, low := h.doList(t, http.MethodGet, "/api/v1/campaigns/"+id+"/monsters?min_cr=0&max_cr=0.25")
	if rec.Code != http.StatusOK {
		t.Fatalf("filtering returned %d", rec.Code)
	}
	if len(low) == 0 || len(low) >= len(all) {
		t.Errorf("the CR filter returned %d of %d", len(low), len(all))
	}

	rec, _ = h.doList(t, http.MethodGet, "/api/v1/campaigns/"+id+"/monsters?min_cr=abc")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a malformed CR returned %d, want 400", rec.Code)
	}
}

// One session runs at a time, and the active route is what a client polls.
func TestSessionLifecycleOverHTTP(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(t)

	rec, _ := h.do(t, http.MethodGet, "/api/v1/campaigns/"+id+"/sessions/active", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("no session in progress returned %d, want 404", rec.Code)
	}

	rec, first := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions",
		map[string]any{"title": "Into the crypt"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating a session returned %d: %s", rec.Code, rec.Body)
	}
	if first["session_number"].(float64) != 1 {
		t.Errorf("session number = %v, want 1", first["session_number"])
	}
	sessionID := first["id"].(string)

	rec, _ = h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions/"+sessionID+"/start", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("starting returned %d: %s", rec.Code, rec.Body)
	}

	rec, active := h.do(t, http.MethodGet, "/api/v1/campaigns/"+id+"/sessions/active", nil)
	if rec.Code != http.StatusOK || active["id"] != sessionID {
		t.Errorf("active session = %d, %v", rec.Code, active["id"])
	}

	rec, _ = h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions/"+sessionID+"/end", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ending returned %d: %s", rec.Code, rec.Body)
	}
	rec, _ = h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions/"+sessionID+"/end", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("ending twice returned %d, want 400", rec.Code)
	}
}

// The whole point of the log: a turn is parsed, resolved, persisted, narrated
// and remembered, and the next turn can see it.
func TestActionTurnOverHTTP(t *testing.T) {
	h := newHarness(t,
		`{"action":"attack","target":"Goblin","weapon":"Rapier","confidence":"high"}`,
		"The rapier slides past the goblin's guard.",
	)
	id := h.campaign(t)

	// A character with a weapon it can actually swing.
	body := characterBody()
	rapier := map[string]any{
		"item_id": "w1", "key": "rapier", "name": "Rapier", "kind": "weapon",
		"weapon": map[string]any{
			"category": "martial", "damage_dice": "1d8", "damage_type": "piercing",
			"properties": []string{"finesse"},
		},
	}
	body["inventory"] = []map[string]any{rapier}
	body["equipment"] = map[string]any{"weapons": []map[string]any{rapier}}
	body["proficiencies"] = map[string]any{"weapons": []string{"simple", "rapier"}}

	rec, created := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/characters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating a character returned %d: %s", rec.Code, rec.Body)
	}
	characterID := created["character_id"].(string)

	if rec, _ := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/monsters/seed", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding monsters returned %d", rec.Code)
	}

	action := map[string]any{"character_id": characterID, "input": "I stab the goblin"}

	// Without a session there is nowhere to log the turn, so it is refused.
	rec, _ = h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/actions", action)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a turn with no session returned %d, want 400: %s", rec.Code, rec.Body)
	}

	rec, session := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions",
		map[string]any{"title": "Into the crypt"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating a session: %s", rec.Body)
	}
	sessionID := session["id"].(string)
	if rec, _ := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions/"+sessionID+"/start", nil); rec.Code != http.StatusOK {
		t.Fatalf("starting the session: %s", rec.Body)
	}

	rec, result := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/actions", action)
	if rec.Code != http.StatusOK {
		t.Fatalf("the turn returned %d: %s", rec.Code, rec.Body)
	}
	if result["narration"] == "" {
		t.Error("no narration was returned")
	}
	if result["attack"] == nil {
		t.Error("the attack was not resolved")
	}
	if result["event"] == nil {
		t.Fatal("the turn was not logged")
	}

	// And the log now feeds the next prompt.
	rec, recent := h.do(t, http.MethodGet, "/api/v1/campaigns/"+id+"/events/recent", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("recent events returned %d", rec.Code)
	}
	if recent["context"] == "nothing has happened yet" {
		t.Error("the turn did not reach the campaign's memory")
	}
}

// A campaign takes its children with it, or they become unreachable orphans.
func TestDeletingACampaignCascades(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(t)

	if rec, _ := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/characters", characterBody()); rec.Code != http.StatusCreated {
		t.Fatalf("creating a character: %d", rec.Code)
	}
	if rec, _ := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/monsters/seed", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding monsters: %d", rec.Code)
	}
	rec, session := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions",
		map[string]any{"title": "Session one"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating a session: %d", rec.Code)
	}
	sessionID := session["id"].(string)
	if rec, _ := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions/"+sessionID+"/events",
		map[string]any{"event_type": "narrative"}); rec.Code != http.StatusCreated {
		t.Fatalf("appending an event: %d", rec.Code)
	}

	if rec, _ := h.do(t, http.MethodDelete, "/api/v1/campaigns/"+id, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("deleting the campaign: %d", rec.Code)
	}

	// Everything hangs off the campaign, so every child route is now a 404.
	for _, path := range []string{
		"/api/v1/campaigns/" + id + "/characters",
		"/api/v1/campaigns/" + id + "/monsters",
		"/api/v1/campaigns/" + id + "/sessions",
	} {
		if rec, _ := h.doList(t, http.MethodGet, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s returned %d after the campaign was deleted, want 404", path, rec.Code)
		}
	}
}

// A whole fight through the routes: enrol, roll, advance, and end when decided.
func TestCombatEncounterOverHTTP(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(t)

	rec, character := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/characters", characterBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating a character: %s", rec.Body)
	}
	characterID := character["character_id"].(string)

	if rec, _ := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/monsters/seed", nil); rec.Code != http.StatusOK {
		t.Fatalf("seeding monsters: %d", rec.Code)
	}
	_, monsters := h.doList(t, http.MethodGet, "/api/v1/campaigns/"+id+"/monsters?q=goblin")
	if len(monsters) == 0 {
		t.Fatal("no goblin to fight")
	}
	monsterID := monsters[0].(map[string]any)["monster_id"].(string)

	rec, encounter := h.do(t, http.MethodPost, "/api/v1/campaigns/"+id+"/encounters",
		map[string]any{"encounter_name": "Cellar ambush"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating an encounter returned %d: %s", rec.Code, rec.Body)
	}
	encounterID := encounter["id"].(string)
	base := "/api/v1/campaigns/" + id + "/encounters/" + encounterID

	for _, body := range []map[string]any{
		{"character_id": characterID, "initiative": 18},
		{"monster_id": monsterID, "initiative": 9},
	} {
		if rec, _ := h.do(t, http.MethodPost, base+"/combatants", body); rec.Code != http.StatusOK {
			t.Fatalf("adding a combatant returned %d: %s", rec.Code, rec.Body)
		}
	}

	rec, rolled := h.do(t, http.MethodPost, base+"/initiative", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("rolling initiative returned %d: %s", rec.Code, rec.Body)
	}
	state := rolled["combat_state"].(map[string]any)
	if state["phase"] != "active" || state["round"].(float64) != 1 {
		t.Errorf("state after initiative = %v", state)
	}
	if rec, _ := h.do(t, http.MethodPost, base+"/initiative", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("rolling initiative twice returned %d, want 400", rec.Code)
	}

	rec, advanced := h.do(t, http.MethodPost, base+"/next-turn", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("advancing returned %d: %s", rec.Code, rec.Body)
	}
	if advanced["combat_state"].(map[string]any)["turn"].(float64) != 1 {
		t.Errorf("turn did not advance: %v", advanced["combat_state"])
	}

	rec, ended := h.do(t, http.MethodPost, base+"/end", map[string]any{"outcome": "victory"})
	if rec.Code != http.StatusOK {
		t.Fatalf("ending returned %d: %s", rec.Code, rec.Body)
	}
	if ended["status"] != "completed" {
		t.Errorf("status = %v, want completed", ended["status"])
	}

	rec, stats := h.do(t, http.MethodGet, base+"/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats returned %d", rec.Code)
	}
	if stats["outcome"] != "victory" {
		t.Errorf("stats outcome = %v", stats["outcome"])
	}
}

// Lists come back as [] rather than null, which a client would otherwise have
// to special-case on every screen.
func TestEmptyListsAreArrays(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(t)

	for _, path := range []string{
		"/api/v1/campaigns/" + id + "/characters",
		"/api/v1/campaigns/" + id + "/monsters",
		"/api/v1/campaigns/" + id + "/sessions",
		"/api/v1/campaigns/" + id + "/encounters",
	} {
		rec := httptest.NewRecorder()
		h.server.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned %d", path, rec.Code)
		}
		if body := rec.Body.String(); body != "[]" {
			t.Errorf("%s returned %q, want []", path, body)
		}
	}
}

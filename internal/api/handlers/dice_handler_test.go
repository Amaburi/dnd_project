package handlers

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/gin-gonic/gin"
)

// diceRouter wires the dice routes onto a scripted roller, so an assertion can
// be about a specific total rather than a range. NewScripted is what makes a
// dice endpoint testable at all: seeded is repeatable, scripted is chosen.
func diceRouter(faces ...int) *gin.Engine {
	roller := dice.NewSeeded(1)
	if len(faces) > 0 {
		roller = dice.NewScripted(faces...)
	}
	h := NewDiceHandler(roller)
	r := gin.New()
	r.POST("/dice/roll", h.Roll)
	r.POST("/dice/d20", h.RollD20)
	r.POST("/dice/damage", h.RollDamage)
	r.GET("/dice/probability", h.Probability)
	r.POST("/dice/probability/check", h.CheckProbability)
	r.POST("/dice/probability/attack", h.AttackProbability)
	return r
}

func call(t *testing.T, r *gin.Engine, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	decoded := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("%s %s returned unparseable JSON: %v\n%s", method, path, err, rec.Body.String())
		}
	}
	return rec, decoded
}

func TestRollReturnsEveryDie(t *testing.T) {
	r := diceRouter(3, 5) // a 2d6 that comes up 3 and 5
	rec, body := call(t, r, http.MethodPost, "/dice/roll", gin.H{"expression": "2d6+2"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got := body["total"].(float64); got != 10 {
		t.Errorf("total = %v, want 10", got)
	}
	// The individual dice must survive to the client: a log showing only the
	// total hides what actually happened at the table.
	rolls, ok := body["rolls"].([]any)
	if !ok || len(rolls) != 2 {
		t.Fatalf("rolls = %v, want two dice", body["rolls"])
	}
}

func TestRollRejectsNonsense(t *testing.T) {
	r := diceRouter()
	for _, expression := range []string{"", "sword", "1d1", "500d6", "2d6000"} {
		rec, body := call(t, r, http.MethodPost, "/dice/roll", gin.H{"expression": expression})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q returned %d, want 400", expression, rec.Code)
		}
		if body["error"] == nil {
			t.Errorf("%q returned no error message", expression)
		}
	}
}

// The d20 endpoint has to keep both dice under advantage, or the client cannot
// show the player the 19 they lost.
func TestD20KeepsBothDiceUnderAdvantage(t *testing.T) {
	r := diceRouter(19, 4)
	rec, body := call(t, r, http.MethodPost, "/dice/d20", gin.H{"modifier": 3, "mode": "advantage"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	roll := body["roll"].(map[string]any)
	if got := roll["natural"].(float64); got != 19 {
		t.Errorf("natural = %v, want the higher die, 19", got)
	}
	if got := roll["total"].(float64); got != 22 {
		t.Errorf("total = %v, want 22", got)
	}
	if rolls := roll["rolls"].([]any); len(rolls) != 2 {
		t.Errorf("rolls = %v, want both dice", rolls)
	}
}

// Passing a DC turns the roll into a resolved check, which is what a UI needs
// to colour the result without reimplementing the comparison.
func TestD20AgainstADCReportsTheOutcome(t *testing.T) {
	r := diceRouter(18)
	_, body := call(t, r, http.MethodPost, "/dice/d20", gin.H{"modifier": 2, "dc": 15})

	if body["outcome"] != "success" {
		t.Errorf("outcome = %v, want success", body["outcome"])
	}
	// DC 15 with +2 needs a natural 13: eight faces out of twenty.
	odds := body["odds"].(map[string]any)
	if odds["needs_natural"].(float64) != 13 {
		t.Errorf("needs_natural = %v, want 13", odds["needs_natural"])
	}
	if math.Abs(odds["success"].(float64)-8.0/20) > 1e-9 {
		t.Errorf("odds.success = %v, want 0.4", odds["success"])
	}
}

func TestD20WithoutADCOmitsTheOutcome(t *testing.T) {
	r := diceRouter(11)
	_, body := call(t, r, http.MethodPost, "/dice/d20", gin.H{"modifier": 2})

	if _, present := body["outcome"]; present {
		t.Errorf("outcome was reported without a DC: %v", body["outcome"])
	}
}

func TestD20RejectsAnUnknownMode(t *testing.T) {
	r := diceRouter()
	rec, _ := call(t, r, http.MethodPost, "/dice/d20", gin.H{"mode": "superadvantage"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// The critical rule most often played wrong: the dice double, the modifier
// does not.
func TestDamageCriticalDoublesOnlyTheDice(t *testing.T) {
	r := diceRouter(8, 8)
	_, body := call(t, r, http.MethodPost, "/dice/damage", gin.H{"expression": "1d8+3", "critical": true})

	if got := body["total"].(float64); got != 19 {
		t.Errorf("total = %v, want 19 (8+8+3), not 22", got)
	}
	if body["critical"] != true {
		t.Errorf("critical = %v, want true", body["critical"])
	}
}

func TestProbabilityOfAnExpression(t *testing.T) {
	r := diceRouter()
	rec, body := call(t, r, http.MethodGet, "/dice/probability?expression=2d6%2B3", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if body["min"].(float64) != 5 || body["max"].(float64) != 15 {
		t.Errorf("span %v-%v, want 5-15", body["min"], body["max"])
	}
	if math.Abs(body["mean"].(float64)-10) > 1e-9 {
		t.Errorf("mean = %v, want 10", body["mean"])
	}
	outcomes := body["outcomes"].([]any)
	if len(outcomes) != 11 {
		t.Fatalf("got %d outcomes, want 11", len(outcomes))
	}
	first := outcomes[0].(map[string]any)
	if math.Abs(first["at_least"].(float64)-1) > 1e-9 {
		t.Errorf("at_least on the lowest total = %v, want 1", first["at_least"])
	}
}

// A target lets the client ask the question it actually has, without walking
// the outcome list itself.
func TestProbabilityAgainstATarget(t *testing.T) {
	r := diceRouter()
	_, body := call(t, r, http.MethodGet, "/dice/probability?expression=3d6&target=10", nil)

	target := body["target"].(map[string]any)
	if math.Abs(target["at_least"].(float64)-135.0/216) > 1e-9 {
		t.Errorf("at_least = %v, want 135/216", target["at_least"])
	}
}

func TestProbabilityRejectsAnImpracticalExpression(t *testing.T) {
	r := diceRouter()
	rec, body := call(t, r, http.MethodGet, "/dice/probability?expression=100d1000", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if body["error"] == nil {
		t.Error("no explanation for the refusal")
	}

	missing, _ := call(t, r, http.MethodGet, "/dice/probability", nil)
	if missing.Code != http.StatusBadRequest {
		t.Errorf("a missing expression returned %d, want 400", missing.Code)
	}
}

func TestCheckProbabilityEndpoint(t *testing.T) {
	r := diceRouter()
	_, body := call(t, r, http.MethodPost, "/dice/probability/check",
		gin.H{"dc": 15, "modifier": 5, "mode": "advantage"})

	if math.Abs(body["success"].(float64)-(1-math.Pow(9.0/20, 2))) > 1e-9 {
		t.Errorf("success = %v, want 0.7975", body["success"])
	}
	if body["needs_natural"].(float64) != 10 {
		t.Errorf("needs_natural = %v, want 10", body["needs_natural"])
	}
}

func TestAttackProbabilityEndpoint(t *testing.T) {
	r := diceRouter()
	_, body := call(t, r, http.MethodPost, "/dice/probability/attack",
		gin.H{"target_ac": 15, "modifier": 7, "damage": "1d8+4"})

	if math.Abs(body["hit"].(float64)-13.0/20) > 1e-9 {
		t.Errorf("hit = %v, want 0.65", body["hit"])
	}
	if math.Abs(body["critical"].(float64)-1.0/20) > 1e-9 {
		t.Errorf("critical = %v, want 0.05", body["critical"])
	}
	if body["expected_damage"].(float64) <= 0 {
		t.Errorf("expected_damage = %v, want a positive number", body["expected_damage"])
	}
}

// Omitting crit_range must not mean "crits on a 0"; it means the ordinary 20.
func TestAttackProbabilityDefaultsTheCritRange(t *testing.T) {
	r := diceRouter()
	_, withDefault := call(t, r, http.MethodPost, "/dice/probability/attack",
		gin.H{"target_ac": 15, "modifier": 7, "damage": "1d8+4"})
	_, explicit := call(t, r, http.MethodPost, "/dice/probability/attack",
		gin.H{"target_ac": 15, "modifier": 7, "damage": "1d8+4", "crit_range": 20})

	if withDefault["expected_damage"] != explicit["expected_damage"] {
		t.Errorf("default crit range gave %v, explicit 20 gave %v",
			withDefault["expected_damage"], explicit["expected_damage"])
	}
}

func TestAttackProbabilityRejectsBadInput(t *testing.T) {
	r := diceRouter()
	for _, body := range []gin.H{
		{"target_ac": 15, "modifier": 7, "damage": "greatsword"},
		{"target_ac": 15, "modifier": 7, "damage": "1d8", "crit_range": 25},
		{"target_ac": 15, "modifier": 7},
	} {
		rec, _ := call(t, r, http.MethodPost, "/dice/probability/attack", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%v returned %d, want 400", body, rec.Code)
		}
	}
}

func TestDiceEndpointsRejectMalformedJSON(t *testing.T) {
	r := diceRouter()
	for _, path := range []string{"/dice/roll", "/dice/d20", "/dice/damage", "/dice/probability/check"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("{not json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s returned %d for malformed JSON, want 400", path, rec.Code)
		}
	}
}

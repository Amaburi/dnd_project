package turn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dnd-campaign/manager/internal/application/memory"
	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/domain/rules"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
)

// --- fakes -----------------------------------------------------------------
//
// Narrow enough to be obvious, which is the point of the interfaces: the
// orchestration is what needs testing and a database would only obscure it.

type fakeCharacters struct{ character *models.Character }

func (f *fakeCharacters) GetCharacterByCharacterID(context.Context, string) (*models.Character, error) {
	return f.character, nil
}

type fakeMonsters struct {
	monsters []*models.Monster
	writes   []models.HitPoints
}

func (f *fakeMonsters) GetMonstersByCampaign(context.Context, string) ([]*models.Monster, error) {
	return f.monsters, nil
}

func (f *fakeMonsters) UpdateHitPoints(_ context.Context, _, _ string, hp models.HitPoints) error {
	f.writes = append(f.writes, hp)
	return nil
}

type fakeSessions struct{ session *models.Session }

func (f *fakeSessions) GetActiveSession(context.Context, string) (*models.Session, error) {
	return f.session, nil
}

type fakeEvents struct {
	appended []*models.StoryEvent
	history  []*models.StoryEvent
}

func (f *fakeEvents) AppendEvent(_ context.Context, event *models.StoryEvent) error {
	f.appended = append(f.appended, event)
	return nil
}

func (f *fakeEvents) GetRecentEvents(context.Context, string, int) ([]*models.StoryEvent, error) {
	return f.history, nil
}

func (f *fakeEvents) GetEventsSince(_ context.Context, _ string, since time.Time, _ int) ([]*models.StoryEvent, error) {
	var out []*models.StoryEvent
	for _, e := range f.history {
		if e.Timestamp.After(since) {
			out = append(out, e)
		}
	}
	return out, nil
}

// --- fixtures ---------------------------------------------------------------

func rapier() models.InventoryItem {
	return models.InventoryItem{
		ItemID: "w1", Key: "rapier", Name: "Rapier", Kind: models.ItemWeapon,
		Weapon: &models.WeaponProperties{
			Category: models.WeaponMartial, DamageDice: "1d8",
			DamageType: models.DamagePiercing,
			Properties: []models.WeaponProperty{models.PropertyFinesse},
		},
	}
}

func hero() *models.Character {
	c := &models.Character{
		CharacterID: "ch1", Name: "Thistle", Type: models.CharacterPlayer,
		BasicInfo: models.BasicInfo{
			Race: models.RaceHalfling, Subrace: "lightfoot",
			Background: models.BackgroundCriminal,
			Classes:    []models.ClassLevel{{Class: models.ClassRogue, Subclass: "thief", Level: 5}},
		},
		AbilityScores: models.AbilityScores{
			Strength: 10, Dexterity: 18, Constitution: 14,
			Intelligence: 12, Wisdom: 13, Charisma: 11,
		},
		Skills: models.SkillProficiencies{models.SkillStealth: models.ProficiencyExpertise},
		Proficiencies: models.Proficiencies{
			Weapons: []string{models.ProfSimpleWeapons, "rapier"},
		},
		Inventory:   []models.InventoryItem{rapier()},
		Equipment:   models.Equipment{Weapons: []models.InventoryItem{rapier()}},
		CombatStats: models.CombatStats{HitPoints: models.HitPoints{Current: 33, Maximum: 33}},
	}
	c.ApplyClassDefaults()
	return c
}

func goblin() *models.Monster {
	for _, m := range models.SRDMonsters() {
		if m.MonsterID == "srd_goblin" {
			copy := m
			copy.CampaignID = "camp1"
			copy.HitPoints = models.HitPoints{Current: 30, Maximum: 30} // survives one hit
			return &copy
		}
	}
	panic("goblin missing")
}

type harness struct {
	service  *Service
	monsters *fakeMonsters
	events   *fakeEvents
	stub     *ai.StubClient
}

// newHarness wires a turn service whose only randomness is a fixed seed and
// whose only model is a stub, so a whole turn is reproducible and free.
func newHarness(t *testing.T, replies ...string) *harness {
	t.Helper()

	narrator, stub := ai.NewStubService(replies...)
	monsters := &fakeMonsters{monsters: []*models.Monster{goblin()}}
	events := &fakeEvents{}
	sessions := &fakeSessions{session: &models.Session{
		SessionID: "s1", CampaignID: "camp1", SessionNumber: 3,
		Status:   models.SessionStatusInProgress,
		Location: models.SessionLocation{CurrentLocation: "the wine cellar"},
	}}

	service := NewService(
		&fakeCharacters{character: hero()}, monsters, sessions, events,
		narrator, rules.NewEngine(dice.NewSeeded(1337)),
	)
	return &harness{service: service, monsters: monsters, events: events, stub: stub}
}

func request(input string) *Request {
	return &Request{CampaignID: "camp1", CharacterID: "ch1", Input: input, Scene: "a damp cellar"}
}

// --- tests ------------------------------------------------------------------

// The whole point of this package: one sentence in, a resolved and logged turn
// out. Before it, the parser, the engine and the log never spoke to each other.
func TestAttackTurnResolvesPersistsAndLogs(t *testing.T) {
	h := newHarness(t,
		`{"action":"attack","target":"Goblin","weapon":"Rapier","confidence":"high"}`,
		"The rapier slips between the goblin's ribs.",
	)

	result, err := h.service.TakeAction(context.Background(), request("I stab the goblin"))
	if err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	if result.Attack == nil {
		t.Fatal("no attack was resolved")
	}
	if result.Narration == "" {
		t.Error("no narration returned")
	}
	if result.NeedsClarification {
		t.Error("a clear sentence asked for clarification")
	}

	// Damage reached the monster and was written back.
	if len(h.monsters.writes) != 1 {
		t.Fatalf("hit points written %d times, want once", len(h.monsters.writes))
	}
	if h.monsters.writes[0].Current >= 30 {
		t.Errorf("monster is at %d hit points; the attack did not stick", h.monsters.writes[0].Current)
	}

	// And the turn was logged.
	if len(h.events.appended) != 1 {
		t.Fatalf("appended %d events, want one", len(h.events.appended))
	}
	event := h.events.appended[0]
	if event.SessionID != "s1" || event.CampaignID != "camp1" {
		t.Errorf("event is attached to %s/%s", event.CampaignID, event.SessionID)
	}
	if event.EventType != "combat_action" {
		t.Errorf("event type = %q, want combat_action", event.EventType)
	}
	if event.Trigger.PlayerInput != "I stab the goblin" {
		t.Errorf("event lost the player's words: %q", event.Trigger.PlayerInput)
	}
	if event.Narrative.AIGeneratedText != result.Narration {
		t.Error("the logged prose differs from what was returned")
	}
	// The engine's verdict is stored beside the prose so a later reader can
	// see what happened, not only how it was told.
	if !strings.Contains(event.Narrative.DMInterpretation, "Thistle") {
		t.Errorf("engine verdict not logged: %q", event.Narrative.DMInterpretation)
	}
	if event.Narrative.DiceResults == nil || event.Narrative.DiceResults.Roll.Natural == 0 {
		t.Error("the attack roll was not logged, so dice history stays empty")
	}
	if len(event.GameStateChanges.HPChanges) != 1 {
		t.Error("the hit point change was not recorded on the event")
	}
}

func TestSkillCheckTurnLogsTheRoll(t *testing.T) {
	h := newHarness(t,
		`{"action":"skill_check","skill":"stealth","suggested_dc":15,"confidence":"high"}`,
		"Thistle folds into the shadow of a wine rack.",
	)

	result, err := h.service.TakeAction(context.Background(), request("I sneak past the guard"))
	if err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	if result.Check == nil {
		t.Fatal("no check was resolved")
	}
	if result.Check.Skill != models.SkillStealth {
		t.Errorf("skill = %q, want stealth", result.Check.Skill)
	}
	if result.Check.DC != 15 {
		t.Errorf("DC = %d, want the parser's suggested 15", result.Check.DC)
	}
	if result.Attack != nil {
		t.Error("a skill check produced an attack")
	}

	event := h.events.appended[0]
	if event.EventType != "dice_roll" {
		t.Errorf("event type = %q, want dice_roll", event.EventType)
	}
	// This is what makes dice history accrue: the roll is on the event, and
	// the repository folds it into the session's totals.
	if event.Narrative.DiceResults == nil {
		t.Fatal("the check roll was not logged")
	}
	if event.Narrative.DiceResults.Roll.Natural < 1 || event.Narrative.DiceResults.Roll.Natural > 20 {
		t.Errorf("logged natural roll = %d", event.Narrative.DiceResults.Roll.Natural)
	}
}

// A narrative turn still logs, which is what gives the campaign continuity
// between the moments that need dice.
func TestNarrativeTurnStillLogs(t *testing.T) {
	h := newHarness(t,
		`{"action":"narrative","confidence":"high"}`,
		"Dust hangs in the lamplight. Racks of bottles recede into the dark.",
	)

	result, err := h.service.TakeAction(context.Background(), request("I look around the cellar"))
	if err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	if result.Check != nil || result.Attack != nil {
		t.Error("a narrative action resolved mechanics")
	}
	if len(h.events.appended) != 1 {
		t.Fatalf("appended %d events, want one", len(h.events.appended))
	}
	if h.events.appended[0].EventType != "narrative" {
		t.Errorf("event type = %q, want narrative", h.events.appended[0].EventType)
	}
}

// The history is what the campaign remembers; the narrator must be shown it.
func TestRecentHistoryReachesTheNarrator(t *testing.T) {
	h := newHarness(t,
		`{"action":"narrative","confidence":"high"}`,
		"prose",
	)
	h.events.history = []*models.StoryEvent{
		{Narrative: models.NarrativeInfo{AIGeneratedText: "The cellar door groans open."}},
		{Narrative: models.NarrativeInfo{AIGeneratedText: "Something scuttles behind the racks."}},
	}

	if _, err := h.service.TakeAction(context.Background(), request("I listen")); err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	// Two calls were made: the parse and the narration. Both should have been
	// told what happened recently.
	if h.stub.CallCount() != 2 {
		t.Fatalf("made %d model calls, want 2 (parse then narrate)", h.stub.CallCount())
	}
	prompt := h.stub.LastPrompt()
	if !strings.Contains(prompt, "scuttles behind the racks") {
		t.Errorf("the narrator was not shown recent history:\n%s", prompt)
	}
}

// A sentence that cannot be read is a question, and a question is not an event.
func TestUnclearTurnAsksAndLogsNothing(t *testing.T) {
	h := newHarness(t,
		`{"action":"unclear","confidence":"low","clarification":"Which of the two goblins?"}`,
	)

	result, err := h.service.TakeAction(context.Background(), request("I attack it"))
	if err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	if !result.NeedsClarification {
		t.Error("an unreadable sentence should ask for clarification")
	}
	if !strings.Contains(result.Clarification, "two goblins") {
		t.Errorf("clarification = %q", result.Clarification)
	}
	if len(h.events.appended) != 0 {
		t.Error("a question was written into the campaign's history")
	}
	if len(h.monsters.writes) != 0 {
		t.Error("an unresolved turn changed the game state")
	}
	// Only the parse call was made; narration was never attempted.
	if h.stub.CallCount() != 1 {
		t.Errorf("made %d model calls, want only the parse", h.stub.CallCount())
	}
}

// An intent naming a weapon the character lacks is repaired into a question by
// the AI layer, so the engine never sees it.
func TestHallucinatedWeaponNeverReachesTheEngine(t *testing.T) {
	h := newHarness(t,
		`{"action":"attack","target":"Goblin","weapon":"Greatsword","confidence":"high"}`,
	)

	result, err := h.service.TakeAction(context.Background(), request("I swing my greatsword"))
	if err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	if !result.NeedsClarification {
		t.Fatal("an impossible weapon should become a question")
	}
	if len(h.monsters.writes) != 0 {
		t.Error("a hallucinated attack changed the game state")
	}
}

// Without a session there is nowhere to log a turn, and an unlogged turn is
// exactly the gap this package closes.
func TestTurnRequiresAnActiveSession(t *testing.T) {
	narrator, _ := ai.NewStubService(`{"action":"narrative","confidence":"high"}`)
	service := NewService(
		&fakeCharacters{character: hero()},
		&fakeMonsters{},
		&fakeSessions{session: nil},
		&fakeEvents{},
		narrator, rules.NewEngine(dice.NewSeeded(1)),
	)

	_, err := service.TakeAction(context.Background(), request("I look around"))
	if err == nil {
		t.Fatal("a turn without a session should be refused")
	}
	if !strings.Contains(err.Error(), "session") {
		t.Errorf("error %q does not explain the missing session", err)
	}
}

func TestTurnRejectsEmptyInput(t *testing.T) {
	h := newHarness(t, `{"action":"narrative","confidence":"high"}`)

	if _, err := h.service.TakeAction(context.Background(), request("   ")); err == nil {
		t.Error("an empty sentence should be refused before any call is made")
	}
	if h.stub.CallCount() != 0 {
		t.Error("a model call was made for an empty sentence")
	}
}

// Dead creatures are not offered as targets, so the parser cannot pick one.
func TestDeadMonstersAreNotTargets(t *testing.T) {
	h := newHarness(t, `{"action":"narrative","confidence":"high"}`, "prose")
	h.monsters.monsters[0].HitPoints = models.HitPoints{Current: 0, Maximum: 30}

	if _, err := h.service.TakeAction(context.Background(), request("I look around")); err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	// The parse prompt lists targets; a corpse should not be among them.
	prompt := h.stub.Requests[0].Messages[1].Content
	if strings.Contains(prompt, "Targets in play: Goblin") {
		t.Errorf("a dead creature was offered as a target:\n%s", prompt)
	}
}

// Cost and tokens are summed across both calls, which is what the session's
// running total is built from.
func TestTurnReportsCombinedUsage(t *testing.T) {
	h := newHarness(t,
		`{"action":"narrative","confidence":"high"}`,
		"prose",
	)

	result, err := h.service.TakeAction(context.Background(), request("I look around"))
	if err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	// The stub reports 150 tokens per call, and two calls were made.
	if result.TokensUsed != 300 {
		t.Errorf("tokens = %d, want both calls counted", result.TokensUsed)
	}
	if result.Cost <= 0 {
		t.Error("cost was not accumulated")
	}
	if h.events.appended[0].Metadata.CostUSD != result.Cost {
		t.Error("the logged cost differs from the reported one")
	}
	if result.Elapsed <= 0 {
		t.Error("elapsed time was not measured")
	}
}

// The same seed and the same replies must produce the same turn, or a rules
// change cannot be told apart from noise.
func TestTurnIsReproducible(t *testing.T) {
	run := func() string {
		h := newHarness(t,
			`{"action":"attack","target":"Goblin","weapon":"Rapier","confidence":"high"}`,
			"prose",
		)
		result, err := h.service.TakeAction(context.Background(), request("I stab the goblin"))
		if err != nil {
			t.Fatalf("TakeAction: %v", err)
		}
		return result.Attack.Summary()
	}

	if run() != run() {
		t.Error("the same seed produced a different turn")
	}
}

// The history a turn sends is budgeted now, not an unbounded dump of the log.
// Without this, a long campaign eventually sends a request the provider
// refuses -- and it refuses the whole turn, not just the history.
func TestTurnBudgetsTheHistoryItSends(t *testing.T) {
	h := newHarness(t, `{"action":"narrative","confidence":"high"}`, "You look around.")
	for i := 1; i <= 60; i++ {
		h.events.history = append(h.events.history, &models.StoryEvent{
			SequenceNumber: i,
			Narrative: models.NarrativeInfo{
				AIGeneratedText: fmt.Sprintf("event %d: the party walked onward through a long and rainy stretch of moor", i),
			},
		})
	}

	budgeted := memory.New(h.events, nil, nil)
	budgeted.Budget = memory.Budget{MaxTokens: 120, MinRecent: 2}
	h.service.Memory = budgeted

	if _, err := h.service.TakeAction(context.Background(), request("I look around")); err != nil {
		t.Fatalf("TakeAction: %v", err)
	}

	prompt := h.stub.LastPrompt()
	if strings.Contains(prompt, "event 1:") {
		t.Error("the oldest event survived a budget that should have cut it")
	}
	if !strings.Contains(prompt, "event 60:") {
		t.Error("the newest event was cut, which is the one thing the budget must never do")
	}
	if !strings.Contains(prompt, "earlier events are omitted") {
		t.Error("the elided history was not disclosed, so the DM reads it as the whole campaign")
	}
}

// A campaign with a rolling summary must send it: that is the only thing that
// remembers session one once session ten is under way.
func TestTurnSendsTheRollingSummary(t *testing.T) {
	h := newHarness(t, `{"action":"narrative","confidence":"high"}`, "You look around.")
	h.service.Memory = staticMemory{summary: "Act one: the party cleared the goblin cave and freed Sildar."}

	if _, err := h.service.TakeAction(context.Background(), request("I look around")); err != nil {
		t.Fatalf("TakeAction: %v", err)
	}
	if !strings.Contains(h.stub.LastPrompt(), "freed Sildar") {
		t.Errorf("the rolling summary never reached the prompt:\n%s", h.stub.LastPrompt())
	}
}

type staticMemory struct{ summary string }

func (s staticMemory) Build(context.Context, string) (memory.Context, error) {
	return memory.Assemble(s.summary, nil, memory.Budget{}), nil
}

type fakeCompactor struct {
	calls int
	err   error
}

func (f *fakeCompactor) Compact(context.Context, string) (bool, error) {
	f.calls++
	return f.err == nil, f.err
}

// Compaction is self-regulating: it runs when the budget actually starts
// cutting history, and not before. A fixed "every N turns" would either
// summarise nothing or pay for a provider call the campaign did not need.
func TestCompactionRunsOnlyWhenHistoryIsBeingDropped(t *testing.T) {
	short := newHarness(t, `{"action":"narrative","confidence":"high"}`, "You look around.")
	short.events.history = []*models.StoryEvent{
		{SequenceNumber: 1, Narrative: models.NarrativeInfo{AIGeneratedText: "The party entered the inn."}},
	}
	quiet := &fakeCompactor{}
	short.service.Compactor = quiet

	if _, err := short.service.TakeAction(context.Background(), request("I look around")); err != nil {
		t.Fatalf("TakeAction: %v", err)
	}
	if quiet.calls != 0 {
		t.Errorf("compacted %d times with one event of history, want none", quiet.calls)
	}

	long := newHarness(t, `{"action":"narrative","confidence":"high"}`, "You look around.")
	for i := 1; i <= 60; i++ {
		long.events.history = append(long.events.history, &models.StoryEvent{
			SequenceNumber: i,
			Narrative: models.NarrativeInfo{
				AIGeneratedText: fmt.Sprintf("event %d: the party walked onward through a long and rainy stretch of moor", i),
			},
		})
	}
	budgeted := memory.New(long.events, nil, nil)
	budgeted.Budget = memory.Budget{MaxTokens: 120, MinRecent: 2}
	long.service.Memory = budgeted
	busy := &fakeCompactor{}
	long.service.Compactor = busy

	if _, err := long.service.TakeAction(context.Background(), request("I look around")); err != nil {
		t.Fatalf("TakeAction: %v", err)
	}
	if busy.calls != 1 {
		t.Errorf("compacted %d times while dropping history, want once", busy.calls)
	}
}

// A failed compaction must not cost the player their turn: the action already
// resolved and was logged, and losing that to a provider hiccup would be worse
// than a slightly oversized prompt next time.
func TestAFailedCompactionDoesNotFailTheTurn(t *testing.T) {
	h := newHarness(t, `{"action":"narrative","confidence":"high"}`, "You look around.")
	for i := 1; i <= 60; i++ {
		h.events.history = append(h.events.history, &models.StoryEvent{
			SequenceNumber: i,
			Narrative:      models.NarrativeInfo{AIGeneratedText: fmt.Sprintf("event %d: a long and rainy stretch of moor was crossed", i)},
		})
	}
	budgeted := memory.New(h.events, nil, nil)
	budgeted.Budget = memory.Budget{MaxTokens: 120, MinRecent: 2}
	h.service.Memory = budgeted
	h.service.Compactor = &fakeCompactor{err: errors.New("provider is down")}

	result, err := h.service.TakeAction(context.Background(), request("I look around"))
	if err != nil {
		t.Fatalf("a compaction failure failed the whole turn: %v", err)
	}
	if result.Narration == "" {
		t.Error("the turn produced no narration")
	}
	if len(h.events.appended) != 1 {
		t.Errorf("appended %d events, want the turn's one", len(h.events.appended))
	}
}

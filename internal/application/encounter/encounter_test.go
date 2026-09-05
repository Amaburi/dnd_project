package encounter

import (
	"context"
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/domain/rules"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
)

// --- fakes -------------------------------------------------------------------

type fakeEncounters struct {
	encounter *models.CombatEncounter
	saves     int
}

func (f *fakeEncounters) GetEncounterByEncounterID(_ context.Context, _, _ string) (*models.CombatEncounter, error) {
	return f.encounter, nil
}

func (f *fakeEncounters) SaveEncounterState(_ context.Context, _ string, e *models.CombatEncounter) error {
	f.saves++
	f.encounter = e
	return nil
}

type fakeMonsters struct{ monsters map[string]*models.Monster }

func (f *fakeMonsters) GetMonsterByMonsterID(_ context.Context, _, monsterID string) (*models.Monster, error) {
	return f.monsters[monsterID], nil
}

func (f *fakeMonsters) UpdateHitPoints(_ context.Context, _, _ string, _ models.HitPoints) error {
	return nil
}

type fakeCharacters struct {
	characters  map[string]*models.Character
	hpWrites    []models.HitPoints
	spellWrites []models.Spells
}

func (f *fakeCharacters) UpdateSpellSlots(_ context.Context, _ string, spells models.Spells) error {
	f.spellWrites = append(f.spellWrites, spells)
	return nil
}

func (f *fakeCharacters) GetCharacterByCharacterID(_ context.Context, id string) (*models.Character, error) {
	return f.characters[id], nil
}

func (f *fakeCharacters) UpdateHitPoints(_ context.Context, _, _ string, hp models.HitPoints) error {
	f.hpWrites = append(f.hpWrites, hp)
	return nil
}

type fakeEvents struct{ appended []*models.StoryEvent }

func (f *fakeEvents) AppendEvent(_ context.Context, e *models.StoryEvent) error {
	f.appended = append(f.appended, e)
	return nil
}

// --- fixtures ----------------------------------------------------------------

func goblin() *models.Monster {
	bonus := 4
	return &models.Monster{
		MonsterID: "m1", CampaignID: "camp1", Name: "Goblin",
		Size: "Small", Type: "humanoid", ArmorClass: 15,
		HitPoints: models.HitPoints{Current: 7, Maximum: 7},
		HitDice:   "2d6",
		Speeds:    models.Speeds{Walk: 30},
		AbilityScores: models.AbilityScores{
			Strength: 8, Dexterity: 14, Constitution: 10,
			Intelligence: 10, Wisdom: 8, Charisma: 8,
		},
		ChallengeRating: 0.25,
		Actions: []models.MonsterAction{{
			Name: "Scimitar", AttackBonus: &bonus,
			DamageDice: "1d6+2", DamageType: models.DamageSlashing,
		}},
	}
}

func fighter() *models.Character {
	c := &models.Character{
		CharacterID: "c1", Name: "Thistle", Type: models.CharacterPlayer,
		BasicInfo: models.BasicInfo{
			Race: models.RaceHuman, Subrace: "standard", Background: models.BackgroundSoldier,
			Classes: []models.ClassLevel{{Class: models.ClassFighter, Subclass: "champion", Level: 3}},
		},
		AbilityScores: models.AbilityScores{
			Strength: 16, Dexterity: 14, Constitution: 14,
			Intelligence: 10, Wisdom: 12, Charisma: 10,
		},
	}
	c.ApplyClassDefaults()
	c.CombatStats.HitPoints = models.HitPoints{Current: 28, Maximum: 28}
	return c
}

func activeEncounter() *models.CombatEncounter {
	monster := goblin().ToCombatant("cb-goblin")
	monster.Initiative = 20
	hero := fighter().ToCombatant("cb-hero")
	hero.Initiative = 10

	return &models.CombatEncounter{
		EncounterID: "e1", CampaignID: "camp1", SessionID: "s1",
		Combatants: []models.Combatant{monster, hero},
		CombatState: models.CombatState{
			Phase: models.PhaseActive, Round: 1, Turn: 0,
		},
	}
}

type harness struct {
	service    *Service
	encounters *fakeEncounters
	characters *fakeCharacters
	events     *fakeEvents
	stub       *ai.StubClient
}

func newHarness(t *testing.T, replies ...string) *harness {
	t.Helper()

	narrator, stub := ai.NewStubService(replies...)
	encounters := &fakeEncounters{encounter: activeEncounter()}
	monsters := &fakeMonsters{monsters: map[string]*models.Monster{"m1": goblin()}}
	characters := &fakeCharacters{characters: map[string]*models.Character{"c1": fighter()}}
	events := &fakeEvents{}

	service := NewService(encounters, monsters, characters, events, narrator,
		rules.NewEngine(dice.NewSeeded(99)))
	return &harness{service: service, encounters: encounters,
		characters: characters, events: events, stub: stub}
}

// --- tests -------------------------------------------------------------------

// The gap this closes: nothing ever resolved a monster's turn. You could track
// whose turn it was and never play it, so every fight was one-sided.
func TestAMonsterTakesItsTurn(t *testing.T) {
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"The goblin darts in, scimitar first.")

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.Attack == nil {
		t.Fatal("the monster made no attack")
	}
	if result.Actor != "Goblin" || result.Target != "Thistle" {
		t.Errorf("%s attacked %s, want Goblin attacking Thistle", result.Actor, result.Target)
	}
	if result.Narration == "" {
		t.Error("the turn produced no narration")
	}
	if h.encounters.saves == 0 {
		t.Error("the encounter was never saved")
	}
	if len(h.events.appended) != 1 {
		t.Errorf("logged %d events, want one", len(h.events.appended))
	}
}

// Damage to a player character has to reach the character sheet, or the party
// heals itself by leaving the encounter.
func TestDamageToACharacterIsPersisted(t *testing.T) {
	// A natural 20 hits whatever the AC, then the damage die.
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"The blade bites.")
	h.service.engine = rules.NewEngine(dice.NewScripted(20, 6))

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if !result.Attack.Hit() {
		t.Fatalf("a natural 20 missed: %s", result.Attack.Summary())
	}
	if len(h.characters.hpWrites) != 1 {
		t.Fatalf("wrote hit points %d times, want once", len(h.characters.hpWrites))
	}
	if h.characters.hpWrites[0].Current >= 28 {
		t.Errorf("the character is still at %d hit points", h.characters.hpWrites[0].Current)
	}
}

// The turn must advance, or calling twice replays the same goblin for ever.
func TestTheTurnAdvances(t *testing.T) {
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"A swing.")

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.NextCombatant != "Thistle" {
		t.Errorf("next up is %q, want Thistle", result.NextCombatant)
	}
	if h.encounters.encounter.CombatState.Turn != 1 {
		t.Errorf("turn index = %d, want 1", h.encounters.encounter.CombatState.Turn)
	}
}

// A player's turn is theirs to take. Resolving it automatically would play the
// game for them.
func TestAPlayersTurnIsNotResolvedAutomatically(t *testing.T) {
	h := newHarness(t, "unused")
	h.encounters.encounter.CombatState.Turn = 1 // the fighter

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if !result.AwaitingPlayer {
		t.Error("a player's turn should be reported as awaiting them")
	}
	if result.Attack != nil {
		t.Error("the service played the player's turn for them")
	}
	if len(h.stub.Requests) != 0 {
		t.Errorf("the provider was called %d times for a player's turn", len(h.stub.Requests))
	}
}

// A paralysed monster does nothing, and asking the model what it does is a
// call spent on an impossible question.
func TestAnIncapacitatedMonsterSkipsItsTurn(t *testing.T) {
	h := newHarness(t, "unused")
	h.encounters.encounter.Combatants[0].Conditions = []models.Condition{models.ConditionParalyzed}

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.Attack != nil {
		t.Error("a paralysed monster attacked")
	}
	if !result.Skipped {
		t.Error("the turn was not reported as skipped")
	}
	if !strings.Contains(result.SkipReason, "paralyzed") {
		t.Errorf("skip reason = %q, want it to name the condition", result.SkipReason)
	}
	if len(h.stub.Requests) != 0 {
		t.Errorf("the provider was called %d times for a creature that cannot act", len(h.stub.Requests))
	}
	// The turn still passes: a skipped turn is still a turn.
	if h.encounters.encounter.CombatState.Turn != 1 {
		t.Errorf("turn index = %d, want the turn to have advanced", h.encounters.encounter.CombatState.Turn)
	}
}

// A fight that is over must end rather than roll initiative into an empty room.
func TestAnEncounterThatIsDecidedEnds(t *testing.T) {
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"The last blow.")
	// The fighter is already down, so the party has nobody standing. The sheet
	// says so too: the source document is what the encounter refreshes from.
	h.encounters.encounter.Combatants[1].Status = models.CombatantDying
	h.encounters.encounter.Combatants[1].HitPoints.Current = 0
	h.characters.characters["c1"].CombatStats.HitPoints.Current = 0

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.Outcome == "" {
		t.Error("a decided encounter reported no outcome")
	}
	if h.encounters.encounter.CombatState.Phase != models.PhaseEnded {
		t.Errorf("phase = %s, want ended", h.encounters.encounter.CombatState.Phase)
	}
}

// Resolving a turn in an encounter that has not started, or has finished, is a
// caller error rather than something to guess at.
func TestResolvingOutsideAnActiveEncounterIsRefused(t *testing.T) {
	for _, phase := range []models.CombatPhase{models.PhaseNotStarted, models.PhaseEnded} {
		h := newHarness(t, "unused")
		h.encounters.encounter.CombatState.Phase = phase

		if _, err := h.service.ResolveTurn(context.Background(), "camp1", "e1"); err == nil {
			t.Errorf("resolving a %s encounter should be refused", phase)
		}
	}
}

// A missing encounter is a 404's worth of information, not a panic.
func TestAMissingEncounterIsReported(t *testing.T) {
	h := newHarness(t, "unused")
	h.encounters.encounter = nil

	if _, err := h.service.ResolveTurn(context.Background(), "camp1", "nope"); err == nil {
		t.Error("a missing encounter should be an error")
	}
}

// A dying character rolls a death save on their turn. It is not a choice, so
// it is not "awaiting the player": leaving it to them means a character who
// never stabilises and never dies.
func TestADyingCharacterRollsADeathSave(t *testing.T) {
	h := newHarness(t)
	h.encounters.encounter.CombatState.Turn = 1
	down := &h.encounters.encounter.Combatants[1]
	down.Status = models.CombatantDying
	down.HitPoints.Current = 0
	h.characters.characters["c1"].CombatStats.HitPoints.Current = 0

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.AwaitingPlayer {
		t.Error("a death save was left to the player to take")
	}
	if result.DeathSave == nil {
		t.Fatal("no death save was rolled")
	}
	if len(h.stub.Requests) != 0 {
		t.Errorf("the provider was called %d times for a death save", len(h.stub.Requests))
	}
	// The tally has to persist, or the character rolls the first save for ever.
	if len(h.characters.hpWrites) == 0 && h.encounters.saves == 0 {
		t.Error("nothing was persisted")
	}
}

// A stable character is unconscious but no longer dying: no save, no action.
func TestAStableCharacterSimplySkips(t *testing.T) {
	h := newHarness(t)
	h.encounters.encounter.CombatState.Turn = 1
	down := &h.encounters.encounter.Combatants[1]
	down.Status = models.CombatantStable
	down.HitPoints.Current = 0
	h.characters.characters["c1"].CombatStats.HitPoints.Current = 0

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.DeathSave != nil {
		t.Error("a stable character rolled a death save")
	}
	if !result.Skipped {
		t.Error("a stable character's turn should be skipped")
	}
}

// The bug this closes: a player attacking through the action endpoint writes
// to the monster's statblock, while the encounter kept its own copy of the
// same creature's hit points. The goblin the party had beaten to 1 hit point
// came back to full on its own turn.
func TestCombatantHitPointsAreRefreshedFromTheirSource(t *testing.T) {
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"A swing.")

	// The party wounded the goblin through the action endpoint: the statblock
	// knows, the encounter's stale copy does not.
	wounded := goblin()
	wounded.HitPoints.Current = 1
	h.service.monsters = &fakeMonsters{monsters: map[string]*models.Monster{"m1": wounded}}

	if _, err := h.service.ResolveTurn(context.Background(), "camp1", "e1"); err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if got := h.encounters.encounter.Combatants[0].HitPoints.Current; got != 1 {
		t.Errorf("the goblin is at %d hit points in the encounter, want the statblock's 1", got)
	}
}

// The same for characters: healing outside combat has to be visible inside it.
func TestCharacterCombatantsAreRefreshedToo(t *testing.T) {
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"A swing.")

	healed := fighter()
	healed.CombatStats.HitPoints.Current = 12
	h.service.characters = &fakeCharacters{characters: map[string]*models.Character{"c1": healed}}

	if _, err := h.service.ResolveTurn(context.Background(), "camp1", "e1"); err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	// 12 at the start of the turn, less whatever the goblin managed.
	if got := h.encounters.encounter.Combatants[1].HitPoints.Current; got > 12 {
		t.Errorf("the fighter is at %d hit points, want no more than the sheet's 12", got)
	}
}

// A held spell has to be at risk, or concentration is a label rather than a
// cost: the wizard holds Hold Person through every blow of the fight.
func TestDamageThreatensAHeldSpell(t *testing.T) {
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"The blade bites deep.")
	// The fighter is holding Web on the goblin. Restrained is deliberate: a
	// restrained creature can still attack, so the goblin gets a turn -- which
	// is what puts the spell at risk in the first place.
	holder := h.characters.characters["c1"]
	holder.BeginConcentration(models.Concentration{
		Spell: "Web", Condition: models.ConditionRestrained,
		Targets: []string{"cb-goblin"},
	})
	h.encounters.encounter.Combatants[0].Conditions = []models.Condition{models.ConditionRestrained}

	// Restrained gives the goblin disadvantage, so two dice are rolled and the
	// lower kept: two 20s to hit regardless. A critical doubles the damage
	// dice, then a natural 1 fails the concentration save.
	h.service.engine = rules.NewEngine(dice.NewScripted(20, 20, 6, 6, 1))

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.Concentration == nil {
		t.Fatal("damage forced no concentration save")
	}
	if !result.Concentration.Broken {
		t.Fatalf("a natural 1 held the spell: %+v", result.Concentration)
	}
	if holder.IsConcentrating() {
		t.Error("the character is still holding the spell")
	}
	// The condition the spell imposed has to lift with it, or the goblin stays
	// webbed for the rest of the campaign.
	if h.encounters.encounter.Combatants[0].HasCondition(models.ConditionRestrained) {
		t.Error("the goblin is still restrained after the spell ended")
	}
	if len(h.characters.spellWrites) == 0 {
		t.Error("the dropped concentration was never persisted")
	}
}

// A blow that misses threatens nothing.
func TestAMissDoesNotThreatenConcentration(t *testing.T) {
	h := newHarness(t,
		`{"action":"Scimitar","target":"Thistle","rationale":"nearest"}`,
		"The blade whistles past.")
	h.service.engine = rules.NewEngine(dice.NewScripted(1)) // a natural 1 misses

	holder := h.characters.characters["c1"]
	holder.BeginConcentration(models.Concentration{Spell: "Hold Person"})

	result, err := h.service.ResolveTurn(context.Background(), "camp1", "e1")
	if err != nil {
		t.Fatalf("ResolveTurn: %v", err)
	}
	if result.Attack.Hit() {
		t.Fatal("a natural 1 hit")
	}
	if result.Concentration != nil {
		t.Error("a miss forced a concentration save")
	}
	if !holder.IsConcentrating() {
		t.Error("a miss dropped the spell")
	}
}

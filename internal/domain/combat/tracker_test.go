package combat

import (
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
)

func combatant(name, side string, initiative, hp int) models.Combatant {
	return models.Combatant{
		CombatantID: name, Name: name, Type: side,
		Initiative: initiative, ArmorClass: 13,
		HitPoints:       models.HitPoints{Current: hp, Maximum: hp},
		Status:          models.CombatantActive,
		Speed:           30,
		MakesDeathSaves: side == "player",
	}
}

func encounter(combatants ...models.Combatant) *Tracker {
	e := &models.CombatEncounter{EncounterID: "e1", EncounterName: "Cellar ambush"}
	t := NewTracker(e)
	for _, c := range combatants {
		_ = t.AddCombatant(c)
	}
	return t
}

func TestInitiativeOrdersHighestFirst(t *testing.T) {
	tr := encounter(
		combatant("Thistle", "player", 12, 30),
		combatant("Goblin", "enemy", 19, 7),
		combatant("Ogre", "enemy", 5, 59),
	)

	if tr.Phase() != models.PhaseNotStarted {
		t.Fatalf("phase = %s, want not_started", tr.Phase())
	}

	if err := tr.RollInitiative(dice.NewSeeded(1)); err != nil {
		t.Fatalf("RollInitiative: %v", err)
	}

	order := tr.Order()
	if len(order) != 3 {
		t.Fatalf("got %d combatants, want 3", len(order))
	}
	if order[0].Name != "Goblin" || order[1].Name != "Thistle" || order[2].Name != "Ogre" {
		t.Errorf("order = %s, %s, %s; want Goblin, Thistle, Ogre",
			order[0].Name, order[1].Name, order[2].Name)
	}
	for i, c := range order {
		if c.TurnOrder != i+1 {
			t.Errorf("%s has turn order %d, want %d", c.Name, c.TurnOrder, i+1)
		}
	}

	if tr.Phase() != models.PhaseActive || tr.Round() != 1 {
		t.Errorf("phase/round = %s/%d, want active/1", tr.Phase(), tr.Round())
	}
	// The first combatant's turn has begun, so their economy is fresh.
	current, ok := tr.Current()
	if !ok || current.Name != "Goblin" {
		t.Fatalf("current = %v, want Goblin", current)
	}
	if current.MovementRemaining != 30 || current.ActionUsed {
		t.Errorf("the first turn did not start cleanly: %+v", current)
	}
}

// Equal initiative is broken on the Dexterity modifier, as the rules direct.
func TestInitiativeTiesBreakOnDexterity(t *testing.T) {
	quick := combatant("Quick", "player", 15, 20)
	quick.InitiativeModifier = 4
	slow := combatant("Slow", "player", 15, 20)
	slow.InitiativeModifier = 1

	tr := encounter(slow, quick)
	if err := tr.RollInitiative(dice.NewSeeded(2)); err != nil {
		t.Fatalf("RollInitiative: %v", err)
	}

	if tr.Order()[0].Name != "Quick" {
		t.Errorf("order = %v; the higher Dexterity should go first", tr.Order())
	}
}

// A hand-set initiative is not overruled by the roller.
func TestRollInitiativeKeepsPresetValues(t *testing.T) {
	tr := encounter(combatant("Boss", "enemy", 25, 100), combatant("Mook", "enemy", 0, 5))

	if err := tr.RollInitiative(dice.NewSeeded(3)); err != nil {
		t.Fatalf("RollInitiative: %v", err)
	}

	boss, _ := tr.Find("Boss")
	if boss.Initiative != 25 {
		t.Errorf("a preset initiative was overwritten: %d", boss.Initiative)
	}
	mook, _ := tr.Find("Mook")
	if mook.Initiative == 0 {
		t.Error("an unset initiative was not rolled")
	}
}

func TestNextTurnAdvancesAndWrapsIntoANewRound(t *testing.T) {
	tr := encounter(
		combatant("A", "player", 20, 20),
		combatant("B", "enemy", 15, 20),
		combatant("C", "player", 10, 20),
	)
	_ = tr.RollInitiative(dice.NewSeeded(4))

	names := []string{"B", "C"}
	for _, want := range names {
		next, ok := tr.NextTurn()
		if !ok || next.Name != want {
			t.Fatalf("next turn = %v, want %s", next, want)
		}
		if tr.Round() != 1 {
			t.Errorf("round advanced early to %d", tr.Round())
		}
	}

	// Wrapping past the last combatant starts round two.
	next, ok := tr.NextTurn()
	if !ok || next.Name != "A" {
		t.Fatalf("next turn = %v, want A", next)
	}
	if tr.Round() != 2 {
		t.Errorf("round = %d, want 2 after wrapping", tr.Round())
	}
}

// The dead keep their place in the order but never act again.
func TestNextTurnSkipsTheDead(t *testing.T) {
	tr := encounter(
		combatant("A", "player", 20, 20),
		combatant("B", "enemy", 15, 20),
		combatant("C", "player", 10, 20),
	)
	_ = tr.RollInitiative(dice.NewSeeded(5))

	b, _ := tr.Find("B")
	b.Status = models.CombatantDead

	next, ok := tr.NextTurn()
	if !ok || next.Name != "C" {
		t.Fatalf("next turn = %v, want C (B is dead)", next)
	}
	// B is still in the order, just skipped.
	if len(tr.Order()) != 3 {
		t.Errorf("the dead were removed from the order: %d combatants", len(tr.Order()))
	}
}

// A dying combatant still takes turns -- that is when death saves are rolled.
func TestNextTurnDoesNotSkipTheDying(t *testing.T) {
	tr := encounter(combatant("A", "player", 20, 20), combatant("B", "player", 10, 20))
	_ = tr.RollInitiative(dice.NewSeeded(6))

	b, _ := tr.Find("B")
	b.Status = models.CombatantDying

	next, ok := tr.NextTurn()
	if !ok || next.Name != "B" {
		t.Fatalf("next turn = %v; the dying still take turns", next)
	}
}

func TestOutcomeDetectsEliminationOnly(t *testing.T) {
	tr := encounter(combatant("Hero", "player", 20, 20), combatant("Goblin", "enemy", 10, 7))
	_ = tr.RollInitiative(dice.NewSeeded(7))

	if _, decided := tr.Outcome(); decided {
		t.Error("a fight with both sides standing is not decided")
	}

	goblin, _ := tr.Find("Goblin")
	goblin.Status = models.CombatantDead

	outcome, decided := tr.Outcome()
	if !decided || outcome != "victory" {
		t.Errorf("outcome = %q (decided=%v), want victory", outcome, decided)
	}
}

// A downed player counts as out even though they are not dead.
func TestDefeatWhenThePartyIsDown(t *testing.T) {
	tr := encounter(combatant("Hero", "player", 20, 20), combatant("Ogre", "enemy", 10, 59))
	_ = tr.RollInitiative(dice.NewSeeded(8))

	hero, _ := tr.Find("Hero")
	hero.Status = models.CombatantDying

	outcome, decided := tr.Outcome()
	if !decided || outcome != "defeat" {
		t.Errorf("outcome = %q (decided=%v), want defeat", outcome, decided)
	}
}

func TestEndIfDecidedClosesTheEncounter(t *testing.T) {
	tr := encounter(combatant("Hero", "player", 20, 20), combatant("Goblin", "enemy", 10, 7))
	_ = tr.RollInitiative(dice.NewSeeded(9))

	goblin, _ := tr.Find("Goblin")
	goblin.Status = models.CombatantDead

	outcome, ended := tr.EndIfDecided()
	if !ended || outcome != "victory" {
		t.Fatalf("EndIfDecided = %q, %v", outcome, ended)
	}
	if tr.Phase() != models.PhaseEnded {
		t.Errorf("phase = %s, want ended", tr.Phase())
	}
	if tr.Encounter().Status != "completed" {
		t.Errorf("status = %q, want completed", tr.Encounter().Status)
	}
	if tr.Encounter().VictoryConditions.Outcome == nil ||
		*tr.Encounter().VictoryConditions.Outcome != "victory" {
		t.Error("the outcome was not recorded on the encounter")
	}
	if tr.Encounter().CombatState.CombatEndedAt == nil {
		t.Error("the end time was not recorded")
	}
	// A finished encounter has no current turn.
	if _, ok := tr.Current(); ok {
		t.Error("a finished encounter still reports a current combatant")
	}
}

func TestAddCombatantRules(t *testing.T) {
	tr := encounter(combatant("A", "player", 20, 20))

	if err := tr.AddCombatant(combatant("A", "player", 15, 20)); err == nil {
		t.Error("a duplicate combatant id should be rejected")
	}
	if err := tr.AddCombatant(models.Combatant{Name: "No ID"}); err == nil {
		t.Error("a combatant without an id should be rejected")
	}

	// Reinforcements mid-fight take their place in the order.
	_ = tr.RollInitiative(dice.NewSeeded(10))
	if err := tr.AddCombatant(combatant("Reinforcement", "enemy", 25, 10)); err != nil {
		t.Fatalf("adding mid-fight: %v", err)
	}
	if tr.Order()[0].Name != "Reinforcement" {
		t.Errorf("the newcomer was not slotted into the order: %v", tr.Order())
	}

	tr.End("victory")
	if err := tr.AddCombatant(combatant("Latecomer", "enemy", 1, 1)); err == nil {
		t.Error("a finished encounter should not accept combatants")
	}
}

func TestRollInitiativeGuards(t *testing.T) {
	empty := encounter()
	if err := empty.RollInitiative(dice.NewSeeded(11)); err == nil {
		t.Error("rolling initiative with no combatants should fail")
	}

	tr := encounter(combatant("A", "player", 20, 20))
	_ = tr.RollInitiative(dice.NewSeeded(12))
	if err := tr.RollInitiative(dice.NewSeeded(12)); err == nil {
		t.Error("rolling initiative twice should fail")
	}
}

func TestLogsRecordTheFight(t *testing.T) {
	tr := encounter(combatant("Hero", "player", 20, 20), combatant("Goblin", "enemy", 10, 7))
	_ = tr.RollInitiative(dice.NewSeeded(13))

	tr.RecordDamage("Hero", "Goblin", 9, models.DamagePiercing)
	tr.RecordDamage("Hero", "Goblin", 4, models.DamagePiercing)
	tr.RecordNarration("The rapier finds a gap.")
	tr.RecordNarration("")
	tr.RecordTurn(models.Turn{CombatantID: "Hero", Notes: "attacked"})

	e := tr.Encounter()
	if len(e.DamageLog) != 2 {
		t.Errorf("damage log has %d entries, want 2", len(e.DamageLog))
	}
	if e.DamageLog[0].Round != 1 {
		t.Errorf("damage was logged in round %d, want 1", e.DamageLog[0].Round)
	}
	if len(e.NarrativeLog) != 1 {
		t.Errorf("narrative log has %d entries; an empty line should be ignored", len(e.NarrativeLog))
	}
	if len(e.TurnHistory) != 1 || e.TurnHistory[0].Round != 1 {
		t.Errorf("turn history = %+v", e.TurnHistory)
	}
	if got := tr.TotalDamage("Hero"); got != 13 {
		t.Errorf("total damage = %d, want 13", got)
	}
}

func TestSidesRemaining(t *testing.T) {
	tr := encounter(
		combatant("Hero", "player", 20, 20),
		combatant("Ally", "npc", 18, 20),
		combatant("Goblin", "enemy", 15, 7),
		combatant("Ogre", "enemy", 10, 59),
	)
	_ = tr.RollInitiative(dice.NewSeeded(14))

	party, foes := tr.SidesRemaining()
	if party != 2 || foes != 2 {
		t.Errorf("sides = %d party, %d foes; want 2 and 2 (npcs fight with the party)", party, foes)
	}

	goblin, _ := tr.Find("Goblin")
	goblin.Status = models.CombatantDead
	ally, _ := tr.Find("Ally")
	ally.Status = models.CombatantStable

	party, foes = tr.SidesRemaining()
	if party != 1 || foes != 1 {
		t.Errorf("sides = %d party, %d foes; want 1 and 1", party, foes)
	}
}

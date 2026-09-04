package rules

import (
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func mover(speed int) *models.Combatant {
	return &models.Combatant{
		Name: "Thistle", Status: models.CombatantActive,
		Speed: speed, MovementRemaining: speed,
		HitPoints: models.HitPoints{Current: 20, Maximum: 20},
	}
}

// Movement is a budget, not a flag: a creature can step, act and step again.
func TestMoveSpendsFromTheBudget(t *testing.T) {
	e := engine(1)
	c := mover(30)

	if err := e.Move(c, 10); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if c.MovementRemaining != 20 {
		t.Errorf("movement left = %d, want 20", c.MovementRemaining)
	}
	if err := e.Move(c, 20); err != nil {
		t.Fatalf("second Move: %v", err)
	}
	if c.MovementRemaining != 0 {
		t.Errorf("movement left = %d, want 0", c.MovementRemaining)
	}

	if err := e.Move(c, 5); err == nil {
		t.Error("moving past the budget should fail")
	}
	if err := e.Move(c, 0); err == nil {
		t.Error("moving zero feet should fail")
	}
}

func TestMoveBlockedByConditionAndState(t *testing.T) {
	e := engine(2)

	held := mover(30)
	held.Conditions = []models.Condition{models.ConditionGrappled}
	if err := e.Move(held, 5); err == nil {
		t.Error("a grappled creature should not move")
	}

	restrained := mover(30)
	restrained.Conditions = []models.Condition{models.ConditionRestrained}
	if err := e.Move(restrained, 5); err == nil {
		t.Error("a restrained creature should not move")
	}

	down := mover(30)
	down.Status = models.CombatantDying
	if err := e.Move(down, 5); err == nil {
		t.Error("a dying creature should not move")
	}
}

// Reporting whether a condition stuck matters: narrating a frightened creature
// that is immune to fear contradicts the state.
func TestApplyConditionHonoursImmunity(t *testing.T) {
	e := engine(3)
	c := mover(30)
	c.ConditionImmunities = []models.Condition{models.ConditionFrightened}

	applied, err := e.ApplyCondition(c, models.ConditionFrightened)
	if err != nil {
		t.Fatalf("ApplyCondition: %v", err)
	}
	if applied {
		t.Error("an immune creature should not gain the condition")
	}
	if c.HasCondition(models.ConditionFrightened) {
		t.Error("the condition was applied despite immunity")
	}

	applied, _ = e.ApplyCondition(c, models.ConditionProne)
	if !applied || !c.HasCondition(models.ConditionProne) {
		t.Error("a condition it is not immune to should apply")
	}

	// Applying twice is a no-op, not a duplicate.
	applied, _ = e.ApplyCondition(c, models.ConditionProne)
	if applied {
		t.Error("re-applying a condition should report no change")
	}

	if _, err := e.ApplyCondition(c, "inspired"); err == nil {
		t.Error("an invented condition should be rejected")
	}

	e.RemoveCondition(c, models.ConditionProne)
	if c.HasCondition(models.ConditionProne) {
		t.Error("the condition survived removal")
	}
}

func caster() *models.Character {
	c := rogue()
	c.BasicInfo.Classes = []models.ClassLevel{{Class: models.ClassWizard, Subclass: "evocation", Level: 5}}
	c.AbilityScores.Intelligence = 18
	c.ApplyClassDefaults()
	return c
}

// The slot is spent before anything is rolled: a spell that misses still costs
// the slot, and taking it afterwards would refund every failure.
func TestSpellAttackSpendsTheSlotEvenOnAMiss(t *testing.T) {
	e := engine(4)
	c := caster()
	before := c.Spells.AvailableSlots(1)

	target := dummy(40, 30) // unreachable AC, so the spell misses
	result, err := e.SpellAttack(c, "Chromatic Orb", 1, "3d8", models.DamageFire, target, models.RollNormal)
	if err != nil {
		t.Fatalf("SpellAttack: %v", err)
	}

	if result.Attack == nil || result.Attack.Hit() {
		t.Fatalf("expected a miss against AC 40, got %+v", result.Attack)
	}
	if got := c.Spells.AvailableSlots(1); got != before-1 {
		t.Errorf("slots = %d, want %d; a miss still costs the slot", got, before-1)
	}
	if target.HitPoints.Current != 30 {
		t.Error("a missed spell dealt damage")
	}
}

func TestSpellAttackDamagesOnAHit(t *testing.T) {
	// A natural 20 hits whatever the AC; the 10s are the damage dice.
	e := scripted(faceCriticalHit, 10)
	c := caster()
	target := dummy(5, 40)

	result, err := e.SpellAttack(c, "Fire Bolt", 0, "2d10", models.DamageFire, target, models.RollNormal)
	if err != nil {
		t.Fatalf("SpellAttack: %v", err)
	}
	if !result.Attack.Hit() {
		t.Fatalf("a natural 20 missed: %s", result.Summary())
	}
	if result.Damage == nil || result.Damage.Dealt <= 0 {
		t.Fatalf("a hit dealt no damage: %+v", result.Damage)
	}
	if target.HitPoints.Current >= 40 {
		t.Error("the target did not lose hit points")
	}
	// A cantrip costs no slot.
	if c.Spells.AvailableSlots(1) != 4 {
		t.Errorf("a cantrip spent a slot: %d left", c.Spells.AvailableSlots(1))
	}
}

func TestSpellAttackWithoutSlots(t *testing.T) {
	e := engine(6)
	c := caster()
	for i := 0; i < 4; i++ {
		_ = c.Spells.ExpendSlot(1)
	}

	if _, err := e.SpellAttack(c, "Chromatic Orb", 1, "3d8", models.DamageFire, dummy(5, 30), models.RollNormal); err == nil {
		t.Error("casting with no slots left should fail")
	}
}

// A successful save halves damage rather than negating it, which is the common
// case in 5e.
func TestSpellSaveHalvesOnSuccess(t *testing.T) {
	e := engine(7)
	c := caster()

	// A hopeless save modifier guarantees a failure, and an overwhelming one
	// guarantees a success, so both branches are exercised exactly.
	failed := dummy(10, 100)
	full, err := e.SpellSave(c, "Burning Hands", 1, models.AbilityDexterity,
		"3d6", models.DamageFire, true, failed, -20)
	if err != nil {
		t.Fatalf("SpellSave: %v", err)
	}
	if full.Save.Succeeded() {
		t.Fatal("a -20 modifier should not pass the save")
	}

	saved := dummy(10, 100)
	half, err := e.SpellSave(c, "Burning Hands", 1, models.AbilityDexterity,
		"3d6", models.DamageFire, true, saved, 40)
	if err != nil {
		t.Fatalf("SpellSave: %v", err)
	}
	if !half.Save.Succeeded() {
		t.Fatal("a +40 modifier should pass the save")
	}
	if half.Damage.Dealt > full.Damage.Dealt {
		t.Errorf("a successful save took %d and a failure %d", half.Damage.Dealt, full.Damage.Dealt)
	}
}

func TestSpellSaveCanNegateEntirely(t *testing.T) {
	e := engine(8)
	c := caster()
	target := dummy(10, 100)

	result, err := e.SpellSave(c, "Sacred Flame", 0, models.AbilityDexterity,
		"2d8", models.DamageRadiant, false, target, 40)
	if err != nil {
		t.Fatalf("SpellSave: %v", err)
	}
	if !result.Save.Succeeded() {
		t.Fatal("a +40 modifier should pass the save")
	}
	if result.Damage.Dealt != 0 {
		t.Errorf("a negating spell dealt %d on a success", result.Damage.Dealt)
	}
	if target.HitPoints.Current != 100 {
		t.Error("the target lost hit points despite saving")
	}
}

func TestSpellFactsAreCompleteForNarration(t *testing.T) {
	e := engine(9)
	c := caster()

	result, _ := e.SpellAttack(c, "Fire Bolt", 0, "2d10", models.DamageFire, dummy(5, 40), models.RollNormal)
	facts := result.Facts()

	for _, key := range []string{
		"attacker", "target", "weapon", "outcome", "damage_total", "damage_type",
		"target_hp", "target_status", "fact_summary", "critical", "hit",
	} {
		value, ok := facts[key]
		if !ok {
			t.Errorf("spell facts are missing %q", key)
			continue
		}
		if value == "" {
			t.Errorf("spell fact %q is empty", key)
		}
	}
	if !strings.Contains(facts["fact_summary"], "Fire Bolt") {
		t.Errorf("summary does not name the spell: %q", facts["fact_summary"])
	}
}

func dying() *models.Combatant {
	return &models.Combatant{
		Name: "Thistle", Status: models.CombatantDying, MakesDeathSaves: true,
		HitPoints: models.HitPoints{Current: 0, Maximum: 30},
	}
}

// Three successes stabilise, three failures kill, and a natural 20 puts the
// creature back on its feet.
func TestDeathSaveOutcomes(t *testing.T) {
	e := engine(10)

	var sawStable, sawDead, sawRevive bool
	for i := 0; i < 400 && !(sawStable && sawDead && sawRevive); i++ {
		c := dying()
		for c.Status == models.CombatantDying {
			result, err := e.DeathSave(c)
			if err != nil {
				t.Fatalf("DeathSave: %v", err)
			}
			switch {
			case result.Regained:
				sawRevive = true
				if c.HitPoints.Current != 1 || c.Status != models.CombatantActive {
					t.Fatalf("a natural 20 left %s at %d hit points, status %s",
						c.Name, c.HitPoints.Current, c.Status)
				}
			case c.Status == models.CombatantDead:
				sawDead = true
			case c.Status == models.CombatantStable:
				sawStable = true
			}
		}
	}

	if !sawStable || !sawDead || !sawRevive {
		t.Errorf("not every outcome occurred: stable=%v dead=%v revived=%v", sawStable, sawDead, sawRevive)
	}

	// Only the dying roll death saves.
	healthy := &models.Combatant{Name: "Hale", Status: models.CombatantActive}
	if _, err := e.DeathSave(healthy); err == nil {
		t.Error("a conscious creature should not roll death saves")
	}
}

func TestStabiliseAndHeal(t *testing.T) {
	e := engine(11)

	c := dying()
	c.DeathSaves = models.DeathSaves{Failures: 2}
	if err := e.Stabilise(c); err != nil {
		t.Fatalf("Stabilise: %v", err)
	}
	if c.Status != models.CombatantStable {
		t.Errorf("status = %s, want stable", c.Status)
	}
	if c.DeathSaves != (models.DeathSaves{}) {
		t.Error("stabilising should clear the tally")
	}
	if c.HitPoints.Current != 0 {
		t.Error("stabilising should not heal")
	}

	// Any healing at all brings a downed creature back.
	down := dying()
	if err := e.Heal(down, 1); err != nil {
		t.Fatalf("Heal: %v", err)
	}
	if down.Status != models.CombatantActive || down.HitPoints.Current != 1 {
		t.Errorf("healing left %s at %d hit points, status %s", down.Name, down.HitPoints.Current, down.Status)
	}

	if err := e.Heal(down, 0); err == nil {
		t.Error("healing zero should fail")
	}
}

// The dead need more than hit points, which is why reviving is a separate
// method a healing spell cannot reach by accident.
func TestTheDeadNeedReviving(t *testing.T) {
	e := engine(12)

	dead := &models.Combatant{Name: "Thistle", Status: models.CombatantDead,
		HitPoints: models.HitPoints{Maximum: 30}}

	if err := e.Heal(dead, 20); err == nil {
		t.Error("healing should not raise the dead")
	}
	if dead.Status != models.CombatantDead {
		t.Error("healing revived a corpse")
	}

	if err := e.Revive(dead); err != nil {
		t.Fatalf("Revive: %v", err)
	}
	if dead.Status != models.CombatantActive || dead.HitPoints.Current != 1 {
		t.Errorf("revived to %d hit points, status %s", dead.HitPoints.Current, dead.Status)
	}

	// Reviving the living is a mistake worth catching.
	if err := e.Revive(dead); err == nil {
		t.Error("reviving a living creature should fail")
	}
}

package rules

import (
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func def(t *testing.T, name string) models.SpellDefinition {
	t.Helper()
	d, ok := models.SpellByName(name)
	if !ok {
		t.Fatalf("%q is not in the spell table", name)
	}
	return d
}

// warlock returns an Eldritch Blast user at a chosen level.
func warlock(level int) *models.Character {
	c := caster()
	c.BasicInfo.Classes = []models.ClassLevel{{Class: models.ClassWarlock, Subclass: "fiend", Level: level}}
	c.AbilityScores.Charisma = 18
	c.ApplyClassDefaults()
	c.Spells.SpellcastingAbility = models.AbilityCharisma
	return c
}

// A cantrip costs nothing, which is the entire reason cantrips exist.
func TestCantripSpendsNoSlot(t *testing.T) {
	e := scripted(faceCriticalHit, 5, 5)
	c := caster()
	before := c.Spells.AvailableSlots(1)

	result, err := e.CastSpell(c, def(t, "Fire Bolt"), 0, dummy(10, 40), models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if got := c.Spells.AvailableSlots(1); got != before {
		t.Errorf("a cantrip spent a slot: %d then %d", before, got)
	}
	if len(result.Attacks) != 1 {
		t.Fatalf("Fire Bolt made %d attack rolls, want 1", len(result.Attacks))
	}
}

// Eldritch Blast is the case a single-roll implementation gets wrong: at 5th
// level it is two separate attacks, each of which can hit or miss on its own.
func TestEldritchBlastRollsOncePerBeam(t *testing.T) {
	// Two 20s then damage: both beams hit.
	e := scripted(faceCriticalHit, 6, 6, faceCriticalHit, 7, 7)
	c := warlock(5)
	target := dummy(10, 200)

	result, err := e.CastSpell(c, def(t, "Eldritch Blast"), 0, target, models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if len(result.Attacks) != 2 {
		t.Fatalf("a 5th level warlock fired %d beams, want 2", len(result.Attacks))
	}
	for i, attack := range result.Attacks {
		if !attack.Hit() {
			t.Errorf("beam %d missed on a natural 20", i+1)
		}
	}
	if result.Damage == nil || result.Damage.Dealt <= 0 {
		t.Fatal("two hits dealt no damage")
	}
	if target.HitPoints.Current >= 200 {
		t.Error("the target lost no hit points")
	}
}

// One beam hitting and one missing is a real outcome and must be reported as
// such, not collapsed into a single hit or a single miss.
func TestBeamsCanHitAndMissIndependently(t *testing.T) {
	// A natural 20 hits whatever the AC; a natural 1 always misses.
	e := scripted(faceCriticalHit, 6, 6, 1)
	c := warlock(5)

	result, err := e.CastSpell(c, def(t, "Eldritch Blast"), 0, dummy(25, 200), models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if len(result.Attacks) != 2 {
		t.Fatalf("fired %d beams, want 2", len(result.Attacks))
	}
	if !result.Attacks[0].Hit() || result.Attacks[1].Hit() {
		t.Errorf("expected a hit then a miss, got %v and %v",
			result.Attacks[0].Outcome, result.Attacks[1].Outcome)
	}
	if result.Hits != 1 {
		t.Errorf("Hits = %d, want 1", result.Hits)
	}
}

// Scorching Ray is the levelled version of the same trap, and it must spend
// exactly one slot however many rays it fires.
func TestScorchingRaySpendsOneSlotForAllRays(t *testing.T) {
	e := scripted(faceCriticalHit, 3, 3, faceCriticalHit, 3, 3, faceCriticalHit, 3, 3, faceCriticalHit, 3, 3)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 2, Total: 3}, {Level: 3, Total: 2}}

	result, err := e.CastSpell(c, def(t, "Scorching Ray"), 3, dummy(10, 300), models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	// A 3rd level slot gives four rays.
	if len(result.Attacks) != 4 {
		t.Errorf("fired %d rays from a level 3 slot, want 4", len(result.Attacks))
	}
	if got := c.Spells.AvailableSlots(3); got != 1 {
		t.Errorf("level 3 slots left = %d, want 1: four rays are still one casting", got)
	}
	if got := c.Spells.AvailableSlots(2); got != 3 {
		t.Errorf("the cast reached into the level 2 slots: %d left of 3", got)
	}
}

// Magic Missile never misses, so it must not roll an attack at all.
func TestMagicMissileAlwaysHits(t *testing.T) {
	e := scripted(4, 4, 4) // three darts of 1d4+1
	c := caster()
	target := dummy(30, 100) // an AC no attack roll would beat

	result, err := e.CastSpell(c, def(t, "Magic Missile"), 1, target, models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if len(result.Attacks) != 0 {
		t.Errorf("Magic Missile rolled %d attacks; it cannot miss", len(result.Attacks))
	}
	if result.Damage == nil {
		t.Fatal("Magic Missile dealt no damage against AC 30")
	}
	// Three darts of 1d4+1, all maximum: 3*(4+1) = 15.
	if result.Damage.Dealt != 15 {
		t.Errorf("dealt %d, want 15 from three maximum darts", result.Damage.Dealt)
	}
}

// A save spell asks the target to resist, and halves rather than negates when
// the spell says so.
func TestFireballHalvesOnASuccessfulSave(t *testing.T) {
	e := engine(21)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 3, Total: 2}}

	failed := dummy(10, 200)
	full, err := e.CastSpellVersusSave(c, def(t, "Fireball"), 3, failed, -20)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if full.Save == nil || full.Save.Succeeded() {
		t.Fatal("a -20 save modifier should fail")
	}

	c.Spells.RestoreAllSlots()
	saved := dummy(10, 200)
	half, err := e.CastSpellVersusSave(c, def(t, "Fireball"), 3, saved, 40)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if !half.Save.Succeeded() {
		t.Fatal("a +40 save modifier should succeed")
	}
	if half.Damage.Dealt >= full.Damage.Dealt {
		t.Errorf("a successful save took %d, a failure %d", half.Damage.Dealt, full.Damage.Dealt)
	}
	if half.Damage.Dealt == 0 {
		t.Error("Fireball halves on a save; it does not negate")
	}
}

// Hold Person deals no damage at all: the save is the whole mechanic, and the
// condition is what it produces.
func TestHoldPersonAppliesAConditionOnlyOnAFailure(t *testing.T) {
	e := engine(22)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 2, Total: 4}}

	failed := dummy(10, 50)
	result, err := e.CastSpellVersusSave(c, def(t, "Hold Person"), 2, failed, -20)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if result.Save.Succeeded() {
		t.Fatal("a -20 modifier should fail the save")
	}
	if !result.ConditionApplied || result.Condition != models.ConditionParalyzed {
		t.Errorf("condition = %q applied=%v, want paralyzed applied", result.Condition, result.ConditionApplied)
	}
	if !failed.HasCondition(models.ConditionParalyzed) {
		t.Error("the target is not actually paralyzed")
	}
	if result.Damage != nil {
		t.Errorf("Hold Person dealt %+v damage", result.Damage)
	}

	saved := dummy(10, 50)
	passed, err := e.CastSpellVersusSave(c, def(t, "Hold Person"), 2, saved, 40)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if passed.ConditionApplied || saved.HasCondition(models.ConditionParalyzed) {
		t.Error("a successful save still applied the condition")
	}
}

// An immune creature is not paralyzed, and the result has to say so or the
// narration will describe a frozen goblin that is still swinging.
func TestConditionImmunityIsHonoured(t *testing.T) {
	e := engine(23)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 2, Total: 4}}

	target := dummy(10, 50)
	target.ConditionImmunities = []models.Condition{models.ConditionParalyzed}

	result, err := e.CastSpellVersusSave(c, def(t, "Hold Person"), 2, target, -20)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if result.ConditionApplied {
		t.Error("an immune creature was reported as paralyzed")
	}
}

// Healing adds the caster's spellcasting modifier; damage never does.
func TestCureWoundsHealsAndAddsTheModifier(t *testing.T) {
	e := scripted(1) // the lowest possible 1d8
	c := caster()
	c.AbilityScores.Intelligence = 18 // +4
	c.Spells.SpellcastingAbility = models.AbilityIntelligence
	c.Spells.Slots = []models.SpellSlot{{Level: 1, Total: 4}}

	hurt := dummy(10, 40)
	hurt.HitPoints.Current = 5

	result, err := e.CastSpell(c, def(t, "Cure Wounds"), 1, hurt, models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	// 1 on the die plus a +4 modifier.
	if result.Healing != 5 {
		t.Errorf("healed %d, want 5 (1d8 rolled 1, plus +4)", result.Healing)
	}
	if hurt.HitPoints.Current != 10 {
		t.Errorf("target is at %d hit points, want 10", hurt.HitPoints.Current)
	}
	if result.Damage != nil {
		t.Error("a healing spell dealt damage")
	}
}

// Healing cannot exceed the maximum, and the reported number must be what was
// actually restored rather than what was rolled.
func TestHealingIsCappedAtTheMaximum(t *testing.T) {
	e := scripted(8)
	c := caster()
	c.Spells.SpellcastingAbility = models.AbilityIntelligence
	c.Spells.Slots = []models.SpellSlot{{Level: 1, Total: 4}}

	nearlyWell := dummy(10, 40)
	nearlyWell.HitPoints.Current = 38

	result, err := e.CastSpell(c, def(t, "Cure Wounds"), 1, nearlyWell, models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if nearlyWell.HitPoints.Current != 40 {
		t.Errorf("healed to %d, over the maximum of 40", nearlyWell.HitPoints.Current)
	}
	if result.Healing != 2 {
		t.Errorf("reported %d healed, want the 2 actually restored", result.Healing)
	}
}

// The slot has to be able to hold the spell, and a refusal must not cost one.
func TestCastingRefusesASlotTooSmallAndSpendsNothing(t *testing.T) {
	e := engine(24)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 2, Total: 3}}

	if _, err := e.CastSpellVersusSave(c, def(t, "Fireball"), 2, dummy(10, 50), 0); err == nil {
		t.Fatal("Fireball from a 2nd level slot should be refused")
	}
	if got := c.Spells.AvailableSlots(2); got != 3 {
		t.Errorf("a refused cast spent a slot: %d left of 3", got)
	}
}

func TestCastingWithoutSlotsIsRefused(t *testing.T) {
	e := engine(25)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 1, Total: 1, Expended: 1}}

	if _, err := e.CastSpell(c, def(t, "Magic Missile"), 1, dummy(10, 50), models.RollNormal); err == nil {
		t.Error("casting with no slots left should be refused")
	}
}

// A utility spell has nothing the engine can decide, and pretending otherwise
// is how the narrator ends up inventing mechanics.
func TestUtilitySpellsAreRefusedByTheEngine(t *testing.T) {
	e := engine(26)
	c := caster()

	_, err := e.CastSpell(c, def(t, "Mage Hand"), 0, dummy(10, 50), models.RollNormal)
	if err == nil {
		t.Fatal("the engine should refuse a spell it cannot resolve")
	}
	if !strings.Contains(err.Error(), "Mage Hand") {
		t.Errorf("the refusal does not name the spell: %v", err)
	}
}

// Using the wrong entry point must not silently do the wrong thing.
func TestSaveSpellsAndAttackSpellsUseTheirOwnPath(t *testing.T) {
	e := engine(27)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 3, Total: 2}}

	if _, err := e.CastSpell(c, def(t, "Fireball"), 3, dummy(10, 50), models.RollNormal); err == nil {
		t.Error("a save spell routed through CastSpell should be refused: it needs the target's save modifier")
	}
	if _, err := e.CastSpellVersusSave(c, def(t, "Fire Bolt"), 0, dummy(10, 50), 0); err == nil {
		t.Error("an attack spell routed through CastSpellVersusSave should be refused")
	}
}

// Facts are the contract with the narrator: every value it may mention, and
// none of them empty.
func TestCastFactsAreCompleteForNarration(t *testing.T) {
	e := scripted(faceCriticalHit, 6, 6)
	c := caster()

	result, err := e.CastSpell(c, def(t, "Fire Bolt"), 0, dummy(10, 40), models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	facts := result.Facts()
	for _, key := range []string{
		"caster", "spell", "target", "outcome", "damage_total", "damage_type",
		"target_hp", "target_status", "fact_summary", "slot_level", "projectiles",
	} {
		value, ok := facts[key]
		if !ok {
			t.Errorf("facts are missing %q", key)
			continue
		}
		if value == "" {
			t.Errorf("fact %q is empty", key)
		}
	}
	if !strings.Contains(facts["fact_summary"], "Fire Bolt") {
		t.Errorf("the summary does not name the spell: %q", facts["fact_summary"])
	}
}

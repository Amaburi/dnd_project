package rules

import (
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func shortbow() models.InventoryItem {
	return models.InventoryItem{
		ItemID: "w2", Key: "shortbow", Name: "Shortbow", Kind: models.ItemWeapon,
		Weapon: &models.WeaponProperties{
			Category: models.WeaponSimple, DamageDice: "1d6",
			DamageType:  models.DamagePiercing,
			Properties:  []models.WeaponProperty{models.PropertyAmmunition, models.PropertyTwoHanded},
			RangeNormal: 80, RangeLong: 320,
		},
	}
}

// helpless returns a target that should be trivially easy to hit.
func helpless(cond models.Condition, ac, hp int) *models.Combatant {
	c := dummy(ac, hp)
	c.Conditions = []models.Condition{cond}
	return c
}

// The whole point: the conditions were recorded and no roll ever read them.
// A paralysed target must actually be easier to hit.
func TestWeaponAttackReadsTheTargetsCondition(t *testing.T) {
	// Two dice are rolled under advantage; a 2 and a 19 against AC 15 hits
	// only if the higher one is kept.
	e := scripted(2, 19, 5)
	attacker := rogue()
	target := helpless(models.ConditionParalyzed, 15, 60)

	attack, err := e.WeaponAttack(attacker, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if len(attack.Roll.Rolls) != 2 {
		t.Fatalf("rolled %d dice, want two: a paralysed target grants advantage", len(attack.Roll.Rolls))
	}
	if !attack.Hit() {
		t.Errorf("kept the wrong die: %v -> %d", attack.Roll.Rolls, attack.Roll.Natural)
	}
}

// A hit on a paralysed creature from within reach is a critical however the
// die landed. It is most of what paralysis is for.
func TestMeleeHitsOnAParalysedTargetAreCritical(t *testing.T) {
	// 19 and 19 under advantage, then damage dice.
	e := scripted(19, 19, 4, 4, 4)
	attacker := rogue()
	target := helpless(models.ConditionParalyzed, 10, 60)

	attack, err := e.WeaponAttack(attacker, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if attack.Outcome != models.AttackCritical {
		t.Errorf("outcome = %s, want a critical against a paralysed target", attack.Outcome)
	}
	if attack.Damage == nil || !attack.Damage.Critical {
		t.Error("the damage was not rolled as a critical")
	}
}

// A natural 1 still misses. An automatic critical applies to hits, not to
// everything.
func TestANaturalOneStillMissesAHelplessTarget(t *testing.T) {
	e := scripted(1, 1)
	attack, err := e.WeaponAttack(rogue(), rapier(),
		helpless(models.ConditionParalyzed, 10, 60), models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if attack.Hit() {
		t.Errorf("a natural 1 hit: %s", attack.Summary())
	}
}

// A ranged attack on a helpless creature is easier, but not a free critical.
func TestRangedAttacksOnTheHelplessDoNotAutoCrit(t *testing.T) {
	e := scripted(15, 15, 3)
	attacker := rogue()
	target := helpless(models.ConditionUnconscious, 10, 60)

	attack, err := e.WeaponAttack(attacker, shortbow(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if !attack.Hit() {
		t.Fatalf("the attack missed: %s", attack.Summary())
	}
	if attack.Outcome == models.AttackCritical {
		t.Error("a bowshot at an unconscious target should not be an automatic critical")
	}
}

// The caller's own situational mode still combines, and never stacks.
func TestSituationalModeCombinesWithDerivedMode(t *testing.T) {
	// Advantage from the target and disadvantage from the situation cancel,
	// so exactly one die is rolled.
	e := scripted(12, 5)
	attack, err := e.WeaponAttack(rogue(), rapier(),
		helpless(models.ConditionParalyzed, 10, 60), models.RollDisadvantage)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if len(attack.Roll.Rolls) != 1 {
		t.Errorf("rolled %d dice, want one: the two sources should cancel", len(attack.Roll.Rolls))
	}
}

// Spell attacks are attacks: the same target conditions apply.
func TestSpellAttacksReadTheTargetsCondition(t *testing.T) {
	e := scripted(2, 18, 6)
	c := caster()
	target := helpless(models.ConditionRestrained, 15, 60)

	def, _ := models.SpellByName("Fire Bolt")
	result, err := e.CastSpell(c, def, 0, target, models.RollNormal)
	if err != nil {
		t.Fatalf("CastSpell: %v", err)
	}
	if len(result.Attacks) != 1 {
		t.Fatalf("made %d attacks, want one", len(result.Attacks))
	}
	if len(result.Attacks[0].Roll.Rolls) != 2 {
		t.Errorf("rolled %d dice, want two: a restrained target grants advantage",
			len(result.Attacks[0].Roll.Rolls))
	}
}

// This is what makes Hold Person worth the slot: the target stops resisting
// anything physical, so the Fireball that follows lands in full.
func TestAHelplessTargetAutomaticallyFailsDexteritySaves(t *testing.T) {
	e := engine(41)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 3, Total: 4}}

	def, _ := models.SpellByName("Fireball")

	// A save modifier so large it could not fail if it were rolled at all.
	target := helpless(models.ConditionParalyzed, 10, 300)
	result, err := e.CastSpellVersusSave(c, def, 3, target, 40)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if result.Save.Succeeded() {
		t.Fatal("a paralysed creature passed a Dexterity save")
	}
	if !result.Save.AutomaticFailure {
		t.Error("the result does not report the save as automatic")
	}

	// A Constitution save is still rolled: paralysis does not stop you
	// resisting poison.
	poisoned := helpless(models.ConditionParalyzed, 10, 300)
	shatter, _ := models.SpellByName("Shatter")
	conSave, err := e.CastSpellVersusSave(c, shatter, 3, poisoned, 40)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if conSave.Save.AutomaticFailure {
		t.Error("a Constitution save was auto-failed")
	}
	if !conSave.Save.Succeeded() {
		t.Error("a +40 Constitution save should succeed")
	}
}

// A save that was never rolled must not be reported as a roll of zero. The
// narrator repeats these facts faithfully, so an invented die is an invented
// scene: "the goblin rolled a 0" describes something that did not happen.
func TestAnAutomaticFailureIsNotReportedAsARollOfZero(t *testing.T) {
	e := engine(42)
	c := caster()
	c.Spells.Slots = []models.SpellSlot{{Level: 3, Total: 4}}

	def, _ := models.SpellByName("Fireball")
	result, err := e.CastSpellVersusSave(c, def, 3, helpless(models.ConditionParalyzed, 10, 300), 5)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}

	summary := result.Save.Summary()
	if strings.Contains(summary, "rolled") {
		t.Errorf("the summary describes a roll that never happened: %q", summary)
	}
	if !strings.Contains(summary, "automatically") {
		t.Errorf("the summary does not say the failure was automatic: %q", summary)
	}

	facts := result.Save.Facts()
	for _, key := range []string{"natural", "all_rolls", "total", "roll_mode", "outcome", "fact_summary"} {
		if facts[key] == "" {
			t.Errorf("fact %q is empty", key)
		}
	}
	if facts["natural"] == "0" || facts["all_rolls"] == "0" {
		t.Errorf("facts report a natural 0: natural=%q all_rolls=%q", facts["natural"], facts["all_rolls"])
	}
	// "yes"/"no" is the vocabulary the rest of the facts already use.
	if facts["automatic_failure"] != "yes" {
		t.Errorf("automatic_failure = %q, want yes", facts["automatic_failure"])
	}

	// An ordinary save still reports its dice.
	rolled := result
	rolled, err = e.CastSpellVersusSave(c, def, 3, dummy(10, 300), 5)
	if err != nil {
		t.Fatalf("CastSpellVersusSave: %v", err)
	}
	if rolled.Save.Facts()["automatic_failure"] != "no" {
		t.Error("an ordinary save was marked automatic")
	}
	if !strings.Contains(rolled.Save.Summary(), "rolled") {
		t.Errorf("an ordinary save does not report its roll: %q", rolled.Save.Summary())
	}
}

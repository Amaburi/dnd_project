package models

import (
	"strings"
	"testing"
)

func rester() *Character {
	c := &Character{
		Name: "Thistle", Type: CharacterPlayer,
		BasicInfo: BasicInfo{
			Race: RaceHuman, Subrace: "standard", Background: BackgroundSoldier,
			Classes: []ClassLevel{{Class: ClassFighter, Subclass: "champion", Level: 6}},
		},
		AbilityScores: AbilityScores{
			Strength: 16, Dexterity: 14, Constitution: 14,
			Intelligence: 10, Wisdom: 12, Charisma: 10,
		},
	}
	c.ApplyClassDefaults()
	c.CombatStats.HitPoints = HitPoints{Current: 10, Maximum: 50}
	c.CombatStats.HitDice = HitDice{{Die: 10, Total: 6, Spent: 4}}
	return c
}

// PHB: a character must have at least one hit point at the start of a long
// rest to gain any benefit from it. Without this, a downed party could sleep
// their way out of a lost fight.
func TestALongRestNeedsAtLeastOneHitPoint(t *testing.T) {
	down := rester()
	down.CombatStats.HitPoints.Current = 0

	ok, reason := down.CanBenefitFromLongRest()
	if ok {
		t.Fatal("a character at 0 hit points benefited from a long rest")
	}
	if !strings.Contains(reason, "1 hit point") {
		t.Errorf("reason = %q, want it to cite the one hit point rule", reason)
	}

	up := rester()
	if ok, reason := up.CanBenefitFromLongRest(); !ok {
		t.Errorf("a conscious character was refused a long rest: %q", reason)
	}
}

func TestTheDeadDoNotRest(t *testing.T) {
	dead := rester()
	dead.Exhaustion = MaxExhaustion

	if ok, reason := dead.CanBenefitFromLongRest(); ok || !strings.Contains(strings.ToLower(reason), "dead") {
		t.Errorf("CanBenefitFromLongRest = (%v, %q), want a refusal naming death", ok, reason)
	}
	if ok, _ := dead.CanBenefitFromShortRest(); ok {
		t.Error("a dead character benefited from a short rest")
	}
}

// A short rest is where hit dice are spent, so being at 0 hit points does not
// block it -- an ally can stabilise you and you can then spend dice.
func TestAShortRestIsAllowedWhileDown(t *testing.T) {
	down := rester()
	down.CombatStats.HitPoints.Current = 0

	if ok, reason := down.CanBenefitFromShortRest(); !ok {
		t.Errorf("a downed character was refused a short rest: %q", reason)
	}
}

// A long rest is the whole resource economy coming back at once.
func TestLongRestRestoresTheResources(t *testing.T) {
	c := rester()
	c.Spells.Slots = []SpellSlot{{Level: 1, Total: 4, Expended: 4}}
	c.Exhaustion = 3

	c.LongRest()

	if c.CombatStats.HitPoints.Current != c.EffectiveHitPointMaximum() {
		t.Errorf("hit points = %d, want the maximum %d",
			c.CombatStats.HitPoints.Current, c.EffectiveHitPointMaximum())
	}
	if c.Spells.AvailableSlots(1) != 4 {
		t.Errorf("%d slots restored, want all 4", c.Spells.AvailableSlots(1))
	}
	if c.Exhaustion != 2 {
		t.Errorf("exhaustion = %d, want one level removed", c.Exhaustion)
	}
	// Half the total, rounded down: 6 dice means 3 back, on top of the 2 left.
	if got := c.CombatStats.HitDice.Available(); got != 5 {
		t.Errorf("%d hit dice available, want 5", got)
	}
}

// Spending a hit die heals for the roll plus Constitution, and cannot be done
// with a die the character has already spent.
func TestSpendingHitDiceIsBounded(t *testing.T) {
	c := rester()
	c.CombatStats.HitDice = HitDice{{Die: 10, Total: 6, Spent: 5}}

	if err := c.SpendHitDie(10, 6); err != nil {
		t.Fatalf("SpendHitDie: %v", err)
	}
	// 6 rolled plus a +2 Constitution modifier.
	if c.CombatStats.HitPoints.Current != 18 {
		t.Errorf("hit points = %d, want 18", c.CombatStats.HitPoints.Current)
	}

	if err := c.SpendHitDie(10, 6); err == nil {
		t.Error("spending a hit die the character does not have should fail")
	}
	if err := c.SpendHitDie(8, 4); err == nil {
		t.Error("spending a die size the character has no pool of should fail")
	}
}

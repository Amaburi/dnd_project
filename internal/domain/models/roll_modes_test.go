package models

import "testing"

func defender(conditions ...Condition) *Combatant {
	return &Combatant{
		Name: "Goblin", Status: CombatantActive, ArmorClass: 15,
		HitPoints:  HitPoints{Current: 7, Maximum: 7},
		Conditions: conditions,
	}
}

// A helpless target is the single biggest swing in 5e combat, and none of it
// was reachable: conditions were stored and never read by any roll.
func TestTargetConditionsGiveTheAttackerAdvantage(t *testing.T) {
	for _, cond := range []Condition{
		ConditionBlinded, ConditionParalyzed, ConditionPetrified,
		ConditionRestrained, ConditionStunned, ConditionUnconscious,
	} {
		if got := defender(cond).DefenderAttackMode(true); got != RollAdvantage {
			t.Errorf("melee against a %s target = %s, want advantage", cond, got)
		}
		if got := defender(cond).DefenderAttackMode(false); got != RollAdvantage {
			t.Errorf("ranged against a %s target = %s, want advantage", cond, got)
		}
	}
}

// Prone is the one condition that cuts both ways: easier to hit up close,
// harder from across the room.
func TestProneDependsOnRange(t *testing.T) {
	if got := defender(ConditionProne).DefenderAttackMode(true); got != RollAdvantage {
		t.Errorf("melee against a prone target = %s, want advantage", got)
	}
	if got := defender(ConditionProne).DefenderAttackMode(false); got != RollDisadvantage {
		t.Errorf("ranged against a prone target = %s, want disadvantage", got)
	}
}

func TestAnInvisibleTargetIsHarderToHit(t *testing.T) {
	if got := defender(ConditionInvisible).DefenderAttackMode(true); got != RollDisadvantage {
		t.Errorf("attacking an invisible target = %s, want disadvantage", got)
	}
}

// Conditions that change nothing about being attacked must change nothing.
func TestHarmlessTargetConditionsChangeNothing(t *testing.T) {
	for _, cond := range []Condition{
		ConditionCharmed, ConditionDeafened, ConditionFrightened,
		ConditionGrappled, ConditionIncapacitated, ConditionPoisoned,
	} {
		if got := defender(cond).DefenderAttackMode(true); got != RollNormal {
			t.Errorf("attacking a %s target = %s, want a normal roll", cond, got)
		}
	}
	if got := defender().DefenderAttackMode(true); got != RollNormal {
		t.Errorf("attacking an unafflicted target = %s, want normal", got)
	}
}

// Advantage and disadvantage never stack, and one of each cancels: a blind
// attacker swinging at a prone target rolls straight.
func TestOpposingSourcesCancelRatherThanStack(t *testing.T) {
	both := defender(ConditionProne, ConditionInvisible)
	if got := both.DefenderAttackMode(true); got != RollNormal {
		t.Errorf("prone and invisible = %s, want them to cancel to normal", got)
	}
}

func afflicted(conditions ...Condition) *Character {
	c := &Character{
		Name: "Thistle", Type: CharacterPlayer, Conditions: conditions,
		CombatStats: CombatStats{HitPoints: HitPoints{Current: 20, Maximum: 20}, Speed: 30},
	}
	return c
}

func TestAttackerConditionsAffectTheirOwnAttacks(t *testing.T) {
	cases := map[Condition]RollMode{
		ConditionBlinded:    RollDisadvantage,
		ConditionFrightened: RollDisadvantage,
		ConditionPoisoned:   RollDisadvantage,
		ConditionProne:      RollDisadvantage,
		ConditionRestrained: RollDisadvantage,
		ConditionInvisible:  RollAdvantage,

		// These cost you actions, not accuracy.
		ConditionCharmed:  RollNormal,
		ConditionDeafened: RollNormal,
		ConditionGrappled: RollNormal,
	}
	for cond, want := range cases {
		if got := afflicted(cond).AttackRollMode(InventoryItem{}); got != want {
			t.Errorf("a %s attacker rolls %s, want %s", cond, got, want)
		}
	}
}

// Poison and fear cloud judgement as well as aim.
func TestAttackerConditionsAffectAbilityChecks(t *testing.T) {
	for _, cond := range []Condition{ConditionFrightened, ConditionPoisoned} {
		if got := afflicted(cond).SkillRollMode(SkillAthletics); got != RollDisadvantage {
			t.Errorf("a %s character checks at %s, want disadvantage", cond, got)
		}
	}
	if got := afflicted(ConditionDeafened).SkillRollMode(SkillAthletics); got != RollNormal {
		t.Errorf("a deafened character checks at %s, want normal", got)
	}
}

// A spell attack is still an attack: the same conditions apply.
func TestSpellAttacksUseTheSameAttackerConditions(t *testing.T) {
	if got := afflicted(ConditionPoisoned).SpellAttackRollMode(); got != RollDisadvantage {
		t.Errorf("a poisoned caster rolls %s, want disadvantage", got)
	}
	if got := afflicted().SpellAttackRollMode(); got != RollNormal {
		t.Errorf("an unafflicted caster rolls %s, want normal", got)
	}
}

// Restrained is disadvantage on Dexterity saves specifically, not on all saves.
func TestRestrainedHampersDexteritySavesOnly(t *testing.T) {
	held := afflicted(ConditionRestrained)
	if got := held.SavingThrowRollMode(AbilityDexterity); got != RollDisadvantage {
		t.Errorf("a restrained Dexterity save is %s, want disadvantage", got)
	}
	if got := held.SavingThrowRollMode(AbilityWisdom); got != RollNormal {
		t.Errorf("a restrained Wisdom save is %s, want normal", got)
	}
}

// This is what makes Hold Person worth casting: the target stops saving
// against anything physical at all.
func TestHelplessCreaturesAutomaticallyFailPhysicalSaves(t *testing.T) {
	for _, cond := range []Condition{
		ConditionParalyzed, ConditionPetrified, ConditionStunned, ConditionUnconscious,
	} {
		target := defender(cond)
		for _, ability := range []Ability{AbilityStrength, AbilityDexterity} {
			if !target.AutoFailsSave(ability) {
				t.Errorf("a %s creature should auto-fail its %s save", cond, ability)
			}
		}
		for _, ability := range []Ability{
			AbilityConstitution, AbilityIntelligence, AbilityWisdom, AbilityCharisma,
		} {
			if target.AutoFailsSave(ability) {
				t.Errorf("a %s creature should still roll its %s save", cond, ability)
			}
		}
	}

	// Merely incapacitated is not helpless.
	if defender(ConditionIncapacitated).AutoFailsSave(AbilityDexterity) {
		t.Error("an incapacitated creature still rolls Dexterity saves")
	}
	if defender().AutoFailsSave(AbilityDexterity) {
		t.Error("an unafflicted creature auto-failed a save")
	}
}

// A hit on a helpless creature from within reach is a critical, whatever the
// die showed. It is most of what paralysis is for.
func TestMeleeHitsOnTheHelplessAreCritical(t *testing.T) {
	for _, cond := range []Condition{ConditionParalyzed, ConditionUnconscious} {
		if !defender(cond).AutoCriticalOnHit(true) {
			t.Errorf("a melee hit on a %s creature should be a critical", cond)
		}
		// From further than five feet it is an ordinary hit.
		if defender(cond).AutoCriticalOnHit(false) {
			t.Errorf("a ranged hit on a %s creature should not auto-crit", cond)
		}
	}

	// Petrified and stunned grant advantage but not automatic criticals.
	for _, cond := range []Condition{ConditionPetrified, ConditionStunned, ConditionRestrained} {
		if defender(cond).AutoCriticalOnHit(true) {
			t.Errorf("a hit on a %s creature should not auto-crit", cond)
		}
	}
}

// A creature at 0 hit points is unconscious whether or not anyone has written
// the condition down, so the status has to count too.
func TestDyingCombatantsCountAsHelpless(t *testing.T) {
	dying := &Combatant{Name: "Thistle", Status: CombatantDying,
		HitPoints: HitPoints{Current: 0, Maximum: 20}}

	if got := dying.DefenderAttackMode(true); got != RollAdvantage {
		t.Errorf("attacking a dying creature = %s, want advantage", got)
	}
	if !dying.AutoCriticalOnHit(true) {
		t.Error("a melee hit on a dying creature should be a critical")
	}
	if !dying.AutoFailsSave(AbilityDexterity) {
		t.Error("a dying creature should auto-fail Dexterity saves")
	}
}

// The parser may nudge the roll, but only from a closed list of real
// circumstances -- a free-text reason would be a lever for talking the
// narrator into advantage.
func TestAdvantageReasonsAreAClosedList(t *testing.T) {
	cases := map[AdvantageReason]RollMode{
		ReasonNone:           RollNormal,
		ReasonAttackerUnseen: RollAdvantage,
		ReasonAllyHelping:    RollAdvantage,
		ReasonTargetUnseen:   RollDisadvantage,
		ReasonAwkward:        RollDisadvantage,
	}
	for reason, want := range cases {
		if got := reason.Mode(); got != want {
			t.Errorf("%q grants %s, want %s", reason, got, want)
		}
		if !reason.Valid() {
			t.Errorf("%q is not accepted as a reason", reason)
		}
	}

	if invented := AdvantageReason("i_am_just_really_good"); invented.Valid() {
		t.Error("an invented reason was accepted")
	}
	if got := AdvantageReason("i_am_just_really_good").Mode(); got != RollNormal {
		t.Errorf("an invented reason granted %s, want a normal roll", got)
	}
}

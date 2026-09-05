package models

import (
	"strings"
	"testing"
)

// The three capabilities are separate in 5e and conflating them is its own
// bug: an incapacitated creature can still speak and still walk, a grappled
// one can still swing, and only some conditions silence you.
func TestConditionsPreventTheRightThings(t *testing.T) {
	cases := []struct {
		condition                Condition
		action, movement, speech bool // true = prevented
	}{
		{ConditionIncapacitated, true, false, false},
		{ConditionParalyzed, true, true, true},
		{ConditionPetrified, true, true, true},
		{ConditionStunned, true, true, false}, // speaks, but only falteringly
		{ConditionUnconscious, true, true, true},

		{ConditionGrappled, false, true, false},
		{ConditionRestrained, false, true, false},

		{ConditionBlinded, false, false, false},
		{ConditionCharmed, false, false, false},
		{ConditionDeafened, false, false, false},
		{ConditionFrightened, false, false, false},
		{ConditionInvisible, false, false, false},
		{ConditionPoisoned, false, false, false},
		{ConditionProne, false, false, false},
	}

	for _, tc := range cases {
		if got := tc.condition.PreventsAction(); got != tc.action {
			t.Errorf("%s.PreventsAction() = %v, want %v", tc.condition, got, tc.action)
		}
		if got := tc.condition.PreventsMovement(); got != tc.movement {
			t.Errorf("%s.PreventsMovement() = %v, want %v", tc.condition, got, tc.movement)
		}
		if got := tc.condition.PreventsSpeech(); got != tc.speech {
			t.Errorf("%s.PreventsSpeech() = %v, want %v", tc.condition, got, tc.speech)
		}
	}

	// Every condition in the closed set must have been considered above.
	if len(cases) != len(Conditions) {
		t.Errorf("%d of %d conditions are covered", len(cases), len(Conditions))
	}
}

// Anything that prevents action also has to be a real condition, or a typo in
// the list silently stops preventing anything.
func TestPreventionListsHoldOnlyRealConditions(t *testing.T) {
	for name, list := range map[string][]Condition{
		"action":   ActionPreventingConditions,
		"movement": MovementPreventingConditions,
		"speech":   SpeechPreventingConditions,
	} {
		if len(list) == 0 {
			t.Errorf("the %s prevention list is empty", name)
		}
		for _, c := range list {
			if !c.Valid() {
				t.Errorf("the %s list holds %q, which is not a condition", name, c)
			}
		}
	}
}

func standing() *Character {
	c := &Character{
		Name: "Thistle", Type: CharacterPlayer,
		CombatStats: CombatStats{HitPoints: HitPoints{Current: 24, Maximum: 24}, Speed: 30},
	}
	return c
}

// The bug this closes: a character at 0 hit points could take a turn. In 5e
// they are unconscious and rolling to survive.
func TestACharacterAtZeroHitPointsCannotAct(t *testing.T) {
	down := standing()
	down.CombatStats.HitPoints.Current = 0

	ok, reason := down.CanAct()
	if ok {
		t.Fatal("a character at 0 hit points was allowed to act")
	}
	if !strings.Contains(strings.ToLower(reason), "unconscious") {
		t.Errorf("reason = %q, want it to say they are unconscious", reason)
	}
	if !down.IsDying() {
		t.Error("a character at 0 hit points and not stable should be dying")
	}

	// Stabilising does not wake anyone up.
	down.CombatStats.DeathSaves = DeathSaves{Successes: 3}
	if ok, _ := down.CanAct(); ok {
		t.Error("a stabilised character is still unconscious and cannot act")
	}
}

func TestAHealthyCharacterCanAct(t *testing.T) {
	ok, reason := standing().CanAct()
	if !ok {
		t.Errorf("a healthy character was blocked: %q", reason)
	}
	if reason != "" {
		t.Errorf("no reason should be given when the character can act, got %q", reason)
	}
}

func TestDeathEndsEverything(t *testing.T) {
	byFailures := standing()
	byFailures.CombatStats.HitPoints.Current = 0
	byFailures.CombatStats.DeathSaves = DeathSaves{Failures: 3}
	if !byFailures.IsDead() {
		t.Error("three failed death saves should be death")
	}
	if ok, reason := byFailures.CanAct(); ok || !strings.Contains(strings.ToLower(reason), "dead") {
		t.Errorf("CanAct = (%v, %q), want a refusal naming death", ok, reason)
	}

	// The sixth level of exhaustion is death, and it is the one level that is
	// not merely a penalty.
	byExhaustion := standing()
	byExhaustion.Exhaustion = 6
	if !byExhaustion.IsDead() {
		t.Error("exhaustion 6 should be death")
	}
	if ok, _ := byExhaustion.CanAct(); ok {
		t.Error("a character dead of exhaustion was allowed to act")
	}

	// Five levels is crippling but survivable, and they can still act.
	byFive := standing()
	byFive.Exhaustion = 5
	if byFive.IsDead() {
		t.Error("exhaustion 5 is not death")
	}
	if ok, reason := byFive.CanAct(); !ok {
		t.Errorf("exhaustion 5 blocked action: %q", reason)
	}
}

func TestIncapacitatingConditionsBlockACharacter(t *testing.T) {
	for _, cond := range []Condition{
		ConditionIncapacitated, ConditionParalyzed, ConditionPetrified,
		ConditionStunned, ConditionUnconscious,
	} {
		c := standing()
		c.Conditions = []Condition{cond}
		ok, reason := c.CanAct()
		if ok {
			t.Errorf("a %s character was allowed to act", cond)
		}
		if !strings.Contains(reason, string(cond)) {
			t.Errorf("reason %q does not name the condition %q", reason, cond)
		}
	}

	// Being held is not being helpless: a grappled character still fights.
	held := standing()
	held.Conditions = []Condition{ConditionGrappled, ConditionProne}
	if ok, reason := held.CanAct(); !ok {
		t.Errorf("a grappled and prone character was blocked from acting: %q", reason)
	}
	if ok, _ := held.CanMove(); ok {
		t.Error("a grappled character should not be able to move")
	}
}

// Speech is its own capability. An incapacitated character can still talk;
// a paralysed one cannot.
func TestSpeechIsSeparateFromAction(t *testing.T) {
	talking := standing()
	talking.Conditions = []Condition{ConditionIncapacitated}
	if ok, _ := talking.CanAct(); ok {
		t.Error("an incapacitated character should not act")
	}
	if ok, reason := talking.CanSpeak(); !ok {
		t.Errorf("an incapacitated character can still speak, but was blocked: %q", reason)
	}

	silent := standing()
	silent.Conditions = []Condition{ConditionParalyzed}
	if ok, _ := silent.CanSpeak(); ok {
		t.Error("a paralysed character cannot speak")
	}

	unconscious := standing()
	unconscious.CombatStats.HitPoints.Current = 0
	if ok, _ := unconscious.CanSpeak(); ok {
		t.Error("an unconscious character cannot speak")
	}
}

// The combatant is the shared representation, so it needs the same answer or
// a monster and a character disagree about the same condition.
func TestCombatantCanActMatchesTheCharacterRule(t *testing.T) {
	held := &Combatant{Name: "Goblin", Status: CombatantActive,
		HitPoints: HitPoints{Current: 7, Maximum: 7}}
	if ok, reason := held.CanAct(); !ok {
		t.Errorf("a healthy combatant was blocked: %q", reason)
	}

	held.Conditions = []Condition{ConditionParalyzed}
	if ok, reason := held.CanAct(); ok {
		t.Errorf("a paralysed combatant was allowed to act (%q)", reason)
	}

	down := &Combatant{Name: "Goblin", Status: CombatantDying}
	if ok, _ := down.CanAct(); ok {
		t.Error("a dying combatant was allowed to act")
	}

	dead := &Combatant{Name: "Goblin", Status: CombatantDead}
	if ok, reason := dead.CanAct(); ok || !strings.Contains(strings.ToLower(reason), "dead") {
		t.Errorf("CanAct = (%v, %q), want a refusal naming death", ok, reason)
	}
}

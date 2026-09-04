package models

// RollMode is whether a d20 roll is made with advantage or disadvantage.
//
// This is the signature 5e mechanic and it cannot be represented by a single
// result value: two dice are rolled and one is kept, and a log that shows only
// the kept die hides why the outcome was what it was.
type RollMode string

const (
	RollNormal       RollMode = "normal"
	RollAdvantage    RollMode = "advantage"
	RollDisadvantage RollMode = "disadvantage"
)

// Combine folds another source of advantage or disadvantage into a mode.
//
// 5e does not stack these: any number of sources of advantage still grants one
// advantage, and a single source of the opposite cancels it to a normal roll.
func (m RollMode) Combine(other RollMode) RollMode {
	switch {
	case m == other:
		return m
	case m == RollNormal:
		return other
	case other == RollNormal:
		return m
	default:
		// One of each: they cancel.
		return RollNormal
	}
}

// D20Result is the outcome of a single d20 roll.
type D20Result struct {
	Mode RollMode `json:"mode" bson:"mode"`

	// Rolls holds every d20 rolled: one normally, two under advantage or
	// disadvantage.
	Rolls []int `json:"rolls" bson:"rolls"`

	// Natural is the die that was kept, before modifiers.
	Natural  int `json:"natural" bson:"natural"`
	Modifier int `json:"modifier" bson:"modifier"`
	Total    int `json:"total" bson:"total"`
}

// IsNatural20 reports whether the kept die showed a 20.
func (r D20Result) IsNatural20() bool { return r.Natural == 20 }

// IsNatural1 reports whether the kept die showed a 1.
func (r D20Result) IsNatural1() bool { return r.Natural == 1 }

// CheckOutcome is the result of comparing a roll against a difficulty class.
type CheckOutcome string

const (
	OutcomeSuccess CheckOutcome = "success"
	OutcomeFailure CheckOutcome = "failure"
)

// AttackOutcome is the result of an attack roll against an armor class.
//
// Attacks have two outcomes ability checks do not: a natural 20 always hits
// and scores a critical, and a natural 1 always misses, regardless of the
// modifiers or the target's AC.
type AttackOutcome string

const (
	AttackHit      AttackOutcome = "hit"
	AttackMiss     AttackOutcome = "miss"
	AttackCritical AttackOutcome = "critical_hit"
	AttackFumble   AttackOutcome = "critical_miss"
)

// Hit reports whether the attack connected, critical or not.
func (o AttackOutcome) Hit() bool {
	return o == AttackHit || o == AttackCritical
}

// ResolveCheck compares a d20 result against a DC.
//
// Note that a natural 20 or 1 has no special meaning on an ability check or
// saving throw in 5e; only attack rolls auto-hit and auto-miss. Treating them
// as automatic here is a common house rule -- if this campaign wants it, make
// that an explicit decision rather than an accident of the resolver.
func ResolveCheck(roll D20Result, dc int) CheckOutcome {
	if roll.Total >= dc {
		return OutcomeSuccess
	}
	return OutcomeFailure
}

// NaturalCrit is the natural d20 roll that scores a critical hit for anyone
// without a feature that widens the range.
const NaturalCrit = 20

// ResolveAttack compares an attack roll against a target's armor class.
//
// critRange is the lowest natural roll that scores a critical: 20 for most
// characters, but 19 for a Champion fighter from level 3 and 18 from 15. Pass
// NaturalCrit when the attacker has no such feature -- hardcoding 20 here
// silently denied Champions the one thing their archetype does.
func ResolveAttack(roll D20Result, targetAC, critRange int) AttackOutcome {
	if critRange < 1 || critRange > NaturalCrit {
		critRange = NaturalCrit
	}

	switch {
	case roll.IsNatural1():
		return AttackFumble
	case roll.Natural >= critRange:
		// A roll in the critical range always hits, whatever the AC.
		return AttackCritical
	case roll.Total >= targetAC:
		return AttackHit
	default:
		return AttackMiss
	}
}

// DiceResults contains dice roll information recorded on a story event.
type DiceResults struct {
	RollType string  `json:"roll_type" bson:"roll_type"` // "ability_check", "saving_throw", "attack", "damage"
	Skill    Skill   `json:"skill,omitempty" bson:"skill,omitempty"`
	Ability  Ability `json:"ability,omitempty" bson:"ability,omitempty"`

	Roll D20Result `json:"roll" bson:"roll"`

	DC      int    `json:"dc" bson:"dc"`
	Outcome string `json:"outcome" bson:"outcome"`
}

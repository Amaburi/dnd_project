package models

// Advantage and disadvantage from the situation.
//
// The conditions were stored and never read: a paralysed goblin was attacked
// at a flat roll, which is most of what paralysis is for. These functions are
// the missing half -- they turn recorded state into the roll it should produce.
//
// Everything here combines through RollMode.Combine, which never stacks and
// cancels opposites. That is real 5e: any number of sources of advantage is
// still one advantage, and a single source of the other makes it a straight
// roll.

// Conditions that make a creature easier to hit, whatever the range.
var easierToHitConditions = []Condition{
	ConditionBlinded, ConditionParalyzed, ConditionPetrified,
	ConditionRestrained, ConditionStunned, ConditionUnconscious,
}

// Conditions that spoil the attacker's own aim.
var clumsyAttackerConditions = []Condition{
	ConditionBlinded, ConditionFrightened, ConditionPoisoned,
	ConditionProne, ConditionRestrained,
}

// Conditions that cloud judgement as well as aim.
//
// Blinded is deliberately absent: RAW it makes a creature fail checks that
// *require sight*, and nothing here knows which checks those are. Guessing
// would be a penalty the rules do not impose.
var clumsyCheckConditions = []Condition{ConditionFrightened, ConditionPoisoned}

// Conditions under which a creature stops resisting anything physical.
var helplessConditions = []Condition{
	ConditionParalyzed, ConditionPetrified, ConditionStunned, ConditionUnconscious,
}

// Conditions under which a hit from close quarters is automatically critical.
//
// Only these two: petrified and stunned grant advantage but not free criticals.
var autoCriticalConditions = []Condition{ConditionParalyzed, ConditionUnconscious}

func anyCondition(held []Condition, set []Condition) bool {
	for _, c := range held {
		if inConditions(set, c) {
			return true
		}
	}
	return false
}

// helpless folds the conditions together with being down, because a creature
// at 0 hit points is unconscious whether or not anyone wrote the condition on
// the sheet.
func (c *Combatant) helpless() bool {
	return c.IsDown() || anyCondition(c.Conditions, helplessConditions)
}

// DefenderAttackMode is the advantage or disadvantage an attack against this
// creature gets from the creature's own state.
//
// melee says whether the attacker is swinging rather than shooting, which is
// the one thing prone needs to know: a creature on the ground is easier to
// reach and harder to shoot.
func (c *Combatant) DefenderAttackMode(melee bool) RollMode {
	mode := RollNormal

	if c.IsDown() || anyCondition(c.Conditions, easierToHitConditions) {
		mode = mode.Combine(RollAdvantage)
	}
	if c.HasCondition(ConditionProne) {
		if melee {
			mode = mode.Combine(RollAdvantage)
		} else {
			mode = mode.Combine(RollDisadvantage)
		}
	}
	if c.HasCondition(ConditionInvisible) {
		mode = mode.Combine(RollDisadvantage)
	}
	return mode
}

// AutoCriticalOnHit reports whether any hit on this creature is automatically
// a critical.
//
// withinFiveFeet is required: a reach weapon at ten feet, or an arrow, hits a
// paralysed creature normally. The rule is about being close enough to place
// the blow, not about the target alone.
func (c *Combatant) AutoCriticalOnHit(withinFiveFeet bool) bool {
	if !withinFiveFeet {
		return false
	}
	return c.IsDown() || anyCondition(c.Conditions, autoCriticalConditions)
}

// AutoFailsSave reports whether the creature fails this save without rolling.
//
// Strength and Dexterity only: a paralysed creature still resists poison and
// still keeps its wits.
func (c *Combatant) AutoFailsSave(a Ability) bool {
	if a != AbilityStrength && a != AbilityDexterity {
		return false
	}
	return c.helpless()
}

// SpellAttackRollMode is the caster's own advantage or disadvantage on a spell
// attack. A spell attack is still an attack, so the same conditions apply.
func (c *Character) SpellAttackRollMode() RollMode {
	mode := RollNormal
	if ExhaustionEffectsFor(c.Exhaustion).DisadvantageOnAttacksAndSaves {
		mode = mode.Combine(RollDisadvantage)
	}
	return mode.Combine(c.conditionAttackMode())
}

// conditionAttackMode is the attacker-side contribution of the character's own
// conditions.
//
// Frightened is applied unconditionally, though RAW it requires the source of
// the fear to be in sight. Nothing here tracks what a creature is afraid of,
// and a frightened character swinging at full accuracy is the further from the
// rules of the two approximations.
func (c *Character) conditionAttackMode() RollMode {
	mode := RollNormal
	if anyCondition(c.Conditions, clumsyAttackerConditions) {
		mode = mode.Combine(RollDisadvantage)
	}
	if c.HasCondition(ConditionInvisible) {
		mode = mode.Combine(RollAdvantage)
	}
	return mode
}

// AdvantageReason is a circumstance the parser may propose.
//
// It is a closed list on purpose. Letting a model grant advantage for free text
// would make "I attack with tremendous advantage from a hidden position" a
// mechanical claim rather than a description, which is exactly the lever the
// two-call design exists to remove. Everything derivable from recorded state is
// derived instead, and these four are the common circumstances that are not.
type AdvantageReason string

const (
	ReasonNone AdvantageReason = ""

	// ReasonAttackerUnseen is attacking from hiding or unseen.
	ReasonAttackerUnseen AdvantageReason = "attacker_unseen"

	// ReasonAllyHelping is the Help action.
	ReasonAllyHelping AdvantageReason = "ally_helping"

	// ReasonTargetUnseen is striking into darkness or at a target you cannot see.
	ReasonTargetUnseen AdvantageReason = "target_unseen"

	// ReasonAwkward is squeezing, poor footing, or a similarly hampered position.
	ReasonAwkward AdvantageReason = "awkward_position"
)

// AdvantageReasons lists every circumstance the parser may name.
var AdvantageReasons = []AdvantageReason{
	ReasonNone, ReasonAttackerUnseen, ReasonAllyHelping,
	ReasonTargetUnseen, ReasonAwkward,
}

// Valid reports whether r is a recognised reason.
func (r AdvantageReason) Valid() bool {
	for _, known := range AdvantageReasons {
		if r == known {
			return true
		}
	}
	return false
}

// Mode is the roll this reason produces. An unrecognised reason produces a
// normal roll rather than an error: a model that invents one should change
// nothing, not break the turn.
func (r AdvantageReason) Mode() RollMode {
	switch r {
	case ReasonAttackerUnseen, ReasonAllyHelping:
		return RollAdvantage
	case ReasonTargetUnseen, ReasonAwkward:
		return RollDisadvantage
	default:
		return RollNormal
	}
}

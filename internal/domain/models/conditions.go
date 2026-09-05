package models

// Condition is one of the fifteen D&D 5e conditions.
//
// This replaces the previous pair of free-string lists (status_effects and
// conditions), which overlapped with no rule about which was authoritative.
// Conditions are a closed set; anything outside it is narrative colour and
// belongs in a description, not here.
type Condition string

const (
	ConditionBlinded       Condition = "blinded"
	ConditionCharmed       Condition = "charmed"
	ConditionDeafened      Condition = "deafened"
	ConditionFrightened    Condition = "frightened"
	ConditionGrappled      Condition = "grappled"
	ConditionIncapacitated Condition = "incapacitated"
	ConditionInvisible     Condition = "invisible"
	ConditionParalyzed     Condition = "paralyzed"
	ConditionPetrified     Condition = "petrified"
	ConditionPoisoned      Condition = "poisoned"
	ConditionProne         Condition = "prone"
	ConditionRestrained    Condition = "restrained"
	ConditionStunned       Condition = "stunned"
	ConditionUnconscious   Condition = "unconscious"
)

// Conditions lists every condition tracked as a flag.
//
// Exhaustion is deliberately absent: it is the one condition with degrees
// (six levels of escalating penalty, reduced by one per long rest), so it is
// modelled as Character.Exhaustion rather than as membership in a list.
var Conditions = []Condition{
	ConditionBlinded, ConditionCharmed, ConditionDeafened, ConditionFrightened,
	ConditionGrappled, ConditionIncapacitated, ConditionInvisible,
	ConditionParalyzed, ConditionPetrified, ConditionPoisoned, ConditionProne,
	ConditionRestrained, ConditionStunned, ConditionUnconscious,
}

// Valid reports whether c is a recognised condition.
func (c Condition) Valid() bool {
	for _, known := range Conditions {
		if c == known {
			return true
		}
	}
	return false
}

// MaxExhaustion is the sixth level of exhaustion, at which a creature dies.
const MaxExhaustion = 6

// DamageType is one of the thirteen D&D 5e damage types.
type DamageType string

const (
	DamageAcid        DamageType = "acid"
	DamageBludgeoning DamageType = "bludgeoning"
	DamageCold        DamageType = "cold"
	DamageFire        DamageType = "fire"
	DamageForce       DamageType = "force"
	DamageLightning   DamageType = "lightning"
	DamageNecrotic    DamageType = "necrotic"
	DamagePiercing    DamageType = "piercing"
	DamagePoison      DamageType = "poison"
	DamagePsychic     DamageType = "psychic"
	DamageRadiant     DamageType = "radiant"
	DamageSlashing    DamageType = "slashing"
	DamageThunder     DamageType = "thunder"
)

// DamageTypes lists every damage type.
var DamageTypes = []DamageType{
	DamageAcid, DamageBludgeoning, DamageCold, DamageFire, DamageForce,
	DamageLightning, DamageNecrotic, DamagePiercing, DamagePoison,
	DamagePsychic, DamageRadiant, DamageSlashing, DamageThunder,
}

// Valid reports whether d is a recognised damage type.
func (d DamageType) Valid() bool {
	for _, known := range DamageTypes {
		if d == known {
			return true
		}
	}
	return false
}

// DamageAffinity is how a creature responds to a damage type.
type DamageAffinity string

const (
	AffinityNormal     DamageAffinity = "normal"
	AffinityResistant  DamageAffinity = "resistant"
	AffinityImmune     DamageAffinity = "immune"
	AffinityVulnerable DamageAffinity = "vulnerable"
)

// Apply scales an amount of damage by this affinity.
//
// Resistance halves and vulnerability doubles, both rounding down, and the
// two cancel out rather than stacking.
func (a DamageAffinity) Apply(amount int) int {
	switch a {
	case AffinityImmune:
		return 0
	case AffinityResistant:
		return amount / 2
	case AffinityVulnerable:
		return amount * 2
	default:
		return amount
	}
}

// The three capabilities a condition can take away.
//
// They are separate lists rather than one "helpless" flag because 5e keeps
// them separate, and conflating them is its own bug: an incapacitated creature
// can still speak and still walk, a grappled one can still swing a sword, and
// a stunned one can speak, if only falteringly.
var (
	// ActionPreventingConditions stop a creature taking actions and reactions.
	// The four beyond "incapacitated" all include it by definition.
	ActionPreventingConditions = []Condition{
		ConditionIncapacitated, ConditionParalyzed, ConditionPetrified,
		ConditionStunned, ConditionUnconscious,
	}

	// MovementPreventingConditions reduce a creature's speed to zero.
	// Incapacitated is deliberately absent: it costs actions, not movement.
	MovementPreventingConditions = []Condition{
		ConditionGrappled, ConditionParalyzed, ConditionPetrified,
		ConditionRestrained, ConditionStunned, ConditionUnconscious,
	}

	// SpeechPreventingConditions silence a creature. Stunned is absent: a
	// stunned creature can speak, only falteringly.
	SpeechPreventingConditions = []Condition{
		ConditionParalyzed, ConditionPetrified, ConditionUnconscious,
	}
)

func inConditions(list []Condition, c Condition) bool {
	for _, known := range list {
		if c == known {
			return true
		}
	}
	return false
}

// PreventsAction reports whether this condition stops a creature acting.
func (c Condition) PreventsAction() bool { return inConditions(ActionPreventingConditions, c) }

// PreventsMovement reports whether this condition reduces speed to zero.
func (c Condition) PreventsMovement() bool { return inConditions(MovementPreventingConditions, c) }

// PreventsSpeech reports whether this condition silences a creature.
func (c Condition) PreventsSpeech() bool { return inConditions(SpeechPreventingConditions, c) }

// firstPreventing returns the first condition in held that satisfies blocks.
func firstPreventing(held []Condition, blocks func(Condition) bool) (Condition, bool) {
	for _, c := range held {
		if blocks(c) {
			return c, true
		}
	}
	return "", false
}

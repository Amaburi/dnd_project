package models

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// SpellSchool is one of the eight schools of magic.
type SpellSchool string

const (
	SchoolAbjuration    SpellSchool = "abjuration"
	SchoolConjuration   SpellSchool = "conjuration"
	SchoolDivination    SpellSchool = "divination"
	SchoolEnchantment   SpellSchool = "enchantment"
	SchoolEvocation     SpellSchool = "evocation"
	SchoolIllusion      SpellSchool = "illusion"
	SchoolNecromancy    SpellSchool = "necromancy"
	SchoolTransmutation SpellSchool = "transmutation"
)

// SpellSchools lists every school.
var SpellSchools = []SpellSchool{
	SchoolAbjuration, SchoolConjuration, SchoolDivination, SchoolEnchantment,
	SchoolEvocation, SchoolIllusion, SchoolNecromancy, SchoolTransmutation,
}

// Valid reports whether s is a recognised school.
func (s SpellSchool) Valid() bool {
	for _, known := range SpellSchools {
		if s == known {
			return true
		}
	}
	return false
}

// SpellResolution is how the engine decides what a spell did.
//
// This is the field that makes a spell castable rather than merely nameable:
// without it nothing knows whether to roll an attack, ask for a save, or apply
// an effect that cannot be resisted.
type SpellResolution string

const (
	// SpellResolutionAttack is a ranged or melee spell attack roll.
	SpellResolutionAttack SpellResolution = "spell_attack"

	// SpellResolutionSave is resisted with a saving throw.
	SpellResolutionSave SpellResolution = "saving_throw"

	// SpellResolutionAuto always lands: Magic Missile, Cure Wounds.
	SpellResolutionAuto SpellResolution = "automatic"

	// SpellResolutionUtility has no dice the engine can resolve -- Mage Hand,
	// Detect Magic. It is narrated, not rolled, and that is not a gap.
	SpellResolutionUtility SpellResolution = "utility"
)

// SpellResolutions lists every resolution kind.
var SpellResolutions = []SpellResolution{
	SpellResolutionAttack, SpellResolutionSave, SpellResolutionAuto, SpellResolutionUtility,
}

// Valid reports whether r is a recognised resolution.
func (r SpellResolution) Valid() bool {
	for _, known := range SpellResolutions {
		if r == known {
			return true
		}
	}
	return false
}

// DamageDice is a dice expression held as numbers rather than a string.
//
// Spell damage is arithmetic -- doubled for a cantrip tier, added to for an
// upcast -- and doing that arithmetic on "3d6" by string surgery is how a
// Fireball ends up doing 36 damage. The string is produced once, at the edge,
// for the roller.
type DamageDice struct {
	Count int `json:"count" bson:"count"`
	Sides int `json:"sides" bson:"sides"`
	Bonus int `json:"bonus,omitempty" bson:"bonus,omitempty"`
}

// IsZero reports whether the expression rolls and adds nothing.
func (d DamageDice) IsZero() bool { return d.Count == 0 && d.Bonus == 0 }

// String renders the expression the way the roller expects it.
func (d DamageDice) String() string {
	if d.Count <= 0 {
		return strconv.Itoa(d.Bonus)
	}
	base := fmt.Sprintf("%dd%d", d.Count, d.Sides)
	switch {
	case d.Bonus > 0:
		return fmt.Sprintf("%s+%d", base, d.Bonus)
	case d.Bonus < 0:
		return fmt.Sprintf("%s%d", base, d.Bonus)
	default:
		return base
	}
}

// WithCount returns the expression with a different number of dice.
func (d DamageDice) WithCount(n int) DamageDice {
	d.Count = n
	return d
}

// Plus adds another expression n times, which is what upcasting does.
//
// The die size of the addition wins when the base rolls no dice, so a spell
// that only upcasts still produces a coherent expression.
func (d DamageDice) Plus(other DamageDice, times int) DamageDice {
	if times <= 0 || other.IsZero() {
		return d
	}
	if d.Count == 0 {
		d.Sides = other.Sides
	}
	d.Count += other.Count * times
	d.Bonus += other.Bonus * times
	return d
}

// Times multiplies the dice, used for a spell that fires several identical
// projectiles at a single target.
func (d DamageDice) Times(n int) DamageDice {
	if n <= 1 {
		return d
	}
	d.Count *= n
	d.Bonus *= n
	return d
}

// dieSizes are the only dice a spell is written with.
var dieSizes = map[int]bool{4: true, 6: true, 8: true, 10: true, 12: true, 20: true, 100: true}

// CantripLevels are the character levels at which a cantrip grows.
var CantripLevels = []int{5, 11, 17}

// CantripTier is the multiplier a cantrip's dice (or beams) get.
//
// Cantrips are the one part of 5e that scales with the *character*, not the
// slot, because they are cast without a slot at all.
func CantripTier(characterLevel int) int {
	tier := 1
	for _, step := range CantripLevels {
		if characterLevel >= step {
			tier++
		}
	}
	return tier
}

// SpellDefinition is what a spell actually does.
//
// models.Spell is what a character *knows* -- a name and a level. This is the
// mechanics behind that name, and the two are deliberately separate: a sheet
// stores the name, the table owns the rules, exactly as ClassTable and
// RaceTable do for classes and races.
type SpellDefinition struct {
	Key    string      `json:"key"`
	Name   string      `json:"name"`
	Level  int         `json:"level"`
	School SpellSchool `json:"school"`

	Resolution SpellResolution `json:"resolution"`

	// SaveAbility and HalfOnSave apply to SpellResolutionSave. A spell that
	// deals no damage cannot halve any, which Validate enforces.
	SaveAbility Ability `json:"save_ability,omitempty"`
	HalfOnSave  bool    `json:"half_on_save,omitempty"`

	Damage       DamageDice `json:"damage,omitempty"`
	DamageType   DamageType `json:"damage_type,omitempty"`
	UpcastDamage DamageDice `json:"upcast_damage,omitempty"`

	Healing       DamageDice `json:"healing,omitempty"`
	UpcastHealing DamageDice `json:"upcast_healing,omitempty"`

	// AddsAbilityModifier is true for healing, which adds the caster's
	// spellcasting modifier. Damage cantrips do not, and adding it would be a
	// quiet buff to every attack the character makes.
	AddsAbilityModifier bool `json:"adds_ability_modifier,omitempty"`

	// Projectiles is how many separate darts, rays or beams the spell fires.
	// Zero means a single undivided effect.
	//
	// It matters because each projectile of an attack spell is its own attack
	// roll: Scorching Ray can hit twice and miss once, and collapsing it into
	// one roll would be a different spell.
	Projectiles       int `json:"projectiles,omitempty"`
	UpcastProjectiles int `json:"upcast_projectiles,omitempty"`

	// Condition is imposed on a failed save, or on a hit for an attack spell.
	// It is the mechanical half of a control spell -- without it, Hold Person
	// would resolve a save that changed nothing.
	Condition Condition `json:"condition,omitempty"`

	Concentration bool `json:"concentration,omitempty"`
	Ritual        bool `json:"ritual,omitempty"`

	// Range in feet. 0 means self or touch; RangeSelf distinguishes them only
	// in prose, which is the narrator's business rather than the engine's.
	Range int `json:"range,omitempty"`

	Classes []Class `json:"classes"`
	Source  string  `json:"source"`
}

// IsCantrip reports whether the spell is cast without a slot.
func (s SpellDefinition) IsCantrip() bool { return s.Level == 0 }

// DealsDamage reports whether resolving this spell can hurt something.
func (s SpellDefinition) DealsDamage() bool { return !s.Damage.IsZero() }

// Heals reports whether resolving this spell restores hit points.
func (s SpellDefinition) Heals() bool { return !s.Healing.IsZero() }

// DamageAt is the damage dice for one projectile of one cast.
//
// A cantrip scales with the caster's level; everything else scales with the
// slot. A cantrip that fires beams grows in beams rather than dice, so its
// per-beam damage never changes.
func (s SpellDefinition) DamageAt(slotLevel, casterLevel int) DamageDice {
	if s.Damage.IsZero() {
		return DamageDice{}
	}
	if s.IsCantrip() {
		if s.Projectiles > 0 {
			return s.Damage
		}
		return s.Damage.WithCount(s.Damage.Count * CantripTier(casterLevel))
	}
	return s.Damage.Plus(s.UpcastDamage, slotLevel-s.Level)
}

// HealingAt is the hit points restored, before the caster's ability modifier.
func (s SpellDefinition) HealingAt(slotLevel int) DamageDice {
	if s.Healing.IsZero() {
		return DamageDice{}
	}
	return s.Healing.Plus(s.UpcastHealing, slotLevel-s.Level)
}

// ProjectilesAt is how many darts, rays or beams the cast produces.
//
// A spell that fires one undivided effect returns 1, so a caller can always
// loop over the result rather than special-casing zero.
func (s SpellDefinition) ProjectilesAt(slotLevel, casterLevel int) int {
	if s.Projectiles <= 0 {
		return 1
	}
	if s.IsCantrip() {
		return s.Projectiles * CantripTier(casterLevel)
	}
	extra := slotLevel - s.Level
	if extra < 0 {
		extra = 0
	}
	return s.Projectiles + s.UpcastProjectiles*extra
}

// ValidateSlot checks that a slot can hold this spell.
func (s SpellDefinition) ValidateSlot(slotLevel int) error {
	if s.IsCantrip() {
		if slotLevel > 0 {
			return Invalid("%s is a cantrip and is cast without a spell slot", s.Name)
		}
		return nil
	}
	if slotLevel < s.Level {
		return Invalid("%s is a level %d spell and cannot be cast from a level %d slot",
			s.Name, s.Level, slotLevel)
	}
	if slotLevel > 9 {
		return Invalid("there is no level %d spell slot", slotLevel)
	}
	return nil
}

// Validate reports a definition that cannot mean anything.
//
// It is the spell counterpart of ValidateSheet and Monster.Validate: a typo in
// a table becomes a rules bug that nobody notices until a player is cheated.
func (s SpellDefinition) Validate() error {
	var problems []string

	if !s.Resolution.Valid() {
		problems = append(problems, fmt.Sprintf("unknown resolution %q", s.Resolution))
	}
	if s.Level < 0 || s.Level > 9 {
		problems = append(problems, fmt.Sprintf("level %d is outside 0-9", s.Level))
	}
	if s.Resolution == SpellResolutionSave && !s.SaveAbility.Valid() {
		problems = append(problems, "a saving throw spell must name the save")
	}
	if s.Resolution != SpellResolutionSave && s.SaveAbility != "" {
		problems = append(problems, "only a saving throw spell has a save ability")
	}
	if s.DealsDamage() && !s.DamageType.Valid() {
		problems = append(problems, fmt.Sprintf("damage needs a valid type, got %q", s.DamageType))
	}
	if !s.DealsDamage() && s.DamageType != "" {
		problems = append(problems, "a damage type without damage")
	}
	if s.HalfOnSave && !s.DealsDamage() {
		problems = append(problems, "half damage on a save, but the spell deals none")
	}
	if s.IsCantrip() && (!s.UpcastDamage.IsZero() || !s.UpcastHealing.IsZero() || s.UpcastProjectiles != 0) {
		problems = append(problems, "a cantrip takes no slot and so cannot be upcast")
	}
	if s.Projectiles < 0 || s.UpcastProjectiles < 0 {
		problems = append(problems, "a negative projectile count")
	}
	if s.Condition != "" && !s.Condition.Valid() {
		problems = append(problems, fmt.Sprintf("unknown condition %q", s.Condition))
	}
	if s.Condition != "" && s.Resolution == SpellResolutionUtility {
		problems = append(problems, "a utility spell resolves nothing, so it cannot impose a condition")
	}
	if s.AddsAbilityModifier && !s.Heals() && !s.DealsDamage() {
		problems = append(problems, "nothing for the ability modifier to apply to")
	}

	for label, dice := range map[string]DamageDice{
		"damage": s.Damage, "upcast damage": s.UpcastDamage,
		"healing": s.Healing, "upcast healing": s.UpcastHealing,
	} {
		if dice.Count > 0 && !dieSizes[dice.Sides] {
			problems = append(problems, fmt.Sprintf("%s uses a d%d, which is not a die", label, dice.Sides))
		}
		if dice.Count < 0 {
			problems = append(problems, fmt.Sprintf("%s rolls a negative number of dice", label))
		}
	}

	if len(problems) > 0 {
		return Invalid("spell %s: %s", s.Name, strings.Join(problems, "; "))
	}
	return nil
}

// SpellKey slugs a spell name into its table key.
func SpellKey(name string) string {
	return strings.NewReplacer(" ", "_", "'", "", "-", "_").
		Replace(strings.ToLower(strings.TrimSpace(name)))
}

// SpellByName looks a spell up by display name or key.
//
// It is forgiving about casing and spacing because the name arrives from a
// character sheet a human typed, or from a model that decided to capitalise.
func SpellByName(name string) (SpellDefinition, bool) {
	def, ok := SpellTable[SpellKey(name)]
	return def, ok
}

// SpellsForClass returns every spell on a class's list, ordered by level then
// name so the result is stable.
func SpellsForClass(c Class) []SpellDefinition {
	var out []SpellDefinition
	for _, def := range SpellTable {
		for _, listed := range def.Classes {
			if listed == c {
				out = append(out, def)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return out[i].Name < out[j].Name
	})
	return out
}

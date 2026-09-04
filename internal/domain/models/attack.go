package models

import "fmt"

// AttackProfile is one weapon as it appears in the Attacks section of a
// character sheet: what to roll to hit, and what to roll for damage.
//
// This is the bridge between the character sheet and the dice roller. Without
// it a weapon is only a description -- nothing turns "longsword" plus a level
// and a Strength score into "+5 to hit, 1d8+3 slashing".
type AttackProfile struct {
	Name    string  `json:"name"`
	Key     string  `json:"key,omitempty"`
	Ability Ability `json:"ability"`

	// Proficient reports whether the proficiency bonus is included. A weapon
	// the character is untrained with still attacks, just without it.
	Proficient  bool `json:"proficient"`
	AttackBonus int  `json:"attack_bonus"`

	DamageDice string     `json:"damage_dice"`
	DamageType DamageType `json:"damage_type"`

	// CritRange is the lowest natural roll that crits with this attack.
	CritRange int `json:"crit_range"`

	// Mode is advantage or disadvantage arising from the attacker alone,
	// such as a Small creature wielding a Heavy weapon.
	Mode RollMode `json:"mode"`

	// DamageBonus is the ability modifier plus any magic bonus. It is added
	// once even on a critical hit: a critical doubles the dice, not the
	// modifiers.
	DamageBonus int `json:"damage_bonus"`

	// VersatileDice is the damage when a versatile weapon is used two-handed.
	VersatileDice string `json:"versatile_dice,omitempty"`

	RangeNormal int `json:"range_normal,omitempty"`
	RangeLong   int `json:"range_long,omitempty"`
	Reach       int `json:"reach,omitempty"`
}

// DamageExpression renders the damage as it would be written on a sheet, e.g.
// "1d8+3" or "2d6-1".
func (a AttackProfile) DamageExpression() string {
	switch {
	case a.DamageBonus > 0:
		return fmt.Sprintf("%s+%d", a.DamageDice, a.DamageBonus)
	case a.DamageBonus < 0:
		return fmt.Sprintf("%s%d", a.DamageDice, a.DamageBonus)
	default:
		return a.DamageDice
	}
}

// RangeDescription renders reach or range in the sheet's shorthand.
func (a AttackProfile) RangeDescription() string {
	if a.RangeNormal > 0 {
		if a.RangeLong > a.RangeNormal {
			return fmt.Sprintf("%d/%d ft.", a.RangeNormal, a.RangeLong)
		}
		return fmt.Sprintf("%d ft.", a.RangeNormal)
	}
	reach := a.Reach
	if reach == 0 {
		reach = 5
	}
	return fmt.Sprintf("%d ft.", reach)
}

// MeleeReach is the default reach of a melee weapon in feet; the reach
// property adds five more.
const MeleeReach = 5

// CritRange is the lowest natural d20 roll that scores a critical hit for this
// character, taking the best of any archetype that widens it.
func (c *Character) CritRange() int {
	best := NaturalCrit
	for _, cl := range c.BasicInfo.Classes {
		if rng := cl.CritRange(); rng < best {
			best = rng
		}
	}
	return best
}

// AttackWith builds the attack profile for a weapon.
//
// The proficiency bonus is added only when the character is trained with the
// weapon, which is why proficiencies had to live on the character rather than
// only in the class table.
func (c *Character) AttackWith(item InventoryItem) (AttackProfile, error) {
	if item.Weapon == nil {
		return AttackProfile{}, Invalid("%q is not a weapon", item.Name)
	}

	w := item.Weapon
	ability := w.AttackAbility(c.AbilityScores)
	modifier := c.AbilityModifier(ability)
	proficient := c.Proficiencies.HasWeapon(item)

	profile := AttackProfile{
		Name:          item.Name,
		Key:           item.Key,
		Ability:       ability,
		Proficient:    proficient,
		AttackBonus:   modifier + w.MagicBonus,
		DamageDice:    w.DamageDice,
		DamageType:    w.DamageType,
		CritRange:     c.CritRange(),
		Mode:          c.AttackRollMode(item),
		DamageBonus:   modifier + w.MagicBonus,
		VersatileDice: w.VersatileDice,
		RangeNormal:   w.RangeNormal,
		RangeLong:     w.RangeLong,
	}

	if proficient {
		profile.AttackBonus += c.ProficiencyBonus()
	}

	if !w.IsRanged() {
		profile.Reach = MeleeReach
		if w.HasProperty(PropertyReach) {
			profile.Reach += 5
		}
	}

	return profile, nil
}

// Attacks returns a profile for every equipped weapon, which is what the
// Attacks section of the sheet lists.
func (c *Character) Attacks() []AttackProfile {
	var profiles []AttackProfile
	for _, item := range c.Equipment.Weapons {
		if profile, err := c.AttackWith(item); err == nil {
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

// UnarmedStrike is the attack every character always has.
//
// Damage is 1 + the Strength modifier, and proficiency always applies.
func (c *Character) UnarmedStrike() AttackProfile {
	modifier := c.AbilityModifier(AbilityStrength)
	return AttackProfile{
		Name:        "Unarmed Strike",
		Key:         "unarmed_strike",
		Ability:     AbilityStrength,
		Proficient:  true,
		AttackBonus: modifier + c.ProficiencyBonus(),
		DamageDice:  "1",
		DamageType:  DamageBludgeoning,
		CritRange:   c.CritRange(),
		DamageBonus: modifier,
		Reach:       MeleeReach,
	}
}

// SpellAttack is the profile for a spell attack roll: proficiency plus the
// spellcasting ability, with no weapon involved.
func (c *Character) SpellAttack() (AttackProfile, bool) {
	if !c.Spells.SpellcastingAbility.Valid() {
		return AttackProfile{}, false
	}
	return AttackProfile{
		Name:        "Spell Attack",
		Ability:     c.Spells.SpellcastingAbility,
		Proficient:  true,
		AttackBonus: c.SpellAttackModifier(),
	}, true
}

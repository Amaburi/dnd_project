package models

// ItemKind classifies an inventory entry, deciding which property block (if
// any) carries its mechanics.
type ItemKind string

const (
	ItemGear       ItemKind = "gear"
	ItemWeapon     ItemKind = "weapon"
	ItemArmor      ItemKind = "armor"
	ItemShield     ItemKind = "shield"
	ItemConsumable ItemKind = "consumable"
	ItemAmmunition ItemKind = "ammunition"
)

// WeaponCategory separates simple from martial weapons for proficiency.
type WeaponCategory string

const (
	WeaponSimple  WeaponCategory = "simple"
	WeaponMartial WeaponCategory = "martial"
)

// WeaponProperty is a 5e weapon property tag.
type WeaponProperty string

const (
	PropertyAmmunition WeaponProperty = "ammunition"
	PropertyFinesse    WeaponProperty = "finesse"
	PropertyHeavy      WeaponProperty = "heavy"
	PropertyLight      WeaponProperty = "light"
	PropertyLoading    WeaponProperty = "loading"
	PropertyReach      WeaponProperty = "reach"
	PropertyThrown     WeaponProperty = "thrown"
	PropertyTwoHanded  WeaponProperty = "two_handed"
	PropertyVersatile  WeaponProperty = "versatile"
)

// ArmorCategory decides how Dexterity contributes to armor class.
type ArmorCategory string

const (
	ArmorLight  ArmorCategory = "light"
	ArmorMedium ArmorCategory = "medium"
	ArmorHeavy  ArmorCategory = "heavy"
)

// WeaponProperties carries the mechanics an attack roll needs.
//
// Without these an attack cannot be resolved from character data at all --
// the damage dice have to come from somewhere.
type WeaponProperties struct {
	Category   WeaponCategory   `json:"category" bson:"category"`
	DamageDice string           `json:"damage_dice" bson:"damage_dice"` // e.g. "1d8"
	DamageType DamageType       `json:"damage_type" bson:"damage_type"`
	Properties []WeaponProperty `json:"properties" bson:"properties"`

	// VersatileDice is the two-handed damage of a versatile weapon, e.g.
	// "1d10" for a longsword. Empty unless PropertyVersatile is present.
	VersatileDice string `json:"versatile_dice,omitempty" bson:"versatile_dice,omitempty"`

	// Ranged weapons carry their normal and long range in feet. A melee
	// weapon leaves both zero; a thrown weapon may set both and still be
	// used in melee.
	RangeNormal int `json:"range_normal,omitempty" bson:"range_normal,omitempty"`
	RangeLong   int `json:"range_long,omitempty" bson:"range_long,omitempty"`

	// MagicBonus applies to both attack and damage rolls (+1/+2/+3 weapons).
	MagicBonus int `json:"magic_bonus,omitempty" bson:"magic_bonus,omitempty"`
}

// HasProperty reports whether the weapon carries a given property.
func (w WeaponProperties) HasProperty(p WeaponProperty) bool {
	for _, have := range w.Properties {
		if have == p {
			return true
		}
	}
	return false
}

// IsRanged reports whether attacks default to Dexterity because the weapon is
// used at range rather than in melee.
func (w WeaponProperties) IsRanged() bool {
	return w.RangeNormal > 0 && !w.HasProperty(PropertyThrown)
}

// AttackAbility returns the ability an attack with this weapon uses.
//
// Melee uses Strength and ranged uses Dexterity, except that a finesse weapon
// may use either -- the wielder picks, so the better modifier is chosen here.
func (w WeaponProperties) AttackAbility(scores AbilityScores) Ability {
	if w.IsRanged() {
		return AbilityDexterity
	}
	if w.HasProperty(PropertyFinesse) {
		if scores.Modifier(AbilityDexterity) > scores.Modifier(AbilityStrength) {
			return AbilityDexterity
		}
	}
	return AbilityStrength
}

// ArmorProperties carries what armor contributes to armor class.
type ArmorProperties struct {
	Category ArmorCategory `json:"category" bson:"category"`
	BaseAC   int           `json:"base_ac" bson:"base_ac"`

	// StrengthRequirement is the Strength score below which heavy armor
	// reduces speed by 10 feet. Zero means no requirement.
	StrengthRequirement int `json:"strength_requirement,omitempty" bson:"strength_requirement,omitempty"`

	// StealthDisadvantage marks armor that imposes disadvantage on Stealth.
	StealthDisadvantage bool `json:"stealth_disadvantage,omitempty" bson:"stealth_disadvantage,omitempty"`

	// MagicBonus adds to AC (+1/+2/+3 armor and shields).
	MagicBonus int `json:"magic_bonus,omitempty" bson:"magic_bonus,omitempty"`
}

// EffectiveAC returns the armor class this armor grants, applying the
// Dexterity rule for its category: light armor adds the full modifier, medium
// caps it at +2, and heavy ignores it entirely.
func (a ArmorProperties) EffectiveAC(dexModifier int) int {
	ac := a.BaseAC + a.MagicBonus
	switch a.Category {
	case ArmorLight:
		return ac + dexModifier
	case ArmorMedium:
		if dexModifier > 2 {
			dexModifier = 2
		}
		return ac + dexModifier
	case ArmorHeavy:
		return ac
	default:
		return ac + dexModifier
	}
}

// InventoryItem represents an item in a character's inventory.
//
// Weapon and Armor are optional blocks carrying the mechanics for their kind;
// plain gear leaves both nil.
type InventoryItem struct {
	ItemID      string   `json:"item_id" bson:"item_id"`
	Name        string   `json:"name" bson:"name"`
	Kind        ItemKind `json:"kind" bson:"kind"`
	Quantity    int      `json:"quantity" bson:"quantity"`
	Weight      float64  `json:"weight" bson:"weight"`
	Equipped    bool     `json:"equipped" bson:"equipped"`
	Description string   `json:"description" bson:"description"`

	Weapon *WeaponProperties `json:"weapon,omitempty" bson:"weapon,omitempty"`
	Armor  *ArmorProperties  `json:"armor,omitempty" bson:"armor,omitempty"`
}

// Equipment represents equipped items.
type Equipment struct {
	Armor       *InventoryItem  `json:"armor" bson:"armor"`
	Shield      *InventoryItem  `json:"shield" bson:"shield"`
	Weapons     []InventoryItem `json:"weapons" bson:"weapons"`
	Accessories []InventoryItem `json:"accessories" bson:"accessories"`
}

// ShieldBonus is the armor class a shield adds.
const ShieldBonus = 2

// ArmorClass computes AC from what is equipped.
//
// Unarmored is 10 + Dexterity modifier; worn armor applies its category's
// Dexterity rule; a shield adds 2 plus any magic bonus of its own.
func (e Equipment) ArmorClass(dexModifier int) int {
	ac := 10 + dexModifier
	if e.Armor != nil && e.Armor.Armor != nil {
		ac = e.Armor.Armor.EffectiveAC(dexModifier)
	}
	if e.Shield != nil {
		ac += ShieldBonus
		if e.Shield.Armor != nil {
			ac += e.Shield.Armor.MagicBonus
		}
	}
	return ac
}

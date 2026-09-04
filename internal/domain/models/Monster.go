package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CreatureSize is a 5e size category.
type CreatureSize string

const (
	SizeTiny       CreatureSize = "tiny"
	SizeSmall      CreatureSize = "small"
	SizeMedium     CreatureSize = "medium"
	SizeLarge      CreatureSize = "large"
	SizeHuge       CreatureSize = "huge"
	SizeGargantuan CreatureSize = "gargantuan"
)

// Monster is a statblock for a hostile or neutral creature.
//
// Monsters were previously stored as Characters with Type "monster", which
// forced a dragon through a schema built around race, class, background and
// experience points. A statblock is a different shape: it has a challenge
// rating rather than a level, flat save and skill bonuses rather than
// proficiencies derived from a class, damage resistances that the character
// sheet has no field for at all, and multiattack and legendary actions that
// have no character equivalent.
type Monster struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	MonsterID  string             `json:"monster_id" bson:"monster_id"`
	CampaignID string             `json:"campaign_id" bson:"campaign_id"`

	Name      string       `json:"name" bson:"name"`
	Size      CreatureSize `json:"size" bson:"size"`
	Type      string       `json:"type" bson:"type"`                           // "dragon", "undead", "beast", ...
	Subtype   string       `json:"subtype,omitempty" bson:"subtype,omitempty"` // e.g. "goblinoid"
	Alignment string       `json:"alignment" bson:"alignment"`

	ArmorClass     int       `json:"armor_class" bson:"armor_class"`
	ArmorNote      string    `json:"armor_note,omitempty" bson:"armor_note,omitempty"` // "natural armor", "chain shirt, shield"
	HitPoints      HitPoints `json:"hit_points" bson:"hit_points"`
	HitDiceFormula string    `json:"hit_dice_formula,omitempty" bson:"hit_dice_formula,omitempty"` // "18d10+36"

	Speeds        Speeds        `json:"speeds" bson:"speeds"`
	AbilityScores AbilityScores `json:"ability_scores" bson:"ability_scores"`

	// Statblocks print flat bonuses rather than proficiency levels, because a
	// monster's bonuses do not derive from a class or a level.
	SavingThrows map[Ability]int `json:"saving_throws,omitempty" bson:"saving_throws,omitempty"`
	Skills       map[Skill]int   `json:"skills,omitempty" bson:"skills,omitempty"`

	// Damage affinities are core combat maths and had no representation at
	// all before: halving, negating and doubling damage by type is what makes
	// damage types mean anything.
	DamageVulnerabilities []DamageType `json:"damage_vulnerabilities,omitempty" bson:"damage_vulnerabilities,omitempty"`
	DamageResistances     []DamageType `json:"damage_resistances,omitempty" bson:"damage_resistances,omitempty"`
	DamageImmunities      []DamageType `json:"damage_immunities,omitempty" bson:"damage_immunities,omitempty"`
	ConditionImmunities   []Condition  `json:"condition_immunities,omitempty" bson:"condition_immunities,omitempty"`

	Senses    Senses   `json:"senses" bson:"senses"`
	Languages []string `json:"languages,omitempty" bson:"languages,omitempty"`

	ChallengeRating  float64 `json:"challenge_rating" bson:"challenge_rating"` // 0.125, 0.25, 0.5, then whole numbers
	XP               int     `json:"xp" bson:"xp"`
	ProficiencyBonus int     `json:"proficiency_bonus" bson:"proficiency_bonus"`

	Traits       []MonsterFeature `json:"traits,omitempty" bson:"traits,omitempty"`
	Actions      []MonsterAction  `json:"actions,omitempty" bson:"actions,omitempty"`
	BonusActions []MonsterAction  `json:"bonus_actions,omitempty" bson:"bonus_actions,omitempty"`
	Reactions    []MonsterAction  `json:"reactions,omitempty" bson:"reactions,omitempty"`

	LegendaryActionsPerRound int             `json:"legendary_actions_per_round,omitempty" bson:"legendary_actions_per_round,omitempty"`
	LegendaryActions         []MonsterAction `json:"legendary_actions,omitempty" bson:"legendary_actions,omitempty"`

	Description string `json:"description,omitempty" bson:"description,omitempty"`
	Source      string `json:"source,omitempty" bson:"source,omitempty"` // book or "homebrew"

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// Speeds holds each movement mode in feet. Zero means the creature lacks it.
type Speeds struct {
	Walk   int  `json:"walk" bson:"walk"`
	Fly    int  `json:"fly,omitempty" bson:"fly,omitempty"`
	Swim   int  `json:"swim,omitempty" bson:"swim,omitempty"`
	Climb  int  `json:"climb,omitempty" bson:"climb,omitempty"`
	Burrow int  `json:"burrow,omitempty" bson:"burrow,omitempty"`
	Hover  bool `json:"hover,omitempty" bson:"hover,omitempty"`
}

// Senses holds special senses in feet, plus passive Perception.
type Senses struct {
	Darkvision        int `json:"darkvision,omitempty" bson:"darkvision,omitempty"`
	Blindsight        int `json:"blindsight,omitempty" bson:"blindsight,omitempty"`
	Tremorsense       int `json:"tremorsense,omitempty" bson:"tremorsense,omitempty"`
	Truesight         int `json:"truesight,omitempty" bson:"truesight,omitempty"`
	PassivePerception int `json:"passive_perception" bson:"passive_perception"`
}

// MonsterFeature is a passive trait, such as Pack Tactics or Regeneration.
type MonsterFeature struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
}

// MonsterAction is something a monster can do on its turn.
type MonsterAction struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`

	// AttackBonus, DamageDice and DamageType are set for attack actions and
	// left zero for anything resolved by a saving throw or by narration.
	AttackBonus *int       `json:"attack_bonus,omitempty" bson:"attack_bonus,omitempty"`
	DamageDice  string     `json:"damage_dice,omitempty" bson:"damage_dice,omitempty"`
	DamageType  DamageType `json:"damage_type,omitempty" bson:"damage_type,omitempty"`
	ReachFeet   int        `json:"reach_feet,omitempty" bson:"reach_feet,omitempty"`
	RangeFeet   int        `json:"range_feet,omitempty" bson:"range_feet,omitempty"`

	// SaveDC and SaveAbility describe actions the target resists instead.
	SaveDC      int     `json:"save_dc,omitempty" bson:"save_dc,omitempty"`
	SaveAbility Ability `json:"save_ability,omitempty" bson:"save_ability,omitempty"`

	// AttacksPerAction is how many attacks this action makes, for
	// multiattack. Zero and one both mean a single attack.
	AttacksPerAction int `json:"attacks_per_action,omitempty" bson:"attacks_per_action,omitempty"`

	// LegendaryCost is how many legendary actions this option consumes.
	LegendaryCost int `json:"legendary_cost,omitempty" bson:"legendary_cost,omitempty"`
}

// AffinityTo reports how the monster responds to a damage type.
//
// Immunity beats resistance, which beats vulnerability, so a creature listed
// under more than one is never scaled twice.
func (m *Monster) AffinityTo(dt DamageType) DamageAffinity {
	for _, t := range m.DamageImmunities {
		if t == dt {
			return AffinityImmune
		}
	}
	for _, t := range m.DamageResistances {
		if t == dt {
			return AffinityResistant
		}
	}
	for _, t := range m.DamageVulnerabilities {
		if t == dt {
			return AffinityVulnerable
		}
	}
	return AffinityNormal
}

// TakeDamage applies damage of a type, scaling it by affinity first, and
// reports the damage actually dealt and any overflow past zero hit points.
func (m *Monster) TakeDamage(amount int, dt DamageType) (dealt, overflow int) {
	dealt = m.AffinityTo(dt).Apply(amount)
	overflow = m.HitPoints.ApplyDamage(dealt)
	return dealt, overflow
}

// ImmuneToCondition reports whether a condition cannot affect this monster.
func (m *Monster) ImmuneToCondition(c Condition) bool {
	for _, immune := range m.ConditionImmunities {
		if immune == c {
			return true
		}
	}
	return false
}

// InitiativeModifier is the Dexterity modifier, as for any creature.
func (m *Monster) InitiativeModifier() int {
	return m.AbilityScores.Modifier(AbilityDexterity)
}

// SavingThrowModifier returns the monster's bonus for a save: its listed
// bonus if it has one, otherwise the bare ability modifier.
func (m *Monster) SavingThrowModifier(a Ability) int {
	if bonus, ok := m.SavingThrows[a]; ok {
		return bonus
	}
	return m.AbilityScores.Modifier(a)
}

// SkillModifier returns the monster's bonus for a skill check: its listed
// bonus if it has one, otherwise the bare ability modifier.
func (m *Monster) SkillModifier(s Skill) int {
	if bonus, ok := m.Skills[s]; ok {
		return bonus
	}
	return m.AbilityScores.Modifier(s.Ability())
}

// ChallengeRatingXP maps challenge rating to the experience it awards.
var ChallengeRatingXP = map[float64]int{
	0: 10, 0.125: 25, 0.25: 50, 0.5: 100,
	1: 200, 2: 450, 3: 700, 4: 1100, 5: 1800, 6: 2300, 7: 2900, 8: 3900,
	9: 5000, 10: 5900, 11: 7200, 12: 8400, 13: 10000, 14: 11500, 15: 13000,
	16: 15000, 17: 18000, 18: 20000, 19: 22000, 20: 25000, 21: 33000,
	22: 41000, 23: 50000, 24: 62000, 25: 75000, 26: 90000, 27: 105000,
	28: 120000, 29: 135000, 30: 155000,
}

// ProficiencyBonusForCR returns the proficiency bonus a monster of a given
// challenge rating uses: +2 up to CR 4, then rising by one every four CRs.
func ProficiencyBonusForCR(cr float64) int {
	if cr < 5 {
		return 2
	}
	if cr > 30 {
		cr = 30
	}
	return 2 + (int(cr)-1)/4
}

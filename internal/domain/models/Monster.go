package models

import (
	"fmt"
	"strconv"
	"strings"
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

// DamageAffinities is how a creature responds to each damage type.
//
// The three lists live in one type so combatants and statblocks resolve
// affinity through the same code: keeping a separate copy on each meant the
// combat path quietly ignored resistances entirely.
type DamageAffinities struct {
	Vulnerabilities []DamageType `json:"vulnerabilities,omitempty" bson:"vulnerabilities,omitempty"`
	Resistances     []DamageType `json:"resistances,omitempty" bson:"resistances,omitempty"`
	Immunities      []DamageType `json:"immunities,omitempty" bson:"immunities,omitempty"`
}

// For reports how a damage type is treated.
//
// Immunity beats resistance, which beats vulnerability, so a creature listed
// under more than one is never scaled twice.
func (a DamageAffinities) For(dt DamageType) DamageAffinity {
	for _, t := range a.Immunities {
		if t == dt {
			return AffinityImmune
		}
	}
	for _, t := range a.Resistances {
		if t == dt {
			return AffinityResistant
		}
	}
	for _, t := range a.Vulnerabilities {
		if t == dt {
			return AffinityVulnerable
		}
	}
	return AffinityNormal
}

// IsEmpty reports whether the creature treats all damage normally.
func (a DamageAffinities) IsEmpty() bool {
	return len(a.Vulnerabilities) == 0 && len(a.Resistances) == 0 && len(a.Immunities) == 0
}

// Monster is a statblock for a hostile or neutral creature.
//
// Monsters are an open catalogue rather than a closed table like classes and
// races: they are stored, not enumerated. What is fixed is the shape.
type Monster struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	MonsterID  string             `json:"monster_id" bson:"monster_id"`
	CampaignID string             `json:"campaign_id" bson:"campaign_id"`

	Name      string       `json:"name" bson:"name"`
	Size      CreatureSize `json:"size" bson:"size"`
	Type      string       `json:"type" bson:"type"`                           // "dragon", "undead", "beast", ...
	Subtype   string       `json:"subtype,omitempty" bson:"subtype,omitempty"` // e.g. "goblinoid"
	Alignment string       `json:"alignment" bson:"alignment"`

	ArmorClass int       `json:"armor_class" bson:"armor_class"`
	ArmorNote  string    `json:"armor_note,omitempty" bson:"armor_note,omitempty"` // "natural armor"
	HitPoints  HitPoints `json:"hit_points" bson:"hit_points"`

	// HitDice is the formula a statblock prints, e.g. "18d10+36". It is
	// parsed rather than decorative: AverageHitPoints checks the printed
	// maximum against what the dice actually average to.
	HitDice string `json:"hit_dice,omitempty" bson:"hit_dice,omitempty"`

	Speeds        Speeds        `json:"speeds" bson:"speeds"`
	AbilityScores AbilityScores `json:"ability_scores" bson:"ability_scores"`

	// Statblocks print flat bonuses rather than proficiency levels, because a
	// monster's bonuses do not derive from a class or a level.
	SavingThrows map[Ability]int `json:"saving_throws,omitempty" bson:"saving_throws,omitempty"`
	Skills       map[Skill]int   `json:"skills,omitempty" bson:"skills,omitempty"`

	Affinities          DamageAffinities `json:"damage_affinities" bson:"damage_affinities"`
	ConditionImmunities []Condition      `json:"condition_immunities,omitempty" bson:"condition_immunities,omitempty"`

	Senses    Senses   `json:"senses" bson:"senses"`
	Languages []string `json:"languages,omitempty" bson:"languages,omitempty"`

	ChallengeRating float64 `json:"challenge_rating" bson:"challenge_rating"` // 0.125, 0.25, 0.5, then whole numbers

	Traits       []MonsterFeature `json:"traits,omitempty" bson:"traits,omitempty"`
	Actions      []MonsterAction  `json:"actions,omitempty" bson:"actions,omitempty"`
	BonusActions []MonsterAction  `json:"bonus_actions,omitempty" bson:"bonus_actions,omitempty"`
	Reactions    []MonsterAction  `json:"reactions,omitempty" bson:"reactions,omitempty"`

	// LegendaryResistancePerDay lets a creature turn a failed save into a
	// success that many times. Nearly every boss has it and it had no field.
	LegendaryResistancePerDay int `json:"legendary_resistance_per_day,omitempty" bson:"legendary_resistance_per_day,omitempty"`

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

// Senses holds special senses in feet. Passive Perception is derived rather
// than stored, so it cannot disagree with the creature's Wisdom.
type Senses struct {
	Darkvision  int `json:"darkvision,omitempty" bson:"darkvision,omitempty"`
	Blindsight  int `json:"blindsight,omitempty" bson:"blindsight,omitempty"`
	Tremorsense int `json:"tremorsense,omitempty" bson:"tremorsense,omitempty"`
	Truesight   int `json:"truesight,omitempty" bson:"truesight,omitempty"`
}

// MonsterFeature is a passive trait, such as Pack Tactics or Regeneration.
type MonsterFeature struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
}

// MultiattackPart is one component of a multiattack, naming another action and
// how many times it is used.
//
// "One bite and two claws" refers to other entries; an attack count on its own
// could not express which attacks.
type MultiattackPart struct {
	ActionName string `json:"action_name" bson:"action_name"`
	Count      int    `json:"count" bson:"count"`
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
	RangeNormal int        `json:"range_normal,omitempty" bson:"range_normal,omitempty"`
	RangeLong   int        `json:"range_long,omitempty" bson:"range_long,omitempty"`

	// SaveDC and SaveAbility describe actions the target resists instead.
	SaveDC      int     `json:"save_dc,omitempty" bson:"save_dc,omitempty"`
	SaveAbility Ability `json:"save_ability,omitempty" bson:"save_ability,omitempty"`

	// Multiattack names the actions this one performs and how often.
	Multiattack []MultiattackPart `json:"multiattack,omitempty" bson:"multiattack,omitempty"`

	// LegendaryCost is how many legendary actions this option consumes.
	LegendaryCost int `json:"legendary_cost,omitempty" bson:"legendary_cost,omitempty"`
}

// IsMultiattack reports whether the action is a multiattack routine.
func (a MonsterAction) IsMultiattack() bool { return len(a.Multiattack) > 0 }

// ---------------------------------------------------------------------------
// Derived values. Proficiency bonus, XP and passive Perception were stored
// fields beside the tables that compute them, which is the same drift the
// character sheet had.
// ---------------------------------------------------------------------------

// ProficiencyBonus is derived from challenge rating.
func (m *Monster) ProficiencyBonus() int {
	return ProficiencyBonusForCR(m.ChallengeRating)
}

// XP is the experience the monster awards, from its challenge rating.
func (m *Monster) XP() int {
	return ChallengeRatingXP[m.ChallengeRating]
}

// PassivePerception is 10 plus the monster's Perception bonus.
func (m *Monster) PassivePerception() int {
	return 10 + m.SkillModifier(SkillPerception)
}

// AffinityTo reports how the monster responds to a damage type.
func (m *Monster) AffinityTo(dt DamageType) DamageAffinity {
	return m.Affinities.For(dt)
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

// Action looks an action up by name, which is how a multiattack resolves its
// component parts.
func (m *Monster) Action(name string) (MonsterAction, bool) {
	for _, group := range [][]MonsterAction{m.Actions, m.BonusActions, m.Reactions, m.LegendaryActions} {
		for _, a := range group {
			if strings.EqualFold(a.Name, name) {
				return a, true
			}
		}
	}
	return MonsterAction{}, false
}

// HitDiceFormula is a parsed "NdX+B" hit point expression.
type HitDiceFormula struct {
	Count int
	Die   int
	Bonus int
}

// Average is the hit points the formula averages to, which is the number a
// statblock prints: each die contributes (die+1)/2, rounded down at the end.
func (f HitDiceFormula) Average() int {
	return (f.Count*(f.Die+1))/2 + f.Bonus
}

// String renders the formula the way a statblock prints it.
func (f HitDiceFormula) String() string {
	if f.Bonus > 0 {
		return fmt.Sprintf("%dd%d+%d", f.Count, f.Die, f.Bonus)
	}
	if f.Bonus < 0 {
		return fmt.Sprintf("%dd%d%d", f.Count, f.Die, f.Bonus)
	}
	return fmt.Sprintf("%dd%d", f.Count, f.Die)
}

// ParseHitDiceFormula reads "8d10+40" and its variants.
func ParseHitDiceFormula(s string) (HitDiceFormula, error) {
	text := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(s)), " ", "")
	if text == "" {
		return HitDiceFormula{}, Invalid("hit dice formula is empty")
	}

	parts := strings.SplitN(text, "d", 2)
	if len(parts) != 2 {
		return HitDiceFormula{}, Invalid("hit dice formula %q is not of the form NdX+B", s)
	}

	count, err := strconv.Atoi(parts[0])
	if err != nil || count < 1 {
		return HitDiceFormula{}, Invalid("hit dice formula %q has an invalid die count", s)
	}

	rest := parts[1]
	bonus := 0
	if i := strings.IndexAny(rest, "+-"); i >= 0 {
		bonus, err = strconv.Atoi(rest[i:])
		if err != nil {
			return HitDiceFormula{}, Invalid("hit dice formula %q has an invalid bonus", s)
		}
		rest = rest[:i]
	}

	die, err := strconv.Atoi(rest)
	if err != nil || die < 1 {
		return HitDiceFormula{}, Invalid("hit dice formula %q has an invalid die size", s)
	}

	return HitDiceFormula{Count: count, Die: die, Bonus: bonus}, nil
}

// AverageHitPoints returns the hit points the monster's dice average to, and
// whether the formula could be read at all.
func (m *Monster) AverageHitPoints() (int, bool) {
	formula, err := ParseHitDiceFormula(m.HitDice)
	if err != nil {
		return 0, false
	}
	return formula.Average(), true
}

// Validate checks a statblock for internal consistency.
//
// The character sheet has ValidateSheet; a statblock had nothing, so one with
// an invented damage type or no hit points saved without complaint.
func (m *Monster) Validate() error {
	var problems []string

	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, "name is required")
	}
	switch m.Size {
	case SizeTiny, SizeSmall, SizeMedium, SizeLarge, SizeHuge, SizeGargantuan:
	default:
		problems = append(problems, fmt.Sprintf("unknown size %q", m.Size))
	}
	if m.ArmorClass < 1 {
		problems = append(problems, fmt.Sprintf("armor class is %d, want at least 1", m.ArmorClass))
	}
	if m.HitPoints.Maximum < 1 {
		problems = append(problems, "hit point maximum must be at least 1")
	}
	if _, ok := ChallengeRatingXP[m.ChallengeRating]; !ok {
		problems = append(problems, fmt.Sprintf("challenge rating %v is not a recognised value", m.ChallengeRating))
	}

	for _, group := range []struct {
		label string
		types []DamageType
	}{
		{"vulnerability", m.Affinities.Vulnerabilities},
		{"resistance", m.Affinities.Resistances},
		{"immunity", m.Affinities.Immunities},
	} {
		for _, dt := range group.types {
			if !dt.Valid() {
				problems = append(problems, fmt.Sprintf("unknown damage %s %q", group.label, dt))
			}
		}
	}

	for _, c := range m.ConditionImmunities {
		if !c.Valid() {
			problems = append(problems, fmt.Sprintf("unknown condition immunity %q", c))
		}
	}
	for a := range m.SavingThrows {
		if !a.Valid() {
			problems = append(problems, fmt.Sprintf("unknown saving throw ability %q", a))
		}
	}
	for s := range m.Skills {
		if !s.Valid() {
			problems = append(problems, fmt.Sprintf("unknown skill %q", s))
		}
	}

	for _, group := range [][]MonsterAction{m.Actions, m.BonusActions, m.Reactions, m.LegendaryActions} {
		for _, a := range group {
			if a.Name == "" {
				problems = append(problems, "an action has no name")
				continue
			}
			if a.DamageType != "" && !a.DamageType.Valid() {
				problems = append(problems, fmt.Sprintf("action %q has unknown damage type %q", a.Name, a.DamageType))
			}
			if a.SaveAbility != "" && !a.SaveAbility.Valid() {
				problems = append(problems, fmt.Sprintf("action %q has unknown save ability %q", a.Name, a.SaveAbility))
			}
			// A multiattack that names an action the monster does not have
			// cannot be resolved.
			for _, part := range a.Multiattack {
				if _, ok := m.Action(part.ActionName); !ok {
					problems = append(problems, fmt.Sprintf(
						"multiattack %q refers to unknown action %q", a.Name, part.ActionName))
				}
				if part.Count < 1 {
					problems = append(problems, fmt.Sprintf(
						"multiattack %q uses %q %d times", a.Name, part.ActionName, part.Count))
				}
			}
		}
	}

	if m.LegendaryActionsPerRound > 0 && len(m.LegendaryActions) == 0 {
		problems = append(problems, "legendary actions are allowed but none are defined")
	}

	// The printed hit points should match what the dice average to. A
	// mismatch is usually a typo in one or the other.
	if m.HitDice != "" {
		if average, ok := m.AverageHitPoints(); !ok {
			problems = append(problems, fmt.Sprintf("hit dice %q could not be parsed", m.HitDice))
		} else if average != m.HitPoints.Maximum {
			problems = append(problems, fmt.Sprintf(
				"hit point maximum is %d but %s averages %d", m.HitPoints.Maximum, m.HitDice, average))
		}
	}

	if len(problems) > 0 {
		return Invalid("invalid statblock: %s", strings.Join(problems, "; "))
	}
	return nil
}

// ToCombatant builds a combat entry from the statblock.
//
// Affinities and condition immunities are copied across so the combat path
// resolves them; before this they lived only on the Monster and a fire-immune
// creature took full fire damage in an encounter. Monsters do not make death
// saves -- at zero hit points they simply die.
func (m *Monster) ToCombatant(combatantID string) Combatant {
	hp := m.HitPoints
	if hp.Current == 0 && hp.Maximum > 0 {
		hp.Current = hp.Maximum
	}

	return Combatant{
		CombatantID:                  combatantID,
		SourceType:                   SourceMonster,
		SourceID:                     m.MonsterID,
		Type:                         "enemy",
		Name:                         m.Name,
		InitiativeModifier:           m.InitiativeModifier(),
		HitPoints:                    hp,
		ArmorClass:                   m.ArmorClass,
		Status:                       CombatantActive,
		Affinities:                   m.Affinities,
		ConditionImmunities:          append([]Condition(nil), m.ConditionImmunities...),
		MakesDeathSaves:              false,
		Speed:                        m.Speeds.Walk,
		MovementRemaining:            m.Speeds.Walk,
		LegendaryResistanceRemaining: m.LegendaryResistancePerDay,
	}
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

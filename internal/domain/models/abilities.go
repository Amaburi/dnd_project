package models

// Ability is one of the six D&D 5e ability scores.
type Ability string

const (
	AbilityStrength     Ability = "strength"
	AbilityDexterity    Ability = "dexterity"
	AbilityConstitution Ability = "constitution"
	AbilityIntelligence Ability = "intelligence"
	AbilityWisdom       Ability = "wisdom"
	AbilityCharisma     Ability = "charisma"
)

// Abilities lists the six abilities in character-sheet order.
var Abilities = []Ability{
	AbilityStrength,
	AbilityDexterity,
	AbilityConstitution,
	AbilityIntelligence,
	AbilityWisdom,
	AbilityCharisma,
}

// Valid reports whether a is one of the six abilities.
func (a Ability) Valid() bool {
	switch a {
	case AbilityStrength, AbilityDexterity, AbilityConstitution,
		AbilityIntelligence, AbilityWisdom, AbilityCharisma:
		return true
	}
	return false
}

// Proficiency is how strongly a character's proficiency bonus applies to a
// roll. 5e is not binary here: rogues and bards double their bonus through
// Expertise, and a bard's Jack of All Trades adds half of it to checks that
// are not otherwise proficient.
type Proficiency string

const (
	ProficiencyNone       Proficiency = ""
	ProficiencyHalf       Proficiency = "half"
	ProficiencyProficient Proficiency = "proficient"
	ProficiencyExpertise  Proficiency = "expertise"
)

// Bonus returns the portion of a proficiency bonus this level contributes.
// Half-proficiency rounds down, per the Jack of All Trades wording.
func (p Proficiency) Bonus(proficiencyBonus int) int {
	switch p {
	case ProficiencyHalf:
		return proficiencyBonus / 2
	case ProficiencyProficient:
		return proficiencyBonus
	case ProficiencyExpertise:
		return proficiencyBonus * 2
	default:
		return 0
	}
}

// AbilityScores represents the six core D&D ability scores.
type AbilityScores struct {
	Strength     int `json:"strength" bson:"strength"`
	Dexterity    int `json:"dexterity" bson:"dexterity"`
	Constitution int `json:"constitution" bson:"constitution"`
	Intelligence int `json:"intelligence" bson:"intelligence"`
	Wisdom       int `json:"wisdom" bson:"wisdom"`
	Charisma     int `json:"charisma" bson:"charisma"`
}

// Score returns the raw score for an ability, or 10 (modifier +0) for an
// unrecognised one.
func (s AbilityScores) Score(a Ability) int {
	switch a {
	case AbilityStrength:
		return s.Strength
	case AbilityDexterity:
		return s.Dexterity
	case AbilityConstitution:
		return s.Constitution
	case AbilityIntelligence:
		return s.Intelligence
	case AbilityWisdom:
		return s.Wisdom
	case AbilityCharisma:
		return s.Charisma
	default:
		return 10
	}
}

// Modifier returns the ability modifier: floor((score - 10) / 2).
//
// Go truncates integer division toward zero, so scores below 10 need the
// floor applied explicitly -- (7-10)/2 is -1 in Go but -2 in D&D.
func (s AbilityScores) Modifier(a Ability) int {
	return AbilityModifier(s.Score(a))
}

// AbilityModifier converts a raw ability score into its modifier.
func AbilityModifier(score int) int {
	diff := score - 10
	if diff < 0 {
		// Round toward negative infinity, not toward zero.
		return -((-diff + 1) / 2)
	}
	return diff / 2
}

// ProficiencyBonusForLevel returns the 5e proficiency bonus for a character
// level: +2 at levels 1-4, rising by one every four levels to +6 at 17-20.
func ProficiencyBonusForLevel(level int) int {
	if level < 1 {
		level = 1
	}
	if level > 20 {
		level = 20
	}
	return 2 + (level-1)/4
}

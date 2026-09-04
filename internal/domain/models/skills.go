package models

// Skill is one of the eighteen D&D 5e skills.
type Skill string

const (
	SkillAcrobatics     Skill = "acrobatics"
	SkillAnimalHandling Skill = "animal_handling"
	SkillArcana         Skill = "arcana"
	SkillAthletics      Skill = "athletics"
	SkillDeception      Skill = "deception"
	SkillHistory        Skill = "history"
	SkillInsight        Skill = "insight"
	SkillIntimidation   Skill = "intimidation"
	SkillInvestigation  Skill = "investigation"
	SkillMedicine       Skill = "medicine"
	SkillNature         Skill = "nature"
	SkillPerception     Skill = "perception"
	SkillPerformance    Skill = "performance"
	SkillPersuasion     Skill = "persuasion"
	SkillReligion       Skill = "religion"
	SkillSleightOfHand  Skill = "sleight_of_hand"
	SkillStealth        Skill = "stealth"
	SkillSurvival       Skill = "survival"
)

// SkillAbility maps each skill to the ability its checks are made with.
// This table is the single source of truth; nothing should hardcode a pairing.
var SkillAbility = map[Skill]Ability{
	SkillAthletics:      AbilityStrength,
	SkillAcrobatics:     AbilityDexterity,
	SkillSleightOfHand:  AbilityDexterity,
	SkillStealth:        AbilityDexterity,
	SkillArcana:         AbilityIntelligence,
	SkillHistory:        AbilityIntelligence,
	SkillInvestigation:  AbilityIntelligence,
	SkillNature:         AbilityIntelligence,
	SkillReligion:       AbilityIntelligence,
	SkillAnimalHandling: AbilityWisdom,
	SkillInsight:        AbilityWisdom,
	SkillMedicine:       AbilityWisdom,
	SkillPerception:     AbilityWisdom,
	SkillSurvival:       AbilityWisdom,
	SkillDeception:      AbilityCharisma,
	SkillIntimidation:   AbilityCharisma,
	SkillPerformance:    AbilityCharisma,
	SkillPersuasion:     AbilityCharisma,
}

// Skills lists every skill in alphabetical order, for iteration and UI.
var Skills = []Skill{
	SkillAcrobatics, SkillAnimalHandling, SkillArcana, SkillAthletics,
	SkillDeception, SkillHistory, SkillInsight, SkillIntimidation,
	SkillInvestigation, SkillMedicine, SkillNature, SkillPerception,
	SkillPerformance, SkillPersuasion, SkillReligion, SkillSleightOfHand,
	SkillStealth, SkillSurvival,
}

// Valid reports whether s is one of the eighteen skills.
func (s Skill) Valid() bool {
	_, ok := SkillAbility[s]
	return ok
}

// Ability returns the ability a skill check uses.
func (s Skill) Ability() Ability {
	return SkillAbility[s]
}

// SkillProficiencies records how proficient a character is in each skill.
//
// A map rather than eighteen booleans: proficiency is four-valued in 5e, and
// an absent key reads as ProficiencyNone without needing a zero entry.
type SkillProficiencies map[Skill]Proficiency

// Level returns the proficiency level for a skill, defaulting to none.
func (sp SkillProficiencies) Level(s Skill) Proficiency {
	if sp == nil {
		return ProficiencyNone
	}
	return sp[s]
}

// SavingThrowProficiencies records saving throw proficiency per ability.
//
// Classes grant plain proficiency, but the map allows the other levels so
// features that grant partial or doubled saves stay representable.
type SavingThrowProficiencies map[Ability]Proficiency

// Level returns the proficiency level for a saving throw, defaulting to none.
func (sp SavingThrowProficiencies) Level(a Ability) Proficiency {
	if sp == nil {
		return ProficiencyNone
	}
	return sp[a]
}

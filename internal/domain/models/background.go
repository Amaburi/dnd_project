package models

// Background is a PHB character background.
type Background string

const (
	BackgroundAcolyte      Background = "acolyte"
	BackgroundCharlatan    Background = "charlatan"
	BackgroundCriminal     Background = "criminal"
	BackgroundEntertainer  Background = "entertainer"
	BackgroundFolkHero     Background = "folk_hero"
	BackgroundGuildArtisan Background = "guild_artisan"
	BackgroundHermit       Background = "hermit"
	BackgroundNoble        Background = "noble"
	BackgroundOutlander    Background = "outlander"
	BackgroundSage         Background = "sage"
	BackgroundSailor       Background = "sailor"
	BackgroundSoldier      Background = "soldier"
	BackgroundUrchin       Background = "urchin"
)

// BackgroundDefinition is what a background grants.
//
// Every background gives exactly two skill proficiencies. They are fixed, not
// chosen, which makes them the easiest half of a character's skill list to
// derive rather than trust.
type BackgroundDefinition struct {
	Background         Background
	SkillProficiencies []Skill
	ToolProficiencies  []string
	Languages          int // number of free languages
	Feature            string
}

// BackgroundTable is the single source of truth for background mechanics.
var BackgroundTable = map[Background]BackgroundDefinition{
	BackgroundAcolyte: {
		Background:         BackgroundAcolyte,
		SkillProficiencies: []Skill{SkillInsight, SkillReligion},
		Languages:          2,
		Feature:            "shelter_of_the_faithful",
	},
	BackgroundCharlatan: {
		Background:         BackgroundCharlatan,
		SkillProficiencies: []Skill{SkillDeception, SkillSleightOfHand},
		ToolProficiencies:  []string{"disguise_kit", "forgery_kit"},
		Feature:            "false_identity",
	},
	BackgroundCriminal: {
		Background:         BackgroundCriminal,
		SkillProficiencies: []Skill{SkillDeception, SkillStealth},
		ToolProficiencies:  []string{"thieves_tools", "gaming_set"},
		Feature:            "criminal_contact",
	},
	BackgroundEntertainer: {
		Background:         BackgroundEntertainer,
		SkillProficiencies: []Skill{SkillAcrobatics, SkillPerformance},
		ToolProficiencies:  []string{"disguise_kit", "musical_instrument"},
		Feature:            "by_popular_demand",
	},
	BackgroundFolkHero: {
		Background:         BackgroundFolkHero,
		SkillProficiencies: []Skill{SkillAnimalHandling, SkillSurvival},
		ToolProficiencies:  []string{"artisans_tools", "vehicles_land"},
		Feature:            "rustic_hospitality",
	},
	BackgroundGuildArtisan: {
		Background:         BackgroundGuildArtisan,
		SkillProficiencies: []Skill{SkillInsight, SkillPersuasion},
		ToolProficiencies:  []string{"artisans_tools"},
		Languages:          1,
		Feature:            "guild_membership",
	},
	BackgroundHermit: {
		Background:         BackgroundHermit,
		SkillProficiencies: []Skill{SkillMedicine, SkillReligion},
		ToolProficiencies:  []string{"herbalism_kit"},
		Languages:          1,
		Feature:            "discovery",
	},
	BackgroundNoble: {
		Background:         BackgroundNoble,
		SkillProficiencies: []Skill{SkillHistory, SkillPersuasion},
		ToolProficiencies:  []string{"gaming_set"},
		Languages:          1,
		Feature:            "position_of_privilege",
	},
	BackgroundOutlander: {
		Background:         BackgroundOutlander,
		SkillProficiencies: []Skill{SkillAthletics, SkillSurvival},
		ToolProficiencies:  []string{"musical_instrument"},
		Languages:          1,
		Feature:            "wanderer",
	},
	BackgroundSage: {
		Background:         BackgroundSage,
		SkillProficiencies: []Skill{SkillArcana, SkillHistory},
		Languages:          2,
		Feature:            "researcher",
	},
	BackgroundSailor: {
		Background:         BackgroundSailor,
		SkillProficiencies: []Skill{SkillAthletics, SkillPerception},
		ToolProficiencies:  []string{"navigators_tools", "vehicles_water"},
		Feature:            "ships_passage",
	},
	BackgroundSoldier: {
		Background:         BackgroundSoldier,
		SkillProficiencies: []Skill{SkillAthletics, SkillIntimidation},
		ToolProficiencies:  []string{"gaming_set", "vehicles_land"},
		Feature:            "military_rank",
	},
	BackgroundUrchin: {
		Background:         BackgroundUrchin,
		SkillProficiencies: []Skill{SkillSleightOfHand, SkillStealth},
		ToolProficiencies:  []string{"disguise_kit", "thieves_tools"},
		Feature:            "city_secrets",
	},
}

// Backgrounds lists every background.
var Backgrounds = []Background{
	BackgroundAcolyte, BackgroundCharlatan, BackgroundCriminal, BackgroundEntertainer,
	BackgroundFolkHero, BackgroundGuildArtisan, BackgroundHermit, BackgroundNoble,
	BackgroundOutlander, BackgroundSage, BackgroundSailor, BackgroundSoldier,
	BackgroundUrchin,
}

// Valid reports whether b is a known background.
func (b Background) Valid() bool {
	_, ok := BackgroundTable[b]
	return ok
}

// Definition returns the background's mechanics, and whether it is known.
func (b Background) Definition() (BackgroundDefinition, bool) {
	def, ok := BackgroundTable[b]
	return def, ok
}

// GrantedSkills returns the two skill proficiencies a background provides.
func (b Background) GrantedSkills() []Skill {
	return BackgroundTable[b].SkillProficiencies
}

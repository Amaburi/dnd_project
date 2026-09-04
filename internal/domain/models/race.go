package models

// Race is a PHB player race.
type Race string

const (
	RaceDwarf      Race = "dwarf"
	RaceElf        Race = "elf"
	RaceHalfling   Race = "halfling"
	RaceHuman      Race = "human"
	RaceDragonborn Race = "dragonborn"
	RaceGnome      Race = "gnome"
	RaceHalfElf    Race = "half_elf"
	RaceHalfOrc    Race = "half_orc"
	RaceTiefling   Race = "tiefling"
)

// AbilityBonus is a racial increase to one ability score.
type AbilityBonus struct {
	Ability Ability `json:"ability" bson:"ability"`
	Bonus   int     `json:"bonus" bson:"bonus"`
}

// RaceDefinition is what a race contributes mechanically.
type RaceDefinition struct {
	Race Race
	Size CreatureSize

	// Speed is the walking speed in feet.
	Speed int

	// AbilityBonuses are the fixed increases. Half-elves also choose two,
	// which is what AbilityChoices counts.
	AbilityBonuses []AbilityBonus
	AbilityChoices int

	// Darkvision is the range in feet, or 0 for none.
	Darkvision int

	// SkillProficiencies are granted outright; SkillChoices is how many the
	// player picks freely on top (half-elf's two).
	SkillProficiencies []Skill
	SkillChoices       int

	Traits   []string
	Subraces map[string]SubraceDefinition
}

// SubraceDefinition is what a subrace adds on top of its race.
type SubraceDefinition struct {
	Name           string
	AbilityBonuses []AbilityBonus

	// SpeedOverride replaces the parent race's speed when non-zero (wood
	// elves move at 35 feet).
	SpeedOverride int

	// DarkvisionOverride replaces the parent's darkvision when non-zero
	// (drow see twice as far as other elves).
	DarkvisionOverride int

	SkillProficiencies []Skill
	Traits             []string
}

// RaceTable is the single source of truth for racial mechanics.
var RaceTable = map[Race]RaceDefinition{
	RaceDwarf: {
		Race: RaceDwarf, Size: SizeMedium, Speed: 25, Darkvision: 60,
		AbilityBonuses: []AbilityBonus{{AbilityConstitution, 2}},
		Traits:         []string{"dwarven_resilience", "stonecunning"},
		Subraces: map[string]SubraceDefinition{
			"hill": {Name: "hill", AbilityBonuses: []AbilityBonus{{AbilityWisdom, 1}},
				Traits: []string{"dwarven_toughness"}},
			"mountain": {Name: "mountain", AbilityBonuses: []AbilityBonus{{AbilityStrength, 2}},
				Traits: []string{"dwarven_armor_training"}},
		},
	},
	RaceElf: {
		Race: RaceElf, Size: SizeMedium, Speed: 30, Darkvision: 60,
		AbilityBonuses:     []AbilityBonus{{AbilityDexterity, 2}},
		SkillProficiencies: []Skill{SkillPerception},
		Traits:             []string{"fey_ancestry", "trance"},
		Subraces: map[string]SubraceDefinition{
			"high": {Name: "high", AbilityBonuses: []AbilityBonus{{AbilityIntelligence, 1}},
				Traits: []string{"cantrip", "elf_weapon_training"}},
			"wood": {Name: "wood", AbilityBonuses: []AbilityBonus{{AbilityWisdom, 1}},
				SpeedOverride: 35, Traits: []string{"mask_of_the_wild", "elf_weapon_training"}},
			"drow": {Name: "drow", AbilityBonuses: []AbilityBonus{{AbilityCharisma, 1}},
				DarkvisionOverride: 120, Traits: []string{"sunlight_sensitivity", "drow_magic"}},
		},
	},
	RaceHalfling: {
		Race: RaceHalfling, Size: SizeSmall, Speed: 25,
		AbilityBonuses: []AbilityBonus{{AbilityDexterity, 2}},
		Traits:         []string{"lucky", "brave", "halfling_nimbleness"},
		Subraces: map[string]SubraceDefinition{
			"lightfoot": {Name: "lightfoot", AbilityBonuses: []AbilityBonus{{AbilityCharisma, 1}},
				Traits: []string{"naturally_stealthy"}},
			"stout": {Name: "stout", AbilityBonuses: []AbilityBonus{{AbilityConstitution, 1}},
				Traits: []string{"stout_resilience"}},
		},
	},
	RaceHuman: {
		Race: RaceHuman, Size: SizeMedium, Speed: 30,
		AbilityBonuses: []AbilityBonus{
			{AbilityStrength, 1}, {AbilityDexterity, 1}, {AbilityConstitution, 1},
			{AbilityIntelligence, 1}, {AbilityWisdom, 1}, {AbilityCharisma, 1},
		},
	},
	RaceDragonborn: {
		Race: RaceDragonborn, Size: SizeMedium, Speed: 30,
		AbilityBonuses: []AbilityBonus{{AbilityStrength, 2}, {AbilityCharisma, 1}},
		Traits:         []string{"draconic_ancestry", "breath_weapon", "damage_resistance"},
	},
	RaceGnome: {
		Race: RaceGnome, Size: SizeSmall, Speed: 25, Darkvision: 60,
		AbilityBonuses: []AbilityBonus{{AbilityIntelligence, 2}},
		Traits:         []string{"gnome_cunning"},
		Subraces: map[string]SubraceDefinition{
			"forest": {Name: "forest", AbilityBonuses: []AbilityBonus{{AbilityDexterity, 1}},
				Traits: []string{"natural_illusionist", "speak_with_small_beasts"}},
			"rock": {Name: "rock", AbilityBonuses: []AbilityBonus{{AbilityConstitution, 1}},
				Traits: []string{"artificers_lore", "tinker"}},
		},
	},
	RaceHalfElf: {
		Race: RaceHalfElf, Size: SizeMedium, Speed: 30, Darkvision: 60,
		AbilityBonuses: []AbilityBonus{{AbilityCharisma, 2}},
		// Plus +1 to two other abilities of the player's choice.
		AbilityChoices: 2,
		SkillChoices:   2,
		Traits:         []string{"fey_ancestry", "skill_versatility"},
	},
	RaceHalfOrc: {
		Race: RaceHalfOrc, Size: SizeMedium, Speed: 30, Darkvision: 60,
		AbilityBonuses:     []AbilityBonus{{AbilityStrength, 2}, {AbilityConstitution, 1}},
		SkillProficiencies: []Skill{SkillIntimidation},
		Traits:             []string{"relentless_endurance", "savage_attacks"},
	},
	RaceTiefling: {
		Race: RaceTiefling, Size: SizeMedium, Speed: 30, Darkvision: 60,
		AbilityBonuses: []AbilityBonus{{AbilityIntelligence, 1}, {AbilityCharisma, 2}},
		Traits:         []string{"hellish_resistance", "infernal_legacy"},
	},
}

// Races lists every race.
var Races = []Race{
	RaceDwarf, RaceElf, RaceHalfling, RaceHuman, RaceDragonborn,
	RaceGnome, RaceHalfElf, RaceHalfOrc, RaceTiefling,
}

// Valid reports whether r is a known race.
func (r Race) Valid() bool {
	_, ok := RaceTable[r]
	return ok
}

// Definition returns the race's mechanics, and whether the race is known.
func (r Race) Definition() (RaceDefinition, bool) {
	def, ok := RaceTable[r]
	return def, ok
}

// HasSubrace reports whether name is a valid subrace for this race.
func (r Race) HasSubrace(name string) bool {
	def, ok := RaceTable[r]
	if !ok {
		return false
	}
	_, found := def.Subraces[name]
	return found
}

// RequiresSubrace reports whether the race defines subraces to choose from.
func (r Race) RequiresSubrace() bool {
	return len(RaceTable[r].Subraces) > 0
}

// Speed returns the walking speed for a race and subrace in feet.
func (r Race) Speed(subrace string) int {
	def, ok := RaceTable[r]
	if !ok {
		return 30
	}
	if sub, found := def.Subraces[subrace]; found && sub.SpeedOverride > 0 {
		return sub.SpeedOverride
	}
	return def.Speed
}

// Darkvision returns the darkvision range in feet, or 0 for none.
func (r Race) Darkvision(subrace string) int {
	def, ok := RaceTable[r]
	if !ok {
		return 0
	}
	if sub, found := def.Subraces[subrace]; found && sub.DarkvisionOverride > 0 {
		return sub.DarkvisionOverride
	}
	return def.Darkvision
}

// AbilityBonuses returns the combined fixed increases from race and subrace.
// The free choices half-elves get are not included; those are recorded on the
// character.
func (r Race) AbilityBonuses(subrace string) []AbilityBonus {
	def, ok := RaceTable[r]
	if !ok {
		return nil
	}

	bonuses := append([]AbilityBonus(nil), def.AbilityBonuses...)
	if sub, found := def.Subraces[subrace]; found {
		bonuses = append(bonuses, sub.AbilityBonuses...)
	}
	return bonuses
}

// GrantedSkills returns the skills a race and subrace grant outright.
func (r Race) GrantedSkills(subrace string) []Skill {
	def, ok := RaceTable[r]
	if !ok {
		return nil
	}

	skills := append([]Skill(nil), def.SkillProficiencies...)
	if sub, found := def.Subraces[subrace]; found {
		skills = append(skills, sub.SkillProficiencies...)
	}
	return skills
}

// ApplyRacialBonuses adds a race's ability increases to a set of base scores.
//
// Character.AbilityScores stores the *final* numbers written on the sheet, as
// a paper character sheet does. This is a creation-time helper for arriving at
// them, not something applied on every read -- applying it twice would inflate
// the character.
func ApplyRacialBonuses(base AbilityScores, r Race, subrace string, chosen []Ability) AbilityScores {
	out := base
	add := func(a Ability, n int) {
		switch a {
		case AbilityStrength:
			out.Strength += n
		case AbilityDexterity:
			out.Dexterity += n
		case AbilityConstitution:
			out.Constitution += n
		case AbilityIntelligence:
			out.Intelligence += n
		case AbilityWisdom:
			out.Wisdom += n
		case AbilityCharisma:
			out.Charisma += n
		}
	}

	for _, b := range r.AbilityBonuses(subrace) {
		add(b.Ability, b.Bonus)
	}

	// Half-elves add +1 to two abilities of their choice.
	limit := RaceTable[r].AbilityChoices
	for i, a := range chosen {
		if i >= limit {
			break
		}
		add(a, 1)
	}
	return out
}

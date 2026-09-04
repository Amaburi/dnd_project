package models

import "fmt"

// Race is a player race.
type Race string

const (
	// PHB
	RaceDwarf      Race = "dwarf"
	RaceElf        Race = "elf"
	RaceHalfling   Race = "halfling"
	RaceHuman      Race = "human"
	RaceDragonborn Race = "dragonborn"
	RaceGnome      Race = "gnome"
	RaceHalfElf    Race = "half_elf"
	RaceHalfOrc    Race = "half_orc"
	RaceTiefling   Race = "tiefling"

	// Beyond the PHB. Each carries its Source so a campaign can restrict
	// itself to core material without guessing where an entry came from.
	RaceAasimar Race = "aasimar"
	RaceGenasi  Race = "genasi"
	RaceGoliath Race = "goliath"
	RaceTabaxi  Race = "tabaxi"
	RaceFirbolg Race = "firbolg"
)

// Source labels for content origin.
const (
	SourceVGtM = "VGtM" // Volo's Guide to Monsters
	SourceEEPC = "EEPC" // Elemental Evil Player's Companion
	SourceSCAG = "SCAG" // Sword Coast Adventurer's Guide
)

// AbilityBonus is a racial increase to one ability score.
type AbilityBonus struct {
	Ability Ability `json:"ability" bson:"ability"`
	Bonus   int     `json:"bonus" bson:"bonus"`
}

// BreathWeapon describes a dragonborn's draconic breath.
type BreathWeapon struct {
	DamageType  DamageType `json:"damage_type" bson:"damage_type"`
	Shape       string     `json:"shape" bson:"shape"` // "line" or "cone"
	Area        string     `json:"area" bson:"area"`   // "5 by 30 ft." or "15 ft."
	SaveAbility Ability    `json:"save_ability" bson:"save_ability"`
}

// RaceDefinition is what a race contributes mechanically.
type RaceDefinition struct {
	Race   Race
	Name   string
	Source string
	Size   CreatureSize

	// Speed is the walking speed in feet.
	Speed int

	// AbilityBonuses are the fixed increases; AbilityChoices counts the free
	// +1s a player assigns (a half-elf's two).
	AbilityBonuses []AbilityBonus
	AbilityChoices int

	// Darkvision is the range in feet, or 0 for none.
	Darkvision int

	// SkillProficiencies are granted outright; SkillChoices is how many the
	// player picks freely on top.
	SkillProficiencies []Skill
	SkillChoices       int

	// Languages every member of the race speaks, plus how many the player
	// picks freely.
	Languages       []string
	LanguageChoices int

	// Proficiencies are the armour, weapon and tool trainings a race grants.
	// These used to exist only as trait names, which meant a mountain dwarf
	// wizard was not actually proficient with the medium armour their race
	// trains them in.
	Proficiencies Proficiencies

	// ToolChoices is how many tools the player picks from ToolOptions, which
	// is how a dwarf chooses between smith's, brewer's and mason's tools.
	ToolChoices int
	ToolOptions []string

	Traits   []string
	Subraces map[string]SubraceDefinition
}

// SubraceDefinition is what a subrace adds on top of its race.
type SubraceDefinition struct {
	Name   string
	Source string

	AbilityBonuses []AbilityBonus
	AbilityChoices int

	// SpeedOverride replaces the parent race's speed when non-zero (wood
	// elves move at 35 feet).
	SpeedOverride int

	// DarkvisionOverride replaces the parent's darkvision when non-zero
	// (drow see twice as far as other elves; fire genasi see where their
	// siblings do not).
	DarkvisionOverride int

	SkillProficiencies []Skill
	SkillChoices       int
	Proficiencies      Proficiencies

	LanguageChoices int

	// CantripChoices is how many cantrips the subrace grants (a high elf's
	// one wizard cantrip), and FeatChoices how many feats (variant human).
	CantripChoices int
	FeatChoices    int

	// BonusHitPointsPerLevel is added to the hit point maximum each level,
	// which is the whole of a hill dwarf's Dwarven Toughness.
	BonusHitPointsPerLevel int

	DamageResistances []DamageType
	Breath            *BreathWeapon

	Traits []string
}

// dragonborn builds one draconic ancestry.
func ancestry(name string, dt DamageType, shape, area string, save Ability) SubraceDefinition {
	return SubraceDefinition{
		Name: name, Source: SourcePHB,
		DamageResistances: []DamageType{dt},
		Breath:            &BreathWeapon{DamageType: dt, Shape: shape, Area: area, SaveAbility: save},
	}
}

const (
	breathLine = "line"
	breathCone = "cone"
	lineArea   = "5 by 30 ft."
	coneArea   = "15 ft."
)

// RaceTable is the single source of truth for racial mechanics.
var RaceTable = map[Race]RaceDefinition{
	RaceDwarf: {
		Race: RaceDwarf, Name: "Dwarf", Source: SourcePHB,
		Size: SizeMedium, Speed: 25, Darkvision: 60,
		Languages:      []string{"common", "dwarvish"},
		AbilityBonuses: []AbilityBonus{{AbilityConstitution, 2}},
		// Dwarven Combat Training.
		Proficiencies: Proficiencies{
			Weapons: []string{"battleaxe", "handaxe", "light_hammer", "warhammer"},
		},
		ToolChoices: 1,
		ToolOptions: []string{"smiths_tools", "brewers_supplies", "masons_tools"},
		Traits:      []string{"dwarven_resilience", "stonecunning", "dwarven_combat_training"},
		Subraces: map[string]SubraceDefinition{
			"hill": {
				Name: "Hill Dwarf", Source: SourcePHB,
				AbilityBonuses:         []AbilityBonus{{AbilityWisdom, 1}},
				BonusHitPointsPerLevel: 1, // Dwarven Toughness
				Traits:                 []string{"dwarven_toughness"},
			},
			"mountain": {
				Name: "Mountain Dwarf", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{{AbilityStrength, 2}},
				// Dwarven Armor Training.
				Proficiencies: Proficiencies{Armor: []string{ProfLightArmor, ProfMediumArmor}},
				Traits:        []string{"dwarven_armor_training"},
			},
		},
	},
	RaceElf: {
		Race: RaceElf, Name: "Elf", Source: SourcePHB,
		Size: SizeMedium, Speed: 30, Darkvision: 60,
		Languages:          []string{"common", "elvish"},
		AbilityBonuses:     []AbilityBonus{{AbilityDexterity, 2}},
		SkillProficiencies: []Skill{SkillPerception},
		Traits:             []string{"fey_ancestry", "trance", "keen_senses"},
		Subraces: map[string]SubraceDefinition{
			"high": {
				Name: "High Elf", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{{AbilityIntelligence, 1}},
				Proficiencies: Proficiencies{
					Weapons: []string{"longsword", "shortsword", "shortbow", "longbow"},
				},
				LanguageChoices: 1,
				CantripChoices:  1, // one wizard cantrip
				Traits:          []string{"cantrip", "elf_weapon_training", "extra_language"},
			},
			"wood": {
				Name: "Wood Elf", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{{AbilityWisdom, 1}},
				SpeedOverride:  35,
				Proficiencies: Proficiencies{
					Weapons: []string{"longsword", "shortsword", "shortbow", "longbow"},
				},
				Traits: []string{"mask_of_the_wild", "elf_weapon_training"},
			},
			"drow": {
				Name: "Dark Elf (Drow)", Source: SourcePHB,
				AbilityBonuses:     []AbilityBonus{{AbilityCharisma, 1}},
				DarkvisionOverride: 120,
				Proficiencies: Proficiencies{
					Weapons: []string{"rapier", "shortsword", "hand_crossbow"},
				},
				Traits: []string{"sunlight_sensitivity", "drow_magic", "drow_weapon_training"},
			},
		},
	},
	RaceHalfling: {
		Race: RaceHalfling, Name: "Halfling", Source: SourcePHB,
		Size: SizeSmall, Speed: 25,
		Languages:      []string{"common", "halfling"},
		AbilityBonuses: []AbilityBonus{{AbilityDexterity, 2}},
		Traits:         []string{"lucky", "brave", "halfling_nimbleness"},
		Subraces: map[string]SubraceDefinition{
			"lightfoot": {
				Name: "Lightfoot Halfling", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{{AbilityCharisma, 1}},
				Traits:         []string{"naturally_stealthy"},
			},
			"stout": {
				Name: "Stout Halfling", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{{AbilityConstitution, 1}},
				Traits:         []string{"stout_resilience"},
			},
		},
	},
	RaceHuman: {
		Race: RaceHuman, Name: "Human", Source: SourcePHB,
		Size: SizeMedium, Speed: 30,
		Languages: []string{"common"}, LanguageChoices: 1,
		// The ability spread lives on the subraces: standard humans raise
		// every score, variants raise two and take a feat instead.
		Subraces: map[string]SubraceDefinition{
			"standard": {
				Name: "Human", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{
					{AbilityStrength, 1}, {AbilityDexterity, 1}, {AbilityConstitution, 1},
					{AbilityIntelligence, 1}, {AbilityWisdom, 1}, {AbilityCharisma, 1},
				},
			},
			"variant": {
				Name: "Variant Human", Source: SourcePHB,
				AbilityChoices: 2,
				SkillChoices:   1,
				FeatChoices:    1,
				Traits:         []string{"variant_feat", "variant_skill"},
			},
		},
	},
	RaceDragonborn: {
		Race: RaceDragonborn, Name: "Dragonborn", Source: SourcePHB,
		Size: SizeMedium, Speed: 30,
		Languages:      []string{"common", "draconic"},
		AbilityBonuses: []AbilityBonus{{AbilityStrength, 2}, {AbilityCharisma, 1}},
		Traits:         []string{"draconic_ancestry", "breath_weapon", "damage_resistance"},
		// Draconic ancestry is a required choice that decides the breath
		// weapon's damage type, shape and save, so it is modelled as a
		// subrace rather than left as an unread trait name.
		Subraces: map[string]SubraceDefinition{
			"black":  ancestry("Black Dragonborn", DamageAcid, breathLine, lineArea, AbilityDexterity),
			"blue":   ancestry("Blue Dragonborn", DamageLightning, breathLine, lineArea, AbilityDexterity),
			"brass":  ancestry("Brass Dragonborn", DamageFire, breathLine, lineArea, AbilityDexterity),
			"bronze": ancestry("Bronze Dragonborn", DamageLightning, breathLine, lineArea, AbilityDexterity),
			"copper": ancestry("Copper Dragonborn", DamageAcid, breathLine, lineArea, AbilityDexterity),
			"gold":   ancestry("Gold Dragonborn", DamageFire, breathCone, coneArea, AbilityDexterity),
			"green":  ancestry("Green Dragonborn", DamagePoison, breathCone, coneArea, AbilityConstitution),
			"red":    ancestry("Red Dragonborn", DamageFire, breathCone, coneArea, AbilityDexterity),
			"silver": ancestry("Silver Dragonborn", DamageCold, breathCone, coneArea, AbilityConstitution),
			"white":  ancestry("White Dragonborn", DamageCold, breathCone, coneArea, AbilityConstitution),
		},
	},
	RaceGnome: {
		Race: RaceGnome, Name: "Gnome", Source: SourcePHB,
		Size: SizeSmall, Speed: 25, Darkvision: 60,
		Languages:      []string{"common", "gnomish"},
		AbilityBonuses: []AbilityBonus{{AbilityIntelligence, 2}},
		Traits:         []string{"gnome_cunning"},
		Subraces: map[string]SubraceDefinition{
			"forest": {
				Name: "Forest Gnome", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{{AbilityDexterity, 1}},
				CantripChoices: 1, // minor illusion
				Traits:         []string{"natural_illusionist", "speak_with_small_beasts"},
			},
			"rock": {
				Name: "Rock Gnome", Source: SourcePHB,
				AbilityBonuses: []AbilityBonus{{AbilityConstitution, 1}},
				Proficiencies:  Proficiencies{Tools: []string{"tinkers_tools"}},
				Traits:         []string{"artificers_lore", "tinker"},
			},
		},
	},
	RaceHalfElf: {
		Race: RaceHalfElf, Name: "Half-Elf", Source: SourcePHB,
		Size: SizeMedium, Speed: 30, Darkvision: 60,
		Languages: []string{"common", "elvish"}, LanguageChoices: 1,
		AbilityBonuses: []AbilityBonus{{AbilityCharisma, 2}},
		AbilityChoices: 2,
		SkillChoices:   2,
		Traits:         []string{"fey_ancestry", "skill_versatility"},
	},
	RaceHalfOrc: {
		Race: RaceHalfOrc, Name: "Half-Orc", Source: SourcePHB,
		Size: SizeMedium, Speed: 30, Darkvision: 60,
		Languages:          []string{"common", "orc"},
		AbilityBonuses:     []AbilityBonus{{AbilityStrength, 2}, {AbilityConstitution, 1}},
		SkillProficiencies: []Skill{SkillIntimidation},
		Traits:             []string{"relentless_endurance", "savage_attacks", "menacing"},
	},
	RaceTiefling: {
		Race: RaceTiefling, Name: "Tiefling", Source: SourcePHB,
		Size: SizeMedium, Speed: 30, Darkvision: 60,
		Languages:      []string{"common", "infernal"},
		AbilityBonuses: []AbilityBonus{{AbilityIntelligence, 1}, {AbilityCharisma, 2}},
		Traits:         []string{"hellish_resistance", "infernal_legacy"},
	},

	// ---- Beyond the PHB ---------------------------------------------------

	RaceAasimar: {
		Race: RaceAasimar, Name: "Aasimar", Source: SourceVGtM,
		Size: SizeMedium, Speed: 30, Darkvision: 60,
		Languages:      []string{"common", "celestial"},
		AbilityBonuses: []AbilityBonus{{AbilityCharisma, 2}},
		Traits:         []string{"celestial_resistance", "healing_hands", "light_bearer"},
		Subraces: map[string]SubraceDefinition{
			"protector": {Name: "Protector Aasimar", Source: SourceVGtM,
				AbilityBonuses: []AbilityBonus{{AbilityWisdom, 1}},
				Traits:         []string{"radiant_soul"}},
			"scourge": {Name: "Scourge Aasimar", Source: SourceVGtM,
				AbilityBonuses: []AbilityBonus{{AbilityConstitution, 1}},
				Traits:         []string{"radiant_consumption"}},
			"fallen": {Name: "Fallen Aasimar", Source: SourceVGtM,
				AbilityBonuses: []AbilityBonus{{AbilityStrength, 1}},
				Traits:         []string{"necrotic_shroud"}},
		},
	},
	RaceGenasi: {
		Race: RaceGenasi, Name: "Genasi", Source: SourceEEPC,
		Size: SizeMedium, Speed: 30,
		Languages:      []string{"common", "primordial"},
		AbilityBonuses: []AbilityBonus{{AbilityConstitution, 2}},
		Subraces: map[string]SubraceDefinition{
			"air": {Name: "Air Genasi", Source: SourceEEPC,
				AbilityBonuses: []AbilityBonus{{AbilityDexterity, 1}},
				Traits:         []string{"unending_breath", "mingle_with_the_wind"}},
			"earth": {Name: "Earth Genasi", Source: SourceEEPC,
				AbilityBonuses: []AbilityBonus{{AbilityStrength, 1}},
				Traits:         []string{"earth_walk", "merge_with_stone"}},
			"fire": {Name: "Fire Genasi", Source: SourceEEPC,
				AbilityBonuses:     []AbilityBonus{{AbilityIntelligence, 1}},
				DarkvisionOverride: 60,
				DamageResistances:  []DamageType{DamageFire},
				Traits:             []string{"fire_resistance", "reach_to_the_blaze"}},
			"water": {Name: "Water Genasi", Source: SourceEEPC,
				AbilityBonuses:    []AbilityBonus{{AbilityWisdom, 1}},
				DamageResistances: []DamageType{DamageAcid},
				Traits:            []string{"acid_resistance", "amphibious", "swim", "call_to_the_wave"}},
		},
	},
	RaceGoliath: {
		Race: RaceGoliath, Name: "Goliath", Source: SourceVGtM,
		Size: SizeMedium, Speed: 30,
		Languages:          []string{"common", "giant"},
		AbilityBonuses:     []AbilityBonus{{AbilityStrength, 2}, {AbilityConstitution, 1}},
		SkillProficiencies: []Skill{SkillAthletics},
		Traits:             []string{"natural_athlete", "stones_endurance", "powerful_build", "mountain_born"},
	},
	RaceTabaxi: {
		Race: RaceTabaxi, Name: "Tabaxi", Source: SourceVGtM,
		Size: SizeMedium, Speed: 30, Darkvision: 60,
		Languages: []string{"common"}, LanguageChoices: 1,
		AbilityBonuses:     []AbilityBonus{{AbilityDexterity, 2}, {AbilityCharisma, 1}},
		SkillProficiencies: []Skill{SkillPerception, SkillStealth},
		Traits:             []string{"feline_agility", "cats_claws", "cats_talent"},
	},
	RaceFirbolg: {
		Race: RaceFirbolg, Name: "Firbolg", Source: SourceVGtM,
		Size: SizeMedium, Speed: 30,
		Languages:      []string{"common", "elvish", "giant"},
		AbilityBonuses: []AbilityBonus{{AbilityWisdom, 2}, {AbilityStrength, 1}},
		Traits:         []string{"firbolg_magic", "hidden_step", "powerful_build", "speech_of_beast_and_leaf"},
	},
}

// Races lists every race, PHB first.
var Races = []Race{
	RaceDwarf, RaceElf, RaceHalfling, RaceHuman, RaceDragonborn,
	RaceGnome, RaceHalfElf, RaceHalfOrc, RaceTiefling,
	RaceAasimar, RaceGenasi, RaceGoliath, RaceTabaxi, RaceFirbolg,
}

// RacesFromSource returns the races published in a given book.
func RacesFromSource(source string) []Race {
	var out []Race
	for _, r := range Races {
		if RaceTable[r].Source == source {
			out = append(out, r)
		}
	}
	return out
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

// Subrace looks a subrace up by key.
func (r Race) Subrace(key string) (SubraceDefinition, bool) {
	def, ok := RaceTable[r]
	if !ok {
		return SubraceDefinition{}, false
	}
	sub, found := def.Subraces[key]
	return sub, found
}

// HasSubrace reports whether name is a valid subrace for this race.
func (r Race) HasSubrace(name string) bool {
	_, ok := r.Subrace(name)
	return ok
}

// RequiresSubrace reports whether the race defines subraces to choose from.
func (r Race) RequiresSubrace() bool {
	return len(RaceTable[r].Subraces) > 0
}

// SubraceKeys lists the subrace keys for a race.
func (r Race) SubraceKeys() []string {
	def, ok := RaceTable[r]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(def.Subraces))
	for key := range def.Subraces {
		keys = append(keys, key)
	}
	return keys
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
// The free choices some races grant are not included; those are recorded on
// the character.
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

// AbilityChoiceCount returns how many free +1s a race and subrace grant.
func (r Race) AbilityChoiceCount(subrace string) int {
	def, ok := RaceTable[r]
	if !ok {
		return 0
	}
	count := def.AbilityChoices
	if sub, found := def.Subraces[subrace]; found {
		count += sub.AbilityChoices
	}
	return count
}

// GrantedLanguages returns the languages a race knows without choosing.
func (r Race) GrantedLanguages() []string {
	return append([]string(nil), RaceTable[r].Languages...)
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

// SkillChoiceCount returns how many skills a race and subrace let the player
// choose freely.
func (r Race) SkillChoiceCount(subrace string) int {
	def, ok := RaceTable[r]
	if !ok {
		return 0
	}
	count := def.SkillChoices
	if sub, found := def.Subraces[subrace]; found {
		count += sub.SkillChoices
	}
	return count
}

// RacialProficiencies returns the armour, weapon and tool trainings a race and
// subrace grant.
//
// These were previously only trait names, so a mountain dwarf's armour
// training and an elf's weapon training never reached the character sheet and
// never affected an attack bonus.
func (r Race) RacialProficiencies(subrace string) Proficiencies {
	def, ok := RaceTable[r]
	if !ok {
		return Proficiencies{}
	}

	out := Proficiencies{}
	out.Merge(def.Proficiencies)
	if sub, found := def.Subraces[subrace]; found {
		out.Merge(sub.Proficiencies)
	}
	out.Languages = addUnique(out.Languages, def.Languages...)
	return out
}

// BonusHitPointsPerLevel is the extra maximum hit point a subrace grants each
// level -- Dwarven Toughness, and nothing else in the PHB.
func (r Race) BonusHitPointsPerLevel(subrace string) int {
	if sub, ok := r.Subrace(subrace); ok {
		return sub.BonusHitPointsPerLevel
	}
	return 0
}

// DamageResistances returns the damage types a race and subrace resist.
func (r Race) DamageResistances(subrace string) []DamageType {
	if sub, ok := r.Subrace(subrace); ok {
		return append([]DamageType(nil), sub.DamageResistances...)
	}
	return nil
}

// Breath returns the breath weapon a draconic ancestry grants.
func (r Race) Breath(subrace string) (BreathWeapon, bool) {
	if sub, ok := r.Subrace(subrace); ok && sub.Breath != nil {
		return *sub.Breath, true
	}
	return BreathWeapon{}, false
}

// BreathWeaponDice returns the damage dice a dragonborn's breath deals at a
// character level: 2d6, rising by one die at 6th, 11th and 16th.
func BreathWeaponDice(level int) string {
	dice := 2
	switch {
	case level >= 16:
		dice = 5
	case level >= 11:
		dice = 4
	case level >= 6:
		dice = 3
	}
	return fmt.Sprintf("%dd6", dice)
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

	limit := r.AbilityChoiceCount(subrace)
	for i, a := range chosen {
		if i >= limit {
			break
		}
		add(a, 1)
	}
	return out
}

package models

// cantripsKnown maps a caster class to the number of cantrips known at each
// level, indexed 1-20.
//
// Prepared casters still have a fixed cantrip count -- cantrips are never
// prepared, they are simply known.
var cantripsKnown = map[Class][21]int{
	ClassBard:     {0, 2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
	ClassCleric:   {0, 3, 3, 3, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
	ClassDruid:    {0, 2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
	ClassSorcerer: {0, 4, 4, 4, 5, 5, 5, 5, 5, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6},
	ClassWarlock:  {0, 2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
	ClassWizard:   {0, 3, 3, 3, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
}

// spellsKnown maps the classes that *know* a fixed list of spells to how many
// they know at each level.
//
// Clerics, druids, paladins and wizards are absent on purpose: they prepare
// from a wider list rather than knowing a fixed set, so their limit is a
// prepared count instead (see PreparedSpellLimit).
var spellsKnown = map[Class][21]int{
	ClassBard:     {0, 4, 5, 6, 7, 8, 9, 10, 11, 12, 14, 15, 15, 16, 18, 19, 19, 20, 22, 22, 22},
	ClassSorcerer: {0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 12, 13, 13, 14, 14, 15, 15, 15, 15},
	ClassWarlock:  {0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15},
	ClassRanger:   {0, 0, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11},
}

// thirdCasterCantrips and thirdCasterSpells cover the Eldritch Knight and
// Arcane Trickster, whose counts come from the archetype rather than the
// class.
//
// NOTE: these two tables are the least certain in this file. They are printed
// inside the fighter and rogue archetype descriptions rather than in a class
// table, and are worth checking against the PHB before relying on them.
var (
	thirdCasterCantrips = map[string][21]int{
		"eldritch_knight":  {0, 0, 0, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
		"arcane_trickster": {0, 0, 0, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
	}
	thirdCasterSpells = [21]int{0, 0, 0, 3, 4, 4, 4, 5, 6, 6, 7, 8, 8, 9, 10, 10, 11, 11, 11, 12, 13}
)

func lookupLevel(table [21]int, level int) int {
	if level < 1 {
		return 0
	}
	if level > 20 {
		level = 20
	}
	return table[level]
}

// CantripsKnown returns how many cantrips a class level grants.
func (cl ClassLevel) CantripsKnown() int {
	if table, ok := cantripsKnown[cl.Class]; ok {
		return lookupLevel(table, cl.Level)
	}
	if table, ok := thirdCasterCantrips[cl.Subclass]; ok {
		return lookupLevel(table, cl.Level)
	}
	return 0
}

// SpellsKnown returns how many spells a class level grants to a class that
// knows a fixed list, or 0 for a preparing class.
func (cl ClassLevel) SpellsKnown() int {
	if table, ok := spellsKnown[cl.Class]; ok {
		return lookupLevel(table, cl.Level)
	}
	if _, ok := thirdCasterCantrips[cl.Subclass]; ok {
		return lookupLevel(thirdCasterSpells, cl.Level)
	}
	return 0
}

// PreparesSpells reports whether a class prepares from a list each day rather
// than knowing a fixed set.
func (c Class) PreparesSpells() bool {
	switch c {
	case ClassCleric, ClassDruid, ClassPaladin, ClassWizard:
		return true
	}
	return false
}

// PreparedSpellLimit returns how many spells a preparing class may have
// prepared, and whether the class prepares at all.
//
// The formula is the spellcasting ability modifier plus the class level, with
// paladins using half their level (rounded down) because they are half
// casters. The result is never below one.
func (c *Character) PreparedSpellLimit(class Class) (int, bool) {
	if !class.PreparesSpells() {
		return 0, false
	}

	level := c.BasicInfo.LevelIn(class)
	if level == 0 {
		return 0, false
	}
	if class == ClassPaladin {
		level /= 2
	}

	def, ok := ClassTable[class]
	if !ok {
		return 0, false
	}

	limit := level + c.AbilityModifier(def.SpellcastingAbility)
	if limit < 1 {
		limit = 1
	}
	return limit, true
}

// expertiseGrants maps a class to the levels at which it grants a pair of
// expertise choices.
//
// Rogues and bards are the only PHB classes that do; each grants two at a
// time, which is why the count is doubled rather than incremented.
var expertiseGrants = map[Class][]int{
	ClassRogue: {1, 6},
	ClassBard:  {3, 10},
}

// ExpertiseBudget returns how many expertise choices the character has earned.
//
// Expertise may be spent on tools as well as skills -- a rogue commonly takes
// thieves' tools -- so this is a ceiling on skill expertise, not a target.
func (c *Character) ExpertiseBudget() int {
	budget := 0
	for _, cl := range c.BasicInfo.Classes {
		for _, level := range expertiseGrants[cl.Class] {
			if cl.Level >= level {
				budget += 2
			}
		}
	}
	return budget
}

// SkillBudget returns how many skill proficiencies the character should have.
//
// It is the sum of skills granted outright by race and background, the
// choices the first class offers, the single extra skill some classes grant
// when multiclassed into, and any free racial picks (a half-elf's two).
func (c *Character) SkillBudget() int {
	budget := len(c.GrantedSkills())

	for i, cl := range c.BasicInfo.Classes {
		def, ok := cl.Class.Definition()
		if !ok {
			continue
		}
		if i == 0 {
			budget += def.SkillChoices
		} else {
			budget += def.MulticlassSkillChoices
		}
	}

	budget += c.BasicInfo.Race.SkillChoiceCount(c.BasicInfo.Subrace)
	return budget
}

// ClassFeatures maps each class to the features gained at each level.
//
// These are names for display and level-up planning, not rules the engine
// resolves -- resolution reads the class and subclass tables, never this. The
// list is a PHB reference and worth spot-checking before it is shown to
// players as authoritative.
var ClassFeatures = map[Class]map[int][]string{
	ClassBarbarian: {
		1: {"Rage", "Unarmored Defense"}, 2: {"Reckless Attack", "Danger Sense"},
		3: {"Primal Path"}, 4: {"Ability Score Improvement"},
		5: {"Extra Attack", "Fast Movement"}, 7: {"Feral Instinct"},
		9: {"Brutal Critical (1 die)"}, 11: {"Relentless Rage"},
		13: {"Brutal Critical (2 dice)"}, 15: {"Persistent Rage"},
		17: {"Brutal Critical (3 dice)"}, 18: {"Indomitable Might"},
		20: {"Primal Champion"},
	},
	ClassBard: {
		1: {"Spellcasting", "Bardic Inspiration (d6)"},
		2: {"Jack of All Trades", "Song of Rest (d6)"},
		3: {"Bard College", "Expertise"}, 4: {"Ability Score Improvement"},
		5: {"Bardic Inspiration (d8)", "Font of Inspiration"}, 6: {"Countercharm"},
		9:  {"Song of Rest (d8)"},
		10: {"Bardic Inspiration (d10)", "Expertise", "Magical Secrets"},
		13: {"Song of Rest (d10)"}, 14: {"Magical Secrets"},
		15: {"Bardic Inspiration (d12)"}, 17: {"Song of Rest (d12)"},
		18: {"Magical Secrets"}, 20: {"Superior Inspiration"},
	},
	ClassCleric: {
		1: {"Spellcasting", "Divine Domain"}, 2: {"Channel Divinity (1/rest)"},
		4: {"Ability Score Improvement"}, 5: {"Destroy Undead (CR 1/2)"},
		6: {"Channel Divinity (2/rest)"}, 8: {"Destroy Undead (CR 1)"},
		10: {"Divine Intervention"}, 11: {"Destroy Undead (CR 2)"},
		14: {"Destroy Undead (CR 3)"}, 17: {"Destroy Undead (CR 4)"},
		18: {"Channel Divinity (3/rest)"}, 20: {"Divine Intervention improvement"},
	},
	ClassDruid: {
		1: {"Druidic", "Spellcasting"}, 2: {"Wild Shape", "Druid Circle"},
		4: {"Ability Score Improvement", "Wild Shape improvement"},
		8: {"Wild Shape improvement"}, 18: {"Timeless Body", "Beast Spells"},
		20: {"Archdruid"},
	},
	ClassFighter: {
		1: {"Fighting Style", "Second Wind"}, 2: {"Action Surge (one use)"},
		3: {"Martial Archetype"}, 4: {"Ability Score Improvement"},
		5: {"Extra Attack"}, 9: {"Indomitable (one use)"},
		11: {"Extra Attack (2)"}, 13: {"Indomitable (two uses)"},
		17: {"Action Surge (two uses)", "Indomitable (three uses)"},
		20: {"Extra Attack (3)"},
	},
	ClassMonk: {
		1: {"Unarmored Defense", "Martial Arts"}, 2: {"Ki", "Unarmored Movement"},
		3: {"Monastic Tradition", "Deflect Missiles"},
		4: {"Ability Score Improvement", "Slow Fall"},
		5: {"Extra Attack", "Stunning Strike"}, 6: {"Ki-Empowered Strikes"},
		7: {"Evasion", "Stillness of Mind"}, 10: {"Purity of Body"},
		13: {"Tongue of the Sun and Moon"}, 14: {"Diamond Soul"},
		15: {"Timeless Body"}, 18: {"Empty Body"}, 20: {"Perfect Self"},
	},
	ClassPaladin: {
		1: {"Divine Sense", "Lay on Hands"},
		2: {"Fighting Style", "Spellcasting", "Divine Smite"},
		3: {"Divine Health", "Sacred Oath"}, 4: {"Ability Score Improvement"},
		5: {"Extra Attack"}, 6: {"Aura of Protection"}, 10: {"Aura of Courage"},
		11: {"Improved Divine Smite"}, 14: {"Cleansing Touch"},
		18: {"Aura improvements"},
	},
	ClassRanger: {
		1: {"Favored Enemy", "Natural Explorer"}, 2: {"Fighting Style", "Spellcasting"},
		3: {"Ranger Archetype", "Primeval Awareness"}, 4: {"Ability Score Improvement"},
		5: {"Extra Attack"}, 8: {"Land's Stride"}, 10: {"Hide in Plain Sight"},
		14: {"Vanish"}, 18: {"Feral Senses"}, 20: {"Foe Slayer"},
	},
	ClassRogue: {
		1: {"Expertise", "Sneak Attack", "Thieves' Cant"}, 2: {"Cunning Action"},
		3: {"Roguish Archetype"}, 4: {"Ability Score Improvement"},
		5: {"Uncanny Dodge"}, 6: {"Expertise"}, 7: {"Evasion"},
		11: {"Reliable Talent"}, 14: {"Blindsense"}, 15: {"Slippery Mind"},
		18: {"Elusive"}, 20: {"Stroke of Luck"},
	},
	ClassSorcerer: {
		1: {"Spellcasting", "Sorcerous Origin"}, 2: {"Font of Magic"},
		3: {"Metamagic"}, 4: {"Ability Score Improvement"},
		10: {"Metamagic"}, 17: {"Metamagic"}, 20: {"Sorcerous Restoration"},
	},
	ClassWarlock: {
		1: {"Otherworldly Patron", "Pact Magic"}, 2: {"Eldritch Invocations"},
		3: {"Pact Boon"}, 4: {"Ability Score Improvement"},
		11: {"Mystic Arcanum (6th level)"}, 13: {"Mystic Arcanum (7th level)"},
		15: {"Mystic Arcanum (8th level)"}, 17: {"Mystic Arcanum (9th level)"},
		20: {"Eldritch Master"},
	},
	ClassWizard: {
		1: {"Spellcasting", "Arcane Recovery"}, 2: {"Arcane Tradition"},
		4: {"Ability Score Improvement"}, 18: {"Spell Mastery"},
		20: {"Signature Spells"},
	},
}

// FeaturesAtLevel returns the features a class grants at one level, including
// the Ability Score Improvement and the archetype choice.
func FeaturesAtLevel(c Class, level int) []string {
	var out []string
	out = append(out, ClassFeatures[c][level]...)

	// ASI levels vary by class, so they are merged in rather than repeated in
	// every row of the table above.
	for _, asi := range AbilityScoreImprovementLevels(c) {
		if asi == level && !contains(out, "Ability Score Improvement") {
			out = append(out, "Ability Score Improvement")
		}
	}
	return out
}

// FeaturesThroughLevel returns every feature a class grants up to and
// including a level, which is what a character sheet's Features box lists.
func FeaturesThroughLevel(c Class, level int) []string {
	var out []string
	for l := 1; l <= level && l <= 20; l++ {
		out = append(out, FeaturesAtLevel(c, l)...)
	}
	return out
}

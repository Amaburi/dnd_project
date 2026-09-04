package models

import (
	"strconv"
	"strings"
)

// Class is one of the twelve PHB classes.
type Class string

const (
	ClassBarbarian Class = "barbarian"
	ClassBard      Class = "bard"
	ClassCleric    Class = "cleric"
	ClassDruid     Class = "druid"
	ClassFighter   Class = "fighter"
	ClassMonk      Class = "monk"
	ClassPaladin   Class = "paladin"
	ClassRanger    Class = "ranger"
	ClassRogue     Class = "rogue"
	ClassSorcerer  Class = "sorcerer"
	ClassWarlock   Class = "warlock"
	ClassWizard    Class = "wizard"
)

// CasterProgression is how quickly a class gains spell slots. It decides both
// the single-class slot table and what the class contributes to a multiclass
// caster level.
type CasterProgression string

const (
	CasterNone  CasterProgression = "none"
	CasterFull  CasterProgression = "full"  // bard, cleric, druid, sorcerer, wizard
	CasterHalf  CasterProgression = "half"  // paladin, ranger
	CasterThird CasterProgression = "third" // eldritch knight, arcane trickster
	CasterPact  CasterProgression = "pact"  // warlock
)

// AbilityMinimum is a single ability score requirement.
type AbilityMinimum struct {
	Ability Ability
	Score   int
}

// ClassDefinition is everything a class determines mechanically.
//
// Class used to be a free string, so the hit die, saving throws, permitted
// skills and spellcasting ability all had to be typed in by hand and could
// silently disagree with the class named beside them.
type ClassDefinition struct {
	Class        Class
	HitDie       int
	Primary      []Ability
	SavingThrows []Ability // the two the class is proficient in

	SkillChoices int
	SkillList    []Skill // empty means "any skill" (bard)

	SpellcastingAbility Ability // "" for non-casters
	Progression         CasterProgression

	// SubclassLevel is when the archetype is chosen. Most classes pick at 3,
	// but clerics, sorcerers and warlocks decide at 1 and druids and wizards
	// at 2.
	SubclassLevel int
	Subclasses    []SubclassDefinition

	ArmorProficiencies  []string
	WeaponProficiencies []string

	// MulticlassSkillChoices is the extra skill granted for multiclassing
	// into this class. Only bard, ranger and rogue give one.
	MulticlassSkillChoices int

	// MulticlassProficiencies is the reduced set granted when this class is
	// taken as a later class. Multiclassing never grants everything a first
	// level in the class would: a fighter taken second brings medium armour
	// but not heavy.
	MulticlassProficiencies Proficiencies

	// MulticlassPrerequisites is satisfied when *any* inner group is fully
	// met, which is how the PHB expresses fighter's "Strength 13 or
	// Dexterity 13" alongside monk's "Dexterity 13 and Wisdom 13".
	MulticlassPrerequisites [][]AbilityMinimum
}

// SubclassCasting describes spellcasting granted by an archetype.
type SubclassCasting struct {
	Ability     Ability
	Progression CasterProgression
}

// SourcePHB marks content from the Player's Handbook. The field exists so
// material from other books can be added later without guessing where an
// entry came from.
const SourcePHB = "PHB"

// SubclassDefinition is an archetype and whatever mechanics it carries.
//
// Archetypes were a list of names, which was enough to validate a sheet but
// not to play one: a Champion crits on 19 and an Eldritch Knight casts spells,
// and neither is expressible as a string.
type SubclassDefinition struct {
	Key    string
	Name   string
	Source string

	// Casting is set for archetypes that grant spellcasting to a class that
	// otherwise has none.
	Casting *SubclassCasting

	// CritRangeAt maps a class level to the lowest natural d20 roll that
	// scores a critical hit from then on. Only the Champion improves it.
	CritRangeAt map[int]int
}

// phbSub builds a plain PHB archetype with no extra mechanics.
func phbSub(key, name string) SubclassDefinition {
	return SubclassDefinition{Key: key, Name: name, Source: SourcePHB}
}

// anyOf builds a prerequisite list of alternatives.
func anyOf(groups ...[]AbilityMinimum) [][]AbilityMinimum { return groups }

// allOf builds a single prerequisite group whose entries must all be met.
func allOf(mins ...AbilityMinimum) []AbilityMinimum { return mins }

func min13(a Ability) AbilityMinimum { return AbilityMinimum{Ability: a, Score: 13} }

// ClassTable is the single source of truth for class mechanics.
var ClassTable = map[Class]ClassDefinition{
	ClassBarbarian: {
		Class: ClassBarbarian, HitDie: 12,
		Primary:      []Ability{AbilityStrength},
		SavingThrows: []Ability{AbilityStrength, AbilityConstitution},
		SkillChoices: 2,
		SkillList: []Skill{SkillAnimalHandling, SkillAthletics, SkillIntimidation,
			SkillNature, SkillPerception, SkillSurvival},
		Progression:             CasterNone,
		SubclassLevel:           3,
		Subclasses:              []SubclassDefinition{phbSub("berserker", "Berserker"), phbSub("totem_warrior", "Totem Warrior")},
		ArmorProficiencies:      []string{"light", "medium", "shields"},
		WeaponProficiencies:     []string{"simple", "martial"},
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfShields}, Weapons: []string{ProfSimpleWeapons, ProfMartialWeapons}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityStrength))),
	},
	ClassBard: {
		Class: ClassBard, HitDie: 8,
		Primary:                 []Ability{AbilityCharisma},
		SavingThrows:            []Ability{AbilityDexterity, AbilityCharisma},
		SkillChoices:            3,
		SkillList:               nil, // any three skills
		SpellcastingAbility:     AbilityCharisma,
		Progression:             CasterFull,
		SubclassLevel:           3,
		Subclasses:              []SubclassDefinition{phbSub("lore", "College of Lore"), phbSub("valor", "College of Valor")},
		ArmorProficiencies:      []string{"light"},
		WeaponProficiencies:     []string{"simple", "hand_crossbow", "longsword", "rapier", "shortsword"},
		MulticlassSkillChoices:  1,
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor}, Tools: []string{"musical_instrument"}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityCharisma))),
	},
	ClassCleric: {
		Class: ClassCleric, HitDie: 8,
		Primary:             []Ability{AbilityWisdom},
		SavingThrows:        []Ability{AbilityWisdom, AbilityCharisma},
		SkillChoices:        2,
		SkillList:           []Skill{SkillHistory, SkillInsight, SkillMedicine, SkillPersuasion, SkillReligion},
		SpellcastingAbility: AbilityWisdom,
		Progression:         CasterFull,
		SubclassLevel:       1,
		Subclasses: []SubclassDefinition{
			phbSub("knowledge", "Knowledge Domain"), phbSub("life", "Life Domain"),
			phbSub("light", "Light Domain"), phbSub("nature", "Nature Domain"),
			phbSub("tempest", "Tempest Domain"), phbSub("trickery", "Trickery Domain"),
			phbSub("war", "War Domain"),
		},
		ArmorProficiencies:      []string{"light", "medium", "shields"},
		WeaponProficiencies:     []string{"simple"},
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor, ProfMediumArmor, ProfShields}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityWisdom))),
	},
	ClassDruid: {
		Class: ClassDruid, HitDie: 8,
		Primary:      []Ability{AbilityWisdom},
		SavingThrows: []Ability{AbilityIntelligence, AbilityWisdom},
		SkillChoices: 2,
		SkillList: []Skill{SkillArcana, SkillAnimalHandling, SkillInsight, SkillMedicine,
			SkillNature, SkillPerception, SkillReligion, SkillSurvival},
		SpellcastingAbility: AbilityWisdom,
		Progression:         CasterFull,
		SubclassLevel:       2,
		Subclasses:          []SubclassDefinition{phbSub("land", "Circle of the Land"), phbSub("moon", "Circle of the Moon")},
		ArmorProficiencies:  []string{"light", "medium", "shields"},
		WeaponProficiencies: []string{"club", "dagger", "dart", "javelin", "mace",
			"quarterstaff", "scimitar", "sickle", "sling", "spear"},
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor, ProfMediumArmor, ProfShields}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityWisdom))),
	},
	ClassFighter: {
		Class: ClassFighter, HitDie: 10,
		Primary:      []Ability{AbilityStrength, AbilityDexterity},
		SavingThrows: []Ability{AbilityStrength, AbilityConstitution},
		SkillChoices: 2,
		SkillList: []Skill{SkillAcrobatics, SkillAnimalHandling, SkillAthletics,
			SkillHistory, SkillInsight, SkillIntimidation, SkillPerception, SkillSurvival},
		Progression:   CasterNone,
		SubclassLevel: 3,
		Subclasses: []SubclassDefinition{
			{
				Key: "champion", Name: "Champion", Source: SourcePHB,
				// Improved Critical at 3, Superior Critical at 15.
				CritRangeAt: map[int]int{3: 19, 15: 18},
			},
			phbSub("battle_master", "Battle Master"),
			{
				Key: "eldritch_knight", Name: "Eldritch Knight", Source: SourcePHB,
				Casting: &SubclassCasting{Ability: AbilityIntelligence, Progression: CasterThird},
			},
		},

		ArmorProficiencies:  []string{"light", "medium", "heavy", "shields"},
		WeaponProficiencies: []string{"simple", "martial"},
		// "Strength 13 or Dexterity 13" -- two alternatives, not both.
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor, ProfMediumArmor, ProfShields}, Weapons: []string{ProfSimpleWeapons, ProfMartialWeapons}},
		MulticlassPrerequisites: anyOf(
			allOf(min13(AbilityStrength)),
			allOf(min13(AbilityDexterity)),
		),
	},
	ClassMonk: {
		Class: ClassMonk, HitDie: 8,
		Primary:      []Ability{AbilityDexterity, AbilityWisdom},
		SavingThrows: []Ability{AbilityStrength, AbilityDexterity},
		SkillChoices: 2,
		SkillList: []Skill{SkillAcrobatics, SkillAthletics, SkillHistory,
			SkillInsight, SkillReligion, SkillStealth},
		Progression:             CasterNone,
		SubclassLevel:           3,
		Subclasses:              []SubclassDefinition{phbSub("open_hand", "Way of the Open Hand"), phbSub("shadow", "Way of Shadow"), phbSub("four_elements", "Way of the Four Elements")},
		ArmorProficiencies:      nil,
		WeaponProficiencies:     []string{"simple", "shortsword"},
		MulticlassProficiencies: Proficiencies{Weapons: []string{ProfSimpleWeapons, "shortsword"}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityDexterity), min13(AbilityWisdom))),
	},
	ClassPaladin: {
		Class: ClassPaladin, HitDie: 10,
		Primary:      []Ability{AbilityStrength, AbilityCharisma},
		SavingThrows: []Ability{AbilityWisdom, AbilityCharisma},
		SkillChoices: 2,
		SkillList: []Skill{SkillAthletics, SkillInsight, SkillIntimidation,
			SkillMedicine, SkillPersuasion, SkillReligion},
		SpellcastingAbility:     AbilityCharisma,
		Progression:             CasterHalf,
		SubclassLevel:           3,
		Subclasses:              []SubclassDefinition{phbSub("devotion", "Oath of Devotion"), phbSub("ancients", "Oath of the Ancients"), phbSub("vengeance", "Oath of Vengeance")},
		ArmorProficiencies:      []string{"light", "medium", "heavy", "shields"},
		WeaponProficiencies:     []string{"simple", "martial"},
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor, ProfMediumArmor, ProfShields}, Weapons: []string{ProfSimpleWeapons, ProfMartialWeapons}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityStrength), min13(AbilityCharisma))),
	},
	ClassRanger: {
		Class: ClassRanger, HitDie: 10,
		Primary:      []Ability{AbilityDexterity, AbilityWisdom},
		SavingThrows: []Ability{AbilityStrength, AbilityDexterity},
		SkillChoices: 3,
		SkillList: []Skill{SkillAnimalHandling, SkillAthletics, SkillInsight,
			SkillInvestigation, SkillNature, SkillPerception, SkillStealth, SkillSurvival},
		SpellcastingAbility:     AbilityWisdom,
		Progression:             CasterHalf,
		SubclassLevel:           3,
		Subclasses:              []SubclassDefinition{phbSub("hunter", "Hunter"), phbSub("beast_master", "Beast Master")},
		ArmorProficiencies:      []string{"light", "medium", "shields"},
		WeaponProficiencies:     []string{"simple", "martial"},
		MulticlassSkillChoices:  1,
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor, ProfMediumArmor, ProfShields}, Weapons: []string{ProfSimpleWeapons, ProfMartialWeapons}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityDexterity), min13(AbilityWisdom))),
	},
	ClassRogue: {
		Class: ClassRogue, HitDie: 8,
		Primary:      []Ability{AbilityDexterity},
		SavingThrows: []Ability{AbilityDexterity, AbilityIntelligence},
		SkillChoices: 4,
		SkillList: []Skill{SkillAcrobatics, SkillAthletics, SkillDeception, SkillInsight,
			SkillIntimidation, SkillInvestigation, SkillPerception, SkillPerformance,
			SkillPersuasion, SkillSleightOfHand, SkillStealth},
		Progression:   CasterNone,
		SubclassLevel: 3,
		Subclasses: []SubclassDefinition{
			phbSub("thief", "Thief"), phbSub("assassin", "Assassin"),
			{
				Key: "arcane_trickster", Name: "Arcane Trickster", Source: SourcePHB,
				Casting: &SubclassCasting{Ability: AbilityIntelligence, Progression: CasterThird},
			},
		},

		ArmorProficiencies:      []string{"light"},
		WeaponProficiencies:     []string{"simple", "hand_crossbow", "longsword", "rapier", "shortsword"},
		MulticlassSkillChoices:  1,
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor}, Tools: []string{"thieves_tools"}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityDexterity))),
	},
	ClassSorcerer: {
		Class: ClassSorcerer, HitDie: 6,
		Primary:      []Ability{AbilityCharisma},
		SavingThrows: []Ability{AbilityConstitution, AbilityCharisma},
		SkillChoices: 2,
		SkillList: []Skill{SkillArcana, SkillDeception, SkillInsight,
			SkillIntimidation, SkillPersuasion, SkillReligion},
		SpellcastingAbility:     AbilityCharisma,
		Progression:             CasterFull,
		SubclassLevel:           1,
		Subclasses:              []SubclassDefinition{phbSub("draconic_bloodline", "Draconic Bloodline"), phbSub("wild_magic", "Wild Magic")},
		ArmorProficiencies:      nil,
		WeaponProficiencies:     []string{"dagger", "dart", "sling", "quarterstaff", "light_crossbow"},
		MulticlassProficiencies: Proficiencies{},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityCharisma))),
	},
	ClassWarlock: {
		Class: ClassWarlock, HitDie: 8,
		Primary:      []Ability{AbilityCharisma},
		SavingThrows: []Ability{AbilityWisdom, AbilityCharisma},
		SkillChoices: 2,
		SkillList: []Skill{SkillArcana, SkillDeception, SkillHistory, SkillIntimidation,
			SkillInvestigation, SkillNature, SkillReligion},
		SpellcastingAbility:     AbilityCharisma,
		Progression:             CasterPact,
		SubclassLevel:           1,
		Subclasses:              []SubclassDefinition{phbSub("archfey", "The Archfey"), phbSub("fiend", "The Fiend"), phbSub("great_old_one", "The Great Old One")},
		ArmorProficiencies:      []string{"light"},
		WeaponProficiencies:     []string{"simple"},
		MulticlassProficiencies: Proficiencies{Armor: []string{ProfLightArmor}, Weapons: []string{ProfSimpleWeapons}},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityCharisma))),
	},
	ClassWizard: {
		Class: ClassWizard, HitDie: 6,
		Primary:      []Ability{AbilityIntelligence},
		SavingThrows: []Ability{AbilityIntelligence, AbilityWisdom},
		SkillChoices: 2,
		SkillList: []Skill{SkillArcana, SkillHistory, SkillInsight,
			SkillInvestigation, SkillMedicine, SkillReligion},
		SpellcastingAbility: AbilityIntelligence,
		Progression:         CasterFull,
		SubclassLevel:       2,
		Subclasses: []SubclassDefinition{
			phbSub("abjuration", "School of Abjuration"), phbSub("conjuration", "School of Conjuration"),
			phbSub("divination", "School of Divination"), phbSub("enchantment", "School of Enchantment"),
			phbSub("evocation", "School of Evocation"), phbSub("illusion", "School of Illusion"),
			phbSub("necromancy", "School of Necromancy"), phbSub("transmutation", "School of Transmutation"),
		},
		ArmorProficiencies:      nil,
		WeaponProficiencies:     []string{"dagger", "dart", "sling", "quarterstaff", "light_crossbow"},
		MulticlassProficiencies: Proficiencies{},
		MulticlassPrerequisites: anyOf(allOf(min13(AbilityIntelligence))),
	},
}

// Classes lists every class in alphabetical order.
var Classes = []Class{
	ClassBarbarian, ClassBard, ClassCleric, ClassDruid, ClassFighter, ClassMonk,
	ClassPaladin, ClassRanger, ClassRogue, ClassSorcerer, ClassWarlock, ClassWizard,
}

// Valid reports whether c is a known class.
func (c Class) Valid() bool {
	_, ok := ClassTable[c]
	return ok
}

// Definition returns the class's mechanics, and whether the class is known.
func (c Class) Definition() (ClassDefinition, bool) {
	def, ok := ClassTable[c]
	return def, ok
}

// HitDie returns the class's hit die, or 0 for an unknown class.
func (c Class) HitDie() int {
	return ClassTable[c].HitDie
}

// HasSubclass reports whether name is a valid archetype for this class.
func (c Class) HasSubclass(name string) bool {
	_, ok := c.Subclass(name)
	return ok
}

// Subclass looks an archetype up by key.
func (c Class) Subclass(key string) (SubclassDefinition, bool) {
	def, ok := ClassTable[c]
	if !ok {
		return SubclassDefinition{}, false
	}
	for _, sub := range def.Subclasses {
		if sub.Key == key {
			return sub, true
		}
	}
	return SubclassDefinition{}, false
}

// SubclassKeys lists the archetype keys for a class, in book order.
func (c Class) SubclassKeys() []string {
	def, ok := ClassTable[c]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(def.Subclasses))
	for _, sub := range def.Subclasses {
		keys = append(keys, sub.Key)
	}
	return keys
}

// ClassLevel is one class a character has levels in.
//
// A character carries a slice of these rather than a single class and level,
// so Fighter 3 / Wizard 2 is representable. Proficiency bonus comes from the
// total, but spell slots come from a weighted caster level, and hit dice stay
// in separate pools per class -- three different notions of "level" that a
// single integer conflated.
type ClassLevel struct {
	Class    Class  `json:"class" bson:"class"`
	Subclass string `json:"subclass,omitempty" bson:"subclass,omitempty"`
	Level    int    `json:"level" bson:"level"`
}

// Casting returns the spellcasting ability and progression for this class and
// archetype, accounting for subclasses like the Eldritch Knight that grant
// casting to a class that otherwise has none.
func (cl ClassLevel) Casting() (Ability, CasterProgression) {
	def, ok := ClassTable[cl.Class]
	if !ok {
		return "", CasterNone
	}
	if def.Progression != CasterNone {
		return def.SpellcastingAbility, def.Progression
	}
	if sub, ok := cl.Class.Subclass(cl.Subclass); ok && sub.Casting != nil {
		// Subclass casting only begins once the archetype is chosen.
		if cl.Level >= def.SubclassLevel {
			return sub.Casting.Ability, sub.Casting.Progression
		}
	}
	return "", CasterNone
}

// CritRange is the lowest natural d20 roll that scores a critical hit for this
// class level, which is 20 unless an archetype improves it.
func (cl ClassLevel) CritRange() int {
	sub, ok := cl.Class.Subclass(cl.Subclass)
	if !ok || sub.CritRangeAt == nil {
		return NaturalCrit
	}

	best := NaturalCrit
	for level, rng := range sub.CritRangeAt {
		if cl.Level >= level && rng < best {
			best = rng
		}
	}
	return best
}

// MeetsMulticlassPrerequisites reports whether the given ability scores allow
// taking a level in this class as a second or later class.
//
// The requirement is satisfied when any one alternative group is fully met,
// which distinguishes fighter's "Strength 13 or Dexterity 13" from monk's
// "Dexterity 13 and Wisdom 13".
func (c Class) MeetsMulticlassPrerequisites(scores AbilityScores) bool {
	def, ok := ClassTable[c]
	if !ok {
		return false
	}
	if len(def.MulticlassPrerequisites) == 0 {
		return true
	}
	for _, group := range def.MulticlassPrerequisites {
		satisfied := true
		for _, req := range group {
			if scores.Score(req.Ability) < req.Score {
				satisfied = false
				break
			}
		}
		if satisfied {
			return true
		}
	}
	return false
}

// DescribePrerequisites renders a class's multiclass requirement for an error
// message, e.g. "Strength 13 or Dexterity 13".
func (c Class) DescribePrerequisites() string {
	def, ok := ClassTable[c]
	if !ok || len(def.MulticlassPrerequisites) == 0 {
		return "none"
	}

	alternatives := make([]string, 0, len(def.MulticlassPrerequisites))
	for _, group := range def.MulticlassPrerequisites {
		parts := make([]string, 0, len(group))
		for _, req := range group {
			parts = append(parts, titleAbility(req.Ability)+" "+strconv.Itoa(req.Score))
		}
		alternatives = append(alternatives, strings.Join(parts, " and "))
	}
	return strings.Join(alternatives, " or ")
}

func titleAbility(a Ability) string {
	switch a {
	case AbilityStrength:
		return "Strength"
	case AbilityDexterity:
		return "Dexterity"
	case AbilityConstitution:
		return "Constitution"
	case AbilityIntelligence:
		return "Intelligence"
	case AbilityWisdom:
		return "Wisdom"
	case AbilityCharisma:
		return "Charisma"
	}
	return string(a)
}

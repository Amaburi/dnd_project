package models

// EquipmentOption is one bundle a player may take at character creation.
type EquipmentOption struct {
	// Label is how the option reads in the book, e.g. "a martial weapon and
	// a shield".
	Label string `json:"label" bson:"label"`

	// Items are the item keys the bundle contains, for wiring up to a real
	// item catalogue later. Some entries are categories ("martial_weapon")
	// rather than specific items, because the book leaves the choice open.
	Items []string `json:"items" bson:"items"`
}

// EquipmentChoice is one "choose one of" decision at creation.
type EquipmentChoice struct {
	Prompt  string            `json:"prompt" bson:"prompt"`
	Options []EquipmentOption `json:"options" bson:"options"`
}

// StartingEquipment is what a class begins play with.
//
// This is creation-time guidance, not a rules input: nothing resolves against
// it, and a character's real gear is whatever ends up in their Inventory. It
// exists so a creation flow does not have to hardcode the book.
type StartingEquipment struct {
	Choices []EquipmentChoice `json:"choices" bson:"choices"`
	Fixed   []string          `json:"fixed" bson:"fixed"`
}

func choice(prompt string, options ...EquipmentOption) EquipmentChoice {
	return EquipmentChoice{Prompt: prompt, Options: options}
}

func option(label string, items ...string) EquipmentOption {
	return EquipmentOption{Label: label, Items: items}
}

// StartingEquipmentTable lists what each class begins with.
var StartingEquipmentTable = map[Class]StartingEquipment{
	ClassBarbarian: {
		Choices: []EquipmentChoice{
			choice("primary weapon", option("a greataxe", "greataxe"),
				option("any martial melee weapon", "martial_melee_weapon")),
			choice("secondary weapon", option("two handaxes", "handaxe", "handaxe"),
				option("any simple weapon", "simple_weapon")),
		},
		Fixed: []string{"explorers_pack", "javelin", "javelin", "javelin", "javelin"},
	},
	ClassBard: {
		Choices: []EquipmentChoice{
			choice("weapon", option("a rapier", "rapier"), option("a longsword", "longsword"),
				option("any simple weapon", "simple_weapon")),
			choice("pack", option("a diplomat's pack", "diplomats_pack"),
				option("an entertainer's pack", "entertainers_pack")),
			choice("instrument", option("a lute", "lute"),
				option("any other musical instrument", "musical_instrument")),
		},
		Fixed: []string{"leather_armor", "dagger"},
	},
	ClassCleric: {
		Choices: []EquipmentChoice{
			choice("weapon", option("a mace", "mace"), option("a warhammer (if proficient)", "warhammer")),
			choice("armor", option("scale mail", "scale_mail"), option("leather armor", "leather_armor"),
				option("chain mail (if proficient)", "chain_mail")),
			choice("ranged option", option("a light crossbow and 20 bolts", "light_crossbow", "crossbow_bolts_20"),
				option("any simple weapon", "simple_weapon")),
			choice("pack", option("a priest's pack", "priests_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
		Fixed: []string{"shield", "holy_symbol"},
	},
	ClassDruid: {
		Choices: []EquipmentChoice{
			choice("shield or weapon", option("a wooden shield", "wooden_shield"),
				option("any simple weapon", "simple_weapon")),
			choice("melee weapon", option("a scimitar", "scimitar"),
				option("any simple melee weapon", "simple_melee_weapon")),
		},
		Fixed: []string{"leather_armor", "explorers_pack", "druidic_focus"},
	},
	ClassFighter: {
		Choices: []EquipmentChoice{
			choice("armor", option("chain mail", "chain_mail"),
				option("leather armor, longbow and 20 arrows", "leather_armor", "longbow", "arrows_20")),
			choice("weapons", option("a martial weapon and a shield", "martial_weapon", "shield"),
				option("two martial weapons", "martial_weapon", "martial_weapon")),
			choice("ranged option", option("a light crossbow and 20 bolts", "light_crossbow", "crossbow_bolts_20"),
				option("two handaxes", "handaxe", "handaxe")),
			choice("pack", option("a dungeoneer's pack", "dungeoneers_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
	},
	ClassMonk: {
		Choices: []EquipmentChoice{
			choice("weapon", option("a shortsword", "shortsword"),
				option("any simple weapon", "simple_weapon")),
			choice("pack", option("a dungeoneer's pack", "dungeoneers_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
		Fixed: []string{"dart", "dart", "dart", "dart", "dart", "dart", "dart", "dart", "dart", "dart"},
	},
	ClassPaladin: {
		Choices: []EquipmentChoice{
			choice("weapons", option("a martial weapon and a shield", "martial_weapon", "shield"),
				option("two martial weapons", "martial_weapon", "martial_weapon")),
			choice("secondary weapon", option("five javelins", "javelin", "javelin", "javelin", "javelin", "javelin"),
				option("any simple melee weapon", "simple_melee_weapon")),
			choice("pack", option("a priest's pack", "priests_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
		Fixed: []string{"chain_mail", "holy_symbol"},
	},
	ClassRanger: {
		Choices: []EquipmentChoice{
			choice("armor", option("scale mail", "scale_mail"), option("leather armor", "leather_armor")),
			choice("melee weapons", option("two shortswords", "shortsword", "shortsword"),
				option("two simple melee weapons", "simple_melee_weapon", "simple_melee_weapon")),
			choice("pack", option("a dungeoneer's pack", "dungeoneers_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
		Fixed: []string{"longbow", "arrows_20"},
	},
	ClassRogue: {
		Choices: []EquipmentChoice{
			choice("primary weapon", option("a rapier", "rapier"), option("a shortsword", "shortsword")),
			choice("secondary weapon", option("a shortbow and 20 arrows", "shortbow", "arrows_20"),
				option("a shortsword", "shortsword")),
			choice("pack", option("a burglar's pack", "burglars_pack"),
				option("a dungeoneer's pack", "dungeoneers_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
		Fixed: []string{"leather_armor", "dagger", "dagger", "thieves_tools"},
	},
	ClassSorcerer: {
		Choices: []EquipmentChoice{
			choice("ranged option", option("a light crossbow and 20 bolts", "light_crossbow", "crossbow_bolts_20"),
				option("any simple weapon", "simple_weapon")),
			choice("focus", option("a component pouch", "component_pouch"),
				option("an arcane focus", "arcane_focus")),
			choice("pack", option("a dungeoneer's pack", "dungeoneers_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
		Fixed: []string{"dagger", "dagger"},
	},
	ClassWarlock: {
		Choices: []EquipmentChoice{
			choice("ranged option", option("a light crossbow and 20 bolts", "light_crossbow", "crossbow_bolts_20"),
				option("any simple weapon", "simple_weapon")),
			choice("focus", option("a component pouch", "component_pouch"),
				option("an arcane focus", "arcane_focus")),
			choice("pack", option("a scholar's pack", "scholars_pack"),
				option("a dungeoneer's pack", "dungeoneers_pack")),
		},
		Fixed: []string{"leather_armor", "simple_weapon", "dagger", "dagger"},
	},
	ClassWizard: {
		Choices: []EquipmentChoice{
			choice("weapon", option("a quarterstaff", "quarterstaff"), option("a dagger", "dagger")),
			choice("focus", option("a component pouch", "component_pouch"),
				option("an arcane focus", "arcane_focus")),
			choice("pack", option("a scholar's pack", "scholars_pack"),
				option("an explorer's pack", "explorers_pack")),
		},
		Fixed: []string{"spellbook"},
	},
}

// StartingEquipmentFor returns what a class begins with, and whether the class
// is known.
func StartingEquipmentFor(c Class) (StartingEquipment, bool) {
	eq, ok := StartingEquipmentTable[c]
	return eq, ok
}

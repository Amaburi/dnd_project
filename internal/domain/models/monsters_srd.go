package models

// SourceSRD marks statblocks taken from the System Reference Document.
const SourceSRD = "SRD"

func bonus(n int) *int { return &n }

// attack builds a melee or ranged attack action.
func attack(name string, toHit int, dice string, dt DamageType, reach int) MonsterAction {
	return MonsterAction{
		Name: name, AttackBonus: bonus(toHit),
		DamageDice: dice, DamageType: dt, ReachFeet: reach,
	}
}

// ranged builds a ranged attack action.
func ranged(name string, toHit int, dice string, dt DamageType, normal, long int) MonsterAction {
	return MonsterAction{
		Name: name, AttackBonus: bonus(toHit),
		DamageDice: dice, DamageType: dt,
		RangeNormal: normal, RangeLong: long,
	}
}

func trait(name, description string) MonsterFeature {
	return MonsterFeature{Name: name, Description: description}
}

// SRDMonsters is a small catalogue of statblocks spanning CR 1/8 to 5.
//
// Monsters are stored rather than enumerated, so this is seed data, not a
// source of truth the way ClassTable is: campaigns own their own monsters and
// may edit or replace any of these. It exists so the collection is not empty
// and so combat has something real to resolve against.
//
// Every entry is validated by the test suite, including that its printed hit
// points match what its hit dice average to.
func SRDMonsters() []Monster {
	return []Monster{
		{
			MonsterID: "srd_kobold", Name: "Kobold", Source: SourceSRD,
			Size: SizeSmall, Type: "humanoid", Subtype: "kobold", Alignment: "lawful evil",
			ArmorClass: 12, HitPoints: HitPoints{Current: 5, Maximum: 5}, HitDice: "2d6-2",
			Speeds:          Speeds{Walk: 30},
			AbilityScores:   AbilityScores{Strength: 7, Dexterity: 15, Constitution: 9, Intelligence: 8, Wisdom: 7, Charisma: 8},
			Senses:          Senses{Darkvision: 60},
			Languages:       []string{"common", "draconic"},
			ChallengeRating: 0.125,
			Traits: []MonsterFeature{
				trait("Sunlight Sensitivity", "Disadvantage on attack rolls and Perception checks relying on sight while in sunlight."),
				trait("Pack Tactics", "Advantage on an attack roll against a creature if at least one ally is within 5 feet of it."),
			},
			Actions: []MonsterAction{
				attack("Dagger", 4, "1d4+2", DamagePiercing, 5),
				ranged("Sling", 4, "1d4+2", DamageBludgeoning, 30, 120),
			},
		},
		{
			MonsterID: "srd_bandit", Name: "Bandit", Source: SourceSRD,
			Size: SizeMedium, Type: "humanoid", Alignment: "any non-lawful",
			ArmorClass: 12, ArmorNote: "leather armor",
			HitPoints: HitPoints{Current: 11, Maximum: 11}, HitDice: "2d8+2",
			Speeds:          Speeds{Walk: 30},
			AbilityScores:   AbilityScores{Strength: 11, Dexterity: 12, Constitution: 12, Intelligence: 10, Wisdom: 10, Charisma: 10},
			Languages:       []string{"common"},
			ChallengeRating: 0.125,
			Actions: []MonsterAction{
				attack("Scimitar", 3, "1d6+1", DamageSlashing, 5),
				ranged("Light Crossbow", 3, "1d8+1", DamagePiercing, 80, 320),
			},
		},
		{
			MonsterID: "srd_goblin", Name: "Goblin", Source: SourceSRD,
			Size: SizeSmall, Type: "humanoid", Subtype: "goblinoid", Alignment: "neutral evil",
			ArmorClass: 15, ArmorNote: "leather armor, shield",
			HitPoints: HitPoints{Current: 7, Maximum: 7}, HitDice: "2d6",
			Speeds:          Speeds{Walk: 30},
			AbilityScores:   AbilityScores{Strength: 8, Dexterity: 14, Constitution: 10, Intelligence: 10, Wisdom: 8, Charisma: 8},
			Skills:          map[Skill]int{SkillStealth: 6},
			Senses:          Senses{Darkvision: 60},
			Languages:       []string{"common", "goblin"},
			ChallengeRating: 0.25,
			Traits: []MonsterFeature{
				trait("Nimble Escape", "Takes the Disengage or Hide action as a bonus action on each of its turns."),
			},
			Actions: []MonsterAction{
				attack("Scimitar", 4, "1d6+2", DamageSlashing, 5),
				ranged("Shortbow", 4, "1d6+2", DamagePiercing, 80, 320),
			},
		},
		{
			MonsterID: "srd_skeleton", Name: "Skeleton", Source: SourceSRD,
			Size: SizeMedium, Type: "undead", Alignment: "lawful evil",
			ArmorClass: 13, ArmorNote: "armor scraps",
			HitPoints: HitPoints{Current: 13, Maximum: 13}, HitDice: "2d8+4",
			Speeds:        Speeds{Walk: 30},
			AbilityScores: AbilityScores{Strength: 10, Dexterity: 14, Constitution: 15, Intelligence: 6, Wisdom: 8, Charisma: 5},
			Affinities: DamageAffinities{
				Vulnerabilities: []DamageType{DamageBludgeoning},
				Immunities:      []DamageType{DamagePoison},
			},
			ConditionImmunities: []Condition{ConditionPoisoned},
			Senses:              Senses{Darkvision: 60},
			Languages:           []string{"understands_all_it_knew_in_life"},
			ChallengeRating:     0.25,
			Actions: []MonsterAction{
				attack("Shortsword", 4, "1d6+2", DamagePiercing, 5),
				ranged("Shortbow", 4, "1d6+2", DamagePiercing, 80, 320),
			},
		},
		{
			MonsterID: "srd_zombie", Name: "Zombie", Source: SourceSRD,
			Size: SizeMedium, Type: "undead", Alignment: "neutral evil",
			ArmorClass: 8,
			HitPoints:  HitPoints{Current: 22, Maximum: 22}, HitDice: "3d8+9",
			Speeds:              Speeds{Walk: 20},
			AbilityScores:       AbilityScores{Strength: 13, Dexterity: 6, Constitution: 16, Intelligence: 3, Wisdom: 6, Charisma: 5},
			SavingThrows:        map[Ability]int{AbilityWisdom: 0},
			Affinities:          DamageAffinities{Immunities: []DamageType{DamagePoison}},
			ConditionImmunities: []Condition{ConditionPoisoned},
			Senses:              Senses{Darkvision: 60},
			ChallengeRating:     0.25,
			Traits: []MonsterFeature{
				trait("Undead Fortitude", "On dropping to 0 hit points from damage that is not radiant or a critical hit, makes a Constitution save DC 5 + the damage taken; on a success it drops to 1 hit point instead."),
			},
			Actions: []MonsterAction{attack("Slam", 3, "1d6+1", DamageBludgeoning, 5)},
		},
		{
			MonsterID: "srd_wolf", Name: "Wolf", Source: SourceSRD,
			Size: SizeMedium, Type: "beast", Alignment: "unaligned",
			ArmorClass: 13, ArmorNote: "natural armor",
			HitPoints: HitPoints{Current: 11, Maximum: 11}, HitDice: "2d8+2",
			Speeds:          Speeds{Walk: 40},
			AbilityScores:   AbilityScores{Strength: 12, Dexterity: 15, Constitution: 12, Intelligence: 3, Wisdom: 12, Charisma: 6},
			Skills:          map[Skill]int{SkillPerception: 3, SkillStealth: 4},
			ChallengeRating: 0.25,
			Traits: []MonsterFeature{
				trait("Keen Hearing and Smell", "Advantage on Perception checks that rely on hearing or smell."),
				trait("Pack Tactics", "Advantage on an attack roll against a creature if at least one ally is within 5 feet of it."),
			},
			Actions: []MonsterAction{
				{
					Name: "Bite", AttackBonus: bonus(4), DamageDice: "2d4+2",
					DamageType: DamagePiercing, ReachFeet: 5,
					SaveDC: 11, SaveAbility: AbilityStrength,
					Description: "On a hit the target must succeed on a DC 11 Strength save or be knocked prone.",
				},
			},
		},
		{
			MonsterID: "srd_orc", Name: "Orc", Source: SourceSRD,
			Size: SizeMedium, Type: "humanoid", Subtype: "orc", Alignment: "chaotic evil",
			ArmorClass: 13, ArmorNote: "hide armor",
			HitPoints: HitPoints{Current: 15, Maximum: 15}, HitDice: "2d8+6",
			Speeds:          Speeds{Walk: 30},
			AbilityScores:   AbilityScores{Strength: 16, Dexterity: 12, Constitution: 16, Intelligence: 7, Wisdom: 11, Charisma: 10},
			Skills:          map[Skill]int{SkillIntimidation: 2},
			Senses:          Senses{Darkvision: 60},
			Languages:       []string{"common", "orc"},
			ChallengeRating: 0.5,
			Traits: []MonsterFeature{
				trait("Aggressive", "As a bonus action, moves up to its speed toward a hostile creature it can see."),
			},
			Actions: []MonsterAction{
				attack("Greataxe", 5, "1d12+3", DamageSlashing, 5),
				ranged("Javelin", 5, "1d6+3", DamagePiercing, 30, 120),
			},
		},
		{
			MonsterID: "srd_dire_wolf", Name: "Dire Wolf", Source: SourceSRD,
			Size: SizeLarge, Type: "beast", Alignment: "unaligned",
			ArmorClass: 14, ArmorNote: "natural armor",
			HitPoints: HitPoints{Current: 37, Maximum: 37}, HitDice: "5d10+10",
			Speeds:          Speeds{Walk: 50},
			AbilityScores:   AbilityScores{Strength: 17, Dexterity: 15, Constitution: 15, Intelligence: 3, Wisdom: 12, Charisma: 7},
			Skills:          map[Skill]int{SkillPerception: 3, SkillStealth: 4},
			ChallengeRating: 1,
			Traits: []MonsterFeature{
				trait("Keen Hearing and Smell", "Advantage on Perception checks that rely on hearing or smell."),
				trait("Pack Tactics", "Advantage on an attack roll against a creature if at least one ally is within 5 feet of it."),
			},
			Actions: []MonsterAction{
				{
					Name: "Bite", AttackBonus: bonus(5), DamageDice: "2d6+3",
					DamageType: DamagePiercing, ReachFeet: 5,
					SaveDC: 13, SaveAbility: AbilityStrength,
					Description: "On a hit the target must succeed on a DC 13 Strength save or be knocked prone.",
				},
			},
		},
		{
			MonsterID: "srd_ogre", Name: "Ogre", Source: SourceSRD,
			Size: SizeLarge, Type: "giant", Alignment: "chaotic evil",
			ArmorClass: 11, ArmorNote: "hide armor",
			HitPoints: HitPoints{Current: 59, Maximum: 59}, HitDice: "7d10+21",
			Speeds:          Speeds{Walk: 40},
			AbilityScores:   AbilityScores{Strength: 19, Dexterity: 8, Constitution: 16, Intelligence: 5, Wisdom: 7, Charisma: 7},
			Senses:          Senses{Darkvision: 60},
			Languages:       []string{"common", "giant"},
			ChallengeRating: 2,
			Actions: []MonsterAction{
				attack("Greatclub", 6, "2d8+4", DamageBludgeoning, 5),
				ranged("Javelin", 6, "2d6+4", DamagePiercing, 30, 120),
			},
		},
		{
			MonsterID: "srd_owlbear", Name: "Owlbear", Source: SourceSRD,
			Size: SizeLarge, Type: "monstrosity", Alignment: "unaligned",
			ArmorClass: 13, ArmorNote: "natural armor",
			HitPoints: HitPoints{Current: 59, Maximum: 59}, HitDice: "7d10+21",
			Speeds:          Speeds{Walk: 40},
			AbilityScores:   AbilityScores{Strength: 20, Dexterity: 12, Constitution: 17, Intelligence: 3, Wisdom: 12, Charisma: 7},
			Skills:          map[Skill]int{SkillPerception: 3},
			Senses:          Senses{Darkvision: 60},
			ChallengeRating: 3,
			Traits: []MonsterFeature{
				trait("Keen Sight and Smell", "Advantage on Perception checks that rely on sight or smell."),
			},
			Actions: []MonsterAction{
				{
					Name:        "Multiattack",
					Description: "Makes two attacks: one with its beak and one with its claws.",
					Multiattack: []MultiattackPart{{ActionName: "Beak", Count: 1}, {ActionName: "Claws", Count: 1}},
				},
				attack("Beak", 7, "1d10+5", DamagePiercing, 5),
				attack("Claws", 7, "2d8+5", DamageSlashing, 5),
			},
		},
		{
			MonsterID: "srd_wight", Name: "Wight", Source: SourceSRD,
			Size: SizeMedium, Type: "undead", Alignment: "neutral evil",
			ArmorClass: 14, ArmorNote: "studded leather",
			HitPoints: HitPoints{Current: 45, Maximum: 45}, HitDice: "6d8+18",
			Speeds:        Speeds{Walk: 30},
			AbilityScores: AbilityScores{Strength: 15, Dexterity: 14, Constitution: 16, Intelligence: 10, Wisdom: 13, Charisma: 15},
			Skills:        map[Skill]int{SkillPerception: 3, SkillStealth: 4},
			// The model has no way to say "from nonmagical attacks", so the
			// physical resistances are described in a trait instead of being
			// asserted here where they would apply to every source.
			Affinities:          DamageAffinities{Resistances: []DamageType{DamageNecrotic}},
			ConditionImmunities: []Condition{ConditionPoisoned},
			Senses:              Senses{Darkvision: 60},
			Languages:           []string{"common"},
			ChallengeRating:     3,
			Traits: []MonsterFeature{
				trait("Sunlight Sensitivity", "Disadvantage on attack rolls and Perception checks relying on sight while in sunlight."),
				trait("Nonmagical Resistance", "Resistant to bludgeoning, piercing and slashing from nonmagical attacks that are not silvered."),
			},
			Actions: []MonsterAction{
				{
					Name:        "Multiattack",
					Description: "Makes two longsword attacks or two longbow attacks. It can use Life Drain in place of one longsword attack.",
					Multiattack: []MultiattackPart{{ActionName: "Longsword", Count: 2}},
				},
				attack("Life Drain", 4, "1d6+2", DamageNecrotic, 5),
				attack("Longsword", 4, "1d8+2", DamageSlashing, 5),
				ranged("Longbow", 4, "1d8+2", DamagePiercing, 150, 600),
			},
		},
		{
			MonsterID: "srd_troll", Name: "Troll", Source: SourceSRD,
			Size: SizeLarge, Type: "giant", Alignment: "chaotic evil",
			ArmorClass: 15, ArmorNote: "natural armor",
			HitPoints: HitPoints{Current: 84, Maximum: 84}, HitDice: "8d10+40",
			Speeds:          Speeds{Walk: 30},
			AbilityScores:   AbilityScores{Strength: 18, Dexterity: 13, Constitution: 20, Intelligence: 7, Wisdom: 9, Charisma: 7},
			Skills:          map[Skill]int{SkillPerception: 2},
			Senses:          Senses{Darkvision: 60},
			Languages:       []string{"giant"},
			ChallengeRating: 5,
			Traits: []MonsterFeature{
				trait("Keen Smell", "Advantage on Perception checks that rely on smell."),
				trait("Regeneration", "Regains 10 hit points at the start of its turn unless it took acid or fire damage since its last turn."),
			},
			Actions: []MonsterAction{
				{
					Name:        "Multiattack",
					Description: "Makes three attacks: one with its bite and two with its claws.",
					Multiattack: []MultiattackPart{{ActionName: "Bite", Count: 1}, {ActionName: "Claw", Count: 2}},
				},
				attack("Bite", 7, "1d6+4", DamagePiercing, 5),
				attack("Claw", 7, "2d6+4", DamageSlashing, 5),
			},
		},
	}
}

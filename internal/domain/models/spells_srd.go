package models

// SRD spell catalogue.
//
// Unlike ClassTable and RaceTable this is a *reference*, not a closed set: a
// campaign may add spells, and a spell absent from here is not illegal. What
// it is, is unresolvable -- the engine can only roll what it has numbers for.
//
// Every entry here is content from the 5.1 SRD. Spells whose mechanics this
// model cannot yet express faithfully are deliberately left out rather than
// approximated: Sleep (a pool of hit points, not damage) and Spiritual Weapon
// (upcast every *two* levels, not every one) are the two that bite, and a
// wrong number here is worse than a missing one, because a wrong one is
// invisible.
//
// Utility entries carry no dice on purpose. SpellResolutionUtility means the
// engine has nothing to decide and the narrator describes the effect, which is
// correct for Mage Hand and would be a rules violation for Fireball.

func damage(count, sides int) DamageDice { return DamageDice{Count: count, Sides: sides} }

// spells is the source list. It is a slice rather than a map so entries read
// in level order; SpellTable is built from it and keyed by slug.
var spells = []SpellDefinition{
	// --- cantrips: attack rolls ---------------------------------------------
	{
		Name: "Fire Bolt", Level: 0, School: SchoolEvocation,
		Resolution: SpellResolutionAttack,
		Damage:     damage(1, 10), DamageType: DamageFire, Range: 120,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Ray of Frost", Level: 0, School: SchoolEvocation,
		Resolution: SpellResolutionAttack,
		Damage:     damage(1, 8), DamageType: DamageCold, Range: 60,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Shocking Grasp", Level: 0, School: SchoolEvocation,
		Resolution: SpellResolutionAttack,
		Damage:     damage(1, 8), DamageType: DamageLightning,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Chill Touch", Level: 0, School: SchoolNecromancy,
		Resolution: SpellResolutionAttack,
		Damage:     damage(1, 8), DamageType: DamageNecrotic, Range: 120,
		Classes: []Class{ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Produce Flame", Level: 0, School: SchoolConjuration,
		Resolution: SpellResolutionAttack,
		Damage:     damage(1, 8), DamageType: DamageFire, Range: 30,
		Classes: []Class{ClassDruid},
	},
	{
		Name: "Thorn Whip", Level: 0, School: SchoolTransmutation,
		Resolution: SpellResolutionAttack,
		Damage:     damage(1, 6), DamageType: DamagePiercing, Range: 30,
		Classes: []Class{ClassDruid},
	},
	{
		// The warlock's whole offence. It gains beams, not dice -- the one
		// cantrip a naive scaling rule gets wrong.
		Name: "Eldritch Blast", Level: 0, School: SchoolEvocation,
		Resolution: SpellResolutionAttack, Projectiles: 1,
		Damage: damage(1, 10), DamageType: DamageForce, Range: 120,
		Classes: []Class{ClassWarlock},
	},

	// --- cantrips: saving throws --------------------------------------------
	{
		Name: "Sacred Flame", Level: 0, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity,
		Damage: damage(1, 8), DamageType: DamageRadiant, Range: 60,
		Classes: []Class{ClassCleric},
	},
	{
		Name: "Vicious Mockery", Level: 0, School: SchoolEnchantment,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom,
		Damage: damage(1, 4), DamageType: DamagePsychic, Range: 60,
		Classes: []Class{ClassBard},
	},
	{
		Name: "Poison Spray", Level: 0, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityConstitution,
		Damage: damage(1, 12), DamageType: DamagePoison, Range: 10,
		Classes: []Class{ClassDruid, ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Acid Splash", Level: 0, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity,
		Damage: damage(1, 6), DamageType: DamageAcid, Range: 60,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},

	// --- cantrips: utility --------------------------------------------------
	{Name: "Light", Level: 0, School: SchoolEvocation, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassBard, ClassCleric, ClassSorcerer, ClassWizard}},
	{Name: "Mage Hand", Level: 0, School: SchoolConjuration, Resolution: SpellResolutionUtility, Range: 30,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Minor Illusion", Level: 0, School: SchoolIllusion, Resolution: SpellResolutionUtility, Range: 30,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Prestidigitation", Level: 0, School: SchoolTransmutation, Resolution: SpellResolutionUtility, Range: 10,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Guidance", Level: 0, School: SchoolDivination, Resolution: SpellResolutionUtility, Concentration: true,
		Classes: []Class{ClassCleric, ClassDruid}},
	{Name: "Resistance", Level: 0, School: SchoolAbjuration, Resolution: SpellResolutionUtility, Concentration: true,
		Classes: []Class{ClassCleric, ClassDruid}},
	{Name: "Spare the Dying", Level: 0, School: SchoolNecromancy, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassCleric}},
	{Name: "Mending", Level: 0, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassBard, ClassCleric, ClassDruid, ClassSorcerer, ClassWizard}},
	{Name: "Dancing Lights", Level: 0, School: SchoolEvocation, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 120,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWizard}},
	{Name: "Druidcraft", Level: 0, School: SchoolTransmutation, Resolution: SpellResolutionUtility, Range: 30,
		Classes: []Class{ClassDruid}},
	{Name: "Message", Level: 0, School: SchoolTransmutation, Resolution: SpellResolutionUtility, Range: 120,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWizard}},
	{Name: "Thaumaturgy", Level: 0, School: SchoolTransmutation, Resolution: SpellResolutionUtility, Range: 30,
		Classes: []Class{ClassCleric}},
	{Name: "Blade Ward", Level: 0, School: SchoolAbjuration, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "True Strike", Level: 0, School: SchoolDivination, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 30,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},

	// --- level 1 ------------------------------------------------------------
	{
		// Never misses, which is why it is Automatic rather than an attack.
		Name: "Magic Missile", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionAuto,
		Damage:     DamageDice{Count: 1, Sides: 4, Bonus: 1}, DamageType: DamageForce,
		Projectiles: 3, UpcastProjectiles: 1, Range: 120,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Burning Hands", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity, HalfOnSave: true,
		Damage: damage(3, 6), DamageType: DamageFire, UpcastDamage: damage(1, 6), Range: 15,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Thunderwave", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityConstitution, HalfOnSave: true,
		Damage: damage(2, 8), DamageType: DamageThunder, UpcastDamage: damage(1, 8), Range: 15,
		Classes: []Class{ClassBard, ClassDruid, ClassSorcerer, ClassWizard},
	},
	{
		Name: "Guiding Bolt", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionAttack,
		Damage:     damage(4, 6), DamageType: DamageRadiant, UpcastDamage: damage(1, 6), Range: 120,
		Classes: []Class{ClassCleric},
	},
	{
		Name: "Inflict Wounds", Level: 1, School: SchoolNecromancy,
		Resolution: SpellResolutionAttack,
		Damage:     damage(3, 10), DamageType: DamageNecrotic, UpcastDamage: damage(1, 10),
		Classes: []Class{ClassCleric},
	},
	{
		Name: "Witch Bolt", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionAttack, Concentration: true,
		Damage: damage(1, 12), DamageType: DamageLightning, UpcastDamage: damage(1, 12), Range: 30,
		Classes: []Class{ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Ray of Sickness", Level: 1, School: SchoolNecromancy,
		Resolution: SpellResolutionAttack,
		Damage:     damage(2, 8), DamageType: DamagePoison, UpcastDamage: damage(1, 8),
		Condition: ConditionPoisoned, Range: 60,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Cure Wounds", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionAuto,
		Healing:    damage(1, 8), UpcastHealing: damage(1, 8), AddsAbilityModifier: true,
		Classes: []Class{ClassBard, ClassCleric, ClassDruid, ClassPaladin, ClassRanger},
	},
	{
		Name: "Healing Word", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionAuto,
		Healing:    damage(1, 4), UpcastHealing: damage(1, 4), AddsAbilityModifier: true, Range: 60,
		Classes: []Class{ClassBard, ClassCleric, ClassDruid},
	},
	{
		Name: "Charm Person", Level: 1, School: SchoolEnchantment,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom,
		Condition: ConditionCharmed, Range: 30,
		Classes: []Class{ClassBard, ClassDruid, ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Entangle", Level: 1, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityStrength,
		Condition: ConditionRestrained, Concentration: true, Range: 90,
		Classes: []Class{ClassDruid},
	},
	{
		Name: "Faerie Fire", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity,
		Concentration: true, Range: 60,
		Classes: []Class{ClassBard, ClassDruid},
	},
	{Name: "Shield", Level: 1, School: SchoolAbjuration, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassSorcerer, ClassWizard}},
	{Name: "Mage Armor", Level: 1, School: SchoolAbjuration, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassSorcerer, ClassWizard}},
	{Name: "Bless", Level: 1, School: SchoolEnchantment, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 30,
		Classes: []Class{ClassCleric, ClassPaladin}},
	{Name: "Detect Magic", Level: 1, School: SchoolDivination, Resolution: SpellResolutionUtility,
		Concentration: true, Ritual: true,
		Classes: []Class{ClassBard, ClassCleric, ClassDruid, ClassPaladin, ClassRanger, ClassSorcerer, ClassWizard}},
	{Name: "Hunter's Mark", Level: 1, School: SchoolDivination, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 90,
		Classes: []Class{ClassRanger}},
	{Name: "Hex", Level: 1, School: SchoolEnchantment, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 90,
		Classes: []Class{ClassWarlock}},
	{Name: "Goodberry", Level: 1, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassDruid, ClassRanger}},
	{Name: "Feather Fall", Level: 1, School: SchoolTransmutation, Resolution: SpellResolutionUtility, Range: 60,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWizard}},
	{Name: "Longstrider", Level: 1, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassBard, ClassDruid, ClassRanger, ClassWizard}},
	{Name: "Comprehend Languages", Level: 1, School: SchoolDivination, Resolution: SpellResolutionUtility,
		Ritual:  true,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Disguise Self", Level: 1, School: SchoolIllusion, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWizard}},
	{Name: "Silent Image", Level: 1, School: SchoolIllusion, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 60,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWizard}},

	// --- level 2 ------------------------------------------------------------
	{
		// Each ray is its own attack roll: it can hit twice and miss once.
		Name: "Scorching Ray", Level: 2, School: SchoolEvocation,
		Resolution: SpellResolutionAttack, Projectiles: 3, UpcastProjectiles: 1,
		Damage: damage(2, 6), DamageType: DamageFire, Range: 120,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Acid Arrow", Level: 2, School: SchoolEvocation,
		Resolution: SpellResolutionAttack,
		Damage:     damage(4, 4), DamageType: DamageAcid, UpcastDamage: damage(1, 4), Range: 90,
		Classes: []Class{ClassWizard},
	},
	{
		Name: "Shatter", Level: 2, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityConstitution, HalfOnSave: true,
		Damage: damage(3, 8), DamageType: DamageThunder, UpcastDamage: damage(1, 8), Range: 60,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Moonbeam", Level: 2, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityConstitution, HalfOnSave: true,
		Damage: damage(2, 10), DamageType: DamageRadiant, UpcastDamage: damage(1, 10),
		Concentration: true, Range: 120,
		Classes: []Class{ClassDruid},
	},
	{
		Name: "Flaming Sphere", Level: 2, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity, HalfOnSave: true,
		Damage: damage(2, 6), DamageType: DamageFire, UpcastDamage: damage(1, 6),
		Concentration: true, Range: 60,
		Classes: []Class{ClassDruid, ClassWizard},
	},
	{
		// No attack and no save for the damage itself: it simply burns.
		Name: "Heat Metal", Level: 2, School: SchoolTransmutation,
		Resolution: SpellResolutionAuto,
		Damage:     damage(2, 8), DamageType: DamageFire, UpcastDamage: damage(1, 8),
		Concentration: true, Range: 60,
		Classes: []Class{ClassBard, ClassDruid},
	},
	{
		Name: "Hold Person", Level: 2, School: SchoolEnchantment,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom,
		Condition: ConditionParalyzed, Concentration: true, Range: 60,
		Classes: []Class{ClassBard, ClassCleric, ClassDruid, ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Web", Level: 2, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity,
		Condition: ConditionRestrained, Concentration: true, Range: 60,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Suggestion", Level: 2, School: SchoolEnchantment,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom,
		Condition: ConditionCharmed, Concentration: true, Range: 30,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{Name: "Misty Step", Level: 2, School: SchoolConjuration, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Invisibility", Level: 2, School: SchoolIllusion, Resolution: SpellResolutionUtility,
		Concentration: true,
		Classes:       []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Blur", Level: 2, School: SchoolIllusion, Resolution: SpellResolutionUtility,
		Concentration: true,
		Classes:       []Class{ClassSorcerer, ClassWizard}},
	{Name: "Darkness", Level: 2, School: SchoolEvocation, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 60,
		Classes: []Class{ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Mirror Image", Level: 2, School: SchoolIllusion, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Lesser Restoration", Level: 2, School: SchoolAbjuration, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassBard, ClassCleric, ClassDruid, ClassPaladin, ClassRanger}},
	{Name: "Aid", Level: 2, School: SchoolAbjuration, Resolution: SpellResolutionUtility, Range: 30,
		Classes: []Class{ClassCleric, ClassPaladin}},
	{Name: "Pass Without Trace", Level: 2, School: SchoolAbjuration, Resolution: SpellResolutionUtility,
		Concentration: true,
		Classes:       []Class{ClassDruid, ClassRanger}},
	{Name: "Magic Weapon", Level: 2, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Concentration: true,
		Classes:       []Class{ClassPaladin, ClassWizard}},
	{Name: "Spike Growth", Level: 2, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 150,
		Classes: []Class{ClassDruid, ClassRanger}},

	// --- level 3 ------------------------------------------------------------
	{
		Name: "Fireball", Level: 3, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity, HalfOnSave: true,
		Damage: damage(8, 6), DamageType: DamageFire, UpcastDamage: damage(1, 6), Range: 150,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Lightning Bolt", Level: 3, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity, HalfOnSave: true,
		Damage: damage(8, 6), DamageType: DamageLightning, UpcastDamage: damage(1, 6), Range: 100,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{
		Name: "Spirit Guardians", Level: 3, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom, HalfOnSave: true,
		Damage: damage(3, 8), DamageType: DamageRadiant, UpcastDamage: damage(1, 8),
		Concentration: true, Range: 15,
		Classes: []Class{ClassCleric},
	},
	{
		Name: "Call Lightning", Level: 3, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity, HalfOnSave: true,
		Damage: damage(3, 10), DamageType: DamageLightning, UpcastDamage: damage(1, 10),
		Concentration: true, Range: 120,
		Classes: []Class{ClassDruid},
	},
	{
		Name: "Vampiric Touch", Level: 3, School: SchoolNecromancy,
		Resolution: SpellResolutionAttack,
		Damage:     damage(3, 6), DamageType: DamageNecrotic, UpcastDamage: damage(1, 6),
		Concentration: true,
		Classes:       []Class{ClassWarlock, ClassWizard},
	},
	{
		Name: "Fear", Level: 3, School: SchoolIllusion,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom,
		Condition: ConditionFrightened, Concentration: true, Range: 30,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Hypnotic Pattern", Level: 3, School: SchoolIllusion,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom,
		Condition: ConditionCharmed, Concentration: true, Range: 120,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard},
	},
	{
		Name: "Stinking Cloud", Level: 3, School: SchoolConjuration,
		Resolution: SpellResolutionSave, SaveAbility: AbilityConstitution,
		Concentration: true, Range: 90,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWizard},
	},
	{
		Name: "Slow", Level: 3, School: SchoolTransmutation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityWisdom,
		Concentration: true, Range: 120,
		Classes: []Class{ClassSorcerer, ClassWizard},
	},
	{Name: "Counterspell", Level: 3, School: SchoolAbjuration, Resolution: SpellResolutionUtility, Range: 60,
		Classes: []Class{ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Dispel Magic", Level: 3, School: SchoolAbjuration, Resolution: SpellResolutionUtility, Range: 120,
		Classes: []Class{ClassBard, ClassCleric, ClassDruid, ClassPaladin, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Fly", Level: 3, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Concentration: true,
		Classes:       []Class{ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Haste", Level: 3, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 30,
		Classes: []Class{ClassSorcerer, ClassWizard}},
	{Name: "Revivify", Level: 3, School: SchoolNecromancy, Resolution: SpellResolutionUtility,
		Classes: []Class{ClassCleric, ClassPaladin}},
	{Name: "Water Breathing", Level: 3, School: SchoolTransmutation, Resolution: SpellResolutionUtility,
		Ritual: true, Range: 30,
		Classes: []Class{ClassDruid, ClassRanger, ClassSorcerer, ClassWizard}},
	{Name: "Major Image", Level: 3, School: SchoolIllusion, Resolution: SpellResolutionUtility,
		Concentration: true, Range: 120,
		Classes: []Class{ClassBard, ClassSorcerer, ClassWarlock, ClassWizard}},
	{Name: "Animate Dead", Level: 3, School: SchoolNecromancy, Resolution: SpellResolutionUtility, Range: 10,
		Classes: []Class{ClassCleric, ClassWizard}},
}

// SpellTable is every spell this system can resolve, keyed by slug.
var SpellTable = buildSpellTable()

func buildSpellTable() map[string]SpellDefinition {
	table := make(map[string]SpellDefinition, len(spells))
	for _, def := range spells {
		def.Key = SpellKey(def.Name)
		def.Source = SourcePHB
		table[def.Key] = def
	}
	return table
}

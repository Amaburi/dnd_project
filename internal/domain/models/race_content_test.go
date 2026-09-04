package models

import (
	"strings"
	"testing"
)

// Racial training was previously only a trait name, so a mountain dwarf's
// armour training and an elf's weapon training never reached the sheet.
func TestRacialProficienciesAreGranted(t *testing.T) {
	mountain := RaceDwarf.RacialProficiencies("mountain")
	if !mountain.HasArmor(ArmorMedium) {
		t.Error("a mountain dwarf should be trained in medium armour")
	}
	if !mountain.HasArmor(ArmorLight) {
		t.Error("a mountain dwarf should be trained in light armour")
	}
	// Dwarven Combat Training comes from the race, not the subrace.
	for _, w := range []string{"battleaxe", "handaxe", "light_hammer", "warhammer"} {
		if !contains(mountain.Weapons, w) {
			t.Errorf("dwarves should be trained with a %s", w)
		}
	}

	// Hill dwarves get the weapons but not the armour.
	hill := RaceDwarf.RacialProficiencies("hill")
	if hill.HasArmor(ArmorMedium) {
		t.Error("a hill dwarf has no armour training")
	}

	drow := RaceElf.RacialProficiencies("drow")
	for _, w := range []string{"rapier", "shortsword", "hand_crossbow"} {
		if !contains(drow.Weapons, w) {
			t.Errorf("drow should be trained with a %s", w)
		}
	}
	if contains(drow.Weapons, "longbow") {
		t.Error("drow weapon training is not the same as other elves'")
	}

	if !RaceGnome.RacialProficiencies("rock").HasTool("tinkers_tools") {
		t.Error("a rock gnome should be proficient with tinker's tools")
	}
}

// A mountain dwarf wizard really is proficient in the armour they wear.
func TestApplyClassDefaultsMergesRacialProficiencies(t *testing.T) {
	c := &Character{
		BasicInfo: BasicInfo{
			Race: RaceDwarf, Subrace: "mountain", Background: BackgroundSage,
			Classes: []ClassLevel{{Class: ClassWizard, Subclass: "evocation", Level: 3}},
		},
		AbilityScores: AbilityScores{Intelligence: 16, Strength: 14},
		Equipment: Equipment{Armor: &InventoryItem{
			Name: "Chain Shirt", Kind: ItemArmor,
			Armor: &ArmorProperties{Category: ArmorMedium, BaseAC: 13},
		}},
	}

	c.ApplyClassDefaults()

	if !c.IsArmorProficient() {
		t.Error("a mountain dwarf wizard in a chain shirt is proficient with it")
	}

	// And the weapon training reaches the attack maths.
	warhammer := InventoryItem{
		Key: "warhammer", Name: "Warhammer", Kind: ItemWeapon,
		Weapon: &WeaponProperties{Category: WeaponMartial, DamageDice: "1d8", DamageType: DamageBludgeoning},
	}
	profile, err := c.AttackWith(warhammer)
	if err != nil {
		t.Fatalf("AttackWith: %v", err)
	}
	if !profile.Proficient {
		t.Error("Dwarven Combat Training should make a warhammer proficient for a wizard")
	}
	// STR +2 and proficiency +2.
	if profile.AttackBonus != 4 {
		t.Errorf("attack bonus = %+d, want +4", profile.AttackBonus)
	}
}

// Dwarven Toughness is a hit point per level and had nowhere to be counted.
func TestHillDwarfBonusHitPoints(t *testing.T) {
	if got := RaceDwarf.BonusHitPointsPerLevel("hill"); got != 1 {
		t.Errorf("hill dwarf bonus = %d, want 1", got)
	}
	if got := RaceDwarf.BonusHitPointsPerLevel("mountain"); got != 0 {
		t.Errorf("mountain dwarf bonus = %d, want 0", got)
	}

	hill := &Character{
		BasicInfo: BasicInfo{Race: RaceDwarf, Subrace: "hill",
			Classes: []ClassLevel{{Class: ClassCleric, Subclass: "life", Level: 4}}},
		AbilityScores: AbilityScores{Constitution: 14}, // +2
	}
	mountain := &Character{
		BasicInfo: BasicInfo{Race: RaceDwarf, Subrace: "mountain",
			Classes: []ClassLevel{{Class: ClassCleric, Subclass: "life", Level: 4}}},
		AbilityScores: AbilityScores{Constitution: 14},
	}

	// d8 cleric: 8 + 2 at first, then 3 x (5 + 2). Hill dwarves add 1 each level.
	if got, want := mountain.ExpectedMaxHitPoints(), 10+21; got != want {
		t.Errorf("mountain dwarf cleric 4 = %d hp, want %d", got, want)
	}
	if got, want := hill.ExpectedMaxHitPoints(), 10+21+4; got != want {
		t.Errorf("hill dwarf cleric 4 = %d hp, want %d", got, want)
	}
}

func TestExpectedMaxHitPointsAcrossClasses(t *testing.T) {
	// Level 1 fighter with CON 16: a full d10 plus 3.
	c := &Character{
		BasicInfo:     BasicInfo{Classes: []ClassLevel{{Class: ClassFighter, Level: 1}}},
		AbilityScores: AbilityScores{Constitution: 16},
	}
	if got := c.ExpectedMaxHitPoints(); got != 13 {
		t.Errorf("fighter 1 = %d hp, want 13", got)
	}

	// Fighter 3 / Wizard 2: 10+3, then 2 x (6+3), then 2 x (4+3).
	c.BasicInfo.Classes = []ClassLevel{
		{Class: ClassFighter, Subclass: "champion", Level: 3},
		{Class: ClassWizard, Subclass: "evocation", Level: 2},
	}
	if got, want := c.ExpectedMaxHitPoints(), 13+18+14; got != want {
		t.Errorf("fighter 3 / wizard 2 = %d hp, want %d", got, want)
	}
}

func TestDraconicAncestry(t *testing.T) {
	if !RaceDragonborn.RequiresSubrace() {
		t.Fatal("dragonborn must choose a draconic ancestry")
	}
	if got := len(RaceDragonborn.SubraceKeys()); got != 10 {
		t.Errorf("got %d ancestries, want 10", got)
	}

	cases := []struct {
		ancestry string
		damage   DamageType
		shape    string
		save     Ability
	}{
		{"red", DamageFire, breathCone, AbilityDexterity},
		{"blue", DamageLightning, breathLine, AbilityDexterity},
		{"green", DamagePoison, breathCone, AbilityConstitution},
		{"white", DamageCold, breathCone, AbilityConstitution},
		{"copper", DamageAcid, breathLine, AbilityDexterity},
	}
	for _, tc := range cases {
		breath, ok := RaceDragonborn.Breath(tc.ancestry)
		if !ok {
			t.Errorf("%s dragonborn has no breath weapon", tc.ancestry)
			continue
		}
		if breath.DamageType != tc.damage {
			t.Errorf("%s breath = %s, want %s", tc.ancestry, breath.DamageType, tc.damage)
		}
		if breath.Shape != tc.shape {
			t.Errorf("%s breath shape = %s, want %s", tc.ancestry, breath.Shape, tc.shape)
		}
		if breath.SaveAbility != tc.save {
			t.Errorf("%s breath save = %s, want %s", tc.ancestry, breath.SaveAbility, tc.save)
		}
		// Ancestry also grants resistance to its own damage type.
		if res := RaceDragonborn.DamageResistances(tc.ancestry); len(res) != 1 || res[0] != tc.damage {
			t.Errorf("%s resistances = %v, want %s", tc.ancestry, res, tc.damage)
		}
	}
}

func TestBreathWeaponScalesWithLevel(t *testing.T) {
	cases := map[int]string{1: "2d6", 5: "2d6", 6: "3d6", 10: "3d6", 11: "4d6", 15: "4d6", 16: "5d6", 20: "5d6"}
	for level, want := range cases {
		if got := BreathWeaponDice(level); got != want {
			t.Errorf("BreathWeaponDice(%d) = %s, want %s", level, got, want)
		}
	}

	c := &Character{
		BasicInfo: BasicInfo{Race: RaceDragonborn, Subrace: "red",
			Classes: []ClassLevel{{Class: ClassPaladin, Subclass: "devotion", Level: 6}}},
		AbilityScores: AbilityScores{Constitution: 16}, // +3
	}
	weapon, dice, dc, ok := c.BreathWeapon()
	if !ok {
		t.Fatal("a red dragonborn should have a breath weapon")
	}
	if weapon.DamageType != DamageFire {
		t.Errorf("damage type = %s, want fire", weapon.DamageType)
	}
	if dice != "3d6" {
		t.Errorf("dice at level 6 = %s, want 3d6", dice)
	}
	// 8 + CON 3 + proficiency 3.
	if dc != 14 {
		t.Errorf("save DC = %d, want 14", dc)
	}

	// Everyone else has none.
	elf := &Character{BasicInfo: BasicInfo{Race: RaceElf, Subrace: "high"}}
	if _, _, _, ok := elf.BreathWeapon(); ok {
		t.Error("elves do not breathe fire")
	}
}

func TestVariantHuman(t *testing.T) {
	if !RaceHuman.RequiresSubrace() {
		t.Fatal("humans now choose between standard and variant")
	}

	variant, ok := RaceHuman.Subrace("variant")
	if !ok {
		t.Fatal("variant human should exist")
	}
	if variant.AbilityChoices != 2 {
		t.Errorf("variant ability choices = %d, want 2", variant.AbilityChoices)
	}
	if variant.SkillChoices != 1 {
		t.Errorf("variant skill choices = %d, want 1", variant.SkillChoices)
	}
	if variant.FeatChoices != 1 {
		t.Errorf("variant feat choices = %d, want 1", variant.FeatChoices)
	}

	// A standard human gets no free choices, a variant gets two.
	if got := RaceHuman.AbilityChoiceCount("standard"); got != 0 {
		t.Errorf("standard human ability choices = %d, want 0", got)
	}
	if got := RaceHuman.AbilityChoiceCount("variant"); got != 2 {
		t.Errorf("variant human ability choices = %d, want 2", got)
	}

	// The extra skill counts toward the budget.
	c := &Character{BasicInfo: BasicInfo{
		Race: RaceHuman, Subrace: "variant", Background: BackgroundSoldier,
		Classes: []ClassLevel{{Class: ClassFighter, Subclass: "champion", Level: 3}},
	}}
	// soldier 2 granted + fighter 2 chosen + variant 1
	if got := c.SkillBudget(); got != 5 {
		t.Errorf("variant human fighter budget = %d, want 5", got)
	}
}

func TestHighElfExtraLanguageAndCantrip(t *testing.T) {
	high, ok := RaceElf.Subrace("high")
	if !ok {
		t.Fatal("high elf should exist")
	}
	if high.LanguageChoices != 1 {
		t.Errorf("high elf language choices = %d, want 1", high.LanguageChoices)
	}
	if high.CantripChoices != 1 {
		t.Errorf("high elf cantrip choices = %d, want 1", high.CantripChoices)
	}

	// Wood elves get neither.
	wood, _ := RaceElf.Subrace("wood")
	if wood.LanguageChoices != 0 || wood.CantripChoices != 0 {
		t.Errorf("wood elf should get no extra language or cantrip, got %+v", wood)
	}
}

// Small creatures have disadvantage with Heavy weapons.
func TestSmallCreatureHeavyWeaponDisadvantage(t *testing.T) {
	greataxe := InventoryItem{
		Key: "greataxe", Name: "Greataxe", Kind: ItemWeapon,
		Weapon: &WeaponProperties{Category: WeaponMartial, DamageDice: "1d12",
			Properties: []WeaponProperty{PropertyHeavy, PropertyTwoHanded}},
	}

	halfling := &Character{
		BasicInfo:     BasicInfo{Race: RaceHalfling, Subrace: "stout"},
		Proficiencies: Proficiencies{Weapons: []string{ProfMartialWeapons}},
	}
	if got := halfling.AttackRollMode(greataxe); got != RollDisadvantage {
		t.Errorf("halfling with a greataxe = %s, want disadvantage", got)
	}
	if got := halfling.AttackRollMode(longsword()); got != RollNormal {
		t.Errorf("halfling with a longsword = %s, want normal", got)
	}

	human := &Character{BasicInfo: BasicInfo{Race: RaceHuman, Subrace: "standard"}}
	if got := human.AttackRollMode(greataxe); got != RollNormal {
		t.Errorf("human with a greataxe = %s, want normal", got)
	}

	// The profile carries it through.
	profile, err := halfling.AttackWith(greataxe)
	if err != nil {
		t.Fatalf("AttackWith: %v", err)
	}
	if profile.Mode != RollDisadvantage {
		t.Errorf("attack profile mode = %s, want disadvantage", profile.Mode)
	}

	// Exhaustion is the other source, and the two do not stack.
	tired := &Character{BasicInfo: BasicInfo{Race: RaceHalfling, Subrace: "stout"}, Exhaustion: 3}
	if got := tired.AttackRollMode(greataxe); got != RollDisadvantage {
		t.Errorf("exhausted halfling = %s, want disadvantage (not doubled)", got)
	}
}

func TestRaceSourcesAndNames(t *testing.T) {
	phb := RacesFromSource(SourcePHB)
	if len(phb) != 9 {
		t.Errorf("got %d PHB races, want 9", len(phb))
	}

	for _, r := range Races {
		def := RaceTable[r]
		if def.Name == "" {
			t.Errorf("race %q has no display name", r)
		}
		if def.Source == "" {
			t.Errorf("race %q has no source", r)
		}
		for key, sub := range def.Subraces {
			if sub.Name == "" {
				t.Errorf("%s/%s has no display name", r, key)
			}
			if sub.Source == "" {
				t.Errorf("%s/%s has no source", r, key)
			}
			if strings.ContainsAny(key, " ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				t.Errorf("%s subrace key %q should be lower_snake_case", r, key)
			}
		}
	}

	// A non-PHB race is reachable and labelled.
	if got := RaceTable[RaceTabaxi].Source; got != SourceVGtM {
		t.Errorf("tabaxi source = %q, want %q", got, SourceVGtM)
	}
	if got := RaceTabaxi.GrantedSkills(""); len(got) != 2 {
		t.Errorf("tabaxi skills = %v, want Perception and Stealth", got)
	}
}

func TestFireGenasiDarkvisionOverride(t *testing.T) {
	// The base genasi has no darkvision; only the fire subrace does.
	if got := RaceGenasi.Darkvision("air"); got != 0 {
		t.Errorf("air genasi darkvision = %d, want 0", got)
	}
	if got := RaceGenasi.Darkvision("fire"); got != 60 {
		t.Errorf("fire genasi darkvision = %d, want 60", got)
	}
	if res := RaceGenasi.DamageResistances("fire"); len(res) != 1 || res[0] != DamageFire {
		t.Errorf("fire genasi resistances = %v, want fire", res)
	}
}

func TestValidateSheetRequiresDraconicAncestry(t *testing.T) {
	c := &Character{
		Name: "Kaan", Type: CharacterPlayer,
		BasicInfo: BasicInfo{
			Race: RaceDragonborn, Background: BackgroundSoldier,
			Classes: []ClassLevel{{Class: ClassPaladin, Subclass: "devotion", Level: 3}},
		},
		AbilityScores: AbilityScores{Strength: 16, Charisma: 14, Constitution: 14},
		Skills: SkillProficiencies{
			SkillAthletics:    ProficiencyProficient,
			SkillIntimidation: ProficiencyProficient,
			SkillPersuasion:   ProficiencyProficient,
			SkillReligion:     ProficiencyProficient,
		},
	}

	err := c.ValidateSheet()
	if err == nil || !strings.Contains(err.Error(), "subrace") {
		t.Fatalf("a dragonborn without an ancestry should be rejected, got %v", err)
	}

	c.BasicInfo.Subrace = "gold"
	if err := c.ValidateSheet(); err != nil {
		t.Errorf("a gold dragonborn paladin was rejected: %v", err)
	}
}

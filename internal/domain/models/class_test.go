package models

import "testing"

// The class table is what every other class-derived value reads from, so an
// entry that is internally inconsistent breaks everything downstream.
func TestClassTableIsComplete(t *testing.T) {
	if len(Classes) != 12 {
		t.Fatalf("got %d classes, want the 12 in the PHB", len(Classes))
	}

	validDice := map[int]bool{6: true, 8: true, 10: true, 12: true}

	for _, c := range Classes {
		def, ok := ClassTable[c]
		if !ok {
			t.Errorf("class %q has no definition", c)
			continue
		}
		if !validDice[def.HitDie] {
			t.Errorf("%s hit die is d%d, want d6/d8/d10/d12", c, def.HitDie)
		}
		if len(def.SavingThrows) != 2 {
			t.Errorf("%s has %d saving throw proficiencies, want 2", c, len(def.SavingThrows))
		}
		for _, a := range def.SavingThrows {
			if !a.Valid() {
				t.Errorf("%s has invalid saving throw ability %q", c, a)
			}
		}
		if def.SkillChoices < 1 {
			t.Errorf("%s grants %d skill choices, want at least 1", c, def.SkillChoices)
		}
		// A nil skill list means "any skill" (bard); a non-nil one must offer
		// at least as many options as the class may choose.
		if def.SkillList != nil && len(def.SkillList) < def.SkillChoices {
			t.Errorf("%s may choose %d skills from a list of %d", c, def.SkillChoices, len(def.SkillList))
		}
		for _, s := range def.SkillList {
			if !s.Valid() {
				t.Errorf("%s lists invalid skill %q", c, s)
			}
		}
		if def.SubclassLevel < 1 || def.SubclassLevel > 3 {
			t.Errorf("%s picks a subclass at level %d, want 1-3", c, def.SubclassLevel)
		}
		if len(def.Subclasses) == 0 {
			t.Errorf("%s has no subclasses", c)
		}
		if def.Progression != CasterNone && !def.SpellcastingAbility.Valid() {
			t.Errorf("%s is a %s caster with no spellcasting ability", c, def.Progression)
		}
		if def.Progression == CasterNone && def.SpellcastingAbility != "" {
			t.Errorf("%s is not a caster but names ability %q", c, def.SpellcastingAbility)
		}
	}
}

func TestKnownHitDiceAndSaves(t *testing.T) {
	cases := []struct {
		class Class
		die   int
		saves [2]Ability
	}{
		{ClassBarbarian, 12, [2]Ability{AbilityStrength, AbilityConstitution}},
		{ClassWizard, 6, [2]Ability{AbilityIntelligence, AbilityWisdom}},
		{ClassRogue, 8, [2]Ability{AbilityDexterity, AbilityIntelligence}},
		{ClassFighter, 10, [2]Ability{AbilityStrength, AbilityConstitution}},
		{ClassMonk, 8, [2]Ability{AbilityStrength, AbilityDexterity}},
	}

	for _, tc := range cases {
		def := ClassTable[tc.class]
		if def.HitDie != tc.die {
			t.Errorf("%s hit die = d%d, want d%d", tc.class, def.HitDie, tc.die)
		}
		if def.SavingThrows[0] != tc.saves[0] || def.SavingThrows[1] != tc.saves[1] {
			t.Errorf("%s saves = %v, want %v", tc.class, def.SavingThrows, tc.saves)
		}
	}
}

// Clerics, sorcerers and warlocks choose at 1; druids and wizards at 2;
// everyone else at 3.
func TestSubclassLevels(t *testing.T) {
	cases := map[Class]int{
		ClassCleric: 1, ClassSorcerer: 1, ClassWarlock: 1,
		ClassDruid: 2, ClassWizard: 2,
		ClassFighter: 3, ClassRogue: 3, ClassBarbarian: 3, ClassBard: 3,
		ClassMonk: 3, ClassPaladin: 3, ClassRanger: 3,
	}
	for class, want := range cases {
		if got := ClassTable[class].SubclassLevel; got != want {
			t.Errorf("%s picks a subclass at %d, want %d", class, got, want)
		}
	}

	if !ClassFighter.HasSubclass("eldritch_knight") {
		t.Error("fighter should have the eldritch knight archetype")
	}
	if ClassFighter.HasSubclass("thief") {
		t.Error("thief is a rogue archetype, not a fighter one")
	}
}

// "Strength 13 or Dexterity 13" for a fighter is a different shape from
// "Dexterity 13 and Wisdom 13" for a monk.
func TestMulticlassPrerequisites(t *testing.T) {
	strongOnly := AbilityScores{Strength: 15, Dexterity: 8, Wisdom: 8, Charisma: 8}
	quickOnly := AbilityScores{Strength: 8, Dexterity: 15, Wisdom: 8, Charisma: 8}
	quickAndWise := AbilityScores{Strength: 8, Dexterity: 15, Wisdom: 14, Charisma: 8}

	// Fighter accepts either alternative.
	if !ClassFighter.MeetsMulticlassPrerequisites(strongOnly) {
		t.Error("Strength 15 should satisfy fighter")
	}
	if !ClassFighter.MeetsMulticlassPrerequisites(quickOnly) {
		t.Error("Dexterity 15 should satisfy fighter")
	}

	// Monk needs both.
	if ClassMonk.MeetsMulticlassPrerequisites(quickOnly) {
		t.Error("Dexterity alone should not satisfy monk, which also needs Wisdom 13")
	}
	if !ClassMonk.MeetsMulticlassPrerequisites(quickAndWise) {
		t.Error("Dexterity 15 and Wisdom 14 should satisfy monk")
	}

	// Paladin needs Strength and Charisma.
	if ClassPaladin.MeetsMulticlassPrerequisites(strongOnly) {
		t.Error("Strength alone should not satisfy paladin, which also needs Charisma 13")
	}

	if got, want := ClassFighter.DescribePrerequisites(), "Strength 13 or Dexterity 13"; got != want {
		t.Errorf("fighter prerequisites described as %q, want %q", got, want)
	}
	if got, want := ClassMonk.DescribePrerequisites(), "Dexterity 13 and Wisdom 13"; got != want {
		t.Errorf("monk prerequisites described as %q, want %q", got, want)
	}
}

func TestSingleClassFullCasterSlots(t *testing.T) {
	// A 5th-level wizard: 4 first, 3 second, 2 third.
	slots, pact, _ := SpellSlotsForClasses([]ClassLevel{{Class: ClassWizard, Level: 5}})
	if pact != 0 {
		t.Errorf("wizard reported %d pact slots, want 0", pact)
	}
	want := []SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 3}, {Level: 3, Total: 2}}
	assertSlots(t, "wizard 5", slots, want)

	// A 20th-level full caster tops out with one 8th and one 9th.
	slots, _, _ = SpellSlotsForClasses([]ClassLevel{{Class: ClassBard, Level: 20}})
	if len(slots) != 9 {
		t.Fatalf("bard 20 has slots at %d levels, want 9", len(slots))
	}
	if slots[8].Total != 1 || slots[8].Level != 9 {
		t.Errorf("bard 20 ninth-level slots = %+v, want one", slots[8])
	}
}

// A single-classed half caster uses its own printed table, which rounds up:
// a paladin has slots at level 2, not level 4.
func TestSingleClassHalfCasterSlots(t *testing.T) {
	if slots, _, _ := SpellSlotsForClasses([]ClassLevel{{Class: ClassPaladin, Level: 1}}); len(slots) != 0 {
		t.Errorf("paladin 1 has %d slot levels, want none", len(slots))
	}

	slots, _, _ := SpellSlotsForClasses([]ClassLevel{{Class: ClassPaladin, Level: 2}})
	assertSlots(t, "paladin 2", slots, []SpellSlot{{Level: 1, Total: 2}})

	slots, _, _ = SpellSlotsForClasses([]ClassLevel{{Class: ClassPaladin, Level: 5}})
	assertSlots(t, "paladin 5", slots, []SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 2}})

	slots, _, _ = SpellSlotsForClasses([]ClassLevel{{Class: ClassRanger, Level: 20}})
	assertSlots(t, "ranger 20", slots, []SpellSlot{
		{Level: 1, Total: 4}, {Level: 2, Total: 3}, {Level: 3, Total: 3},
		{Level: 4, Total: 3}, {Level: 5, Total: 2},
	})
}

// Third casters get nothing until the archetype is chosen at level 3, and the
// archetype is what grants the casting in the first place.
func TestThirdCasterSubclassSlots(t *testing.T) {
	champion := []ClassLevel{{Class: ClassFighter, Subclass: "champion", Level: 7}}
	if slots, _, _ := SpellSlotsForClasses(champion); len(slots) != 0 {
		t.Errorf("a champion fighter has %d slot levels, want none", len(slots))
	}

	// Eldritch Knight 3 is caster level 1.
	ek3 := []ClassLevel{{Class: ClassFighter, Subclass: "eldritch_knight", Level: 3}}
	slots, _, _ := SpellSlotsForClasses(ek3)
	assertSlots(t, "eldritch knight 3", slots, []SpellSlot{{Level: 1, Total: 2}})

	// Eldritch Knight 7 is caster level 3: 4 first and 2 second.
	ek7 := []ClassLevel{{Class: ClassFighter, Subclass: "eldritch_knight", Level: 7}}
	slots, _, _ = SpellSlotsForClasses(ek7)
	assertSlots(t, "eldritch knight 7", slots, []SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 2}})

	// Below level 3 the archetype is not yet chosen, so no casting.
	ek2 := []ClassLevel{{Class: ClassFighter, Subclass: "eldritch_knight", Level: 2}}
	if slots, _, _ := SpellSlotsForClasses(ek2); len(slots) != 0 {
		t.Errorf("eldritch knight 2 has %d slot levels, want none", len(slots))
	}
}

// Multiclass caster level rounds half and third casters *down*, unlike the
// single-class tables. Fighter 3 (EK) / Wizard 2 is caster level 3, not 4.
func TestMulticlassCasterLevelRoundsDown(t *testing.T) {
	classes := []ClassLevel{
		{Class: ClassFighter, Subclass: "eldritch_knight", Level: 3},
		{Class: ClassWizard, Level: 2},
	}

	slots, _, _ := SpellSlotsForClasses(classes)
	// wizard 2 + floor(3/3) = 3 -> 4 first, 2 second
	assertSlots(t, "EK 3 / wizard 2", slots, []SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 2}})

	// Paladin 3 / Sorcerer 3 is 3 + floor(3/2) = 4, not 3 + 2.
	slots, _, _ = SpellSlotsForClasses([]ClassLevel{
		{Class: ClassPaladin, Level: 3},
		{Class: ClassSorcerer, Level: 3},
	})
	assertSlots(t, "paladin 3 / sorcerer 3", slots, []SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 3}})
}

// Pact magic never merges into the combined caster level, and is reported on
// its own because those slots come back on a short rest.
func TestWarlockPactMagicStaysSeparate(t *testing.T) {
	slots, count, level := SpellSlotsForClasses([]ClassLevel{{Class: ClassWarlock, Level: 5}})
	if len(slots) != 0 {
		t.Errorf("warlock reported %d ordinary slot levels, want none", len(slots))
	}
	if count != 2 || level != 3 {
		t.Errorf("warlock 5 pact slots = %d x level %d, want 2 x 3", count, level)
	}

	// A warlock/sorcerer keeps two separate pools: the sorcerer half uses its
	// own single-class table, and the pact slots stand alone.
	slots, count, level = SpellSlotsForClasses([]ClassLevel{
		{Class: ClassWarlock, Level: 2},
		{Class: ClassSorcerer, Level: 3},
	})
	assertSlots(t, "warlock 2 / sorcerer 3", slots, []SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 2}})
	if count != 2 || level != 1 {
		t.Errorf("pact slots = %d x level %d, want 2 x 1", count, level)
	}

	// Warlock 20 caps at four 5th-level slots.
	_, count, level = SpellSlotsForClasses([]ClassLevel{{Class: ClassWarlock, Level: 20}})
	if count != 4 || level != 5 {
		t.Errorf("warlock 20 pact slots = %d x level %d, want 4 x 5", count, level)
	}
}

func TestTotalLevelAndProficiencyBonusAcrossClasses(t *testing.T) {
	c := &Character{BasicInfo: BasicInfo{Classes: []ClassLevel{
		{Class: ClassFighter, Subclass: "eldritch_knight", Level: 3},
		{Class: ClassWizard, Level: 2},
	}}}

	if got := c.Level(); got != 5 {
		t.Errorf("total level = %d, want 5", got)
	}
	// +3 for a 5th-level character, not the +2 of either half.
	if got := c.ProficiencyBonus(); got != 3 {
		t.Errorf("proficiency bonus = %d, want 3", got)
	}
	if got := c.BasicInfo.PrimaryClass(); got != ClassFighter {
		t.Errorf("primary class = %q, want fighter", got)
	}
	if got := c.BasicInfo.LevelIn(ClassWizard); got != 2 {
		t.Errorf("wizard levels = %d, want 2", got)
	}
	if !c.BasicInfo.IsMulticlassed() {
		t.Error("a character with two classes should report as multiclassed")
	}
}

// Only the first class grants saving throw proficiencies when multiclassing.
func TestOnlyFirstClassGrantsSaves(t *testing.T) {
	c := &Character{BasicInfo: BasicInfo{Classes: []ClassLevel{
		{Class: ClassFighter, Level: 3}, // STR, CON
		{Class: ClassWizard, Level: 2},  // INT, WIS -- not granted
	}}}

	saves := c.GrantedSaveProficiencies()
	if len(saves) != 2 || saves[0] != AbilityStrength || saves[1] != AbilityConstitution {
		t.Errorf("granted saves = %v, want the fighter's Strength and Constitution", saves)
	}
}

func TestHitDicePoolsPerClass(t *testing.T) {
	classes := []ClassLevel{
		{Class: ClassFighter, Level: 3}, // 3d10
		{Class: ClassWizard, Level: 2},  // 2d6
	}

	dice := HitDiceForClasses(classes, nil)
	if len(dice) != 2 {
		t.Fatalf("got %d pools, want 2", len(dice))
	}
	// Largest die first.
	if dice[0].Die != 10 || dice[0].Total != 3 {
		t.Errorf("first pool = %+v, want 3d10", dice[0])
	}
	if dice[1].Die != 6 || dice[1].Total != 2 {
		t.Errorf("second pool = %+v, want 2d6", dice[1])
	}
	if got := dice.TotalDice(); got != 5 {
		t.Errorf("total dice = %d, want 5 (the character level)", got)
	}
	if got, want := dice.String(), "3/3d10, 2/2d6"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}

	if err := dice.Spend(10); err != nil {
		t.Fatalf("spending a d10: %v", err)
	}
	if got := dice.Available(); got != 4 {
		t.Errorf("available after spending one = %d, want 4", got)
	}
	if err := dice.Spend(12); err == nil {
		t.Error("spending a d12 the character does not have should fail")
	}

	// Classes sharing a die size merge into one pool.
	merged := HitDiceForClasses([]ClassLevel{
		{Class: ClassCleric, Level: 2}, // d8
		{Class: ClassRogue, Level: 3},  // d8
	}, nil)
	if len(merged) != 1 || merged[0].Die != 8 || merged[0].Total != 5 {
		t.Errorf("merged pools = %v, want a single 5d8", merged)
	}
}

func TestHitDiceLongRestReturnsHalfRoundedDown(t *testing.T) {
	dice := HitDice{{Die: 10, Total: 3, Spent: 3}, {Die: 6, Total: 2, Spent: 2}}

	// 5 total dice, so half rounded down is 2, taken from the largest first.
	dice.RegainOnLongRest()
	if got := dice.Available(); got != 2 {
		t.Errorf("available after a long rest = %d, want 2", got)
	}
	if dice[0].Spent != 1 {
		t.Errorf("d10 pool spent = %d, want 1 (largest dice recovered first)", dice[0].Spent)
	}

	// A level 1 character regains at least one die rather than none.
	single := HitDice{{Die: 8, Total: 1, Spent: 1}}
	single.RegainOnLongRest()
	if single.Available() != 1 {
		t.Errorf("a 1st-level character regained %d dice, want 1", single.Available())
	}
}

func TestExpectedHitDicePreservesSpent(t *testing.T) {
	c := &Character{
		BasicInfo:   BasicInfo{Classes: []ClassLevel{{Class: ClassFighter, Level: 3}}},
		CombatStats: CombatStats{HitDice: HitDice{{Die: 10, Total: 2, Spent: 1}}},
	}

	// Levelling from fighter 2 to 3 adds a die without refunding the spent one.
	dice := c.ExpectedHitDice()
	if len(dice) != 1 || dice[0].Total != 3 || dice[0].Spent != 1 {
		t.Errorf("recomputed dice = %v, want 3d10 with one spent", dice)
	}
}

func TestSpellcastingClassPicksTheHighestCaster(t *testing.T) {
	c := &Character{
		BasicInfo: BasicInfo{Classes: []ClassLevel{
			{Class: ClassFighter, Level: 2},
			{Class: ClassWizard, Level: 5},
		}},
		AbilityScores: AbilityScores{Intelligence: 18},
	}

	cl, ok := c.SpellcastingClass()
	if !ok || cl.Class != ClassWizard {
		t.Fatalf("spellcasting class = %+v (ok=%v), want wizard", cl, ok)
	}

	// A character with no casting class reports none rather than guessing.
	mundane := &Character{BasicInfo: levels(ClassBarbarian, 5)}
	if _, ok := mundane.SpellcastingClass(); ok {
		t.Error("a barbarian should have no spellcasting class")
	}
}

func assertSlots(t *testing.T, label string, got, want []SpellSlot) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d slot levels %v, want %d %v", label, len(got), got, len(want), want)
	}
	for i := range want {
		if got[i].Level != want[i].Level || got[i].Total != want[i].Total {
			t.Errorf("%s: slot %d = level %d x%d, want level %d x%d",
				label, i, got[i].Level, got[i].Total, want[i].Level, want[i].Total)
		}
	}
}

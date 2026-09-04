package models

import (
	"strings"
	"testing"
)

func TestRaceTableIsConsistent(t *testing.T) {
	if len(Races) != 9 {
		t.Fatalf("got %d races, want the 9 PHB races", len(Races))
	}
	for _, r := range Races {
		def, ok := RaceTable[r]
		if !ok {
			t.Errorf("race %q has no definition", r)
			continue
		}
		if def.Speed < 20 || def.Speed > 40 {
			t.Errorf("%s speed is %d feet, which looks wrong", r, def.Speed)
		}
		if def.Size != SizeSmall && def.Size != SizeMedium {
			t.Errorf("%s size is %q, want small or medium", r, def.Size)
		}
		for _, b := range def.AbilityBonuses {
			if !b.Ability.Valid() {
				t.Errorf("%s grants a bonus to invalid ability %q", r, b.Ability)
			}
			if b.Bonus < 1 || b.Bonus > 2 {
				t.Errorf("%s grants %+d to %s, want +1 or +2", r, b.Bonus, b.Ability)
			}
		}
		for _, s := range def.SkillProficiencies {
			if !s.Valid() {
				t.Errorf("%s grants invalid skill %q", r, s)
			}
		}
	}
}

func TestKnownRacialTraits(t *testing.T) {
	// Dwarves are slow; wood elves are fast.
	if got := RaceDwarf.Speed("hill"); got != 25 {
		t.Errorf("hill dwarf speed = %d, want 25", got)
	}
	if got := RaceElf.Speed("wood"); got != 35 {
		t.Errorf("wood elf speed = %d, want 35 (subrace overrides the base 30)", got)
	}
	if got := RaceElf.Speed("high"); got != 30 {
		t.Errorf("high elf speed = %d, want the base 30", got)
	}

	// Drow see twice as far as other elves.
	if got := RaceElf.Darkvision("drow"); got != 120 {
		t.Errorf("drow darkvision = %d, want 120", got)
	}
	if got := RaceElf.Darkvision("high"); got != 60 {
		t.Errorf("high elf darkvision = %d, want 60", got)
	}
	if got := RaceHuman.Darkvision(""); got != 0 {
		t.Errorf("human darkvision = %d, want 0", got)
	}

	// Elves get Perception; half-orcs get Intimidation.
	if got := RaceElf.GrantedSkills("high"); len(got) != 1 || got[0] != SkillPerception {
		t.Errorf("elf skills = %v, want Perception", got)
	}
	if got := RaceHalfOrc.GrantedSkills(""); len(got) != 1 || got[0] != SkillIntimidation {
		t.Errorf("half-orc skills = %v, want Intimidation", got)
	}
}

func TestApplyRacialBonuses(t *testing.T) {
	base := AbilityScores{Strength: 15, Dexterity: 13, Constitution: 14,
		Intelligence: 12, Wisdom: 10, Charisma: 8}

	// Mountain dwarf: +2 CON from the race, +2 STR from the subrace.
	got := ApplyRacialBonuses(base, RaceDwarf, "mountain", nil)
	if got.Strength != 17 || got.Constitution != 16 {
		t.Errorf("mountain dwarf = STR %d CON %d, want 17 and 16", got.Strength, got.Constitution)
	}
	if got.Dexterity != 13 {
		t.Errorf("untouched ability changed to %d, want 13", got.Dexterity)
	}

	// Half-elf: +2 CHA fixed, plus +1 to two of the player's choosing.
	got = ApplyRacialBonuses(base, RaceHalfElf, "", []Ability{AbilityDexterity, AbilityWisdom})
	if got.Charisma != 10 || got.Dexterity != 14 || got.Wisdom != 11 {
		t.Errorf("half-elf = CHA %d DEX %d WIS %d, want 10, 14 and 11",
			got.Charisma, got.Dexterity, got.Wisdom)
	}

	// A third choice is ignored: half-elves get exactly two.
	got = ApplyRacialBonuses(base, RaceHalfElf, "",
		[]Ability{AbilityDexterity, AbilityWisdom, AbilityStrength})
	if got.Strength != 15 {
		t.Errorf("a third ability choice was applied: STR %d, want 15", got.Strength)
	}

	// Humans raise everything by one.
	got = ApplyRacialBonuses(base, RaceHuman, "", nil)
	if got.Strength != 16 || got.Charisma != 9 {
		t.Errorf("human = STR %d CHA %d, want 16 and 9", got.Strength, got.Charisma)
	}
}

// Every background grants exactly two fixed skills.
func TestBackgroundTableIsConsistent(t *testing.T) {
	if len(Backgrounds) != 13 {
		t.Fatalf("got %d backgrounds, want 13", len(Backgrounds))
	}
	for _, b := range Backgrounds {
		def, ok := BackgroundTable[b]
		if !ok {
			t.Errorf("background %q has no definition", b)
			continue
		}
		if len(def.SkillProficiencies) != 2 {
			t.Errorf("%s grants %d skills, want exactly 2", b, len(def.SkillProficiencies))
		}
		for _, s := range def.SkillProficiencies {
			if !s.Valid() {
				t.Errorf("%s grants invalid skill %q", b, s)
			}
		}
		if def.Feature == "" {
			t.Errorf("%s has no feature", b)
		}
	}

	if got := BackgroundSoldier.GrantedSkills(); got[0] != SkillAthletics || got[1] != SkillIntimidation {
		t.Errorf("soldier skills = %v, want Athletics and Intimidation", got)
	}
}

func newValidCharacter() *Character {
	return &Character{
		Name: "Thistle",
		Type: CharacterPlayer,
		BasicInfo: BasicInfo{
			Race:       RaceHalfling,
			Subrace:    "lightfoot",
			Background: BackgroundCriminal,
			Classes:    []ClassLevel{{Class: ClassRogue, Subclass: "thief", Level: 3}},
		},
		AbilityScores: AbilityScores{Strength: 10, Dexterity: 17, Constitution: 14,
			Intelligence: 12, Wisdom: 13, Charisma: 11},
		Skills: SkillProficiencies{
			SkillDeception:     ProficiencyProficient, // criminal background
			SkillStealth:       ProficiencyExpertise,  // rogue list
			SkillAcrobatics:    ProficiencyProficient, // rogue list
			SkillInvestigation: ProficiencyProficient, // rogue list
		},
	}
}

func TestValidateSheetAcceptsALegalCharacter(t *testing.T) {
	if err := newValidCharacter().ValidateSheet(); err != nil {
		t.Fatalf("a legal character was rejected: %v", err)
	}
}

func TestValidateSheetRejectsBadSubraceAndSubclass(t *testing.T) {
	c := newValidCharacter()
	c.BasicInfo.Subrace = "mountain" // a dwarf subrace
	c.BasicInfo.Classes[0].Subclass = "champion"

	err := c.ValidateSheet()
	if err == nil {
		t.Fatal("expected an error for a mismatched subrace and subclass")
	}
	if !strings.Contains(err.Error(), "subrace") {
		t.Errorf("error %q does not mention the subrace", err)
	}
	if !strings.Contains(err.Error(), "champion") {
		t.Errorf("error %q does not mention the wrong archetype", err)
	}
}

// An archetype is chosen at a specific level; before it, having one is wrong.
func TestValidateSheetChecksSubclassTiming(t *testing.T) {
	c := newValidCharacter()
	c.BasicInfo.Classes[0].Level = 2
	c.BasicInfo.Classes[0].Subclass = "thief"

	if err := c.ValidateSheet(); err == nil {
		t.Error("a rogue 2 with an archetype should be rejected; rogues choose at 3")
	}

	// And at the subclass level, missing one is equally wrong.
	c.BasicInfo.Classes[0].Level = 3
	c.BasicInfo.Classes[0].Subclass = ""
	if err := c.ValidateSheet(); err == nil {
		t.Error("a rogue 3 without an archetype should be rejected")
	}
}

func TestValidateSheetEnforcesMulticlassPrerequisites(t *testing.T) {
	c := newValidCharacter()
	// Dexterity 17 satisfies rogue, but Wisdom 13 alone does not meet
	// paladin's Strength 13 and Charisma 13.
	c.BasicInfo.Classes = append(c.BasicInfo.Classes,
		ClassLevel{Class: ClassPaladin, Level: 2})

	err := c.ValidateSheet()
	if err == nil {
		t.Fatal("multiclassing into paladin without the ability scores should fail")
	}
	if !strings.Contains(err.Error(), "Charisma 13") {
		t.Errorf("error %q does not explain the requirement", err)
	}

	// Raising the scores makes it legal.
	c.AbilityScores.Strength = 13
	c.AbilityScores.Charisma = 13
	if err := c.ValidateSheet(); err != nil {
		t.Errorf("a legal multiclass was rejected: %v", err)
	}
}

func TestValidateSheetRejectsUngrantedSkill(t *testing.T) {
	c := newValidCharacter()
	// Nature is on neither the rogue list, the criminal background nor the
	// halfling's racial grants.
	c.Skills[SkillNature] = ProficiencyProficient

	err := c.ValidateSheet()
	if err == nil {
		t.Fatal("a skill from no available source should be rejected")
	}
	if !strings.Contains(err.Error(), "nature") {
		t.Errorf("error %q does not name the offending skill", err)
	}
}

// The bard chooses any three skills, so there is no list to check against.
func TestValidateSheetSkipsSkillCheckForUnconstrainedClasses(t *testing.T) {
	c := &Character{
		BasicInfo: BasicInfo{
			Race: RaceHuman, Background: BackgroundEntertainer,
			Classes: []ClassLevel{{Class: ClassBard, Subclass: "lore", Level: 3}},
		},
		AbilityScores: AbilityScores{Charisma: 16},
		Skills: SkillProficiencies{
			SkillNature:   ProficiencyProficient,
			SkillMedicine: ProficiencyProficient,
		},
	}

	if err := c.ValidateSheet(); err != nil {
		t.Errorf("a bard should be able to take any skill: %v", err)
	}
}

func TestValidateSheetRejectsOverTwentyTotalLevels(t *testing.T) {
	c := newValidCharacter()
	c.AbilityScores.Strength = 15
	c.BasicInfo.Classes = []ClassLevel{
		{Class: ClassRogue, Subclass: "thief", Level: 15},
		{Class: ClassFighter, Subclass: "champion", Level: 10},
	}

	err := c.ValidateSheet()
	if err == nil || !strings.Contains(err.Error(), "total level") {
		t.Errorf("25 total levels should be rejected, got %v", err)
	}
}

func TestApplyClassDefaultsFillsDerivedChoices(t *testing.T) {
	c := &Character{
		BasicInfo: BasicInfo{
			Race: RaceElf, Subrace: "high",
			Background: BackgroundSage,
			Classes:    []ClassLevel{{Class: ClassWizard, Subclass: "evocation", Level: 5}},
		},
		AbilityScores: AbilityScores{Intelligence: 16, Dexterity: 14},
	}

	c.ApplyClassDefaults()

	// Wizard saves are Intelligence and Wisdom.
	if c.SavingThrows.Level(AbilityIntelligence) != ProficiencyProficient ||
		c.SavingThrows.Level(AbilityWisdom) != ProficiencyProficient {
		t.Errorf("saving throws = %v, want Intelligence and Wisdom", c.SavingThrows)
	}
	if c.SavingThrows.Level(AbilityStrength) != ProficiencyNone {
		t.Error("wizard should not be proficient in Strength saves")
	}

	// Sage grants Arcana and History; high elf grants Perception.
	for _, s := range []Skill{SkillArcana, SkillHistory, SkillPerception} {
		if c.Skills.Level(s) != ProficiencyProficient {
			t.Errorf("%s should be granted by race or background", s)
		}
	}

	// 5d6 of hit dice.
	if len(c.CombatStats.HitDice) != 1 || c.CombatStats.HitDice[0].Die != 6 ||
		c.CombatStats.HitDice[0].Total != 5 {
		t.Errorf("hit dice = %v, want 5d6", c.CombatStats.HitDice)
	}

	// Wizard 5: 4 first, 3 second, 2 third; spellcasting off Intelligence.
	if c.Spells.SpellcastingAbility != AbilityIntelligence {
		t.Errorf("spellcasting ability = %q, want intelligence", c.Spells.SpellcastingAbility)
	}
	assertSlots(t, "wizard 5", c.Spells.Slots,
		[]SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 3}, {Level: 3, Total: 2}})

	// Spell save DC 8 + PB 3 + INT 3 = 14.
	if got := c.SpellSaveDC(); got != 14 {
		t.Errorf("SpellSaveDC = %d, want 14", got)
	}
}

// Levelling up adds slots but never refunds ones already spent.
func TestReconcileSpellSlotsPreservesExpended(t *testing.T) {
	c := &Character{
		BasicInfo: BasicInfo{Classes: []ClassLevel{{Class: ClassWizard, Level: 3}}},
		Spells: Spells{Slots: []SpellSlot{
			{Level: 1, Total: 4, Expended: 3},
			{Level: 2, Total: 2, Expended: 1},
		}},
	}

	c.BasicInfo.Classes[0].Level = 4 // wizard 4: 4 first, 3 second
	c.ReconcileSpellSlots()

	if len(c.Spells.Slots) != 2 {
		t.Fatalf("slots = %v, want two levels", c.Spells.Slots)
	}
	if c.Spells.Slots[0].Expended != 3 {
		t.Errorf("first-level expended = %d, want the 3 already spent", c.Spells.Slots[0].Expended)
	}
	if c.Spells.Slots[1].Total != 3 || c.Spells.Slots[1].Expended != 1 {
		t.Errorf("second-level slots = %+v, want 3 total with 1 expended", c.Spells.Slots[1])
	}
}

func TestLongRestRestoresResources(t *testing.T) {
	c := &Character{
		BasicInfo: BasicInfo{Classes: []ClassLevel{{Class: ClassCleric, Level: 4}}},
		CombatStats: CombatStats{
			HitPoints:  HitPoints{Current: 3, Maximum: 28, Temporary: 5},
			HitDice:    HitDice{{Die: 8, Total: 4, Spent: 4}},
			DeathSaves: DeathSaves{Failures: 2},
		},
		Exhaustion: 2,
		Spells:     Spells{Slots: []SpellSlot{{Level: 1, Total: 4, Expended: 4}}},
	}

	c.LongRest()

	if c.CombatStats.HitPoints.Current != 28 {
		t.Errorf("hit points = %d, want the full 28", c.CombatStats.HitPoints.Current)
	}
	if c.CombatStats.HitPoints.Temporary != 0 {
		t.Errorf("temporary hit points = %d, want 0 (they do not survive a rest)",
			c.CombatStats.HitPoints.Temporary)
	}
	if c.CombatStats.DeathSaves != (DeathSaves{}) {
		t.Errorf("death saves = %+v, want cleared", c.CombatStats.DeathSaves)
	}
	if got := c.CombatStats.HitDice.Available(); got != 2 {
		t.Errorf("hit dice available = %d, want 2 (half of 4)", got)
	}
	if got := c.Spells.AvailableSlots(1); got != 4 {
		t.Errorf("spell slots = %d, want all 4 back", got)
	}
	if c.Exhaustion != 1 {
		t.Errorf("exhaustion = %d, want 1 (a long rest reduces it by one)", c.Exhaustion)
	}
}

package models

import (
	"strings"
	"testing"
)

// The Champion's whole archetype is a wider critical range, and hardcoding a
// natural 20 in ResolveAttack silently denied it.
func TestChampionCriticalRange(t *testing.T) {
	cases := []struct {
		level int
		want  int
	}{
		{2, NaturalCrit}, // archetype not chosen yet
		{3, 19},          // Improved Critical
		{14, 19},
		{15, 18}, // Superior Critical
		{20, 18},
	}

	for _, tc := range cases {
		cl := ClassLevel{Class: ClassFighter, Subclass: "champion", Level: tc.level}
		if got := cl.CritRange(); got != tc.want {
			t.Errorf("champion %d crit range = %d, want %d", tc.level, got, tc.want)
		}
	}

	// Other archetypes are unaffected.
	battleMaster := ClassLevel{Class: ClassFighter, Subclass: "battle_master", Level: 15}
	if got := battleMaster.CritRange(); got != NaturalCrit {
		t.Errorf("battle master crit range = %d, want %d", got, NaturalCrit)
	}
}

func TestResolveAttackHonoursCritRange(t *testing.T) {
	roll19 := D20Result{Natural: 19, Modifier: 2, Total: 21}

	// An ordinary character hits with a 19 but does not crit.
	if got := ResolveAttack(roll19, 15, NaturalCrit); got != AttackHit {
		t.Errorf("natural 19 for an ordinary attacker = %s, want hit", got)
	}
	// A Champion crits on it.
	if got := ResolveAttack(roll19, 15, 19); got != AttackCritical {
		t.Errorf("natural 19 with a 19-20 range = %s, want critical_hit", got)
	}
	// And a critical lands regardless of AC.
	if got := ResolveAttack(D20Result{Natural: 19, Total: 21}, 40, 19); got != AttackCritical {
		t.Errorf("a critical against AC 40 = %s, want critical_hit", got)
	}
	// A natural 1 is still a miss, whatever the range.
	if got := ResolveAttack(D20Result{Natural: 1, Total: 25}, 5, 18); got != AttackFumble {
		t.Errorf("natural 1 = %s, want critical_miss", got)
	}
}

func TestCharacterCritRangeTakesTheBestClass(t *testing.T) {
	c := &Character{BasicInfo: BasicInfo{Classes: []ClassLevel{
		{Class: ClassRogue, Subclass: "thief", Level: 5},
		{Class: ClassFighter, Subclass: "champion", Level: 3},
	}}}

	if got := c.CritRange(); got != 19 {
		t.Errorf("crit range = %d, want 19 from the champion levels", got)
	}

	profile, err := c.AttackWith(longsword())
	if err != nil {
		t.Fatalf("AttackWith: %v", err)
	}
	if profile.CritRange != 19 {
		t.Errorf("attack profile crit range = %d, want 19", profile.CritRange)
	}
}

func TestSubclassLookup(t *testing.T) {
	sub, ok := ClassFighter.Subclass("eldritch_knight")
	if !ok {
		t.Fatal("eldritch knight should be a fighter archetype")
	}
	if sub.Name != "Eldritch Knight" {
		t.Errorf("name = %q, want Eldritch Knight", sub.Name)
	}
	if sub.Source != SourcePHB {
		t.Errorf("source = %q, want %q", sub.Source, SourcePHB)
	}
	if sub.Casting == nil || sub.Casting.Progression != CasterThird {
		t.Errorf("casting = %+v, want a third-caster entry", sub.Casting)
	}

	if _, ok := ClassFighter.Subclass("thief"); ok {
		t.Error("thief is a rogue archetype")
	}

	keys := ClassWizard.SubclassKeys()
	if len(keys) != 8 {
		t.Errorf("wizard has %d schools, want 8", len(keys))
	}
}

// Every archetype needs a key, a name and a source, or the table is unusable
// for display and validation alike.
func TestEverySubclassIsWellFormed(t *testing.T) {
	for _, c := range Classes {
		def := ClassTable[c]
		seen := map[string]bool{}
		for _, sub := range def.Subclasses {
			if sub.Key == "" || sub.Name == "" {
				t.Errorf("%s has an archetype with a missing key or name: %+v", c, sub)
			}
			if sub.Source == "" {
				t.Errorf("%s/%s has no source", c, sub.Key)
			}
			if seen[sub.Key] {
				t.Errorf("%s lists %q twice", c, sub.Key)
			}
			seen[sub.Key] = true
			if strings.ContainsAny(sub.Key, " ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				t.Errorf("%s/%s key should be lower_snake_case", c, sub.Key)
			}
		}
	}
}

// Only bard, ranger and rogue grant an extra skill when multiclassed into.
func TestMulticlassSkillGrant(t *testing.T) {
	for _, c := range []Class{ClassBard, ClassRanger, ClassRogue} {
		if got := ClassTable[c].MulticlassSkillChoices; got != 1 {
			t.Errorf("%s multiclass skill choices = %d, want 1", c, got)
		}
	}
	for _, c := range []Class{ClassFighter, ClassWizard, ClassCleric, ClassBarbarian} {
		if got := ClassTable[c].MulticlassSkillChoices; got != 0 {
			t.Errorf("%s multiclass skill choices = %d, want 0", c, got)
		}
	}
}

func TestSkillBudget(t *testing.T) {
	// Halfling (no granted skills) + criminal background (2) + rogue (4).
	rogue := newValidCharacter()
	if got := rogue.SkillBudget(); got != 6 {
		t.Errorf("rogue budget = %d, want 6", got)
	}

	// Multiclassing into fighter adds nothing; into bard it adds one.
	rogue.BasicInfo.Classes = append(rogue.BasicInfo.Classes,
		ClassLevel{Class: ClassFighter, Subclass: "champion", Level: 3})
	if got := rogue.SkillBudget(); got != 6 {
		t.Errorf("budget after a fighter multiclass = %d, want 6", got)
	}

	rogue.BasicInfo.Classes[1] = ClassLevel{Class: ClassBard, Subclass: "lore", Level: 3}
	if got := rogue.SkillBudget(); got != 7 {
		t.Errorf("budget after a bard multiclass = %d, want 7", got)
	}

	// Half-elf: 2 free racial picks on top.
	halfElf := &Character{BasicInfo: BasicInfo{
		Race: RaceHalfElf, Background: BackgroundSage,
		Classes: []ClassLevel{{Class: ClassWizard, Subclass: "evocation", Level: 2}},
	}}
	// sage 2 granted + wizard 2 chosen + half-elf 2 chosen
	if got := halfElf.SkillBudget(); got != 6 {
		t.Errorf("half-elf wizard budget = %d, want 6", got)
	}
}

// Checking only that each skill has a legal source let a rogue claim every
// skill on their list.
func TestValidateSheetEnforcesSkillCount(t *testing.T) {
	c := newValidCharacter()
	for _, s := range []Skill{SkillPerception, SkillPerformance, SkillPersuasion, SkillIntimidation} {
		c.Skills[s] = ProficiencyProficient
	}

	err := c.ValidateSheet()
	if err == nil {
		t.Fatal("a rogue with 8 skill proficiencies should be rejected")
	}
	if !strings.Contains(err.Error(), "skill proficiencies") {
		t.Errorf("error %q does not explain the skill budget", err)
	}
}

func TestExpertiseBudget(t *testing.T) {
	cases := []struct {
		classes []ClassLevel
		want    int
	}{
		{[]ClassLevel{{Class: ClassRogue, Level: 1}}, 2},
		{[]ClassLevel{{Class: ClassRogue, Level: 5}}, 2},
		{[]ClassLevel{{Class: ClassRogue, Level: 6}}, 4},
		{[]ClassLevel{{Class: ClassBard, Level: 2}}, 0},
		{[]ClassLevel{{Class: ClassBard, Level: 3}}, 2},
		{[]ClassLevel{{Class: ClassBard, Level: 10}}, 4},
		{[]ClassLevel{{Class: ClassFighter, Level: 20}}, 0},
		// Both classes contribute.
		{[]ClassLevel{{Class: ClassRogue, Level: 6}, {Class: ClassBard, Level: 3}}, 6},
	}

	for _, tc := range cases {
		c := &Character{BasicInfo: BasicInfo{Classes: tc.classes}}
		if got := c.ExpertiseBudget(); got != tc.want {
			t.Errorf("%v expertise budget = %d, want %d", tc.classes, got, tc.want)
		}
	}
}

func TestValidateSheetEnforcesExpertiseBudget(t *testing.T) {
	c := newValidCharacter() // rogue 3: two expertise choices
	c.Skills[SkillAcrobatics] = ProficiencyExpertise
	c.Skills[SkillInvestigation] = ProficiencyExpertise
	// Stealth is already expertise, making three.

	err := c.ValidateSheet()
	if err == nil {
		t.Fatal("a rogue 3 with three expertise skills should be rejected")
	}
	if !strings.Contains(err.Error(), "expertise") {
		t.Errorf("error %q does not explain the expertise budget", err)
	}
}

func TestCantripsAndSpellsKnown(t *testing.T) {
	cases := []struct {
		cl               ClassLevel
		cantrips, spells int
	}{
		{ClassLevel{Class: ClassWizard, Level: 1}, 3, 0}, // wizard prepares
		{ClassLevel{Class: ClassWizard, Level: 10}, 5, 0},
		{ClassLevel{Class: ClassSorcerer, Level: 1}, 4, 2},
		{ClassLevel{Class: ClassSorcerer, Level: 20}, 6, 15},
		{ClassLevel{Class: ClassBard, Level: 1}, 2, 4},
		{ClassLevel{Class: ClassBard, Level: 20}, 4, 22},
		{ClassLevel{Class: ClassWarlock, Level: 5}, 3, 6},
		{ClassLevel{Class: ClassRanger, Level: 1}, 0, 0}, // no casting until 2
		{ClassLevel{Class: ClassRanger, Level: 2}, 0, 2},
		{ClassLevel{Class: ClassBarbarian, Level: 20}, 0, 0},
	}

	for _, tc := range cases {
		if got := tc.cl.CantripsKnown(); got != tc.cantrips {
			t.Errorf("%s %d cantrips = %d, want %d", tc.cl.Class, tc.cl.Level, got, tc.cantrips)
		}
		if got := tc.cl.SpellsKnown(); got != tc.spells {
			t.Errorf("%s %d spells known = %d, want %d", tc.cl.Class, tc.cl.Level, got, tc.spells)
		}
	}

	// Third casters take their counts from the archetype.
	ek := ClassLevel{Class: ClassFighter, Subclass: "eldritch_knight", Level: 3}
	if got := ek.CantripsKnown(); got != 2 {
		t.Errorf("eldritch knight 3 cantrips = %d, want 2", got)
	}
	champion := ClassLevel{Class: ClassFighter, Subclass: "champion", Level: 10}
	if got := champion.CantripsKnown(); got != 0 {
		t.Errorf("champion cantrips = %d, want 0", got)
	}
}

func TestPreparedSpellLimit(t *testing.T) {
	cleric := &Character{
		BasicInfo:     BasicInfo{Classes: []ClassLevel{{Class: ClassCleric, Subclass: "life", Level: 5}}},
		AbilityScores: AbilityScores{Wisdom: 16}, // +3
	}
	limit, prepares := cleric.PreparedSpellLimit(ClassCleric)
	if !prepares || limit != 8 {
		t.Errorf("cleric 5 with WIS 16 prepares %d (ok=%v), want 8", limit, prepares)
	}

	// Paladins use half their level.
	paladin := &Character{
		BasicInfo:     BasicInfo{Classes: []ClassLevel{{Class: ClassPaladin, Subclass: "devotion", Level: 6}}},
		AbilityScores: AbilityScores{Charisma: 14}, // +2
	}
	limit, _ = paladin.PreparedSpellLimit(ClassPaladin)
	if limit != 5 {
		t.Errorf("paladin 6 with CHA 14 prepares %d, want 5", limit)
	}

	// Sorcerers know their spells; they do not prepare.
	sorcerer := &Character{
		BasicInfo: BasicInfo{Classes: []ClassLevel{{Class: ClassSorcerer, Subclass: "wild_magic", Level: 5}}},
	}
	if _, prepares := sorcerer.PreparedSpellLimit(ClassSorcerer); prepares {
		t.Error("sorcerers do not prepare spells")
	}

	// The limit never drops below one, however poor the ability.
	weak := &Character{
		BasicInfo:     BasicInfo{Classes: []ClassLevel{{Class: ClassPaladin, Level: 2}}},
		AbilityScores: AbilityScores{Charisma: 6}, // -2
	}
	limit, _ = weak.PreparedSpellLimit(ClassPaladin)
	if limit != 1 {
		t.Errorf("a level 2 paladin with CHA 6 prepares %d, want at least 1", limit)
	}
}

func TestClassFeaturesIncludeASIs(t *testing.T) {
	// Every class has features at level 1.
	for _, c := range Classes {
		if len(FeaturesAtLevel(c, 1)) == 0 {
			t.Errorf("%s has no level 1 features", c)
		}
	}

	// ASI levels differ, and are merged in rather than duplicated.
	if got := FeaturesAtLevel(ClassFighter, 6); !contains(got, "Ability Score Improvement") {
		t.Errorf("fighter level 6 = %v, want an ASI", got)
	}
	if got := FeaturesAtLevel(ClassWizard, 6); contains(got, "Ability Score Improvement") {
		t.Errorf("wizard level 6 = %v, want no ASI", got)
	}
	if got := FeaturesAtLevel(ClassRogue, 10); !contains(got, "Ability Score Improvement") {
		t.Errorf("rogue level 10 = %v, want an ASI", got)
	}

	// No duplicates where the table already names the ASI.
	fourth := FeaturesAtLevel(ClassFighter, 4)
	count := 0
	for _, f := range fourth {
		if f == "Ability Score Improvement" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("fighter level 4 lists the ASI %d times, want once", count)
	}

	// Cumulative view for the sheet's Features box.
	through5 := FeaturesThroughLevel(ClassFighter, 5)
	for _, want := range []string{"Second Wind", "Action Surge (one use)", "Martial Archetype", "Extra Attack"} {
		if !contains(through5, want) {
			t.Errorf("fighter through level 5 is missing %q: %v", want, through5)
		}
	}
}

func TestStartingEquipmentCoversEveryClass(t *testing.T) {
	for _, c := range Classes {
		eq, ok := StartingEquipmentFor(c)
		if !ok {
			t.Errorf("%s has no starting equipment", c)
			continue
		}
		if len(eq.Choices) == 0 && len(eq.Fixed) == 0 {
			t.Errorf("%s starting equipment is empty", c)
		}
		for _, ch := range eq.Choices {
			if ch.Prompt == "" {
				t.Errorf("%s has an unlabelled equipment choice", c)
			}
			if len(ch.Options) < 2 {
				t.Errorf("%s choice %q offers %d options, want at least 2", c, ch.Prompt, len(ch.Options))
			}
			for _, o := range ch.Options {
				if o.Label == "" || len(o.Items) == 0 {
					t.Errorf("%s choice %q has an empty option: %+v", c, ch.Prompt, o)
				}
			}
		}
	}

	// A spot check that the data is the book's, not a placeholder.
	wizard, _ := StartingEquipmentFor(ClassWizard)
	if !contains(wizard.Fixed, "spellbook") {
		t.Errorf("wizards should start with a spellbook, got %v", wizard.Fixed)
	}
}

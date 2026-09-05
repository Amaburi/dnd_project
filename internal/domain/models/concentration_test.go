package models

import (
	"strings"
	"testing"
)

func concentrator() *Character {
	c := &Character{
		Name: "Alaric", Type: CharacterPlayer,
		BasicInfo: BasicInfo{
			Race: RaceHuman, Subrace: "standard", Background: BackgroundSage,
			Classes: []ClassLevel{{Class: ClassWizard, Subclass: "evocation", Level: 5}},
		},
		AbilityScores: AbilityScores{
			Strength: 8, Dexterity: 14, Constitution: 14,
			Intelligence: 18, Wisdom: 12, Charisma: 10,
		},
	}
	c.ApplyClassDefaults()
	c.CombatStats.HitPoints = HitPoints{Current: 30, Maximum: 30}
	return c
}

// The DC is ten, or half the damage if that is worse. Getting this wrong makes
// concentration either unbreakable or impossible to hold.
func TestConcentrationDC(t *testing.T) {
	cases := map[int]int{
		0: 10, 1: 10, 9: 10, 19: 10, 20: 10, 21: 10,
		22: 11, 30: 15, 45: 22, 100: 50,
	}
	for damage, want := range cases {
		if got := ConcentrationDC(damage); got != want {
			t.Errorf("ConcentrationDC(%d) = %d, want %d", damage, got, want)
		}
	}
}

// One spell at a time. Without this a caster stacks every buff they own and
// never pays for any of them.
func TestASecondConcentrationSpellEndsTheFirst(t *testing.T) {
	c := concentrator()
	if c.IsConcentrating() {
		t.Fatal("a fresh character is already concentrating")
	}

	if replaced := c.BeginConcentration(Concentration{Spell: "Hold Person"}); replaced != "" {
		t.Errorf("the first spell replaced %q", replaced)
	}
	if !c.IsConcentrating() {
		t.Fatal("concentration did not start")
	}

	replaced := c.BeginConcentration(Concentration{Spell: "Web"})
	if replaced != "Hold Person" {
		t.Errorf("replaced %q, want Hold Person", replaced)
	}
	if c.Spells.Concentrating.Spell != "Web" {
		t.Errorf("concentrating on %q, want Web", c.Spells.Concentrating.Spell)
	}

	c.EndConcentration()
	if c.IsConcentrating() {
		t.Error("concentration survived being ended")
	}
	// Ending it twice is a no-op, not a panic.
	c.EndConcentration()
}

// Being knocked out ends concentration: you cannot hold a spell together while
// unconscious.
func TestLosingConsciousnessEndsConcentration(t *testing.T) {
	c := concentrator()
	c.BeginConcentration(Concentration{Spell: "Hold Person"})

	c.CombatStats.HitPoints.Current = 0
	if kept, reason := c.KeepsConcentration(); kept {
		t.Error("an unconscious caster held concentration")
	} else if !strings.Contains(strings.ToLower(reason), "unconscious") {
		t.Errorf("reason = %q, want it to name unconsciousness", reason)
	}

	incapacitated := concentrator()
	incapacitated.BeginConcentration(Concentration{Spell: "Hold Person"})
	incapacitated.Conditions = []Condition{ConditionStunned}
	if kept, _ := incapacitated.KeepsConcentration(); kept {
		t.Error("a stunned caster held concentration")
	}

	healthy := concentrator()
	healthy.BeginConcentration(Concentration{Spell: "Hold Person"})
	if kept, reason := healthy.KeepsConcentration(); !kept {
		t.Errorf("a healthy caster lost concentration: %q", reason)
	}
}

// The spell records what it did, so ending it can undo it. Without the target
// list a broken Hold Person leaves its victim paralysed for ever.
func TestConcentrationRemembersWhatItImposed(t *testing.T) {
	c := concentrator()
	c.BeginConcentration(Concentration{
		Spell: "Hold Person", SlotLevel: 2,
		Condition: ConditionParalyzed, Targets: []string{"cb-goblin"},
	})

	held := c.Spells.Concentrating
	if held.Condition != ConditionParalyzed {
		t.Errorf("condition = %q, want paralyzed", held.Condition)
	}
	if len(held.Targets) != 1 || held.Targets[0] != "cb-goblin" {
		t.Errorf("targets = %v, want the goblin", held.Targets)
	}
}

// A spell definition says whether it needs concentration, and the model has to
// agree with the catalogue.
func TestTheCatalogueMarksConcentrationSpells(t *testing.T) {
	for _, name := range []string{"Hold Person", "Web", "Moonbeam", "Hunter's Mark"} {
		def, ok := SpellByName(name)
		if !ok {
			t.Fatalf("%s is not in the table", name)
		}
		if !def.Concentration {
			t.Errorf("%s should require concentration", name)
		}
	}
	for _, name := range []string{"Magic Missile", "Fireball", "Cure Wounds"} {
		def, _ := SpellByName(name)
		if def.Concentration {
			t.Errorf("%s should not require concentration", name)
		}
	}
}

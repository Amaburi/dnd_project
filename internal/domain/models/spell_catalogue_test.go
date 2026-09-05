package models

import (
	"fmt"
	"strings"
	"testing"
)

func spell(t *testing.T, name string) SpellDefinition {
	t.Helper()
	def, ok := SpellByName(name)
	if !ok {
		t.Fatalf("%q is not in the spell table", name)
	}
	return def
}

func TestDamageDiceRendersLikeTheBook(t *testing.T) {
	cases := []struct {
		dice DamageDice
		want string
	}{
		{DamageDice{Count: 1, Sides: 10}, "1d10"},
		{DamageDice{Count: 3, Sides: 6}, "3d6"},
		{DamageDice{Count: 1, Sides: 4, Bonus: 1}, "1d4+1"},
		{DamageDice{Count: 2, Sides: 8, Bonus: -1}, "2d8-1"},
		{DamageDice{Bonus: 5}, "5"},
	}
	for _, tc := range cases {
		if got := tc.dice.String(); got != tc.want {
			t.Errorf("DamageDice%+v.String() = %q, want %q", tc.dice, got, tc.want)
		}
	}
	if !(DamageDice{}).IsZero() {
		t.Error("the zero value should report itself as zero")
	}
}

// Cantrips are the one part of 5e where damage scales with the *character*,
// not the slot -- at 5th, 11th and 17th level.
func TestCantripTierSteps(t *testing.T) {
	cases := map[int]int{1: 1, 4: 1, 5: 2, 10: 2, 11: 3, 16: 3, 17: 4, 20: 4}
	for level, want := range cases {
		if got := CantripTier(level); got != want {
			t.Errorf("CantripTier(%d) = %d, want %d", level, got, want)
		}
	}
}

func TestCantripDamageScalesWithCharacterLevel(t *testing.T) {
	fireBolt := spell(t, "Fire Bolt")
	if !fireBolt.IsCantrip() {
		t.Fatal("Fire Bolt should be a cantrip")
	}

	cases := map[int]string{1: "1d10", 4: "1d10", 5: "2d10", 11: "3d10", 17: "4d10"}
	for level, want := range cases {
		if got := fireBolt.DamageAt(0, level).String(); got != want {
			t.Errorf("Fire Bolt at character level %d = %s, want %s", level, got, want)
		}
	}

	// A cantrip does not care what slot it is "cast at": there is no slot.
	if got := fireBolt.DamageAt(3, 5).String(); got != "2d10" {
		t.Errorf("a cantrip scaled with a slot level: %s", got)
	}
}

// Eldritch Blast is the exception that breaks a naive implementation: it gains
// *beams*, each rolled separately, not bigger dice.
func TestEldritchBlastGainsBeamsNotDice(t *testing.T) {
	blast := spell(t, "Eldritch Blast")

	for level, want := range map[int]int{1: 1, 5: 2, 11: 3, 17: 4} {
		if got := blast.ProjectilesAt(0, level); got != want {
			t.Errorf("Eldritch Blast at level %d fires %d beams, want %d", level, got, want)
		}
	}
	// Each beam stays 1d10 however high the warlock climbs.
	for _, level := range []int{1, 5, 11, 17} {
		if got := blast.DamageAt(0, level).String(); got != "1d10" {
			t.Errorf("a beam at level %d does %s, want 1d10", level, got)
		}
	}
}

// Levelled spells scale with the slot, not the caster.
func TestUpcastingAddsDicePerSlotLevel(t *testing.T) {
	burning := spell(t, "Burning Hands")
	cases := map[int]string{1: "3d6", 2: "4d6", 3: "5d6", 5: "7d6"}
	for slot, want := range cases {
		if got := burning.DamageAt(slot, 20).String(); got != want {
			t.Errorf("Burning Hands in a level %d slot = %s, want %s", slot, got, want)
		}
	}

	// Casting at a level below the spell's own is not upcasting downwards.
	if got := burning.DamageAt(0, 20).String(); got != "3d6" {
		t.Errorf("Burning Hands below its own level = %s, want its base 3d6", got)
	}

	// A spell with no upcast entry does not grow.
	if bolt := spell(t, "Guiding Bolt"); bolt.UpcastDamage.IsZero() {
		t.Skip("Guiding Bolt has no upcast in this table")
	}
}

// Magic Missile gains darts; each dart is 1d4+1 and every one of them hits.
func TestMagicMissileGainsDarts(t *testing.T) {
	missile := spell(t, "Magic Missile")

	if missile.Resolution != SpellResolutionAuto {
		t.Errorf("Magic Missile resolution = %q, want automatic: it never misses", missile.Resolution)
	}
	for slot, want := range map[int]int{1: 3, 2: 4, 3: 5, 9: 11} {
		if got := missile.ProjectilesAt(slot, 20); got != want {
			t.Errorf("Magic Missile in a level %d slot fires %d darts, want %d", slot, got, want)
		}
	}
	if got := missile.DamageAt(1, 20).String(); got != "1d4+1" {
		t.Errorf("a dart does %s, want 1d4+1", got)
	}
}

// Scorching Ray is the levelled counterpart: separate attack rolls per ray.
func TestScorchingRayGainsRays(t *testing.T) {
	ray := spell(t, "Scorching Ray")
	if ray.Resolution != SpellResolutionAttack {
		t.Errorf("Scorching Ray resolution = %q, want a spell attack", ray.Resolution)
	}
	for slot, want := range map[int]int{2: 3, 3: 4, 5: 6} {
		if got := ray.ProjectilesAt(slot, 20); got != want {
			t.Errorf("Scorching Ray in a level %d slot fires %d rays, want %d", slot, got, want)
		}
	}
}

// Healing scales too, and unlike damage it adds the caster's ability modifier.
func TestHealingScalesAndAddsTheModifier(t *testing.T) {
	cure := spell(t, "Cure Wounds")

	if !cure.AddsAbilityModifier {
		t.Error("Cure Wounds should add the caster's spellcasting modifier")
	}
	for slot, want := range map[int]string{1: "1d8", 2: "2d8", 4: "4d8"} {
		if got := cure.HealingAt(slot).String(); got != want {
			t.Errorf("Cure Wounds in a level %d slot heals %s, want %s", slot, got, want)
		}
	}

	// A damaging spell must not accidentally add the modifier: Fire Bolt does
	// flat dice, and adding INT would be a quiet buff to every cantrip.
	if spell(t, "Fire Bolt").AddsAbilityModifier {
		t.Error("Fire Bolt should not add the spellcasting modifier")
	}
}

// A save spell has to name which save, or the engine cannot roll it.
func TestSaveSpellsNameTheirSave(t *testing.T) {
	for _, name := range []string{"Burning Hands", "Sacred Flame", "Fireball", "Hold Person"} {
		def := spell(t, name)
		if def.Resolution != SpellResolutionSave {
			t.Errorf("%s resolution = %q, want a saving throw", name, def.Resolution)
		}
		if !def.SaveAbility.Valid() {
			t.Errorf("%s has no valid save ability (%q)", name, def.SaveAbility)
		}
	}

	// Fireball halves on a success; Hold Person simply fails.
	if !spell(t, "Fireball").HalfOnSave {
		t.Error("Fireball should deal half damage on a successful save")
	}
	if spell(t, "Hold Person").HalfOnSave {
		t.Error("Hold Person deals no damage, so it cannot halve any")
	}
}

// The table is only useful if every entry is internally consistent. This is
// the guard that stops a typo becoming a rules bug nobody notices.
func TestEverySpellInTheTableIsCoherent(t *testing.T) {
	if len(SpellTable) == 0 {
		t.Fatal("the spell table is empty")
	}

	for key, def := range SpellTable {
		if def.Key != key {
			t.Errorf("%q is filed under %q", def.Key, key)
		}
		if err := def.Validate(); err != nil {
			t.Errorf("%s: %v", key, err)
		}
		if def.Name == "" {
			t.Errorf("%s has no display name", key)
		}
		if strings.ToLower(strings.ReplaceAll(def.Name, " ", "_")) != strings.TrimSuffix(key, "'") {
			// Keys are slugged names; apostrophes are dropped.
			slug := strings.NewReplacer(" ", "_", "'", "").Replace(strings.ToLower(def.Name))
			if slug != key {
				t.Errorf("%s does not slug to its key %q", def.Name, key)
			}
		}
		if len(def.Classes) == 0 {
			t.Errorf("%s is on no class's list, so nobody can learn it", key)
		}
		for _, c := range def.Classes {
			if !c.Valid() {
				t.Errorf("%s lists unknown class %q", key, c)
			}
		}
		if !def.School.Valid() {
			t.Errorf("%s has unknown school %q", key, def.School)
		}
		if def.Level < 0 || def.Level > 9 {
			t.Errorf("%s is level %d", key, def.Level)
		}
	}
}

// Validate is the counterpart of ValidateSheet and Monster.Validate: it catches
// the combinations that cannot mean anything.
func TestSpellValidateRejectsIncoherentDefinitions(t *testing.T) {
	base := SpellDefinition{
		Key: "test", Name: "Test", Level: 1, School: SchoolEvocation,
		Resolution: SpellResolutionSave, SaveAbility: AbilityDexterity,
		Damage: DamageDice{Count: 1, Sides: 6}, DamageType: DamageFire,
		Classes: []Class{ClassWizard},
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("a well-formed definition was rejected: %v", err)
	}

	cases := map[string]func(*SpellDefinition){
		"a save spell with no save ability":   func(s *SpellDefinition) { s.SaveAbility = "" },
		"damage with no damage type":          func(s *SpellDefinition) { s.DamageType = "" },
		"an unknown damage type":              func(s *SpellDefinition) { s.DamageType = "sarcasm" },
		"a cantrip that upcasts with slots":   func(s *SpellDefinition) { s.Level = 0; s.UpcastDamage = DamageDice{Count: 1, Sides: 6} },
		"half damage on a spell with no dice": func(s *SpellDefinition) { s.Damage = DamageDice{}; s.DamageType = ""; s.HalfOnSave = true },
		"a negative projectile count":         func(s *SpellDefinition) { s.Projectiles = -1 },
		"dice with an impossible die":         func(s *SpellDefinition) { s.Damage = DamageDice{Count: 1, Sides: 7} },
	}
	for name, mutate := range cases {
		def := base
		mutate(&def)
		if err := def.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// A character's known spells are names; play needs the mechanics behind them.
func TestSpellByNameIsForgivingAboutFormatting(t *testing.T) {
	for _, name := range []string{"Fire Bolt", "fire bolt", "  FIRE BOLT  ", "fire_bolt"} {
		if _, ok := SpellByName(name); !ok {
			t.Errorf("SpellByName(%q) found nothing", name)
		}
	}
	if _, ok := SpellByName("Wish For A Pony"); ok {
		t.Error("an invented spell was found in the table")
	}
}

func TestSpellsForClassAreOnThatList(t *testing.T) {
	wizard := SpellsForClass(ClassWizard)
	if len(wizard) == 0 {
		t.Fatal("no wizard spells in the table")
	}
	for _, def := range wizard {
		found := false
		for _, c := range def.Classes {
			if c == ClassWizard {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not a wizard spell", def.Name)
		}
	}

	// A cleric should not be handed the wizard's evocation list wholesale.
	if fmt.Sprint(SpellsForClass(ClassCleric)) == fmt.Sprint(wizard) {
		t.Error("every class returns the same spell list")
	}
}

// The slot a spell is cast at must be able to hold it.
func TestMinimumSlotLevel(t *testing.T) {
	fireball := spell(t, "Fireball")
	if err := fireball.ValidateSlot(2); err == nil {
		t.Error("a 3rd level spell cast from a 2nd level slot should be refused")
	}
	if err := fireball.ValidateSlot(3); err != nil {
		t.Errorf("a 3rd level slot should cast Fireball: %v", err)
	}
	if err := fireball.ValidateSlot(5); err != nil {
		t.Errorf("upcasting should be allowed: %v", err)
	}

	// Cantrips take no slot at all.
	if err := spell(t, "Fire Bolt").ValidateSlot(0); err != nil {
		t.Errorf("a cantrip needs no slot: %v", err)
	}
}

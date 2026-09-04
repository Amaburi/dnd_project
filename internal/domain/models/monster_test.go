package models

import (
	"strings"
	"testing"
)

func goblin() Monster {
	for _, m := range SRDMonsters() {
		if m.MonsterID == "srd_goblin" {
			return m
		}
	}
	panic("goblin missing from the SRD catalogue")
}

// Combat used to take a bare damage amount, so a fire-immune creature took
// full fire damage in every actual encounter.
func TestCombatantAppliesDamageAffinity(t *testing.T) {
	c := &Combatant{
		HitPoints: HitPoints{Current: 30, Maximum: 30},
		Status:    CombatantActive,
		Affinities: DamageAffinities{
			Immunities:      []DamageType{DamagePoison},
			Resistances:     []DamageType{DamageFire},
			Vulnerabilities: []DamageType{DamageCold},
		},
	}

	if dealt := c.TakeDamage(20, DamagePoison, false); dealt != 0 {
		t.Errorf("poison against an immune creature dealt %d, want 0", dealt)
	}
	if c.HitPoints.Current != 30 {
		t.Errorf("immune creature is at %d hit points, want 30", c.HitPoints.Current)
	}

	if dealt := c.TakeDamage(10, DamageFire, false); dealt != 5 {
		t.Errorf("resisted fire dealt %d, want 5", dealt)
	}
	if dealt := c.TakeDamage(5, DamageCold, false); dealt != 10 {
		t.Errorf("cold against a vulnerable creature dealt %d, want 10", dealt)
	}
	if c.HitPoints.Current != 15 {
		t.Errorf("hit points = %d, want 15", c.HitPoints.Current)
	}
}

// Monsters die at zero hit points; only characters roll to stabilise.
func TestMonstersDoNotMakeDeathSaves(t *testing.T) {
	monster := &Combatant{
		HitPoints:       HitPoints{Current: 5, Maximum: 20},
		Status:          CombatantActive,
		MakesDeathSaves: false,
	}
	monster.TakeDamage(5, DamageSlashing, false)
	if monster.Status != CombatantDead {
		t.Errorf("monster at 0 hit points is %s, want dead", monster.Status)
	}
	if monster.DeathSaves != (DeathSaves{}) {
		t.Errorf("monster rolled death saves: %+v", monster.DeathSaves)
	}

	character := &Combatant{
		HitPoints:       HitPoints{Current: 5, Maximum: 20},
		Status:          CombatantActive,
		MakesDeathSaves: true,
	}
	character.TakeDamage(5, DamageSlashing, false)
	if character.Status != CombatantDying {
		t.Errorf("character at 0 hit points is %s, want dying", character.Status)
	}
}

func TestCombatantConditionImmunity(t *testing.T) {
	c := &Combatant{ConditionImmunities: []Condition{ConditionPoisoned, ConditionCharmed}}

	if c.AddCondition(ConditionPoisoned) {
		t.Error("an immune combatant should not gain the condition")
	}
	if c.HasCondition(ConditionPoisoned) {
		t.Error("poisoned was applied despite immunity")
	}

	if !c.AddCondition(ConditionProne) {
		t.Error("a condition it is not immune to should apply")
	}
	if c.AddCondition(ConditionProne) {
		t.Error("applying the same condition twice should be a no-op")
	}
}

func TestLegendaryResistance(t *testing.T) {
	c := &Combatant{LegendaryResistanceRemaining: 2}

	if !c.UseLegendaryResistance() || !c.UseLegendaryResistance() {
		t.Fatal("both uses should be available")
	}
	if c.UseLegendaryResistance() {
		t.Error("a third use should be refused")
	}
	if c.LegendaryResistanceRemaining != 0 {
		t.Errorf("remaining = %d, want 0", c.LegendaryResistanceRemaining)
	}
}

// The bridge between statblock and combat: both sources feed one Combatant.
func TestMonsterToCombatant(t *testing.T) {
	m := goblin()
	m.Affinities = DamageAffinities{Resistances: []DamageType{DamageFire}}
	m.ConditionImmunities = []Condition{ConditionCharmed}
	m.LegendaryResistancePerDay = 3

	c := m.ToCombatant("c1")

	if c.SourceType != SourceMonster || c.SourceID != m.MonsterID {
		t.Errorf("source = %s/%s, want monster/%s", c.SourceType, c.SourceID, m.MonsterID)
	}
	if c.MakesDeathSaves {
		t.Error("a monster should not make death saves")
	}
	if c.ArmorClass != m.ArmorClass {
		t.Errorf("armor class = %d, want %d", c.ArmorClass, m.ArmorClass)
	}
	if c.HitPoints.Current != m.HitPoints.Maximum {
		t.Errorf("current hit points = %d, want a full %d", c.HitPoints.Current, m.HitPoints.Maximum)
	}
	if c.InitiativeModifier != m.InitiativeModifier() {
		t.Errorf("initiative modifier = %d, want %d", c.InitiativeModifier, m.InitiativeModifier())
	}
	if c.Speed != m.Speeds.Walk || c.MovementRemaining != m.Speeds.Walk {
		t.Errorf("speed = %d / movement %d, want %d", c.Speed, c.MovementRemaining, m.Speeds.Walk)
	}
	if c.LegendaryResistanceRemaining != 3 {
		t.Errorf("legendary resistance = %d, want 3", c.LegendaryResistanceRemaining)
	}
	// The affinities must survive the crossing, or combat ignores them again.
	if c.AffinityTo(DamageFire) != AffinityResistant {
		t.Error("fire resistance did not reach the combatant")
	}
	if !c.ImmuneToCondition(ConditionCharmed) {
		t.Error("condition immunity did not reach the combatant")
	}
}

func TestCharacterToCombatant(t *testing.T) {
	ch := newValidCharacter()
	ch.CombatStats.HitPoints = HitPoints{Current: 18, Maximum: 24}

	c := ch.ToCombatant("c2")

	if c.SourceType != SourceCharacter || c.SourceID != ch.CharacterID {
		t.Errorf("source = %s/%s, want character", c.SourceType, c.SourceID)
	}
	if !c.MakesDeathSaves {
		t.Error("a player character makes death saves")
	}
	if c.Type != "player" {
		t.Errorf("type = %q, want player", c.Type)
	}
	if c.ArmorClass != ch.ArmorClass() {
		t.Errorf("armor class = %d, want the computed %d", c.ArmorClass, ch.ArmorClass())
	}
	if c.HitPoints.Current != 18 {
		t.Errorf("current hit points = %d, want the sheet's 18", c.HitPoints.Current)
	}

	// NPCs are characters too, and are labelled as such.
	ch.Type = CharacterNPC
	if got := ch.ToCombatant("c3").Type; got != "npc" {
		t.Errorf("npc combatant type = %q, want npc", got)
	}
}

func TestMonsterDerivesProficiencyXPAndPassivePerception(t *testing.T) {
	m := goblin()

	// CR 1/4 -> proficiency +2, 50 XP.
	if got := m.ProficiencyBonus(); got != 2 {
		t.Errorf("proficiency bonus = %d, want 2", got)
	}
	if got := m.XP(); got != 50 {
		t.Errorf("XP = %d, want 50", got)
	}
	// Goblin has WIS 8 and no Perception bonus: 10 + (-1).
	if got := m.PassivePerception(); got != 9 {
		t.Errorf("passive perception = %d, want 9", got)
	}

	// A creature with a listed Perception bonus uses it.
	troll := Monster{
		AbilityScores: AbilityScores{Wisdom: 9},
		Skills:        map[Skill]int{SkillPerception: 2},
	}
	if got := troll.PassivePerception(); got != 12 {
		t.Errorf("passive perception with a listed bonus = %d, want 12", got)
	}

	// Higher CR raises the proficiency bonus.
	boss := Monster{ChallengeRating: 17}
	if got := boss.ProficiencyBonus(); got != 6 {
		t.Errorf("CR 17 proficiency = %d, want 6", got)
	}
}

func TestParseHitDiceFormula(t *testing.T) {
	cases := []struct {
		text    string
		count   int
		die     int
		bonus   int
		average int
	}{
		{"2d6", 2, 6, 0, 7},
		{"2d6-2", 2, 6, -2, 5},
		{"3d8+9", 3, 8, 9, 22},
		{"5d10+10", 5, 10, 10, 37},
		{"7d10+21", 7, 10, 21, 59},
		{"8d10+40", 8, 10, 40, 84},
		{"18d10+36", 18, 10, 36, 135},
		{" 6D8 + 18 ", 6, 8, 18, 45},
	}

	for _, tc := range cases {
		got, err := ParseHitDiceFormula(tc.text)
		if err != nil {
			t.Errorf("ParseHitDiceFormula(%q): %v", tc.text, err)
			continue
		}
		if got.Count != tc.count || got.Die != tc.die || got.Bonus != tc.bonus {
			t.Errorf("ParseHitDiceFormula(%q) = %+v, want %dd%d%+d", tc.text, got, tc.count, tc.die, tc.bonus)
		}
		if avg := got.Average(); avg != tc.average {
			t.Errorf("%q averages %d, want %d", tc.text, avg, tc.average)
		}
	}

	for _, bad := range []string{"", "d6", "2x6", "2d", "abc", "0d6"} {
		if _, err := ParseHitDiceFormula(bad); err == nil {
			t.Errorf("ParseHitDiceFormula(%q) should have failed", bad)
		}
	}
}

func TestMonsterValidate(t *testing.T) {
	valid := goblin()
	valid.CampaignID = "c1"
	if err := valid.Validate(); err != nil {
		t.Fatalf("a well-formed statblock was rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Monster)
		want   string
	}{
		{"no name", func(m *Monster) { m.Name = "" }, "name is required"},
		{"bad size", func(m *Monster) { m.Size = "enormous" }, "unknown size"},
		{"no hit points", func(m *Monster) { m.HitPoints.Maximum = 0; m.HitDice = "" }, "hit point maximum"},
		{"bad CR", func(m *Monster) { m.ChallengeRating = 7.5 }, "challenge rating"},
		{"invented damage type", func(m *Monster) {
			m.Affinities.Resistances = []DamageType{"banana"}
		}, "unknown damage resistance"},
		{"invented condition", func(m *Monster) {
			m.ConditionImmunities = []Condition{"inspired"}
		}, "unknown condition immunity"},
		{"hit points disagree with dice", func(m *Monster) { m.HitPoints.Maximum = 99 }, "averages"},
		{"multiattack names nothing", func(m *Monster) {
			m.Actions = append(m.Actions, MonsterAction{
				Name:        "Multiattack",
				Multiattack: []MultiattackPart{{ActionName: "Tail Slap", Count: 1}},
			})
		}, "unknown action"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := goblin()
			tc.mutate(&m)
			err := m.Validate()
			if err == nil {
				t.Fatalf("expected %q to be rejected", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// A multiattack that names other actions is resolvable; a bare count was not.
func TestMultiattackResolvesItsParts(t *testing.T) {
	var owlbear Monster
	for _, m := range SRDMonsters() {
		if m.MonsterID == "srd_owlbear" {
			owlbear = m
		}
	}

	multi, ok := owlbear.Action("Multiattack")
	if !ok {
		t.Fatal("owlbear should have a multiattack")
	}
	if !multi.IsMultiattack() {
		t.Error("the action should report as a multiattack")
	}
	if len(multi.Multiattack) != 2 {
		t.Fatalf("multiattack has %d parts, want 2", len(multi.Multiattack))
	}

	for _, part := range multi.Multiattack {
		action, ok := owlbear.Action(part.ActionName)
		if !ok {
			t.Errorf("multiattack names %q, which the owlbear does not have", part.ActionName)
			continue
		}
		if action.AttackBonus == nil {
			t.Errorf("%q has no attack bonus", action.Name)
		}
	}
}

// Every seeded statblock must be valid, including its printed hit points
// matching what its dice average to.
func TestSRDCatalogueIsValid(t *testing.T) {
	monsters := SRDMonsters()
	if len(monsters) < 10 {
		t.Fatalf("catalogue has %d monsters, want a useful spread", len(monsters))
	}

	seenID, seenName := map[string]bool{}, map[string]bool{}
	for _, m := range monsters {
		m.CampaignID = "c1"
		if err := m.Validate(); err != nil {
			t.Errorf("%s: %v", m.Name, err)
		}
		if seenID[m.MonsterID] {
			t.Errorf("duplicate monster_id %q", m.MonsterID)
		}
		if seenName[m.Name] {
			t.Errorf("duplicate name %q", m.Name)
		}
		seenID[m.MonsterID], seenName[m.Name] = true, true

		if m.Source != SourceSRD {
			t.Errorf("%s source = %q, want %q", m.Name, m.Source, SourceSRD)
		}
		if m.Speeds.Walk == 0 {
			t.Errorf("%s has no walking speed", m.Name)
		}
		if len(m.Actions) == 0 {
			t.Errorf("%s has no actions", m.Name)
		}
	}
}

// The catalogue should span a usable range of challenge ratings.
func TestSRDCatalogueSpansChallengeRatings(t *testing.T) {
	low, high := 99.0, 0.0
	for _, m := range SRDMonsters() {
		if m.ChallengeRating < low {
			low = m.ChallengeRating
		}
		if m.ChallengeRating > high {
			high = m.ChallengeRating
		}
	}

	if low > 0.125 {
		t.Errorf("lowest challenge rating is %v, want something at or below 1/8", low)
	}
	if high < 5 {
		t.Errorf("highest challenge rating is %v, want at least 5", high)
	}
}

func TestMonsterActionLookupIsCaseInsensitive(t *testing.T) {
	m := goblin()
	if _, ok := m.Action("scimitar"); !ok {
		t.Error("action lookup should not depend on case")
	}
	if _, ok := m.Action("Longsword"); ok {
		t.Error("a goblin has no longsword")
	}
}

package rules

import (
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
)

// A fixed seed makes every assertion below exact rather than statistical.
func engine(seed int64) *Engine { return NewEngine(dice.NewSeeded(seed)) }

// scripted builds an engine whose dice the test states outright.
//
// A seeded roller is repeatable but not controllable: a test needing a hit had
// to hope the seed obliged, and the ones that did not were skipped. Naming the
// faces makes the outcome part of the test rather than a lucky accident.
func scripted(faces ...int) *Engine { return NewEngine(dice.NewScripted(faces...)) }

// Faces that force an outcome whatever the modifiers are.
const (
	faceCriticalHit = 20 // always hits, whatever the AC
	faceFumble      = 1  // always misses, whatever the bonus
)

func rogue() *models.Character {
	c := &models.Character{
		Name: "Thistle", Type: models.CharacterPlayer,
		BasicInfo: models.BasicInfo{
			Race: models.RaceHalfling, Subrace: "lightfoot",
			Background: models.BackgroundCriminal,
			Classes:    []models.ClassLevel{{Class: models.ClassRogue, Subclass: "thief", Level: 5}},
		},
		AbilityScores: models.AbilityScores{
			Strength: 10, Dexterity: 18, Constitution: 14,
			Intelligence: 12, Wisdom: 13, Charisma: 11,
		},
		Skills: models.SkillProficiencies{
			models.SkillStealth:    models.ProficiencyExpertise,
			models.SkillAcrobatics: models.ProficiencyProficient,
		},
		Proficiencies: models.Proficiencies{
			Weapons: []string{models.ProfSimpleWeapons, "rapier", "shortsword"},
		},
	}
	c.ApplyClassDefaults()
	return c
}

func rapier() models.InventoryItem {
	return models.InventoryItem{
		ItemID: "w1", Key: "rapier", Name: "Rapier", Kind: models.ItemWeapon,
		Weapon: &models.WeaponProperties{
			Category: models.WeaponMartial, DamageDice: "1d8",
			DamageType: models.DamagePiercing,
			Properties: []models.WeaponProperty{models.PropertyFinesse},
		},
	}
}

func dummy(ac, hp int) *models.Combatant {
	return &models.Combatant{
		CombatantID: "t1", Name: "Training Dummy",
		ArmorClass: ac,
		HitPoints:  models.HitPoints{Current: hp, Maximum: hp},
		Status:     models.CombatantActive,
	}
}

func TestSkillCheckUsesTheCharactersModifier(t *testing.T) {
	e := engine(1)
	c := rogue()

	// Rogue 5: proficiency +3, DEX +4, Stealth expertise doubles the +3.
	want := c.SkillModifier(models.SkillStealth)
	if want != 10 {
		t.Fatalf("test fixture drifted: Stealth modifier = %d, want 10", want)
	}

	result := e.SkillCheck(c, models.SkillStealth, 15, models.RollNormal)
	if result.Modifier != want {
		t.Errorf("modifier = %d, want %d", result.Modifier, want)
	}
	if result.Roll.Total != result.Roll.Natural+want {
		t.Errorf("total = %d, want natural %d plus %d", result.Roll.Total, result.Roll.Natural, want)
	}
	if result.Margin != result.Roll.Total-15 {
		t.Errorf("margin = %d, want total minus DC", result.Margin)
	}
	if result.Kind != KindSkillCheck || result.Skill != models.SkillStealth {
		t.Errorf("kind/skill = %s/%s", result.Kind, result.Skill)
	}
	if result.Succeeded() != (result.Roll.Total >= 15) {
		t.Error("outcome disagrees with the total")
	}
}

// The character's own state folds into the roll mode: exhaustion imposes
// disadvantage on ability checks without the caller asking for it.
func TestSkillCheckFoldsInCharacterState(t *testing.T) {
	e := engine(2)
	c := rogue()
	c.Exhaustion = 1

	result := e.SkillCheck(c, models.SkillAcrobatics, 12, models.RollNormal)
	if result.Roll.Mode != models.RollDisadvantage {
		t.Errorf("mode = %s, want disadvantage from exhaustion", result.Roll.Mode)
	}

	// And a situational advantage cancels it back to normal.
	result = e.SkillCheck(c, models.SkillAcrobatics, 12, models.RollAdvantage)
	if result.Roll.Mode != models.RollNormal {
		t.Errorf("mode = %s, want normal (advantage cancels disadvantage)", result.Roll.Mode)
	}
}

func TestSavingThrowUsesProficiency(t *testing.T) {
	e := engine(3)
	c := rogue()

	// Rogue saves are DEX and INT, applied by ApplyClassDefaults.
	dex := e.SavingThrow(c, models.AbilityDexterity, 15, models.RollNormal)
	if dex.Modifier != 4+3 {
		t.Errorf("DEX save modifier = %d, want +7", dex.Modifier)
	}

	str := e.SavingThrow(c, models.AbilityStrength, 15, models.RollNormal)
	if str.Modifier != 0 {
		t.Errorf("STR save modifier = %d, want +0 (unproficient, STR 10)", str.Modifier)
	}
	if dex.Kind != KindSavingThrow {
		t.Errorf("kind = %s, want saving_throw", dex.Kind)
	}
}

func TestWeaponAttackAppliesDamageToTheTarget(t *testing.T) {
	e := engine(4)
	c := rogue()
	target := dummy(10, 40) // low AC so the attack lands

	result, err := e.WeaponAttack(c, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}

	// DEX +4 (finesse) plus proficiency +3.
	if result.AttackBonus != 7 {
		t.Errorf("attack bonus = %+d, want +7", result.AttackBonus)
	}
	if !result.Hit() {
		t.Fatalf("a +7 attack against AC 10 missed: %s", result.Summary())
	}
	if result.Damage == nil {
		t.Fatal("a hit produced no damage")
	}
	if result.Damage.Type != models.DamagePiercing {
		t.Errorf("damage type = %s, want piercing", result.Damage.Type)
	}
	if got := 40 - target.HitPoints.Current; got != result.Damage.Dealt {
		t.Errorf("target lost %d hit points but the result says %d", got, result.Damage.Dealt)
	}
	if result.TargetHP.Current != target.HitPoints.Current {
		t.Error("the result's target hit points disagree with the combatant")
	}
}

func TestWeaponAttackMissesLeaveTheTargetAlone(t *testing.T) {
	e := engine(5)
	c := rogue()
	target := dummy(40, 40) // unreachable AC

	result, err := e.WeaponAttack(c, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if result.Hit() {
		t.Fatalf("a +7 attack should not reach AC 40: %s", result.Summary())
	}
	if result.Damage != nil {
		t.Error("a miss produced damage")
	}
	if target.HitPoints.Current != 40 {
		t.Errorf("target lost hit points on a miss: %d", target.HitPoints.Current)
	}
}

// Resistance halves what the target actually loses, and the result reports
// both what was rolled and what was dealt.
func TestAttackHonoursTargetResistance(t *testing.T) {
	e := engine(6)
	c := rogue()

	target := dummy(5, 100)
	target.Affinities = models.DamageAffinities{Resistances: []models.DamageType{models.DamagePiercing}}

	result, err := e.WeaponAttack(c, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if !result.Hit() {
		t.Fatalf("attack missed AC 5: %s", result.Summary())
	}
	if result.Damage.Affinity != models.AffinityResistant {
		t.Errorf("affinity = %s, want resistant", result.Damage.Affinity)
	}
	if result.Damage.Dealt != result.Damage.Rolled/2 {
		t.Errorf("dealt %d from a rolled %d, want half", result.Damage.Dealt, result.Damage.Rolled)
	}
	if !strings.Contains(result.Summary(), "resisted") {
		t.Errorf("summary does not mention resistance: %q", result.Summary())
	}
}

func TestAttackAgainstImmuneTargetDealsNothing(t *testing.T) {
	// A natural 20 hits regardless of AC, and maximum damage dice make the
	// immunity the only reason nothing lands.
	e := scripted(faceCriticalHit, 8)
	c := rogue()

	target := dummy(5, 50)
	target.Affinities = models.DamageAffinities{Immunities: []models.DamageType{models.DamagePiercing}}

	result, err := e.WeaponAttack(c, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if !result.Hit() {
		t.Fatalf("a natural 20 missed: %s", result.Summary())
	}
	if result.Damage.Rolled <= 0 {
		t.Fatal("the damage dice rolled nothing, so immunity proves nothing")
	}
	if result.Damage.Dealt != 0 {
		t.Errorf("an immune target lost %d hit points", result.Damage.Dealt)
	}
	if target.HitPoints.Current != 50 {
		t.Errorf("target at %d hit points, want an untouched 50", target.HitPoints.Current)
	}
}

// Dropping a monster to 0 kills it; the result says so.
func TestAttackThatKills(t *testing.T) {
	e := scripted(faceCriticalHit, 8)
	c := rogue()
	target := dummy(5, 1) // one hit point, and it does not make death saves

	result, err := e.WeaponAttack(c, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if !result.Hit() {
		t.Fatalf("a natural 20 missed: %s", result.Summary())
	}
	if result.TargetStatus != models.CombatantDead {
		t.Errorf("target status = %s, want dead", result.TargetStatus)
	}
	if !strings.Contains(result.Summary(), "dies") {
		t.Errorf("summary does not report the death: %q", result.Summary())
	}
}

func TestMonsterAttack(t *testing.T) {
	e := engine(9)

	var goblin models.Monster
	for _, m := range models.SRDMonsters() {
		if m.MonsterID == "srd_goblin" {
			goblin = m
		}
	}
	scimitar, ok := goblin.Action("Scimitar")
	if !ok {
		t.Fatal("goblin has no scimitar")
	}

	target := dummy(10, 30)
	result, err := e.MonsterAttack(&goblin, scimitar, target, models.RollNormal)
	if err != nil {
		t.Fatalf("MonsterAttack: %v", err)
	}
	if result.Attacker != "Goblin" || result.Weapon != "Scimitar" {
		t.Errorf("attacker/weapon = %s/%s", result.Attacker, result.Weapon)
	}
	if result.AttackBonus != 4 {
		t.Errorf("attack bonus = %+d, want +4", result.AttackBonus)
	}

	// A non-attack action is rejected rather than silently rolling nothing.
	if _, err := e.MonsterAttack(&goblin, models.MonsterAction{Name: "Howl"}, target, models.RollNormal); err == nil {
		t.Error("a non-attack action should be rejected")
	}
}

// Everything the narration layer is allowed to say comes from Facts, so a
// prompt built from it cannot invent an outcome.
func TestFactsCoverTheOutcome(t *testing.T) {
	e := engine(10)
	c := rogue()
	target := dummy(10, 40)

	attack, _ := e.WeaponAttack(c, rapier(), target, models.RollNormal)
	facts := attack.Facts()

	for _, key := range []string{
		"attacker", "target", "weapon", "attack_total", "target_ac", "outcome",
		"hit", "critical", "damage_total", "damage_type", "damage_affinity",
		"target_status", "target_hp", "fact_summary", "natural", "roll_mode",
	} {
		if _, ok := facts[key]; !ok {
			t.Errorf("attack facts are missing %q", key)
		}
	}
	// Every value must be non-empty, or a prompt would interpolate a blank.
	for key, value := range facts {
		if value == "" {
			t.Errorf("attack fact %q is empty", key)
		}
	}
	if facts["hit"] != "yes" && facts["hit"] != "no" {
		t.Errorf("hit = %q, want yes or no", facts["hit"])
	}

	check := e.SkillCheck(c, models.SkillStealth, 15, models.RollNormal)
	checkFacts := check.Facts()
	for _, key := range []string{
		"check_kind", "actor", "ability", "skill", "dc", "total",
		"outcome", "margin", "was_close", "fact_summary",
	} {
		if _, ok := checkFacts[key]; !ok {
			t.Errorf("check facts are missing %q", key)
		}
	}
	for key, value := range checkFacts {
		if value == "" {
			t.Errorf("check fact %q is empty", key)
		}
	}
}

// A miss still supplies every damage key, so a narration template never fails
// to build for want of a variable.
func TestFactsAreCompleteOnAMiss(t *testing.T) {
	// A natural 1 misses whatever the bonus, so the miss is stated rather
	// than hoped for.
	e := scripted(faceFumble)
	c := rogue()
	target := dummy(5, 40)

	attack, err := e.WeaponAttack(c, rapier(), target, models.RollNormal)
	if err != nil {
		t.Fatalf("WeaponAttack: %v", err)
	}
	if attack.Hit() {
		t.Fatalf("a natural 1 hit: %s", attack.Summary())
	}

	facts := attack.Facts()
	if facts["damage_total"] != "0" {
		t.Errorf("damage_total on a miss = %q, want 0", facts["damage_total"])
	}
	if facts["hit"] != "no" {
		t.Errorf("hit = %q, want no", facts["hit"])
	}
	for key, value := range facts {
		if value == "" {
			t.Errorf("fact %q is empty on a miss", key)
		}
	}
}

// The summary is the sentence the narration must not contradict.
func TestSummaryStatesTheOutcome(t *testing.T) {
	e := engine(12)
	c := rogue()

	check := e.SkillCheck(c, models.SkillStealth, 15, models.RollNormal)
	summary := check.Summary()
	if !strings.Contains(summary, "Thistle") || !strings.Contains(summary, "DC 15") {
		t.Errorf("check summary = %q, want the actor and DC", summary)
	}
	if check.Succeeded() && !strings.Contains(summary, "succeeds") {
		t.Errorf("a success reads as %q", summary)
	}
	if !check.Succeeded() && !strings.Contains(summary, "fails") {
		t.Errorf("a failure reads as %q", summary)
	}
}

// The same seed replays the same fight, which is what makes a rules change
// visible rather than lost in noise.
func TestResolutionIsReproducible(t *testing.T) {
	run := func() string {
		e := engine(2025)
		c := rogue()
		target := dummy(13, 30)

		var log []string
		for i := 0; i < 5; i++ {
			result, err := e.WeaponAttack(c, rapier(), target, models.RollNormal)
			if err != nil {
				t.Fatalf("WeaponAttack: %v", err)
			}
			log = append(log, result.Summary())
		}
		return strings.Join(log, "\n")
	}

	if run() != run() {
		t.Error("the same seed produced a different fight")
	}
}

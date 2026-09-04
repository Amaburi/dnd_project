package models

import "testing"

// levels builds a single-class BasicInfo for tests that only care about the
// character's level.
func levels(c Class, level int) BasicInfo {
	return BasicInfo{Classes: []ClassLevel{{Class: c, Level: level}}}
}

// Go truncates integer division toward zero, so a naive (score-10)/2 gives -1
// for a score of 7 where D&D wants -2.
func TestAbilityModifierRoundsTowardNegativeInfinity(t *testing.T) {
	cases := map[int]int{
		1: -5, 2: -4, 3: -4, 4: -3, 5: -3, 6: -2, 7: -2, 8: -1, 9: -1,
		10: 0, 11: 0, 12: 1, 13: 1, 14: 2, 15: 2, 16: 3, 17: 3,
		18: 4, 19: 4, 20: 5, 22: 6, 30: 10,
	}
	for score, want := range cases {
		if got := AbilityModifier(score); got != want {
			t.Errorf("AbilityModifier(%d) = %d, want %d", score, got, want)
		}
	}
}

func TestProficiencyBonusByLevel(t *testing.T) {
	cases := map[int]int{
		1: 2, 4: 2, 5: 3, 8: 3, 9: 4, 12: 4, 13: 5, 16: 5, 17: 6, 20: 6,
	}
	for level, want := range cases {
		if got := ProficiencyBonusForLevel(level); got != want {
			t.Errorf("ProficiencyBonusForLevel(%d) = %d, want %d", level, got, want)
		}
	}

	// Out-of-range levels clamp rather than producing nonsense.
	if got := ProficiencyBonusForLevel(0); got != 2 {
		t.Errorf("ProficiencyBonusForLevel(0) = %d, want 2", got)
	}
	if got := ProficiencyBonusForLevel(99); got != 6 {
		t.Errorf("ProficiencyBonusForLevel(99) = %d, want 6", got)
	}
}

// Every skill must map to a governing ability, or a check cannot be resolved.
func TestEverySkillHasAnAbility(t *testing.T) {
	if len(Skills) != 18 {
		t.Fatalf("got %d skills, want the 18 in the PHB", len(Skills))
	}
	for _, s := range Skills {
		ability, ok := SkillAbility[s]
		if !ok {
			t.Errorf("skill %q has no governing ability", s)
			continue
		}
		if !ability.Valid() {
			t.Errorf("skill %q maps to invalid ability %q", s, ability)
		}
	}
	if len(SkillAbility) != len(Skills) {
		t.Errorf("SkillAbility has %d entries but there are %d skills", len(SkillAbility), len(Skills))
	}
}

// Proficiency is four-valued: a rogue's Expertise doubles the bonus and a
// bard's Jack of All Trades adds half of it, rounded down.
func TestSkillModifierAppliesProficiencyLevels(t *testing.T) {
	c := &Character{
		BasicInfo:     levels(ClassRogue, 5), // proficiency bonus +3
		AbilityScores: AbilityScores{Dexterity: 16, Wisdom: 12},
		Skills: SkillProficiencies{
			SkillStealth:    ProficiencyExpertise,
			SkillAcrobatics: ProficiencyProficient,
			SkillPerception: ProficiencyHalf,
		},
	}

	// Stealth: DEX +3, expertise doubles the +3 bonus.
	if got, want := c.SkillModifier(SkillStealth), 3+6; got != want {
		t.Errorf("expertise Stealth = %d, want %d", got, want)
	}
	// Acrobatics: DEX +3 plus the plain +3.
	if got, want := c.SkillModifier(SkillAcrobatics), 3+3; got != want {
		t.Errorf("proficient Acrobatics = %d, want %d", got, want)
	}
	// Perception: WIS +1 plus half of +3, rounded down to +1.
	if got, want := c.SkillModifier(SkillPerception), 1+1; got != want {
		t.Errorf("half-proficient Perception = %d, want %d", got, want)
	}
	// Athletics: unproficient, so the bare Strength modifier of a 0 score.
	if got, want := c.SkillModifier(SkillAthletics), AbilityModifier(0); got != want {
		t.Errorf("unproficient Athletics = %d, want %d", got, want)
	}
}

func TestPassivePerceptionIsTenPlusModifier(t *testing.T) {
	c := &Character{
		BasicInfo:     levels(ClassRanger, 1),
		AbilityScores: AbilityScores{Wisdom: 14},
		Skills:        SkillProficiencies{SkillPerception: ProficiencyProficient},
	}
	// 10 + WIS +2 + proficiency +2
	if got, want := c.PassivePerception(), 14; got != want {
		t.Errorf("PassivePerception = %d, want %d", got, want)
	}
}

func TestArmorClassByArmorCategory(t *testing.T) {
	dexterous := AbilityScores{Dexterity: 18} // +4

	unarmored := &Character{AbilityScores: dexterous}
	if got, want := unarmored.ArmorClass(), 14; got != want {
		t.Errorf("unarmored AC = %d, want %d", got, want)
	}

	// Light armor adds the full Dexterity modifier.
	leather := &Character{AbilityScores: dexterous, Equipment: Equipment{
		Armor: &InventoryItem{Name: "Leather", Kind: ItemArmor,
			Armor: &ArmorProperties{Category: ArmorLight, BaseAC: 11}},
	}}
	if got, want := leather.ArmorClass(), 15; got != want {
		t.Errorf("light armor AC = %d, want %d", got, want)
	}

	// Medium armor caps the Dexterity contribution at +2.
	halfPlate := &Character{AbilityScores: dexterous, Equipment: Equipment{
		Armor: &InventoryItem{Name: "Half Plate", Kind: ItemArmor,
			Armor: &ArmorProperties{Category: ArmorMedium, BaseAC: 15}},
	}}
	if got, want := halfPlate.ArmorClass(), 17; got != want {
		t.Errorf("medium armor AC = %d, want %d", got, want)
	}

	// Heavy armor ignores Dexterity entirely.
	plate := &Character{AbilityScores: dexterous, Equipment: Equipment{
		Armor: &InventoryItem{Name: "Plate", Kind: ItemArmor,
			Armor: &ArmorProperties{Category: ArmorHeavy, BaseAC: 18}},
	}}
	if got, want := plate.ArmorClass(), 18; got != want {
		t.Errorf("heavy armor AC = %d, want %d", got, want)
	}

	// A shield adds 2, plus its own magic bonus.
	withShield := &Character{AbilityScores: dexterous, Equipment: Equipment{
		Armor: &InventoryItem{Name: "Plate", Kind: ItemArmor,
			Armor: &ArmorProperties{Category: ArmorHeavy, BaseAC: 18}},
		Shield: &InventoryItem{Name: "+1 Shield", Kind: ItemShield,
			Armor: &ArmorProperties{MagicBonus: 1}},
	}}
	if got, want := withShield.ArmorClass(), 21; got != want {
		t.Errorf("plate and +1 shield AC = %d, want %d", got, want)
	}
}

// A finesse weapon lets the wielder pick the better of Strength and Dexterity.
func TestWeaponAttackAbility(t *testing.T) {
	scores := AbilityScores{Strength: 10, Dexterity: 18}

	rapier := WeaponProperties{DamageDice: "1d8", Properties: []WeaponProperty{PropertyFinesse}}
	if got := rapier.AttackAbility(scores); got != AbilityDexterity {
		t.Errorf("finesse weapon with high DEX used %q, want dexterity", got)
	}

	mace := WeaponProperties{DamageDice: "1d6"}
	if got := mace.AttackAbility(scores); got != AbilityStrength {
		t.Errorf("melee weapon used %q, want strength", got)
	}

	longbow := WeaponProperties{DamageDice: "1d8", RangeNormal: 150, RangeLong: 600}
	if got := longbow.AttackAbility(scores); got != AbilityDexterity {
		t.Errorf("ranged weapon used %q, want dexterity", got)
	}
}

func TestHitPointsDamageUsesTemporaryFirst(t *testing.T) {
	hp := HitPoints{Current: 20, Maximum: 20, Temporary: 5}

	if overflow := hp.ApplyDamage(3); overflow != 0 {
		t.Errorf("overflow = %d, want 0", overflow)
	}
	if hp.Temporary != 2 || hp.Current != 20 {
		t.Errorf("after 3 damage: current=%d temp=%d, want 20/2", hp.Current, hp.Temporary)
	}

	// The remaining 2 temporary absorb part of the next hit.
	hp.ApplyDamage(6)
	if hp.Temporary != 0 || hp.Current != 16 {
		t.Errorf("after 6 more damage: current=%d temp=%d, want 16/0", hp.Current, hp.Temporary)
	}
}

func TestHitPointsClampAtZeroAndReportOverflow(t *testing.T) {
	hp := HitPoints{Current: 5, Maximum: 12}

	overflow := hp.ApplyDamage(9)
	if hp.Current != 0 {
		t.Errorf("current = %d, want 0 (hit points never go negative)", hp.Current)
	}
	if overflow != 4 {
		t.Errorf("overflow = %d, want 4", overflow)
	}
	if hp.IsMassiveDamage(overflow) {
		t.Error("4 overflow against a 12 maximum should not be massive damage")
	}
}

// Damage that exceeds the hit point maximum after dropping a creature to 0
// kills outright rather than knocking it unconscious.
func TestMassiveDamageKillsOutright(t *testing.T) {
	hp := HitPoints{Current: 5, Maximum: 12}
	overflow := hp.ApplyDamage(20)

	if overflow != 15 {
		t.Fatalf("overflow = %d, want 15", overflow)
	}
	if !hp.IsMassiveDamage(overflow) {
		t.Error("15 overflow against a 12 maximum is massive damage")
	}
}

func TestTemporaryHitPointsDoNotStack(t *testing.T) {
	hp := HitPoints{Current: 10, Maximum: 10, Temporary: 8}

	hp.AddTemporary(5) // smaller, so it is refused
	if hp.Temporary != 8 {
		t.Errorf("temp = %d, want 8 (a smaller grant is not taken)", hp.Temporary)
	}

	hp.AddTemporary(12) // larger, so it replaces
	if hp.Temporary != 12 {
		t.Errorf("temp = %d, want 12 (a larger grant replaces)", hp.Temporary)
	}
}

func TestHealCapsAtMaximumAndIgnoresTemporary(t *testing.T) {
	hp := HitPoints{Current: 4, Maximum: 10, Temporary: 3}
	hp.Heal(100)

	if hp.Current != 10 {
		t.Errorf("current = %d, want 10", hp.Current)
	}
	if hp.Temporary != 3 {
		t.Errorf("temp = %d, want 3 (healing does not restore temporary hit points)", hp.Temporary)
	}
}

func TestSpellSlotsExpendAndRestore(t *testing.T) {
	s := &Spells{Slots: []SpellSlot{{Level: 1, Total: 4}, {Level: 2, Total: 3}}}

	if got := s.AvailableSlots(1); got != 4 {
		t.Errorf("available level 1 = %d, want 4", got)
	}

	for i := 0; i < 4; i++ {
		if err := s.ExpendSlot(1); err != nil {
			t.Fatalf("expend %d: %v", i+1, err)
		}
	}
	if err := s.ExpendSlot(1); err == nil {
		t.Error("expending a fifth level 1 slot should fail")
	}

	// Upcasting is why callers pass the level used, not the spell's level.
	if got := s.HighestSlotLevel(); got != 2 {
		t.Errorf("highest remaining slot = %d, want 2", got)
	}

	// A caster with no slots at a level is different from having spent them.
	if err := s.ExpendSlot(9); err == nil {
		t.Error("expending a slot the caster does not have should fail")
	}

	s.RestoreAllSlots()
	if got := s.AvailableSlots(1); got != 4 {
		t.Errorf("after a long rest, level 1 = %d, want 4", got)
	}
}

// Advantage and disadvantage never stack, and one of each cancels out.
func TestRollModeCombine(t *testing.T) {
	cases := []struct{ a, b, want RollMode }{
		{RollNormal, RollNormal, RollNormal},
		{RollAdvantage, RollNormal, RollAdvantage},
		{RollAdvantage, RollAdvantage, RollAdvantage},
		{RollDisadvantage, RollDisadvantage, RollDisadvantage},
		{RollAdvantage, RollDisadvantage, RollNormal},
		{RollDisadvantage, RollAdvantage, RollNormal},
	}
	for _, tc := range cases {
		if got := tc.a.Combine(tc.b); got != tc.want {
			t.Errorf("%s.Combine(%s) = %s, want %s", tc.a, tc.b, got, tc.want)
		}
	}
}

// A natural 20 hits regardless of AC and a natural 1 misses regardless of
// modifiers -- but only on attack rolls.
func TestResolveAttackHonoursNaturalRolls(t *testing.T) {
	nat20 := D20Result{Natural: 20, Modifier: -5, Total: 15}
	if got := ResolveAttack(nat20, 30, NaturalCrit); got != AttackCritical {
		t.Errorf("natural 20 against AC 30 = %s, want critical_hit", got)
	}

	nat1 := D20Result{Natural: 1, Modifier: 20, Total: 21}
	if got := ResolveAttack(nat1, 5, NaturalCrit); got != AttackFumble {
		t.Errorf("natural 1 against AC 5 = %s, want critical_miss", got)
	}

	if got := ResolveAttack(D20Result{Natural: 12, Modifier: 3, Total: 15}, 15, NaturalCrit); got != AttackHit {
		t.Errorf("meeting AC exactly = %s, want hit", got)
	}
	if got := ResolveAttack(D20Result{Natural: 12, Modifier: 3, Total: 15}, 16, NaturalCrit); got != AttackMiss {
		t.Errorf("one below AC = %s, want miss", got)
	}
}

// Ability checks have no automatic success or failure in 5e.
func TestResolveCheckIgnoresNaturalRolls(t *testing.T) {
	if got := ResolveCheck(D20Result{Natural: 20, Total: 21}, 25); got != OutcomeFailure {
		t.Errorf("natural 20 short of the DC = %s, want failure", got)
	}
	if got := ResolveCheck(D20Result{Natural: 1, Total: 16}, 15); got != OutcomeSuccess {
		t.Errorf("natural 1 that still meets the DC = %s, want success", got)
	}
}

func TestDamageAffinityScaling(t *testing.T) {
	if got := AffinityResistant.Apply(7); got != 3 {
		t.Errorf("resistance halves 7 to %d, want 3 (rounded down)", got)
	}
	if got := AffinityVulnerable.Apply(7); got != 14 {
		t.Errorf("vulnerability doubles 7 to %d, want 14", got)
	}
	if got := AffinityImmune.Apply(7); got != 0 {
		t.Errorf("immunity reduces 7 to %d, want 0", got)
	}
	if got := AffinityNormal.Apply(7); got != 7 {
		t.Errorf("normal leaves 7 as %d", got)
	}
}

// Immunity beats resistance beats vulnerability, so a creature listed under
// more than one is never scaled twice.
func TestMonsterDamageAffinityPrecedence(t *testing.T) {
	m := &Monster{
		HitPoints: HitPoints{Current: 30, Maximum: 30},
		Affinities: DamageAffinities{
			Immunities:      []DamageType{DamagePoison},
			Resistances:     []DamageType{DamageFire, DamageSlashing},
			Vulnerabilities: []DamageType{DamageFire, DamageCold},
		},
	}

	if got := m.AffinityTo(DamageFire); got != AffinityResistant {
		t.Errorf("fire listed as both resistant and vulnerable resolved to %s, want resistant", got)
	}
	if got := m.AffinityTo(DamagePoison); got != AffinityImmune {
		t.Errorf("poison = %s, want immune", got)
	}
	if got := m.AffinityTo(DamageCold); got != AffinityVulnerable {
		t.Errorf("cold = %s, want vulnerable", got)
	}
	if got := m.AffinityTo(DamageRadiant); got != AffinityNormal {
		t.Errorf("unlisted type = %s, want normal", got)
	}

	dealt, _ := m.TakeDamage(10, DamageSlashing)
	if dealt != 5 {
		t.Errorf("resisted slashing dealt %d, want 5", dealt)
	}
	if m.HitPoints.Current != 25 {
		t.Errorf("monster at %d hit points, want 25", m.HitPoints.Current)
	}

	dealt, _ = m.TakeDamage(50, DamagePoison)
	if dealt != 0 || m.HitPoints.Current != 25 {
		t.Errorf("immunity let %d damage through, leaving %d hit points", dealt, m.HitPoints.Current)
	}
}

func TestSpellSaveDCAndAttackModifier(t *testing.T) {
	wizard := &Character{
		BasicInfo:     levels(ClassWizard, 9), // proficiency +4
		AbilityScores: AbilityScores{Intelligence: 18},
		Spells:        Spells{SpellcastingAbility: AbilityIntelligence},
	}

	if got, want := wizard.SpellSaveDC(), 8+4+4; got != want {
		t.Errorf("SpellSaveDC = %d, want %d", got, want)
	}
	if got, want := wizard.SpellAttackModifier(), 4+4; got != want {
		t.Errorf("SpellAttackModifier = %d, want %d", got, want)
	}

	// A non-caster reports zero rather than a bonus built from nothing.
	fighter := &Character{BasicInfo: levels(ClassFighter, 9)}
	if got := fighter.SpellSaveDC(); got != 0 {
		t.Errorf("non-caster SpellSaveDC = %d, want 0", got)
	}
}

// The save to keep concentrating is DC 10 or half the damage, whichever is
// higher.
func TestConcentrationSaveDC(t *testing.T) {
	cases := map[int]int{5: 10, 20: 10, 21: 10, 22: 11, 50: 25}
	for damage, want := range cases {
		if got := ConcentrationSaveDC(damage); got != want {
			t.Errorf("ConcentrationSaveDC(%d) = %d, want %d", damage, got, want)
		}
	}
}

func TestCombatantDropsToDyingThenDies(t *testing.T) {
	c := &Combatant{
		HitPoints:       HitPoints{Current: 6, Maximum: 20},
		Status:          CombatantActive,
		MakesDeathSaves: true,
	}

	// Dropping to 0 without massive damage means dying, not dead.
	c.TakeDamage(6, DamageSlashing, false)
	if c.Status != CombatantDying {
		t.Fatalf("status = %s, want dying", c.Status)
	}
	if c.HitPoints.Current != 0 {
		t.Errorf("hit points = %d, want 0", c.HitPoints.Current)
	}

	// Damage taken while down is an automatic death save failure; a critical
	// counts as two.
	c.TakeDamage(1, DamageSlashing, false)
	if c.DeathSaves.Failures != 1 {
		t.Errorf("failures = %d, want 1", c.DeathSaves.Failures)
	}
	c.TakeDamage(1, DamageSlashing, true)
	if c.DeathSaves.Failures != 3 {
		t.Errorf("failures after a critical = %d, want 3", c.DeathSaves.Failures)
	}
	if c.Status != CombatantDead {
		t.Errorf("status = %s, want dead after three failures", c.Status)
	}
}

func TestCombatantMassiveDamageSkipsDeathSaves(t *testing.T) {
	c := &Combatant{
		HitPoints:       HitPoints{Current: 5, Maximum: 10},
		Status:          CombatantActive,
		MakesDeathSaves: true,
	}

	c.TakeDamage(20, DamageSlashing, false) // 15 overflow against a 10 maximum
	if c.Status != CombatantDead {
		t.Errorf("status = %s, want dead outright from massive damage", c.Status)
	}
	if c.DeathSaves.Failures != 0 {
		t.Errorf("failures = %d, want 0 (no saves are rolled)", c.DeathSaves.Failures)
	}
}

// Regaining even a single hit point ends the dying state and clears the saves.
func TestCombatantHealingClearsDeathSaves(t *testing.T) {
	c := &Combatant{
		HitPoints:  HitPoints{Current: 0, Maximum: 20},
		Status:     CombatantDying,
		DeathSaves: DeathSaves{Successes: 1, Failures: 2},
	}

	c.Heal(1)
	if c.Status != CombatantActive {
		t.Errorf("status = %s, want active", c.Status)
	}
	if c.DeathSaves != (DeathSaves{}) {
		t.Errorf("death saves = %+v, want cleared", c.DeathSaves)
	}

	// The dead do not benefit from ordinary healing.
	dead := &Combatant{HitPoints: HitPoints{Maximum: 20}, Status: CombatantDead}
	dead.Heal(10)
	if dead.Status != CombatantDead || dead.HitPoints.Current != 0 {
		t.Error("healing revived a dead combatant")
	}
}

func TestStartTurnResetsPerTurnResources(t *testing.T) {
	c := &Combatant{
		ActionUsed: true, BonusActionUsed: true, ReactionUsed: true,
		MovementRemaining: 0, Speed: 30,
	}
	c.StartTurn()

	if c.ActionUsed || c.BonusActionUsed || c.ReactionUsed {
		t.Error("action economy not reset at the start of the turn")
	}
	if c.MovementRemaining != 30 {
		t.Errorf("movement = %d, want the full speed of 30", c.MovementRemaining)
	}
}

func TestConditionsAreAClosedSet(t *testing.T) {
	if len(Conditions) != 14 {
		t.Errorf("got %d flag conditions, want 14 (exhaustion is tracked by level)", len(Conditions))
	}
	if !ConditionProne.Valid() || !ConditionParalyzed.Valid() {
		t.Error("a known condition failed validation")
	}
	if Condition("inspired").Valid() {
		t.Error("an invented condition passed validation")
	}
}

func TestCharacterConditionHelpers(t *testing.T) {
	c := &Character{}

	c.AddCondition(ConditionPoisoned)
	c.AddCondition(ConditionPoisoned) // duplicate
	if len(c.Conditions) != 1 {
		t.Errorf("conditions = %v, want one entry", c.Conditions)
	}
	if !c.HasCondition(ConditionPoisoned) {
		t.Error("HasCondition did not see the applied condition")
	}

	c.RemoveCondition(ConditionPoisoned)
	if c.HasCondition(ConditionPoisoned) {
		t.Error("condition survived removal")
	}
}

func TestCarryingCapacityIsStrengthTimesFifteen(t *testing.T) {
	c := &Character{AbilityScores: AbilityScores{Strength: 14}}
	if got, want := c.CarryingCapacity(), 210; got != want {
		t.Errorf("CarryingCapacity = %d, want %d", got, want)
	}
}

func TestProficiencyBonusForChallengeRating(t *testing.T) {
	cases := map[float64]int{0: 2, 0.25: 2, 4: 2, 5: 3, 8: 3, 9: 4, 17: 6, 21: 7}
	for cr, want := range cases {
		if got := ProficiencyBonusForCR(cr); got != want {
			t.Errorf("ProficiencyBonusForCR(%v) = %d, want %d", cr, got, want)
		}
	}
}

package models

import (
	"strings"
	"testing"
)

func longsword() InventoryItem {
	return InventoryItem{
		ItemID: "i1", Key: "longsword", Name: "Longsword", Kind: ItemWeapon, Weight: 3,
		Weapon: &WeaponProperties{
			Category: WeaponMartial, DamageDice: "1d8", DamageType: DamageSlashing,
			Properties: []WeaponProperty{PropertyVersatile}, VersatileDice: "1d10",
		},
	}
}

func shortbow() InventoryItem {
	return InventoryItem{
		ItemID: "i2", Key: "shortbow", Name: "Shortbow", Kind: ItemWeapon, Weight: 2,
		Weapon: &WeaponProperties{
			Category: WeaponSimple, DamageDice: "1d6", DamageType: DamagePiercing,
			Properties:  []WeaponProperty{PropertyAmmunition, PropertyTwoHanded},
			RangeNormal: 80, RangeLong: 320,
		},
	}
}

// The proficiency bonus is added only when the character is trained with the
// weapon; the ability modifier applies either way.
func TestAttackBonusDependsOnProficiency(t *testing.T) {
	c := &Character{
		BasicInfo:     levels(ClassFighter, 5), // proficiency +3
		AbilityScores: AbilityScores{Strength: 18, Dexterity: 14},
		Proficiencies: Proficiencies{Weapons: []string{ProfSimpleWeapons, ProfMartialWeapons}},
	}

	profile, err := c.AttackWith(longsword())
	if err != nil {
		t.Fatalf("AttackWith: %v", err)
	}
	// STR +4 and proficiency +3.
	if profile.AttackBonus != 7 {
		t.Errorf("attack bonus = %+d, want +7", profile.AttackBonus)
	}
	if profile.DamageBonus != 4 {
		t.Errorf("damage bonus = %+d, want +4 (no proficiency on damage)", profile.DamageBonus)
	}
	if got := profile.DamageExpression(); got != "1d8+4" {
		t.Errorf("damage = %q, want 1d8+4", got)
	}

	// Untrained, the proficiency bonus falls away but the attack still works.
	untrained := &Character{
		BasicInfo:     levels(ClassWizard, 5),
		AbilityScores: AbilityScores{Strength: 18},
	}
	profile, _ = untrained.AttackWith(longsword())
	if profile.Proficient {
		t.Error("a wizard should not be proficient with a longsword")
	}
	if profile.AttackBonus != 4 {
		t.Errorf("untrained attack bonus = %+d, want +4", profile.AttackBonus)
	}
}

// A named weapon proficiency covers that weapon without covering its whole
// category, which is how rogues get rapiers but not martial weapons.
func TestNamedWeaponProficiency(t *testing.T) {
	rogue := &Character{
		BasicInfo:     levels(ClassRogue, 3),
		AbilityScores: AbilityScores{Dexterity: 16},
		Proficiencies: Proficiencies{Weapons: []string{ProfSimpleWeapons, "rapier", "longsword"}},
	}

	if !rogue.Proficiencies.HasWeapon(longsword()) {
		t.Error("a longsword named outright should be proficient")
	}

	greataxe := InventoryItem{
		Key: "greataxe", Name: "Greataxe", Kind: ItemWeapon,
		Weapon: &WeaponProperties{Category: WeaponMartial, DamageDice: "1d12"},
	}
	if rogue.Proficiencies.HasWeapon(greataxe) {
		t.Error("a martial weapon not named should not be proficient")
	}
}

func TestRangedAttackUsesDexterityAndReportsRange(t *testing.T) {
	c := &Character{
		BasicInfo:     levels(ClassRanger, 5),
		AbilityScores: AbilityScores{Strength: 8, Dexterity: 18},
		Proficiencies: Proficiencies{Weapons: []string{ProfSimpleWeapons}},
	}

	profile, err := c.AttackWith(shortbow())
	if err != nil {
		t.Fatalf("AttackWith: %v", err)
	}
	if profile.Ability != AbilityDexterity {
		t.Errorf("ability = %q, want dexterity", profile.Ability)
	}
	if profile.AttackBonus != 7 { // DEX +4, proficiency +3
		t.Errorf("attack bonus = %+d, want +7", profile.AttackBonus)
	}
	if got := profile.RangeDescription(); got != "80/320 ft." {
		t.Errorf("range = %q, want 80/320 ft.", got)
	}
}

func TestMeleeReachAndVersatile(t *testing.T) {
	c := &Character{
		BasicInfo:     levels(ClassFighter, 1),
		AbilityScores: AbilityScores{Strength: 16},
		Proficiencies: Proficiencies{Weapons: []string{ProfMartialWeapons}},
	}

	profile, _ := c.AttackWith(longsword())
	if got := profile.RangeDescription(); got != "5 ft." {
		t.Errorf("melee reach = %q, want 5 ft.", got)
	}
	if profile.VersatileDice != "1d10" {
		t.Errorf("versatile dice = %q, want 1d10", profile.VersatileDice)
	}

	halberd := InventoryItem{
		Key: "halberd", Name: "Halberd", Kind: ItemWeapon,
		Weapon: &WeaponProperties{Category: WeaponMartial, DamageDice: "1d10",
			Properties: []WeaponProperty{PropertyReach, PropertyHeavy, PropertyTwoHanded}},
	}
	profile, _ = c.AttackWith(halberd)
	if got := profile.RangeDescription(); got != "10 ft." {
		t.Errorf("reach weapon = %q, want 10 ft.", got)
	}
}

func TestAttackWithNonWeaponFails(t *testing.T) {
	c := &Character{BasicInfo: levels(ClassFighter, 1)}
	rope := InventoryItem{Name: "Rope", Kind: ItemGear}

	if _, err := c.AttackWith(rope); err == nil {
		t.Error("attacking with rope should fail")
	}
}

func TestArmorProficiencyDetection(t *testing.T) {
	plate := InventoryItem{Name: "Plate", Kind: ItemArmor,
		Armor: &ArmorProperties{Category: ArmorHeavy, BaseAC: 18}}

	fighter := &Character{
		Proficiencies: Proficiencies{Armor: []string{ProfLightArmor, ProfMediumArmor, ProfHeavyArmor, ProfShields}},
		Equipment:     Equipment{Armor: &plate},
	}
	if !fighter.IsArmorProficient() {
		t.Error("a fighter in plate should be proficient")
	}

	wizard := &Character{Equipment: Equipment{Armor: &plate}}
	if wizard.IsArmorProficient() {
		t.Error("a wizard in plate should not be proficient")
	}

	// A shield needs its own proficiency.
	shield := InventoryItem{Name: "Shield", Kind: ItemShield, Armor: &ArmorProperties{}}
	noShield := &Character{
		Proficiencies: Proficiencies{Armor: []string{ProfLightArmor}},
		Equipment:     Equipment{Shield: &shield},
	}
	if noShield.IsArmorProficient() {
		t.Error("carrying a shield without shield proficiency should not be proficient")
	}
}

// Multiclassing never grants everything a first level in the class would.
func TestMulticlassProficienciesAreReduced(t *testing.T) {
	first := ClassProficiencies(ClassFighter, true)
	if !contains(first.Armor, ProfHeavyArmor) {
		t.Error("a fighter taken first should have heavy armour")
	}

	later := ClassProficiencies(ClassFighter, false)
	if contains(later.Armor, ProfHeavyArmor) {
		t.Error("a fighter taken as a multiclass should not gain heavy armour")
	}
	if !contains(later.Armor, ProfMediumArmor) {
		t.Error("a fighter taken as a multiclass should still gain medium armour")
	}

	// Wizards and sorcerers grant nothing when multiclassed into.
	if len(ClassProficiencies(ClassWizard, false).Armor) != 0 {
		t.Error("multiclassing into wizard should grant no armour")
	}
}

// Heavy armour below its Strength requirement costs 10 feet; exhaustion halves
// speed at 2 and removes it at 5.
func TestSpeedPenalties(t *testing.T) {
	plate := InventoryItem{Name: "Plate", Kind: ItemArmor,
		Armor: &ArmorProperties{Category: ArmorHeavy, BaseAC: 18, StrengthRequirement: 15}}

	strong := &Character{
		BasicInfo:     BasicInfo{Race: RaceHuman, Subrace: "standard"},
		AbilityScores: AbilityScores{Strength: 16},
		Equipment:     Equipment{Armor: &plate},
	}
	if got := strong.Speed(); got != 30 {
		t.Errorf("speed with sufficient Strength = %d, want 30", got)
	}

	weak := &Character{
		BasicInfo:     BasicInfo{Race: RaceHuman, Subrace: "standard"},
		AbilityScores: AbilityScores{Strength: 12},
		Equipment:     Equipment{Armor: &plate},
	}
	if got := weak.Speed(); got != 20 {
		t.Errorf("speed under the Strength requirement = %d, want 20", got)
	}

	tired := &Character{BasicInfo: BasicInfo{Race: RaceHuman, Subrace: "standard"}, Exhaustion: 2}
	if got := tired.Speed(); got != 15 {
		t.Errorf("speed at exhaustion 2 = %d, want 15", got)
	}

	exhausted := &Character{BasicInfo: BasicInfo{Race: RaceHuman, Subrace: "standard"}, Exhaustion: 5}
	if got := exhausted.Speed(); got != 0 {
		t.Errorf("speed at exhaustion 5 = %d, want 0", got)
	}

	// Dwarves are slow to begin with, and the penalties compound.
	dwarf := &Character{
		BasicInfo:     BasicInfo{Race: RaceDwarf, Subrace: "hill"},
		AbilityScores: AbilityScores{Strength: 10},
		Equipment:     Equipment{Armor: &plate},
	}
	if got := dwarf.Speed(); got != 15 {
		t.Errorf("dwarf under the Strength requirement = %d, want 15", got)
	}
}

func TestExhaustionEffectsAccumulate(t *testing.T) {
	if e := ExhaustionEffectsFor(0); e != (ExhaustionEffects{}) {
		t.Errorf("no exhaustion should impose nothing, got %+v", e)
	}

	e := ExhaustionEffectsFor(3)
	if !e.DisadvantageOnAbilityChecks || !e.SpeedHalved || !e.DisadvantageOnAttacksAndSaves {
		t.Errorf("level 3 should carry every effect below it, got %+v", e)
	}
	if e.HitPointMaximumHalved {
		t.Error("hit point maximum is not halved until level 4")
	}

	if !ExhaustionEffectsFor(6).Dead {
		t.Error("level 6 exhaustion is death")
	}

	c := &Character{
		CombatStats: CombatStats{HitPoints: HitPoints{Maximum: 40}},
		Exhaustion:  4,
	}
	if got := c.EffectiveHitPointMaximum(); got != 20 {
		t.Errorf("effective maximum at exhaustion 4 = %d, want 20", got)
	}
}

func TestSkillRollModeFromArmorAndExhaustion(t *testing.T) {
	stealthy := InventoryItem{Name: "Chain Mail", Kind: ItemArmor,
		Armor: &ArmorProperties{Category: ArmorHeavy, BaseAC: 16, StealthDisadvantage: true}}

	c := &Character{Equipment: Equipment{Armor: &stealthy}}
	if got := c.SkillRollMode(SkillStealth); got != RollDisadvantage {
		t.Errorf("Stealth in noisy armour = %s, want disadvantage", got)
	}
	if got := c.SkillRollMode(SkillAthletics); got != RollNormal {
		t.Errorf("Athletics in noisy armour = %s, want normal", got)
	}

	tired := &Character{Exhaustion: 1}
	if got := tired.SkillRollMode(SkillAthletics); got != RollDisadvantage {
		t.Errorf("any check at exhaustion 1 = %s, want disadvantage", got)
	}
	if got := tired.SavingThrowRollMode(AbilityDexterity); got != RollNormal {
		t.Errorf("saves at exhaustion 1 = %s, want normal until level 3", got)
	}
	if got := (&Character{Exhaustion: 3}).SavingThrowRollMode(AbilityDexterity); got != RollDisadvantage {
		t.Errorf("saves at exhaustion 3 = %s, want disadvantage", got)
	}
}

func TestCurrencyConversionAndWeight(t *testing.T) {
	purse := Currency{Copper: 5, Silver: 3, Gold: 2, Platinum: 1}

	// 5 + 30 + 200 + 1000
	if got := purse.TotalInCopper(); got != 1235 {
		t.Errorf("total = %d cp, want 1235", got)
	}
	if got := purse.TotalInGold(); got != 12.35 {
		t.Errorf("total = %v gp, want 12.35", got)
	}
	// 11 coins at 50 to the pound.
	if got := purse.Weight(); got != 0.22 {
		t.Errorf("weight = %v lb, want 0.22", got)
	}
	if got := purse.String(); got != "1pp 2gp 3sp 5cp" {
		t.Errorf("String() = %q, want %q", got, "1pp 2gp 3sp 5cp")
	}
}

// Coins are fungible: paying 3 gp out of a purse of silver is fine.
func TestCurrencySpendMakesChange(t *testing.T) {
	purse := Currency{Silver: 50} // 500 cp

	if err := purse.Spend(Currency{Gold: 3}); err != nil {
		t.Fatalf("spending 3gp from 50sp: %v", err)
	}
	if got := purse.TotalInCopper(); got != 200 {
		t.Errorf("remaining = %d cp, want 200", got)
	}
	// Change settles into the largest denominations that fit.
	if purse.Gold != 2 || purse.Silver != 0 || purse.Copper != 0 {
		t.Errorf("change = %s, want 2gp", purse.String())
	}

	if err := purse.Spend(Currency{Gold: 5}); err == nil {
		t.Error("overspending should fail")
	}
	if got := purse.TotalInCopper(); got != 200 {
		t.Errorf("a failed spend changed the purse to %d cp", got)
	}
}

func TestEncumbranceCountsInventoryAndCoins(t *testing.T) {
	c := &Character{
		AbilityScores: AbilityScores{Strength: 10}, // capacity 150
		Currency:      Currency{Gold: 100},         // 2 lb
		Inventory: []InventoryItem{
			{Name: "Rations", Weight: 2, Quantity: 10}, // 20 lb
			{Name: "Plate", Weight: 65, Quantity: 1},
		},
	}

	if got := c.CarriedWeight(); got != 87 {
		t.Errorf("carried weight = %v lb, want 87", got)
	}
	if c.IsOverloaded() {
		t.Error("87 lb against a 150 lb capacity is not overloaded")
	}
	if got := c.CarryingCapacity(); got != 150 {
		t.Errorf("capacity = %d, want 150", got)
	}
	if got := c.PushDragLiftCapacity(); got != 300 {
		t.Errorf("push/drag/lift = %d, want 300", got)
	}

	c.Inventory = append(c.Inventory, InventoryItem{Name: "Anvil", Weight: 100, Quantity: 1})
	if !c.IsOverloaded() {
		t.Error("187 lb against a 150 lb capacity is overloaded")
	}
}

func TestAttunementLimit(t *testing.T) {
	c := &Character{Inventory: []InventoryItem{
		{ItemID: "a", Name: "Ring of Protection", RequiresAttunement: true},
		{ItemID: "b", Name: "Cloak of Elvenkind", RequiresAttunement: true},
		{ItemID: "c", Name: "Boots of Speed", RequiresAttunement: true},
		{ItemID: "d", Name: "Wand of Magic Missiles", RequiresAttunement: true},
		{ItemID: "e", Name: "Rope"},
	}}

	for _, id := range []string{"a", "b", "c"} {
		if err := c.Attune(id); err != nil {
			t.Fatalf("attuning %s: %v", id, err)
		}
	}
	if got := len(c.AttunedItems()); got != MaxAttunedItems {
		t.Errorf("attuned to %d items, want %d", got, MaxAttunedItems)
	}

	err := c.Attune("d")
	if err == nil {
		t.Fatal("a fourth attunement should be refused")
	}
	if !strings.Contains(err.Error(), "maximum") {
		t.Errorf("error %q does not explain the limit", err)
	}

	// Releasing one makes room.
	if err := c.EndAttunement("a"); err != nil {
		t.Fatalf("ending attunement: %v", err)
	}
	if err := c.Attune("d"); err != nil {
		t.Errorf("attuning after releasing one: %v", err)
	}

	// An item that needs no attunement cannot be attuned.
	if err := c.Attune("e"); err == nil {
		t.Error("attuning to a mundane rope should fail")
	}
}

func TestShortRestRestoresPactSlotsAndShortRestFeatures(t *testing.T) {
	two := 2
	one := 1
	c := &Character{
		BasicInfo: BasicInfo{Classes: []ClassLevel{
			{Class: ClassWarlock, Subclass: "fiend", Level: 5},
		}},
		Spells: Spells{PactSlots: SpellSlot{Level: 3, Total: 2, Expended: 2}},
		FeaturesAndAbilities: []Feature{
			{Name: "Action Surge", UsesPerDay: &one, UsesSpent: 1, Recharge: RechargeShortRest},
			{Name: "Divine Sense", UsesPerDay: &two, UsesSpent: 2, Recharge: RechargeLongRest},
		},
	}

	c.ShortRest()

	if got := c.Spells.AvailablePactSlots(); got != 2 {
		t.Errorf("pact slots after a short rest = %d, want 2", got)
	}
	if c.FeaturesAndAbilities[0].UsesSpent != 0 {
		t.Error("Action Surge should return on a short rest")
	}
	if c.FeaturesAndAbilities[1].UsesSpent != 2 {
		t.Error("Divine Sense should not return on a short rest")
	}

	// A long rest covers both.
	c.LongRest()
	if c.FeaturesAndAbilities[1].UsesSpent != 0 {
		t.Error("Divine Sense should return on a long rest")
	}
}

func TestSpendHitDieHealsWithConstitution(t *testing.T) {
	c := &Character{
		AbilityScores: AbilityScores{Constitution: 16}, // +3
		CombatStats: CombatStats{
			HitPoints: HitPoints{Current: 4, Maximum: 40},
			HitDice:   HitDice{{Die: 10, Total: 5, Spent: 0}},
		},
	}

	if err := c.SpendHitDie(10, 6); err != nil {
		t.Fatalf("SpendHitDie: %v", err)
	}
	if got := c.CombatStats.HitPoints.Current; got != 13 {
		t.Errorf("hit points = %d, want 13 (4 + 6 rolled + 3 CON)", got)
	}
	if got := c.CombatStats.HitDice.Available(); got != 4 {
		t.Errorf("hit dice remaining = %d, want 4", got)
	}

	// A die the character does not have.
	if err := c.SpendHitDie(6, 3); err == nil {
		t.Error("spending a d6 should fail for a d10 character")
	}
}

// A negative Constitution modifier can cancel the roll, but never damages.
func TestSpendHitDieNeverDealsDamage(t *testing.T) {
	c := &Character{
		AbilityScores: AbilityScores{Constitution: 6}, // -2
		CombatStats: CombatStats{
			HitPoints: HitPoints{Current: 5, Maximum: 20},
			HitDice:   HitDice{{Die: 6, Total: 3}},
		},
	}

	if err := c.SpendHitDie(6, 1); err != nil {
		t.Fatalf("SpendHitDie: %v", err)
	}
	if got := c.CombatStats.HitPoints.Current; got != 5 {
		t.Errorf("hit points = %d, want 5 (healing floors at zero, never damages)", got)
	}
}

func TestPactSlotsReconcileFromClassLevels(t *testing.T) {
	c := &Character{BasicInfo: BasicInfo{Classes: []ClassLevel{
		{Class: ClassWarlock, Subclass: "archfey", Level: 5},
	}}}

	c.ReconcileSpellSlots()

	if c.Spells.PactSlots.Total != 2 || c.Spells.PactSlots.Level != 3 {
		t.Errorf("pact slots = %+v, want 2 at level 3", c.Spells.PactSlots)
	}
	if err := c.Spells.ExpendPactSlot(); err != nil {
		t.Fatalf("expending a pact slot: %v", err)
	}
	if got := c.Spells.AvailablePactSlots(); got != 1 {
		t.Errorf("available pact slots = %d, want 1", got)
	}

	// A non-warlock has none to spend.
	wizard := &Character{BasicInfo: levels(ClassWizard, 5)}
	wizard.ReconcileSpellSlots()
	if err := wizard.Spells.ExpendPactSlot(); err == nil {
		t.Error("a wizard should have no pact slots")
	}
}

func TestApplyClassDefaultsPopulatesProficienciesAndLanguages(t *testing.T) {
	c := &Character{
		BasicInfo: BasicInfo{
			Race: RaceDwarf, Subrace: "mountain",
			Background: BackgroundSoldier,
			Classes: []ClassLevel{
				{Class: ClassFighter, Subclass: "champion", Level: 3},
				{Class: ClassRogue, Subclass: "thief", Level: 3},
			},
		},
		AbilityScores: AbilityScores{Strength: 16, Dexterity: 14},
	}

	c.ApplyClassDefaults()

	// Fighter first: heavy armour and martial weapons.
	if !c.Proficiencies.HasArmor(ArmorHeavy) {
		t.Error("fighter taken first should grant heavy armour")
	}
	if !contains(c.Proficiencies.Weapons, ProfMartialWeapons) {
		t.Error("fighter should grant martial weapons")
	}
	// Rogue second: thieves' tools from the reduced multiclass set.
	if !c.Proficiencies.HasTool("thieves_tools") {
		t.Error("rogue multiclass should grant thieves' tools")
	}
	// Background tools and racial languages.
	if !c.Proficiencies.HasTool("gaming_set") {
		t.Error("soldier background should grant a gaming set")
	}
	for _, lang := range []string{"common", "dwarvish"} {
		if !c.Proficiencies.SpeaksLanguage(lang) {
			t.Errorf("a dwarf should speak %s", lang)
		}
	}

	// And the attack maths works end to end off those proficiencies.
	profile, err := c.AttackWith(longsword())
	if err != nil {
		t.Fatalf("AttackWith: %v", err)
	}
	// STR +3, proficiency +3 at total level 6.
	if profile.AttackBonus != 6 {
		t.Errorf("attack bonus = %+d, want +6", profile.AttackBonus)
	}
}

func TestXPThresholds(t *testing.T) {
	cases := map[int]int{0: 1, 299: 1, 300: 2, 900: 3, 6499: 4, 6500: 5, 355000: 20, 999999: 20}
	for xp, want := range cases {
		if got := LevelForXP(xp); got != want {
			t.Errorf("LevelForXP(%d) = %d, want %d", xp, got, want)
		}
	}

	if got := XPForLevel(5); got != 6500 {
		t.Errorf("XPForLevel(5) = %d, want 6500", got)
	}

	remaining, more := XPToNextLevel(300)
	if !more || remaining != 600 {
		t.Errorf("XPToNextLevel(300) = %d, %v; want 600, true", remaining, more)
	}
	if _, more := XPToNextLevel(355000); more {
		t.Error("level 20 should have no next level")
	}
}

func TestAbilityScoreImprovementLevels(t *testing.T) {
	if got := len(AbilityScoreImprovementLevels(ClassFighter)); got != 7 {
		t.Errorf("fighter has %d ASIs, want 7", got)
	}
	if got := len(AbilityScoreImprovementLevels(ClassRogue)); got != 6 {
		t.Errorf("rogue has %d ASIs, want 6", got)
	}
	if got := len(AbilityScoreImprovementLevels(ClassWizard)); got != 5 {
		t.Errorf("wizard has %d ASIs, want 5", got)
	}
}

func TestValidateSheetRejectsScoreAboveTwenty(t *testing.T) {
	c := newValidCharacter()
	c.AbilityScores.Dexterity = 22

	err := c.ValidateSheet()
	if err == nil || !strings.Contains(err.Error(), "maximum") {
		t.Errorf("a score of 22 should be rejected, got %v", err)
	}
}

func TestFeatureUses(t *testing.T) {
	two := 2
	f := Feature{Name: "Second Wind", UsesPerDay: &two, Recharge: RechargeShortRest}

	if left, limited := f.UsesRemaining(); !limited || left != 2 {
		t.Errorf("remaining = %d (limited=%v), want 2, true", left, limited)
	}
	if err := f.Use(); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if err := f.Use(); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if err := f.Use(); err == nil {
		t.Error("a third use should fail")
	}

	// An unlimited feature never runs out.
	unlimited := Feature{Name: "Darkvision"}
	if _, limited := unlimited.UsesRemaining(); limited {
		t.Error("a feature with no cap should not report as limited")
	}
	if err := unlimited.Use(); err != nil {
		t.Errorf("using an unlimited feature: %v", err)
	}
}

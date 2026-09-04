package models

import (
	"fmt"
	"strings"
)

// GrantedSkills returns the skill proficiencies a character receives without
// choosing them: those from their race, subrace and background.
//
// Class skills are chosen from a list rather than granted, so they are not
// included; ValidateSheet checks those against the class list instead.
func (c *Character) GrantedSkills() []Skill {
	seen := map[Skill]bool{}
	var out []Skill

	add := func(skills []Skill) {
		for _, s := range skills {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}

	add(c.BasicInfo.Race.GrantedSkills(c.BasicInfo.Subrace))
	add(c.BasicInfo.Background.GrantedSkills())
	return out
}

// AllowedClassSkills returns every skill the character's classes could have
// offered, which is the pool their chosen proficiencies must come from.
//
// A nil result means "any skill": the bard chooses freely, so no constraint
// can be checked.
func (c *Character) AllowedClassSkills() ([]Skill, bool) {
	seen := map[Skill]bool{}
	var out []Skill

	for _, cl := range c.BasicInfo.Classes {
		def, ok := cl.Class.Definition()
		if !ok {
			continue
		}
		if def.SkillList == nil {
			return nil, false // unconstrained
		}
		for _, s := range def.SkillList {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out, true
}

// ValidateSheet checks a character sheet for internal consistency.
//
// This is deliberately separate from the repository's required-field checks:
// those keep a document storable, while this asks whether the numbers on it
// could describe a legal 5e character. Call it at creation and on level up;
// existing sheets predating a rules change should not be blocked from saving.
func (c *Character) ValidateSheet() error {
	var problems []string

	if !c.BasicInfo.Race.Valid() {
		problems = append(problems, fmt.Sprintf("unknown race %q", c.BasicInfo.Race))
	} else {
		switch {
		case c.BasicInfo.Race.RequiresSubrace() && c.BasicInfo.Subrace == "":
			problems = append(problems, fmt.Sprintf("race %q requires a subrace", c.BasicInfo.Race))
		case c.BasicInfo.Subrace != "" && !c.BasicInfo.Race.HasSubrace(c.BasicInfo.Subrace):
			problems = append(problems, fmt.Sprintf("%q is not a subrace of %q", c.BasicInfo.Subrace, c.BasicInfo.Race))
		}
	}

	if !c.BasicInfo.Background.Valid() {
		problems = append(problems, fmt.Sprintf("unknown background %q", c.BasicInfo.Background))
	}

	if len(c.BasicInfo.Classes) == 0 {
		problems = append(problems, "character has no class levels")
	}

	total := 0
	seenClass := map[Class]bool{}
	for _, cl := range c.BasicInfo.Classes {
		def, ok := cl.Class.Definition()
		if !ok {
			problems = append(problems, fmt.Sprintf("unknown class %q", cl.Class))
			continue
		}
		if cl.Level < 1 {
			problems = append(problems, fmt.Sprintf("%s has level %d, want at least 1", cl.Class, cl.Level))
		}
		if seenClass[cl.Class] {
			problems = append(problems, fmt.Sprintf("class %q listed more than once", cl.Class))
		}
		seenClass[cl.Class] = true
		total += cl.Level

		// The archetype is only chosen once the class reaches its subclass
		// level, so an earlier one is as wrong as an unknown one.
		switch {
		case cl.Subclass == "" && cl.Level >= def.SubclassLevel:
			problems = append(problems, fmt.Sprintf("%s %d must choose a subclass at level %d",
				cl.Class, cl.Level, def.SubclassLevel))
		case cl.Subclass != "" && !cl.Class.HasSubclass(cl.Subclass):
			problems = append(problems, fmt.Sprintf("%q is not a %s archetype", cl.Subclass, cl.Class))
		case cl.Subclass != "" && cl.Level < def.SubclassLevel:
			problems = append(problems, fmt.Sprintf("%s does not choose an archetype until level %d",
				cl.Class, def.SubclassLevel))
		}

		// Multiclass prerequisites apply to every class after the first, and
		// to the first class as well once a second is taken.
		if len(c.BasicInfo.Classes) > 1 && !cl.Class.MeetsMulticlassPrerequisites(c.AbilityScores) {
			problems = append(problems, fmt.Sprintf("multiclassing into %s requires %s",
				cl.Class, cl.Class.DescribePrerequisites()))
		}
	}

	if total > 20 {
		problems = append(problems, fmt.Sprintf("total level is %d, want at most 20", total))
	}

	// Skill proficiencies must be granted by race or background, or drawn
	// from a class list.
	if allowed, constrained := c.AllowedClassSkills(); constrained {
		permitted := map[Skill]bool{}
		for _, s := range c.GrantedSkills() {
			permitted[s] = true
		}
		for _, s := range allowed {
			permitted[s] = true
		}
		for skill, level := range c.Skills {
			if level == ProficiencyNone {
				continue
			}
			if !skill.Valid() {
				problems = append(problems, fmt.Sprintf("unknown skill %q", skill))
				continue
			}
			if !permitted[skill] {
				problems = append(problems, fmt.Sprintf(
					"%s proficiency is not granted by %s, %s or any of the character's classes",
					skill, c.BasicInfo.Race, c.BasicInfo.Background))
			}
		}
	}

	// The number of skills must fit the budget from race, background and
	// class. Checking only that each skill has a source let a rogue claim
	// every skill on their list.
	proficientSkills, expertiseSkills := 0, 0
	for _, level := range c.Skills {
		if level == ProficiencyNone {
			continue
		}
		proficientSkills++
		if level == ProficiencyExpertise {
			expertiseSkills++
		}
	}
	if budget := c.SkillBudget(); proficientSkills > budget {
		problems = append(problems, fmt.Sprintf(
			"%d skill proficiencies but only %d are granted by race, background and class",
			proficientSkills, budget))
	}
	// Expertise may also be spent on tools, so the budget is a ceiling.
	if budget := c.ExpertiseBudget(); expertiseSkills > budget {
		problems = append(problems, fmt.Sprintf(
			"%d skills with expertise but only %d expertise choices earned", expertiseSkills, budget))
	}

	for _, a := range Abilities {
		if score := c.AbilityScores.Score(a); score > MaxAbilityScore {
			problems = append(problems, fmt.Sprintf("%s is %d, above the maximum of %d",
				a, score, MaxAbilityScore))
		}
	}

	for _, cond := range c.Conditions {
		if !cond.Valid() {
			problems = append(problems, fmt.Sprintf("unknown condition %q", cond))
		}
	}
	if c.Exhaustion < 0 || c.Exhaustion > MaxExhaustion {
		problems = append(problems, fmt.Sprintf("exhaustion is %d, want 0-%d", c.Exhaustion, MaxExhaustion))
	}

	if len(problems) > 0 {
		return Invalid("invalid character sheet: %s", strings.Join(problems, "; "))
	}
	return nil
}

// ApplyClassDefaults fills in what the character's classes determine, without
// overwriting anything already chosen.
//
// It sets the first class's saving throw proficiencies, rebuilds the hit dice
// pools, and reconciles spell slots against the class tables. Use it at
// creation and on level up rather than typing the numbers in.
func (c *Character) ApplyClassDefaults() {
	if len(c.BasicInfo.Classes) == 0 {
		return
	}

	// Only the first class grants saving throw proficiencies.
	if c.SavingThrows == nil {
		c.SavingThrows = SavingThrowProficiencies{}
	}
	for _, a := range c.GrantedSaveProficiencies() {
		if c.SavingThrows.Level(a) == ProficiencyNone {
			c.SavingThrows[a] = ProficiencyProficient
		}
	}

	// Race and background skills are granted, not chosen.
	if c.Skills == nil {
		c.Skills = SkillProficiencies{}
	}
	for _, s := range c.GrantedSkills() {
		if c.Skills.Level(s) == ProficiencyNone {
			c.Skills[s] = ProficiencyProficient
		}
	}

	// Proficiencies come from the first class in full and from later classes
	// in the reduced multiclass set, plus the background's tools and the
	// race's languages.
	for i, cl := range c.BasicInfo.Classes {
		c.Proficiencies.Merge(ClassProficiencies(cl.Class, i == 0))
	}
	if def, ok := c.BasicInfo.Background.Definition(); ok {
		c.Proficiencies.Tools = addUnique(c.Proficiencies.Tools, def.ToolProficiencies...)
	}
	c.Proficiencies.Languages = addUnique(c.Proficiencies.Languages,
		c.BasicInfo.Race.GrantedLanguages()...)

	// Racial training is a real proficiency grant, not just a trait name: a
	// mountain dwarf is trained in medium armour and an elf with a longsword.
	c.Proficiencies.Merge(c.BasicInfo.Race.RacialProficiencies(c.BasicInfo.Subrace))

	c.CombatStats.HitDice = c.ExpectedHitDice()

	if cl, ok := c.SpellcastingClass(); ok {
		ability, _ := cl.Casting()
		c.Spells.SpellcastingAbility = ability
		c.Spells.SpellcastingClass = string(cl.Class)
	}
	c.ReconcileSpellSlots()
}

// ReconcileSpellSlots brings the character's slots in line with what their
// class levels entitle them to, preserving how many are already expended.
//
// Levelling up adds slots; it never refunds ones already spent, so a wizard
// who levels mid-adventuring-day does not get a free rest out of it.
func (c *Character) ReconcileSpellSlots() {
	expected, _, _ := c.ExpectedSpellSlots()

	expendedAt := map[int]int{}
	for _, slot := range c.Spells.Slots {
		expendedAt[slot.Level] = slot.Expended
	}

	for i := range expected {
		if spent := expendedAt[expected[i].Level]; spent > 0 {
			if spent > expected[i].Total {
				spent = expected[i].Total
			}
			expected[i].Expended = spent
		}
	}
	c.Spells.Slots = expected

	_, pactCount, pactLevel := c.ExpectedSpellSlots()
	expendedPact := c.Spells.PactSlots.Expended
	if expendedPact > pactCount {
		expendedPact = pactCount
	}
	c.Spells.PactSlots = SpellSlot{Level: pactLevel, Total: pactCount, Expended: expendedPact}
}

// rechargeFeatures resets the limited-use features that return at a given rest.
func (c *Character) rechargeFeatures(at FeatureRecharge) {
	for i := range c.FeaturesAndAbilities {
		if c.FeaturesAndAbilities[i].Recharge == at {
			c.FeaturesAndAbilities[i].UsesSpent = 0
		}
	}
}

// SpendHitDie spends one hit die and applies the healing it rolled.
//
// The roll is supplied rather than made here: the model owns the rules, not
// the randomness. Healing is the rolled value plus the Constitution modifier,
// and a total below one still restores nothing rather than dealing damage.
func (c *Character) SpendHitDie(die, rolled int) error {
	if err := c.CombatStats.HitDice.Spend(die); err != nil {
		return err
	}

	healing := rolled + c.AbilityModifier(AbilityConstitution)
	if healing < 0 {
		healing = 0
	}
	c.CombatStats.HitPoints.Heal(healing)
	return nil
}

// ShortRest restores what an hour's rest returns on its own: warlock pact
// slots and every short-rest feature.
//
// Hit dice are deliberately not spent here. Spending them is a player choice
// made one die at a time, with a roll for each -- see SpendHitDie.
func (c *Character) ShortRest() {
	c.Spells.RestorePactSlots()
	c.rechargeFeatures(RechargeShortRest)
}

// LongRest restores hit points, returns every spell slot, gives back half the
// character's hit dice and reduces exhaustion by one.
func (c *Character) LongRest() {
	c.CombatStats.HitPoints.Current = c.EffectiveHitPointMaximum()
	c.CombatStats.HitPoints.Temporary = 0
	c.CombatStats.DeathSaves.Reset()
	c.CombatStats.HitDice.RegainOnLongRest()
	c.Spells.RestoreAllSlots()
	c.Spells.RestorePactSlots()

	// A long rest also covers everything a short rest would.
	c.rechargeFeatures(RechargeShortRest)
	c.rechargeFeatures(RechargeLongRest)

	if c.Exhaustion > 0 {
		c.Exhaustion--
	}
}

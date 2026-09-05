package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// CastResult is everything one casting produced.
//
// It is separate from SpellCastResult because a cast is not one roll: Eldritch
// Blast and Scorching Ray fire several projectiles, each its own attack, and
// collapsing them into a single hit-or-miss would be a different spell.
type CastResult struct {
	Caster     string                 `json:"caster"`
	Spell      string                 `json:"spell"`
	SlotLevel  int                    `json:"slot_level"`
	Resolution models.SpellResolution `json:"resolution"`
	Target     string                 `json:"target"`

	// Attacks holds one entry per projectile. Empty for a save or automatic
	// spell, which roll no attack at all.
	Attacks []AttackResult `json:"attacks,omitempty"`
	Hits    int            `json:"hits"`

	// Save is the target's saving throw, for a spell they resist.
	Save *CheckResult `json:"save,omitempty"`

	// Damage is the total across every projectile that landed.
	Damage  *DamageResult `json:"damage,omitempty"`
	Healing int           `json:"healing,omitempty"`

	Condition        models.Condition `json:"condition,omitempty"`
	ConditionApplied bool             `json:"condition_applied,omitempty"`

	Projectiles  int                    `json:"projectiles"`
	TargetStatus models.CombatantStatus `json:"target_status"`
	TargetHP     models.HitPoints       `json:"target_hit_points"`
}

// Summary is the sentence the narration must not contradict.
func (r CastResult) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s casts %s", r.Caster, r.Spell)
	if r.SlotLevel > 0 {
		fmt.Fprintf(&b, " at level %d", r.SlotLevel)
	}

	switch {
	case len(r.Attacks) > 0:
		if r.Projectiles > 1 {
			fmt.Fprintf(&b, " at %s: %d of %d land", r.Target, r.Hits, r.Projectiles)
		} else if r.Hits > 0 {
			fmt.Fprintf(&b, " and hits %s", r.Target)
		} else {
			fmt.Fprintf(&b, " at %s and misses", r.Target)
		}
	case r.Save != nil:
		if r.Save.AutomaticFailure {
			fmt.Fprintf(&b, "; %s cannot resist and automatically fails the DC %d %s save",
				r.Target, r.Save.DC, r.Save.Ability)
			break
		}
		outcome := "fails"
		if r.Save.Succeeded() {
			outcome = "succeeds on"
		}
		fmt.Fprintf(&b, "; %s %s a DC %d %s save", r.Target, outcome, r.Save.DC, r.Save.Ability)
	case r.Healing > 0:
		fmt.Fprintf(&b, " on %s", r.Target)
	}

	if r.Damage != nil && r.Damage.Dealt > 0 {
		fmt.Fprintf(&b, " for %d %s damage", r.Damage.Dealt, r.Damage.Type)
	}
	if r.Healing > 0 {
		fmt.Fprintf(&b, ", restoring %d hit points", r.Healing)
	}
	if r.ConditionApplied {
		fmt.Fprintf(&b, "; %s is %s", r.Target, r.Condition)
	}
	if r.TargetStatus == models.CombatantDead {
		fmt.Fprintf(&b, "; %s dies", r.Target)
	} else if r.TargetStatus == models.CombatantDying {
		fmt.Fprintf(&b, "; %s falls unconscious", r.Target)
	}
	return b.String()
}

// Facts is the complete set of values a narration prompt may reference.
//
// Every entry is non-empty even when nothing landed: a blank value in a prompt
// reads as an invitation to invent one.
func (r CastResult) Facts() map[string]string {
	outcome := "no effect"
	switch {
	case len(r.Attacks) > 0 && r.Hits > 0:
		outcome = "hit"
	case len(r.Attacks) > 0:
		outcome = "miss"
	case r.Save != nil && r.Save.Succeeded():
		outcome = "resisted"
	case r.Save != nil && r.Save.AutomaticFailure:
		outcome = "could not resist at all"
	case r.Save != nil:
		outcome = "failed to resist"
	case r.Healing > 0:
		outcome = "healed"
	case r.Damage != nil:
		outcome = "hit"
	}

	// Every key is always present. A narration template names the variables
	// it needs, and BuildPrompt refuses a missing one -- so a fact that
	// appears only sometimes would make a spell with no save unnarratable.
	facts := map[string]string{
		"caster":          r.Caster,
		"spell":           r.Spell,
		"slot_level":      strconv.Itoa(r.SlotLevel),
		"target":          orUnknown(r.Target),
		"outcome":         outcome,
		"projectiles":     strconv.Itoa(r.Projectiles),
		"hits":            strconv.Itoa(r.Hits),
		"damage_total":    "0",
		"damage_type":     "none",
		"healing":         strconv.Itoa(r.Healing),
		"condition":       "none",
		"damage_affinity": "normal",
		"save_ability":    "none",
		"save_dc":         "0",
		"save_total":      "none",
		"save_automatic":  "no",
		"target_hp":       fmt.Sprintf("%d/%d", r.TargetHP.Current, r.TargetHP.Maximum),
		"target_status":   string(r.TargetStatus),
		"fact_summary":    r.Summary(),
	}
	if r.Damage != nil {
		facts["damage_total"] = strconv.Itoa(r.Damage.Dealt)
		facts["damage_type"] = string(r.Damage.Type)
		facts["damage_affinity"] = string(r.Damage.Affinity)
	}
	if r.ConditionApplied {
		facts["condition"] = string(r.Condition)
	}
	if r.Save != nil {
		facts["save_ability"] = string(r.Save.Ability)
		facts["save_dc"] = strconv.Itoa(r.Save.DC)
		facts["save_automatic"] = boolText(r.Save.AutomaticFailure)
		// A save that was never rolled reports no total; a 0 would read as a
		// roll and the narration would describe one.
		if !r.Save.AutomaticFailure {
			facts["save_total"] = strconv.Itoa(r.Save.Roll.Total)
		}
	}
	return facts
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "no one in particular"
	}
	return s
}

// casterLevel is the level a cantrip scales against.
func casterLevel(c *models.Character) int {
	if level := c.BasicInfo.TotalLevel(); level > 0 {
		return level
	}
	return 1
}

// prepareCast validates the cast and takes the slot.
//
// The slot is taken here, once, before anything is rolled: a spell that misses
// still costs the slot, and a spell that fires four rays still costs only one.
// A refused cast must spend nothing, which is why every check happens first.
func (e *Engine) prepareCast(caster *models.Character, def models.SpellDefinition, slotLevel int) error {
	if err := def.ValidateSlot(slotLevel); err != nil {
		return err
	}
	if def.Resolution == models.SpellResolutionUtility {
		return models.Invalid(
			"%s has no roll for the engine to make; describe it rather than resolving it", def.Name)
	}
	return e.spendSlot(caster, slotLevel)
}

// CastSpell resolves a spell that hits automatically or with attack rolls.
//
// A spell the target resists goes through CastSpellVersusSave instead, because
// that needs the target's saving throw modifier -- routing one through the
// other is refused rather than quietly resolved the wrong way.
func (e *Engine) CastSpell(
	caster *models.Character,
	def models.SpellDefinition,
	slotLevel int,
	target *models.Combatant,
	situational models.RollMode,
) (CastResult, error) {
	if def.Resolution == models.SpellResolutionSave {
		return CastResult{}, models.Invalid(
			"%s is resisted with a saving throw; resolve it with CastSpellVersusSave", def.Name)
	}
	if err := e.prepareCast(caster, def, slotLevel); err != nil {
		return CastResult{}, err
	}

	level := casterLevel(caster)
	projectiles := def.ProjectilesAt(slotLevel, level)
	result := CastResult{
		Caster: caster.Name, Spell: def.Name, SlotLevel: slotLevel,
		Resolution: def.Resolution, Target: target.Name,
		Projectiles: projectiles, Condition: def.Condition,
	}

	switch def.Resolution {
	case models.SpellResolutionAttack:
		if err := e.castAttackSpell(caster, def, slotLevel, level, projectiles, target, situational, &result); err != nil {
			return CastResult{}, err
		}

	case models.SpellResolutionAuto:
		if err := e.castAutomaticSpell(caster, def, slotLevel, level, projectiles, target, &result); err != nil {
			return CastResult{}, err
		}
	}

	result.TargetStatus = target.Status
	result.TargetHP = target.HitPoints
	return result, nil
}

// castAttackSpell rolls one attack per projectile and sums what landed.
func (e *Engine) castAttackSpell(
	caster *models.Character,
	def models.SpellDefinition,
	slotLevel, level, projectiles int,
	target *models.Combatant,
	situational models.RollMode,
	result *CastResult,
) error {
	bonus := caster.SpellAttackModifier()
	perProjectile := def.DamageAt(slotLevel, level)

	// A spell attack is still an attack: the caster's own conditions and the
	// target's both apply. Range 0 on the definition is a touch spell, which is
	// melee and close enough for the helpless rule.
	melee := def.Range == 0
	mode := caster.SpellAttackRollMode().
		Combine(target.DefenderAttackMode(melee)).
		Combine(situational)

	for i := 0; i < projectiles; i++ {
		roll := e.roller.D20(bonus, mode)
		outcome := models.ResolveAttack(roll, target.ArmorClass, caster.CritRange())
		outcome = upgradeToCritical(outcome, target, melee)

		attack := AttackResult{
			Attacker: caster.Name, Target: target.Name, Weapon: def.Name,
			AttackBonus: bonus, Roll: roll,
			TargetAC: target.ArmorClass, CritRange: caster.CritRange(),
			Outcome: outcome,
		}

		if outcome.Hit() && !perProjectile.IsZero() {
			damage, err := e.applyDamage(target, perProjectile.String(), def.DamageType,
				outcome == models.AttackCritical)
			if err != nil {
				return err
			}
			attack.Damage = damage
			addDamage(result, damage)
			result.Hits++
		}

		attack.TargetStatus = target.Status
		attack.TargetHP = target.HitPoints
		result.Attacks = append(result.Attacks, attack)
	}

	// A condition rides on a hit for an attack spell.
	if result.Hits > 0 && def.Condition != "" {
		applied, err := e.ApplyCondition(target, def.Condition)
		if err != nil {
			return err
		}
		result.ConditionApplied = applied
	}
	return nil
}

// castAutomaticSpell resolves a spell that cannot miss: Magic Missile's darts,
// Cure Wounds' healing, Heat Metal's burn.
func (e *Engine) castAutomaticSpell(
	caster *models.Character,
	def models.SpellDefinition,
	slotLevel, level, projectiles int,
	target *models.Combatant,
	result *CastResult,
) error {
	if def.Heals() {
		healing := def.HealingAt(slotLevel)
		rolled, err := e.roller.RollDamage(healing.String(), false)
		if err != nil {
			return err
		}
		amount := rolled.Total
		if def.AddsAbilityModifier {
			amount += caster.AbilityModifier(caster.Spells.SpellcastingAbility)
		}
		if amount < 0 {
			amount = 0
		}

		// Report what was actually restored, not what was rolled: healing
		// stops at the maximum and a narration saying otherwise is wrong.
		before := target.HitPoints.Current
		if amount > 0 {
			if err := e.Heal(target, amount); err != nil {
				return err
			}
		}
		result.Healing = target.HitPoints.Current - before
	}

	if def.DealsDamage() {
		// Every projectile lands, so they roll together: three darts of 1d4+1
		// is 3d4+3 against a single target, which is exactly RAW.
		total := def.DamageAt(slotLevel, level).Times(projectiles)
		damage, err := e.applyDamage(target, total.String(), def.DamageType, false)
		if err != nil {
			return err
		}
		addDamage(result, damage)
		result.Hits = projectiles
	}

	if def.Condition != "" {
		applied, err := e.ApplyCondition(target, def.Condition)
		if err != nil {
			return err
		}
		result.ConditionApplied = applied
	}
	return nil
}

// CastSpellVersusSave resolves a spell the target resists.
//
// A successful save halves the damage when the spell says so and negates it
// otherwise, and a spell that imposes a condition imposes it only on a failure.
func (e *Engine) CastSpellVersusSave(
	caster *models.Character,
	def models.SpellDefinition,
	slotLevel int,
	target *models.Combatant,
	targetSaveModifier int,
) (CastResult, error) {
	if def.Resolution != models.SpellResolutionSave {
		return CastResult{}, models.Invalid(
			"%s is not resisted with a saving throw; resolve it with CastSpell", def.Name)
	}
	if err := e.prepareCast(caster, def, slotLevel); err != nil {
		return CastResult{}, err
	}

	dc := caster.SpellSaveDC()

	// A helpless creature does not roll Strength or Dexterity saves at all --
	// it fails them. This is most of what Hold Person is for, and skipping the
	// roll matters: rolling and then discarding the result would consume a die
	// from a scripted sequence and make the outcome look decided when it was
	// not.
	automatic := target.AutoFailsSave(def.SaveAbility)

	save := CheckResult{
		Kind: KindSavingThrow, Actor: target.Name, Ability: def.SaveAbility,
		Modifier: targetSaveModifier, DC: dc, AutomaticFailure: automatic,
	}
	if automatic {
		save.Outcome = models.OutcomeFailure
		save.Margin = 0
	} else {
		mode := models.RollNormal
		if def.SaveAbility == models.AbilityDexterity && target.HasCondition(models.ConditionRestrained) {
			mode = models.RollDisadvantage
		}
		roll := e.roller.D20(targetSaveModifier, mode)
		save.Roll = roll
		save.Outcome = models.ResolveCheck(roll, dc)
		save.Margin = roll.Total - dc
	}

	result := CastResult{
		Caster: caster.Name, Spell: def.Name, SlotLevel: slotLevel,
		Resolution: def.Resolution, Target: target.Name,
		Save: &save, Projectiles: 1, Condition: def.Condition,
	}

	if def.DealsDamage() {
		expression := def.DamageAt(slotLevel, casterLevel(caster))
		rolled, err := e.roller.RollDamage(expression.String(), false)
		if err != nil {
			return CastResult{}, err
		}

		amount := rolled.Total
		if save.Succeeded() {
			if def.HalfOnSave {
				amount /= 2
			} else {
				amount = 0
			}
		}

		affinity := target.AffinityTo(def.DamageType)
		dealt := target.TakeDamage(amount, def.DamageType, false)
		result.Damage = &DamageResult{
			Expression: rolled.Expression.String(),
			Rolls:      rolled.Rolls,
			Modifier:   rolled.Modifier,
			Rolled:     amount,
			Type:       def.DamageType,
			Affinity:   affinity,
			Dealt:      dealt,
		}
		if !save.Succeeded() {
			result.Hits = 1
		}
	}

	// The condition lands only on a failed save. Immunity is reported rather
	// than assumed, or the narration describes a paralysed creature that is
	// still swinging.
	if !save.Succeeded() && def.Condition != "" {
		applied, err := e.ApplyCondition(target, def.Condition)
		if err != nil {
			return CastResult{}, err
		}
		result.ConditionApplied = applied
	}

	result.TargetStatus = target.Status
	result.TargetHP = target.HitPoints
	return result, nil
}

// addDamage folds one projectile's damage into the cast's running total.
func addDamage(result *CastResult, damage *DamageResult) {
	if result.Damage == nil {
		copied := *damage
		result.Damage = &copied
		return
	}
	result.Damage.Rolled += damage.Rolled
	result.Damage.Dealt += damage.Dealt
	result.Damage.Rolls = append(result.Damage.Rolls, damage.Rolls...)
	if damage.Critical {
		result.Damage.Critical = true
	}
}

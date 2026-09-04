package rules

import (
	"strconv"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// Move spends a combatant's movement.
//
// Movement is a per-turn budget rather than a flag, so a creature can step,
// act, and step again -- which is how disengaging and kiting work at a table.
func (e *Engine) Move(c *models.Combatant, feet int) error {
	if feet <= 0 {
		return models.Invalid("movement must be positive")
	}
	if c.Status == models.CombatantDead || c.IsDown() {
		return models.Invalid("%s cannot move", c.Name)
	}
	if c.HasCondition(models.ConditionGrappled) || c.HasCondition(models.ConditionRestrained) {
		return models.Invalid("%s is held fast and cannot move", c.Name)
	}
	if feet > c.MovementRemaining {
		return models.Invalid("%s has %d feet of movement left, not %d", c.Name, c.MovementRemaining, feet)
	}

	c.MovementRemaining -= feet
	return nil
}

// ApplyCondition puts a condition on a combatant, honouring immunity.
//
// Reporting whether it stuck matters: a narration that says a creature is
// frightened when it is immune to fear contradicts the state.
func (e *Engine) ApplyCondition(c *models.Combatant, cond models.Condition) (applied bool, err error) {
	if !cond.Valid() {
		return false, models.Invalid("unknown condition %q", cond)
	}
	if c.ImmuneToCondition(cond) {
		return false, nil
	}
	return c.AddCondition(cond), nil
}

// RemoveCondition clears a condition.
func (e *Engine) RemoveCondition(c *models.Combatant, cond models.Condition) {
	for i, have := range c.Conditions {
		if have == cond {
			c.Conditions = append(c.Conditions[:i], c.Conditions[i+1:]...)
			return
		}
	}
}

// SpellCastResult is the outcome of casting a spell in combat.
type SpellCastResult struct {
	Caster    string `json:"caster"`
	Spell     string `json:"spell"`
	SlotLevel int    `json:"slot_level"`

	// Attack is set for a spell resolved with an attack roll, Save for one
	// the target resists. A spell that does neither leaves both nil.
	Attack *AttackResult `json:"attack,omitempty"`
	Save   *CheckResult  `json:"save,omitempty"`

	Damage *DamageResult `json:"damage,omitempty"`

	TargetStatus models.CombatantStatus `json:"target_status"`
	TargetHP     models.HitPoints       `json:"target_hit_points"`
}

// Summary is the engine's sentence for a cast.
func (r SpellCastResult) Summary() string {
	switch {
	case r.Attack != nil:
		return r.Caster + " casts " + r.Spell + ": " + r.Attack.Summary()
	case r.Save != nil:
		text := r.Caster + " casts " + r.Spell + "; " + r.Save.Summary()
		if r.Damage != nil {
			text += " and takes " + strconv.Itoa(r.Damage.Dealt) + " " + string(r.Damage.Type) + " damage"
		}
		return text
	default:
		return r.Caster + " casts " + r.Spell
	}
}

// Facts returns the values a narration prompt may reference.
func (r SpellCastResult) Facts() map[string]string {
	facts := map[string]string{
		"attacker":          r.Caster,
		"weapon":            r.Spell,
		"target":            "-",
		"roll_mode":         string(models.RollNormal),
		"natural":           "0",
		"all_rolls":         "-",
		"attack_bonus":      "+0",
		"attack_total":      "0",
		"target_ac":         "0",
		"outcome":           "cast",
		"hit":               "no",
		"critical":          "no",
		"damage_total":      "0",
		"damage_type":       "-",
		"damage_expression": "-",
		"damage_affinity":   string(models.AffinityNormal),
		"target_status":     string(r.TargetStatus),
		"target_hp":         strconv.Itoa(r.TargetHP.Current) + "/" + strconv.Itoa(r.TargetHP.Maximum),
		"fact_summary":      r.Summary(),
	}

	if r.Attack != nil {
		for key, value := range r.Attack.Facts() {
			facts[key] = value
		}
		facts["weapon"] = r.Spell
		facts["fact_summary"] = r.Summary()
	}
	if r.Save != nil {
		facts["target"] = r.Save.Actor
		facts["outcome"] = string(r.Save.Outcome)
		facts["natural"] = strconv.Itoa(r.Save.Roll.Natural)
		facts["attack_total"] = strconv.Itoa(r.Save.Roll.Total)
		facts["target_ac"] = strconv.Itoa(r.Save.DC)
	}
	if r.Damage != nil {
		facts["damage_total"] = strconv.Itoa(r.Damage.Dealt)
		facts["damage_type"] = string(r.Damage.Type)
		facts["damage_expression"] = r.Damage.Expression
		facts["damage_affinity"] = string(r.Damage.Affinity)
	}
	return facts
}

// SpellAttack resolves a spell that hits with an attack roll.
//
// The slot is spent before anything is rolled: a spell that misses still costs
// the slot, and taking it afterwards would refund every failure.
func (e *Engine) SpellAttack(
	caster *models.Character,
	spellName string,
	slotLevel int,
	damageExpression string,
	damageType models.DamageType,
	target *models.Combatant,
	situational models.RollMode,
) (SpellCastResult, error) {
	if err := e.spendSlot(caster, slotLevel); err != nil {
		return SpellCastResult{}, err
	}

	bonus := caster.SpellAttackModifier()
	roll := e.roller.D20(bonus, situational)
	outcome := models.ResolveAttack(roll, target.ArmorClass, caster.CritRange())

	attack := AttackResult{
		Attacker: caster.Name, Target: target.Name, Weapon: spellName,
		AttackBonus: bonus, Roll: roll,
		TargetAC: target.ArmorClass, CritRange: caster.CritRange(),
		Outcome: outcome,
	}

	result := SpellCastResult{Caster: caster.Name, Spell: spellName, SlotLevel: slotLevel}

	if outcome.Hit() && damageExpression != "" {
		damage, err := e.applyDamage(target, damageExpression, damageType,
			outcome == models.AttackCritical)
		if err != nil {
			return SpellCastResult{}, err
		}
		attack.Damage = damage
		result.Damage = damage
	}

	attack.TargetStatus = target.Status
	attack.TargetHP = target.HitPoints
	result.Attack = &attack
	result.TargetStatus = target.Status
	result.TargetHP = target.HitPoints
	return result, nil
}

// SpellSave resolves a spell the target resists with a saving throw.
//
// A successful save halves the damage rather than negating it, which is the
// common case; a spell that negates entirely passes halfOnSuccess = false.
func (e *Engine) SpellSave(
	caster *models.Character,
	spellName string,
	slotLevel int,
	saveAbility models.Ability,
	damageExpression string,
	damageType models.DamageType,
	halfOnSuccess bool,
	target *models.Combatant,
	targetSaveModifier int,
) (SpellCastResult, error) {
	if err := e.spendSlot(caster, slotLevel); err != nil {
		return SpellCastResult{}, err
	}

	dc := caster.SpellSaveDC()
	roll := e.roller.D20(targetSaveModifier, models.RollNormal)

	save := CheckResult{
		Kind: KindSavingThrow, Actor: target.Name, Ability: saveAbility,
		Modifier: targetSaveModifier, Roll: roll, DC: dc,
		Outcome: models.ResolveCheck(roll, dc),
		Margin:  roll.Total - dc,
	}

	result := SpellCastResult{
		Caster: caster.Name, Spell: spellName, SlotLevel: slotLevel, Save: &save,
	}

	if damageExpression != "" {
		rolled, err := e.roller.RollDamage(damageExpression, false)
		if err != nil {
			return SpellCastResult{}, err
		}

		amount := rolled.Total
		if save.Succeeded() {
			if !halfOnSuccess {
				amount = 0
			} else {
				amount /= 2
			}
		}

		affinity := target.AffinityTo(damageType)
		dealt := target.TakeDamage(amount, damageType, false)

		result.Damage = &DamageResult{
			Expression: rolled.Expression.String(),
			Rolls:      rolled.Rolls,
			Modifier:   rolled.Modifier,
			Rolled:     amount,
			Type:       damageType,
			Affinity:   affinity,
			Dealt:      dealt,
		}
	}

	result.TargetStatus = target.Status
	result.TargetHP = target.HitPoints
	return result, nil
}

// spendSlot takes the spell slot a cast requires. Cantrips (level 0) are free.
func (e *Engine) spendSlot(caster *models.Character, slotLevel int) error {
	if slotLevel <= 0 {
		return nil
	}
	return caster.Spells.ExpendSlot(slotLevel)
}

// DeathSaveResult is one death saving throw and what it changed.
type DeathSaveResult struct {
	Combatant string                 `json:"combatant"`
	Roll      models.D20Result       `json:"roll"`
	Saves     models.DeathSaves      `json:"death_saves"`
	Status    models.CombatantStatus `json:"status"`
	Regained  bool                   `json:"regained_hit_point"`
}

// Summary is the engine's sentence for a death save.
func (r DeathSaveResult) Summary() string {
	switch {
	case r.Regained:
		return r.Combatant + " rolls a natural 20 and comes back with 1 hit point"
	case r.Status == models.CombatantDead:
		return r.Combatant + " fails a third death save and dies"
	case r.Status == models.CombatantStable:
		return r.Combatant + " stabilises"
	default:
		return r.Combatant + " rolls a death save: " +
			strconv.Itoa(r.Saves.Successes) + " successes, " +
			strconv.Itoa(r.Saves.Failures) + " failures"
	}
}

// DeathSave rolls a death saving throw for a dying combatant.
//
// Three successes stabilise, three failures kill, and a natural 20 puts the
// creature back on its feet with a single hit point.
func (e *Engine) DeathSave(c *models.Combatant) (DeathSaveResult, error) {
	if c.Status != models.CombatantDying {
		return DeathSaveResult{}, models.Invalid("%s is not making death saves", c.Name)
	}

	roll, regained := e.roller.DeathSave(&c.DeathSaves)

	switch {
	case regained:
		c.HitPoints.Current = 1
		c.Status = models.CombatantActive
	case c.DeathSaves.Dead():
		c.Status = models.CombatantDead
	case c.DeathSaves.Stabilised():
		c.Status = models.CombatantStable
	}

	return DeathSaveResult{
		Combatant: c.Name, Roll: roll, Saves: c.DeathSaves,
		Status: c.Status, Regained: regained,
	}, nil
}

// Stabilise steadies a dying creature without healing it, as a successful
// Medicine check does.
func (e *Engine) Stabilise(c *models.Combatant) error {
	if c.Status != models.CombatantDying {
		return models.Invalid("%s is not dying", c.Name)
	}
	c.Status = models.CombatantStable
	c.DeathSaves.Reset()
	return nil
}

// Heal restores hit points to a combatant.
//
// Any healing at all brings a downed creature back to consciousness and clears
// its death saves; the dead need more than hit points.
func (e *Engine) Heal(c *models.Combatant, amount int) error {
	if amount <= 0 {
		return models.Invalid("healing must be positive")
	}
	if c.Status == models.CombatantDead {
		return models.Invalid("%s is dead and beyond healing", c.Name)
	}
	c.Heal(amount)
	return nil
}

// Revive returns a dead creature to life with one hit point, as raise dead
// does. It is a separate method from Heal on purpose: bringing back the dead
// should never be something a healing spell does by accident.
func (e *Engine) Revive(c *models.Combatant) error {
	if c.Status != models.CombatantDead {
		return models.Invalid("%s is not dead", c.Name)
	}
	c.Status = models.CombatantActive
	c.DeathSaves.Reset()
	c.HitPoints.Current = 1
	return nil
}

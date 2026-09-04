// Package rules resolves D&D 5e actions.
//
// The engine is authoritative: it decides what happened. Everything it returns
// is a statement of fact, and the narration layer's job is to describe those
// facts, never to change or contradict them. That boundary is why every result
// type carries a Facts map -- it is the exact set of values a prompt may use.
package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
)

// Engine resolves checks, saves and attacks.
type Engine struct {
	roller *dice.Roller
}

// NewEngine creates an engine backed by a roller.
func NewEngine(roller *dice.Roller) *Engine {
	if roller == nil {
		roller = dice.New()
	}
	return &Engine{roller: roller}
}

// Roller exposes the underlying roller for callers that need a raw roll.
func (e *Engine) Roller() *dice.Roller { return e.roller }

// CheckKind distinguishes the three d20 tests that share a shape.
type CheckKind string

const (
	KindAbilityCheck CheckKind = "ability_check"
	KindSkillCheck   CheckKind = "skill_check"
	KindSavingThrow  CheckKind = "saving_throw"
)

// CheckResult is the outcome of an ability check, skill check or saving throw.
type CheckResult struct {
	Kind    CheckKind      `json:"kind"`
	Actor   string         `json:"actor"`
	Ability models.Ability `json:"ability"`
	Skill   models.Skill   `json:"skill,omitempty"`

	Modifier int              `json:"modifier"`
	Roll     models.D20Result `json:"roll"`
	DC       int              `json:"dc"`

	Outcome models.CheckOutcome `json:"outcome"`
	// Margin is how far the total cleared or missed the DC, which is what
	// separates "just barely" from "comfortably".
	Margin int `json:"margin"`
}

// Succeeded reports whether the check met its DC.
func (r CheckResult) Succeeded() bool { return r.Outcome == models.OutcomeSuccess }

// Summary is a one-line statement of what happened, in the engine's words.
//
// The narration layer may dress this up but must not contradict it.
func (r CheckResult) Summary() string {
	label := string(r.Ability)
	if r.Skill != "" {
		label = string(r.Skill)
	}

	verb := "fails"
	if r.Succeeded() {
		verb = "succeeds on"
	}

	return fmt.Sprintf("%s %s a DC %d %s %s (rolled %s = %d)",
		r.Actor, verb, r.DC, label, strings.ReplaceAll(string(r.Kind), "_", " "),
		formatRolls(r.Roll), r.Roll.Total)
}

// Facts returns the values a narration prompt may reference.
//
// Nothing outside this map is established, so a prompt built from it cannot
// invent a result the engine did not produce.
func (r CheckResult) Facts() map[string]string {
	skill := string(r.Skill)
	if skill == "" {
		skill = "-"
	}
	return map[string]string{
		"check_kind":   string(r.Kind),
		"actor":        r.Actor,
		"ability":      string(r.Ability),
		"skill":        skill,
		"dc":           strconv.Itoa(r.DC),
		"roll_mode":    string(r.Roll.Mode),
		"natural":      strconv.Itoa(r.Roll.Natural),
		"all_rolls":    formatRolls(r.Roll),
		"modifier":     withSign(r.Modifier),
		"total":        strconv.Itoa(r.Roll.Total),
		"outcome":      string(r.Outcome),
		"margin":       strconv.Itoa(r.Margin),
		"was_close":    boolText(abs(r.Margin) <= 2),
		"fact_summary": r.Summary(),
	}
}

// AttackResult is the outcome of one attack.
type AttackResult struct {
	Attacker string `json:"attacker"`
	Target   string `json:"target"`
	Weapon   string `json:"weapon"`

	AttackBonus int              `json:"attack_bonus"`
	Roll        models.D20Result `json:"roll"`
	TargetAC    int              `json:"target_ac"`
	CritRange   int              `json:"crit_range"`

	Outcome models.AttackOutcome `json:"outcome"`
	Damage  *DamageResult        `json:"damage,omitempty"`

	// TargetStatus is the target's state after the attack resolved, so a
	// narration knows whether it just dropped someone.
	TargetStatus models.CombatantStatus `json:"target_status"`
	TargetHP     models.HitPoints       `json:"target_hit_points"`
}

// Hit reports whether the attack connected.
func (r AttackResult) Hit() bool { return r.Outcome.Hit() }

// DamageResult is what a hit actually cost the target.
type DamageResult struct {
	Expression string                `json:"expression"`
	Rolls      []int                 `json:"rolls"`
	Modifier   int                   `json:"modifier"`
	Rolled     int                   `json:"rolled"`
	Type       models.DamageType     `json:"type"`
	Critical   bool                  `json:"critical"`
	Affinity   models.DamageAffinity `json:"affinity"`
	// Dealt is what the target lost after resistance or immunity. It differs
	// from Rolled often enough that reporting only one loses the explanation.
	Dealt int `json:"dealt"`
}

// Summary is a one-line statement of the attack, in the engine's words.
func (r AttackResult) Summary() string {
	switch r.Outcome {
	case models.AttackFumble:
		return fmt.Sprintf("%s misses %s outright with %s (natural 1)", r.Attacker, r.Target, r.Weapon)
	case models.AttackMiss:
		return fmt.Sprintf("%s misses %s with %s (rolled %d against AC %d)",
			r.Attacker, r.Target, r.Weapon, r.Roll.Total, r.TargetAC)
	}

	text := fmt.Sprintf("%s hits %s with %s (rolled %d against AC %d)",
		r.Attacker, r.Target, r.Weapon, r.Roll.Total, r.TargetAC)
	if r.Outcome == models.AttackCritical {
		text = fmt.Sprintf("%s scores a critical hit on %s with %s (natural %d)",
			r.Attacker, r.Target, r.Weapon, r.Roll.Natural)
	}
	if r.Damage != nil {
		text += fmt.Sprintf(" for %d %s damage", r.Damage.Dealt, r.Damage.Type)
		switch r.Damage.Affinity {
		case models.AffinityResistant:
			text += " (resisted, halved)"
		case models.AffinityImmune:
			text += " (immune, no damage)"
		case models.AffinityVulnerable:
			text += " (vulnerable, doubled)"
		}
	}
	switch r.TargetStatus {
	case models.CombatantDead:
		text += fmt.Sprintf("; %s dies", r.Target)
	case models.CombatantDying:
		text += fmt.Sprintf("; %s drops to 0 hit points and is dying", r.Target)
	}
	return text
}

// Facts returns the values a narration prompt may reference.
func (r AttackResult) Facts() map[string]string {
	facts := map[string]string{
		"attacker":      r.Attacker,
		"target":        r.Target,
		"weapon":        r.Weapon,
		"roll_mode":     string(r.Roll.Mode),
		"natural":       strconv.Itoa(r.Roll.Natural),
		"all_rolls":     formatRolls(r.Roll),
		"attack_bonus":  withSign(r.AttackBonus),
		"attack_total":  strconv.Itoa(r.Roll.Total),
		"target_ac":     strconv.Itoa(r.TargetAC),
		"outcome":       string(r.Outcome),
		"hit":           boolText(r.Hit()),
		"critical":      boolText(r.Outcome == models.AttackCritical),
		"target_status": string(r.TargetStatus),
		"target_hp":     fmt.Sprintf("%d/%d", r.TargetHP.Current, r.TargetHP.Maximum),
		"fact_summary":  r.Summary(),

		// Present but empty on a miss, so every template variable always has
		// a value and BuildPrompt never rejects the call.
		"damage_total":      "0",
		"damage_type":       "-",
		"damage_expression": "-",
		"damage_affinity":   string(models.AffinityNormal),
	}

	if r.Damage != nil {
		facts["damage_total"] = strconv.Itoa(r.Damage.Dealt)
		facts["damage_type"] = string(r.Damage.Type)
		facts["damage_expression"] = r.Damage.Expression
		facts["damage_affinity"] = string(r.Damage.Affinity)
	}
	return facts
}

// ---------------------------------------------------------------------------
// Resolution
// ---------------------------------------------------------------------------

// SkillCheck resolves a skill check for a character.
//
// The character's own state -- exhaustion, armour that hampers Stealth --
// is folded into the roll mode alongside whatever the situation adds.
func (e *Engine) SkillCheck(c *models.Character, skill models.Skill, dc int, situational models.RollMode) CheckResult {
	modifier := c.SkillModifier(skill)
	mode := c.SkillRollMode(skill).Combine(situational)
	roll := e.roller.D20(modifier, mode)

	return CheckResult{
		Kind: KindSkillCheck, Actor: c.Name,
		Ability: skill.Ability(), Skill: skill,
		Modifier: modifier, Roll: roll, DC: dc,
		Outcome: models.ResolveCheck(roll, dc),
		Margin:  roll.Total - dc,
	}
}

// AbilityCheck resolves a raw ability check, with no skill attached.
func (e *Engine) AbilityCheck(c *models.Character, ability models.Ability, dc int, situational models.RollMode) CheckResult {
	modifier := c.AbilityModifier(ability)

	mode := situational
	if c.ExhaustionEffects().DisadvantageOnAbilityChecks {
		mode = mode.Combine(models.RollDisadvantage)
	}
	roll := e.roller.D20(modifier, mode)

	return CheckResult{
		Kind: KindAbilityCheck, Actor: c.Name, Ability: ability,
		Modifier: modifier, Roll: roll, DC: dc,
		Outcome: models.ResolveCheck(roll, dc),
		Margin:  roll.Total - dc,
	}
}

// SavingThrow resolves a saving throw for a character.
func (e *Engine) SavingThrow(c *models.Character, ability models.Ability, dc int, situational models.RollMode) CheckResult {
	modifier := c.SavingThrowModifier(ability)
	mode := c.SavingThrowRollMode(ability).Combine(situational)
	roll := e.roller.D20(modifier, mode)

	return CheckResult{
		Kind: KindSavingThrow, Actor: c.Name, Ability: ability,
		Modifier: modifier, Roll: roll, DC: dc,
		Outcome: models.ResolveCheck(roll, dc),
		Margin:  roll.Total - dc,
	}
}

// MonsterSavingThrow resolves a saving throw for a monster in combat.
func (e *Engine) MonsterSavingThrow(m *models.Monster, ability models.Ability, dc int, situational models.RollMode) CheckResult {
	modifier := m.SavingThrowModifier(ability)
	roll := e.roller.D20(modifier, situational)

	return CheckResult{
		Kind: KindSavingThrow, Actor: m.Name, Ability: ability,
		Modifier: modifier, Roll: roll, DC: dc,
		Outcome: models.ResolveCheck(roll, dc),
		Margin:  roll.Total - dc,
	}
}

// WeaponAttack resolves a character's attack against a combatant.
//
// Damage is applied to the target, so the result reports what the target
// actually lost after resistance rather than what the dice showed.
func (e *Engine) WeaponAttack(
	attacker *models.Character,
	weapon models.InventoryItem,
	target *models.Combatant,
	situational models.RollMode,
) (AttackResult, error) {
	profile, err := attacker.AttackWith(weapon)
	if err != nil {
		return AttackResult{}, err
	}

	mode := profile.Mode.Combine(situational)
	roll := e.roller.D20(profile.AttackBonus, mode)
	outcome := models.ResolveAttack(roll, target.ArmorClass, profile.CritRange)

	result := AttackResult{
		Attacker: attacker.Name, Target: target.Name, Weapon: profile.Name,
		AttackBonus: profile.AttackBonus, Roll: roll,
		TargetAC: target.ArmorClass, CritRange: profile.CritRange,
		Outcome: outcome,
	}

	if outcome.Hit() {
		damage, err := e.applyDamage(target, profile.DamageExpression(), profile.DamageType,
			outcome == models.AttackCritical)
		if err != nil {
			return AttackResult{}, err
		}
		result.Damage = damage
	}

	result.TargetStatus = target.Status
	result.TargetHP = target.HitPoints
	return result, nil
}

// MonsterAttack resolves a monster's attack action against a combatant.
func (e *Engine) MonsterAttack(
	attacker *models.Monster,
	action models.MonsterAction,
	target *models.Combatant,
	situational models.RollMode,
) (AttackResult, error) {
	if action.AttackBonus == nil {
		return AttackResult{}, models.Invalid("action %q is not an attack", action.Name)
	}

	roll := e.roller.D20(*action.AttackBonus, situational)
	outcome := models.ResolveAttack(roll, target.ArmorClass, models.NaturalCrit)

	result := AttackResult{
		Attacker: attacker.Name, Target: target.Name, Weapon: action.Name,
		AttackBonus: *action.AttackBonus, Roll: roll,
		TargetAC: target.ArmorClass, CritRange: models.NaturalCrit,
		Outcome: outcome,
	}

	if outcome.Hit() && action.DamageDice != "" {
		damage, err := e.applyDamage(target, action.DamageDice, action.DamageType,
			outcome == models.AttackCritical)
		if err != nil {
			return AttackResult{}, err
		}
		result.Damage = damage
	}

	result.TargetStatus = target.Status
	result.TargetHP = target.HitPoints
	return result, nil
}

// applyDamage rolls damage and hands it to the target, recording both what was
// rolled and what was actually dealt.
func (e *Engine) applyDamage(
	target *models.Combatant,
	expression string,
	damageType models.DamageType,
	critical bool,
) (*DamageResult, error) {
	rolled, err := e.roller.RollDamage(expression, critical)
	if err != nil {
		return nil, err
	}

	affinity := target.AffinityTo(damageType)
	dealt := target.TakeDamage(rolled.Total, damageType, critical)

	return &DamageResult{
		Expression: rolled.Expression.String(),
		Rolls:      rolled.Rolls,
		Modifier:   rolled.Modifier,
		Rolled:     rolled.Total,
		Type:       damageType,
		Critical:   critical,
		Affinity:   affinity,
		Dealt:      dealt,
	}, nil
}

// ---------------------------------------------------------------------------

func formatRolls(r models.D20Result) string {
	parts := make([]string, len(r.Rolls))
	for i, roll := range r.Rolls {
		parts[i] = strconv.Itoa(roll)
	}
	joined := strings.Join(parts, ", ")
	if len(r.Rolls) > 1 {
		return fmt.Sprintf("%s keeping %d", joined, r.Natural)
	}
	return joined
}

func withSign(n int) string {
	if n >= 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func boolText(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

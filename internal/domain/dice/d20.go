package dice

import "github.com/dnd-campaign/manager/internal/domain/models"

// D20Sides is the die every check, save and attack in 5e is made with.
const D20Sides = 20

// D20 rolls a d20 with a modifier, honouring advantage and disadvantage.
//
// Both dice are kept in the result rather than only the one used: a log that
// shows a 7 hides that the player rolled 19 and 7 at disadvantage, which is
// most of what made the moment interesting.
func (r *Roller) D20(modifier int, mode models.RollMode) models.D20Result {
	result := models.D20Result{Mode: mode, Modifier: modifier}

	switch mode {
	case models.RollAdvantage, models.RollDisadvantage:
		first, second := r.die(D20Sides), r.die(D20Sides)
		result.Rolls = []int{first, second}

		keep := first
		if mode == models.RollAdvantage && second > keep {
			keep = second
		}
		if mode == models.RollDisadvantage && second < keep {
			keep = second
		}
		result.Natural = keep
	default:
		roll := r.die(D20Sides)
		result.Rolls = []int{roll}
		result.Natural = roll
	}

	result.Total = result.Natural + modifier
	return result
}

// RollDamage rolls a damage expression, doubling the dice on a critical hit.
//
// Only the dice double. The modifier is added once, which is the single most
// commonly misplayed part of a critical.
func (r *Roller) RollDamage(expression string, critical bool) (Result, error) {
	e, err := Parse(expression)
	if err != nil {
		return Result{}, err
	}
	if critical {
		e = e.Doubled()
	}

	result := r.RollExpression(e)
	// Damage never heals the target: a modifier low enough to take the total
	// below zero deals nothing instead.
	if result.Total < 0 {
		result.Total = 0
	}
	return result, nil
}

// DeathSave rolls a death saving throw and applies it to the tally.
//
// The rules it encodes: 10 or higher succeeds, a natural 20 restores a hit
// point outright, and a natural 1 counts as two failures.
func (r *Roller) DeathSave(saves *models.DeathSaves) (roll models.D20Result, regainsHitPoint bool) {
	roll = r.D20(0, models.RollNormal)

	switch {
	case roll.IsNatural20():
		saves.Reset()
		return roll, true
	case roll.IsNatural1():
		saves.Failures += 2
	case roll.Total >= 10:
		saves.Successes++
	default:
		saves.Failures++
	}

	if saves.Failures > models.DeathSaveThreshold {
		saves.Failures = models.DeathSaveThreshold
	}
	if saves.Successes > models.DeathSaveThreshold {
		saves.Successes = models.DeathSaveThreshold
	}
	return roll, false
}

// RollInitiative rolls initiative for a combatant and records it.
func (r *Roller) RollInitiative(c *models.Combatant) models.D20Result {
	roll := r.D20(c.InitiativeModifier, models.RollNormal)
	c.Initiative = roll.Total
	return roll
}

// RollHitDie rolls one hit die of the given size, as a short rest does.
func (r *Roller) RollHitDie(sides int) int {
	return r.die(sides)
}

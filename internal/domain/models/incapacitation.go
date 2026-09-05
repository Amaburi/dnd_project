package models

import "fmt"

// Whether a creature may act at all.
//
// These live on the model rather than in the engine for the reason the rest of
// the 5e rules here do: an engine that has to remember to check is an engine
// that will one day forget, and the failure is silent -- a character at 0 hit
// points swinging a sword reads perfectly normally in a log.

// IsDead reports whether the character is beyond acting or healing.
//
// Two roads lead here: three failed death saves, and the sixth level of
// exhaustion, which is the one level that is not merely a penalty.
func (c *Character) IsDead() bool {
	return c.Exhaustion >= MaxExhaustion || c.CombatStats.DeathSaves.Dead()
}

// IsDown reports whether the character is at zero hit points, and so
// unconscious whether they are dying or stabilised.
func (c *Character) IsDown() bool {
	return c.CombatStats.HitPoints.Current <= 0
}

// IsDying reports whether the character is unconscious and still rolling to
// survive: not yet stabilised and not yet dead.
func (c *Character) IsDying() bool {
	return c.IsDown() && !c.IsDead() && !c.CombatStats.DeathSaves.Stabilised()
}

// CanAct reports whether the character may take an action, and why not.
//
// A saving throw is deliberately not an action: an unconscious creature still
// makes them (auto-failing Strength and Dexterity), and refusing one here would
// break the death save that is the whole of their turn.
func (c *Character) CanAct() (bool, string) {
	if c.IsDead() {
		return false, fmt.Sprintf("%s is dead", c.Name)
	}
	if c.IsDown() {
		return false, fmt.Sprintf(
			"%s is at 0 hit points and unconscious; they cannot act until they are healed or stabilised",
			c.Name)
	}
	if cond, blocked := firstPreventing(c.Conditions, Condition.PreventsAction); blocked {
		return false, fmt.Sprintf("%s is %s and cannot take actions", c.Name, cond)
	}
	return true, ""
}

// CanMove reports whether the character may move, and why not.
func (c *Character) CanMove() (bool, string) {
	if c.IsDead() {
		return false, fmt.Sprintf("%s is dead", c.Name)
	}
	if c.IsDown() {
		return false, fmt.Sprintf("%s is unconscious and cannot move", c.Name)
	}
	if cond, blocked := firstPreventing(c.Conditions, Condition.PreventsMovement); blocked {
		return false, fmt.Sprintf("%s is %s and cannot move", c.Name, cond)
	}
	if c.Speed() <= 0 {
		return false, fmt.Sprintf("%s has no movement left", c.Name)
	}
	return true, ""
}

// CanSpeak reports whether the character may speak, and why not.
//
// Speech is its own capability: an incapacitated creature can still talk, and
// a stunned one can, if only falteringly.
func (c *Character) CanSpeak() (bool, string) {
	if c.IsDead() {
		return false, fmt.Sprintf("%s is dead", c.Name)
	}
	if c.IsDown() {
		return false, fmt.Sprintf("%s is unconscious and cannot speak", c.Name)
	}
	if cond, blocked := firstPreventing(c.Conditions, Condition.PreventsSpeech); blocked {
		return false, fmt.Sprintf("%s is %s and cannot speak", c.Name, cond)
	}
	return true, ""
}

// CanAct reports whether a combatant may take an action, and why not.
//
// Combatant is the shared representation for characters and monsters, so it
// has to give the same answer as Character.CanAct for the same condition, or a
// goblin and a wizard disagree about what "paralysed" means.
func (c *Combatant) CanAct() (bool, string) {
	switch c.Status {
	case CombatantDead:
		return false, fmt.Sprintf("%s is dead", c.Name)
	case CombatantDying, CombatantStable, CombatantUnconscious:
		return false, fmt.Sprintf("%s is unconscious and cannot act", c.Name)
	}
	if cond, blocked := firstPreventing(c.Conditions, Condition.PreventsAction); blocked {
		return false, fmt.Sprintf("%s is %s and cannot take actions", c.Name, cond)
	}
	return true, ""
}

// CanMove reports whether a combatant may move, and why not.
func (c *Combatant) CanMove() (bool, string) {
	if c.Status == CombatantDead {
		return false, fmt.Sprintf("%s is dead", c.Name)
	}
	if c.IsDown() {
		return false, fmt.Sprintf("%s is unconscious and cannot move", c.Name)
	}
	if cond, blocked := firstPreventing(c.Conditions, Condition.PreventsMovement); blocked {
		return false, fmt.Sprintf("%s is held fast and cannot move (%s)", c.Name, cond)
	}
	return true, ""
}

// SyncHitPoints refreshes a combatant from its source document and re-derives
// the status those hit points imply.
//
// The source is authoritative: a creature can be damaged or healed outside the
// encounter -- a player attacking through the action endpoint, a potion between
// fights -- and a combatant that kept its own tally would drift. Status is
// re-derived because hit points and status must agree: a creature restored
// above zero is no longer dying, and one dropped to zero is.
//
// The dead stay dead. Hit points do not undo that.
func (c *Combatant) SyncHitPoints(hp HitPoints) {
	if c.Status == CombatantDead {
		return
	}
	c.HitPoints = hp

	switch {
	case hp.Current > 0:
		if c.Status == CombatantDying || c.Status == CombatantStable {
			c.Status = CombatantActive
			c.DeathSaves = DeathSaves{}
		}
	case c.Status == CombatantActive:
		if c.MakesDeathSaves {
			c.Status = CombatantDying
		} else {
			c.Status = CombatantDead
		}
	}
}

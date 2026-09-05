package models

import "fmt"

// Concentration is the spell a caster is holding together.
//
// It exists because the catalogue marks spells as requiring concentration and
// nothing enforced it: a wizard could hold Hold Person, Web, Haste and Moonbeam
// at once and never pay for any of them, and a paralysed victim stayed
// paralysed for ever because nothing remembered what had done it.
type Concentration struct {
	Spell     string `json:"spell" bson:"spell"`
	SlotLevel int    `json:"slot_level,omitempty" bson:"slot_level,omitempty"`

	// Condition and Targets are what the spell imposed, so ending it can undo
	// it. Without the target list a broken Hold Person leaves its victim held.
	Condition Condition `json:"condition,omitempty" bson:"condition,omitempty"`
	Targets   []string  `json:"targets,omitempty" bson:"targets,omitempty"`
}

// ConcentrationDC is the Constitution save for holding a spell after damage:
// ten, or half the damage taken if that is worse.
//
// Both halves matter. A flat 10 makes concentration nearly unbreakable at high
// levels; half the damage alone makes a single arrow a real threat to a spell.
func ConcentrationDC(damage int) int {
	if damage < 0 {
		damage = 0
	}
	if half := damage / 2; half > 10 {
		return half
	}
	return 10
}

// IsConcentrating reports whether the character is holding a spell.
func (c *Character) IsConcentrating() bool { return c.Spells.Concentrating != nil }

// BeginConcentration starts holding a spell and returns the name of the one it
// replaced, if any.
//
// A caster concentrates on one spell at a time; starting a second ends the
// first. The replaced name is returned rather than discarded so the turn can
// tell the player what they just gave up.
func (c *Character) BeginConcentration(held Concentration) (replaced string) {
	if c.Spells.Concentrating != nil {
		replaced = c.Spells.Concentrating.Spell
	}
	copied := held
	c.Spells.Concentrating = &copied
	return replaced
}

// EndConcentration stops holding whatever was held. Ending nothing is a no-op.
func (c *Character) EndConcentration() { c.Spells.Concentrating = nil }

// KeepsConcentration reports whether the character can still hold a spell at
// all, regardless of any saving throw.
//
// Being incapacitated or killed ends concentration outright -- there is no save
// against falling unconscious.
func (c *Character) KeepsConcentration() (bool, string) {
	if c.IsDead() {
		return false, fmt.Sprintf("%s is dead", c.Name)
	}
	if c.IsDown() {
		return false, fmt.Sprintf("%s is unconscious and cannot hold a spell together", c.Name)
	}
	if cond, blocked := firstPreventing(c.Conditions, Condition.PreventsAction); blocked {
		return false, fmt.Sprintf("%s is %s and cannot hold a spell together", c.Name, cond)
	}
	return true, ""
}

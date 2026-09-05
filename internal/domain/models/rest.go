package models

import "fmt"

// Whether a rest does anything.
//
// The rules for this are easy to skip and expensive to skip: without them a
// downed party sleeps its way out of a lost fight, and a dead character wakes
// up refreshed.

// CanBenefitFromLongRest reports whether a long rest would restore anything.
//
// A character must have at least one hit point at the start of a long rest to
// gain its benefits (PHB, "Resting"). Being unconscious is not resting, and a
// party that has been dropped needs someone to stabilise and heal them before
// the day can end.
func (c *Character) CanBenefitFromLongRest() (bool, string) {
	if c.IsDead() {
		return false, fmt.Sprintf("%s is dead and cannot rest", c.Name)
	}
	if c.IsDown() {
		return false, fmt.Sprintf(
			"%s needs at least 1 hit point to benefit from a long rest; stabilise and heal them first",
			c.Name)
	}
	return true, ""
}

// CanBenefitFromShortRest reports whether a short rest would restore anything.
//
// Unlike a long rest this is allowed at zero hit points: spending hit dice is
// what a short rest is for, and an ally who has stabilised you has bought you
// exactly that chance.
func (c *Character) CanBenefitFromShortRest() (bool, string) {
	if c.IsDead() {
		return false, fmt.Sprintf("%s is dead and cannot rest", c.Name)
	}
	return true, ""
}

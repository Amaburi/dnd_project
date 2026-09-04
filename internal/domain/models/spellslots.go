package models

// fullCasterSlots is the spell slot table for a full caster, indexed by caster
// level 1-20. Each row lists slots at spell levels 1 through 9.
//
// The PHB multiclass spellcaster table is the same numbers, which is why one
// table serves both paths.
var fullCasterSlots = [21][9]int{
	{},                          // index 0 unused
	{2},                         // 1
	{3},                         // 2
	{4, 2},                      // 3
	{4, 3},                      // 4
	{4, 3, 2},                   // 5
	{4, 3, 3},                   // 6
	{4, 3, 3, 1},                // 7
	{4, 3, 3, 2},                // 8
	{4, 3, 3, 3, 1},             // 9
	{4, 3, 3, 3, 2},             // 10
	{4, 3, 3, 3, 2, 1},          // 11
	{4, 3, 3, 3, 2, 1},          // 12
	{4, 3, 3, 3, 2, 1, 1},       // 13
	{4, 3, 3, 3, 2, 1, 1},       // 14
	{4, 3, 3, 3, 2, 1, 1, 1},    // 15
	{4, 3, 3, 3, 2, 1, 1, 1},    // 16
	{4, 3, 3, 3, 2, 1, 1, 1, 1}, // 17
	{4, 3, 3, 3, 3, 1, 1, 1, 1}, // 18
	{4, 3, 3, 3, 3, 2, 1, 1, 1}, // 19
	{4, 3, 3, 3, 3, 2, 2, 1, 1}, // 20
}

// pactSlots is the warlock's Pact Magic table: a count of slots, all at a
// single level, indexed by warlock level.
//
// Pact magic is separate from ordinary spell slots in every way that matters:
// the slots are always at the caster's highest level, they do not combine with
// other classes when multiclassing, and they return on a *short* rest.
var pactSlots = [21]struct{ Count, Level int }{
	{},     // 0
	{1, 1}, // 1
	{2, 1}, // 2
	{2, 2}, // 3
	{2, 2}, // 4
	{2, 3}, // 5
	{2, 3}, // 6
	{2, 4}, // 7
	{2, 4}, // 8
	{2, 5}, // 9
	{2, 5}, // 10
	{3, 5}, // 11
	{3, 5}, // 12
	{3, 5}, // 13
	{3, 5}, // 14
	{3, 5}, // 15
	{3, 5}, // 16
	{4, 5}, // 17
	{4, 5}, // 18
	{4, 5}, // 19
	{4, 5}, // 20
}

// SlotsForCasterLevel returns the spell slots of a full caster at a given
// caster level.
func SlotsForCasterLevel(casterLevel int) []SpellSlot {
	if casterLevel < 1 {
		return nil
	}
	if casterLevel > 20 {
		casterLevel = 20
	}

	var slots []SpellSlot
	for i, count := range fullCasterSlots[casterLevel] {
		if count > 0 {
			slots = append(slots, SpellSlot{Level: i + 1, Total: count})
		}
	}
	return slots
}

// PactSlotsForLevel returns a warlock's pact magic slots at a given level.
func PactSlotsForLevel(warlockLevel int) (count, level int) {
	if warlockLevel < 1 {
		return 0, 0
	}
	if warlockLevel > 20 {
		warlockLevel = 20
	}
	s := pactSlots[warlockLevel]
	return s.Count, s.Level
}

// singleClassCasterLevel converts a class level into the caster level used by
// that class's own spell slot table.
//
// Half and third casters round *up* here, which is what reproduces the printed
// paladin and Eldritch Knight tables -- a paladin gains their first slots at
// level 2, and an Eldritch Knight at level 3.
func singleClassCasterLevel(level int, p CasterProgression) int {
	switch p {
	case CasterFull:
		return level
	case CasterHalf:
		if level < 2 {
			return 0
		}
		return (level + 1) / 2
	case CasterThird:
		if level < 3 {
			return 0
		}
		return (level + 2) / 3
	default:
		return 0
	}
}

// multiclassContribution is what a class contributes to a combined caster
// level.
//
// Here half and third casters round *down*, per the PHB multiclassing rules.
// The asymmetry with singleClassCasterLevel is deliberate and is a genuine
// quirk of 5e: a single-classed paladin 3 has more slots than the same
// character would derive from the multiclass table.
func multiclassContribution(level int, p CasterProgression) int {
	switch p {
	case CasterFull:
		return level
	case CasterHalf:
		return level / 2
	case CasterThird:
		return level / 3
	default:
		// Pact magic never joins the combined caster level.
		return 0
	}
}

// SpellSlotsForClasses computes the spell slots a character has from their
// class levels, and separately any warlock pact magic slots.
//
// A character with exactly one spellcasting class uses that class's own table;
// anyone combining two or more uses the PHB multiclass rules. Warlock levels
// are excluded from the combined caster level and reported on their own.
func SpellSlotsForClasses(classes []ClassLevel) (slots []SpellSlot, pactCount, pactLevel int) {
	var (
		casting    []ClassLevel
		warlockLvl int
		combined   int
	)

	for _, cl := range classes {
		_, progression := cl.Casting()
		if progression == CasterPact {
			warlockLvl += cl.Level
			continue
		}
		if progression == CasterNone {
			continue
		}
		casting = append(casting, cl)
		combined += multiclassContribution(cl.Level, progression)
	}

	switch len(casting) {
	case 0:
		// Only pact magic, or no casting at all.
	case 1:
		_, progression := casting[0].Casting()
		// A lone caster class also multiclassed with a warlock still uses its
		// own table, since pact magic never merges.
		slots = SlotsForCasterLevel(singleClassCasterLevel(casting[0].Level, progression))
	default:
		slots = SlotsForCasterLevel(combined)
	}

	if warlockLvl > 0 {
		pactCount, pactLevel = PactSlotsForLevel(warlockLvl)
	}
	return slots, pactCount, pactLevel
}

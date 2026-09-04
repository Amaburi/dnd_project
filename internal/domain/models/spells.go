package models

// MaxSpellLevel is the highest spell level in 5e.
const MaxSpellLevel = 9

// Spell is a spell a character knows, with the level it is cast at.
type Spell struct {
	Name  string `json:"name" bson:"name"`
	Level int    `json:"level" bson:"level"` // 0 for cantrips
	// Concentration and Ritual affect how a spell can be used, and matter
	// enough to combat and downtime to be worth carrying here.
	Concentration bool `json:"concentration,omitempty" bson:"concentration,omitempty"`
	Ritual        bool `json:"ritual,omitempty" bson:"ritual,omitempty"`
}

// SpellSlot is a character's slots at one spell level.
//
// Slots are the resource that actually gates casting: without them a spell
// list is only a description of what a character could do, never a limit on
// what they may do this adventuring day.
type SpellSlot struct {
	Level    int `json:"level" bson:"level"`
	Total    int `json:"total" bson:"total"`
	Expended int `json:"expended" bson:"expended"`
}

// Available returns how many slots at this level remain.
func (s SpellSlot) Available() int {
	if s.Expended > s.Total {
		return 0
	}
	return s.Total - s.Expended
}

// Spells contains spellcasting information.
//
// Slots are stored as a slice rather than a map because BSON document keys
// must be strings; a slice also keeps the levels in order.
type Spells struct {
	SpellcastingAbility Ability `json:"spellcasting_ability" bson:"spellcasting_ability"`
	SpellcastingClass   string  `json:"spellcasting_class,omitempty" bson:"spellcasting_class,omitempty"`

	Cantrips []string `json:"cantrips" bson:"cantrips"`
	Known    []Spell  `json:"known" bson:"known"`

	// Prepared names the subset of Known that is currently prepared. Classes
	// that prepare (cleric, druid, wizard) use it; classes that simply know
	// their list (sorcerer, warlock, bard) leave it empty.
	Prepared []string `json:"prepared,omitempty" bson:"prepared,omitempty"`

	Slots []SpellSlot `json:"slots" bson:"slots"`

	// PactSlots is the warlock's Pact Magic pool. It is stored separately
	// because it never merges with ordinary slots and returns on a short
	// rest rather than a long one.
	PactSlots SpellSlot `json:"pact_slots" bson:"pact_slots"`
}

// AvailablePactSlots returns how many pact magic slots remain.
func (s *Spells) AvailablePactSlots() int {
	return s.PactSlots.Available()
}

// ExpendPactSlot spends one pact magic slot.
//
// Pact slots are always cast at the warlock's highest level, so unlike
// ExpendSlot there is no level to choose.
func (s *Spells) ExpendPactSlot() error {
	if s.PactSlots.Total < 1 {
		return Invalid("character has no pact magic slots")
	}
	if s.PactSlots.Available() < 1 {
		return Invalid("no pact magic slots remaining")
	}
	s.PactSlots.Expended++
	return nil
}

// RestorePactSlots returns every pact slot, as a short or long rest does.
func (s *Spells) RestorePactSlots() {
	s.PactSlots.Expended = 0
}

// SlotsAt returns the slot record for a level, and whether the caster has any
// slots at that level at all.
func (s *Spells) SlotsAt(level int) (SpellSlot, bool) {
	for _, slot := range s.Slots {
		if slot.Level == level {
			return slot, true
		}
	}
	return SpellSlot{Level: level}, false
}

// AvailableSlots returns how many unexpended slots exist at a level.
func (s *Spells) AvailableSlots(level int) int {
	slot, ok := s.SlotsAt(level)
	if !ok {
		return 0
	}
	return slot.Available()
}

// ExpendSlot spends one slot at the given level.
//
// Casting at a higher level than a spell's own is legal and is how upcasting
// works, so callers pass the level actually used, not the spell's base level.
func (s *Spells) ExpendSlot(level int) error {
	if level < 1 || level > MaxSpellLevel {
		return Invalid("spell slot level must be between 1 and %d, got %d", MaxSpellLevel, level)
	}
	for i := range s.Slots {
		if s.Slots[i].Level != level {
			continue
		}
		if s.Slots[i].Available() < 1 {
			return Invalid("no level %d spell slots remaining", level)
		}
		s.Slots[i].Expended++
		return nil
	}
	return Invalid("character has no level %d spell slots", level)
}

// RestoreSlot returns one expended slot at a level, for effects that recover
// a single slot without a long rest.
func (s *Spells) RestoreSlot(level int) error {
	for i := range s.Slots {
		if s.Slots[i].Level != level {
			continue
		}
		if s.Slots[i].Expended < 1 {
			return Invalid("no expended level %d spell slots to restore", level)
		}
		s.Slots[i].Expended--
		return nil
	}
	return Invalid("character has no level %d spell slots", level)
}

// RestoreAllSlots returns every expended slot, as a long rest does.
func (s *Spells) RestoreAllSlots() {
	for i := range s.Slots {
		s.Slots[i].Expended = 0
	}
}

// HighestSlotLevel returns the highest level at which any slot remains, or 0
// when the caster is out of slots entirely.
func (s *Spells) HighestSlotLevel() int {
	highest := 0
	for _, slot := range s.Slots {
		if slot.Available() > 0 && slot.Level > highest {
			highest = slot.Level
		}
	}
	return highest
}

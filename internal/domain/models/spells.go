package models

import "fmt"

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

// HitDice tracks the dice a character spends to heal on a short rest.
//
// Without these a short rest cannot restore anything, which is half of 5e's
// resource economy.
type HitDice struct {
	Die   int `json:"die" bson:"die"`     // 6, 8, 10 or 12 by class
	Total int `json:"total" bson:"total"` // equal to character level
	Spent int `json:"spent" bson:"spent"`
}

// Available returns how many hit dice remain to spend.
func (h HitDice) Available() int {
	if h.Spent > h.Total {
		return 0
	}
	return h.Total - h.Spent
}

// Spend consumes one hit die.
func (h *HitDice) Spend() error {
	if h.Available() < 1 {
		return Invalid("no hit dice remaining")
	}
	h.Spent++
	return nil
}

// RegainOnLongRest returns hit dice as a long rest does: at least one, and up
// to half the character's total, rounded down.
func (h *HitDice) RegainOnLongRest() {
	regained := h.Total / 2
	if regained < 1 {
		regained = 1
	}
	if regained > h.Spent {
		regained = h.Spent
	}
	h.Spent -= regained
}

// String renders the pool as a dice expression, e.g. "3/5d8".
func (h HitDice) String() string {
	return fmt.Sprintf("%d/%dd%d", h.Available(), h.Total, h.Die)
}

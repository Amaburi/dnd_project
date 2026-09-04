// Package combat runs an encounter: initiative, turn order and the state
// machine that moves between rounds.
//
// The tracker owns *whose turn it is*; the rules engine owns *what happens on
// it*. Keeping those apart is what lets a turn be resolved without the tracker
// knowing anything about damage, and an encounter be advanced without the
// engine knowing anything about rounds.
package combat

import (
	"sort"

	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
)

// Tracker advances an encounter.
type Tracker struct {
	encounter *models.CombatEncounter
}

// NewTracker wraps an encounter.
func NewTracker(encounter *models.CombatEncounter) *Tracker {
	if encounter.CombatState.Phase == "" {
		encounter.CombatState.Phase = models.PhaseNotStarted
	}
	return &Tracker{encounter: encounter}
}

// Encounter returns the encounter being tracked.
func (t *Tracker) Encounter() *models.CombatEncounter { return t.encounter }

// Phase reports where the encounter is.
func (t *Tracker) Phase() models.CombatPhase { return t.encounter.CombatState.Phase }

// Round is the current round, counting from 1 once combat starts.
func (t *Tracker) Round() int { return t.encounter.CombatState.Round }

// AddCombatant enrols a creature.
//
// Combatants may only join before initiative is rolled or during an active
// encounter; adding one to a finished fight is a mistake worth catching.
func (t *Tracker) AddCombatant(c models.Combatant) error {
	if t.Phase() == models.PhaseEnded {
		return models.Invalid("the encounter has ended")
	}
	if c.CombatantID == "" {
		return models.Invalid("combatant id is required")
	}
	for _, existing := range t.encounter.Combatants {
		if existing.CombatantID == c.CombatantID {
			return models.Invalid("combatant %q is already in this encounter", c.CombatantID)
		}
	}
	if c.Status == "" {
		c.Status = models.CombatantActive
	}

	t.encounter.Combatants = append(t.encounter.Combatants, c)

	// A creature joining a fight already under way still needs a place in the
	// order, so the order is rebuilt rather than appended to.
	if t.Phase() == models.PhaseActive {
		t.sortInitiative()
	}
	return nil
}

// RollInitiative rolls for every combatant and starts the encounter.
//
// A combatant that already has an initiative keeps it, so a DM who set one by
// hand is not overruled.
func (t *Tracker) RollInitiative(roller *dice.Roller) error {
	if t.Phase() == models.PhaseActive {
		return models.Invalid("initiative has already been rolled")
	}
	if t.Phase() == models.PhaseEnded {
		return models.Invalid("the encounter has ended")
	}
	if len(t.encounter.Combatants) == 0 {
		return models.Invalid("an encounter needs combatants before initiative")
	}

	for i := range t.encounter.Combatants {
		if t.encounter.Combatants[i].Initiative == 0 {
			roller.RollInitiative(&t.encounter.Combatants[i])
		}
	}

	t.sortInitiative()
	t.encounter.CombatState.Phase = models.PhaseActive
	t.encounter.CombatState.Round = 1
	t.encounter.CombatState.Turn = 0
	t.encounter.Status = "active"

	if current, ok := t.Current(); ok {
		current.StartTurn()
	}
	return nil
}

// sortInitiative orders combatants highest first, breaking ties on the
// Dexterity modifier as the rules direct, and then on name so the order is at
// least stable rather than arbitrary.
func (t *Tracker) sortInitiative() {
	combatants := t.encounter.Combatants

	sort.SliceStable(combatants, func(i, j int) bool {
		if combatants[i].Initiative != combatants[j].Initiative {
			return combatants[i].Initiative > combatants[j].Initiative
		}
		if combatants[i].InitiativeModifier != combatants[j].InitiativeModifier {
			return combatants[i].InitiativeModifier > combatants[j].InitiativeModifier
		}
		return combatants[i].Name < combatants[j].Name
	})

	for i := range combatants {
		combatants[i].TurnOrder = i + 1
	}
}

// Order returns the combatants in initiative order.
func (t *Tracker) Order() []models.Combatant {
	return t.encounter.Combatants
}

// Current returns whose turn it is.
func (t *Tracker) Current() (*models.Combatant, bool) {
	if t.Phase() != models.PhaseActive {
		return nil, false
	}
	index := t.encounter.CombatState.Turn
	if index < 0 || index >= len(t.encounter.Combatants) {
		return nil, false
	}
	return &t.encounter.Combatants[index], true
}

// Find returns a combatant by id.
func (t *Tracker) Find(combatantID string) (*models.Combatant, bool) {
	for i := range t.encounter.Combatants {
		if t.encounter.Combatants[i].CombatantID == combatantID {
			return &t.encounter.Combatants[i], true
		}
	}
	return nil, false
}

// NextTurn advances to the next combatant who can act.
//
// Creatures that are down are skipped rather than removed: they are still in
// the order, still take up space, and may be healed back into it. Wrapping
// past the last combatant starts a new round.
func (t *Tracker) NextTurn() (*models.Combatant, bool) {
	if t.Phase() != models.PhaseActive {
		return nil, false
	}

	total := len(t.encounter.Combatants)
	if total == 0 {
		return nil, false
	}

	// At most one full lap: if everyone is down there is no next turn.
	for step := 0; step < total; step++ {
		t.encounter.CombatState.Turn++
		if t.encounter.CombatState.Turn >= total {
			t.encounter.CombatState.Turn = 0
			t.encounter.CombatState.Round++
			t.encounter.CombatState.DurationRounds = t.encounter.CombatState.Round
		}

		current := &t.encounter.Combatants[t.encounter.CombatState.Turn]
		if current.Status == models.CombatantDead {
			continue
		}

		current.StartTurn()
		return current, true
	}

	return nil, false
}

// SidesRemaining reports how many combatants on each side are still standing.
func (t *Tracker) SidesRemaining() (party, foes int) {
	for i := range t.encounter.Combatants {
		c := &t.encounter.Combatants[i]
		if c.IsDown() {
			continue
		}
		if c.Side() == models.SideParty {
			party++
		} else {
			foes++
		}
	}
	return party, foes
}

// Outcome reports whether the encounter has been decided, and how.
//
// Only elimination is detected automatically: capture, escape and negotiation
// are the DM's call, and guessing at them would end fights that were still
// being played.
func (t *Tracker) Outcome() (string, bool) {
	if t.Phase() != models.PhaseActive {
		return "", false
	}

	party, foes := t.SidesRemaining()
	switch {
	case party == 0 && foes == 0:
		return "mutual_destruction", true
	case foes == 0:
		return "victory", true
	case party == 0:
		return "defeat", true
	default:
		return "", false
	}
}

// End closes the encounter with an outcome.
func (t *Tracker) End(outcome string) {
	if t.Phase() == models.PhaseEnded {
		return
	}

	t.encounter.CombatState.Phase = models.PhaseEnded
	t.encounter.Status = "completed"
	t.encounter.VictoryConditions.Outcome = &outcome

	now := nowUTC()
	t.encounter.CombatState.CombatEndedAt = &now
	t.encounter.CombatState.DurationRounds = t.encounter.CombatState.Round
	t.encounter.UpdatedAt = now
}

// EndIfDecided closes the encounter when one side has been eliminated, and
// reports the outcome if it did.
func (t *Tracker) EndIfDecided() (string, bool) {
	outcome, decided := t.Outcome()
	if !decided {
		return "", false
	}
	t.End(outcome)
	return outcome, true
}

// RecordTurn appends a turn to the encounter's history, which is what makes a
// fight reviewable afterwards.
func (t *Tracker) RecordTurn(turn models.Turn) {
	turn.Round = t.encounter.CombatState.Round
	turn.Turn = t.encounter.CombatState.Turn
	t.encounter.TurnHistory = append(t.encounter.TurnHistory, turn)
	t.encounter.UpdatedAt = nowUTC()
}

// RecordDamage appends to the damage log.
func (t *Tracker) RecordDamage(attacker, target string, amount int, damageType models.DamageType) {
	t.encounter.DamageLog = append(t.encounter.DamageLog, models.DamageLogEntry{
		Attacker: attacker, Target: target, Damage: amount,
		DamageType: damageType, Round: t.encounter.CombatState.Round,
		Timestamp: nowUTC(),
	})
	t.encounter.UpdatedAt = nowUTC()
}

// RecordNarration appends to the narrative log.
func (t *Tracker) RecordNarration(text string) {
	if text == "" {
		return
	}
	t.encounter.NarrativeLog = append(t.encounter.NarrativeLog, models.NarrativeLogEntry{
		Text: text, Round: t.encounter.CombatState.Round, Timestamp: nowUTC(),
	})
	t.encounter.UpdatedAt = nowUTC()
}

// TotalDamage sums what a combatant has dealt, for the after-action summary.
func (t *Tracker) TotalDamage(attacker string) int {
	total := 0
	for _, entry := range t.encounter.DamageLog {
		if entry.Attacker == attacker {
			total += entry.Damage
		}
	}
	return total
}

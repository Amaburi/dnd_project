// Package encounter plays a monster's turn.
//
// The combat tracker knew whose turn it was and the rules engine knew how to
// resolve an attack, but nothing joined them: an encounter could be created,
// initiative rolled and turns advanced, and no monster ever swung. Every fight
// was one-sided. This is the thread between them.
//
// The order mirrors application/turn and is deliberate:
//
//	whose turn → may they act → choose → resolve → persist → narrate → log → advance
//
// Choosing comes from the model and resolving does not. The model proposes an
// action from the statblock; the engine decides whether it lands.
package encounter

import (
	"context"
	"fmt"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/combat"
	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/domain/rules"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
)

// The stores this service needs, named as narrowly as it uses them.
type (
	// EncounterStore reads and writes the fight in progress.
	EncounterStore interface {
		GetEncounterByEncounterID(ctx context.Context, campaignID, encounterID string) (*models.CombatEncounter, error)
		SaveEncounterState(ctx context.Context, campaignID string, e *models.CombatEncounter) error
	}

	// MonsterStore reads the acting statblock and writes back what it lost.
	MonsterStore interface {
		GetMonsterByMonsterID(ctx context.Context, campaignID, monsterID string) (*models.Monster, error)
		UpdateHitPoints(ctx context.Context, campaignID, monsterID string, hp models.HitPoints) error
	}

	// CharacterStore writes back what a character lost.
	//
	// Without this a monster's blow lands in the encounter and evaporates: the
	// party would heal itself simply by leaving the fight.
	CharacterStore interface {
		GetCharacterByCharacterID(ctx context.Context, characterID string) (*models.Character, error)
		UpdateHitPoints(ctx context.Context, campaignID, characterID string, hp models.HitPoints) error
		UpdateSpellSlots(ctx context.Context, characterID string, spells models.Spells) error
	}

	// EventStore is the campaign's memory.
	EventStore interface {
		AppendEvent(ctx context.Context, event *models.StoryEvent) error
	}

	// Narrator is the subset of ai.Service a monster's turn uses.
	Narrator interface {
		ChooseTactics(ctx context.Context, req *ai.TacticsRequest) (*ai.TacticsResponse, error)
		NarrateAction(ctx context.Context, req *ai.NarrationRequest) (*ai.NarrationResponse, error)
	}
)

// Service resolves one combatant's turn.
type Service struct {
	encounters EncounterStore
	monsters   MonsterStore
	characters CharacterStore
	events     EventStore
	narrator   Narrator
	engine     *rules.Engine
}

// NewService wires an encounter service.
func NewService(
	encounters EncounterStore,
	monsters MonsterStore,
	characters CharacterStore,
	events EventStore,
	narrator Narrator,
	engine *rules.Engine,
) *Service {
	return &Service{
		encounters: encounters, monsters: monsters, characters: characters,
		events: events, narrator: narrator, engine: engine,
	}
}

// Result is what one resolved turn produced.
type Result struct {
	Round int    `json:"round"`
	Actor string `json:"actor"`

	// AwaitingPlayer marks a turn that belongs to a player character. The
	// service will not take it: resolving it automatically would play the game
	// for them. They act through the action endpoint instead.
	AwaitingPlayer bool `json:"awaiting_player"`

	// Skipped marks a turn nothing happened on, with the reason.
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skip_reason,omitempty"`

	// DeathSave is set when the turn belonged to a dying character. Rolling
	// one is not a choice, so it is not left to the player.
	DeathSave *rules.DeathSaveResult `json:"death_save,omitempty"`

	Target    string              `json:"target,omitempty"`
	Action    string              `json:"action,omitempty"`
	Attack    *rules.AttackResult `json:"attack,omitempty"`
	Rationale string              `json:"rationale,omitempty"`

	// Fallback reports that the model's choice was unusable and a default was
	// substituted, rather than the turn being lost.
	Fallback     bool   `json:"fallback,omitempty"`
	FallbackNote string `json:"fallback_note,omitempty"`

	// Concentration is the save a damaged caster made to hold their spell, set
	// only when there was a spell at risk.
	Concentration *rules.ConcentrationResult `json:"concentration,omitempty"`

	Narration string `json:"narration,omitempty"`

	NextCombatant string `json:"next_combatant,omitempty"`
	Outcome       string `json:"outcome,omitempty"`

	Event      *models.StoryEvent `json:"event,omitempty"`
	TokensUsed int                `json:"tokens_used"`
	Cost       float64            `json:"cost_usd"`
	Elapsed    time.Duration      `json:"elapsed"`
}

// ResolveTurn plays the current combatant's turn.
func (s *Service) ResolveTurn(ctx context.Context, campaignID, encounterID string) (*Result, error) {
	started := time.Now()

	encounter, err := s.encounters.GetEncounterByEncounterID(ctx, campaignID, encounterID)
	if err != nil {
		return nil, err
	}
	if encounter == nil {
		return nil, models.NotFound("encounter")
	}

	tracker := combat.NewTracker(encounter)
	if tracker.Phase() != models.PhaseActive {
		return nil, models.Invalid(
			"the encounter is %s; roll initiative before taking turns", tracker.Phase())
	}

	current, ok := tracker.Current()
	if !ok {
		return nil, models.Invalid("the encounter has no combatant whose turn it is")
	}

	// Hit points are refreshed from the sheet and the statblock before
	// anything is decided.
	//
	// The encounter is not the only thing that damages a creature: a player
	// attacking through the action endpoint writes to the monster's statblock,
	// and healing outside a fight writes to the character sheet. Without this,
	// the goblin the party beat to 1 hit point came back to full on its own
	// turn. The source document is authoritative; the combatant is a working
	// copy refreshed at the start of every turn.
	if err := s.refresh(ctx, campaignID, encounter); err != nil {
		return nil, err
	}

	result := &Result{Round: tracker.Round(), Actor: current.Name}

	// A dying character rolls a death save. It is automatic, not a decision,
	// so leaving it to the player means a character who never stabilises and
	// never dies.
	if current.Status == models.CombatantDying {
		save, err := s.engine.DeathSave(current)
		if err != nil {
			return nil, err
		}
		result.DeathSave = &save
		result.SkipReason = save.Summary()

		if err := s.persistTarget(ctx, campaignID, current); err != nil {
			return nil, err
		}
		if err := s.finish(ctx, campaignID, tracker, result, started); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Whether the creature can act at all is settled before whose turn it is.
	// An unconscious or paralysed player character has no turn to take, and
	// reporting it as awaiting them would stall the fight on input that can
	// never come. The turn still passes: a skipped turn is still a turn.
	if canAct, reason := current.CanAct(); !canAct {
		result.Skipped, result.SkipReason = true, reason
		if err := s.finish(ctx, campaignID, tracker, result, started); err != nil {
			return nil, err
		}
		return result, nil
	}

	// A player's turn belongs to the player.
	if current.Side() == models.SideParty {
		result.AwaitingPlayer = true
		result.Elapsed = time.Since(started)
		return result, nil
	}

	monster, err := s.monsters.GetMonsterByMonsterID(ctx, campaignID, current.SourceID)
	if err != nil {
		return nil, err
	}
	if monster == nil {
		return nil, models.NotFound("monster " + current.SourceID)
	}

	allies, enemies := sides(encounter, current)
	if len(enemies) == 0 {
		result.Skipped, result.SkipReason = true, "there is no one left to attack"
		if err := s.finish(ctx, campaignID, tracker, result, started); err != nil {
			return nil, err
		}
		return result, nil
	}

	tactics, err := s.narrator.ChooseTactics(ctx, &ai.TacticsRequest{
		Monster: monster, Self: *current,
		Enemies: enemies, Allies: allies, Round: tracker.Round(),
	})
	if err != nil {
		return nil, err
	}
	result.TokensUsed += tactics.TokensUsed
	result.Cost += tactics.Cost
	result.Rationale = tactics.Choice.Rationale
	result.Fallback, result.FallbackNote = tactics.Fallback, tactics.FallbackNote

	if tactics.Action == nil || tactics.Target == nil {
		result.Skipped, result.SkipReason = true, "no usable action was available"
		if err := s.finish(ctx, campaignID, tracker, result, started); err != nil {
			return nil, err
		}
		return result, nil
	}
	result.Action = tactics.Action.Name

	// The target is resolved against the encounter's own slice, so damage
	// lands on the combatant the tracker is tracking rather than on a copy.
	target := find(encounter, tactics.Target.CombatantID)
	if target == nil {
		return nil, models.Invalid("%s is not in this encounter", tactics.Target.Name)
	}
	result.Target = target.Name

	attack, err := s.engine.MonsterAttack(current, *tactics.Action, target, models.RollNormal)
	if err != nil {
		return nil, err
	}
	result.Attack = &attack
	current.ActionUsed = true

	if err := s.persistTarget(ctx, campaignID, target); err != nil {
		return nil, err
	}

	// Damage threatens a held spell. Without this, concentration is a label
	// rather than a cost: the wizard holds Hold Person through the whole fight.
	if attack.Damage != nil && attack.Damage.Dealt > 0 {
		if err := s.threatenConcentration(ctx, campaignID, encounter, target, attack.Damage.Dealt, result); err != nil {
			return nil, err
		}
	}

	narration, err := s.narrator.NarrateAction(ctx, &ai.NarrationRequest{
		Facts:   attack.Facts(),
		Context: fmt.Sprintf("round %d of a fight", tracker.Round()),
	})
	if err != nil {
		return nil, err
	}
	result.Narration = narration.Text
	result.TokensUsed += narration.TokensUsed
	result.Cost += narration.Cost

	tracker.RecordNarration(narration.Text)
	if attack.Damage != nil && attack.Damage.Dealt > 0 {
		tracker.RecordDamage(current.Name, target.Name, attack.Damage.Dealt, attack.Damage.Type)
	}

	if err := s.finish(ctx, campaignID, tracker, result, started); err != nil {
		return nil, err
	}
	return result, nil
}

// finish logs the turn, advances the order, ends a decided fight and saves.
//
// Everything after the attack happens here so a skipped turn and a resolved one
// leave the encounter in the same shape.
func (s *Service) finish(
	ctx context.Context,
	campaignID string,
	tracker *combat.Tracker,
	result *Result,
	started time.Time,
) error {
	encounter := tracker.Encounter()

	if result.Attack != nil || result.Skipped || result.DeathSave != nil {
		if err := s.logTurn(ctx, encounter, result); err != nil {
			return err
		}
	}

	// A fight is checked for a winner before the turn passes, so a killing
	// blow ends the encounter rather than handing initiative to a corpse.
	if outcome, decided := tracker.EndIfDecided(); decided {
		result.Outcome = outcome
	} else if next, ok := tracker.NextTurn(); ok {
		result.NextCombatant = next.Name
	}

	if err := s.encounters.SaveEncounterState(ctx, campaignID, encounter); err != nil {
		return err
	}
	result.Elapsed = time.Since(started)
	return nil
}

// threatenConcentration makes a damaged caster hold their spell or lose it.
//
// Losing it lifts whatever the spell imposed. Without that step a dropped Hold
// Person leaves its victim paralysed for the rest of the campaign, which is
// worse than never having enforced concentration at all.
func (s *Service) threatenConcentration(
	ctx context.Context,
	campaignID string,
	encounter *models.CombatEncounter,
	target *models.Combatant,
	damage int,
	result *Result,
) error {
	if target.SourceType != models.SourceCharacter {
		return nil
	}

	character, err := s.characters.GetCharacterByCharacterID(ctx, target.SourceID)
	if err != nil {
		return err
	}
	if character == nil || !character.IsConcentrating() {
		return nil
	}

	// The sheet has to agree with the combatant before the save is rolled:
	// being knocked out ends concentration without one.
	character.CombatStats.HitPoints = target.HitPoints
	character.Conditions = target.Conditions

	check, err := s.engine.ConcentrationCheck(character, damage)
	if err != nil {
		return err
	}
	result.Concentration = &check

	if check.Broken {
		for _, combatantID := range check.Targets {
			if victim := find(encounter, combatantID); victim != nil && check.Condition != "" {
				s.engine.RemoveCondition(victim, check.Condition)
			}
		}
	}
	return s.characters.UpdateSpellSlots(ctx, target.SourceID, character.Spells)
}

// refresh reloads every combatant's hit points from its source document.
//
// A combatant with no source -- an ad-hoc creature that lives only in the
// encounter -- is left alone: there is nothing to refresh it from.
func (s *Service) refresh(ctx context.Context, campaignID string, encounter *models.CombatEncounter) error {
	for i := range encounter.Combatants {
		c := &encounter.Combatants[i]
		switch c.SourceType {
		case models.SourceMonster:
			monster, err := s.monsters.GetMonsterByMonsterID(ctx, campaignID, c.SourceID)
			if err != nil {
				return err
			}
			if monster != nil {
				c.SyncHitPoints(monster.HitPoints)
			}
		case models.SourceCharacter:
			character, err := s.characters.GetCharacterByCharacterID(ctx, c.SourceID)
			if err != nil {
				return err
			}
			if character != nil {
				c.SyncHitPoints(character.CombatStats.HitPoints)
			}
		}
	}
	return nil
}

// persistTarget writes the target's hit points back to whatever it came from.
func (s *Service) persistTarget(ctx context.Context, campaignID string, target *models.Combatant) error {
	switch target.SourceType {
	case models.SourceCharacter:
		return s.characters.UpdateHitPoints(ctx, campaignID, target.SourceID, target.HitPoints)
	case models.SourceMonster:
		return s.monsters.UpdateHitPoints(ctx, campaignID, target.SourceID, target.HitPoints)
	default:
		// A combatant with no source is an ad-hoc creature that lives only in
		// the encounter, and the encounter is saved anyway.
		return nil
	}
}

// logTurn appends the turn to the campaign's memory.
func (s *Service) logTurn(ctx context.Context, encounter *models.CombatEncounter, result *Result) error {
	summary := result.SkipReason
	if result.Attack != nil {
		summary = result.Attack.Summary()
	}

	event := &models.StoryEvent{
		CampaignID: encounter.CampaignID,
		SessionID:  encounter.SessionID,
		EventType:  "combat_action",
		Trigger: models.EventTrigger{
			Type: "ai_initiated", Intent: result.Action, Target: result.Target,
		},
		Narrative: models.NarrativeInfo{
			AIGeneratedText:  result.Narration,
			DMInterpretation: summary,
		},
		Timestamp: time.Now().UTC(),
	}
	if err := s.events.AppendEvent(ctx, event); err != nil {
		return err
	}
	result.Event = event
	return nil
}

// sides splits the encounter into the actor's allies and its enemies.
func sides(encounter *models.CombatEncounter, actor *models.Combatant) (allies, enemies []models.Combatant) {
	for i := range encounter.Combatants {
		c := &encounter.Combatants[i]
		if c.CombatantID == actor.CombatantID || c.IsDown() {
			continue
		}
		if c.Side() == actor.Side() {
			allies = append(allies, *c)
		} else {
			enemies = append(enemies, *c)
		}
	}
	return allies, enemies
}

// find returns the encounter's own copy of a combatant, so damage lands on the
// creature the tracker is tracking.
func find(encounter *models.CombatEncounter, combatantID string) *models.Combatant {
	for i := range encounter.Combatants {
		if encounter.Combatants[i].CombatantID == combatantID {
			return &encounter.Combatants[i]
		}
	}
	return nil
}

// Package turn orchestrates one player action from sentence to logged event.
//
// Every piece it uses already existed and none of them knew about each other:
// the parser produced intents nobody resolved, the engine resolved actions
// nobody logged, and the log held a history nobody read. This package is the
// thread between them, and it is the only place in the codebase that is
// allowed to know the whole sequence.
//
// The order is deliberate and never varies:
//
//	parse → resolve → persist → narrate → log
//
// Narration comes after resolution because the model must be handed a verdict,
// not asked for one.
package turn

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/domain/rules"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
)

// The stores this service needs, named as narrowly as it uses them.
//
// They are interfaces so a turn can be tested end to end without a database:
// the orchestration is the part most worth testing and the part hardest to
// reach through Mongo.
type (
	// CharacterStore reads the acting character.
	CharacterStore interface {
		GetCharacterByCharacterID(ctx context.Context, characterID string) (*models.Character, error)
	}

	// MonsterStore reads the creatures that can be targeted, and writes back
	// the hit points an attack changed.
	MonsterStore interface {
		GetMonstersByCampaign(ctx context.Context, campaignID string) ([]*models.Monster, error)
		UpdateHitPoints(ctx context.Context, campaignID, monsterID string, hp models.HitPoints) error
	}

	// SessionStore finds the session a turn belongs to.
	SessionStore interface {
		GetActiveSession(ctx context.Context, campaignID string) (*models.Session, error)
	}

	// EventStore is the campaign's memory.
	EventStore interface {
		AppendEvent(ctx context.Context, event *models.StoryEvent) error
		GetRecentEvents(ctx context.Context, campaignID string, limit int) ([]*models.StoryEvent, error)
	}

	// Narrator is the subset of ai.Service a turn uses.
	Narrator interface {
		ExtractIntent(ctx context.Context, req *ai.IntentRequest) (*ai.IntentResponse, error)
		NarrateAction(ctx context.Context, req *ai.NarrationRequest) (*ai.NarrationResponse, error)
		NarrateCheck(ctx context.Context, req *ai.NarrationRequest) (*ai.NarrationResponse, error)
		GenerateNarrative(ctx context.Context, req *ai.NarrativeRequest) (*ai.NarrativeResponse, error)
	}
)

// Service runs a turn.
type Service struct {
	characters CharacterStore
	monsters   MonsterStore
	sessions   SessionStore
	events     EventStore
	narrator   Narrator
	engine     *rules.Engine

	// RecentEventLimit is how much history feeds a prompt.
	RecentEventLimit int
}

// NewService wires a turn service.
func NewService(
	characters CharacterStore,
	monsters MonsterStore,
	sessions SessionStore,
	events EventStore,
	narrator Narrator,
	engine *rules.Engine,
) *Service {
	return &Service{
		characters: characters, monsters: monsters, sessions: sessions,
		events: events, narrator: narrator, engine: engine,
		RecentEventLimit: 10,
	}
}

// Request is one player action.
type Request struct {
	CampaignID  string
	CharacterID string
	Input       string

	// Style carries the campaign's voice through to the narration.
	Style ai.NarrationStyle
	// Scene is a line of context for the narrator, such as where the party is.
	Scene string
}

// Result is everything the turn produced.
type Result struct {
	Intent models.Intent `json:"intent"`

	// Exactly one of these is set when the action was resolved mechanically.
	Check  *rules.CheckResult  `json:"check,omitempty"`
	Attack *rules.AttackResult `json:"attack,omitempty"`

	Narration string             `json:"narration"`
	Event     *models.StoryEvent `json:"event,omitempty"`
	Session   string             `json:"session_id"`

	// NeedsClarification is set when the sentence could not be read; nothing
	// was resolved and nothing was logged.
	NeedsClarification bool   `json:"needs_clarification"`
	Clarification      string `json:"clarification,omitempty"`

	TokensUsed int           `json:"tokens_used"`
	Cost       float64       `json:"cost_usd"`
	Elapsed    time.Duration `json:"elapsed"`
}

// TakeAction runs one full turn.
func (s *Service) TakeAction(ctx context.Context, req *Request) (*Result, error) {
	started := time.Now()

	if strings.TrimSpace(req.Input) == "" {
		return nil, models.Invalid("input is required")
	}

	// A turn needs a session to belong to. Without one there is nowhere to log
	// what happened, and an unlogged turn is exactly the hole this closes.
	session, err := s.sessions.GetActiveSession(ctx, req.CampaignID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, models.Invalid("no session is in progress; start one before taking actions")
	}

	actor, err := s.characters.GetCharacterByCharacterID(ctx, req.CharacterID)
	if err != nil {
		return nil, err
	}
	if actor == nil {
		return nil, models.NotFound("character")
	}

	monsters, err := s.monsters.GetMonstersByCampaign(ctx, req.CampaignID)
	if err != nil {
		return nil, err
	}

	// The history is read before the parse so the model can tell "attack it"
	// from "attack the other one".
	history, err := s.events.GetRecentEvents(ctx, req.CampaignID, s.RecentEventLimit)
	if err != nil {
		return nil, err
	}
	recent := models.NarrativeContext(history)

	result := &Result{Session: session.SessionID}

	// --- parse -------------------------------------------------------------
	targets := make([]string, 0, len(monsters))
	byName := make(map[string]*models.Monster, len(monsters))
	for _, m := range monsters {
		if m.HitPoints.Current <= 0 {
			continue // the dead are not targets
		}
		targets = append(targets, m.Name)
		byName[strings.ToLower(m.Name)] = m
	}

	parsed, err := s.narrator.ExtractIntent(ctx, &ai.IntentRequest{
		PlayerInput: req.Input,
		Options:     models.ActionOptionsFor(actor, targets),
		Situation:   situationFrom(session, recent),
	})
	if err != nil {
		return nil, err
	}

	result.Intent = parsed.Intent
	result.TokensUsed += parsed.TokensUsed
	result.Cost += parsed.Cost

	if parsed.Intent.Action == models.IntentUnclear {
		// Nothing happened, so nothing is logged. Asking a question is not an
		// event in the campaign's history.
		result.NeedsClarification = true
		result.Clarification = parsed.Intent.Clarification
		result.Elapsed = time.Since(started)
		return result, nil
	}

	// --- resolve, persist, narrate ------------------------------------------
	var (
		narration *ai.NarrationResponse
		eventType string
		dice      *models.DiceResults
		changes   models.GameStateChanges
	)

	switch parsed.Intent.Action {
	case models.IntentAttack:
		attack, target, err := s.resolveAttack(ctx, req, actor, parsed.Intent, byName)
		if err != nil {
			return nil, err
		}
		result.Attack = attack
		eventType = "combat_action"
		changes = attackChanges(attack, target)

		narration, err = s.narrator.NarrateAction(ctx, &ai.NarrationRequest{
			Facts: attack.Facts(), Context: sceneWith(req.Scene, recent), Style: req.Style,
		})
		if err != nil {
			return nil, err
		}

	case models.IntentSkillCheck, models.IntentSavingThrow:
		check := s.resolveCheck(actor, parsed.Intent)
		result.Check = &check
		eventType = "dice_roll"
		dice = checkDice(check)

		narration, err = s.narrator.NarrateCheck(ctx, &ai.NarrationRequest{
			Facts: check.Facts(), Context: sceneWith(req.Scene, recent), Style: req.Style,
		})
		if err != nil {
			return nil, err
		}

	default:
		// Nothing mechanical happened: describe the scene instead. The history
		// is handed to the narrator here, which is what gives the campaign a
		// memory rather than a series of disconnected moments.
		eventType = narrativeEventType(parsed.Intent.Action)
		prose, err := s.narrator.GenerateNarrative(ctx, &ai.NarrativeRequest{
			PlayerInput:    req.Input,
			Location:       orDefault(req.Scene, "unspecified"),
			PartyStatus:    partyStatus(actor),
			RecentEvents:   recent,
			DMStyle:        orDefault(req.Style.NarrativeVoice, "collaborative"),
			NarrativeVoice: orDefault(req.Style.NarrativeVoice, "third person, present tense"),
			HumorLevel:     "occasional",
			DetailLevel:    "moderate",
		})
		if err != nil {
			return nil, err
		}
		narration = &ai.NarrationResponse{
			Text: prose.Narrative, TokensUsed: prose.TokensUsed,
			Cost: prose.Cost, ProcessingTime: prose.ProcessingTime,
		}
	}

	result.Narration = strings.TrimSpace(narration.Text)
	result.TokensUsed += narration.TokensUsed
	result.Cost += narration.Cost

	if attack := result.Attack; attack != nil {
		dice = attackDice(*attack)
	}

	// --- log ---------------------------------------------------------------
	event := &models.StoryEvent{
		CampaignID: req.CampaignID,
		SessionID:  session.SessionID,
		EventType:  eventType,
		Trigger: models.EventTrigger{
			Type:        "player_action",
			PlayerInput: req.Input,
			Intent:      string(parsed.Intent.Action),
			Target:      parsed.Intent.Target,
		},
		AIContext: models.AIContextInfo{
			// Split across the two calls this turn made.
			PromptTokens:     parsed.TokensUsed,
			CompletionTokens: narration.TokensUsed,
			Temperature:      ai.GetTemperature("action_narration"),
		},
		Narrative: models.NarrativeInfo{
			AIGeneratedText:  result.Narration,
			DMInterpretation: engineVerdict(result),
			DiceResults:      dice,
		},
		GameStateChanges: changes,
		Metadata: models.EventMetadata{
			ProcessingTimeMs: int(time.Since(started).Milliseconds()),
			CostUSD:          result.Cost,
		},
		Timestamp: time.Now().UTC(),
	}

	if err := s.events.AppendEvent(ctx, event); err != nil {
		return nil, err
	}
	result.Event = event
	result.Elapsed = time.Since(started)
	return result, nil
}

// resolveAttack runs an attack and writes the target's new hit points back.
//
// Damage that is not persisted is damage that did not happen, and a monster
// healing to full between turns is the most obvious way for the log and the
// game state to disagree.
func (s *Service) resolveAttack(
	ctx context.Context,
	req *Request,
	actor *models.Character,
	intent models.Intent,
	byName map[string]*models.Monster,
) (*rules.AttackResult, *models.Monster, error) {
	monster, ok := byName[strings.ToLower(intent.Target)]
	if !ok {
		return nil, nil, models.Invalid("%q is not a creature in this campaign", intent.Target)
	}

	weapon, err := weaponFor(actor, intent.Weapon)
	if err != nil {
		return nil, nil, err
	}

	combatant := monster.ToCombatant(monster.MonsterID)
	combatant.HitPoints = monster.HitPoints

	attack, err := s.engine.WeaponAttack(actor, weapon, &combatant, models.RollNormal)
	if err != nil {
		return nil, nil, err
	}

	monster.HitPoints = combatant.HitPoints
	if err := s.monsters.UpdateHitPoints(ctx, req.CampaignID, monster.MonsterID, monster.HitPoints); err != nil {
		return nil, nil, err
	}

	return &attack, monster, nil
}

// resolveCheck runs a skill check or saving throw.
func (s *Service) resolveCheck(actor *models.Character, intent models.Intent) rules.CheckResult {
	dc := intent.SuggestedDC
	if dc == 0 {
		dc = models.DifficultyClasses["medium"]
	}

	if intent.Action == models.IntentSavingThrow {
		ability := intent.Ability
		if !ability.Valid() {
			ability = models.AbilityConstitution
		}
		return s.engine.SavingThrow(actor, ability, dc, models.RollNormal)
	}

	if intent.Skill.Valid() {
		return s.engine.SkillCheck(actor, intent.Skill, dc, models.RollNormal)
	}

	ability := intent.Ability
	if !ability.Valid() {
		ability = models.AbilityDexterity
	}
	return s.engine.AbilityCheck(actor, ability, dc, models.RollNormal)
}

// weaponFor resolves the parser's weapon name to an item the character holds.
func weaponFor(actor *models.Character, name string) (models.InventoryItem, error) {
	for _, w := range actor.Equipment.Weapons {
		if name == "" || strings.EqualFold(w.Name, name) {
			return w, nil
		}
	}
	for _, item := range actor.Inventory {
		if item.Weapon != nil && strings.EqualFold(item.Name, name) {
			return item, nil
		}
	}
	if len(actor.Equipment.Weapons) > 0 {
		return actor.Equipment.Weapons[0], nil
	}
	return models.InventoryItem{}, models.Invalid("%s has no weapon to attack with", actor.Name)
}

// engineVerdict is the engine's own sentence, stored beside the prose so a
// later reader can see what actually happened rather than only how it was told.
func engineVerdict(r *Result) string {
	switch {
	case r.Attack != nil:
		return r.Attack.Summary()
	case r.Check != nil:
		return r.Check.Summary()
	default:
		return ""
	}
}

func checkDice(check rules.CheckResult) *models.DiceResults {
	return &models.DiceResults{
		RollType: string(check.Kind),
		Skill:    check.Skill,
		Ability:  check.Ability,
		Roll:     check.Roll,
		DC:       check.DC,
		Outcome:  string(check.Outcome),
	}
}

func attackDice(attack rules.AttackResult) *models.DiceResults {
	return &models.DiceResults{
		RollType: "attack",
		Roll:     attack.Roll,
		DC:       attack.TargetAC,
		Outcome:  string(attack.Outcome),
	}
}

func attackChanges(attack *rules.AttackResult, target *models.Monster) models.GameStateChanges {
	changes := models.GameStateChanges{
		CharactersInvolved: []string{attack.Attacker, attack.Target},
	}
	if attack.Damage != nil && attack.Damage.Dealt > 0 {
		changes.HPChanges = []models.HPChange{{
			CharacterID: target.MonsterID,
			Amount:      -attack.Damage.Dealt,
			NewHP:       target.HitPoints.Current,
		}}
	}
	return changes
}

func narrativeEventType(action models.IntentAction) string {
	switch action {
	case models.IntentTalk:
		return "dialogue"
	case models.IntentMove:
		return "exploration"
	default:
		return "narrative"
	}
}

func situationFrom(session *models.Session, recent string) string {
	where := strings.TrimSpace(session.Location.CurrentLocation)
	if where == "" {
		where = "an unnamed place"
	}
	return fmt.Sprintf("session %d at %s; recently: %s",
		session.SessionNumber, where, firstLine(recent))
}

func sceneWith(scene, recent string) string {
	scene = strings.TrimSpace(scene)
	if scene == "" {
		scene = "no additional context"
	}
	return fmt.Sprintf("%s\nRecently: %s", scene, firstLine(recent))
}

func partyStatus(actor *models.Character) string {
	hp := actor.CombatStats.HitPoints
	return fmt.Sprintf("%s at %d/%d hit points", actor.Name, hp.Current, hp.Maximum)
}

func firstLine(text string) string {
	if i := strings.LastIndex(text, "\n"); i >= 0 {
		return strings.TrimSpace(text[i+1:])
	}
	return strings.TrimSpace(text)
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dnd-campaign/manager/internal/application/memory"
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
	// CharacterStore reads the acting character and writes back the resources
	// a turn consumed.
	//
	// Spell slots are the only thing a turn currently spends, and writing them
	// back is not optional: an unsaved slot is a spell the character can cast
	// for ever.
	CharacterStore interface {
		GetCharacterByCharacterID(ctx context.Context, characterID string) (*models.Character, error)
		UpdateSpellSlots(ctx context.Context, characterID string, spells models.Spells) error
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
		GetEventsSince(ctx context.Context, campaignID string, since time.Time, limit int) ([]*models.StoryEvent, error)
	}

	// HistoryBuilder decides how much of the campaign a prompt gets to see.
	HistoryBuilder interface {
		Build(ctx context.Context, campaignID string) (memory.Context, error)
	}

	// Compactor folds history too old to send into a rolling summary.
	Compactor interface {
		Compact(ctx context.Context, campaignID string) (bool, error)
	}

	// Narrator is the subset of ai.Service a turn uses.
	Narrator interface {
		ExtractIntent(ctx context.Context, req *ai.IntentRequest) (*ai.IntentResponse, error)
		NarrateAction(ctx context.Context, req *ai.NarrationRequest) (*ai.NarrationResponse, error)
		NarrateCast(ctx context.Context, req *ai.NarrationRequest) (*ai.NarrationResponse, error)
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

	// Memory decides how much of the campaign a prompt sees.
	//
	// It defaults to a budgeted view of recent events with no long-term
	// summary, which is correct for a young campaign. main installs one wired
	// to the campaign store and the provider, so the rolling summary is
	// carried too. Replacing it is how a caller changes the token budget.
	Memory HistoryBuilder

	// Compactor folds old history into a rolling summary. Optional: without
	// one, history past the budget is simply forgotten, which is the correct
	// behaviour for a campaign with no provider wired up.
	//
	// It runs when the budget actually starts cutting events, not on a fixed
	// schedule -- a campaign that fits its budget never pays for a summary it
	// does not need, and one that does not fit compacts exactly once and then
	// fits again.
	Compactor Compactor
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
		Memory: memory.New(events, nil, nil),
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
	Cast   *rules.CastResult   `json:"cast,omitempty"`

	Narration string             `json:"narration"`
	Event     *models.StoryEvent `json:"event,omitempty"`
	Session   string             `json:"session_id"`

	// Warning reports something that went wrong beside the turn itself, such
	// as a failed history compaction. The turn still happened.
	Warning string `json:"warning,omitempty"`

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

	// A dead character can do nothing at all -- not even roll a saving throw --
	// so there is no sentence worth paying a provider to parse.
	if actor.IsDead() {
		return &Result{
			Session:            session.SessionID,
			NeedsClarification: true,
			Clarification:      fmt.Sprintf("%s is dead and can no longer act.", actor.Name),
			Elapsed:            time.Since(started),
		}, nil
	}

	monsters, err := s.monsters.GetMonstersByCampaign(ctx, req.CampaignID)
	if err != nil {
		return nil, err
	}

	// The history is read before the parse so the model can tell "attack it"
	// from "attack the other one". It is budgeted rather than dumped: an
	// oversized request is refused whole, so an unbounded log would eventually
	// cost the turn, not just the history.
	remembered, err := s.Memory.Build(ctx, req.CampaignID)
	if err != nil {
		return nil, err
	}
	recent := remembered.Block()

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

	// What the character may attempt depends on what they turned out to be
	// attempting, so this waits for the parse. A refusal is the player's
	// situation, not an error: nothing is resolved and nothing is logged.
	if refusal := actionRefusal(actor, parsed.Intent.Action); refusal != "" {
		result.NeedsClarification = true
		result.Clarification = refusal
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

	case models.IntentCastSpell:
		cast, target, refusal, err := s.resolveCast(ctx, req, actor, parsed.Intent, byName, result)
		if err != nil {
			return nil, err
		}
		// A refusal is a fact about the situation -- no slots, a spell the
		// character does not know -- not a server error. The player is told
		// and nothing is logged, exactly as for an unreadable sentence.
		if refusal != "" {
			result.NeedsClarification = true
			result.Clarification = refusal
			result.Elapsed = time.Since(started)
			return result, nil
		}

		// A utility spell has no roll for the engine to make. The slot is
		// still spent; the effect is described rather than resolved.
		if cast == nil {
			eventType = "narrative"
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
				Text: prose.Narrative, TokensUsed: prose.TokensUsed, Cost: prose.Cost,
			}
			break
		}

		result.Cast = cast
		eventType = "combat_action"
		changes = castChanges(cast, target)

		narration, err = s.narrator.NarrateCast(ctx, &ai.NarrationRequest{
			Facts: cast.Facts(), Context: sceneWith(req.Scene, recent), Style: req.Style,
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

	// Compaction happens last and cannot fail the turn. The action has already
	// resolved and been logged; losing that to a provider hiccup would be far
	// worse than one oversized prompt next time.
	if remembered.Dropped > 0 && s.Compactor != nil {
		if _, err := s.Compactor.Compact(ctx, req.CampaignID); err != nil {
			result.Warning = fmt.Sprintf("history could not be summarised: %v", err)
		}
	}

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

	attack, err := s.engine.WeaponAttack(actor, weapon, &combatant, intent.Advantage.Mode())
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
// resolveCast turns a parsed cast into an engine result.
//
// It returns (nil, nil, "", nil) for a utility spell, whose effect the narrator
// describes because there is nothing to roll, and a non-empty refusal for
// anything the situation forbids -- an unknown spell, no slots left, a slot too
// small. A refusal is not an error: it is something the player can fix, and
// turning it into a 500 would lose the turn instead of explaining it.
func (s *Service) resolveCast(
	ctx context.Context,
	req *Request,
	actor *models.Character,
	intent models.Intent,
	byName map[string]*models.Monster,
	result *Result,
) (*rules.CastResult, *models.Monster, string, error) {
	def, ok := models.SpellByName(intent.Spell)
	if !ok {
		return nil, nil, fmt.Sprintf(
			"I do not have rules for %q. Which spell is it, or would you rather describe what you do?",
			intent.Spell), nil
	}

	// An omitted slot level means the spell's own level, never zero: casting a
	// levelled spell "at level 0" would make it free.
	slotLevel := intent.SlotLevel
	if slotLevel < def.Level {
		slotLevel = def.Level
	}
	if err := def.ValidateSlot(slotLevel); err != nil {
		return nil, nil, err.Error(), nil
	}
	if slotLevel > 0 && actor.Spells.AvailableSlots(slotLevel) < 1 {
		return nil, nil, fmt.Sprintf(
			"%s has no level %d spell slots left. Cast it from a higher slot, or do something else?",
			actor.Name, slotLevel), nil
	}

	// A utility spell still costs its slot; only the description is the
	// narrator's business.
	if def.Resolution == models.SpellResolutionUtility {
		if slotLevel > 0 {
			if err := actor.Spells.ExpendSlot(slotLevel); err != nil {
				return nil, nil, err.Error(), nil
			}
			if err := s.saveSpells(ctx, req, actor); err != nil {
				return nil, nil, "", err
			}
		}
		return nil, nil, "", nil
	}

	monster, ok := byName[strings.ToLower(intent.Target)]
	if !ok {
		return nil, nil, fmt.Sprintf(
			"%s needs a target. Who is it aimed at?", def.Name), nil
	}

	combatant := monster.ToCombatant(monster.MonsterID)
	combatant.HitPoints = monster.HitPoints

	var (
		cast rules.CastResult
		err  error
	)
	if def.Resolution == models.SpellResolutionSave {
		// The target rolls its own save, from its own statblock.
		cast, err = s.engine.CastSpellVersusSave(actor, def, slotLevel, &combatant,
			monster.SavingThrowModifier(def.SaveAbility))
	} else {
		cast, err = s.engine.CastSpell(actor, def, slotLevel, &combatant, intent.Advantage.Mode())
	}
	if err != nil {
		// The engine refuses what the rules forbid. That is the player's
		// situation, not a fault.
		if errors.Is(err, models.ErrValidation) {
			return nil, nil, err.Error(), nil
		}
		return nil, nil, "", err
	}

	monster.HitPoints = combatant.HitPoints
	if err := s.monsters.UpdateHitPoints(ctx, req.CampaignID, monster.MonsterID, monster.HitPoints); err != nil {
		return nil, nil, "", err
	}

	// A concentration spell replaces whatever was already held: 5e allows
	// exactly one, which is what makes concentration a cost rather than a
	// formality. What the spell imposed is recorded with it, because ending it
	// has to undo it -- otherwise a dropped Hold Person leaves its victim held.
	if def.Concentration {
		held := models.Concentration{
			Spell: def.Name, SlotLevel: slotLevel, Condition: def.Condition,
		}
		if cast.ConditionApplied {
			held.Targets = []string{combatant.CombatantID}
		}
		if replaced := actor.BeginConcentration(held); replaced != "" {
			// Dropping a held spell is something the player needs told: they
			// spent a slot on it and it is gone.
			result.Warning = fmt.Sprintf("concentration on %s ended", replaced)
		}
	}

	// The slot is gone whether or not the spell landed, so it is written back
	// before anything else can fail. Concentration rides along in the same
	// document.
	if slotLevel > 0 || def.Concentration {
		if err := s.saveSpells(ctx, req, actor); err != nil {
			return nil, nil, "", err
		}
	}

	return &cast, monster, "", nil
}

// saveSpells persists the caster's slots.
func (s *Service) saveSpells(ctx context.Context, req *Request, actor *models.Character) error {
	return s.characters.UpdateSpellSlots(ctx, req.CharacterID, actor.Spells)
}

func (s *Service) resolveCheck(actor *models.Character, intent models.Intent) rules.CheckResult {
	dc := intent.SuggestedDC
	if dc == 0 {
		dc = models.DifficultyClasses["medium"]
	}

	// The engine folds in everything it can derive from the character; this is
	// only the situational half the parser read from the sentence.
	situational := intent.Advantage.Mode()

	if intent.Action == models.IntentSavingThrow {
		ability := intent.Ability
		if !ability.Valid() {
			ability = models.AbilityConstitution
		}
		return s.engine.SavingThrow(actor, ability, dc, situational)
	}

	if intent.Skill.Valid() {
		return s.engine.SkillCheck(actor, intent.Skill, dc, situational)
	}

	ability := intent.Ability
	if !ability.Valid() {
		ability = models.AbilityDexterity
	}
	return s.engine.AbilityCheck(actor, ability, dc, situational)
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

// actionRefusal reports why the character may not do what they just tried,
// or "" if they may.
//
// The three capabilities are kept apart because 5e keeps them apart: an
// incapacitated character can still speak, a grappled one can still swing, and
// a saving throw is not an action at all -- an unconscious creature still makes
// them, and that is the whole of a dying character's turn.
func actionRefusal(actor *models.Character, action models.IntentAction) string {
	var (
		allowed bool
		reason  string
	)

	switch action {
	case models.IntentSavingThrow, models.IntentUnclear:
		return ""
	case models.IntentTalk:
		allowed, reason = actor.CanSpeak()
	case models.IntentMove:
		allowed, reason = actor.CanMove()
	default:
		allowed, reason = actor.CanAct()
	}

	if allowed {
		return ""
	}
	return reason
}

func castChanges(cast *rules.CastResult, target *models.Monster) models.GameStateChanges {
	changes := models.GameStateChanges{
		CharactersInvolved: []string{cast.Caster, cast.Target},
	}
	if cast.Damage != nil && cast.Damage.Dealt > 0 {
		changes.HPChanges = []models.HPChange{{
			CharacterID: target.MonsterID,
			Amount:      -cast.Damage.Dealt,
			NewHP:       target.HitPoints.Current,
		}}
	}
	if cast.Healing > 0 {
		changes.HPChanges = append(changes.HPChanges, models.HPChange{
			CharacterID: target.MonsterID,
			Amount:      cast.Healing,
			NewHP:       target.HitPoints.Current,
		})
	}
	if cast.ConditionApplied {
		changes.ConditionsApplied = []models.Condition{cast.Condition}
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

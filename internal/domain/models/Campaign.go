package models

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Campaign represents a D&D campaign
type Campaign struct {
	ID               primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CampaignID       string             `json:"campaign_id" bson:"campaign_id"`
	Title            string             `json:"title" bson:"title"`
	Description      string             `json:"description" bson:"description"`
	Setting          Setting            `json:"setting" bson:"setting"`
	DMSettings       DMSettings         `json:"dm_settings" bson:"dm_settings"`
	AIPersonality    AIPersonality      `json:"ai_personality" bson:"ai_personality"`
	Status           string             `json:"status" bson:"status"`
	CreatedBy        string             `json:"created_by" bson:"created_by"`
	CurrentSessionID string             `json:"current_session_id" bson:"current_session_id"`
	Summary          CampaignSummary    `json:"summary" bson:"summary"`
	CreatedAt        time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" bson:"updated_at"`
}

// CampaignSummary is the rolling recap of everything too old to send verbatim.
//
// It exists because the event log grows without limit and a context window does
// not. Rather than forgetting the first session the moment the tenth begins,
// old events are folded into this and the watermark advances past them.
type CampaignSummary struct {
	Text string `json:"text" bson:"text"`

	// Through is the timestamp of the newest event this summary covers.
	// Everything after it is still sent verbatim.
	//
	// A timestamp rather than a sequence number because sequence numbers
	// restart per session, so they cannot order a whole campaign. Two events
	// sharing a timestamp to the nanosecond would cost one history line, which
	// is why the watermark is compared with a strict "after".
	Through time.Time `json:"through" bson:"through"`

	// EventCount is how many events have been folded in, for reporting.
	EventCount int       `json:"event_count" bson:"event_count"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}

// DMSettings contains dungeon master preferences
type DMSettings struct {
	Tone   string   `json:"tone" bson:"tone"`
	Pacing string   `json:"pacing" bson:"pacing"`
	Themes []string `json:"themes" bson:"themes"`
}

// AIPersonality defines the AI DM's personality
type AIPersonality struct {
	DMStyle        string `json:"dm_style" bson:"dm_style"`
	NarrativeVoice string `json:"narrative_voice" bson:"narrative_voice"`
	HumorLevel     string `json:"humor_level" bson:"humor_level"`
	DetailLevel    string `json:"detail_level" bson:"detail_level"`
}

// StoryProgress used to hold ActivePlotThreads, MainQuestStage and
// CompletedArcs, none of which were ever written.
//
// All three are gone. A bare string cannot be advanced or resolved, a counter
// cannot say what the current act is about, and having two places that claim to
// track one thing is the bug this project keeps finding. They are entities now:
// see PlotThread, Consequence and StoryArc.

// Coordinates represents a position in the game world
type Coordinates struct {
	X int `json:"x" bson:"x"`
	Y int `json:"y" bson:"y"`
}

// StoryEvent represents a narrative event in the campaign
type StoryEvent struct {
	ID               primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	EventID          string             `json:"event_id" bson:"event_id"`
	CampaignID       string             `json:"campaign_id" bson:"campaign_id"`
	SessionID        string             `json:"session_id" bson:"session_id"`
	EventType        string             `json:"event_type" bson:"event_type"` // "narrative", "dialogue", "combat_action", "dice_roll", "exploration"
	SequenceNumber   int                `json:"sequence_number" bson:"sequence_number"`
	Trigger          EventTrigger       `json:"trigger" bson:"trigger"`
	AIContext        AIContextInfo      `json:"ai_context" bson:"ai_context"`
	Narrative        NarrativeInfo      `json:"narrative" bson:"narrative"`
	GameStateChanges GameStateChanges   `json:"game_state_changes" bson:"game_state_changes"`
	Consequences     Consequences       `json:"consequences" bson:"consequences"`
	PlayerReactions  []string           `json:"player_reactions" bson:"player_reactions"`
	Metadata         EventMetadata      `json:"metadata" bson:"metadata"`
	Timestamp        time.Time          `json:"timestamp" bson:"timestamp"`
}

// EventTrigger describes what triggered the event
type EventTrigger struct {
	Type        string `json:"type" bson:"type"` // "player_action", "ai_initiated", "dice_result", "time_passed"
	PlayerInput string `json:"player_input" bson:"player_input"`
	Intent      string `json:"intent" bson:"intent"`
	Target      string `json:"target" bson:"target"`
}

// AIContextInfo contains AI processing information
type AIContextInfo struct {
	PromptTokens        int     `json:"prompt_tokens" bson:"prompt_tokens"`
	CompletionTokens    int     `json:"completion_tokens" bson:"completion_tokens"`
	Model               string  `json:"model" bson:"model"`
	Temperature         float64 `json:"temperature" bson:"temperature"`
	SystemPromptVersion string  `json:"system_prompt_version" bson:"system_prompt_version"`
}

// NarrativeInfo contains the narrative content
type NarrativeInfo struct {
	AIGeneratedText  string       `json:"ai_generated_text" bson:"ai_generated_text"`
	DMInterpretation string       `json:"dm_interpretation" bson:"dm_interpretation"`
	DiceResults      *DiceResults `json:"dice_results" bson:"dice_results"`
}

// GameStateChanges tracks changes to game state
type GameStateChanges struct {
	LocationChanged    bool        `json:"location_changed" bson:"location_changed"`
	NewLocation        string      `json:"new_location" bson:"new_location"`
	CharactersInvolved []string    `json:"characters_involved" bson:"characters_involved"`
	ConditionsApplied  []Condition `json:"conditions_applied" bson:"conditions_applied"`
	ItemsGained        []string    `json:"items_gained" bson:"items_gained"`
	HPChanges          []HPChange  `json:"hp_changes" bson:"hp_changes"`
}

// HPChange represents a change in hit points
type HPChange struct {
	CharacterID string `json:"character_id" bson:"character_id"`
	Amount      int    `json:"amount" bson:"amount"` // Positive for healing, negative for damage
	NewHP       int    `json:"new_hp" bson:"new_hp"`
}

// Consequences describes the results of an event
type Consequences struct {
	Immediate       []string `json:"immediate" bson:"immediate"`
	PotentialFuture []string `json:"potential_future" bson:"potential_future"`
}

// EventMetadata contains processing metadata
type EventMetadata struct {
	ProcessingTimeMs int     `json:"processing_time_ms" bson:"processing_time_ms"`
	CostUSD          float64 `json:"cost_usd" bson:"cost_usd"`
}

// CombatEncounter represents a combat encounter
type CombatEncounter struct {
	ID                primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	EncounterID       string              `json:"encounter_id" bson:"encounter_id"`
	CampaignID        string              `json:"campaign_id" bson:"campaign_id"`
	SessionID         string              `json:"session_id" bson:"session_id"`
	EncounterName     string              `json:"encounter_name" bson:"encounter_name"`
	Description       string              `json:"description" bson:"description"`
	EncounterType     string              `json:"encounter_type" bson:"encounter_type"` // "combat", "social", "environmental"
	Status            string              `json:"status" bson:"status"`                 // "active", "completed", "aborted"
	Combatants        []Combatant         `json:"combatants" bson:"combatants"`
	CombatState       CombatState         `json:"combat_state" bson:"combat_state"`
	TurnHistory       []Turn              `json:"turn_history" bson:"turn_history"`
	DamageLog         []DamageLogEntry    `json:"damage_log" bson:"damage_log"`
	NarrativeLog      []NarrativeLogEntry `json:"narrative_log" bson:"narrative_log"`
	VictoryConditions VictoryConditions   `json:"victory_conditions" bson:"victory_conditions"`
	Treasure          []string            `json:"treasure" bson:"treasure"`
	AISummary         EncounterAISummary  `json:"ai_summary" bson:"ai_summary"`
	CreatedAt         time.Time           `json:"created_at" bson:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at" bson:"updated_at"`
}

// CombatantStatus is a combatant's state in the turn order.
//
// "unconscious" and "dead" alone could not express the state 5e spends the
// most time in: a creature at 0 hit points that is neither stable nor dead and
// is rolling death saves each turn.
type CombatantStatus string

const (
	CombatantActive      CombatantStatus = "active"
	CombatantDying       CombatantStatus = "dying" // at 0 HP, rolling death saves
	CombatantStable      CombatantStatus = "stable"
	CombatantUnconscious CombatantStatus = "unconscious" // from a spell or effect, not from 0 HP
	CombatantDead        CombatantStatus = "dead"
)

// CombatantSource says which collection a combatant's statblock came from,
// since players and NPCs are Characters while hostiles are Monsters.
type CombatantSource string

const (
	SourceCharacter CombatantSource = "character"
	SourceMonster   CombatantSource = "monster"
)

// Combatant represents a participant in combat
type Combatant struct {
	CombatantID string          `json:"combatant_id" bson:"combatant_id"`
	SourceType  CombatantSource `json:"source_type" bson:"source_type"`
	SourceID    string          `json:"source_id" bson:"source_id"` // character_id or monster_id
	Type        string          `json:"type" bson:"type"`           // "player", "enemy", "npc"
	Name        string          `json:"name" bson:"name"`

	Initiative         int `json:"initiative" bson:"initiative"`
	InitiativeModifier int `json:"initiative_modifier" bson:"initiative_modifier"`
	TurnOrder          int `json:"turn_order" bson:"turn_order"`

	HitPoints  HitPoints `json:"hit_points" bson:"hit_points"`
	ArmorClass int       `json:"armor_class" bson:"armor_class"`

	Status     CombatantStatus `json:"status" bson:"status"`
	Conditions []Condition     `json:"conditions" bson:"conditions"`
	Exhaustion int             `json:"exhaustion,omitempty" bson:"exhaustion,omitempty"`

	// Affinities and ConditionImmunities are copied from the statblock or
	// sheet when the encounter starts, so damage resolution never has to
	// reach back to the source document mid-combat.
	Affinities          DamageAffinities `json:"damage_affinities" bson:"damage_affinities"`
	ConditionImmunities []Condition      `json:"condition_immunities,omitempty" bson:"condition_immunities,omitempty"`

	// MakesDeathSaves separates player characters, who fall unconscious and
	// roll to stabilise, from monsters, which die at zero hit points.
	MakesDeathSaves bool `json:"makes_death_saves" bson:"makes_death_saves"`

	// LegendaryResistanceRemaining counts uses left today.
	LegendaryResistanceRemaining int `json:"legendary_resistance_remaining,omitempty" bson:"legendary_resistance_remaining,omitempty"`

	Speed int `json:"speed" bson:"speed"`

	// Action and bonus action are once per turn, so they are flags rather
	// than counters. A reaction is once per round and refreshes at the start
	// of the creature's turn.
	ActionUsed        bool `json:"action_used" bson:"action_used"`
	BonusActionUsed   bool `json:"bonus_action_used" bson:"bonus_action_used"`
	ReactionUsed      bool `json:"reaction_used" bson:"reaction_used"`
	MovementRemaining int  `json:"movement_remaining" bson:"movement_remaining"`

	ConcentratingOn *string    `json:"concentrating_on" bson:"concentrating_on"`
	DeathSaves      DeathSaves `json:"death_saves" bson:"death_saves"`
}

// StartTurn resets the per-turn resources at the start of a combatant's turn.
func (c *Combatant) StartTurn() {
	c.ActionUsed = false
	c.BonusActionUsed = false
	c.ReactionUsed = false
	c.MovementRemaining = c.Speed
}

// AffinityTo reports how this combatant treats a damage type.
func (c *Combatant) AffinityTo(dt DamageType) DamageAffinity {
	return c.Affinities.For(dt)
}

// ImmuneToCondition reports whether a condition cannot affect this combatant.
func (c *Combatant) ImmuneToCondition(cond Condition) bool {
	for _, immune := range c.ConditionImmunities {
		if immune == cond {
			return true
		}
	}
	return false
}

// AddCondition applies a condition unless the combatant is immune to it.
func (c *Combatant) AddCondition(cond Condition) bool {
	if c.ImmuneToCondition(cond) || c.HasCondition(cond) {
		return false
	}
	c.Conditions = append(c.Conditions, cond)
	return true
}

// UseLegendaryResistance turns a failed save into a success, if any uses
// remain.
func (c *Combatant) UseLegendaryResistance() bool {
	if c.LegendaryResistanceRemaining < 1 {
		return false
	}
	c.LegendaryResistanceRemaining--
	return true
}

// HasCondition reports whether a condition is currently applied.
func (c *Combatant) HasCondition(cond Condition) bool {
	for _, have := range c.Conditions {
		if have == cond {
			return true
		}
	}
	return false
}

// TakeDamage applies damage of a type and updates the combatant's status.
//
// The damage type is what makes resistance and immunity mean anything: this
// used to take a bare amount, so a fire-immune creature took full fire damage
// in every actual encounter. It returns the damage dealt after scaling.
//
// The rest encodes 5e's rules around dropping to 0: leftover damage of at
// least the hit point maximum kills outright, damage taken while already down
// adds a death save failure (two on a critical), and anything that does not
// make death saves simply dies.
func (c *Combatant) TakeDamage(amount int, dt DamageType, critical bool) int {
	if c.Status == CombatantDead {
		return 0
	}

	dealt := c.AffinityTo(dt).Apply(amount)
	if dealt <= 0 {
		return 0
	}

	alreadyDown := c.HitPoints.Current == 0
	overflow := c.HitPoints.ApplyDamage(dealt)

	switch {
	case c.HitPoints.IsMassiveDamage(overflow):
		c.Status = CombatantDead
	case !c.MakesDeathSaves && c.HitPoints.Current == 0:
		// Monsters do not linger at zero hit points.
		c.Status = CombatantDead
	case alreadyDown:
		c.DeathSaves.Failures++
		if critical {
			c.DeathSaves.Failures++
		}
		if c.DeathSaves.Dead() {
			c.Status = CombatantDead
		} else {
			c.Status = CombatantDying
		}
	case c.HitPoints.Current == 0:
		c.Status = CombatantDying
		c.DeathSaves.Reset()
	}

	return dealt
}

// Heal restores hit points, bringing a downed combatant back to consciousness
// and clearing any death saves, as regaining even 1 hit point does.
func (c *Combatant) Heal(amount int) {
	if c.Status == CombatantDead || amount <= 0 {
		return
	}
	c.HitPoints.Heal(amount)
	if c.HitPoints.Current > 0 {
		c.Status = CombatantActive
		c.DeathSaves.Reset()
	}
}

// CombatPhase is where an encounter is in its lifecycle.
//
// Rounds and turns alone could not say whether initiative had been rolled yet,
// so every caller had to infer it from whether Round was zero.
type CombatPhase string

const (
	PhaseNotStarted CombatPhase = "not_started"
	PhaseActive     CombatPhase = "active"
	PhaseEnded      CombatPhase = "ended"
)

// CombatantSide separates the party from what it is fighting.
type CombatantSide string

const (
	SideParty CombatantSide = "party"
	SideFoes  CombatantSide = "foes"
)

// Side returns which side a combatant fights on. Players and friendly NPCs are
// the party; everything else opposes it.
func (c *Combatant) Side() CombatantSide {
	if c.Type == "enemy" {
		return SideFoes
	}
	return SideParty
}

// IsDown reports whether a combatant can no longer act.
func (c *Combatant) IsDown() bool {
	return c.Status == CombatantDead ||
		c.Status == CombatantDying ||
		c.Status == CombatantStable ||
		c.Status == CombatantUnconscious
}

// CombatState tracks the current state of combat
type CombatState struct {
	Phase                 CombatPhase `json:"phase" bson:"phase"`
	Round                 int         `json:"round" bson:"round"`
	Turn                  int         `json:"turn" bson:"turn"`
	CombatStartedAt       time.Time   `json:"combat_started_at" bson:"combat_started_at"`
	CombatEndedAt         *time.Time  `json:"combat_ended_at" bson:"combat_ended_at"`
	DurationRounds        int         `json:"duration_rounds" bson:"duration_rounds"`
	EnvironmentConditions []string    `json:"environment_conditions" bson:"environment_conditions"`
}

// Turn represents a single turn in combat
type Turn struct {
	Round        int      `json:"round" bson:"round"`
	Turn         int      `json:"turn" bson:"turn"`
	CombatantID  string   `json:"combatant_id" bson:"combatant_id"`
	Actions      []Action `json:"actions" bson:"actions"`
	Movement     Movement `json:"movement" bson:"movement"`
	BonusActions []Action `json:"bonus_actions" bson:"bonus_actions"`
	Reactions    []Action `json:"reactions" bson:"reactions"`
	FreeActions  []Action `json:"free_actions" bson:"free_actions"`
	Notes        string   `json:"notes" bson:"notes"`
}

// Action represents an action taken in combat
type Action struct {
	ActionType  string        `json:"action_type" bson:"action_type"` // "attack", "spell", "item", "dash", "dodge", "help"
	Description string        `json:"description" bson:"description"`
	Target      string        `json:"target" bson:"target"`
	Attack      *AttackResult `json:"attack,omitempty" bson:"attack,omitempty"`
	Damage      *Damage       `json:"damage,omitempty" bson:"damage,omitempty"`
}

// AttackResult records an attack roll and how it landed.
type AttackResult struct {
	Roll     D20Result     `json:"roll" bson:"roll"`
	TargetAC int           `json:"target_ac" bson:"target_ac"`
	Outcome  AttackOutcome `json:"outcome" bson:"outcome"`
}

// Damage represents damage dealt.
//
// Rolled is what the dice and modifiers produced; Dealt is what the target
// actually lost after resistance, immunity or vulnerability was applied. They
// differ often enough that recording only one loses the explanation.
type Damage struct {
	DamageType DamageType     `json:"damage_type" bson:"damage_type"`
	DamageRoll string         `json:"damage_roll" bson:"damage_roll"` // e.g. "1d8+3"
	Rolled     int            `json:"rolled" bson:"rolled"`
	Affinity   DamageAffinity `json:"affinity" bson:"affinity"`
	Dealt      int            `json:"dealt" bson:"dealt"`
	Critical   bool           `json:"critical" bson:"critical"`
}

// Movement represents movement in combat
type Movement struct {
	Distance int    `json:"distance" bson:"distance"`
	From     string `json:"from" bson:"from"`
	To       string `json:"to" bson:"to"`
}

// DamageLogEntry logs damage dealt
type DamageLogEntry struct {
	Attacker   string     `json:"attacker" bson:"attacker"`
	Target     string     `json:"target" bson:"target"`
	Damage     int        `json:"damage" bson:"damage"`
	DamageType DamageType `json:"damage_type" bson:"damage_type"`
	Round      int        `json:"round" bson:"round"`
	Timestamp  time.Time  `json:"timestamp" bson:"timestamp"`
}

// NarrativeLogEntry logs narrative descriptions
type NarrativeLogEntry struct {
	Text      string    `json:"text" bson:"text"`
	Round     int       `json:"round" bson:"round"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}

// VictoryConditions defines how combat ends
type VictoryConditions struct {
	Type     string  `json:"type" bson:"type"` // "elimination", "capture", "escape", "negotiation"
	Criteria string  `json:"criteria" bson:"criteria"`
	Outcome  *string `json:"outcome" bson:"outcome"`
}

// EncounterAISummary contains AI-generated summary
type EncounterAISummary struct {
	NarrativeSummary string   `json:"narrative_summary" bson:"narrative_summary"`
	Highlights       []string `json:"highlights" bson:"highlights"`
	PivotMoments     []string `json:"pivot_moments" bson:"pivot_moments"`
}

// GameLog represents a raw game log entry
type GameLog struct {
	ID         primitive.ObjectID     `json:"id" bson:"_id,omitempty"`
	LogID      string                 `json:"log_id" bson:"log_id"`
	CampaignID string                 `json:"campaign_id" bson:"campaign_id"`
	SessionID  string                 `json:"session_id" bson:"session_id"`
	LogType    string                 `json:"log_type" bson:"log_type"` // "chat", "action", "system", "dice_roll"
	Content    string                 `json:"content" bson:"content"`
	Speaker    string                 `json:"speaker" bson:"speaker"`
	Metadata   map[string]interface{} `json:"metadata" bson:"metadata"`
	Timestamp  time.Time              `json:"timestamp" bson:"timestamp"`
}

// AIContext stores durable campaign context to feed into prompts.
//
// Retrieval is deliberately not semantic: a single campaign's context fits in
// a prompt window, so the last N story events plus a rolling summary beat an
// embedding index that would need a vector store, an embedding model and a
// reindex on every edit. Select by ContextType and recency.
type AIContext struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ContextID   string             `json:"context_id" bson:"context_id"`
	CampaignID  string             `json:"campaign_id" bson:"campaign_id"`
	ContextType string             `json:"context_type" bson:"context_type"` // "world", "character", "plot", "session"
	Content     string             `json:"content" bson:"content"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" bson:"updated_at"`
}

// NarrativeContext renders events as the recent-history block a prompt takes.
//
// Prompts want a few readable lines, not a JSON dump: a model given the whole
// event structure spends its attention on the shape instead of the story.
func NarrativeContext(events []*StoryEvent) string {
	var b strings.Builder
	for _, event := range events {
		line := strings.TrimSpace(event.Narrative.AIGeneratedText)
		if line == "" {
			line = strings.TrimSpace(event.Trigger.PlayerInput)
		}
		if line == "" {
			line = strings.TrimSpace(event.Narrative.DMInterpretation)
		}
		if line == "" {
			continue
		}

		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- %s", line)
	}

	// An empty history must read as a sentence: a blank value in a prompt
	// reads as an invitation to invent a past.
	if b.Len() == 0 {
		return "nothing has happened yet"
	}
	return b.String()
}

package models

import (
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
	StoryProgress    StoryProgress      `json:"story_progress" bson:"story_progress"`
	CreatedAt        time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" bson:"updated_at"`
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

// StoryProgress tracks campaign story progression
type StoryProgress struct {
	MainQuestStage    int      `json:"main_quest_stage" bson:"main_quest_stage"`
	CompletedArcs     []string `json:"completed_arcs" bson:"completed_arcs"`
	ActivePlotThreads []string `json:"active_plot_threads" bson:"active_plot_threads"`
}

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

// DiceResults contains dice roll information
type DiceResults struct {
	RollType    string `json:"roll_type" bson:"roll_type"` // "ability_check", "saving_throw", "attack", "damage"
	Skill       string `json:"skill" bson:"skill"`
	Modifier    int    `json:"modifier" bson:"modifier"`
	Roll        int    `json:"roll" bson:"roll"`
	Total       int    `json:"total" bson:"total"`
	NaturalRoll int    `json:"natural_roll" bson:"natural_roll"`
	DC          int    `json:"dc" bson:"dc"`
	Outcome     string `json:"outcome" bson:"outcome"` // "success", "failure", "critical_success", "critical_failure"
}

// GameStateChanges tracks changes to game state
type GameStateChanges struct {
	LocationChanged    bool       `json:"location_changed" bson:"location_changed"`
	NewLocation        string     `json:"new_location" bson:"new_location"`
	CharactersInvolved []string   `json:"characters_involved" bson:"characters_involved"`
	ConditionsApplied  []string   `json:"conditions_applied" bson:"conditions_applied"`
	ItemsGained        []string   `json:"items_gained" bson:"items_gained"`
	HPChanges          []HPChange `json:"hp_changes" bson:"hp_changes"`
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

// Combatant represents a participant in combat
type Combatant struct {
	CombatantID        string     `json:"combatant_id" bson:"combatant_id"`
	CharacterID        string     `json:"character_id" bson:"character_id"`
	Type               string     `json:"type" bson:"type"` // "player", "enemy", "npc"
	Name               string     `json:"name" bson:"name"`
	Initiative         int        `json:"initiative" bson:"initiative"`
	InitiativeModifier int        `json:"initiative_modifier" bson:"initiative_modifier"`
	TurnOrder          int        `json:"turn_order" bson:"turn_order"`
	CurrentHP          int        `json:"current_hp" bson:"current_hp"`
	MaxHP              int        `json:"max_hp" bson:"max_hp"`
	TemporaryHP        int        `json:"temporary_hp" bson:"temporary_hp"`
	ArmorClass         int        `json:"armor_class" bson:"armor_class"`
	Status             string     `json:"status" bson:"status"` // "active", "unconscious", "dead"
	Conditions         []string   `json:"conditions" bson:"conditions"`
	ActionsTaken       int        `json:"actions_taken" bson:"actions_taken"`
	BonusActionsTaken  int        `json:"bonus_actions_taken" bson:"bonus_actions_taken"`
	ReactionsRemaining int        `json:"reactions_remaining" bson:"reactions_remaining"`
	MovementRemaining  int        `json:"movement_remaining" bson:"movement_remaining"`
	ConcentratingOn    *string    `json:"concentrating_on" bson:"concentrating_on"`
	DeathSaves         DeathSaves `json:"death_saves" bson:"death_saves"`
}

// DeathSaves tracks death saving throws
type DeathSaves struct {
	Successes int `json:"successes" bson:"successes"`
	Failures  int `json:"failures" bson:"failures"`
}

// CombatState tracks the current state of combat
type CombatState struct {
	Round                 int        `json:"round" bson:"round"`
	Turn                  int        `json:"turn" bson:"turn"`
	CombatStartedAt       time.Time  `json:"combat_started_at" bson:"combat_started_at"`
	CombatEndedAt         *time.Time `json:"combat_ended_at" bson:"combat_ended_at"`
	DurationRounds        int        `json:"duration_rounds" bson:"duration_rounds"`
	EnvironmentConditions []string   `json:"environment_conditions" bson:"environment_conditions"`
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
	ActionType  string      `json:"action_type" bson:"action_type"` // "attack", "spell", "item", "dash", "dodge", "help"
	Description string      `json:"description" bson:"description"`
	Target      string      `json:"target" bson:"target"`
	RollResult  *RollResult `json:"roll_result" bson:"roll_result"`
	Damage      *Damage     `json:"damage" bson:"damage"`
}

// RollResult contains the result of a roll
type RollResult struct {
	Roll     int  `json:"roll" bson:"roll"`
	Natural  int  `json:"natural" bson:"natural"`
	Modifier int  `json:"modifier" bson:"modifier"`
	Total    int  `json:"total" bson:"total"`
	Hit      bool `json:"hit" bson:"hit"`
	Critical bool `json:"critical" bson:"critical"`
}

// Damage represents damage dealt
type Damage struct {
	DamageType     string `json:"damage_type" bson:"damage_type"`
	DamageRoll     string `json:"damage_roll" bson:"damage_roll"`
	DamageTotal    int    `json:"damage_total" bson:"damage_total"`
	DamageModifier string `json:"damage_modifier" bson:"damage_modifier"`
	HitPoints      int    `json:"hit_points" bson:"hit_points"`
}

// Movement represents movement in combat
type Movement struct {
	Distance int    `json:"distance" bson:"distance"`
	From     string `json:"from" bson:"from"`
	To       string `json:"to" bson:"to"`
}

// DamageLogEntry logs damage dealt
type DamageLogEntry struct {
	Attacker   string    `json:"attacker" bson:"attacker"`
	Target     string    `json:"target" bson:"target"`
	Damage     int       `json:"damage" bson:"damage"`
	DamageType string    `json:"damage_type" bson:"damage_type"`
	Round      int       `json:"round" bson:"round"`
	Timestamp  time.Time `json:"timestamp" bson:"timestamp"`
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

// AIContext stores AI context for the campaign
type AIContext struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ContextID   string             `json:"context_id" bson:"context_id"`
	CampaignID  string             `json:"campaign_id" bson:"campaign_id"`
	ContextType string             `json:"context_type" bson:"context_type"` // "world", "character", "plot", "session"
	Content     string             `json:"content" bson:"content"`
	Embedding   []float64          `json:"embedding" bson:"embedding"` // Vector embedding for semantic search
	Relevance   float64            `json:"relevance" bson:"relevance"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at" bson:"updated_at"`
}

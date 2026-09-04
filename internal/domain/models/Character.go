package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Character represents a player character or NPC in a campaign
type Character struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CharacterID string             `json:"character_id" bson:"character_id"`
	CampaignID  string             `json:"campaign_id" bson:"campaign_id"`
	Type        string             `json:"type" bson:"type"`
	Name        string             `json:"name" bson:"name"`
	PlayerName  string             `json:"player_name,omitempty" bson:"player_name,omitempty"`

	BasicInfo            BasicInfo       `json:"basic_info" bson:"basic_info"`
	AbilityScores        AbilityScores   `json:"ability_scores" bson:"ability_scores"`
	DerivedStats         DerivedStats    `json:"derived_stats" bson:"derived_stats"`
	Skills               Skills          `json:"skills" bson:"skills"`
	SavingThrows         SavingThrows    `json:"saving_throws" bson:"saving_throws"`
	Inventory            []InventoryItem `json:"inventory" bson:"inventory"`
	Equipment            Equipment       `json:"equipment" bson:"equipment"`
	Spells               Spells          `json:"spells" bson:"spells"`
	FeaturesAndAbilities []Feature       `json:"features_and_abilities" bson:"features_and_abilities"`
	BackgroundStory      BackgroundStory `json:"background_story" bson:"background_story"`
	StatusEffects        []string        `json:"status_effects" bson:"status_effects"`
	Conditions           []string        `json:"conditions" bson:"conditions"`
	Relationships        []Relationship  `json:"relationships" bson:"relationships"`

	AIMetadata AIMetadata `json:"ai_metadata" bson:"ai_metadata"`

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// BasicInfo contains basic character information
type BasicInfo struct {
	Race             string `json:"race" bson:"race"`
	Class            string `json:"class" bson:"class"`
	Background       string `json:"background" bson:"background"`
	Level            int    `json:"level" bson:"level"`
	ExperiencePoints int    `json:"experience_points" bson:"experience_points"`
	Alignment        string `json:"alignment" bson:"alignment"`
}

// AbilityScores represents the six core D&D ability scores
type AbilityScores struct {
	Strength     int `json:"strength" bson:"strength"`
	Dexterity    int `json:"dexterity" bson:"dexterity"`
	Constitution int `json:"constitution" bson:"constitution"`
	Intelligence int `json:"intelligence" bson:"intelligence"`
	Wisdom       int `json:"wisdom" bson:"wisdom"`
	Charisma     int `json:"charisma" bson:"charisma"`
}

// DerivedStats contains calculated stats based on ability scores
type DerivedStats struct {
	HitPoints          HitPoints `json:"hit_points" bson:"hit_points"`
	ArmorClass         int       `json:"armor_class" bson:"armor_class"`
	ProficiencyBonus   int       `json:"proficiency_bonus" bson:"proficiency_bonus"`
	InitiativeModifier int       `json:"initiative_modifier" bson:"initiative_modifier"`
	Speed              int       `json:"speed" bson:"speed"`
	PassivePerception  int       `json:"passive_perception" bson:"passive_perception"`
}

// HitPoints represents character health
type HitPoints struct {
	Current   int `json:"current" bson:"current"`
	Maximum   int `json:"maximum" bson:"maximum"`
	Temporary int `json:"temporary" bson:"temporary"`
}

// Skills represents proficiency in D&D skills
type Skills struct {
	Acrobatics     bool `json:"acrobatics" bson:"acrobatics"`
	AnimalHandling bool `json:"animal_handling" bson:"animal_handling"`
	Arcana         bool `json:"arcana" bson:"arcana"`
	Athletics      bool `json:"athletics" bson:"athletics"`
	Deception      bool `json:"deception" bson:"deception"`
	History        bool `json:"history" bson:"history"`
	Insight        bool `json:"insight" bson:"insight"`
	Intimidation   bool `json:"intimidation" bson:"intimidation"`
	Investigation  bool `json:"investigation" bson:"investigation"`
	Medicine       bool `json:"medicine" bson:"medicine"`
	Nature         bool `json:"nature" bson:"nature"`
	Perception     bool `json:"perception" bson:"perception"`
	Performance    bool `json:"performance" bson:"performance"`
	Persuasion     bool `json:"persuasion" bson:"persuasion"`
	Religion       bool `json:"religion" bson:"religion"`
	SleightOfHand  bool `json:"sleight_of_hand" bson:"sleight_of_hand"`
	Stealth        bool `json:"stealth" bson:"stealth"`
	Survival       bool `json:"survival" bson:"survival"`
}

// SavingThrows represents proficiency in saving throws
type SavingThrows struct {
	Strength     bool `json:"strength" bson:"strength"`
	Dexterity    bool `json:"dexterity" bson:"dexterity"`
	Constitution bool `json:"constitution" bson:"constitution"`
	Intelligence bool `json:"intelligence" bson:"intelligence"`
	Wisdom       bool `json:"wisdom" bson:"wisdom"`
	Charisma     bool `json:"charisma" bson:"charisma"`
}

// InventoryItem represents an item in character's inventory
type InventoryItem struct {
	ItemID      string  `json:"item_id" bson:"item_id"`
	Name        string  `json:"name" bson:"name"`
	Quantity    int     `json:"quantity" bson:"quantity"`
	Weight      float64 `json:"weight" bson:"weight"`
	Equipped    bool    `json:"equipped" bson:"equipped"`
	Description string  `json:"description" bson:"description"`
}

// Equipment represents equipped items
type Equipment struct {
	Armor       *InventoryItem  `json:"armor" bson:"armor"`
	Shield      *InventoryItem  `json:"shield" bson:"shield"`
	Weapons     []InventoryItem `json:"weapons" bson:"weapons"`
	Accessories []InventoryItem `json:"accessories" bson:"accessories"`
}

// Feature represents a character feature or ability
type Feature struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
	UsesPerDay  *int   `json:"uses_per_day" bson:"uses_per_day"` // nil means unlimited
}

// Spells contains spellcasting information
type Spells struct {
	SpellcastingAbility string              `json:"spellcasting_ability" bson:"spellcasting_ability"`
	SpellSaveDC         int                 `json:"spell_save_dc" bson:"spell_save_dc"`
	SpellAttackModifier int                 `json:"spell_attack_modifier" bson:"spell_attack_modifier"`
	CantripsKnown       []string            `json:"cantrips_known" bson:"cantrips_known"`
	SpellsKnown         map[string][]string `json:"spells_known" bson:"spells_known"` // key: spell level (e.g., "1st", "2nd")
}

// BackgroundStory contains character narrative information
type BackgroundStory struct {
	GeneratedByAI     bool     `json:"generated_by_ai" bson:"generated_by_ai"`
	Backstory         string   `json:"backstory" bson:"backstory"`
	PersonalityTraits []string `json:"personality_traits" bson:"personality_traits"`
	Ideals            []string `json:"ideals" bson:"ideals"`
	Bonds             []string `json:"bonds" bson:"bonds"`
	Flaws             []string `json:"flaws" bson:"flaws"`
}

// Relationship represents a relationship between characters
type Relationship struct {
	CharacterID   string    `json:"character_id" bson:"character_id"`     // ID of the related character
	CharacterName string    `json:"character_name" bson:"character_name"` // Name for quick reference
	RelationType  string    `json:"relation_type" bson:"relation_type"`   // e.g., "ally", "enemy", "friend", "rival", "family", "mentor", "student"
	Strength      int       `json:"strength" bson:"strength"`             // Relationship strength: -100 (hostile) to 100 (devoted)
	Description   string    `json:"description" bson:"description"`       // Narrative description of the relationship
	History       string    `json:"history" bson:"history"`               // How they met, shared experiences
	Notes         string    `json:"notes" bson:"notes"`                   // Additional notes
	CreatedAt     time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" bson:"updated_at"`
}

// AIMetadata contains AI-specific character information
type AIMetadata struct {
	Personality   string            `json:"personality" bson:"personality"`
	SpeechPattern string            `json:"speech_pattern" bson:"speech_pattern"`
	Motivation    string            `json:"motivation" bson:"motivation"`
	Quirks        []string          `json:"quirks" bson:"quirks"`
	VoiceStyle    string            `json:"voice_style" bson:"voice_style"`
	BehaviorNotes string            `json:"behavior_notes" bson:"behavior_notes"`
	Relationships map[string]string `json:"relationships" bson:"relationships"` // Deprecated: use Relationships array instead
}

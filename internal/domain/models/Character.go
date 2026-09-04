package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CharacterType distinguishes the kinds of creature stored as a Character.
//
// Hostile creatures are no longer one of these: a dragon is not a levelled
// class character, and lives in Monster with a proper statblock.
type CharacterType string

const (
	CharacterPlayer CharacterType = "player"
	CharacterNPC    CharacterType = "npc"
)

// Character represents a player character or NPC in a campaign
type Character struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CharacterID string             `json:"character_id" bson:"character_id"`
	CampaignID  string             `json:"campaign_id" bson:"campaign_id"`
	Type        CharacterType      `json:"type" bson:"type"`
	Name        string             `json:"name" bson:"name"`
	PlayerName  string             `json:"player_name,omitempty" bson:"player_name,omitempty"`

	BasicInfo     BasicInfo     `json:"basic_info" bson:"basic_info"`
	AbilityScores AbilityScores `json:"ability_scores" bson:"ability_scores"`
	CombatStats   CombatStats   `json:"combat_stats" bson:"combat_stats"`

	Skills       SkillProficiencies       `json:"skills" bson:"skills"`
	SavingThrows SavingThrowProficiencies `json:"saving_throws" bson:"saving_throws"`

	Inventory            []InventoryItem `json:"inventory" bson:"inventory"`
	Equipment            Equipment       `json:"equipment" bson:"equipment"`
	Spells               Spells          `json:"spells" bson:"spells"`
	FeaturesAndAbilities []Feature       `json:"features_and_abilities" bson:"features_and_abilities"`
	BackgroundStory      BackgroundStory `json:"background_story" bson:"background_story"`

	// Conditions is the closed 5e set. Exhaustion is separate because it has
	// six degrees rather than being present or absent.
	Conditions []Condition `json:"conditions" bson:"conditions"`
	Exhaustion int         `json:"exhaustion" bson:"exhaustion"`

	Relationships []Relationship `json:"relationships" bson:"relationships"`

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

// CombatStats holds the values that are genuinely stored rather than derived.
//
// Armor class, proficiency bonus, initiative modifier, passive perception and
// the spell save DC used to be fields here. They are all pure functions of
// ability scores, level and equipment, so storing them guaranteed they would
// drift the first time a character levelled up or changed armor. They are
// methods on Character now; only the bonuses that nothing can infer -- magic
// items, feats like Alert -- remain as data.
type CombatStats struct {
	HitPoints  HitPoints  `json:"hit_points" bson:"hit_points"`
	HitDice    HitDice    `json:"hit_dice" bson:"hit_dice"`
	DeathSaves DeathSaves `json:"death_saves" bson:"death_saves"`
	Speed      int        `json:"speed" bson:"speed"`

	// ArmorClassBonus covers effects equipment cannot express, such as a ring
	// of protection or the mage armor spell.
	ArmorClassBonus int `json:"armor_class_bonus,omitempty" bson:"armor_class_bonus,omitempty"`

	// InitiativeBonus covers feats and features on top of the Dexterity
	// modifier, such as Alert's +5.
	InitiativeBonus int `json:"initiative_bonus,omitempty" bson:"initiative_bonus,omitempty"`
}

// HitPoints represents character health
type HitPoints struct {
	Current   int `json:"current" bson:"current"`
	Maximum   int `json:"maximum" bson:"maximum"`
	Temporary int `json:"temporary" bson:"temporary"`
}

// ApplyDamage subtracts damage and reports how much was left over after
// current hit points reached zero.
//
// Temporary hit points absorb damage first, current hit points never go below
// zero, and the overflow is returned rather than discarded because 5e needs
// it: damage that exceeds the target's hit point maximum after dropping them
// to 0 kills outright instead of knocking them unconscious.
func (h *HitPoints) ApplyDamage(amount int) (overflow int) {
	if amount <= 0 {
		return 0
	}

	if h.Temporary > 0 {
		absorbed := amount
		if absorbed > h.Temporary {
			absorbed = h.Temporary
		}
		h.Temporary -= absorbed
		amount -= absorbed
	}

	h.Current -= amount
	if h.Current < 0 {
		overflow = -h.Current
		h.Current = 0
	}
	return overflow
}

// IsMassiveDamage reports whether leftover damage kills outright.
func (h HitPoints) IsMassiveDamage(overflow int) bool {
	return overflow >= h.Maximum && h.Maximum > 0
}

// Heal restores hit points up to the maximum. Temporary hit points are not
// affected: healing never restores them.
func (h *HitPoints) Heal(amount int) {
	if amount <= 0 {
		return
	}
	h.Current += amount
	if h.Current > h.Maximum {
		h.Current = h.Maximum
	}
}

// AddTemporary grants temporary hit points.
//
// They do not stack: a new grant replaces the old one only if it is larger,
// and the recipient otherwise keeps what they have.
func (h *HitPoints) AddTemporary(amount int) {
	if amount > h.Temporary {
		h.Temporary = amount
	}
}

// DeathSaves tracks death saving throws
type DeathSaves struct {
	Successes int `json:"successes" bson:"successes"`
	Failures  int `json:"failures" bson:"failures"`
}

// DeathSaveThreshold is the number of successes that stabilises a creature,
// and the number of failures that kills it.
const DeathSaveThreshold = 3

// Stabilised reports whether three successes have been recorded.
func (d DeathSaves) Stabilised() bool { return d.Successes >= DeathSaveThreshold }

// Dead reports whether three failures have been recorded.
func (d DeathSaves) Dead() bool { return d.Failures >= DeathSaveThreshold }

// Reset clears both tallies, as regaining any hit points does.
func (d *DeathSaves) Reset() { d.Successes, d.Failures = 0, 0 }

// Feature represents a character feature or ability
type Feature struct {
	Name        string `json:"name" bson:"name"`
	Description string `json:"description" bson:"description"`
	UsesPerDay  *int   `json:"uses_per_day" bson:"uses_per_day"` // nil means unlimited
	UsesSpent   int    `json:"uses_spent,omitempty" bson:"uses_spent,omitempty"`
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
	RelationType  string    `json:"relation_type" bson:"relation_type"`   // e.g. "ally", "enemy", "friend", "rival", "family", "mentor", "student"
	Strength      int       `json:"strength" bson:"strength"`             // Relationship strength: -100 (hostile) to 100 (devoted)
	Description   string    `json:"description" bson:"description"`       // Narrative description of the relationship
	History       string    `json:"history" bson:"history"`               // How they met, shared experiences
	Notes         string    `json:"notes" bson:"notes"`                   // Additional notes
	CreatedAt     time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" bson:"updated_at"`
}

// AIMetadata contains AI-specific character information
type AIMetadata struct {
	Personality   string   `json:"personality" bson:"personality"`
	SpeechPattern string   `json:"speech_pattern" bson:"speech_pattern"`
	Motivation    string   `json:"motivation" bson:"motivation"`
	Quirks        []string `json:"quirks" bson:"quirks"`
	VoiceStyle    string   `json:"voice_style" bson:"voice_style"`
	BehaviorNotes string   `json:"behavior_notes" bson:"behavior_notes"`
}

// ---------------------------------------------------------------------------
// Derived values. Each is a pure function of the stored data, so a character
// can never disagree with itself about its own numbers.
// ---------------------------------------------------------------------------

// ProficiencyBonus returns the bonus for the character's level.
func (c *Character) ProficiencyBonus() int {
	return ProficiencyBonusForLevel(c.BasicInfo.Level)
}

// AbilityModifier returns the modifier for one ability.
func (c *Character) AbilityModifier(a Ability) int {
	return c.AbilityScores.Modifier(a)
}

// SkillModifier returns the total modifier for a skill check: the governing
// ability's modifier plus whatever share of the proficiency bonus applies.
func (c *Character) SkillModifier(s Skill) int {
	return c.AbilityModifier(s.Ability()) +
		c.Skills.Level(s).Bonus(c.ProficiencyBonus())
}

// SavingThrowModifier returns the total modifier for a saving throw.
func (c *Character) SavingThrowModifier(a Ability) int {
	return c.AbilityModifier(a) +
		c.SavingThrows.Level(a).Bonus(c.ProficiencyBonus())
}

// ArmorClass computes AC from equipped armor and Dexterity, plus any bonus
// that equipment cannot express.
func (c *Character) ArmorClass() int {
	return c.Equipment.ArmorClass(c.AbilityModifier(AbilityDexterity)) +
		c.CombatStats.ArmorClassBonus
}

// InitiativeModifier is the Dexterity modifier plus feat bonuses.
func (c *Character) InitiativeModifier() int {
	return c.AbilityModifier(AbilityDexterity) + c.CombatStats.InitiativeBonus
}

// PassivePerception is 10 plus the character's Perception check modifier.
func (c *Character) PassivePerception() int {
	return 10 + c.SkillModifier(SkillPerception)
}

// SpellSaveDC is 8 + proficiency bonus + spellcasting ability modifier.
func (c *Character) SpellSaveDC() int {
	if !c.Spells.SpellcastingAbility.Valid() {
		return 0
	}
	return 8 + c.ProficiencyBonus() + c.AbilityModifier(c.Spells.SpellcastingAbility)
}

// SpellAttackModifier is proficiency bonus + spellcasting ability modifier.
func (c *Character) SpellAttackModifier() int {
	if !c.Spells.SpellcastingAbility.Valid() {
		return 0
	}
	return c.ProficiencyBonus() + c.AbilityModifier(c.Spells.SpellcastingAbility)
}

// CarryingCapacity is Strength score times 15, in pounds.
func (c *Character) CarryingCapacity() int {
	return c.AbilityScores.Strength * 15
}

// HasCondition reports whether a condition is currently applied.
func (c *Character) HasCondition(cond Condition) bool {
	for _, have := range c.Conditions {
		if have == cond {
			return true
		}
	}
	return false
}

// AddCondition applies a condition, ignoring duplicates.
func (c *Character) AddCondition(cond Condition) {
	if !c.HasCondition(cond) {
		c.Conditions = append(c.Conditions, cond)
	}
}

// RemoveCondition clears a condition.
func (c *Character) RemoveCondition(cond Condition) {
	for i, have := range c.Conditions {
		if have == cond {
			c.Conditions = append(c.Conditions[:i], c.Conditions[i+1:]...)
			return
		}
	}
}

// ConcentrationSaveDC is the Constitution save required to keep concentrating
// after taking damage: DC 10, or half the damage taken if that is higher.
func ConcentrationSaveDC(damage int) int {
	if half := damage / 2; half > 10 {
		return half
	}
	return 10
}

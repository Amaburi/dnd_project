package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Session represents a game session within a campaign
type Session struct {
	ID               primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	SessionID        string             `json:"session_id" bson:"session_id"`
	CampaignID       string             `json:"campaign_id" bson:"campaign_id"`
	SessionNumber    int                `json:"session_number" bson:"session_number"`
	Title            string             `json:"title" bson:"title"`
	Date             SessionDate        `json:"date" bson:"date"`
	Participants     []Participant      `json:"participants" bson:"participants"`
	Location         SessionLocation    `json:"location" bson:"location"`
	SessionSummary   SessionSummary     `json:"session_summary" bson:"session_summary"`
	DiceRollsSummary DiceRollsSummary   `json:"dice_rolls_summary" bson:"dice_rolls_summary"`
	CombatEncounters []string           `json:"combat_encounters" bson:"combat_encounters"` // Encounter IDs
	AIInteractions   AIInteractions     `json:"ai_interactions" bson:"ai_interactions"`
	Status           string             `json:"status" bson:"status"` // "completed", "in_progress", "scheduled", "cancelled"
	Notes            string             `json:"notes" bson:"notes"`
	CreatedAt        time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at" bson:"updated_at"`
}

// SessionDate contains session timing information
type SessionDate struct {
	Planned     time.Time  `json:"planned" bson:"planned"`
	ActualStart *time.Time `json:"actual_start,omitempty" bson:"actual_start,omitempty"`
	ActualEnd   *time.Time `json:"actual_end,omitempty" bson:"actual_end,omitempty"`
}

// Participant represents a session participant
type Participant struct {
	CharacterID   string     `json:"character_id" bson:"character_id"`
	CharacterName string     `json:"character_name" bson:"character_name"`
	PlayerName    string     `json:"player_name" bson:"player_name"`
	Attendance    string     `json:"attendance" bson:"attendance"` // "present", "absent", "late"
	JoinedAt      *time.Time `json:"joined_at,omitempty" bson:"joined_at,omitempty"`
	LeftAt        *time.Time `json:"left_at,omitempty" bson:"left_at,omitempty"`
}

// SessionLocation represents the current location in the game
type SessionLocation struct {
	CurrentLocation string       `json:"current_location" bson:"current_location"`
	Coordinates     *Coordinates `json:"coordinates,omitempty" bson:"coordinates,omitempty"`
	Environment     string       `json:"environment" bson:"environment"`
}

// SessionSummary contains a summary of the session
type SessionSummary struct {
	NarrativeSummary string   `json:"narrative_summary" bson:"narrative_summary"`
	KeyEvents        []string `json:"key_events" bson:"key_events"`
	TreasureFound    []string `json:"treasure_found" bson:"treasure_found"`
	Deaths           []string `json:"deaths" bson:"deaths"`
}

// DiceRollsSummary contains statistics about dice rolls
type DiceRollsSummary struct {
	TotalRolls  int     `json:"total_rolls" bson:"total_rolls"`
	Natural20s  int     `json:"natural_20s" bson:"natural_20s"`
	Natural1s   int     `json:"natural_1s" bson:"natural_1s"`
	AverageRoll float64 `json:"average_roll" bson:"average_roll"`
}

// AIInteractions tracks AI usage in a session
type AIInteractions struct {
	TotalPrompts    int     `json:"total_prompts" bson:"total_prompts"`
	TotalTokensUsed int     `json:"total_tokens_used" bson:"total_tokens_used"`
	CostEstimate    float64 `json:"cost_estimate" bson:"cost_estimate"`
}

// Session status constants
const (
	SessionStatusScheduled  = "scheduled"
	SessionStatusInProgress = "in_progress"
	SessionStatusCompleted  = "completed"
	SessionStatusCancelled  = "cancelled"
)

// Attendance constants
const (
	AttendancePresent = "present"
	AttendanceAbsent  = "absent"
	AttendanceLate    = "late"
)

// SessionCreateRequest represents the request to create a new session
type SessionCreateRequest struct {
	CampaignID    string    `json:"campaign_id" binding:"required"`
	SessionNumber int       `json:"session_number" binding:"required,min=1"`
	Title         string    `json:"title" binding:"required"`
	PlannedDate   time.Time `json:"planned_date" binding:"required"`
	Notes         string    `json:"notes"`
}

// SessionUpdateRequest represents the request to update a session
type SessionUpdateRequest struct {
	Title            *string           `json:"title,omitempty"`
	Status           *string           `json:"status,omitempty"`
	Location         *SessionLocation  `json:"location,omitempty"`
	SessionSummary   *SessionSummary   `json:"session_summary,omitempty"`
	DiceRollsSummary *DiceRollsSummary `json:"dice_rolls_summary,omitempty"`
	Notes            *string           `json:"notes,omitempty"`
}

// SessionResponse represents the response for a session
type SessionResponse struct {
	ID               string           `json:"id"`
	SessionID        string           `json:"session_id"`
	CampaignID       string           `json:"campaign_id"`
	SessionNumber    int              `json:"session_number"`
	Title            string           `json:"title"`
	Date             SessionDate      `json:"date"`
	Participants     []Participant    `json:"participants"`
	Location         SessionLocation  `json:"location"`
	SessionSummary   SessionSummary   `json:"session_summary"`
	DiceRollsSummary DiceRollsSummary `json:"dice_rolls_summary"`
	CombatEncounters []string         `json:"combat_encounters"`
	AIInteractions   AIInteractions   `json:"ai_interactions"`
	Status           string           `json:"status"`
	Notes            string           `json:"notes"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// ToResponse converts Session model to SessionResponse
func (s *Session) ToResponse() SessionResponse {
	return SessionResponse{
		ID:               s.ID.Hex(),
		SessionID:        s.SessionID,
		CampaignID:       s.CampaignID,
		SessionNumber:    s.SessionNumber,
		Title:            s.Title,
		Date:             s.Date,
		Participants:     s.Participants,
		Location:         s.Location,
		SessionSummary:   s.SessionSummary,
		DiceRollsSummary: s.DiceRollsSummary,
		CombatEncounters: s.CombatEncounters,
		AIInteractions:   s.AIInteractions,
		Status:           s.Status,
		Notes:            s.Notes,
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

// AddParticipant adds a participant to the session
func (s *Session) AddParticipant(participant Participant) {
	s.Participants = append(s.Participants, participant)
	s.UpdatedAt = time.Now()
}

// RemoveParticipant removes a participant from the session
func (s *Session) RemoveParticipant(characterID string) {
	for i, p := range s.Participants {
		if p.CharacterID == characterID {
			s.Participants = append(s.Participants[:i], s.Participants[i+1:]...)
			s.UpdatedAt = time.Now()
			break
		}
	}
}

// UpdateParticipantAttendance updates a participant's attendance
func (s *Session) UpdateParticipantAttendance(characterID string, attendance string) {
	for i, p := range s.Participants {
		if p.CharacterID == characterID {
			s.Participants[i].Attendance = attendance
			s.UpdatedAt = time.Now()
			break
		}
	}
}

// StartSession marks the session as started
func (s *Session) StartSession() {
	now := time.Now()
	s.Date.ActualStart = &now
	s.Status = SessionStatusInProgress
	s.UpdatedAt = now
}

// EndSession marks the session as completed
func (s *Session) EndSession() {
	now := time.Now()
	s.Date.ActualEnd = &now
	s.Status = SessionStatusCompleted
	s.UpdatedAt = now
}

// AddCombatEncounter adds a combat encounter ID to the session
func (s *Session) AddCombatEncounter(encounterID string) {
	s.CombatEncounters = append(s.CombatEncounters, encounterID)
	s.UpdatedAt = time.Now()
}

// UpdateAIInteractions updates AI interaction statistics
func (s *Session) UpdateAIInteractions(prompts, tokens int, cost float64) {
	s.AIInteractions.TotalPrompts += prompts
	s.AIInteractions.TotalTokensUsed += tokens
	s.AIInteractions.CostEstimate += cost
	s.UpdatedAt = time.Now()
}

// UpdateDiceRollStats updates dice roll statistics
func (s *Session) UpdateDiceRollStats(roll int, isNatural20, isNatural1 bool) {
	s.DiceRollsSummary.TotalRolls++
	if isNatural20 {
		s.DiceRollsSummary.Natural20s++
	}
	if isNatural1 {
		s.DiceRollsSummary.Natural1s++
	}

	// Recalculate average
	currentTotal := s.DiceRollsSummary.AverageRoll * float64(s.DiceRollsSummary.TotalRolls-1)
	s.DiceRollsSummary.AverageRoll = (currentTotal + float64(roll)) / float64(s.DiceRollsSummary.TotalRolls)
	s.UpdatedAt = time.Now()
}

// IsActive checks if the session is currently active
func (s *Session) IsActive() bool {
	return s.Status == SessionStatusInProgress
}

// IsCompleted checks if the session is completed
func (s *Session) IsCompleted() bool {
	return s.Status == SessionStatusCompleted
}

// GetDuration returns the duration of the session if it has ended
func (s *Session) GetDuration() *time.Duration {
	if s.Date.ActualStart != nil && s.Date.ActualEnd != nil {
		duration := s.Date.ActualEnd.Sub(*s.Date.ActualStart)
		return &duration
	}
	return nil
}

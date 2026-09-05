package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EncounterRepository persists combat encounters.
//
// An encounter carries its own history -- turn by turn, every point of damage,
// every line of narration -- so a fight can be reviewed after it ends rather
// than surviving only as a memory of who won.
type EncounterRepository struct {
	client *Client
}

// NewEncounterRepository creates a new encounter repository.
func NewEncounterRepository(client *Client) *EncounterRepository {
	return &EncounterRepository{client: client}
}

func (r *EncounterRepository) collection() *mongo.Collection {
	return r.client.Database().Collection(string(CombatEncounters))
}

// CreateEncounter stores a new encounter.
func (r *EncounterRepository) CreateEncounter(ctx context.Context, e *models.CombatEncounter) error {
	if e.CampaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if e.EncounterName == "" {
		return models.Invalid("encounter name is required")
	}

	// The _id is assigned by MongoDB; a client-supplied one is ignored.
	e.ID = primitive.NilObjectID

	if e.EncounterID == "" {
		e.EncounterID = primitive.NewObjectID().Hex()
	}
	if e.EncounterType == "" {
		e.EncounterType = "combat"
	}
	if e.Status == "" {
		e.Status = "active"
	}
	if e.CombatState.Phase == "" {
		e.CombatState.Phase = models.PhaseNotStarted
	}
	if e.CombatState.CombatStartedAt.IsZero() {
		e.CombatState.CombatStartedAt = time.Now().UTC()
	}
	if e.Combatants == nil {
		e.Combatants = []models.Combatant{}
	}

	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now

	result, err := r.collection().InsertOne(ctx, e)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Invalid("encounter_id %q already exists", e.EncounterID)
		}
		return fmt.Errorf("failed to insert encounter: %w", err)
	}

	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		e.ID = id
	}
	return nil
}

// GetEncounterInCampaign retrieves an encounter scoped to its campaign.
func (r *EncounterRepository) GetEncounterInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) (*models.CombatEncounter, error) {
	var e models.CombatEncounter
	err := r.collection().FindOne(ctx, bson.M{"_id": id, "campaign_id": campaignID}).Decode(&e)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to find encounter: %w", err)
	}
	return &e, nil
}

// findEncounters runs a filter, newest first.
func (r *EncounterRepository) findEncounters(ctx context.Context, filter bson.M) ([]*models.CombatEncounter, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find encounters: %w", err)
	}
	defer cursor.Close(ctx)

	var encounters []*models.CombatEncounter
	if err = cursor.All(ctx, &encounters); err != nil {
		return nil, fmt.Errorf("failed to decode encounters: %w", err)
	}
	if encounters == nil {
		// Encode an empty result as [] rather than null.
		encounters = []*models.CombatEncounter{}
	}
	return encounters, nil
}

// GetEncountersByCampaign returns every encounter in a campaign.
func (r *EncounterRepository) GetEncountersByCampaign(ctx context.Context, campaignID string) ([]*models.CombatEncounter, error) {
	return r.findEncounters(ctx, bson.M{"campaign_id": campaignID})
}

// GetEncountersBySession returns the encounters fought in one session.
func (r *EncounterRepository) GetEncountersBySession(ctx context.Context, campaignID, sessionID string) ([]*models.CombatEncounter, error) {
	return r.findEncounters(ctx, bson.M{"campaign_id": campaignID, "session_id": sessionID})
}

// GetActiveEncounter returns the fight currently under way, if any.
func (r *EncounterRepository) GetActiveEncounter(ctx context.Context, campaignID string) (*models.CombatEncounter, error) {
	var e models.CombatEncounter
	err := r.collection().FindOne(ctx, bson.M{
		"campaign_id":        campaignID,
		"combat_state.phase": bson.M{"$in": []models.CombatPhase{models.PhaseNotStarted, models.PhaseActive}},
	}).Decode(&e)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find the active encounter: %w", err)
	}
	return &e, nil
}

// SaveEncounter writes back the state a tracker changed.
//
// The whole encounter is rewritten on purpose: combat mutates combatants,
// state, and three separate logs at once, and picking those apart into
// targeted updates would be more code and more ways to save half a turn. The
// immutable fields are still excluded.
func (r *EncounterRepository) SaveEncounter(ctx context.Context, campaignID string, e *models.CombatEncounter) error {
	if e.ID.IsZero() {
		return models.Invalid("encounter ID is required")
	}
	if campaignID == "" {
		return models.Invalid("campaign_id is required")
	}

	now := time.Now().UTC()
	update := bson.M{
		"session_id":         e.SessionID,
		"encounter_name":     e.EncounterName,
		"description":        e.Description,
		"encounter_type":     e.EncounterType,
		"status":             e.Status,
		"combatants":         emptyIfNil(e.Combatants),
		"combat_state":       e.CombatState,
		"turn_history":       emptyIfNil(e.TurnHistory),
		"damage_log":         emptyIfNil(e.DamageLog),
		"narrative_log":      emptyIfNil(e.NarrativeLog),
		"victory_conditions": e.VictoryConditions,
		"treasure":           emptyIfNil(e.Treasure),
		"ai_summary":         e.AISummary,
		"updated_at":         now,
	}

	result, err := r.collection().UpdateOne(
		ctx,
		bson.M{"_id": e.ID, "campaign_id": campaignID},
		bson.M{"$set": update},
	)
	if err != nil {
		return fmt.Errorf("failed to save encounter: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("encounter")
	}

	e.CampaignID = campaignID
	e.UpdatedAt = now
	return nil
}

// DeleteEncounterInCampaign deletes an encounter scoped to its campaign.
func (r *EncounterRepository) DeleteEncounterInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) error {
	result, err := r.collection().DeleteOne(ctx, bson.M{"_id": id, "campaign_id": campaignID})
	if err != nil {
		return fmt.Errorf("failed to delete encounter: %w", err)
	}
	if result.DeletedCount == 0 {
		return models.NotFound("encounter")
	}
	return nil
}

// DeleteEncountersByCampaign deletes every encounter in a campaign.
func (r *EncounterRepository) DeleteEncountersByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.collection().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete encounters: %w", err)
	}
	return nil
}

// EncounterStats is the after-action summary of a fight.
type EncounterStats struct {
	Rounds         int            `json:"rounds"`
	TotalDamage    int            `json:"total_damage"`
	DamageBySource map[string]int `json:"damage_by_source"`
	Downed         []string       `json:"downed"`
	Survivors      []string       `json:"survivors"`
	Outcome        string         `json:"outcome,omitempty"`
}

// Stats summarises an encounter once it is over.
//
// This is the analytics the roadmap asks for, computed from the logs rather
// than accumulated during the fight: a derived number cannot drift from the
// events it derives from.
func Stats(e *models.CombatEncounter) EncounterStats {
	stats := EncounterStats{
		Rounds:         e.CombatState.Round,
		DamageBySource: map[string]int{},
	}

	for _, entry := range e.DamageLog {
		stats.TotalDamage += entry.Damage
		stats.DamageBySource[entry.Attacker] += entry.Damage
	}

	for _, c := range e.Combatants {
		if c.IsDown() {
			stats.Downed = append(stats.Downed, c.Name)
			continue
		}
		stats.Survivors = append(stats.Survivors, c.Name)
	}

	if e.VictoryConditions.Outcome != nil {
		stats.Outcome = *e.VictoryConditions.Outcome
	}
	return stats
}

// GetEncounterByEncounterID looks an encounter up by its business ID.
func (r *EncounterRepository) GetEncounterByEncounterID(ctx context.Context, campaignID, encounterID string) (*models.CombatEncounter, error) {
	encounters, err := r.findEncounters(ctx, bson.M{"campaign_id": campaignID, "encounter_id": encounterID})
	if err != nil {
		return nil, err
	}
	if len(encounters) == 0 {
		return nil, nil
	}
	return encounters[0], nil
}

// SaveEncounterState is SaveEncounter under the name the encounter service
// uses, so that service depends on what it does rather than on this type.
func (r *EncounterRepository) SaveEncounterState(ctx context.Context, campaignID string, e *models.CombatEncounter) error {
	return r.SaveEncounter(ctx, campaignID, e)
}

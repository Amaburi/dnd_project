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

// SessionRepository handles game session database operations.
type SessionRepository struct {
	client *Client
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(client *Client) *SessionRepository {
	return &SessionRepository{client: client}
}

func (r *SessionRepository) collection() *mongo.Collection {
	return r.client.Database().Collection(string(Sessions))
}

// CreateSession stores a new session.
func (r *SessionRepository) CreateSession(ctx context.Context, session *models.Session) error {
	if session.CampaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if session.Title == "" {
		return models.Invalid("session title is required")
	}

	// The _id is assigned by MongoDB; a client-supplied one is ignored.
	session.ID = primitive.NilObjectID

	if session.SessionID == "" {
		session.SessionID = primitive.NewObjectID().Hex()
	}

	// Session numbers run in sequence within a campaign, so the caller does
	// not have to know what the last one was.
	if session.SessionNumber < 1 {
		next, err := r.NextSessionNumber(ctx, session.CampaignID)
		if err != nil {
			return err
		}
		session.SessionNumber = next
	}

	if session.Status == "" {
		session.Status = models.SessionStatusScheduled
	}
	if session.Participants == nil {
		session.Participants = []models.Participant{}
	}
	if session.CombatEncounters == nil {
		session.CombatEncounters = []string{}
	}

	now := time.Now().UTC()
	session.CreatedAt = now
	session.UpdatedAt = now

	result, err := r.collection().InsertOne(ctx, session)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Invalid("session_id %q already exists", session.SessionID)
		}
		return fmt.Errorf("failed to insert session: %w", err)
	}

	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		session.ID = id
	}
	return nil
}

// NextSessionNumber returns the number the next session in a campaign takes.
func (r *SessionRepository) NextSessionNumber(ctx context.Context, campaignID string) (int, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "session_number", Value: -1}})

	var last models.Session
	err := r.collection().FindOne(ctx, bson.M{"campaign_id": campaignID}, opts).Decode(&last)
	switch {
	case err == mongo.ErrNoDocuments:
		return 1, nil
	case err != nil:
		return 0, fmt.Errorf("failed to read the last session number: %w", err)
	}
	return last.SessionNumber + 1, nil
}

// GetSessionInCampaign retrieves a session scoped to its campaign.
func (r *SessionRepository) GetSessionInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) (*models.Session, error) {
	var session models.Session
	err := r.collection().FindOne(ctx, bson.M{"_id": id, "campaign_id": campaignID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to find session: %w", err)
	}
	return &session, nil
}

// GetSessionBySessionID retrieves a session by its business ID, which is what
// story events reference.
func (r *SessionRepository) GetSessionBySessionID(ctx context.Context, campaignID, sessionID string) (*models.Session, error) {
	var session models.Session
	err := r.collection().FindOne(ctx, bson.M{"campaign_id": campaignID, "session_id": sessionID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find session: %w", err)
	}
	return &session, nil
}

// findSessions runs a filter and decodes the result set, newest first.
func (r *SessionRepository) findSessions(ctx context.Context, filter bson.M) ([]*models.Session, error) {
	opts := options.Find().SetSort(bson.D{{Key: "session_number", Value: -1}})

	cursor, err := r.collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find sessions: %w", err)
	}
	defer cursor.Close(ctx)

	var sessions []*models.Session
	if err = cursor.All(ctx, &sessions); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}
	if sessions == nil {
		// Encode an empty result as [] rather than null.
		sessions = []*models.Session{}
	}
	return sessions, nil
}

// GetSessionsByCampaign returns every session in a campaign, newest first.
func (r *SessionRepository) GetSessionsByCampaign(ctx context.Context, campaignID string) ([]*models.Session, error) {
	return r.findSessions(ctx, bson.M{"campaign_id": campaignID})
}

// GetActiveSession returns the session currently in progress, if any.
//
// A campaign has at most one: StartSession closes any other that was left
// open, because two live sessions would make "which one does this event
// belong to" unanswerable.
func (r *SessionRepository) GetActiveSession(ctx context.Context, campaignID string) (*models.Session, error) {
	var session models.Session
	err := r.collection().FindOne(ctx, bson.M{
		"campaign_id": campaignID,
		"status":      models.SessionStatusInProgress,
	}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find the active session: %w", err)
	}
	return &session, nil
}

// UpdateSession updates the mutable fields of a session.
//
// As elsewhere, the update document is built explicitly: $set-ing the decoded
// struct would blank the uniquely indexed session_id, detach the session from
// its campaign and zero created_at.
func (r *SessionRepository) UpdateSession(ctx context.Context, campaignID string, session *models.Session) error {
	if session.ID.IsZero() {
		return models.Invalid("session ID is required")
	}
	if campaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if session.Title == "" {
		return models.Invalid("session title is required")
	}

	now := time.Now().UTC()
	update := bson.M{
		"session_number":     session.SessionNumber,
		"title":              session.Title,
		"date":               session.Date,
		"participants":       emptyIfNil(session.Participants),
		"location":           session.Location,
		"session_summary":    session.SessionSummary,
		"dice_rolls_summary": session.DiceRollsSummary,
		"combat_encounters":  emptyIfNil(session.CombatEncounters),
		"ai_interactions":    session.AIInteractions,
		"status":             session.Status,
		"notes":              session.Notes,
		"updated_at":         now,
	}

	result, err := r.collection().UpdateOne(
		ctx,
		bson.M{"_id": session.ID, "campaign_id": campaignID},
		bson.M{"$set": update},
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("session")
	}

	session.CampaignID = campaignID
	session.UpdatedAt = now
	return nil
}

// StartSession marks a session in progress and closes any other that was left
// open in the same campaign.
func (r *SessionRepository) StartSession(ctx context.Context, campaignID string, id primitive.ObjectID) (*models.Session, error) {
	session, err := r.GetSessionInCampaign(ctx, campaignID, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, models.NotFound("session")
	}
	if session.IsCompleted() {
		return nil, models.Invalid("session %d has already ended", session.SessionNumber)
	}

	// Only one session runs at a time; a forgotten one is closed rather than
	// left to collect events belonging to this one.
	if _, err := r.collection().UpdateMany(ctx,
		bson.M{
			"campaign_id": campaignID,
			"status":      models.SessionStatusInProgress,
			"_id":         bson.M{"$ne": id},
		},
		bson.M{"$set": bson.M{
			"status":          models.SessionStatusCompleted,
			"date.actual_end": time.Now().UTC(),
			"updated_at":      time.Now().UTC(),
		}},
	); err != nil {
		return nil, fmt.Errorf("failed to close the previous session: %w", err)
	}

	session.StartSession()
	if err := r.applyLifecycle(ctx, campaignID, session); err != nil {
		return nil, err
	}
	return session, nil
}

// EndSession marks a session completed.
func (r *SessionRepository) EndSession(ctx context.Context, campaignID string, id primitive.ObjectID) (*models.Session, error) {
	session, err := r.GetSessionInCampaign(ctx, campaignID, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, models.NotFound("session")
	}
	if !session.IsActive() {
		return nil, models.Invalid("session %d is not in progress", session.SessionNumber)
	}

	session.EndSession()
	if err := r.applyLifecycle(ctx, campaignID, session); err != nil {
		return nil, err
	}
	return session, nil
}

// applyLifecycle writes the fields the model's lifecycle methods touched.
func (r *SessionRepository) applyLifecycle(ctx context.Context, campaignID string, session *models.Session) error {
	result, err := r.collection().UpdateOne(ctx,
		bson.M{"_id": session.ID, "campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"status":     session.Status,
			"date":       session.Date,
			"updated_at": session.UpdatedAt,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update the session's state: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("session")
	}
	return nil
}

// RecordDiceRoll folds one roll into the session's running statistics.
//
// This is where dice history accrues: individual rolls live on their story
// events, and the session keeps the totals so a summary does not have to
// replay the whole log.
func (r *SessionRepository) RecordDiceRoll(ctx context.Context, campaignID, sessionID string, natural int) error {
	session, err := r.GetSessionBySessionID(ctx, campaignID, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return models.NotFound("session")
	}

	session.UpdateDiceRollStats(natural, natural == 20, natural == 1)

	_, err = r.collection().UpdateOne(ctx,
		bson.M{"_id": session.ID},
		bson.M{"$set": bson.M{
			"dice_rolls_summary": session.DiceRollsSummary,
			"updated_at":         session.UpdatedAt,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to record the dice roll: %w", err)
	}
	return nil
}

// RecordAIUsage folds one AI call into the session's running totals, which is
// how token spend is tracked over an evening rather than per request.
func (r *SessionRepository) RecordAIUsage(ctx context.Context, campaignID, sessionID string, prompts, tokens int, cost float64) error {
	result, err := r.collection().UpdateOne(ctx,
		bson.M{"campaign_id": campaignID, "session_id": sessionID},
		bson.M{
			"$inc": bson.M{
				"ai_interactions.total_prompts":     prompts,
				"ai_interactions.total_tokens_used": tokens,
				"ai_interactions.cost_estimate":     cost,
			},
			"$set": bson.M{"updated_at": time.Now().UTC()},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to record AI usage: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("session")
	}
	return nil
}

// DeleteSessionInCampaign deletes a session scoped to its campaign.
func (r *SessionRepository) DeleteSessionInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) error {
	result, err := r.collection().DeleteOne(ctx, bson.M{"_id": id, "campaign_id": campaignID})
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	if result.DeletedCount == 0 {
		return models.NotFound("session")
	}
	return nil
}

// DeleteSessionsByCampaign deletes every session in a campaign.
func (r *SessionRepository) DeleteSessionsByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.collection().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete sessions: %w", err)
	}
	return nil
}

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

// Event types a story event may carry.
const (
	EventNarrative    = "narrative"
	EventDialogue     = "dialogue"
	EventCombatAction = "combat_action"
	EventDiceRoll     = "dice_roll"
	EventExploration  = "exploration"
)

// maxAppendRetries bounds how many times AppendEvent re-reads the sequence
// after losing a race for the next number.
const maxAppendRetries = 5

// StoryEventRepository handles the append-only log of what happened in play.
//
// This is the campaign's memory. Everything downstream depends on it: the AI's
// sense of recent events, the session's dice statistics, and any later account
// of how the party got here.
type StoryEventRepository struct {
	client   *Client
	sessions *SessionRepository
}

// NewStoryEventRepository creates a new story event repository.
//
// It holds the session repository because appending an event also folds its
// dice roll and AI usage into the session's running totals -- keeping those in
// step is the point of writing through one place.
func NewStoryEventRepository(client *Client, sessions *SessionRepository) *StoryEventRepository {
	return &StoryEventRepository{client: client, sessions: sessions}
}

func (r *StoryEventRepository) collection() *mongo.Collection {
	return r.client.Database().Collection(string(StoryEvents))
}

// AppendEvent adds an event to the end of a session's log.
//
// The log is append-only: there is no update method, because rewriting what
// happened is how a campaign's history and its state drift apart. Correcting
// the record means appending a correction.
func (r *StoryEventRepository) AppendEvent(ctx context.Context, event *models.StoryEvent) error {
	if event.CampaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if event.SessionID == "" {
		return models.Invalid("session_id is required")
	}
	if event.EventType == "" {
		event.EventType = EventNarrative
	}

	// The _id is assigned by MongoDB; a client-supplied one is ignored.
	event.ID = primitive.NilObjectID
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.PlayerReactions == nil {
		event.PlayerReactions = []string{}
	}

	var lastErr error
	for attempt := 0; attempt < maxAppendRetries; attempt++ {
		next, err := r.NextSequenceNumber(ctx, event.CampaignID, event.SessionID)
		if err != nil {
			return err
		}
		event.SequenceNumber = next
		// A fresh id each attempt: the previous one may be what collided.
		event.EventID = primitive.NewObjectID().Hex()

		result, err := r.collection().InsertOne(ctx, event)
		if err == nil {
			if id, ok := result.InsertedID.(primitive.ObjectID); ok {
				event.ID = id
			}
			r.foldIntoSession(ctx, event)
			return nil
		}
		if !mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("failed to append story event: %w", err)
		}
		// Another writer took this sequence number; read it again.
		lastErr = err
	}

	return fmt.Errorf("failed to append story event after %d attempts: %w", maxAppendRetries, lastErr)
}

// foldIntoSession updates the running totals an event contributes to.
//
// Failures here are deliberately not fatal: the event itself is already
// durable, and losing a statistic is a smaller harm than rejecting a write
// that succeeded.
func (r *StoryEventRepository) foldIntoSession(ctx context.Context, event *models.StoryEvent) {
	if r.sessions == nil {
		return
	}

	if dice := event.Narrative.DiceResults; dice != nil && dice.Roll.Natural > 0 {
		_ = r.sessions.RecordDiceRoll(ctx, event.CampaignID, event.SessionID, dice.Roll.Natural)
	}
	if event.AIContext.PromptTokens > 0 || event.AIContext.CompletionTokens > 0 {
		tokens := event.AIContext.PromptTokens + event.AIContext.CompletionTokens
		_ = r.sessions.RecordAIUsage(ctx, event.CampaignID, event.SessionID, 1, tokens, event.Metadata.CostUSD)
	}
}

// NextSequenceNumber returns the position the next event in a session takes.
func (r *StoryEventRepository) NextSequenceNumber(ctx context.Context, campaignID, sessionID string) (int, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "sequence_number", Value: -1}})

	var last models.StoryEvent
	err := r.collection().FindOne(ctx,
		bson.M{"campaign_id": campaignID, "session_id": sessionID}, opts).Decode(&last)
	switch {
	case err == mongo.ErrNoDocuments:
		return 1, nil
	case err != nil:
		return 0, fmt.Errorf("failed to read the last sequence number: %w", err)
	}
	return last.SequenceNumber + 1, nil
}

// findEvents runs a filter with an explicit sort.
func (r *StoryEventRepository) findEvents(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*models.StoryEvent, error) {
	cursor, err := r.collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find story events: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*models.StoryEvent
	if err = cursor.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("failed to decode story events: %w", err)
	}
	if events == nil {
		// Encode an empty result as [] rather than null.
		events = []*models.StoryEvent{}
	}
	return events, nil
}

// GetEventsBySession returns a session's log in the order it happened.
func (r *StoryEventRepository) GetEventsBySession(ctx context.Context, campaignID, sessionID string) ([]*models.StoryEvent, error) {
	opts := options.Find().SetSort(bson.D{{Key: "sequence_number", Value: 1}})
	return r.findEvents(ctx, bson.M{"campaign_id": campaignID, "session_id": sessionID}, opts)
}

// GetEventsByType filters a session's log by kind, which is how a dice history
// is read back.
func (r *StoryEventRepository) GetEventsByType(ctx context.Context, campaignID, sessionID, eventType string) ([]*models.StoryEvent, error) {
	opts := options.Find().SetSort(bson.D{{Key: "sequence_number", Value: 1}})
	return r.findEvents(ctx, bson.M{
		"campaign_id": campaignID,
		"session_id":  sessionID,
		"event_type":  eventType,
	}, opts)
}

// DefaultRecentEvents is how many events feed the AI's sense of "recently"
// when a caller does not say.
const DefaultRecentEvents = 10

// GetRecentEvents returns the most recent events in a campaign, oldest first.
//
// This is what fills the "Recent Events" a narration prompt asks for. The query
// takes the newest N and then reverses them, because a prompt reads better in
// the order things happened while the interesting events are the latest ones.
func (r *StoryEventRepository) GetRecentEvents(ctx context.Context, campaignID string, limit int) ([]*models.StoryEvent, error) {
	if limit < 1 {
		limit = DefaultRecentEvents
	}
	if limit > 200 {
		limit = 200
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(int64(limit))

	events, err := r.findEvents(ctx, bson.M{"campaign_id": campaignID}, opts)
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

// CountEventsBySession returns how many events a session has logged.
func (r *StoryEventRepository) CountEventsBySession(ctx context.Context, campaignID, sessionID string) (int64, error) {
	count, err := r.collection().CountDocuments(ctx,
		bson.M{"campaign_id": campaignID, "session_id": sessionID})
	if err != nil {
		return 0, fmt.Errorf("failed to count story events: %w", err)
	}
	return count, nil
}

// DeleteEventsByCampaign deletes every event in a campaign.
func (r *StoryEventRepository) DeleteEventsByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.collection().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete story events: %w", err)
	}
	return nil
}

// DeleteEventsBySession deletes a single session's log.
func (r *StoryEventRepository) DeleteEventsBySession(ctx context.Context, campaignID, sessionID string) error {
	if _, err := r.collection().DeleteMany(ctx,
		bson.M{"campaign_id": campaignID, "session_id": sessionID}); err != nil {
		return fmt.Errorf("failed to delete story events: %w", err)
	}
	return nil
}

// GetEventsSince returns events newer than a watermark, oldest first.
//
// This is what a rolling summary makes possible: everything the summary already
// covers is never read, parsed or paid for again. The comparison is a strict
// "after", so the event that set the watermark is not returned twice.
func (r *StoryEventRepository) GetEventsSince(ctx context.Context, campaignID string, since time.Time, limit int) ([]*models.StoryEvent, error) {
	if limit < 1 {
		limit = DefaultRecentEvents
	}
	if limit > 200 {
		limit = 200
	}

	// Sorted newest-first and then reversed, like GetRecentEvents: when more
	// than `limit` events have accumulated the interesting ones are the latest,
	// but a prompt reads better in the order things happened.
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(int64(limit))

	filter := bson.M{"campaign_id": campaignID, "timestamp": bson.M{"$gt": since}}
	events, err := r.findEvents(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, nil
}

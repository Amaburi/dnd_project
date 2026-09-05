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

// StoryRepository stores the DM's outstanding work: plot threads and the
// consequences the party has set in motion.
//
// One repository for both because they are read together -- the memory block
// asks for everything still owed, not for threads and then separately for
// consequences.
type StoryRepository struct {
	client *Client
}

// NewStoryRepository creates a new story repository.
func NewStoryRepository(client *Client) *StoryRepository {
	return &StoryRepository{client: client}
}

func (r *StoryRepository) threads() *mongo.Collection {
	return r.client.Database().Collection(string(PlotThreads))
}

func (r *StoryRepository) consequences() *mongo.Collection {
	return r.client.Database().Collection(string(Consequences))
}

// --- plot threads -----------------------------------------------------------

// CreateThread opens a plot thread.
func (r *StoryRepository) CreateThread(ctx context.Context, thread *models.PlotThread) error {
	if thread.Status == "" {
		thread.Status = models.ThreadOpen
	}
	if thread.Urgency == "" {
		thread.Urgency = models.ThreadActive
	}
	if err := thread.Validate(); err != nil {
		return err
	}

	if thread.ThreadID == "" {
		thread.ThreadID = primitive.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	thread.OpenedAt, thread.UpdatedAt = now, now

	if _, err := r.threads().InsertOne(ctx, thread); err != nil {
		return fmt.Errorf("failed to create plot thread: %w", err)
	}
	return nil
}

func (r *StoryRepository) findThreads(ctx context.Context, filter bson.M) ([]*models.PlotThread, error) {
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	cursor, err := r.threads().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find plot threads: %w", err)
	}
	defer cursor.Close(ctx)

	threads := []*models.PlotThread{}
	if err := cursor.All(ctx, &threads); err != nil {
		return nil, fmt.Errorf("failed to decode plot threads: %w", err)
	}
	return threads, nil
}

// GetThreadsByCampaign lists every thread, open or finished.
func (r *StoryRepository) GetThreadsByCampaign(ctx context.Context, campaignID string) ([]*models.PlotThread, error) {
	return r.findThreads(ctx, bson.M{"campaign_id": campaignID})
}

// GetLiveThreads lists the threads still outstanding.
//
// This is what reaches a prompt, so it filters in the query rather than in Go:
// a campaign with two hundred resolved threads should not decode them all to
// throw them away.
func (r *StoryRepository) GetLiveThreads(ctx context.Context, campaignID string) ([]*models.PlotThread, error) {
	return r.findThreads(ctx, bson.M{
		"campaign_id": campaignID,
		"status":      bson.M{"$in": []models.ThreadStatus{models.ThreadOpen, models.ThreadDormant}},
	})
}

// GetThreadByThreadID looks one thread up.
func (r *StoryRepository) GetThreadByThreadID(ctx context.Context, campaignID, threadID string) (*models.PlotThread, error) {
	threads, err := r.findThreads(ctx, bson.M{"campaign_id": campaignID, "thread_id": threadID})
	if err != nil {
		return nil, err
	}
	if len(threads) == 0 {
		return nil, nil
	}
	return threads[0], nil
}

// SaveThread writes a thread back after it has been advanced or resolved.
//
// Field by field, as everywhere here: thread_id is uniquely indexed and
// opened_at is immutable, and $set-ing the struct would blank both.
func (r *StoryRepository) SaveThread(ctx context.Context, campaignID string, thread *models.PlotThread) error {
	if thread.ThreadID == "" {
		return models.Invalid("thread_id is required")
	}
	if err := thread.Validate(); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := r.threads().UpdateOne(
		ctx,
		bson.M{"thread_id": thread.ThreadID, "campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"title":            thread.Title,
			"summary":          thread.Summary,
			"status":           thread.Status,
			"urgency":          thread.Urgency,
			"involves":         thread.Involves,
			"beats":            thread.Beats,
			"resolution":       thread.Resolution,
			"resolved_session": thread.ResolvedSession,
			"resolved_at":      thread.ResolvedAt,
			"updated_at":       now,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to save plot thread: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("plot thread")
	}
	thread.UpdatedAt = now
	return nil
}

// DeleteThreadsByCampaign removes every thread in a campaign.
func (r *StoryRepository) DeleteThreadsByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.threads().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete plot threads: %w", err)
	}
	return nil
}

// --- consequences -----------------------------------------------------------

// CreateConsequence records something the party set in motion.
func (r *StoryRepository) CreateConsequence(ctx context.Context, c *models.Consequence) error {
	if c.Status == "" {
		c.Status = models.ConsequencePending
	}
	if c.Severity == "" {
		c.Severity = models.SeverityModerate
	}
	if err := c.Validate(); err != nil {
		return err
	}

	if c.ConsequenceID == "" {
		c.ConsequenceID = primitive.NewObjectID().Hex()
	}
	c.CreatedAt = time.Now().UTC()

	if _, err := r.consequences().InsertOne(ctx, c); err != nil {
		return fmt.Errorf("failed to create consequence: %w", err)
	}
	return nil
}

func (r *StoryRepository) findConsequences(ctx context.Context, filter bson.M) ([]*models.Consequence, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := r.consequences().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find consequences: %w", err)
	}
	defer cursor.Close(ctx)

	out := []*models.Consequence{}
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("failed to decode consequences: %w", err)
	}
	return out, nil
}

// GetConsequencesByCampaign lists every consequence, settled or not.
func (r *StoryRepository) GetConsequencesByCampaign(ctx context.Context, campaignID string) ([]*models.Consequence, error) {
	return r.findConsequences(ctx, bson.M{"campaign_id": campaignID})
}

// GetPendingConsequences lists what the party is still owed.
func (r *StoryRepository) GetPendingConsequences(ctx context.Context, campaignID string) ([]*models.Consequence, error) {
	return r.findConsequences(ctx, bson.M{
		"campaign_id": campaignID,
		"status":      bson.M{"$in": []any{models.ConsequencePending, ""}},
	})
}

// GetConsequenceByID looks one consequence up.
func (r *StoryRepository) GetConsequenceByID(ctx context.Context, campaignID, consequenceID string) (*models.Consequence, error) {
	found, err := r.findConsequences(ctx, bson.M{"campaign_id": campaignID, "consequence_id": consequenceID})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

// SaveConsequence writes a consequence back after it has landed or been averted.
func (r *StoryRepository) SaveConsequence(ctx context.Context, campaignID string, c *models.Consequence) error {
	if c.ConsequenceID == "" {
		return models.Invalid("consequence_id is required")
	}
	if err := c.Validate(); err != nil {
		return err
	}

	result, err := r.consequences().UpdateOne(
		ctx,
		bson.M{"consequence_id": c.ConsequenceID, "campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"thread_id":        c.ThreadID,
			"cause":            c.Cause,
			"expected":         c.Expected,
			"severity":         c.Severity,
			"status":           c.Status,
			"outcome":          c.Outcome,
			"resolved_session": c.ResolvedSession,
			"resolved_at":      c.ResolvedAt,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to save consequence: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("consequence")
	}
	return nil
}

// DeleteConsequencesByCampaign removes every consequence in a campaign.
func (r *StoryRepository) DeleteConsequencesByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.consequences().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete consequences: %w", err)
	}
	return nil
}

// --- story arcs -------------------------------------------------------------

func (r *StoryRepository) arcs() *mongo.Collection {
	return r.client.Database().Collection(string(StoryArcs))
}

// CreateArc opens a story arc.
func (r *StoryRepository) CreateArc(ctx context.Context, arc *models.StoryArc) error {
	if arc.Status == "" {
		arc.Status = models.ArcUpcoming
	}
	if arc.Stage == "" {
		arc.Stage = models.ArcSetup
	}
	if err := arc.Validate(); err != nil {
		return err
	}

	if arc.ArcID == "" {
		arc.ArcID = primitive.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	arc.CreatedAt, arc.UpdatedAt = now, now

	if _, err := r.arcs().InsertOne(ctx, arc); err != nil {
		return fmt.Errorf("failed to create story arc: %w", err)
	}
	return nil
}

func (r *StoryRepository) findArcs(ctx context.Context, filter bson.M) ([]*models.StoryArc, error) {
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}, {Key: "created_at", Value: 1}})
	cursor, err := r.arcs().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find story arcs: %w", err)
	}
	defer cursor.Close(ctx)

	out := []*models.StoryArc{}
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("failed to decode story arcs: %w", err)
	}
	return out, nil
}

// GetArcsByCampaign lists every arc in campaign order.
func (r *StoryRepository) GetArcsByCampaign(ctx context.Context, campaignID string) ([]*models.StoryArc, error) {
	return r.findArcs(ctx, bson.M{"campaign_id": campaignID})
}

// GetActiveArc returns the arc currently running, or nil.
//
// The lowest-ordered active arc, so a campaign that has marked two active by
// mistake still gets a stable answer rather than whichever the driver returned
// first.
func (r *StoryRepository) GetActiveArc(ctx context.Context, campaignID string) (*models.StoryArc, error) {
	found, err := r.findArcs(ctx, bson.M{"campaign_id": campaignID, "status": models.ArcActive})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

// GetArcByArcID looks one arc up.
func (r *StoryRepository) GetArcByArcID(ctx context.Context, campaignID, arcID string) (*models.StoryArc, error) {
	found, err := r.findArcs(ctx, bson.M{"campaign_id": campaignID, "arc_id": arcID})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

// SaveArc writes an arc back after it has advanced or ended.
func (r *StoryRepository) SaveArc(ctx context.Context, campaignID string, arc *models.StoryArc) error {
	if arc.ArcID == "" {
		return models.Invalid("arc_id is required")
	}
	if err := arc.Validate(); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := r.arcs().UpdateOne(
		ctx,
		bson.M{"arc_id": arc.ArcID, "campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"title":             arc.Title,
			"premise":           arc.Premise,
			"order":             arc.Order,
			"status":            arc.Status,
			"stage":             arc.Stage,
			"thread_ids":        arc.ThreadIDs,
			"resolution":        arc.Resolution,
			"started_session":   arc.StartedSession,
			"completed_session": arc.CompletedSession,
			"completed_at":      arc.CompletedAt,
			"updated_at":        now,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to save story arc: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("story arc")
	}
	arc.UpdatedAt = now
	return nil
}

// DeleteArcsByCampaign removes every arc in a campaign.
func (r *StoryRepository) DeleteArcsByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.arcs().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete story arcs: %w", err)
	}
	return nil
}

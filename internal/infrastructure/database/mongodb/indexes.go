package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateIndexes creates all required indexes for performance
func (c *Client) CreateIndexes(ctx context.Context) error {
	// Campaigns indexes
	if err := c.createCampaignIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create campaign indexes: %w", err)
	}

	// Characters indexes
	if err := c.createCharacterIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create character indexes: %w", err)
	}

	// Monster indexes
	if err := c.createMonsterIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create monster indexes: %w", err)
	}

	// NPC indexes
	if err := c.createNPCIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create npc indexes: %w", err)
	}

	// Plot thread and consequence indexes
	if err := c.createStoryIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create story indexes: %w", err)
	}

	// Location indexes
	if err := c.createLocationIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create location indexes: %w", err)
	}

	// Sessions indexes
	if err := c.createSessionIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create session indexes: %w", err)
	}

	// Encounter indexes
	if err := c.createEncounterIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create encounter indexes: %w", err)
	}

	// Story events indexes
	if err := c.createStoryEventIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create story event indexes: %w", err)
	}

	return nil
}

func (c *Client) createCampaignIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(Campaigns))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "campaign_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "created_by", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "description", Value: "text"},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func (c *Client) createCharacterIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(Characters))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "character_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "campaign_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "type", Value: 1}},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func (c *Client) createMonsterIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(Monsters))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "monster_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "campaign_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "challenge_rating", Value: 1}},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func (c *Client) createNPCIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(NPCs))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "npc_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "campaign_id", Value: 1}},
		},
		{
			// Names are how a player refers to an NPC ("I talk to Toblen"), so
			// the lookup that matters most is by campaign and name.
			Keys: bson.D{{Key: "campaign_id", Value: 1}, {Key: "name", Value: 1}},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func (c *Client) createStoryIndexes(ctx context.Context) error {
	threads := c.Database().Collection(string(PlotThreads))
	if _, err := threads.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "thread_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			// The query that matters is "what is still open in this campaign",
			// which every prompt runs.
			Keys: bson.D{{Key: "campaign_id", Value: 1}, {Key: "status", Value: 1}},
		},
	}); err != nil {
		return err
	}

	consequences := c.Database().Collection(string(Consequences))
	if _, err := consequences.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "consequence_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "campaign_id", Value: 1}, {Key: "status", Value: 1}},
		},
	}); err != nil {
		return err
	}

	arcs := c.Database().Collection(string(StoryArcs))
	_, err := arcs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "arc_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			// "which arc is running" on every prompt, then campaign order.
			Keys: bson.D{{Key: "campaign_id", Value: 1}, {Key: "status", Value: 1}, {Key: "order", Value: 1}},
		},
	})
	return err
}

func (c *Client) createLocationIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(Locations))
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "location_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			// A session names where the party is by id or by name, so both are
			// lookups that run on every turn.
			Keys: bson.D{{Key: "campaign_id", Value: 1}, {Key: "name", Value: 1}},
		},
	})
	return err
}

func (c *Client) createSessionIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(Sessions))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "session_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "campaign_id", Value: 1},
				{Key: "session_number", Value: -1},
			},
		},
		{
			// Finding the one session in progress is on the hot path.
			Keys: bson.D{
				{Key: "campaign_id", Value: 1},
				{Key: "status", Value: 1},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func (c *Client) createEncounterIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(CombatEncounters))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "encounter_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "campaign_id", Value: 1},
				{Key: "session_id", Value: 1},
			},
		},
		{
			// Finding the fight under way is on the hot path.
			Keys: bson.D{
				{Key: "campaign_id", Value: 1},
				{Key: "combat_state.phase", Value: 1},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func (c *Client) createStoryEventIndexes(ctx context.Context) error {
	coll := c.Database().Collection(string(StoryEvents))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			// Unique so two writers cannot take the same position in a
			// session's log: AppendEvent relies on the collision to retry
			// rather than silently interleaving events.
			Keys: bson.D{
				{Key: "campaign_id", Value: 1},
				{Key: "session_id", Value: 1},
				{Key: "sequence_number", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			// Recent-events queries sort by time across a whole campaign.
			Keys: bson.D{
				{Key: "campaign_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
		{
			Keys: bson.D{{Key: "event_type", Value: 1}},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

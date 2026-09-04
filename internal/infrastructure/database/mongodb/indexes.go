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

	// Sessions indexes
	if err := c.createSessionIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create session indexes: %w", err)
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
			Keys: bson.D{
				{Key: "campaign_id", Value: 1},
				{Key: "session_id", Value: 1},
				{Key: "sequence_number", Value: 1},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

// CollectionName represents collection names
type CollectionName string

const (
	Campaigns        CollectionName = "campaigns"
	Characters       CollectionName = "characters"
	Monsters         CollectionName = "monsters"
	Sessions         CollectionName = "sessions"
	StoryEvents      CollectionName = "story_events"
	CombatEncounters CollectionName = "combat_encounters"
	GameLogs         CollectionName = "game_logs"
)

// InitializeCollections creates all required collections
func (c *Client) InitializeCollections(ctx context.Context) error {
	collections := []CollectionName{
		Campaigns,
		Characters,
		Monsters,
		Sessions,
		StoryEvents,
		CombatEncounters,
		GameLogs,
	}

	for _, name := range collections {
		if err := c.createCollection(ctx, name); err != nil {
			return fmt.Errorf("failed to create collection %s: %w", name, err)
		}
	}

	return nil
}

func (c *Client) createCollection(ctx context.Context, name CollectionName) error {
	// Check if collection exists
	collections, err := c.database.ListCollectionNames(ctx, bson.M{"name": string(name)})
	if err != nil {
		return err
	}

	if len(collections) > 0 {
		return nil // Collection already exists
	}

	// Create collection
	err = c.database.CreateCollection(ctx, string(name))
	return err
}

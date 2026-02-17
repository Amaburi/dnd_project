package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CharacterRepository handles character database operations
type CharacterRepository struct {
	client *Client // MongoDB client
}

// NewCharacterRepository creates a new character repository
func NewCharacterRepository(client *Client) *CharacterRepository {
	return &CharacterRepository{
		client: client,
	}
}

// CreateCharacter creates a new character
func (r *CharacterRepository) CreateCharacter(ctx context.Context, character *models.Character) error {
	// Validate required fields
	if character.Name == "" {
		return fmt.Errorf("character name is required")
	}
	if character.CampaignID == "" {
		return fmt.Errorf("campaign_id is required")
	}
	if character.Type == "" {
		return fmt.Errorf("character type is required")
	}

	// Generate CharacterID if not set
	if character.CharacterID == "" {
		character.CharacterID = primitive.NewObjectID().Hex()
	}

	// Set timestamps
	now := primitive.NewDateTimeFromTime(time.Now())
	character.CreatedAt = now
	character.UpdatedAt = now

	// Insert to MongoDB
	collection := r.client.Database().Collection(string(Characters))
	_, err := collection.InsertOne(ctx, character)
	if err != nil {
		return fmt.Errorf("failed to insert character: %w", err)
	}
	return nil
}

// GetCharacterByID retrieves a character by ID
func (r *CharacterRepository) GetCharacterByID(ctx context.Context, id primitive.ObjectID) (*models.Character, error) {
	var character models.Character
	collection := r.client.Database().Collection(string(Characters))
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&character)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to find character: %w", err)
	}
	return &character, nil
}

// GetCharacterByCharacterID retrieves a character by character_id field
func (r *CharacterRepository) GetCharacterByCharacterID(ctx context.Context, characterID string) (*models.Character, error) {
	var character models.Character
	collection := r.client.Database().Collection(string(Characters))
	err := collection.FindOne(ctx, bson.M{"character_id": characterID}).Decode(&character)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find character: %w", err)
	}
	return &character, nil
}

// GetCharactersByCampaign gets all characters in a campaign
func (r *CharacterRepository) GetCharactersByCampaign(ctx context.Context, campaignID string) ([]*models.Character, error) {
	collection := r.client.Database().Collection(string(Characters))
	cursor, err := collection.Find(ctx, bson.M{"campaign_id": campaignID})
	if err != nil {
		return nil, fmt.Errorf("failed to find characters: %w", err)
	}
	defer cursor.Close(ctx)

	var characters []*models.Character
	if err = cursor.All(ctx, &characters); err != nil {
		return nil, fmt.Errorf("failed to decode characters: %w", err)
	}
	return characters, nil
}

// GetCharactersByType gets characters by type (player/npc/enemy/monster)
func (r *CharacterRepository) GetCharactersByType(ctx context.Context, campaignID string, charType string) ([]*models.Character, error) {
	collection := r.client.Database().Collection(string(Characters))
	cursor, err := collection.Find(ctx, bson.M{
		"campaign_id": campaignID,
		"type":        charType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find characters: %w", err)
	}
	defer cursor.Close(ctx)

	var characters []*models.Character
	if err = cursor.All(ctx, &characters); err != nil {
		return nil, fmt.Errorf("failed to decode characters: %w", err)
	}
	return characters, nil
}

// GetPlayerCharacters gets all player characters in a campaign
func (r *CharacterRepository) GetPlayerCharacters(ctx context.Context, campaignID string) ([]*models.Character, error) {
	return r.GetCharactersByType(ctx, campaignID, "player")
}

// GetNPCs gets all NPCs in a campaign
func (r *CharacterRepository) GetNPCs(ctx context.Context, campaignID string) ([]*models.Character, error) {
	return r.GetCharactersByType(ctx, campaignID, "npc")
}

// GetEnemies gets all enemies in a campaign
func (r *CharacterRepository) GetEnemies(ctx context.Context, campaignID string) ([]*models.Character, error) {
	collection := r.client.Database().Collection(string(Characters))
	cursor, err := collection.Find(ctx, bson.M{
		"campaign_id": campaignID,
		"type":        bson.M{"$in": []string{"enemy", "monster"}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find enemies: %w", err)
	}
	defer cursor.Close(ctx)

	var characters []*models.Character
	if err = cursor.All(ctx, &characters); err != nil {
		return nil, fmt.Errorf("failed to decode characters: %w", err)
	}
	return characters, nil
}

// UpdateCharacter updates a character
func (r *CharacterRepository) UpdateCharacter(ctx context.Context, character *models.Character) error {
	// Validate ID
	if character.ID.IsZero() {
		return fmt.Errorf("character ID is required")
	}

	// Update timestamp
	character.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": character.ID},
		bson.M{"$set": character},
	)
	if err != nil {
		return fmt.Errorf("failed to update character: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("character not found")
	}

	return nil
}

// UpdateCharacterHP updates a character's hit points
func (r *CharacterRepository) UpdateCharacterHP(ctx context.Context, characterID primitive.ObjectID, currentHP, maxHP, tempHP int) error {
	collection := r.client.Database().Collection(string(Characters))
	_, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": characterID},
		bson.M{
			"$set": bson.M{
				"derived_stats.hit_points.current":   currentHP,
				"derived_stats.hit_points.maximum":   maxHP,
				"derived_stats.hit_points.temporary": tempHP,
				"updated_at":                         primitive.NewDateTimeFromTime(time.Now()),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update character HP: %w", err)
	}
	return nil
}

// AddStatusEffect adds a status effect to a character
func (r *CharacterRepository) AddStatusEffect(ctx context.Context, characterID primitive.ObjectID, effect string) error {
	collection := r.client.Database().Collection(string(Characters))
	_, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": characterID},
		bson.M{
			"$addToSet": bson.M{"status_effects": effect},
			"$set":      bson.M{"updated_at": primitive.NewDateTimeFromTime(time.Now())},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to add status effect: %w", err)
	}
	return nil
}

// RemoveStatusEffect removes a status effect from a character
func (r *CharacterRepository) RemoveStatusEffect(ctx context.Context, characterID primitive.ObjectID, effect string) error {
	collection := r.client.Database().Collection(string(Characters))
	_, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": characterID},
		bson.M{
			"$pull": bson.M{"status_effects": effect},
			"$set":  bson.M{"updated_at": primitive.NewDateTimeFromTime(time.Now())},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to remove status effect: %w", err)
	}
	return nil
}

// DeleteCharacter deletes a character
func (r *CharacterRepository) DeleteCharacter(ctx context.Context, id primitive.ObjectID) error {
	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete character: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("character not found")
	}

	return nil
}

// DeleteCharactersByCampaign deletes all characters in a campaign
func (r *CharacterRepository) DeleteCharactersByCampaign(ctx context.Context, campaignID string) error {
	collection := r.client.Database().Collection(string(Characters))
	_, err := collection.DeleteMany(ctx, bson.M{"campaign_id": campaignID})
	if err != nil {
		return fmt.Errorf("failed to delete characters: %w", err)
	}
	return nil
}

// CountCharactersByCampaign counts characters in a campaign
func (r *CharacterRepository) CountCharactersByCampaign(ctx context.Context, campaignID string) (int64, error) {
	collection := r.client.Database().Collection(string(Characters))
	count, err := collection.CountDocuments(ctx, bson.M{"campaign_id": campaignID})
	if err != nil {
		return 0, fmt.Errorf("failed to count characters: %w", err)
	}
	return count, nil
}

// SearchCharacters searches characters by name
func (r *CharacterRepository) SearchCharacters(ctx context.Context, campaignID string, query string) ([]*models.Character, error) {
	collection := r.client.Database().Collection(string(Characters))
	cursor, err := collection.Find(ctx, bson.M{
		"campaign_id": campaignID,
		"name":        bson.M{"$regex": query, "$options": "i"}, // Case-insensitive search
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search characters: %w", err)
	}
	defer cursor.Close(ctx)

	var characters []*models.Character
	if err = cursor.All(ctx, &characters); err != nil {
		return nil, fmt.Errorf("failed to decode characters: %w", err)
	}
	return characters, nil
}

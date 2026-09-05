package mongodb

import (
	"context"
	"fmt"
	"regexp"
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

// emptyIfNil keeps optional slices out of BSON as [] rather than null, so a
// document never alternates between missing and empty across updates.
func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// CreateCharacter creates a new character
func (r *CharacterRepository) CreateCharacter(ctx context.Context, character *models.Character) error {
	// Validate required fields
	if character.Name == "" {
		return models.Invalid("character name is required")
	}
	if character.CampaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if character.Type == "" {
		return models.Invalid("character type is required")
	}

	// A new character must describe a legal 5e sheet. Updates deliberately do
	// not run this: sheets written before a rules change should stay editable
	// rather than becoming unsaveable.
	if err := character.ValidateSheet(); err != nil {
		return err
	}

	// The _id is assigned by MongoDB; a client-supplied one is ignored.
	character.ID = primitive.NilObjectID

	// Generate CharacterID if not set
	if character.CharacterID == "" {
		character.CharacterID = primitive.NewObjectID().Hex()
	}

	// Set timestamps
	now := time.Now().UTC()
	character.CreatedAt = now
	character.UpdatedAt = now

	// Insert to MongoDB
	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.InsertOne(ctx, character)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Invalid("character_id %q already exists", character.CharacterID)
		}
		return fmt.Errorf("failed to insert character: %w", err)
	}

	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		character.ID = id
	}
	return nil
}

// GetCharacterInCampaign retrieves a character scoped to its campaign.
//
// Scoping on campaign_id as well as _id is what keeps a character from being
// readable through an unrelated campaign's URL.
func (r *CharacterRepository) GetCharacterInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) (*models.Character, error) {
	var character models.Character
	collection := r.client.Database().Collection(string(Characters))
	err := collection.FindOne(ctx, bson.M{"_id": id, "campaign_id": campaignID}).Decode(&character)
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

// findCharacters runs a filter and decodes the result set.
func (r *CharacterRepository) findCharacters(ctx context.Context, filter bson.M) ([]*models.Character, error) {
	collection := r.client.Database().Collection(string(Characters))
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find characters: %w", err)
	}
	defer cursor.Close(ctx)

	var characters []*models.Character
	if err = cursor.All(ctx, &characters); err != nil {
		return nil, fmt.Errorf("failed to decode characters: %w", err)
	}
	if characters == nil {
		// Encode an empty result as [] rather than null.
		characters = []*models.Character{}
	}
	return characters, nil
}

// GetCharactersByCampaign gets all characters in a campaign
func (r *CharacterRepository) GetCharactersByCampaign(ctx context.Context, campaignID string) ([]*models.Character, error) {
	return r.findCharacters(ctx, bson.M{"campaign_id": campaignID})
}

// GetCharactersByType gets characters by type (player or npc)
func (r *CharacterRepository) GetCharactersByType(ctx context.Context, campaignID string, charType models.CharacterType) ([]*models.Character, error) {
	return r.findCharacters(ctx, bson.M{
		"campaign_id": campaignID,
		"type":        charType,
	})
}

// GetPlayerCharacters gets all player characters in a campaign
func (r *CharacterRepository) GetPlayerCharacters(ctx context.Context, campaignID string) ([]*models.Character, error) {
	return r.GetCharactersByType(ctx, campaignID, models.CharacterPlayer)
}

// GetNPCs gets all NPCs in a campaign
func (r *CharacterRepository) GetNPCs(ctx context.Context, campaignID string) ([]*models.Character, error) {
	return r.GetCharactersByType(ctx, campaignID, models.CharacterNPC)
}

// UpdateCharacter updates the mutable fields of a character within a campaign.
//
// As with campaigns, the update document is built explicitly: $set-ing the
// decoded struct would blank the uniquely indexed character_id, detach the
// character from its campaign and zero created_at.
func (r *CharacterRepository) UpdateCharacter(ctx context.Context, campaignID string, character *models.Character) error {
	if character.ID.IsZero() {
		return models.Invalid("character ID is required")
	}
	if campaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if character.Name == "" {
		return models.Invalid("character name is required")
	}
	if character.Type == "" {
		return models.Invalid("character type is required")
	}

	now := time.Now().UTC()
	update := bson.M{
		"type":                   character.Type,
		"name":                   character.Name,
		"player_name":            character.PlayerName,
		"basic_info":             character.BasicInfo,
		"ability_scores":         character.AbilityScores,
		"combat_stats":           character.CombatStats,
		"skills":                 character.Skills,
		"saving_throws":          character.SavingThrows,
		"proficiencies":          character.Proficiencies,
		"inventory":              emptyIfNil(character.Inventory),
		"equipment":              character.Equipment,
		"spells":                 character.Spells,
		"features_and_abilities": emptyIfNil(character.FeaturesAndAbilities),
		"background_story":       character.BackgroundStory,
		"conditions":             emptyIfNil(character.Conditions),
		"exhaustion":             character.Exhaustion,
		"currency":               character.Currency,
		"inspiration":            character.Inspiration,
		"relationships":          emptyIfNil(character.Relationships),
		"ai_metadata":            character.AIMetadata,
		"updated_at":             now,
	}

	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": character.ID, "campaign_id": campaignID},
		bson.M{"$set": update},
	)
	if err != nil {
		return fmt.Errorf("failed to update character: %w", err)
	}

	if result.MatchedCount == 0 {
		return models.NotFound("character")
	}

	character.CampaignID = campaignID
	character.UpdatedAt = now
	return nil
}

// UpdateCharacterHP updates a character's hit points
func (r *CharacterRepository) UpdateCharacterHP(ctx context.Context, characterID primitive.ObjectID, currentHP, maxHP, tempHP int) error {
	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": characterID},
		bson.M{
			"$set": bson.M{
				"combat_stats.hit_points.current":   currentHP,
				"combat_stats.hit_points.maximum":   maxHP,
				"combat_stats.hit_points.temporary": tempHP,
				"updated_at":                        time.Now().UTC(),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to update character HP: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("character")
	}
	return nil
}

// AddCondition applies a 5e condition to a character.
//
// Unknown strings are rejected rather than stored: conditions are a closed set
// of fifteen, and the free-form status_effects list this replaced meant no
// caller could rely on what a condition value would be.
func (r *CharacterRepository) AddCondition(ctx context.Context, characterID primitive.ObjectID, condition models.Condition) error {
	if !condition.Valid() {
		return models.Invalid("unknown condition %q", condition)
	}

	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": characterID},
		bson.M{
			"$addToSet": bson.M{"conditions": condition},
			"$set":      bson.M{"updated_at": time.Now().UTC()},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to add condition: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("character")
	}
	return nil
}

// RemoveCondition clears a condition from a character.
func (r *CharacterRepository) RemoveCondition(ctx context.Context, characterID primitive.ObjectID, condition models.Condition) error {
	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": characterID},
		bson.M{
			"$pull": bson.M{"conditions": condition},
			"$set":  bson.M{"updated_at": time.Now().UTC()},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to remove condition: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("character")
	}
	return nil
}

// SetExhaustion sets a character's exhaustion level.
//
// Exhaustion is the one condition with degrees -- six levels of escalating
// penalty, reduced by one per long rest -- so it is a number, not membership
// in the condition list.
func (r *CharacterRepository) SetExhaustion(ctx context.Context, characterID primitive.ObjectID, level int) error {
	if level < 0 || level > models.MaxExhaustion {
		return models.Invalid("exhaustion must be between 0 and %d, got %d", models.MaxExhaustion, level)
	}

	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": characterID},
		bson.M{"$set": bson.M{"exhaustion": level, "updated_at": time.Now().UTC()}},
	)
	if err != nil {
		return fmt.Errorf("failed to set exhaustion: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("character")
	}
	return nil
}

// DeleteCharacterInCampaign deletes a character scoped to its campaign.
func (r *CharacterRepository) DeleteCharacterInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) error {
	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.DeleteOne(ctx, bson.M{"_id": id, "campaign_id": campaignID})
	if err != nil {
		return fmt.Errorf("failed to delete character: %w", err)
	}

	if result.DeletedCount == 0 {
		return models.NotFound("character")
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

// SearchCharacters searches characters by name.
//
// The query is quoted before it reaches $regex: an unescaped caller string is
// both a regex-injection and a ReDoS vector.
func (r *CharacterRepository) SearchCharacters(ctx context.Context, campaignID string, query string) ([]*models.Character, error) {
	if query == "" {
		return r.GetCharactersByCampaign(ctx, campaignID)
	}

	return r.findCharacters(ctx, bson.M{
		"campaign_id": campaignID,
		"name":        bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"}, // Case-insensitive search
	})
}

// UpdateSpellSlots writes back the caster's spell resources.
//
// It is its own method rather than part of UpdateCharacter for the reason the
// rest of this file sets fields individually: a turn holds a character it read
// at the start of the turn, and $set-ing that whole struct would write every
// other field back as it was then, silently reverting anything else that had
// changed. Only what the cast actually spent is written.
//
// Cantrips never reach here, because they cost nothing to write.
func (r *CharacterRepository) UpdateSpellSlots(ctx context.Context, characterID string, spells models.Spells) error {
	if characterID == "" {
		return models.Invalid("character_id is required")
	}

	collection := r.client.Database().Collection(string(Characters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"character_id": characterID},
		bson.M{"$set": bson.M{
			"spells.slots":      spells.Slots,
			"spells.pact_slots": spells.PactSlots,
			"updated_at":        time.Now().UTC(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update spell slots: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("character")
	}
	return nil
}

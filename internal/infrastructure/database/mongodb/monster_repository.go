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

// MonsterRepository handles monster statblock database operations.
type MonsterRepository struct {
	client *Client
}

// NewMonsterRepository creates a new monster repository.
func NewMonsterRepository(client *Client) *MonsterRepository {
	return &MonsterRepository{client: client}
}

// CreateMonster stores a new statblock.
func (r *MonsterRepository) CreateMonster(ctx context.Context, monster *models.Monster) error {
	if monster.CampaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	// A statblock must be internally consistent before it is stored. Updates
	// skip this so entries predating a rules change stay editable.
	if err := monster.Validate(); err != nil {
		return err
	}

	// The _id is assigned by MongoDB; a client-supplied one is ignored.
	monster.ID = primitive.NilObjectID

	if monster.MonsterID == "" {
		monster.MonsterID = primitive.NewObjectID().Hex()
	}

	now := time.Now().UTC()
	monster.CreatedAt = now
	monster.UpdatedAt = now

	collection := r.client.Database().Collection(string(Monsters))
	result, err := collection.InsertOne(ctx, monster)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Invalid("monster_id %q already exists", monster.MonsterID)
		}
		return fmt.Errorf("failed to insert monster: %w", err)
	}

	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		monster.ID = id
	}
	return nil
}

// GetMonsterInCampaign retrieves a statblock scoped to its campaign.
func (r *MonsterRepository) GetMonsterInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) (*models.Monster, error) {
	var monster models.Monster
	collection := r.client.Database().Collection(string(Monsters))
	err := collection.FindOne(ctx, bson.M{"_id": id, "campaign_id": campaignID}).Decode(&monster)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to find monster: %w", err)
	}
	return &monster, nil
}

// findMonsters runs a filter and decodes the result set.
func (r *MonsterRepository) findMonsters(ctx context.Context, filter bson.M) ([]*models.Monster, error) {
	collection := r.client.Database().Collection(string(Monsters))
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find monsters: %w", err)
	}
	defer cursor.Close(ctx)

	var monsters []*models.Monster
	if err = cursor.All(ctx, &monsters); err != nil {
		return nil, fmt.Errorf("failed to decode monsters: %w", err)
	}
	if monsters == nil {
		// Encode an empty result as [] rather than null.
		monsters = []*models.Monster{}
	}
	return monsters, nil
}

// GetMonstersByCampaign gets every statblock in a campaign.
func (r *MonsterRepository) GetMonstersByCampaign(ctx context.Context, campaignID string) ([]*models.Monster, error) {
	return r.findMonsters(ctx, bson.M{"campaign_id": campaignID})
}

// GetMonstersByChallengeRating gets statblocks within a challenge rating band,
// which is how an encounter is built to a budget.
func (r *MonsterRepository) GetMonstersByChallengeRating(ctx context.Context, campaignID string, min, max float64) ([]*models.Monster, error) {
	return r.findMonsters(ctx, bson.M{
		"campaign_id":      campaignID,
		"challenge_rating": bson.M{"$gte": min, "$lte": max},
	})
}

// SearchMonsters searches statblocks by name within a campaign.
//
// The query is quoted before it reaches $regex: an unescaped caller string is
// both a regex-injection and a ReDoS vector.
func (r *MonsterRepository) SearchMonsters(ctx context.Context, campaignID, query string) ([]*models.Monster, error) {
	if query == "" {
		return r.GetMonstersByCampaign(ctx, campaignID)
	}
	return r.findMonsters(ctx, bson.M{
		"campaign_id": campaignID,
		"name":        bson.M{"$regex": regexp.QuoteMeta(query), "$options": "i"},
	})
}

// UpdateMonster updates the mutable fields of a statblock within a campaign.
//
// As elsewhere, the update document is built explicitly: $set-ing the decoded
// struct would blank the uniquely indexed monster_id, detach the statblock
// from its campaign and zero created_at.
func (r *MonsterRepository) UpdateMonster(ctx context.Context, campaignID string, monster *models.Monster) error {
	if monster.ID.IsZero() {
		return models.Invalid("monster ID is required")
	}
	if campaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if monster.Name == "" {
		return models.Invalid("monster name is required")
	}

	now := time.Now().UTC()
	update := bson.M{
		"name":                         monster.Name,
		"size":                         monster.Size,
		"type":                         monster.Type,
		"subtype":                      monster.Subtype,
		"alignment":                    monster.Alignment,
		"armor_class":                  monster.ArmorClass,
		"armor_note":                   monster.ArmorNote,
		"hit_points":                   monster.HitPoints,
		"hit_dice":                     monster.HitDice,
		"speeds":                       monster.Speeds,
		"ability_scores":               monster.AbilityScores,
		"saving_throws":                monster.SavingThrows,
		"skills":                       monster.Skills,
		"damage_affinities":            monster.Affinities,
		"condition_immunities":         emptyIfNil(monster.ConditionImmunities),
		"senses":                       monster.Senses,
		"languages":                    emptyIfNil(monster.Languages),
		"challenge_rating":             monster.ChallengeRating,
		"traits":                       emptyIfNil(monster.Traits),
		"actions":                      emptyIfNil(monster.Actions),
		"bonus_actions":                emptyIfNil(monster.BonusActions),
		"reactions":                    emptyIfNil(monster.Reactions),
		"legendary_resistance_per_day": monster.LegendaryResistancePerDay,
		"legendary_actions_per_round":  monster.LegendaryActionsPerRound,
		"legendary_actions":            emptyIfNil(monster.LegendaryActions),
		"description":                  monster.Description,
		"source":                       monster.Source,
		"updated_at":                   now,
	}

	collection := r.client.Database().Collection(string(Monsters))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": monster.ID, "campaign_id": campaignID},
		bson.M{"$set": update},
	)
	if err != nil {
		return fmt.Errorf("failed to update monster: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("monster")
	}

	monster.CampaignID = campaignID
	monster.UpdatedAt = now
	return nil
}

// DeleteMonsterInCampaign deletes a statblock scoped to its campaign.
func (r *MonsterRepository) DeleteMonsterInCampaign(ctx context.Context, campaignID string, id primitive.ObjectID) error {
	collection := r.client.Database().Collection(string(Monsters))
	result, err := collection.DeleteOne(ctx, bson.M{"_id": id, "campaign_id": campaignID})
	if err != nil {
		return fmt.Errorf("failed to delete monster: %w", err)
	}
	if result.DeletedCount == 0 {
		return models.NotFound("monster")
	}
	return nil
}

// DeleteMonstersByCampaign deletes every statblock in a campaign, so deleting
// a campaign does not strand them.
func (r *MonsterRepository) DeleteMonstersByCampaign(ctx context.Context, campaignID string) error {
	collection := r.client.Database().Collection(string(Monsters))
	if _, err := collection.DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete monsters: %w", err)
	}
	return nil
}

// SeedSRDMonsters copies the SRD catalogue into a campaign, skipping any
// statblock already present.
//
// Campaigns own their monsters, so this stamps copies rather than sharing a
// global catalogue: editing a goblin in one campaign must not change another's.
func (r *MonsterRepository) SeedSRDMonsters(ctx context.Context, campaignID string) (int, error) {
	if campaignID == "" {
		return 0, models.Invalid("campaign_id is required")
	}

	existing, err := r.GetMonstersByCampaign(ctx, campaignID)
	if err != nil {
		return 0, err
	}
	present := make(map[string]bool, len(existing))
	for _, m := range existing {
		present[m.Name] = true
	}

	seeded := 0
	for _, monster := range models.SRDMonsters() {
		if present[monster.Name] {
			continue
		}
		monster.CampaignID = campaignID
		// The catalogue's ids are shared across campaigns, so let the
		// repository mint a unique one per copy.
		monster.MonsterID = ""
		if err := r.CreateMonster(ctx, &monster); err != nil {
			return seeded, fmt.Errorf("failed to seed %s: %w", monster.Name, err)
		}
		seeded++
	}
	return seeded, nil
}

// UpdateHitPoints writes a monster's hit points back after combat.
//
// Damage that is not persisted is damage that did not happen: without this a
// creature heals to full between turns and the event log and the game state
// stop agreeing.
func (r *MonsterRepository) UpdateHitPoints(ctx context.Context, campaignID, monsterID string, hp models.HitPoints) error {
	collection := r.client.Database().Collection(string(Monsters))
	result, err := collection.UpdateOne(ctx,
		bson.M{"campaign_id": campaignID, "monster_id": monsterID},
		bson.M{"$set": bson.M{"hit_points": hp, "updated_at": time.Now().UTC()}},
	)
	if err != nil {
		return fmt.Errorf("failed to update monster hit points: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("monster")
	}
	return nil
}

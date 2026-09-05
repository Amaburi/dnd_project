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

type CampaignRepository struct {
	client *Client
}

func NewCampaignRepository(client *Client) *CampaignRepository {
	return &CampaignRepository{client: client}
}

func (r *CampaignRepository) CreateCampaign(ctx context.Context, campaign *models.Campaign) error {
	// Validate required fields
	if campaign.Title == "" {
		return models.Invalid("campaign title is required")
	}
	if campaign.CreatedBy == "" {
		return models.Invalid("created_by is required")
	}

	// The _id is assigned by MongoDB; a client-supplied one is ignored.
	campaign.ID = primitive.NilObjectID

	// Generate CampaignID if not set
	if campaign.CampaignID == "" {
		campaign.CampaignID = primitive.NewObjectID().Hex()
	}

	// Set timestamps
	now := time.Now().UTC()
	campaign.CreatedAt = now
	campaign.UpdatedAt = now

	// Insert to MongoDB
	collection := r.client.Database().Collection(string(Campaigns))
	result, err := collection.InsertOne(ctx, campaign)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.Invalid("campaign_id %q already exists", campaign.CampaignID)
		}
		return fmt.Errorf("failed to insert campaign: %w", err)
	}

	if id, ok := result.InsertedID.(primitive.ObjectID); ok {
		campaign.ID = id
	}
	return nil
}

func (r *CampaignRepository) GetCampaignByID(ctx context.Context, id primitive.ObjectID) (*models.Campaign, error) {
	var campaign models.Campaign
	collection := r.client.Database().Collection(string(Campaigns))
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&campaign)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to find campaign: %w", err)
	}
	return &campaign, nil
}

// GetCampaignByCampaignID retrieves a campaign by campaign_id field
func (r *CampaignRepository) GetCampaignByCampaignID(ctx context.Context, campaignID string) (*models.Campaign, error) {
	var campaign models.Campaign
	collection := r.client.Database().Collection(string(Campaigns))
	err := collection.FindOne(ctx, bson.M{"campaign_id": campaignID}).Decode(&campaign)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find campaign: %w", err)
	}
	return &campaign, nil
}

// GetCampaignsByUser retrieves all campaigns created by a user
func (r *CampaignRepository) GetCampaignsByUser(ctx context.Context, userID string) ([]*models.Campaign, error) {
	collection := r.client.Database().Collection(string(Campaigns))
	cursor, err := collection.Find(ctx, bson.M{"created_by": userID})
	if err != nil {
		return nil, fmt.Errorf("failed to find campaigns: %w", err)
	}
	defer cursor.Close(ctx)

	var campaigns []*models.Campaign
	if err = cursor.All(ctx, &campaigns); err != nil {
		return nil, fmt.Errorf("failed to decode campaigns: %w", err)
	}
	if campaigns == nil {
		// Encode an empty result as [] rather than null.
		campaigns = []*models.Campaign{}
	}
	return campaigns, nil
}

// DeleteCampaign deletes a campaign by ID
func (r *CampaignRepository) DeleteCampaign(ctx context.Context, id primitive.ObjectID) error {
	collection := r.client.Database().Collection(string(Campaigns))
	result, err := collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete campaign: %w", err)
	}

	if result.DeletedCount == 0 {
		return models.NotFound("campaign")
	}

	return nil
}

// UpdateCampaign updates the mutable fields of an existing campaign.
//
// The update document is built field by field on purpose. Passing the decoded
// struct to $set would also write every field the caller omitted -- blanking
// the uniquely indexed campaign_id and zeroing created_at.
func (r *CampaignRepository) UpdateCampaign(ctx context.Context, campaign *models.Campaign) error {
	if campaign.ID.IsZero() {
		return models.Invalid("campaign ID is required")
	}
	if campaign.Title == "" {
		return models.Invalid("campaign title is required")
	}

	now := time.Now().UTC()
	update := bson.M{
		"title":              campaign.Title,
		"description":        campaign.Description,
		"setting":            campaign.Setting,
		"dm_settings":        campaign.DMSettings,
		"ai_personality":     campaign.AIPersonality,
		"status":             campaign.Status,
		"current_session_id": campaign.CurrentSessionID,
		"updated_at":         now,
	}

	collection := r.client.Database().Collection(string(Campaigns))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": campaign.ID},
		bson.M{"$set": update},
	)
	if err != nil {
		return fmt.Errorf("failed to update campaign: %w", err)
	}

	if result.MatchedCount == 0 {
		return models.NotFound("campaign")
	}

	campaign.UpdatedAt = now
	return nil
}

// UpdateSummary stores the campaign's rolling summary.
//
// It is its own method rather than part of UpdateCampaign because the summary
// is written by the compaction pass, not by a user editing a campaign: folding
// it into the PUT path would let a client with a stale body erase the campaign's
// long memory. The subfields are set individually for the same reason the rest
// of this file does it -- $set-ing a struct writes every omitted field as its
// zero value.
func (r *CampaignRepository) UpdateSummary(ctx context.Context, campaignID string, summary models.CampaignSummary) error {
	if campaignID == "" {
		return models.Invalid("campaign ID is required")
	}
	if summary.Text == "" {
		return models.Invalid("summary text is required")
	}

	now := time.Now().UTC()
	if summary.UpdatedAt.IsZero() {
		summary.UpdatedAt = now
	}

	collection := r.client.Database().Collection(string(Campaigns))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"summary.text":        summary.Text,
			"summary.through":     summary.Through,
			"summary.event_count": summary.EventCount,
			"summary.updated_at":  summary.UpdatedAt,
			"updated_at":          now,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update campaign summary: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("campaign")
	}
	return nil
}

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
		return fmt.Errorf("campaign title is required")
	}
	if campaign.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}

	// Generate CampaignID if not set
	if campaign.CampaignID == "" {
		campaign.CampaignID = primitive.NewObjectID().Hex()
	}

	// Set timestamps
	now := time.Now()
	campaign.CreatedAt = now
	campaign.UpdatedAt = now

	// Insert to MongoDB
	collection := r.client.Database().Collection(string(Campaigns))
	_, err := collection.InsertOne(ctx, campaign)
	if err != nil {
		return fmt.Errorf("failed to insert campaign: %w", err)
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
		return fmt.Errorf("campaign not found")
	}

	return nil
}

// UpdateCampaign updates an existing campaign
func (r *CampaignRepository) UpdateCampaign(ctx context.Context, campaign *models.Campaign) error {
	// Validate required fields
	if campaign.Title == "" {
		return fmt.Errorf("campaign title is required")
	}
	if campaign.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}
	if campaign.ID.IsZero() {
		return fmt.Errorf("campaign ID is required")
	}

	// Update timestamp
	campaign.UpdatedAt = time.Now()

	collection := r.client.Database().Collection(string(Campaigns))
	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": campaign.ID},
		bson.M{"$set": campaign},
	)
	if err != nil {
		return fmt.Errorf("failed to update campaign: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("campaign not found")
	}

	return nil
}

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

// LocationRepository stores places and the things in them.
type LocationRepository struct {
	client *Client

	// sessions resolves where the party currently is. A location is a place in
	// the world; which one the party is standing in is session state.
	sessions *SessionRepository
}

// NewLocationRepository creates a new location repository.
func NewLocationRepository(client *Client, sessions *SessionRepository) *LocationRepository {
	return &LocationRepository{client: client, sessions: sessions}
}

func (r *LocationRepository) collection() *mongo.Collection {
	return r.client.Database().Collection(string(Locations))
}

// CreateLocation stores a new place.
func (r *LocationRepository) CreateLocation(ctx context.Context, location *models.Location) error {
	if location.Lighting == "" {
		location.Lighting = models.LightingBright
	}
	if err := location.Validate(); err != nil {
		return err
	}

	if location.LocationID == "" {
		location.LocationID = primitive.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	location.CreatedAt, location.UpdatedAt = now, now

	if _, err := r.collection().InsertOne(ctx, location); err != nil {
		return fmt.Errorf("failed to create location: %w", err)
	}
	return nil
}

func (r *LocationRepository) find(ctx context.Context, filter bson.M) ([]*models.Location, error) {
	opts := options.Find().SetSort(bson.D{{Key: "name", Value: 1}})
	cursor, err := r.collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find locations: %w", err)
	}
	defer cursor.Close(ctx)

	out := []*models.Location{}
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("failed to decode locations: %w", err)
	}
	return out, nil
}

// GetLocationsByCampaign lists every place in a campaign.
func (r *LocationRepository) GetLocationsByCampaign(ctx context.Context, campaignID string) ([]*models.Location, error) {
	return r.find(ctx, bson.M{"campaign_id": campaignID})
}

// GetLocationByLocationID looks one place up.
func (r *LocationRepository) GetLocationByLocationID(ctx context.Context, campaignID, locationID string) (*models.Location, error) {
	found, err := r.find(ctx, bson.M{"campaign_id": campaignID, "location_id": locationID})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

// GetCurrentLocation is where the party is standing.
//
// The active session names it. Nothing named, or nothing matching, is not an
// error: a campaign with no mapped rooms simply has no scenery, which is how
// every campaign ran before locations existed.
func (r *LocationRepository) GetCurrentLocation(ctx context.Context, campaignID string) (*models.Location, error) {
	session, err := r.sessions.GetActiveSession(ctx, campaignID)
	if err != nil || session == nil {
		return nil, err
	}

	named := session.Location.CurrentLocation
	if named == "" {
		return nil, nil
	}

	// The session may name a location by its id or by its name, because a DM
	// setting the scene by hand will type the name.
	if found, err := r.GetLocationByLocationID(ctx, campaignID, named); err != nil || found != nil {
		return found, err
	}
	byName, err := r.find(ctx, bson.M{"campaign_id": campaignID, "name": named})
	if err != nil || len(byName) == 0 {
		return nil, err
	}
	return byName[0], nil
}

// SaveLocation writes a place back after the party has changed it.
//
// Interactables carry state that is earned in play -- a chest that has been
// picked, a flagstone that has been found -- so this is the path a turn uses.
func (r *LocationRepository) SaveLocation(ctx context.Context, campaignID string, location *models.Location) error {
	if location.LocationID == "" {
		return models.Invalid("location_id is required")
	}
	if err := location.Validate(); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := r.collection().UpdateOne(
		ctx,
		bson.M{"location_id": location.LocationID, "campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"name":          location.Name,
			"description":   location.Description,
			"ambience":      location.Ambience,
			"lighting":      location.Lighting,
			"interactables": location.Interactables,
			"exits":         location.Exits,
			"npc_ids":       location.NPCIDs,
			"visited":       location.Visited,
			"updated_at":    now,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to save location: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("location")
	}
	location.UpdatedAt = now
	return nil
}

// DeleteLocationsByCampaign removes every place in a campaign.
func (r *LocationRepository) DeleteLocationsByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.collection().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete locations: %w", err)
	}
	return nil
}

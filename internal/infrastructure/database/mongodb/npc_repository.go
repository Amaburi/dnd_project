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
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NPCRepository stores the people the party meets.
type NPCRepository struct {
	client *Client
}

// NewNPCRepository creates a new NPC repository.
func NewNPCRepository(client *Client) *NPCRepository {
	return &NPCRepository{client: client}
}

func (r *NPCRepository) collection() *mongo.Collection {
	return r.client.Database().Collection(string(NPCs))
}

// CreateNPC stores a new NPC.
func (r *NPCRepository) CreateNPC(ctx context.Context, npc *models.NPC) error {
	if npc.CampaignID == "" {
		return models.Invalid("campaign_id is required")
	}
	if npc.Status == "" {
		npc.Status = models.NPCAlive
	}
	if err := npc.Validate(); err != nil {
		return err
	}

	if npc.NPCID == "" {
		npc.NPCID = primitive.NewObjectID().Hex()
	}
	now := time.Now().UTC()
	npc.CreatedAt, npc.UpdatedAt = now, now

	if _, err := r.collection().InsertOne(ctx, npc); err != nil {
		return fmt.Errorf("failed to create npc: %w", err)
	}
	return nil
}

func (r *NPCRepository) findNPCs(ctx context.Context, filter bson.M, opts ...*options.FindOptions) ([]*models.NPC, error) {
	cursor, err := r.collection().Find(ctx, filter, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to find npcs: %w", err)
	}
	defer cursor.Close(ctx)

	npcs := []*models.NPC{}
	if err := cursor.All(ctx, &npcs); err != nil {
		return nil, fmt.Errorf("failed to decode npcs: %w", err)
	}
	return npcs, nil
}

// GetNPCsByCampaign lists every NPC in a campaign, most recently seen first.
func (r *NPCRepository) GetNPCsByCampaign(ctx context.Context, campaignID string) ([]*models.NPC, error) {
	opts := options.Find().SetSort(bson.D{{Key: "last_seen", Value: -1}, {Key: "name", Value: 1}})
	return r.findNPCs(ctx, bson.M{"campaign_id": campaignID}, opts)
}

// GetNPCByNPCID looks an NPC up by its business ID.
func (r *NPCRepository) GetNPCByNPCID(ctx context.Context, campaignID, npcID string) (*models.NPC, error) {
	npcs, err := r.findNPCs(ctx, bson.M{"campaign_id": campaignID, "npc_id": npcID})
	if err != nil {
		return nil, err
	}
	if len(npcs) == 0 {
		return nil, nil
	}
	return npcs[0], nil
}

// GetNPCByName finds an NPC by name, case-insensitively.
//
// This is the lookup that matters in play: a player says "I talk to Toblen",
// not "I talk to npc_6512af". The query is escaped before it reaches $regex --
// never interpolate user input into a query operator unescaped.
func (r *NPCRepository) GetNPCByName(ctx context.Context, campaignID, name string) (*models.NPC, error) {
	pattern := "^" + regexp.QuoteMeta(name) + "$"
	npcs, err := r.findNPCs(ctx, bson.M{
		"campaign_id": campaignID,
		"name":        bson.M{"$regex": pattern, "$options": "i"},
	})
	if err != nil {
		return nil, err
	}
	if len(npcs) == 0 {
		return nil, nil
	}
	return npcs[0], nil
}

// UpdateNPC writes the editable fields of an NPC.
//
// Field by field, like every other update here: $set-ing the struct would blank
// the uniquely indexed npc_id and reset created_at. Disposition, memories and
// the meeting counters are deliberately absent -- those are earned in play and
// are written by RecordInteraction, not by a client PUT with a stale body.
func (r *NPCRepository) UpdateNPC(ctx context.Context, campaignID string, npc *models.NPC) error {
	if npc.NPCID == "" {
		return models.Invalid("npc_id is required")
	}
	if err := npc.Validate(); err != nil {
		return err
	}

	now := time.Now().UTC()
	result, err := r.collection().UpdateOne(
		ctx,
		bson.M{"npc_id": npc.NPCID, "campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"name":         npc.Name,
			"role":         npc.Role,
			"race":         npc.Race,
			"location":     npc.Location,
			"appearance":   npc.Appearance,
			"personality":  npc.Personality,
			"voice":        npc.Voice,
			"mannerisms":   npc.Mannerisms,
			"motivations":  npc.Motivations,
			"knowledge":    npc.Knowledge,
			"status":       npc.Status,
			"statblock_id": npc.StatblockID,
			"updated_at":   now,
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update npc: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("npc")
	}
	npc.UpdatedAt = now
	return nil
}

// SaveMemory writes what an NPC remembers and how they feel about the party.
//
// Separate from UpdateNPC because these are earned in play: folding them into
// the edit path would let a client with a stale body undo a grudge.
func (r *NPCRepository) SaveMemory(ctx context.Context, campaignID string, npc *models.NPC) error {
	if npc.NPCID == "" {
		return models.Invalid("npc_id is required")
	}

	result, err := r.collection().UpdateOne(
		ctx,
		bson.M{"npc_id": npc.NPCID, "campaign_id": campaignID},
		bson.M{"$set": bson.M{
			"disposition": npc.Disposition,
			"memories":    npc.Memories,
			"times_met":   npc.TimesMet,
			"first_met":   npc.FirstMet,
			"last_seen":   npc.LastSeen,
			"updated_at":  time.Now().UTC(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to save npc memory: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.NotFound("npc")
	}
	return nil
}

// DeleteNPCInCampaign removes one NPC.
func (r *NPCRepository) DeleteNPCInCampaign(ctx context.Context, campaignID, npcID string) error {
	result, err := r.collection().DeleteOne(ctx,
		bson.M{"npc_id": npcID, "campaign_id": campaignID})
	if err != nil {
		return fmt.Errorf("failed to delete npc: %w", err)
	}
	if result.DeletedCount == 0 {
		return models.NotFound("npc")
	}
	return nil
}

// DeleteNPCsByCampaign removes every NPC in a campaign.
func (r *NPCRepository) DeleteNPCsByCampaign(ctx context.Context, campaignID string) error {
	if _, err := r.collection().DeleteMany(ctx, bson.M{"campaign_id": campaignID}); err != nil {
		return fmt.Errorf("failed to delete npcs: %w", err)
	}
	return nil
}

package handlers

import (
	"net/http"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CampaignHandler handles campaign HTTP requests
type CampaignHandler struct {
	campaignRepo *mongodb.CampaignRepository
	// characterRepo is needed so deleting a campaign can take its characters
	// with it instead of orphaning them.
	characterRepo *mongodb.CharacterRepository
}

// NewCampaignHandler creates a new campaign handler
func NewCampaignHandler(campaignRepo *mongodb.CampaignRepository, characterRepo *mongodb.CharacterRepository) *CampaignHandler {
	return &CampaignHandler{
		campaignRepo:  campaignRepo,
		characterRepo: characterRepo,
	}
}

// CreateCampaign handles POST /api/v1/campaigns
func (h *CampaignHandler) CreateCampaign(c *gin.Context) {
	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.campaignRepo.CreateCampaign(c.Request.Context(), &campaign); err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

// ListCampaigns handles GET /api/v1/campaigns
func (h *CampaignHandler) ListCampaigns(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id query parameter required"})
		return
	}

	campaigns, err := h.campaignRepo.GetCampaignsByUser(c.Request.Context(), userID)
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, campaigns)
}

// GetCampaign handles GET /api/v1/campaigns/:id
func (h *CampaignHandler) GetCampaign(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign ID"})
		return
	}

	campaign, err := h.campaignRepo.GetCampaignByID(c.Request.Context(), id)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if campaign == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// UpdateCampaign handles PUT /api/v1/campaigns/:id
func (h *CampaignHandler) UpdateCampaign(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign ID"})
		return
	}

	var campaign models.Campaign
	if err := c.ShouldBindJSON(&campaign); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns the identity; the body cannot redirect the update.
	campaign.ID = id

	if err := h.campaignRepo.UpdateCampaign(c.Request.Context(), &campaign); err != nil {
		respondRepoError(c, err)
		return
	}

	// Re-read so the response carries the immutable fields (campaign_id,
	// created_by, created_at) that the update deliberately left alone.
	updated, err := h.campaignRepo.GetCampaignByID(c.Request.Context(), id)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteCampaign handles DELETE /api/v1/campaigns/:id
func (h *CampaignHandler) DeleteCampaign(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign ID"})
		return
	}

	ctx := c.Request.Context()

	// Resolve the business ID first: characters reference the campaign by
	// campaign_id, not by _id.
	campaign, err := h.campaignRepo.GetCampaignByID(ctx, id)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if campaign == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
		return
	}

	// Characters go first. If the campaign delete then fails the characters are
	// already gone, but the reverse order would leave unreachable orphans.
	if err := h.characterRepo.DeleteCharactersByCampaign(ctx, campaign.CampaignID); err != nil {
		respondRepoError(c, err)
		return
	}

	if err := h.campaignRepo.DeleteCampaign(ctx, id); err != nil {
		respondRepoError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

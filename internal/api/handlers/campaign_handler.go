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
}

// NewCampaignHandler creates a new campaign handler
func NewCampaignHandler(campaignRepo *mongodb.CampaignRepository) *CampaignHandler {
	return &CampaignHandler{
		campaignRepo: campaignRepo,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	campaign.ID = id

	if err := h.campaignRepo.UpdateCampaign(c.Request.Context(), &campaign); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// DeleteCampaign handles DELETE /api/v1/campaigns/:id
func (h *CampaignHandler) DeleteCampaign(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign ID"})
		return
	}

	if err := h.campaignRepo.DeleteCampaign(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

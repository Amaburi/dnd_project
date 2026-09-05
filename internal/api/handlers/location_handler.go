package handlers

import (
	"net/http"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LocationHandler manages places and the things in them.
type LocationHandler struct {
	locations    *mongodb.LocationRepository
	campaignRepo *mongodb.CampaignRepository
}

// NewLocationHandler creates a new location handler.
func NewLocationHandler(locations *mongodb.LocationRepository, campaignRepo *mongodb.CampaignRepository) *LocationHandler {
	return &LocationHandler{locations: locations, campaignRepo: campaignRepo}
}

func (h *LocationHandler) resolveCampaignID(c *gin.Context) (string, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		badRequest(c, "invalid campaign ID")
		return "", false
	}
	campaign, err := h.campaignRepo.GetCampaignByID(c.Request.Context(), id)
	if err != nil {
		respondRepoError(c, err)
		return "", false
	}
	if campaign == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "campaign not found"})
		return "", false
	}
	return campaign.CampaignID, true
}

// CreateLocation handles POST /api/v1/campaigns/:id/locations
func (h *LocationHandler) CreateLocation(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var location models.Location
	if err := c.ShouldBindJSON(&location); err != nil {
		badRequest(c, err.Error())
		return
	}
	location.CampaignID = campaignID

	if err := h.locations.CreateLocation(c.Request.Context(), &location); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, location)
}

// ListLocations handles GET /api/v1/campaigns/:id/locations
func (h *LocationHandler) ListLocations(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	locations, err := h.locations.GetLocationsByCampaign(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, locations)
}

// GetLocation handles GET /api/v1/campaigns/:id/locations/:location_id
//
// The full document, hidden things included: this is the DM's view. What the
// party can see is a different question, answered by /scene.
func (h *LocationHandler) GetLocation(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	location, err := h.locations.GetLocationByLocationID(c.Request.Context(), campaignID, c.Param("location_id"))
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if location == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "location not found"})
		return
	}
	c.JSON(http.StatusOK, location)
}

// UpdateLocation handles PUT /api/v1/campaigns/:id/locations/:location_id
func (h *LocationHandler) UpdateLocation(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var location models.Location
	if err := c.ShouldBindJSON(&location); err != nil {
		badRequest(c, err.Error())
		return
	}
	location.LocationID = c.Param("location_id")
	location.CampaignID = campaignID

	if err := h.locations.SaveLocation(c.Request.Context(), campaignID, &location); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, location)
}

// CurrentScene handles GET /api/v1/campaigns/:id/scene
//
// What the party can see where they are standing: the same block the DM's
// prompt receives, so a UI can show exactly what the model was told and nothing
// it was not. Hidden things are absent here as they are there.
func (h *LocationHandler) CurrentScene(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	location, err := h.locations.GetCurrentLocation(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if location == nil {
		c.JSON(http.StatusOK, gin.H{
			"scene":         "",
			"interactables": []string{},
			"exits":         []string{},
			"note":          "no location is set for the active session",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"location_id":   location.LocationID,
		"name":          location.Name,
		"lighting":      location.Lighting,
		"scene":         location.SceneBlock(),
		"interactables": location.VisibleInteractables(),
		"exits":         location.VisibleExits(),
	})
}

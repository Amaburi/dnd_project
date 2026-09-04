package handlers

import (
	"net/http"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CharacterHandler handles character HTTP requests
type CharacterHandler struct {
	characterRepo *mongodb.CharacterRepository
	// campaignRepo resolves the :id path segment (a campaign _id) to the
	// campaign_id that characters are actually keyed by.
	campaignRepo *mongodb.CampaignRepository
}

// NewCharacterHandler creates a new character handler
func NewCharacterHandler(characterRepo *mongodb.CharacterRepository, campaignRepo *mongodb.CampaignRepository) *CharacterHandler {
	return &CharacterHandler{
		characterRepo: characterRepo,
		campaignRepo:  campaignRepo,
	}
}

// resolveCampaignID turns the :id path segment into the campaign's business
// campaign_id, responding and returning ok=false if it cannot.
//
// Campaign routes address campaigns by _id, but characters reference their
// campaign by the campaign_id string. Storing the raw path value would link
// characters to an identifier no campaign actually carries.
func (h *CharacterHandler) resolveCampaignID(c *gin.Context) (string, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign ID"})
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

// characterID parses the :char_id path segment.
func characterID(c *gin.Context) (primitive.ObjectID, bool) {
	charID, err := primitive.ObjectIDFromHex(c.Param("char_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return primitive.NilObjectID, false
	}
	return charID, true
}

// CreateCharacter handles POST /api/v1/campaigns/:id/characters
func (h *CharacterHandler) CreateCharacter(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var character models.Character
	if err := c.ShouldBindJSON(&character); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns the campaign link; the body cannot choose it.
	character.CampaignID = campaignID

	if err := h.characterRepo.CreateCharacter(c.Request.Context(), &character); err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, character)
}

// ListCharacters handles GET /api/v1/campaigns/:id/characters
func (h *CharacterHandler) ListCharacters(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	// An optional ?q= filters by name.
	characters, err := h.characterRepo.SearchCharacters(c.Request.Context(), campaignID, c.Query("q"))
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, characters)
}

// GetCharacter handles GET /api/v1/campaigns/:id/characters/:char_id
func (h *CharacterHandler) GetCharacter(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	charID, ok := characterID(c)
	if !ok {
		return
	}

	character, err := h.characterRepo.GetCharacterInCampaign(c.Request.Context(), campaignID, charID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if character == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
		return
	}

	c.JSON(http.StatusOK, character)
}

// UpdateCharacter handles PUT /api/v1/campaigns/:id/characters/:char_id
func (h *CharacterHandler) UpdateCharacter(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	charID, ok := characterID(c)
	if !ok {
		return
	}

	var character models.Character
	if err := c.ShouldBindJSON(&character); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns the identity; the body cannot redirect the update.
	character.ID = charID

	if err := h.characterRepo.UpdateCharacter(c.Request.Context(), campaignID, &character); err != nil {
		respondRepoError(c, err)
		return
	}

	// Re-read so the response carries character_id and created_at, which the
	// update leaves untouched.
	updated, err := h.characterRepo.GetCharacterInCampaign(c.Request.Context(), campaignID, charID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteCharacter handles DELETE /api/v1/campaigns/:id/characters/:char_id
func (h *CharacterHandler) DeleteCharacter(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	charID, ok := characterID(c)
	if !ok {
		return
	}

	if err := h.characterRepo.DeleteCharacterInCampaign(c.Request.Context(), campaignID, charID); err != nil {
		respondRepoError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

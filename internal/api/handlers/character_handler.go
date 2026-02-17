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
}

// NewCharacterHandler creates a new character handler
func NewCharacterHandler(characterRepo *mongodb.CharacterRepository) *CharacterHandler {
	return &CharacterHandler{
		characterRepo: characterRepo,
	}
}

// CreateCharacter handles POST /api/v1/campaigns/:id/characters
func (h *CharacterHandler) CreateCharacter(c *gin.Context) {
	campaignID := c.Param("id")

	var character models.Character
	if err := c.ShouldBindJSON(&character); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	character.CampaignID = campaignID

	if err := h.characterRepo.CreateCharacter(c.Request.Context(), &character); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, character)
}

// ListCharacters handles GET /api/v1/campaigns/:id/characters
func (h *CharacterHandler) ListCharacters(c *gin.Context) {
	campaignID := c.Param("id")

	characters, err := h.characterRepo.GetCharactersByCampaign(c.Request.Context(), campaignID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, characters)
}

// GetCharacter handles GET /api/v1/campaigns/:id/characters/:char_id
func (h *CharacterHandler) GetCharacter(c *gin.Context) {
	charID, err := primitive.ObjectIDFromHex(c.Param("char_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}

	character, err := h.characterRepo.GetCharacterByID(c.Request.Context(), charID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	charID, err := primitive.ObjectIDFromHex(c.Param("char_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}

	var character models.Character
	if err := c.ShouldBindJSON(&character); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	character.ID = charID

	if err := h.characterRepo.UpdateCharacter(c.Request.Context(), &character); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, character)
}

// DeleteCharacter handles DELETE /api/v1/campaigns/:id/characters/:char_id
func (h *CharacterHandler) DeleteCharacter(c *gin.Context) {
	charID, err := primitive.ObjectIDFromHex(c.Param("char_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid character ID"})
		return
	}

	if err := h.characterRepo.DeleteCharacter(c.Request.Context(), charID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

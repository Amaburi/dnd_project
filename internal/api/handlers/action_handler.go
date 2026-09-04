package handlers

import (
	"net/http"

	"github.com/dnd-campaign/manager/internal/application/turn"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ActionHandler runs a player's turn.
//
// This is the endpoint a client calls per player action: one request parses the
// sentence, resolves it against the rules, persists the consequences, narrates
// the verdict and appends it to the campaign's log.
type ActionHandler struct {
	turns        *turn.Service
	campaignRepo *mongodb.CampaignRepository
}

// NewActionHandler creates a new action handler.
func NewActionHandler(turns *turn.Service, campaignRepo *mongodb.CampaignRepository) *ActionHandler {
	return &ActionHandler{turns: turns, campaignRepo: campaignRepo}
}

// actionRequest is the body a client posts.
type actionRequest struct {
	CharacterID string `json:"character_id" binding:"required"`
	Input       string `json:"input" binding:"required"`
	Scene       string `json:"scene"`

	NarrativeVoice string `json:"narrative_voice"`
	CombatTone     string `json:"combat_tone"`
}

// TakeAction handles POST /api/v1/campaigns/:id/actions
func (h *ActionHandler) TakeAction(c *gin.Context) {
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

	var body actionRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Fall back to the campaign's own voice when the client does not override it.
	voice := body.NarrativeVoice
	if voice == "" {
		voice = campaign.AIPersonality.NarrativeVoice
	}

	result, err := h.turns.TakeAction(c.Request.Context(), &turn.Request{
		CampaignID:  campaign.CampaignID,
		CharacterID: body.CharacterID,
		Input:       body.Input,
		Scene:       body.Scene,
		Style:       ai.NarrationStyle{NarrativeVoice: voice, CombatTone: body.CombatTone},
	})
	if err != nil {
		respondRepoError(c, err)
		return
	}

	// A turn that could not be read is a question, not a failure: the client
	// shows the clarification and the player answers it.
	c.JSON(http.StatusOK, result)
}

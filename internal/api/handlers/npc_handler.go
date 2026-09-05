package handlers

import (
	"net/http"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NPCHandler handles the people the party meets.
type NPCHandler struct {
	npcs         *mongodb.NPCRepository
	campaignRepo *mongodb.CampaignRepository
}

// NewNPCHandler creates a new NPC handler.
func NewNPCHandler(npcs *mongodb.NPCRepository, campaignRepo *mongodb.CampaignRepository) *NPCHandler {
	return &NPCHandler{npcs: npcs, campaignRepo: campaignRepo}
}

// resolveCampaignID turns the :id path segment into the campaign's business ID.
func (h *NPCHandler) resolveCampaignID(c *gin.Context) (string, bool) {
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

// CreateNPC handles POST /api/v1/campaigns/:id/npcs
func (h *NPCHandler) CreateNPC(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var npc models.NPC
	if err := c.ShouldBindJSON(&npc); err != nil {
		badRequest(c, err.Error())
		return
	}
	npc.CampaignID = campaignID

	// Disposition and memory are earned in play, not declared at creation: a
	// client that could post "disposition: 100" could buy a friendship.
	npc.Disposition = 0
	npc.Memories = nil
	npc.TimesMet = 0

	if err := h.npcs.CreateNPC(c.Request.Context(), &npc); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, npc)
}

// ListNPCs handles GET /api/v1/campaigns/:id/npcs
func (h *NPCHandler) ListNPCs(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	npcs, err := h.npcs.GetNPCsByCampaign(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, npcs)
}

// GetNPC handles GET /api/v1/campaigns/:id/npcs/:npc_id
func (h *NPCHandler) GetNPC(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	npc, err := h.npcs.GetNPCByNPCID(c.Request.Context(), campaignID, c.Param("npc_id"))
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if npc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "npc not found"})
		return
	}
	c.JSON(http.StatusOK, npc)
}

// UpdateNPC handles PUT /api/v1/campaigns/:id/npcs/:npc_id
//
// Identity and personality only. Disposition and memories are not editable
// here: they are earned, and a PUT with a stale body would undo a grudge.
func (h *NPCHandler) UpdateNPC(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var npc models.NPC
	if err := c.ShouldBindJSON(&npc); err != nil {
		badRequest(c, err.Error())
		return
	}
	npc.NPCID = c.Param("npc_id")
	npc.CampaignID = campaignID

	if err := h.npcs.UpdateNPC(c.Request.Context(), campaignID, &npc); err != nil {
		respondRepoError(c, err)
		return
	}

	// Re-read so the response carries what was earned as well as what changed.
	updated, err := h.npcs.GetNPCByNPCID(c.Request.Context(), campaignID, npc.NPCID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteNPC handles DELETE /api/v1/campaigns/:id/npcs/:npc_id
func (h *NPCHandler) DeleteNPC(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	if err := h.npcs.DeleteNPCInCampaign(c.Request.Context(), campaignID, c.Param("npc_id")); err != nil {
		respondRepoError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

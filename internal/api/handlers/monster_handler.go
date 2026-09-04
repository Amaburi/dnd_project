package handlers

import (
	"net/http"
	"strconv"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MonsterHandler handles monster statblock HTTP requests.
type MonsterHandler struct {
	monsterRepo *mongodb.MonsterRepository
	// campaignRepo resolves the :id path segment (a campaign _id) to the
	// campaign_id that monsters are keyed by.
	campaignRepo *mongodb.CampaignRepository
}

// NewMonsterHandler creates a new monster handler.
func NewMonsterHandler(monsterRepo *mongodb.MonsterRepository, campaignRepo *mongodb.CampaignRepository) *MonsterHandler {
	return &MonsterHandler{monsterRepo: monsterRepo, campaignRepo: campaignRepo}
}

// resolveCampaignID turns the :id path segment into the campaign's business
// campaign_id, responding and returning ok=false if it cannot.
func (h *MonsterHandler) resolveCampaignID(c *gin.Context) (string, bool) {
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

// monsterID parses the :monster_id path segment.
func monsterID(c *gin.Context) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param("monster_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid monster ID"})
		return primitive.NilObjectID, false
	}
	return id, true
}

// CreateMonster handles POST /api/v1/campaigns/:id/monsters
func (h *MonsterHandler) CreateMonster(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var monster models.Monster
	if err := c.ShouldBindJSON(&monster); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns the campaign link; the body cannot choose it.
	monster.CampaignID = campaignID

	if err := h.monsterRepo.CreateMonster(c.Request.Context(), &monster); err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, monster)
}

// ListMonsters handles GET /api/v1/campaigns/:id/monsters
//
// Optional query parameters: ?q= filters by name, ?min_cr= and ?max_cr= filter
// by challenge rating for building an encounter to a budget.
func (h *MonsterHandler) ListMonsters(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	minCR, maxCR, filtered, err := challengeRatingRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var monsters []*models.Monster
	if filtered {
		monsters, err = h.monsterRepo.GetMonstersByChallengeRating(c.Request.Context(), campaignID, minCR, maxCR)
	} else {
		monsters, err = h.monsterRepo.SearchMonsters(c.Request.Context(), campaignID, c.Query("q"))
	}
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, monsters)
}

// challengeRatingRange reads the optional min_cr and max_cr query parameters.
func challengeRatingRange(c *gin.Context) (min, max float64, filtered bool, err error) {
	minText, hasMin := c.GetQuery("min_cr")
	maxText, hasMax := c.GetQuery("max_cr")
	if !hasMin && !hasMax {
		return 0, 0, false, nil
	}

	max = 30
	if hasMin {
		if min, err = strconv.ParseFloat(minText, 64); err != nil {
			return 0, 0, false, models.Invalid("min_cr must be a number")
		}
	}
	if hasMax {
		if max, err = strconv.ParseFloat(maxText, 64); err != nil {
			return 0, 0, false, models.Invalid("max_cr must be a number")
		}
	}
	if min > max {
		return 0, 0, false, models.Invalid("min_cr %v is greater than max_cr %v", min, max)
	}
	return min, max, true, nil
}

// GetMonster handles GET /api/v1/campaigns/:id/monsters/:monster_id
func (h *MonsterHandler) GetMonster(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	id, ok := monsterID(c)
	if !ok {
		return
	}

	monster, err := h.monsterRepo.GetMonsterInCampaign(c.Request.Context(), campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if monster == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "monster not found"})
		return
	}

	c.JSON(http.StatusOK, monster)
}

// UpdateMonster handles PUT /api/v1/campaigns/:id/monsters/:monster_id
func (h *MonsterHandler) UpdateMonster(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	id, ok := monsterID(c)
	if !ok {
		return
	}

	var monster models.Monster
	if err := c.ShouldBindJSON(&monster); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns the identity; the body cannot redirect the update.
	monster.ID = id

	if err := h.monsterRepo.UpdateMonster(c.Request.Context(), campaignID, &monster); err != nil {
		respondRepoError(c, err)
		return
	}

	// Re-read so the response carries monster_id and created_at, which the
	// update leaves untouched.
	updated, err := h.monsterRepo.GetMonsterInCampaign(c.Request.Context(), campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "monster not found"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// DeleteMonster handles DELETE /api/v1/campaigns/:id/monsters/:monster_id
func (h *MonsterHandler) DeleteMonster(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	id, ok := monsterID(c)
	if !ok {
		return
	}

	if err := h.monsterRepo.DeleteMonsterInCampaign(c.Request.Context(), campaignID, id); err != nil {
		respondRepoError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// SeedMonsters handles POST /api/v1/campaigns/:id/monsters/seed
//
// Copies the SRD catalogue into the campaign, skipping any statblock already
// present, so calling it twice is harmless.
func (h *MonsterHandler) SeedMonsters(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	seeded, err := h.monsterRepo.SeedSRDMonsters(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"seeded": seeded})
}

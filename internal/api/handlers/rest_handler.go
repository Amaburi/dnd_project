package handlers

import (
	"fmt"
	"net/http"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/gin-gonic/gin"
)

// restRequest is a rest a character is taking.
type restRequest struct {
	// Type is "short" or "long".
	Type string `json:"type"`

	// HitDice are the die sizes to spend, one entry per die. Short rests only:
	// a long rest already restores every hit point, so spending dice during one
	// would burn a resource for nothing.
	HitDice []int `json:"hit_dice,omitempty"`
}

// spentDie records one hit die and what it bought.
type spentDie struct {
	Die    int `json:"die"`
	Rolled int `json:"rolled"`
	Healed int `json:"healed"`
}

type restResponse struct {
	Type            string            `json:"type"`
	HitPointsBefore int               `json:"hit_points_before"`
	HitPointsAfter  int               `json:"hit_points_after"`
	Healed          int               `json:"healed"`
	HitDiceSpent    []spentDie        `json:"hit_dice_spent"`
	Character       *models.Character `json:"character"`
}

// Rest handles POST /api/v1/campaigns/:id/characters/:char_id/rest
//
// The rules live on the character (ShortRest, LongRest, SpendHitDie); this
// moves the character between the request and the store and owns the only
// thing the model deliberately does not -- the randomness of a hit die.
func (h *CharacterHandler) Rest(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	charID, ok := characterID(c)
	if !ok {
		return
	}

	var req restRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, err.Error())
		return
	}
	if req.Type != "short" && req.Type != "long" {
		badRequest(c, `type must be "short" or "long"`)
		return
	}
	if req.Type == "long" && len(req.HitDice) > 0 {
		badRequest(c, "a long rest restores every hit point; spending hit dice during one would waste them")
		return
	}

	ctx := c.Request.Context()
	character, err := h.characterRepo.GetCharacterInCampaign(ctx, campaignID, charID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if character == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
		return
	}

	allowed, reason := character.CanBenefitFromShortRest()
	if req.Type == "long" {
		allowed, reason = character.CanBenefitFromLongRest()
	}
	if !allowed {
		badRequest(c, reason)
		return
	}

	response := restResponse{
		Type:            req.Type,
		HitPointsBefore: character.CombatStats.HitPoints.Current,
		HitDiceSpent:    []spentDie{},
	}

	if req.Type == "long" {
		character.LongRest()
	} else {
		character.ShortRest()

		// Each die is rolled and spent one at a time, and a failure stops the
		// rest rather than rolling on: the dice already spent stay spent, which
		// is what happens at a table, and the response says exactly which.
		for _, die := range req.HitDice {
			before := character.CombatStats.HitPoints.Current
			rolled := h.roller.RollHitDie(die)
			if err := character.SpendHitDie(die, rolled); err != nil {
				badRequest(c, fmt.Sprintf("%s (%d of %d dice were spent)",
					err.Error(), len(response.HitDiceSpent), len(req.HitDice)))
				return
			}
			response.HitDiceSpent = append(response.HitDiceSpent, spentDie{
				Die: die, Rolled: rolled,
				Healed: character.CombatStats.HitPoints.Current - before,
			})
		}
	}

	if err := h.characterRepo.UpdateCharacter(ctx, campaignID, character); err != nil {
		respondRepoError(c, err)
		return
	}

	response.HitPointsAfter = character.CombatStats.HitPoints.Current
	response.Healed = response.HitPointsAfter - response.HitPointsBefore
	response.Character = character
	c.JSON(http.StatusOK, response)
}

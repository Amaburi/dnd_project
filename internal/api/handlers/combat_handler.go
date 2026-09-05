package handlers

import (
	"net/http"

	"github.com/dnd-campaign/manager/internal/application/encounter"
	"github.com/dnd-campaign/manager/internal/domain/combat"
	"github.com/dnd-campaign/manager/internal/domain/dice"
	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CombatHandler runs encounters.
//
// The tracker owns whose turn it is and the rules engine owns what happens on
// it; this handler only moves an encounter between the two and saves the
// result.
type CombatHandler struct {
	encounters   *mongodb.EncounterRepository
	characters   *mongodb.CharacterRepository
	monsters     *mongodb.MonsterRepository
	sessions     *mongodb.SessionRepository
	campaignRepo *mongodb.CampaignRepository
	roller       *dice.Roller

	// turns plays a monster's turn. The orchestration lives in
	// application/encounter so it can be tested without a database.
	turns *encounter.Service
}

// NewCombatHandler creates a new combat handler.
func NewCombatHandler(
	encounters *mongodb.EncounterRepository,
	characters *mongodb.CharacterRepository,
	monsters *mongodb.MonsterRepository,
	sessions *mongodb.SessionRepository,
	campaignRepo *mongodb.CampaignRepository,
	roller *dice.Roller,
	turns *encounter.Service,
) *CombatHandler {
	return &CombatHandler{
		encounters: encounters, characters: characters, monsters: monsters,
		sessions: sessions, campaignRepo: campaignRepo, roller: roller,
		turns: turns,
	}
}

func (h *CombatHandler) resolveCampaignID(c *gin.Context) (string, bool) {
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

// loadEncounter resolves the campaign and encounter, returning a tracker.
func (h *CombatHandler) loadEncounter(c *gin.Context) (string, *combat.Tracker, bool) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return "", nil, false
	}

	id, err := primitive.ObjectIDFromHex(c.Param("encounter_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid encounter ID"})
		return "", nil, false
	}

	encounter, err := h.encounters.GetEncounterInCampaign(c.Request.Context(), campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return "", nil, false
	}
	if encounter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
		return "", nil, false
	}
	return campaignID, combat.NewTracker(encounter), true
}

// save writes the tracker's state back and responds with the encounter.
func (h *CombatHandler) save(c *gin.Context, campaignID string, tracker *combat.Tracker, status int) {
	if err := h.encounters.SaveEncounter(c.Request.Context(), campaignID, tracker.Encounter()); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(status, tracker.Encounter())
}

// CreateEncounter handles POST /api/v1/campaigns/:id/encounters
//
// The encounter is attached to the session in progress, so a fight always has
// a place in the campaign's history.
func (h *CombatHandler) CreateEncounter(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var encounter models.CombatEncounter
	if err := c.ShouldBindJSON(&encounter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	encounter.CampaignID = campaignID

	if session, err := h.sessions.GetActiveSession(c.Request.Context(), campaignID); err == nil && session != nil {
		encounter.SessionID = session.SessionID
	}

	if err := h.encounters.CreateEncounter(c.Request.Context(), &encounter); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, encounter)
}

// ListEncounters handles GET /api/v1/campaigns/:id/encounters
func (h *CombatHandler) ListEncounters(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	encounters, err := h.encounters.GetEncountersByCampaign(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, encounters)
}

// GetActiveEncounter handles GET /api/v1/campaigns/:id/encounters/active
func (h *CombatHandler) GetActiveEncounter(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	encounter, err := h.encounters.GetActiveEncounter(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if encounter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no encounter is under way"})
		return
	}
	c.JSON(http.StatusOK, encounter)
}

// GetEncounter handles GET /api/v1/campaigns/:id/encounters/:encounter_id
func (h *CombatHandler) GetEncounter(c *gin.Context) {
	_, tracker, ok := h.loadEncounter(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, tracker.Encounter())
}

// joinRequest adds a creature to an encounter, by whichever id it has.
type joinRequest struct {
	CharacterID string `json:"character_id"`
	MonsterID   string `json:"monster_id"`
	Initiative  int    `json:"initiative"`
}

// AddCombatant handles POST /api/v1/campaigns/:id/encounters/:encounter_id/combatants
//
// The combatant is built from the sheet or statblock rather than the request
// body, so its armour class, affinities and death-save behaviour cannot be
// misreported by a client.
func (h *CombatHandler) AddCombatant(c *gin.Context) {
	campaignID, tracker, ok := h.loadEncounter(c)
	if !ok {
		return
	}

	var body joinRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	var combatant models.Combatant

	switch {
	case body.CharacterID != "":
		character, err := h.characters.GetCharacterByCharacterID(ctx, body.CharacterID)
		if err != nil {
			respondRepoError(c, err)
			return
		}
		if character == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "character not found"})
			return
		}
		combatant = character.ToCombatant(character.CharacterID)

	case body.MonsterID != "":
		monsters, err := h.monsters.GetMonstersByCampaign(ctx, campaignID)
		if err != nil {
			respondRepoError(c, err)
			return
		}
		var found *models.Monster
		for _, m := range monsters {
			if m.MonsterID == body.MonsterID {
				found = m
				break
			}
		}
		if found == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "monster not found"})
			return
		}
		combatant = found.ToCombatant(found.MonsterID)

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "character_id or monster_id is required"})
		return
	}

	combatant.Initiative = body.Initiative

	if err := tracker.AddCombatant(combatant); err != nil {
		respondRepoError(c, err)
		return
	}
	h.save(c, campaignID, tracker, http.StatusOK)
}

// RollInitiative handles POST /api/v1/campaigns/:id/encounters/:encounter_id/initiative
func (h *CombatHandler) RollInitiative(c *gin.Context) {
	campaignID, tracker, ok := h.loadEncounter(c)
	if !ok {
		return
	}

	if err := tracker.RollInitiative(h.roller); err != nil {
		respondRepoError(c, err)
		return
	}
	h.save(c, campaignID, tracker, http.StatusOK)
}

// NextTurn handles POST /api/v1/campaigns/:id/encounters/:encounter_id/next-turn
//
// Advancing also checks whether the fight is over, so a client never has to
// ask separately and can never miss it.
func (h *CombatHandler) NextTurn(c *gin.Context) {
	campaignID, tracker, ok := h.loadEncounter(c)
	if !ok {
		return
	}

	if _, decided := tracker.EndIfDecided(); !decided {
		if _, ok := tracker.NextTurn(); !ok {
			c.JSON(http.StatusConflict, gin.H{"error": "the encounter cannot advance"})
			return
		}
		tracker.EndIfDecided()
	}
	h.save(c, campaignID, tracker, http.StatusOK)
}

// EndEncounter handles POST /api/v1/campaigns/:id/encounters/:encounter_id/end
func (h *CombatHandler) EndEncounter(c *gin.Context) {
	campaignID, tracker, ok := h.loadEncounter(c)
	if !ok {
		return
	}

	var body struct {
		Outcome string `json:"outcome"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Outcome == "" {
		body.Outcome = "resolved"
	}

	tracker.End(body.Outcome)
	h.save(c, campaignID, tracker, http.StatusOK)
}

// EncounterStats handles GET /api/v1/campaigns/:id/encounters/:encounter_id/stats
func (h *CombatHandler) EncounterStats(c *gin.Context) {
	_, tracker, ok := h.loadEncounter(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, mongodb.Stats(tracker.Encounter()))
}

// DeleteEncounter handles DELETE /api/v1/campaigns/:id/encounters/:encounter_id
func (h *CombatHandler) DeleteEncounter(c *gin.Context) {
	campaignID, tracker, ok := h.loadEncounter(c)
	if !ok {
		return
	}

	if err := h.encounters.DeleteEncounterInCampaign(c.Request.Context(), campaignID, tracker.Encounter().ID); err != nil {
		respondRepoError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ResolveTurn handles POST /api/v1/campaigns/:id/encounters/:encounter_id/resolve-turn
//
// This is the half of combat that was missing: the tracker could say whose turn
// it was and nothing could play it, so every fight was one-sided. It resolves
// the current monster's turn -- choose, roll, apply, narrate, advance.
//
// A player's turn is reported as awaiting them rather than played for them;
// players act through POST /campaigns/:id/actions.
func (h *CombatHandler) ResolveTurn(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	id, err := primitive.ObjectIDFromHex(c.Param("encounter_id"))
	if err != nil {
		badRequest(c, "invalid encounter ID")
		return
	}

	ctx := c.Request.Context()
	encounter, err := h.encounters.GetEncounterInCampaign(ctx, campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if encounter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "encounter not found"})
		return
	}

	result, err := h.turns.ResolveTurn(ctx, campaignID, encounter.EncounterID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

package handlers

import (
	"net/http"
	"strconv"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SessionHandler handles game session and story event HTTP requests.
//
// Sessions and their events share a handler because they are one feature: a
// session is the container, the events are what happened inside it, and the
// two are always read together.
type SessionHandler struct {
	sessionRepo *mongodb.SessionRepository
	eventRepo   *mongodb.StoryEventRepository
	// campaignRepo resolves the :id path segment (a campaign _id) to the
	// campaign_id that sessions are keyed by.
	campaignRepo *mongodb.CampaignRepository
}

// NewSessionHandler creates a new session handler.
func NewSessionHandler(
	sessionRepo *mongodb.SessionRepository,
	eventRepo *mongodb.StoryEventRepository,
	campaignRepo *mongodb.CampaignRepository,
) *SessionHandler {
	return &SessionHandler{sessionRepo: sessionRepo, eventRepo: eventRepo, campaignRepo: campaignRepo}
}

// resolveCampaignID turns the :id path segment into the campaign's business
// campaign_id, responding and returning ok=false if it cannot.
func (h *SessionHandler) resolveCampaignID(c *gin.Context) (string, bool) {
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

// sessionObjectID parses the :session_id path segment.
func sessionObjectID(c *gin.Context) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(c.Param("session_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return primitive.NilObjectID, false
	}
	return id, true
}

// loadSession resolves the campaign and session in one step, which every
// per-session route needs.
func (h *SessionHandler) loadSession(c *gin.Context) (string, *models.Session, bool) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return "", nil, false
	}
	id, ok := sessionObjectID(c)
	if !ok {
		return "", nil, false
	}

	session, err := h.sessionRepo.GetSessionInCampaign(c.Request.Context(), campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return "", nil, false
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return "", nil, false
	}
	return campaignID, session, true
}

// CreateSession handles POST /api/v1/campaigns/:id/sessions
func (h *SessionHandler) CreateSession(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var session models.Session
	if err := c.ShouldBindJSON(&session); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns the campaign link; the body cannot choose it.
	session.CampaignID = campaignID

	if err := h.sessionRepo.CreateSession(c.Request.Context(), &session); err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, session)
}

// ListSessions handles GET /api/v1/campaigns/:id/sessions
func (h *SessionHandler) ListSessions(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	sessions, err := h.sessionRepo.GetSessionsByCampaign(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// GetActiveSession handles GET /api/v1/campaigns/:id/sessions/active
//
// A separate route rather than a filter, because "which session am I in?" is
// the question a client asks on every page load.
func (h *SessionHandler) GetActiveSession(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	session, err := h.sessionRepo.GetActiveSession(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no session is in progress"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// GetSession handles GET /api/v1/campaigns/:id/sessions/:session_id
func (h *SessionHandler) GetSession(c *gin.Context) {
	_, session, ok := h.loadSession(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, session)
}

// UpdateSession handles PUT /api/v1/campaigns/:id/sessions/:session_id
func (h *SessionHandler) UpdateSession(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	id, ok := sessionObjectID(c)
	if !ok {
		return
	}

	var session models.Session
	if err := c.ShouldBindJSON(&session); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns the identity; the body cannot redirect the update.
	session.ID = id

	if err := h.sessionRepo.UpdateSession(c.Request.Context(), campaignID, &session); err != nil {
		respondRepoError(c, err)
		return
	}

	// Re-read so the response carries session_id and created_at, which the
	// update leaves untouched.
	updated, err := h.sessionRepo.GetSessionInCampaign(c.Request.Context(), campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, updated)
}

// StartSession handles POST /api/v1/campaigns/:id/sessions/:session_id/start
func (h *SessionHandler) StartSession(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	id, ok := sessionObjectID(c)
	if !ok {
		return
	}

	session, err := h.sessionRepo.StartSession(c.Request.Context(), campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, session)
}

// EndSession handles POST /api/v1/campaigns/:id/sessions/:session_id/end
func (h *SessionHandler) EndSession(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	id, ok := sessionObjectID(c)
	if !ok {
		return
	}

	session, err := h.sessionRepo.EndSession(c.Request.Context(), campaignID, id)
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteSession handles DELETE /api/v1/campaigns/:id/sessions/:session_id
//
// A session's events go with it: an orphaned log belongs to a session nothing
// can name.
func (h *SessionHandler) DeleteSession(c *gin.Context) {
	campaignID, session, ok := h.loadSession(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := h.eventRepo.DeleteEventsBySession(ctx, campaignID, session.SessionID); err != nil {
		respondRepoError(c, err)
		return
	}
	if err := h.sessionRepo.DeleteSessionInCampaign(ctx, campaignID, session.ID); err != nil {
		respondRepoError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Story events
// ---------------------------------------------------------------------------

// AppendEvent handles POST /api/v1/campaigns/:id/sessions/:session_id/events
//
// There is no update or partial-edit route on purpose: the log is append-only,
// because rewriting what happened is how a campaign's history and its state
// drift apart. A correction is another event.
func (h *SessionHandler) AppendEvent(c *gin.Context) {
	campaignID, session, ok := h.loadSession(c)
	if !ok {
		return
	}

	var event models.StoryEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The path owns both links; the body cannot choose either.
	event.CampaignID = campaignID
	event.SessionID = session.SessionID

	if err := h.eventRepo.AppendEvent(c.Request.Context(), &event); err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusCreated, event)
}

// ListEvents handles GET /api/v1/campaigns/:id/sessions/:session_id/events
//
// Optional ?type= filters by event type, which is how a dice history is read.
func (h *SessionHandler) ListEvents(c *gin.Context) {
	campaignID, session, ok := h.loadSession(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	var (
		events []*models.StoryEvent
		err    error
	)
	if eventType := c.Query("type"); eventType != "" {
		events, err = h.eventRepo.GetEventsByType(ctx, campaignID, session.SessionID, eventType)
	} else {
		events, err = h.eventRepo.GetEventsBySession(ctx, campaignID, session.SessionID)
	}
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, events)
}

// RecentEvents handles GET /api/v1/campaigns/:id/events/recent
//
// This is what fills the "Recent Events" a narration prompt asks for, so the
// response carries both the events and the rendered context block.
func (h *SessionHandler) RecentEvents(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	limit := mongodb.DefaultRecentEvents
	if text := c.Query("limit"); text != "" {
		parsed, err := strconv.Atoi(text)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a number"})
			return
		}
		limit = parsed
	}

	events, err := h.eventRepo.GetRecentEvents(c.Request.Context(), campaignID, limit)
	if err != nil {
		respondRepoError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events":  events,
		"context": models.NarrativeContext(events),
	})
}

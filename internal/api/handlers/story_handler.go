package handlers

import (
	"net/http"
	"time"

	"github.com/dnd-campaign/manager/internal/application/story"
	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/database/mongodb"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// StoryHandler manages the DM's outstanding work.
type StoryHandler struct {
	story        *mongodb.StoryRepository
	campaignRepo *mongodb.CampaignRepository

	// review reads recent play and writes down what it opened. Optional: the
	// threads and consequences routes work without it, they are just managed
	// by hand.
	review *story.Service
}

// NewStoryHandler creates a new story handler.
func NewStoryHandler(
	storyRepo *mongodb.StoryRepository,
	campaignRepo *mongodb.CampaignRepository,
	review *story.Service,
) *StoryHandler {
	return &StoryHandler{story: storyRepo, campaignRepo: campaignRepo, review: review}
}

// Review handles POST /api/v1/campaigns/:id/story/review
//
// It reads the recent log and records the storylines it opened and the choices
// that have not come back around. Run it at a session boundary rather than per
// turn: it costs a provider call, and a storyline reads better over a stretch
// of play than over one sentence.
func (h *StoryHandler) Review(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}
	if h.review == nil {
		c.JSON(http.StatusServiceUnavailable,
			gin.H{"error": "story review is not configured; no AI provider is wired up"})
		return
	}

	result, err := h.review.Review(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result, "summary": result.Summary()})
}

func (h *StoryHandler) resolveCampaignID(c *gin.Context) (string, bool) {
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

// loadThread fetches the thread named in the path.
func (h *StoryHandler) loadThread(c *gin.Context) (string, *models.PlotThread, bool) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return "", nil, false
	}
	thread, err := h.story.GetThreadByThreadID(c.Request.Context(), campaignID, c.Param("thread_id"))
	if err != nil {
		respondRepoError(c, err)
		return "", nil, false
	}
	if thread == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plot thread not found"})
		return "", nil, false
	}
	return campaignID, thread, true
}

// CreateThread handles POST /api/v1/campaigns/:id/threads
func (h *StoryHandler) CreateThread(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var thread models.PlotThread
	if err := c.ShouldBindJSON(&thread); err != nil {
		badRequest(c, err.Error())
		return
	}
	thread.CampaignID = campaignID

	// A thread's history is written by advancing it, not declared at creation.
	thread.Beats = nil
	thread.Resolution = ""

	if err := h.story.CreateThread(c.Request.Context(), &thread); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, thread)
}

// ListThreads handles GET /api/v1/campaigns/:id/threads
//
// Everything by default; ?status=live for only what is outstanding, which is
// what the DM is actually holding.
func (h *StoryHandler) ListThreads(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	var (
		threads []*models.PlotThread
		err     error
	)
	if c.Query("status") == "live" {
		threads, err = h.story.GetLiveThreads(ctx, campaignID)
	} else {
		threads, err = h.story.GetThreadsByCampaign(ctx, campaignID)
	}
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, threads)
}

// GetThread handles GET /api/v1/campaigns/:id/threads/:thread_id
func (h *StoryHandler) GetThread(c *gin.Context) {
	_, thread, ok := h.loadThread(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, thread)
}

// AdvanceThread handles POST /api/v1/campaigns/:id/threads/:thread_id/advance
func (h *StoryHandler) AdvanceThread(c *gin.Context) {
	campaignID, thread, ok := h.loadThread(c)
	if !ok {
		return
	}

	var beat models.ThreadBeat
	if err := c.ShouldBindJSON(&beat); err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := thread.Advance(beat); err != nil {
		respondRepoError(c, err)
		return
	}
	if err := h.story.SaveThread(c.Request.Context(), campaignID, thread); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, thread)
}

// ResolveThread handles POST /api/v1/campaigns/:id/threads/:thread_id/resolve
//
// The same route closes a thread three ways, because they differ only in what
// happened: finished, walked away from, or gone quiet.
func (h *StoryHandler) ResolveThread(c *gin.Context) {
	campaignID, thread, ok := h.loadThread(c)
	if !ok {
		return
	}

	var body struct {
		Disposition string `json:"disposition"` // resolved | abandoned | dormant | reopened
		How         string `json:"how"`
		SessionID   string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	var err error
	switch body.Disposition {
	case "", "resolved":
		err = thread.Resolve(body.How, body.SessionID)
	case "abandoned":
		err = thread.Abandon(body.How)
	case "dormant":
		err = thread.SetDormant()
	case "reopened":
		err = thread.Reopen()
	default:
		badRequest(c, `disposition must be "resolved", "abandoned", "dormant" or "reopened"`)
		return
	}
	if err != nil {
		respondRepoError(c, err)
		return
	}

	if err := h.story.SaveThread(c.Request.Context(), campaignID, thread); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, thread)
}

// CreateConsequence handles POST /api/v1/campaigns/:id/consequences
func (h *StoryHandler) CreateConsequence(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var consequence models.Consequence
	if err := c.ShouldBindJSON(&consequence); err != nil {
		badRequest(c, err.Error())
		return
	}
	consequence.CampaignID = campaignID

	// A consequence is created pending. Declaring it already landed would make
	// it a record of the past rather than a debt owed to the party.
	consequence.Status = models.ConsequencePending
	consequence.Outcome = ""

	if err := h.story.CreateConsequence(c.Request.Context(), &consequence); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, consequence)
}

// ListConsequences handles GET /api/v1/campaigns/:id/consequences
func (h *StoryHandler) ListConsequences(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	var (
		found []*models.Consequence
		err   error
	)
	if c.Query("status") == "pending" {
		found, err = h.story.GetPendingConsequences(ctx, campaignID)
	} else {
		found, err = h.story.GetConsequencesByCampaign(ctx, campaignID)
	}
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, found)
}

// SettleConsequence handles POST /api/v1/campaigns/:id/consequences/:consequence_id/settle
func (h *StoryHandler) SettleConsequence(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	consequence, err := h.story.GetConsequenceByID(ctx, campaignID, c.Param("consequence_id"))
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if consequence == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "consequence not found"})
		return
	}

	var body struct {
		Disposition string `json:"disposition"` // realised | averted | expired
		Outcome     string `json:"outcome"`
		SessionID   string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	switch body.Disposition {
	case "", "realised":
		err = consequence.Realise(body.Outcome, body.SessionID)
	case "averted":
		err = consequence.Avert(body.Outcome, body.SessionID)
	case "expired":
		err = consequence.Expire(body.Outcome)
	default:
		badRequest(c, `disposition must be "realised", "averted" or "expired"`)
		return
	}
	if err != nil {
		respondRepoError(c, err)
		return
	}

	if err := h.story.SaveConsequence(ctx, campaignID, consequence); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, consequence)
}

// --- story arcs -------------------------------------------------------------

// CreateArc handles POST /api/v1/campaigns/:id/arcs
func (h *StoryHandler) CreateArc(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	var arc models.StoryArc
	if err := c.ShouldBindJSON(&arc); err != nil {
		badRequest(c, err.Error())
		return
	}
	arc.CampaignID = campaignID

	// How an arc ended is written by ending it, not declared at creation.
	arc.Resolution = ""
	arc.CompletedAt = time.Time{}

	if err := h.story.CreateArc(c.Request.Context(), &arc); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, arc)
}

// ListArcs handles GET /api/v1/campaigns/:id/arcs
func (h *StoryHandler) ListArcs(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	arcs, err := h.story.GetArcsByCampaign(c.Request.Context(), campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, arcs)
}

// GetArc handles GET /api/v1/campaigns/:id/arcs/:arc_id
//
// The response carries the arc's progress, because "two of five storylines
// settled" is the number a DM actually wants and it cannot be read off the
// document alone.
func (h *StoryHandler) GetArc(c *gin.Context) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	arc, err := h.story.GetArcByArcID(ctx, campaignID, c.Param("arc_id"))
	if err != nil {
		respondRepoError(c, err)
		return
	}
	if arc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "story arc not found"})
		return
	}

	threads, err := h.story.GetThreadsByCampaign(ctx, campaignID)
	if err != nil {
		respondRepoError(c, err)
		return
	}
	settled, total := arc.Progress(threads)
	canComplete, blocker := arc.CanComplete(threads)

	c.JSON(http.StatusOK, gin.H{
		"arc": arc,
		"progress": gin.H{
			"settled": settled, "total": total,
			"can_complete": canComplete, "blocked_by": blocker,
		},
	})
}

// AdvanceArc handles POST /api/v1/campaigns/:id/arcs/:arc_id/advance
func (h *StoryHandler) AdvanceArc(c *gin.Context) {
	campaignID, arc, ok := h.loadArc(c)
	if !ok {
		return
	}
	if err := arc.AdvanceStage(); err != nil {
		respondRepoError(c, err)
		return
	}
	if err := h.story.SaveArc(c.Request.Context(), campaignID, arc); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, arc)
}

// CompleteArc handles POST /api/v1/campaigns/:id/arcs/:arc_id/complete
//
// Completion is checked against the arc's storylines: an arc cannot be finished
// while the threads it is made of are still open, or the DM would believe it is
// done while the prompt still lists them as outstanding.
func (h *StoryHandler) CompleteArc(c *gin.Context) {
	campaignID, arc, ok := h.loadArc(c)
	if !ok {
		return
	}

	var body struct {
		Disposition string `json:"disposition"` // completed | abandoned | activated
		How         string `json:"how"`
		SessionID   string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	var err error
	switch body.Disposition {
	case "", "completed":
		var threads []*models.PlotThread
		if threads, err = h.story.GetThreadsByCampaign(ctx, campaignID); err == nil {
			err = arc.CompleteWith(body.How, body.SessionID, threads)
		}
	case "abandoned":
		err = arc.Abandon(body.How)
	case "activated":
		arc.Status = models.ArcActive
		arc.StartedSession = body.SessionID
	default:
		badRequest(c, `disposition must be "completed", "abandoned" or "activated"`)
		return
	}
	if err != nil {
		respondRepoError(c, err)
		return
	}

	if err := h.story.SaveArc(ctx, campaignID, arc); err != nil {
		respondRepoError(c, err)
		return
	}
	c.JSON(http.StatusOK, arc)
}

// loadArc fetches the arc named in the path.
func (h *StoryHandler) loadArc(c *gin.Context) (string, *models.StoryArc, bool) {
	campaignID, ok := h.resolveCampaignID(c)
	if !ok {
		return "", nil, false
	}
	arc, err := h.story.GetArcByArcID(c.Request.Context(), campaignID, c.Param("arc_id"))
	if err != nil {
		respondRepoError(c, err)
		return "", nil, false
	}
	if arc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "story arc not found"})
		return "", nil, false
	}
	return campaignID, arc, true
}

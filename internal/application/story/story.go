// Package story turns what happened into what is still outstanding.
//
// The DM opens storylines while narrating and then forgets them, and the party
// makes choices that nothing remembers. This is the pass that writes both down.
//
// It is deliberately not part of a turn. A turn already makes two provider
// calls; a third on every one would raise the cost of play by half for a
// question that only has a new answer occasionally. It also reads better over a
// stretch of play than over one sentence -- a storyline is easier to recognise
// from a session than from a sentence.
//
// Everything the model returns is a *proposal*. This package decides what
// lands: it caps how much can be created at once, refuses to touch a thread
// that does not exist, and drops anything the domain rejects.
package story

import (
	"context"
	"fmt"
	"strings"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
)

// How much one review may change.
//
// A model that opens ten threads a session makes the story block useless, and
// the block is the whole reason any of this exists. Being sparing is worth more
// than being complete: a missed thread can be added by hand, while a log full
// of noise cannot be un-noised.
const (
	MaxNewThreadsPerReview      = 3
	MaxNewConsequencesPerReview = 3

	// ReviewWindow is how many recent events a review reads.
	ReviewWindow = 40
)

// The stores this service needs, named as narrowly as it uses them.
type (
	// Store is the DM's outstanding work.
	Store interface {
		GetLiveThreads(ctx context.Context, campaignID string) ([]*models.PlotThread, error)
		GetPendingConsequences(ctx context.Context, campaignID string) ([]*models.Consequence, error)
		CreateThread(ctx context.Context, thread *models.PlotThread) error
		CreateConsequence(ctx context.Context, c *models.Consequence) error
		SaveThread(ctx context.Context, campaignID string, thread *models.PlotThread) error
	}

	// EventStore is the campaign's log.
	EventStore interface {
		GetRecentEvents(ctx context.Context, campaignID string, limit int) ([]*models.StoryEvent, error)
	}

	// Reviewer is the subset of ai.Service this uses.
	Reviewer interface {
		ReviewStory(ctx context.Context, req *ai.StoryReviewRequest) (*ai.StoryReviewResponse, error)
	}
)

// Service reviews recent play and records what it opened.
type Service struct {
	store    Store
	events   EventStore
	reviewer Reviewer

	Window int
}

// NewService wires a story service.
func NewService(store Store, events EventStore, reviewer Reviewer) *Service {
	return &Service{store: store, events: events, reviewer: reviewer, Window: ReviewWindow}
}

// Result is what a review changed.
type Result struct {
	ThreadsOpened        int `json:"threads_opened"`
	ThreadsAdvanced      int `json:"threads_advanced"`
	ThreadsResolved      int `json:"threads_resolved"`
	ConsequencesRecorded int `json:"consequences_recorded"`

	// Skipped counts proposals that were refused: over the cap, already
	// tracked, naming a thread that does not exist, or rejected by the rules.
	Skipped int `json:"skipped"`

	// Notes explains each refusal, so a review that did nothing can say why
	// rather than looking like a failure.
	Notes []string `json:"notes,omitempty"`

	TokensUsed int     `json:"tokens_used"`
	Cost       float64 `json:"cost_usd"`
}

// Summary is a one-line account of what the review did.
func (r Result) Summary() string {
	return fmt.Sprintf("%s opened, %s advanced, %s resolved, %s recorded, %d skipped",
		plural(r.ThreadsOpened, "thread"), plural(r.ThreadsAdvanced, "thread"),
		plural(r.ThreadsResolved, "thread"), plural(r.ConsequencesRecorded, "consequence"),
		r.Skipped)
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func (r *Result) skip(note string) {
	r.Skipped++
	r.Notes = append(r.Notes, note)
}

// Review reads recent play and records what it opened, advanced and closed.
func (s *Service) Review(ctx context.Context, campaignID string) (*Result, error) {
	result := &Result{}

	recent, err := s.events.GetRecentEvents(ctx, campaignID, s.Window)
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, len(recent))
	for _, e := range recent {
		if line := eventLine(e); line != "" {
			lines = append(lines, line)
		}
	}
	// Nothing to review is not an error, and not a provider call to pay for.
	if len(lines) == 0 {
		return result, nil
	}

	threads, err := s.store.GetLiveThreads(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	pending, err := s.store.GetPendingConsequences(ctx, campaignID)
	if err != nil {
		return nil, err
	}

	review, err := s.reviewer.ReviewStory(ctx, &ai.StoryReviewRequest{
		RecentEvents:        lines,
		OpenThreads:         deref(threads),
		PendingConsequences: derefConsequences(pending),
	})
	if err != nil {
		return nil, err
	}
	result.TokensUsed, result.Cost = review.TokensUsed, review.Cost

	byID := make(map[string]*models.PlotThread, len(threads))
	titles := make(map[string]bool, len(threads))
	for _, t := range threads {
		byID[t.ThreadID] = t
		titles[normaliseTitle(t.Title)] = true
	}

	s.openThreads(ctx, campaignID, review.NewThreads, titles, result)
	s.recordConsequences(ctx, campaignID, review.NewConsequences, result)
	s.applyBeats(ctx, campaignID, review.Advanced, byID, result)
	s.applyResolutions(ctx, campaignID, review.Resolved, byID, result)

	return result, nil
}

func (s *Service) openThreads(
	ctx context.Context,
	campaignID string,
	proposals []ai.ProposedThread,
	existing map[string]bool,
	result *Result,
) {
	for _, p := range proposals {
		if result.ThreadsOpened >= MaxNewThreadsPerReview {
			result.skip(fmt.Sprintf("not opening %q: already opened %d threads this review",
				p.Title, MaxNewThreadsPerReview))
			continue
		}
		// Opening the same thread every session fills the block with
		// duplicates and stops it being worth sending.
		if existing[normaliseTitle(p.Title)] {
			result.skip(fmt.Sprintf("%q is already open", p.Title))
			continue
		}

		thread := &models.PlotThread{
			CampaignID: campaignID, Title: p.Title, Summary: p.Summary,
			Status: models.ThreadOpen, Urgency: p.Urgency, Involves: p.Involves,
		}
		if err := s.store.CreateThread(ctx, thread); err != nil {
			result.skip(fmt.Sprintf("could not open %q: %v", p.Title, err))
			continue
		}
		existing[normaliseTitle(p.Title)] = true
		result.ThreadsOpened++
	}
}

func (s *Service) recordConsequences(
	ctx context.Context,
	campaignID string,
	proposals []ai.ProposedConsequence,
	result *Result,
) {
	for _, p := range proposals {
		if result.ConsequencesRecorded >= MaxNewConsequencesPerReview {
			result.skip(fmt.Sprintf("not recording %q: already recorded %d consequences this review",
				p.Cause, MaxNewConsequencesPerReview))
			continue
		}

		consequence := &models.Consequence{
			CampaignID: campaignID, ThreadID: p.ThreadID,
			Cause: p.Cause, Expected: p.Expected,
			Severity: p.Severity, Status: models.ConsequencePending,
		}
		if err := s.store.CreateConsequence(ctx, consequence); err != nil {
			result.skip(fmt.Sprintf("could not record %q: %v", p.Cause, err))
			continue
		}
		result.ConsequencesRecorded++
	}
}

func (s *Service) applyBeats(
	ctx context.Context,
	campaignID string,
	proposals []ai.ProposedBeat,
	byID map[string]*models.PlotThread,
	result *Result,
) {
	for _, p := range proposals {
		thread, ok := byID[p.ThreadID]
		if !ok {
			// The model may only name a thread from the list it was given.
			result.skip(fmt.Sprintf("no open thread %q to advance", p.ThreadID))
			continue
		}
		if err := thread.Advance(models.ThreadBeat{Summary: p.Summary}); err != nil {
			result.skip(fmt.Sprintf("could not advance %q: %v", thread.Title, err))
			continue
		}
		if err := s.store.SaveThread(ctx, campaignID, thread); err != nil {
			result.skip(fmt.Sprintf("could not save %q: %v", thread.Title, err))
			continue
		}
		result.ThreadsAdvanced++
	}
}

func (s *Service) applyResolutions(
	ctx context.Context,
	campaignID string,
	proposals []ai.ProposedResolution,
	byID map[string]*models.PlotThread,
	result *Result,
) {
	for _, p := range proposals {
		thread, ok := byID[p.ThreadID]
		if !ok {
			result.skip(fmt.Sprintf("no open thread %q to resolve", p.ThreadID))
			continue
		}
		if err := thread.Resolve(p.How, ""); err != nil {
			result.skip(fmt.Sprintf("could not resolve %q: %v", thread.Title, err))
			continue
		}
		if err := s.store.SaveThread(ctx, campaignID, thread); err != nil {
			result.skip(fmt.Sprintf("could not save %q: %v", thread.Title, err))
			continue
		}
		result.ThreadsResolved++
	}
}

// normaliseTitle is how two proposals are judged to be the same thread.
//
// Case and surrounding space only: anything cleverer would start merging
// threads that merely sound alike, which is worse than a duplicate.
func normaliseTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

// eventLine renders one event the way the review reads it.
func eventLine(e *models.StoryEvent) string {
	for _, candidate := range []string{
		e.Narrative.AIGeneratedText,
		e.Narrative.DMInterpretation,
		e.Trigger.PlayerInput,
	} {
		if line := strings.TrimSpace(candidate); line != "" {
			return line
		}
	}
	return ""
}

func deref(threads []*models.PlotThread) []models.PlotThread {
	out := make([]models.PlotThread, 0, len(threads))
	for _, t := range threads {
		if t != nil {
			out = append(out, *t)
		}
	}
	return out
}

func derefConsequences(consequences []*models.Consequence) []models.Consequence {
	out := make([]models.Consequence, 0, len(consequences))
	for _, c := range consequences {
		if c != nil {
			out = append(out, *c)
		}
	}
	return out
}

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// StoryReviewRequest asks what the last stretch of play opened and set in motion.
//
// It is a separate pass rather than part of a turn on purpose. A turn already
// makes two provider calls; adding a third to every one would raise the cost of
// play by half for a question that only has a new answer occasionally. Running
// it at a session boundary is both cheaper and more accurate -- a storyline is
// easier to recognise from a stretch of play than from one sentence.
type StoryReviewRequest struct {
	RecentEvents []string

	// OpenThreads and PendingConsequences are sent so the review does not
	// propose what is already tracked. Without them it opens the same thread
	// every session and the block fills with duplicates.
	OpenThreads         []models.PlotThread
	PendingConsequences []models.Consequence
}

// ProposedThread is a storyline the review thinks was opened.
type ProposedThread struct {
	Title    string               `json:"title"`
	Summary  string               `json:"summary"`
	Urgency  models.ThreadUrgency `json:"urgency"`
	Involves []string             `json:"involves"`
}

// ProposedConsequence is something the review thinks the party set in motion.
type ProposedConsequence struct {
	Cause    string                     `json:"cause"`
	Expected string                     `json:"expected"`
	Severity models.ConsequenceSeverity `json:"severity"`
	ThreadID string                     `json:"thread_id"`
}

// ProposedBeat is a development on a thread that already exists.
type ProposedBeat struct {
	ThreadID string `json:"thread_id"`
	Summary  string `json:"summary"`
}

// ProposedResolution is a thread the review thinks is finished.
type ProposedResolution struct {
	ThreadID string `json:"thread_id"`
	How      string `json:"how"`
}

// StoryReviewResponse is what the review proposes.
//
// Every field is a *proposal*. Nothing here is applied by this package: the
// application layer checks each one against what actually exists and bounds how
// many can land, because a model that opens ten threads a session makes the
// story block useless.
type StoryReviewResponse struct {
	NewThreads      []ProposedThread      `json:"new_threads"`
	NewConsequences []ProposedConsequence `json:"new_consequences"`
	Advanced        []ProposedBeat        `json:"advanced"`
	Resolved        []ProposedResolution  `json:"resolved"`

	TokensUsed     int
	Cost           float64
	ProcessingTime time.Duration
}

// ReviewStory asks what the last stretch of play opened, advanced and closed.
func (s *Service) ReviewStory(ctx context.Context, req *StoryReviewRequest) (*StoryReviewResponse, error) {
	started := time.Now()

	events := make([]string, 0, len(req.RecentEvents))
	for _, e := range req.RecentEvents {
		if line := strings.TrimSpace(e); line != "" {
			events = append(events, "- "+line)
		}
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("no events to review")
	}

	response, err := s.completeTemplate(ctx, "story_review", 800, map[string]string{
		"recent_events":        strings.Join(events, "\n"),
		"open_threads":         renderThreads(req.OpenThreads),
		"pending_consequences": renderConsequences(req.PendingConsequences),
	})
	if err != nil {
		return nil, err
	}

	review, err := ParseStoryReview(response.Text)
	if err != nil {
		return nil, err
	}
	review.TokensUsed = response.TokensUsed
	review.Cost = response.Cost
	review.ProcessingTime = time.Since(started)
	return review, nil
}

// ParseStoryReview reads the review out of a model's reply and normalises it.
//
// An invented urgency or severity is normalised rather than rejected: a
// bookkeeping pass should not fail the whole review because one word was wrong,
// and the closed sets have sensible middles to fall back to.
func ParseStoryReview(content string) (*StoryReviewResponse, error) {
	payload := extractJSONObject(content)
	if payload == "" {
		return nil, models.Invalid("no JSON object found in the model's reply")
	}

	var review StoryReviewResponse
	if err := json.Unmarshal([]byte(payload), &review); err != nil {
		return nil, models.Invalid("the model's reply was not valid JSON: %v", err)
	}

	threads := review.NewThreads[:0]
	for _, t := range review.NewThreads {
		t.Title = strings.TrimSpace(t.Title)
		t.Summary = strings.TrimSpace(t.Summary)
		if t.Title == "" {
			continue // a thread with no title is not a thread
		}
		if !t.Urgency.Valid() || t.Urgency == "" {
			t.Urgency = models.ThreadActive
		}
		threads = append(threads, t)
	}
	review.NewThreads = threads

	consequences := review.NewConsequences[:0]
	for _, c := range review.NewConsequences {
		c.Cause = strings.TrimSpace(c.Cause)
		c.Expected = strings.TrimSpace(c.Expected)
		// Both halves are required: a cause with nothing expected is history,
		// and something expected with no cause is a prediction.
		if c.Cause == "" || c.Expected == "" {
			continue
		}
		if !c.Severity.Valid() || c.Severity == "" {
			c.Severity = models.SeverityModerate
		}
		consequences = append(consequences, c)
	}
	review.NewConsequences = consequences

	beats := review.Advanced[:0]
	for _, b := range review.Advanced {
		b.ThreadID = strings.TrimSpace(b.ThreadID)
		b.Summary = strings.TrimSpace(b.Summary)
		if b.ThreadID != "" && b.Summary != "" {
			beats = append(beats, b)
		}
	}
	review.Advanced = beats

	resolutions := review.Resolved[:0]
	for _, r := range review.Resolved {
		r.ThreadID = strings.TrimSpace(r.ThreadID)
		r.How = strings.TrimSpace(r.How)
		if r.ThreadID != "" && r.How != "" {
			resolutions = append(resolutions, r)
		}
	}
	review.Resolved = resolutions

	return &review, nil
}

func renderThreads(threads []models.PlotThread) string {
	if len(threads) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, t := range threads {
		fmt.Fprintf(&b, "\n- [%s] %s", t.ThreadID, t.Title)
		if summary := strings.TrimSpace(t.Summary); summary != "" {
			fmt.Fprintf(&b, ": %s", summary)
		}
	}
	return strings.TrimPrefix(b.String(), "\n")
}

func renderConsequences(consequences []models.Consequence) string {
	if len(consequences) == 0 {
		return "(none)"
	}
	var b strings.Builder
	for _, c := range consequences {
		fmt.Fprintf(&b, "\n- [%s] %s -> %s", c.ConsequenceID,
			strings.TrimSpace(c.Cause), strings.TrimSpace(c.Expected))
	}
	return strings.TrimPrefix(b.String(), "\n")
}

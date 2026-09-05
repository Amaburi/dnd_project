package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

func reviewEvents() []string {
	return []string{
		"The party arrived in Phandalin and found the town cowed by a gang called the Redbrands.",
		"Thistle let the goblin chief Klarg walk away rather than finish him.",
	}
}

// The DM opens storylines while narrating and then forgets them. This is the
// pass that writes them down.
func TestReviewProposesThreadsAndConsequences(t *testing.T) {
	service, stub := NewStubService(`{
	  "new_threads": [
	    {"title": "The Redbrands hold Phandalin", "summary": "A gang has the town cowed", "urgency": "active", "involves": ["Phandalin"]}
	  ],
	  "new_consequences": [
	    {"cause": "Thistle spared Klarg", "expected": "Klarg comes looking for them", "severity": "moderate"}
	  ]
	}`)

	review, err := service.ReviewStory(context.Background(), &StoryReviewRequest{
		RecentEvents: reviewEvents(),
	})
	if err != nil {
		t.Fatalf("ReviewStory: %v", err)
	}
	if len(review.NewThreads) != 1 {
		t.Fatalf("proposed %d threads, want one", len(review.NewThreads))
	}
	if review.NewThreads[0].Title != "The Redbrands hold Phandalin" {
		t.Errorf("title = %q", review.NewThreads[0].Title)
	}
	if review.NewThreads[0].Urgency != models.ThreadActive {
		t.Errorf("urgency = %q", review.NewThreads[0].Urgency)
	}
	if len(review.NewConsequences) != 1 {
		t.Fatalf("proposed %d consequences, want one", len(review.NewConsequences))
	}
	if review.NewConsequences[0].Severity != models.SeverityModerate {
		t.Errorf("severity = %q", review.NewConsequences[0].Severity)
	}

	// The events have to reach the prompt, or it is proposing from nothing.
	if !strings.Contains(stub.LastPrompt(), "Redbrands") {
		t.Errorf("the events never reached the prompt:\n%s", stub.LastPrompt())
	}
}

// Existing threads are sent so the review does not open the same one every
// session. Without this the block fills with duplicates and stops being useful.
func TestReviewIsToldWhatIsAlreadyOpen(t *testing.T) {
	service, stub := NewStubService(`{"new_threads": []}`)

	if _, err := service.ReviewStory(context.Background(), &StoryReviewRequest{
		RecentEvents: reviewEvents(),
		OpenThreads: []models.PlotThread{{
			ThreadID: "t1", Title: "The Redbrands hold Phandalin", Status: models.ThreadOpen,
		}},
		PendingConsequences: []models.Consequence{{
			ConsequenceID: "c1", Cause: "Thistle spared Klarg", Expected: "he returns",
		}},
	}); err != nil {
		t.Fatalf("ReviewStory: %v", err)
	}

	prompt := stub.LastPrompt()
	for _, want := range []string{"t1", "The Redbrands hold Phandalin", "c1", "spared Klarg"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt is missing %q:\n%s", want, prompt)
		}
	}
}

// An invented urgency or severity must normalise, not break the review.
func TestReviewNormalisesInventedValues(t *testing.T) {
	service, _ := NewStubService(`{
	  "new_threads": [{"title": "Something stirs", "summary": "x", "urgency": "extremely spicy"}],
	  "new_consequences": [{"cause": "a", "expected": "b", "severity": "cataclysmic"}]
	}`)

	review, err := service.ReviewStory(context.Background(), &StoryReviewRequest{RecentEvents: reviewEvents()})
	if err != nil {
		t.Fatalf("ReviewStory: %v", err)
	}
	if review.NewThreads[0].Urgency != models.ThreadActive {
		t.Errorf("urgency = %q, want it normalised to active", review.NewThreads[0].Urgency)
	}
	if review.NewConsequences[0].Severity != models.SeverityModerate {
		t.Errorf("severity = %q, want it normalised to moderate", review.NewConsequences[0].Severity)
	}
}

// A proposal with nothing in it is dropped rather than stored as an empty row.
func TestReviewDropsEmptyProposals(t *testing.T) {
	service, _ := NewStubService(`{
	  "new_threads": [{"title": "   ", "summary": "x"}, {"title": "Real one", "summary": "y"}],
	  "new_consequences": [{"cause": "", "expected": "something"}]
	}`)

	review, err := service.ReviewStory(context.Background(), &StoryReviewRequest{RecentEvents: reviewEvents()})
	if err != nil {
		t.Fatalf("ReviewStory: %v", err)
	}
	if len(review.NewThreads) != 1 || review.NewThreads[0].Title != "Real one" {
		t.Errorf("threads = %+v, want only the titled one", review.NewThreads)
	}
	if len(review.NewConsequences) != 0 {
		t.Errorf("consequences = %+v, want the causeless one dropped", review.NewConsequences)
	}
}

// Reviewing nothing is a caller error, not a provider call to pay for.
func TestReviewingNoEventsIsRefused(t *testing.T) {
	service, stub := NewStubService("unused")
	if _, err := service.ReviewStory(context.Background(), &StoryReviewRequest{}); err == nil {
		t.Error("reviewing no events should be an error")
	}
	if len(stub.Requests) != 0 {
		t.Errorf("the provider was called %d times for nothing to review", len(stub.Requests))
	}
}

// The review reads history; it must not write fiction. That is the contract
// that keeps a bookkeeping pass from becoming a second narrator.
func TestReviewPromptForbidsInvention(t *testing.T) {
	service, stub := NewStubService(`{"new_threads": []}`)
	if _, err := service.ReviewStory(context.Background(), &StoryReviewRequest{RecentEvents: reviewEvents()}); err != nil {
		t.Fatalf("ReviewStory: %v", err)
	}

	system := strings.ToLower(stub.Requests[0].Messages[0].Content)
	for _, want := range []string{"never invent", "do not continue the story"} {
		if !strings.Contains(system, want) {
			t.Errorf("the system prompt does not say %q:\n%s", want, system)
		}
	}
}

// Bookkeeping runs cool: this is not a creative task.
func TestReviewTemperatureIsLow(t *testing.T) {
	if got := GetTemperature("story_review"); got > 0.3 {
		t.Errorf("story_review temperature = %v, want a low value", got)
	}
}

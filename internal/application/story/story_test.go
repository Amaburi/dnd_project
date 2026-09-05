package story

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
	"github.com/dnd-campaign/manager/internal/infrastructure/ai"
)

type fakeStore struct {
	threads      []*models.PlotThread
	consequences []*models.Consequence

	createdThreads      []*models.PlotThread
	createdConsequences []*models.Consequence
	savedThreads        []*models.PlotThread
}

func (f *fakeStore) GetLiveThreads(context.Context, string) ([]*models.PlotThread, error) {
	return f.threads, nil
}

func (f *fakeStore) GetPendingConsequences(context.Context, string) ([]*models.Consequence, error) {
	return f.consequences, nil
}

func (f *fakeStore) CreateThread(_ context.Context, t *models.PlotThread) error {
	f.createdThreads = append(f.createdThreads, t)
	return nil
}

func (f *fakeStore) CreateConsequence(_ context.Context, c *models.Consequence) error {
	f.createdConsequences = append(f.createdConsequences, c)
	return nil
}

func (f *fakeStore) SaveThread(_ context.Context, _ string, t *models.PlotThread) error {
	f.savedThreads = append(f.savedThreads, t)
	return nil
}

type fakeEvents struct{ all []*models.StoryEvent }

func (f *fakeEvents) GetRecentEvents(context.Context, string, int) ([]*models.StoryEvent, error) {
	return f.all, nil
}

func events(lines ...string) *fakeEvents {
	out := &fakeEvents{}
	for i, line := range lines {
		out.all = append(out.all, &models.StoryEvent{
			SequenceNumber: i + 1,
			Narrative:      models.NarrativeInfo{AIGeneratedText: line},
			Timestamp:      time.Now().UTC(),
		})
	}
	return out
}

type harness struct {
	service *Service
	store   *fakeStore
	stub    *ai.StubClient
}

func newHarness(t *testing.T, reply string, lines ...string) *harness {
	t.Helper()
	if len(lines) == 0 {
		lines = []string{"The party arrived in Phandalin."}
	}
	reviewer, stub := ai.NewStubService(reply)
	store := &fakeStore{}
	return &harness{
		service: NewService(store, events(lines...), reviewer),
		store:   store, stub: stub,
	}
}

// The point of the pass: a storyline the DM opened while narrating gets
// written down instead of forgotten.
func TestReviewOpensProposedThreads(t *testing.T) {
	h := newHarness(t, `{
	  "new_threads": [{"title": "The Redbrands hold Phandalin", "summary": "A gang has the town cowed", "urgency": "pressing"}],
	  "new_consequences": [{"cause": "Thistle spared Klarg", "expected": "he comes looking", "severity": "major"}]
	}`)

	result, err := h.service.Review(context.Background(), "camp1")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(h.store.createdThreads) != 1 {
		t.Fatalf("created %d threads, want one", len(h.store.createdThreads))
	}
	created := h.store.createdThreads[0]
	if created.CampaignID != "camp1" {
		t.Errorf("campaign = %q", created.CampaignID)
	}
	if created.Urgency != models.ThreadPressing {
		t.Errorf("urgency = %q", created.Urgency)
	}
	if len(h.store.createdConsequences) != 1 {
		t.Fatalf("created %d consequences, want one", len(h.store.createdConsequences))
	}
	if h.store.createdConsequences[0].Status != models.ConsequencePending {
		t.Errorf("a new consequence is %q, want pending", h.store.createdConsequences[0].Status)
	}
	if result.ThreadsOpened != 1 || result.ConsequencesRecorded != 1 {
		t.Errorf("result = %+v", result)
	}
}

// A model that opens ten threads a session makes the story block useless, so
// the number that can land at once is bounded.
func TestTooManyProposalsAreCapped(t *testing.T) {
	var proposals []string
	for i := 0; i < 12; i++ {
		proposals = append(proposals, `{"title": "Thread `+string(rune('A'+i))+`", "summary": "x"}`)
	}
	h := newHarness(t, `{"new_threads": [`+strings.Join(proposals, ",")+`]}`)

	result, err := h.service.Review(context.Background(), "camp1")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(h.store.createdThreads) > MaxNewThreadsPerReview {
		t.Errorf("created %d threads, want at most %d",
			len(h.store.createdThreads), MaxNewThreadsPerReview)
	}
	if result.Skipped == 0 {
		t.Error("the capped proposals were not reported as skipped")
	}
}

// The same thread must not be opened every session. Without this the block
// fills with duplicates and stops being worth sending.
func TestAnAlreadyOpenThreadIsNotOpenedAgain(t *testing.T) {
	h := newHarness(t, `{"new_threads": [{"title": "the redbrands HOLD phandalin", "summary": "again"}]}`)
	h.store.threads = []*models.PlotThread{{
		ThreadID: "t1", CampaignID: "camp1",
		Title: "The Redbrands hold Phandalin", Status: models.ThreadOpen,
	}}

	if _, err := h.service.Review(context.Background(), "camp1"); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if len(h.store.createdThreads) != 0 {
		t.Errorf("a duplicate thread was opened: %+v", h.store.createdThreads[0])
	}
}

// Advancing and resolving may only touch threads that exist and are live.
func TestAdvancingAndResolvingOnlyTouchRealThreads(t *testing.T) {
	h := newHarness(t, `{
	  "advanced": [
	    {"thread_id": "t1", "summary": "The party found the hideout"},
	    {"thread_id": "does-not-exist", "summary": "something"}
	  ],
	  "resolved": [{"thread_id": "t1", "how": "Glasstaff was captured"}]
	}`)
	h.store.threads = []*models.PlotThread{{
		ThreadID: "t1", CampaignID: "camp1", Title: "The Redbrands", Status: models.ThreadOpen,
	}}

	result, err := h.service.Review(context.Background(), "camp1")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if result.ThreadsAdvanced != 1 {
		t.Errorf("advanced %d threads, want one: the invented id must be ignored", result.ThreadsAdvanced)
	}
	if result.ThreadsResolved != 1 {
		t.Errorf("resolved %d threads, want one", result.ThreadsResolved)
	}

	saved := h.store.threads[0]
	if len(saved.Beats) != 1 {
		t.Errorf("the beat was not recorded: %+v", saved.Beats)
	}
	if saved.Status != models.ThreadResolved {
		t.Errorf("status = %q, want resolved", saved.Status)
	}
}

// Nothing to review is not an error and costs no provider call.
func TestReviewWithNoEventsDoesNothing(t *testing.T) {
	h := newHarness(t, "unused")
	h.service.events = &fakeEvents{}

	result, err := h.service.Review(context.Background(), "camp1")
	if err != nil {
		t.Fatalf("an empty log should not be an error: %v", err)
	}
	if result.ThreadsOpened != 0 {
		t.Errorf("result = %+v", result)
	}
	if len(h.stub.Requests) != 0 {
		t.Errorf("the provider was called %d times for an empty log", len(h.stub.Requests))
	}
}

func TestReviewReportsWhatItDid(t *testing.T) {
	h := newHarness(t, `{
	  "new_threads": [{"title": "A thread", "summary": "x"}],
	  "new_consequences": [{"cause": "a", "expected": "b"}]
	}`)

	result, err := h.service.Review(context.Background(), "camp1")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if result.TokensUsed == 0 {
		t.Error("the review reported no token usage")
	}
	summary := result.Summary()
	for _, want := range []string{"1 thread", "1 consequence"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q is missing %q", summary, want)
		}
	}
}

package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

type fakeEvents struct {
	all []*models.StoryEvent

	// sinceCalls records the watermarks Build asked for, so a test can prove
	// the summary is actually used to skip work rather than being decorative.
	sinceCalls []time.Time
}

func (f *fakeEvents) GetRecentEvents(_ context.Context, _ string, limit int) ([]*models.StoryEvent, error) {
	if limit > 0 && len(f.all) > limit {
		return f.all[len(f.all)-limit:], nil
	}
	return f.all, nil
}

func (f *fakeEvents) GetEventsSince(_ context.Context, _ string, since time.Time, limit int) ([]*models.StoryEvent, error) {
	f.sinceCalls = append(f.sinceCalls, since)
	var out []*models.StoryEvent
	for _, e := range f.all {
		if e.Timestamp.After(since) {
			out = append(out, e)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

type fakeCampaigns struct {
	campaign *models.Campaign
	saved    []models.CampaignSummary
	saveErr  error
}

func (f *fakeCampaigns) GetCampaignByCampaignID(_ context.Context, _ string) (*models.Campaign, error) {
	return f.campaign, nil
}

func (f *fakeCampaigns) UpdateSummary(_ context.Context, _ string, s models.CampaignSummary) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, s)
	f.campaign.Summary = s
	return nil
}

type fakeSummarizer struct {
	reply    string
	err      error
	previous []string
	batches  [][]string
}

func (f *fakeSummarizer) SummarizeHistory(_ context.Context, previous string, lines []string) (string, error) {
	f.previous = append(f.previous, previous)
	f.batches = append(f.batches, lines)
	if f.err != nil {
		return "", f.err
	}
	return f.reply, nil
}

func timedEvents(n int) []*models.StoryEvent {
	base := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	out := make([]*models.StoryEvent, n)
	for i := range out {
		out[i] = event(i+1, "the party pressed on")
		out[i].Timestamp = base.Add(time.Duration(i) * time.Minute)
	}
	return out
}

func campaign() *models.Campaign {
	return &models.Campaign{CampaignID: "camp-1", Title: "Phandelver"}
}

// With no campaign store there is no summary to read, and the service still
// has to work: that is the configuration the turn service boots with.
func TestBuildWithoutACampaignStoreUsesRecentEvents(t *testing.T) {
	events := &fakeEvents{all: timedEvents(5)}
	s := New(events, nil, nil)

	c, err := s.Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(c.Recent) != 5 {
		t.Errorf("kept %d events, want 5", len(c.Recent))
	}
	if len(events.sinceCalls) != 0 {
		t.Error("asked for events since a watermark with no summary to provide one")
	}
}

// The watermark is the point of the summary: everything it already covers is
// never read again.
func TestBuildReadsOnlyEventsAfterTheSummary(t *testing.T) {
	all := timedEvents(10)
	camp := campaign()
	camp.Summary = models.CampaignSummary{
		Text:    "The party cleared the goblin cave.",
		Through: all[5].Timestamp,
	}

	events := &fakeEvents{all: all}
	s := New(events, &fakeCampaigns{campaign: camp}, nil)

	c, err := s.Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(events.sinceCalls) != 1 || !events.sinceCalls[0].Equal(all[5].Timestamp) {
		t.Fatalf("watermark = %v, want %v", events.sinceCalls, all[5].Timestamp)
	}
	if len(c.Recent) != 4 {
		t.Errorf("kept %d events, want the 4 after the watermark", len(c.Recent))
	}
	if !strings.Contains(c.Block(), "goblin cave") {
		t.Error("the stored summary did not reach the block")
	}
}

// The budget configured on the service is the one that applies.
func TestBuildAppliesTheConfiguredBudget(t *testing.T) {
	s := New(&fakeEvents{all: timedEvents(40)}, nil, nil)
	s.Budget = Budget{MaxTokens: 50, MinRecent: 2}

	c, err := s.Build(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if c.Tokens > 50 {
		t.Errorf("built %d tokens against a budget of 50", c.Tokens)
	}
	if !c.Truncated {
		t.Error("40 events under a 50-token budget should have truncated")
	}
}

// Compaction is what stops the log growing past any budget. Below the
// threshold it must do nothing at all -- an AI call per turn to summarise five
// events would cost more than the events themselves.
func TestCompactDoesNothingBelowTheThreshold(t *testing.T) {
	summarizer := &fakeSummarizer{reply: "should not be called"}
	campaigns := &fakeCampaigns{campaign: campaign()}
	s := New(&fakeEvents{all: timedEvents(5)}, campaigns, summarizer)
	s.CompactAfter = 20

	compacted, err := s.Compact(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if compacted {
		t.Error("Compact reported work with only 5 events")
	}
	if len(summarizer.batches) != 0 {
		t.Errorf("the provider was called %d times for nothing", len(summarizer.batches))
	}
	if len(campaigns.saved) != 0 {
		t.Error("a summary was written when none was generated")
	}
}

// Above the threshold the older events are folded into the summary and the
// watermark advances past exactly those events.
func TestCompactFoldsOlderEventsIntoTheSummary(t *testing.T) {
	all := timedEvents(30)
	summarizer := &fakeSummarizer{reply: "The party crossed the moor and lost a horse."}
	campaigns := &fakeCampaigns{campaign: campaign()}
	s := New(&fakeEvents{all: all}, campaigns, summarizer)
	s.CompactAfter = 20
	s.Retain = 10

	compacted, err := s.Compact(context.Background(), "camp-1")
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if !compacted {
		t.Fatal("30 events past a threshold of 20 should have compacted")
	}

	if len(summarizer.batches) != 1 {
		t.Fatalf("the provider was called %d times, want once", len(summarizer.batches))
	}
	// 30 events, retain the newest 10, so the oldest 20 are folded in.
	if got := len(summarizer.batches[0]); got != 20 {
		t.Errorf("summarised %d events, want 20", got)
	}

	if len(campaigns.saved) != 1 {
		t.Fatalf("wrote %d summaries, want one", len(campaigns.saved))
	}
	saved := campaigns.saved[0]
	if saved.Text != summarizer.reply {
		t.Errorf("stored %q, want the model's reply", saved.Text)
	}
	// The watermark is the last event folded in -- the 20th, not the 30th, or
	// the ten retained events would be forgotten without ever being summarised.
	if !saved.Through.Equal(all[19].Timestamp) {
		t.Errorf("watermark = %v, want %v", saved.Through, all[19].Timestamp)
	}
	if saved.EventCount != 20 {
		t.Errorf("EventCount = %d, want 20", saved.EventCount)
	}
	if saved.UpdatedAt.IsZero() || saved.UpdatedAt.Location() != time.UTC {
		t.Errorf("UpdatedAt = %v, want a UTC timestamp", saved.UpdatedAt)
	}
}

// Compacting twice must build on the first summary, not replace it, or the
// campaign forgets its first act the moment it has a second.
func TestCompactCarriesThePreviousSummaryForward(t *testing.T) {
	camp := campaign()
	camp.Summary = models.CampaignSummary{Text: "Act one: the goblin cave.", Through: time.Time{}}

	summarizer := &fakeSummarizer{reply: "Acts one and two."}
	s := New(&fakeEvents{all: timedEvents(30)}, &fakeCampaigns{campaign: camp}, summarizer)
	s.CompactAfter = 20
	s.Retain = 10

	if _, err := s.Compact(context.Background(), "camp-1"); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if len(summarizer.previous) != 1 || summarizer.previous[0] != "Act one: the goblin cave." {
		t.Errorf("previous summary passed as %q, want the stored one", summarizer.previous)
	}
}

// A provider failure must not lose history: the old summary and watermark stay
// exactly as they were, so the next attempt sees the same work to do.
func TestCompactLeavesTheSummaryAloneWhenTheProviderFails(t *testing.T) {
	camp := campaign()
	camp.Summary = models.CampaignSummary{Text: "Act one.", Through: time.Time{}}
	campaigns := &fakeCampaigns{campaign: camp}

	s := New(&fakeEvents{all: timedEvents(30)}, campaigns,
		&fakeSummarizer{err: errors.New("provider is down")})
	s.CompactAfter = 20
	s.Retain = 10

	compacted, err := s.Compact(context.Background(), "camp-1")
	if err == nil {
		t.Fatal("a provider failure should be reported")
	}
	if compacted {
		t.Error("Compact claimed work it did not do")
	}
	if len(campaigns.saved) != 0 {
		t.Error("a summary was written despite the failure")
	}
	if camp.Summary.Text != "Act one." {
		t.Errorf("the previous summary was damaged: %q", camp.Summary.Text)
	}
}

// An empty reply is a failure, not a summary. Storing it would advance the
// watermark past events nothing remembers.
func TestCompactRejectsAnEmptySummary(t *testing.T) {
	campaigns := &fakeCampaigns{campaign: campaign()}
	s := New(&fakeEvents{all: timedEvents(30)}, campaigns, &fakeSummarizer{reply: "   "})
	s.CompactAfter = 20
	s.Retain = 10

	if _, err := s.Compact(context.Background(), "camp-1"); err == nil {
		t.Error("an empty summary should be refused")
	}
	if len(campaigns.saved) != 0 {
		t.Error("an empty summary was stored")
	}
}

// Without a summarizer there is nothing to compact with, and asking is a
// programming error rather than a runtime one -- it must not panic.
func TestCompactWithoutTheCollaboratorsIsANoOp(t *testing.T) {
	s := New(&fakeEvents{all: timedEvents(50)}, nil, nil)
	compacted, err := s.Compact(context.Background(), "camp-1")
	if err != nil || compacted {
		t.Errorf("Compact = (%v, %v), want (false, nil)", compacted, err)
	}
}

package models

import (
	"strings"
	"testing"
)

func arc() *StoryArc {
	return &StoryArc{
		ArcID: "a1", CampaignID: "camp1", Order: 1,
		Title:   "Phandalin",
		Premise: "The party reaches the town and finds it under the Redbrands' thumb.",
		Status:  ArcActive, Stage: ArcSetup,
		ThreadIDs: []string{"t1", "t2"},
	}
}

func resolvedThread(id string) *PlotThread {
	return &PlotThread{ThreadID: id, CampaignID: "camp1", Title: id, Status: ThreadResolved}
}

func openThread(id string) *PlotThread {
	return &PlotThread{ThreadID: id, CampaignID: "camp1", Title: id, Status: ThreadOpen}
}

// An arc has a shape: it builds, it peaks, it lands. The DM needs to know which
// of those it is in or every scene is pitched the same way.
func TestAnArcAdvancesThroughItsStages(t *testing.T) {
	a := arc()
	want := []ArcStage{ArcRising, ArcClimax, ArcResolution}

	for _, stage := range want {
		if err := a.AdvanceStage(); err != nil {
			t.Fatalf("AdvanceStage: %v", err)
		}
		if a.Stage != stage {
			t.Fatalf("stage = %q, want %q", a.Stage, stage)
		}
	}

	// There is nothing after resolution.
	if err := a.AdvanceStage(); err == nil {
		t.Error("an arc advanced past its resolution")
	}
}

// The rule worth having: an arc cannot be finished while the storylines it is
// made of are still open. Otherwise the DM believes it is done while the prompt
// still lists its threads as outstanding.
func TestAnArcCannotCompleteWhileItsThreadsAreLive(t *testing.T) {
	a := arc()
	threads := []*PlotThread{resolvedThread("t1"), openThread("t2")}

	ok, reason := a.CanComplete(threads)
	if ok {
		t.Fatal("an arc completed with an open thread")
	}
	if !strings.Contains(reason, "t2") {
		t.Errorf("the reason does not name the outstanding thread: %q", reason)
	}

	if err := a.Complete("the Redbrands were broken", ""); err == nil {
		t.Error("Complete ignored the open thread")
	}
	if a.Status == ArcCompleted {
		t.Error("the arc was marked complete anyway")
	}
}

func TestAnArcCompletesOnceItsThreadsAreSettled(t *testing.T) {
	a := arc()
	// Abandoned counts as settled: the party walked away, which is an answer.
	abandoned := openThread("t2")
	_ = abandoned.Abandon("the party left it alone")
	threads := []*PlotThread{resolvedThread("t1"), abandoned}

	if ok, reason := a.CanComplete(threads); !ok {
		t.Fatalf("settled threads still blocked completion: %q", reason)
	}
	if err := a.CompleteWith("the Redbrands were broken", "s5", threads); err != nil {
		t.Fatalf("CompleteWith: %v", err)
	}
	if a.Status != ArcCompleted {
		t.Errorf("status = %q", a.Status)
	}
	if a.CompletedAt.IsZero() {
		t.Error("no completion timestamp")
	}
	if !strings.Contains(a.Resolution, "Redbrands") {
		t.Errorf("resolution = %q", a.Resolution)
	}

	// Completing twice would overwrite the ending with a second one.
	if err := a.CompleteWith("something else", "s6", threads); err == nil {
		t.Error("an arc completed twice")
	}
}

// An arc with no threads is a heading, not a storyline, and completing one is
// the DM's call rather than something to derive.
func TestAnArcWithNoThreadsCanBeCompletedDirectly(t *testing.T) {
	a := arc()
	a.ThreadIDs = nil

	if ok, _ := a.CanComplete(nil); !ok {
		t.Error("an arc with no threads was blocked from completing")
	}
	if err := a.Complete("it simply ended", "s2"); err != nil {
		t.Errorf("Complete: %v", err)
	}
}

// Progress is what the DM needs to pace: two of five done is a different scene
// from four of five.
func TestArcProgressCountsSettledThreads(t *testing.T) {
	a := arc()
	a.ThreadIDs = []string{"t1", "t2", "t3"}
	threads := []*PlotThread{resolvedThread("t1"), openThread("t2"), openThread("t3")}

	settled, total := a.Progress(threads)
	if settled != 1 || total != 3 {
		t.Errorf("progress = %d/%d, want 1/3", settled, total)
	}

	// A thread the arc names but the campaign does not have still counts
	// towards the total: silently shrinking the denominator would report an
	// arc as nearly done because a thread went missing.
	missing := a.WithThreads([]string{"t1", "gone"})
	settled, total = missing.Progress([]*PlotThread{resolvedThread("t1")})
	if settled != 1 || total != 2 {
		t.Errorf("progress = %d/%d, want 1/2 with the missing thread counted", settled, total)
	}
}

// Abandoning an arc is not completing it: the difference is whether the story
// landed.
func TestAnArcCanBeAbandoned(t *testing.T) {
	a := arc()
	if err := a.Abandon("the party sailed away and never came back"); err != nil {
		t.Fatalf("Abandon: %v", err)
	}
	if a.Status != ArcAbandoned {
		t.Errorf("status = %q", a.Status)
	}
	if a.IsLive() {
		t.Error("an abandoned arc is still live")
	}

	// A completed arc cannot then be abandoned: it already ended.
	done := arc()
	done.ThreadIDs = nil
	_ = done.Complete("it ended", "s1")
	if err := done.Abandon("actually no"); err == nil {
		t.Error("a completed arc was abandoned")
	}
}

func TestArcValidation(t *testing.T) {
	if err := arc().Validate(); err != nil {
		t.Fatalf("a well-formed arc was rejected: %v", err)
	}
	cases := map[string]func(*StoryArc){
		"no title":              func(a *StoryArc) { a.Title = "" },
		"no campaign":           func(a *StoryArc) { a.CampaignID = "" },
		"an invented status":    func(a *StoryArc) { a.Status = "brewing" },
		"an invented stage":     func(a *StoryArc) { a.Stage = "denouement-ish" },
		"a negative order":      func(a *StoryArc) { a.Order = -1 },
		"completed with no end": func(a *StoryArc) { a.Status = ArcCompleted; a.Resolution = "" },
	}
	for name, mutate := range cases {
		a := arc()
		mutate(a)
		if err := a.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// The block is what the DM reads to know where in the campaign it stands.
func TestArcBlockDescribesWhereTheCampaignIs(t *testing.T) {
	a := arc()
	a.Stage = ArcClimax
	threads := []*PlotThread{resolvedThread("t1"), openThread("t2")}

	block := a.Block(threads)
	for _, want := range []string{"Phandalin", string(ArcClimax), "1 of 2"} {
		if !strings.Contains(block, want) {
			t.Errorf("the block is missing %q:\n%s", want, block)
		}
	}

	// A finished arc is history and does not belong in the DM's current view.
	if got := ArcBlock(nil, nil); !strings.Contains(strings.ToLower(got), "no arc") {
		t.Errorf("with no active arc the block says %q", got)
	}
}

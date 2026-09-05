package models

import (
	"strings"
	"testing"
)

func thread() *PlotThread {
	return &PlotThread{
		ThreadID: "t1", CampaignID: "camp1",
		Title:   "The Redbrands hold Phandalin",
		Summary: "A gang of thugs has the town cowed and nobody will say why.",
		Status:  ThreadOpen, Urgency: ThreadActive,
	}
}

// A thread that cannot change state is a note, not a tracked storyline.
func TestAThreadAdvancesThroughBeats(t *testing.T) {
	th := thread()
	if !th.IsLive() {
		t.Fatal("a new open thread is not live")
	}

	if err := th.Advance(ThreadBeat{Summary: "The party found the Redbrand hideout"}); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if len(th.Beats) != 1 {
		t.Fatalf("recorded %d beats, want one", len(th.Beats))
	}
	if th.Beats[0].At.IsZero() {
		t.Error("the beat has no timestamp")
	}
	if th.UpdatedAt.IsZero() {
		t.Error("advancing did not touch UpdatedAt")
	}

	// A beat with nothing in it is a mistake worth catching.
	if err := th.Advance(ThreadBeat{Summary: "   "}); err == nil {
		t.Error("an empty beat was recorded")
	}
}

// Resolving is final and keeps its history: a resolved thread is still the
// record of what happened, not a deleted row.
func TestResolvingAThreadKeepsItsHistory(t *testing.T) {
	th := thread()
	_ = th.Advance(ThreadBeat{Summary: "The party found the hideout"})

	if err := th.Resolve("Glasstaff was captured and the gang scattered", "s3"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if th.Status != ThreadResolved {
		t.Errorf("status = %q, want resolved", th.Status)
	}
	if th.IsLive() {
		t.Error("a resolved thread is still live")
	}
	if th.ResolvedAt.IsZero() || th.ResolvedSession != "s3" {
		t.Errorf("resolution was not recorded: %v / %q", th.ResolvedAt, th.ResolvedSession)
	}
	if len(th.Beats) != 1 {
		t.Error("resolving discarded the thread's history")
	}

	// Resolving twice is a mistake: the second answer would overwrite the first.
	if err := th.Resolve("something else entirely", "s4"); err == nil {
		t.Error("a resolved thread was resolved again")
	}
	// And a resolved thread cannot quietly gain new beats.
	if err := th.Advance(ThreadBeat{Summary: "more happens"}); err == nil {
		t.Error("a resolved thread accepted a new beat")
	}
}

// A thread the party has walked away from is not the same as one they finished,
// and both differ from one merely gone quiet.
func TestThreadsCanGoDormantOrBeAbandoned(t *testing.T) {
	dormant := thread()
	if err := dormant.SetDormant(); err != nil {
		t.Fatalf("SetDormant: %v", err)
	}
	if dormant.Status != ThreadDormant {
		t.Errorf("status = %q", dormant.Status)
	}
	// Dormant is still live: it can come back.
	if !dormant.IsLive() {
		t.Error("a dormant thread should still be live")
	}
	if err := dormant.Reopen(); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if dormant.Status != ThreadOpen {
		t.Errorf("status after reopening = %q", dormant.Status)
	}

	abandoned := thread()
	if err := abandoned.Abandon("the party left the region for good"); err != nil {
		t.Fatalf("Abandon: %v", err)
	}
	if abandoned.IsLive() {
		t.Error("an abandoned thread is still live")
	}
	if !strings.Contains(abandoned.Resolution, "left the region") {
		t.Errorf("resolution = %q", abandoned.Resolution)
	}
	// An abandoned thread can be picked up again -- players change their minds.
	if err := abandoned.Reopen(); err != nil {
		t.Errorf("an abandoned thread could not be reopened: %v", err)
	}
	// A resolved one cannot: it is finished.
	resolved := thread()
	_ = resolved.Resolve("done", "s1")
	if err := resolved.Reopen(); err == nil {
		t.Error("a resolved thread was reopened")
	}
}

// Beats are bounded, because everything that reaches a prompt is.
func TestThreadBeatsArePruned(t *testing.T) {
	th := thread()
	for i := 0; i < MaxThreadBeats*3; i++ {
		_ = th.Advance(ThreadBeat{Summary: "another development"})
	}
	if len(th.Beats) > MaxThreadBeats {
		t.Errorf("holding %d beats, want at most %d", len(th.Beats), MaxThreadBeats)
	}
	// The ones kept are the most recent: a thread's current state is what
	// matters, unlike an NPC's grudge.
	_ = th.Advance(ThreadBeat{Summary: "the newest development"})
	if th.Beats[len(th.Beats)-1].Summary != "the newest development" {
		t.Error("the newest beat was not kept")
	}
}

func TestThreadValidation(t *testing.T) {
	if err := thread().Validate(); err != nil {
		t.Fatalf("a well-formed thread was rejected: %v", err)
	}
	cases := map[string]func(*PlotThread){
		"no title":            func(p *PlotThread) { p.Title = "" },
		"no campaign":         func(p *PlotThread) { p.CampaignID = "" },
		"an invented status":  func(p *PlotThread) { p.Status = "simmering" },
		"an invented urgency": func(p *PlotThread) { p.Urgency = "spicy" },
		"a beat with no text": func(p *PlotThread) { p.Beats = []ThreadBeat{{}} },
	}
	for name, mutate := range cases {
		th := thread()
		mutate(th)
		if err := th.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func consequence() *Consequence {
	return &Consequence{
		ConsequenceID: "c1", CampaignID: "camp1",
		Cause:    "The party spared Klarg and let him walk",
		Expected: "Klarg gathers what is left of his warband and comes looking",
		Severity: SeverityModerate, Status: ConsequencePending,
	}
}

// A consequence exists precisely because it has not happened yet.
func TestAConsequenceLandsOnce(t *testing.T) {
	c := consequence()
	if !c.IsPending() {
		t.Fatal("a new consequence is not pending")
	}

	if err := c.Realise("Klarg ambushed them on the Triboar Trail", "s4"); err != nil {
		t.Fatalf("Realise: %v", err)
	}
	if c.Status != ConsequenceRealised {
		t.Errorf("status = %q", c.Status)
	}
	if c.IsPending() {
		t.Error("a realised consequence is still pending")
	}
	if c.ResolvedAt.IsZero() {
		t.Error("no resolution timestamp")
	}
	if !strings.Contains(c.Outcome, "Triboar Trail") {
		t.Errorf("outcome = %q", c.Outcome)
	}

	// It cannot land twice: the second telling would overwrite the first.
	if err := c.Realise("something else", "s5"); err == nil {
		t.Error("a consequence landed twice")
	}
}

func TestAConsequenceCanBeAvertedOrExpire(t *testing.T) {
	averted := consequence()
	if err := averted.Avert("the party hunted Klarg down first", "s3"); err != nil {
		t.Fatalf("Avert: %v", err)
	}
	if averted.Status != ConsequenceAverted || averted.IsPending() {
		t.Errorf("status = %q, pending = %v", averted.Status, averted.IsPending())
	}

	expired := consequence()
	if err := expired.Expire("the campaign moved on and it stopped mattering"); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if expired.Status != ConsequenceExpired || expired.IsPending() {
		t.Errorf("status = %q, pending = %v", expired.Status, expired.IsPending())
	}
}

func TestConsequenceValidation(t *testing.T) {
	if err := consequence().Validate(); err != nil {
		t.Fatalf("a well-formed consequence was rejected: %v", err)
	}
	cases := map[string]func(*Consequence){
		"no cause":              func(c *Consequence) { c.Cause = "" },
		"nothing expected":      func(c *Consequence) { c.Expected = "" },
		"no campaign":           func(c *Consequence) { c.CampaignID = "" },
		"an invented severity":  func(c *Consequence) { c.Severity = "catastrophic-ish" },
		"an invented status":    func(c *Consequence) { c.Status = "brewing" },
		"resolved with no note": func(c *Consequence) { c.Status = ConsequenceRealised; c.Outcome = "" },
	}
	for name, mutate := range cases {
		con := consequence()
		mutate(con)
		if err := con.Validate(); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// The block is what the DM actually sees. Threads it has opened and debts it
// has not paid are exactly what an AI DM forgets.
func TestStoryBlockListsWhatIsOutstanding(t *testing.T) {
	live := thread()
	_ = live.Advance(ThreadBeat{Summary: "The party found the hideout"})

	done := thread()
	done.Title = "The goblin ambush"
	_ = done.Resolve("cleared out", "s1")

	pending := consequence()
	landed := consequence()
	landed.Cause = "they burned the ledger"
	_ = landed.Realise("the guild came asking", "s2")

	block := StoryBlock([]*PlotThread{live, done}, []*Consequence{pending, landed})

	if !strings.Contains(block, "Redbrands") {
		t.Errorf("an open thread is missing:\n%s", block)
	}
	if !strings.Contains(block, "found the hideout") {
		t.Errorf("the thread's latest beat is missing:\n%s", block)
	}
	if !strings.Contains(block, "spared Klarg") {
		t.Errorf("a pending consequence is missing:\n%s", block)
	}

	// Finished business is not the DM's outstanding work and must not crowd
	// the budget.
	if strings.Contains(block, "goblin ambush") {
		t.Errorf("a resolved thread is still being sent:\n%s", block)
	}
	if strings.Contains(block, "burned the ledger") {
		t.Errorf("a landed consequence is still being sent:\n%s", block)
	}
}

// Nothing outstanding must read as a sentence, never as an empty heading: a
// blank field is an invitation to invent a plot.
func TestAnEmptyStoryBlockReadsAsASentence(t *testing.T) {
	block := StoryBlock(nil, nil)
	if strings.TrimSpace(block) == "" {
		t.Fatal("an empty story block is empty")
	}
	if !strings.Contains(strings.ToLower(block), "no open") {
		t.Errorf("block = %q, want it to say there is nothing outstanding", block)
	}
}

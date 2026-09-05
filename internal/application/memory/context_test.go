package memory

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// event builds a story event whose narration is a known length, so a budget
// assertion can be about a specific number rather than a range.
func event(sequence int, text string) *models.StoryEvent {
	return &models.StoryEvent{
		EventID:        fmt.Sprintf("e%d", sequence),
		SequenceNumber: sequence,
		Narrative:      models.NarrativeInfo{AIGeneratedText: text},
	}
}

func events(n int, text string) []*models.StoryEvent {
	out := make([]*models.StoryEvent, n)
	for i := range out {
		out[i] = event(i+1, fmt.Sprintf("%s %d", text, i+1))
	}
	return out
}

// A token estimate that under-counts is worse than useless: it produces a
// request the provider refuses. This one is allowed to be generous.
func TestEstimateTokensNeverUnderCounts(t *testing.T) {
	cases := []struct {
		text    string
		atLeast int
	}{
		{"", 0},
		{"the quick brown fox", 4},
		{"Thistle stabs the goblin for seven piercing damage.", 9},
	}
	for _, tc := range cases {
		if got := EstimateTokens(tc.text); got < tc.atLeast {
			t.Errorf("EstimateTokens(%q) = %d, want at least %d", tc.text, got, tc.atLeast)
		}
	}

	// Every word costs at least one token, however short.
	if got := EstimateTokens("a b c d e f g h"); got < 8 {
		t.Errorf("eight one-letter words estimated at %d tokens, want at least 8", got)
	}
	// Longer text costs more. Monotonicity is the property callers rely on.
	short, long := EstimateTokens("a goblin"), EstimateTokens("a goblin and a hobgoblin and an orc")
	if long <= short {
		t.Errorf("longer text estimated at %d, shorter at %d", long, short)
	}
}

// With no budget the whole history goes through, which is the behaviour the
// turn service had before any of this existed.
func TestAssembleWithoutABudgetKeepsEverything(t *testing.T) {
	c := Assemble(Sources{Summary: "the party left Neverwinter", Events: events(20, "a thing happened")}, Budget{})

	if len(c.Recent) != 20 {
		t.Errorf("kept %d events, want all 20", len(c.Recent))
	}
	if c.Dropped != 0 || c.Truncated {
		t.Errorf("dropped %d events, truncated %v; want neither", c.Dropped, c.Truncated)
	}
	if !strings.Contains(c.Block(), "the party left Neverwinter") {
		t.Error("the summary is missing from the block")
	}
}

// Recency wins. A model that cannot see the last exchange contradicts it.
func TestAssembleKeepsTheNewestEventsUnderABudget(t *testing.T) {
	all := events(30, "the party walked onward through the mist and the rain")
	c := Assemble(Sources{Summary: "", Events: all}, Budget{MaxTokens: 60})

	if len(c.Recent) == 0 || len(c.Recent) == 30 {
		t.Fatalf("kept %d of 30 events, want some but not all", len(c.Recent))
	}
	if c.Tokens > 60 {
		t.Errorf("assembled %d tokens, over the budget of 60", c.Tokens)
	}
	if c.Dropped != 30-len(c.Recent) {
		t.Errorf("dropped = %d, but %d events were cut", c.Dropped, 30-len(c.Recent))
	}
	if !c.Truncated {
		t.Error("Truncated should be set when events were cut")
	}

	// The kept events must be the last ones, still in chronological order.
	last := c.Recent[len(c.Recent)-1]
	if last.SequenceNumber != 30 {
		t.Errorf("newest kept event is #%d, want #30", last.SequenceNumber)
	}
	for i := 1; i < len(c.Recent); i++ {
		if c.Recent[i].SequenceNumber <= c.Recent[i-1].SequenceNumber {
			t.Fatalf("events are out of order: #%d after #%d",
				c.Recent[i].SequenceNumber, c.Recent[i-1].SequenceNumber)
		}
	}
}

// The floor is what stops a tight budget producing an amnesiac DM.
func TestMinRecentSurvivesEvenAnImpossibleBudget(t *testing.T) {
	c := Assemble(Sources{Summary: "", Events: events(10, "a very long narration that will not fit in any small budget at all")}, Budget{MaxTokens: 1, MinRecent: 3})

	if len(c.Recent) != 3 {
		t.Fatalf("kept %d events, want the 3 the floor guarantees", len(c.Recent))
	}
	if c.Recent[2].SequenceNumber != 10 {
		t.Errorf("the floor kept the wrong events: newest is #%d", c.Recent[2].SequenceNumber)
	}
	if !c.OverBudget {
		t.Error("OverBudget should be set when the floor beats the budget")
	}
}

// The summary is the only thing that remembers session one, so it outranks
// the middle of the history -- but not the last few exchanges.
func TestSummaryIsKeptAheadOfOlderEvents(t *testing.T) {
	summary := "Session one: the party cleared the goblin cave and freed Sildar."
	all := events(30, "the party walked onward through the mist")

	with := Assemble(Sources{Summary: summary, Events: all}, Budget{MaxTokens: 80, MinRecent: 2})
	if with.Summary == "" {
		t.Fatal("the summary was dropped while older events were kept")
	}
	if !strings.Contains(with.Block(), "Sildar") {
		t.Error("the summary is missing from the block")
	}

	// Paying for the summary means fewer events fit.
	without := Assemble(Sources{Summary: "", Events: all}, Budget{MaxTokens: 80, MinRecent: 2})
	if len(with.Recent) >= len(without.Recent) {
		t.Errorf("the summary cost nothing: %d events with it, %d without",
			len(with.Recent), len(without.Recent))
	}
}

// A summary too large for the budget is dropped rather than allowed to crowd
// out the scene the player is standing in.
func TestAnOversizedSummaryIsDropped(t *testing.T) {
	summary := strings.Repeat("a long and rambling recap of everything that ever happened. ", 40)
	c := Assemble(Sources{Summary: summary, Events: events(5, "the party moved on")}, Budget{MaxTokens: 40, MinRecent: 2})

	if c.Summary != "" {
		t.Error("an oversized summary should be dropped, not truncated mid-sentence")
	}
	if len(c.Recent) < 2 {
		t.Errorf("kept %d events, want at least the floor of 2", len(c.Recent))
	}
	if !c.Truncated {
		t.Error("Truncated should be set when the summary was dropped")
	}
}

// An empty history must read as a sentence: a blank value in a prompt reads
// as an invitation to invent a past.
func TestEmptyContextStillReadsAsASentence(t *testing.T) {
	block := Assemble(Sources{Summary: "", Events: nil}, Budget{}).Block()
	if strings.TrimSpace(block) == "" {
		t.Fatal("an empty context produced an empty block")
	}
	if !strings.Contains(block, "nothing has happened yet") {
		t.Errorf("block = %q, want it to say nothing has happened", block)
	}
}

// Events with no text at all contribute nothing rather than a bare dash.
func TestEventsWithoutTextAreSkipped(t *testing.T) {
	all := []*models.StoryEvent{event(1, "the door opens"), event(2, ""), event(3, "a goblin steps through")}
	block := Assemble(Sources{Summary: "", Events: all}, Budget{}).Block()

	if strings.Contains(block, "- \n") || strings.HasSuffix(strings.TrimSpace(block), "-") {
		t.Errorf("an empty event produced a bare bullet:\n%s", block)
	}
	for _, want := range []string{"the door opens", "a goblin steps through"} {
		if !strings.Contains(block, want) {
			t.Errorf("block is missing %q:\n%s", want, block)
		}
	}
}

// The reported token count has to describe the block that is actually sent,
// or a budget built on it is fiction.
func TestReportedTokensMatchTheBlock(t *testing.T) {
	c := Assemble(Sources{Summary: "the party left Neverwinter", Events: events(6, "something happened")}, Budget{})
	if got := EstimateTokens(c.Block()); got > c.Tokens+8 {
		t.Errorf("block estimates at %d tokens but Context reports %d", got, c.Tokens)
	}
}

// A player's own words are what the parser needs when the narration is thin.
func TestPlayerInputIsUsedWhenThereIsNoNarration(t *testing.T) {
	e := &models.StoryEvent{SequenceNumber: 1, Trigger: models.EventTrigger{PlayerInput: "I open the chest"}}
	if block := Assemble(Sources{Events: []*models.StoryEvent{e}}, Budget{}).Block(); !strings.Contains(block, "I open the chest") {
		t.Errorf("player input was not used as a fallback:\n%s", block)
	}
}

// Silently eliding history is how a DM decides the campaign began this
// morning. If events were cut, the block has to say so.
func TestDroppedHistoryIsDisclosedInTheBlock(t *testing.T) {
	all := events(30, "the party walked onward through the mist and the rain")
	c := Assemble(Sources{Summary: "", Events: all}, Budget{MaxTokens: 60})

	if c.Dropped == 0 {
		t.Fatal("this budget should have dropped events")
	}
	block := c.Block()
	if !strings.Contains(block, "earlier events") {
		t.Errorf("the block does not disclose the %d dropped events:\n%s", c.Dropped, block)
	}
	if strings.Contains(block, "nothing has happened yet") {
		t.Error("a truncated history claimed nothing had happened")
	}
}

// The worst case: the budget is too small for even one event and there is no
// floor. Claiming an empty campaign would be a lie.
func TestEverythingDroppedStillAdmitsThereWasAHistory(t *testing.T) {
	c := Assemble(Sources{Events: events(5, strings.Repeat("a very long narration indeed ", 20))}, Budget{MaxTokens: 10})

	if len(c.Recent) != 0 {
		t.Fatalf("kept %d events, expected none to fit", len(c.Recent))
	}
	if strings.Contains(c.Block(), "nothing has happened yet") {
		t.Errorf("an elided history read as an empty one:\n%s", c.Block())
	}
}

// The DM's outstanding work outranks the middle of the history: a thread it
// opened and forgot is worse than a scene it cannot recall in detail.
func TestOpenThreadsAreKeptAheadOfOlderEvents(t *testing.T) {
	story := "Open plot threads:\n- The Redbrands hold Phandalin"
	all := events(30, "the party walked onward through the mist")

	with := Assemble(Sources{Story: story, Events: all}, Budget{MaxTokens: 90, MinRecent: 2})
	if with.Story == "" {
		t.Fatal("the story block was dropped while older events were kept")
	}
	if !strings.Contains(with.Block(), "Redbrands") {
		t.Errorf("the story block is missing from the prompt:\n%s", with.Block())
	}

	without := Assemble(Sources{Events: all}, Budget{MaxTokens: 90, MinRecent: 2})
	if len(with.Recent) >= len(without.Recent) {
		t.Errorf("the story block cost nothing: %d events with it, %d without",
			len(with.Recent), len(without.Recent))
	}
}

// Threads outrank the rolling summary too: the summary is background, a thread
// is an obligation.
func TestThreadsOutrankTheSummary(t *testing.T) {
	// Sized so exactly one of the two fits: the short story block, or the long
	// summary, but not both.
	story := "Open plot threads:\n- The Redbrands hold Phandalin"
	summary := strings.Repeat("Long ago the party left Neverwinter. ", 8)

	c := Assemble(
		Sources{Summary: summary, Story: story, Events: events(5, "a thing happened")},
		Budget{MaxTokens: 60, MinRecent: 1},
	)
	if c.Story == "" {
		t.Error("the story block lost to the summary")
	}
	if c.Summary != "" {
		t.Error("both fitted, so this test proves nothing about priority")
	}
}

// Both present and both fitting is the ordinary case.
func TestSummaryAndStoryBothReachThePrompt(t *testing.T) {
	c := Assemble(Sources{
		Summary: "The party cleared the goblin cave.",
		Story:   "Open plot threads:\n- The Redbrands hold Phandalin",
		Events:  events(3, "a thing happened"),
	}, Budget{})

	block := c.Block()
	for _, want := range []string{"goblin cave", "Redbrands", "a thing happened 3"} {
		if !strings.Contains(block, want) {
			t.Errorf("the block is missing %q:\n%s", want, block)
		}
	}
}

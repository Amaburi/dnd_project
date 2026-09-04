package memory

import (
	"fmt"
	"strings"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// Budget caps how much campaign memory a prompt may carry.
type Budget struct {
	// MaxTokens is the ceiling for the whole memory block. Zero means no
	// ceiling, which is what a short campaign wants and what the turn service
	// did before budgets existed.
	MaxTokens int

	// MinRecent is how many of the newest events survive whatever the budget
	// says. A DM that cannot see the exchange it is answering will contradict
	// it, which is worse than any amount of overspending. Zero means no floor.
	MinRecent int
}

// Context is the campaign memory handed to a prompt.
type Context struct {
	// Summary covers everything older than Recent. Empty means either nothing
	// has been compacted yet or the summary did not fit.
	Summary string `json:"summary,omitempty"`

	// Recent holds the surviving events, oldest first.
	Recent []*models.StoryEvent `json:"recent"`

	// Dropped counts events the budget cut.
	Dropped int `json:"dropped"`

	// Tokens is the estimated cost of Block.
	Tokens int `json:"tokens"`

	// Truncated reports that something was cut -- events, the summary, or both.
	Truncated bool `json:"truncated"`

	// OverBudget reports that MinRecent won and the block exceeds MaxTokens.
	// The caller decides whether to spend it; hiding this would turn a budget
	// into a suggestion nobody could audit.
	OverBudget bool `json:"over_budget"`
}

// eventLine renders one event the way the history block lists it.
//
// The fallbacks matter: an event whose narration failed still tells the model
// what the player tried, and the player's own words are often the clearer
// record of intent anyway.
func eventLine(e *models.StoryEvent) string {
	for _, candidate := range []string{
		e.Narrative.AIGeneratedText,
		e.Trigger.PlayerInput,
		e.Narrative.DMInterpretation,
	} {
		if line := strings.TrimSpace(candidate); line != "" {
			return line
		}
	}
	return ""
}

// Assemble chooses what fits, newest first.
//
// The priority order is deliberate and is the whole of the policy:
//
//  1. the newest MinRecent events, whatever the budget says
//  2. the summary, because it is the only thing that remembers session one
//  3. older events, newest first, while they fit
//
// Immediate coherence outranks long-term memory, which outranks the middle of
// the history -- the part a summary already covers.
func Assemble(summary string, chronological []*models.StoryEvent, budget Budget) Context {
	// Drop events that would render as a bare bullet before anything is
	// counted, so they neither cost tokens nor reach the block.
	usable := make([]*models.StoryEvent, 0, len(chronological))
	for _, e := range chronological {
		if eventLine(e) != "" {
			usable = append(usable, e)
		}
	}

	c := Context{}
	summary = strings.TrimSpace(summary)

	if budget.MaxTokens <= 0 {
		c.Summary = summary
		c.Recent = usable
		c.Tokens = EstimateTokens(c.Block())
		return c
	}

	floor := budget.MinRecent
	if floor > len(usable) {
		floor = len(usable)
	}

	// Step 1: the floor, unconditionally.
	keep := usable[len(usable)-floor:]
	spent := blockTokens("", keep, len(usable)-floor)

	// Step 2: the summary, if the remaining budget covers it. An oversized
	// summary is dropped whole rather than truncated: half a recap ending
	// mid-sentence invites the model to complete the thought itself.
	if summary != "" {
		if withSummary := blockTokens(summary, keep, len(usable)-floor); withSummary <= budget.MaxTokens {
			c.Summary = summary
			spent = withSummary
		} else {
			c.Truncated = true
		}
	}

	// Step 3: older events, newest first, while they fit.
	for i := len(usable) - floor - 1; i >= 0; i-- {
		candidate := usable[i:]
		cost := blockTokens(c.Summary, candidate, i)
		if cost > budget.MaxTokens {
			break
		}
		keep, spent = candidate, cost
	}

	c.Recent = keep
	c.Tokens = spent
	c.Dropped = len(usable) - len(keep)
	if c.Dropped > 0 {
		c.Truncated = true
	}
	c.OverBudget = spent > budget.MaxTokens
	return c
}

// blockTokens is what a candidate selection would cost. It renders the real
// block rather than summing the parts, so the headings, bullets and the
// elision notice are counted too -- a budget built on an estimate of something
// else is fiction.
func blockTokens(summary string, events []*models.StoryEvent, dropped int) int {
	return EstimateTokens(Context{Summary: summary, Recent: events, Dropped: dropped}.Block())
}

// Block renders the memory as the prompt variable a template receives.
func (c Context) Block() string {
	var b strings.Builder

	if summary := strings.TrimSpace(c.Summary); summary != "" {
		b.WriteString("The story so far:\n")
		b.WriteString(summary)
	}

	var lines []string
	for _, e := range c.Recent {
		if line := eventLine(e); line != "" {
			lines = append(lines, "- "+line)
		}
	}

	// Eliding history silently is how a DM decides the campaign began this
	// morning: the oldest surviving line reads as the beginning unless
	// something says otherwise.
	if c.Dropped > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "(%d earlier events are omitted for length.)", c.Dropped)
	}

	if len(lines) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("Most recently:\n")
		b.WriteString(strings.Join(lines, "\n"))
	}

	// An empty history must read as a sentence. A blank value in a prompt
	// reads as an invitation to invent a past.
	if b.Len() == 0 {
		return "nothing has happened yet"
	}
	return b.String()
}

// String summarises what the budget did, for a log line.
func (c Context) String() string {
	return fmt.Sprintf("memory: %d events, %d dropped, ~%d tokens", len(c.Recent), c.Dropped, c.Tokens)
}

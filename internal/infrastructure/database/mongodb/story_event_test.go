package mongodb

import (
	"strings"
	"testing"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// NarrativeContext is what turns the event log into the "Recent Events" block
// a prompt reads, so its shape matters as much as its content.
func TestNarrativeContextRendersReadableLines(t *testing.T) {
	events := []*models.StoryEvent{
		{
			SequenceNumber: 1,
			Narrative:      models.NarrativeInfo{AIGeneratedText: "The cellar door groans open."},
		},
		{
			SequenceNumber: 2,
			Narrative:      models.NarrativeInfo{AIGeneratedText: "A goblin lunges from the dark."},
		},
	}

	got := models.NarrativeContext(events)
	for _, want := range []string{"cellar door", "goblin lunges"} {
		if !strings.Contains(got, want) {
			t.Errorf("context is missing %q:\n%s", want, got)
		}
	}
	if lines := strings.Split(got, "\n"); len(lines) != 2 {
		t.Errorf("got %d lines, want one per event:\n%s", len(lines), got)
	}
	if !strings.HasPrefix(got, "- ") {
		t.Errorf("lines should be bulleted for readability, got:\n%s", got)
	}
}

// An event may carry the player's words, the DM's reading of them, or the
// generated prose. Whichever is present should reach the context.
func TestNarrativeContextFallsBackThroughTheEvent(t *testing.T) {
	events := []*models.StoryEvent{
		{Trigger: models.EventTrigger{PlayerInput: "I search the shelves"}},
		{Narrative: models.NarrativeInfo{DMInterpretation: "she finds a false panel"}},
		{Narrative: models.NarrativeInfo{AIGeneratedText: "Dust sheets slide away."}},
	}

	got := models.NarrativeContext(events)
	for _, want := range []string{"search the shelves", "false panel", "Dust sheets"} {
		if !strings.Contains(got, want) {
			t.Errorf("context is missing %q:\n%s", want, got)
		}
	}
}

// An empty history must read as a sentence, not an empty string: a blank value
// in a prompt reads as an invitation to invent a past.
func TestNarrativeContextHandlesAnEmptyLog(t *testing.T) {
	for name, events := range map[string][]*models.StoryEvent{
		"no events":    {},
		"nil slice":    nil,
		"empty events": {{SequenceNumber: 1}, {SequenceNumber: 2}},
	} {
		got := models.NarrativeContext(events)
		if got == "" {
			t.Errorf("%s produced an empty context", name)
		}
		if !strings.Contains(got, "nothing has happened") {
			t.Errorf("%s produced %q, want a readable placeholder", name, got)
		}
	}
}

func TestNarrativeContextTrimsWhitespace(t *testing.T) {
	events := []*models.StoryEvent{
		{Narrative: models.NarrativeInfo{AIGeneratedText: "\n  The torch gutters.  \n"}},
	}

	got := models.NarrativeContext(events)
	if got != "- The torch gutters." {
		t.Errorf("context = %q, want the line trimmed", got)
	}
}

// The event type constants are what a client filters on, so they must match
// the values the model documents.
func TestEventTypeConstants(t *testing.T) {
	cases := map[string]string{
		EventNarrative:    "narrative",
		EventDialogue:     "dialogue",
		EventCombatAction: "combat_action",
		EventDiceRoll:     "dice_roll",
		EventExploration:  "exploration",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("event type constant = %q, want %q", got, want)
		}
	}
}

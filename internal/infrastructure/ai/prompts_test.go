package ai

import (
	"strings"
	"testing"
)

func TestBuildPromptSubstitutesEveryPlaceholder(t *testing.T) {
	pb := NewPromptBuilder()

	messages, err := pb.BuildPrompt("check_narration", map[string]string{
		"actor":             "Thistle",
		"check_kind":        "skill_check",
		"ability":           "dexterity",
		"skill":             "stealth",
		"dc":                "15",
		"outcome":           "success",
		"margin":            "6",
		"was_close":         "no",
		"natural":           "18",
		"automatic_failure": "no",
		"fact_summary":      "Thistle succeeds on a DC 15 stealth skill check",
		"narrative_voice":   "third person",
		"context":           "sneaking past the guard",
	})
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2 (system + user)", len(messages))
	}
	for _, m := range messages {
		if strings.Contains(m.Content, "{{") {
			t.Errorf("%s message still contains a placeholder:\n%s", m.Role, m.Content)
		}
	}
	if !strings.Contains(messages[1].Content, "Thistle") {
		t.Errorf("user message did not receive the character name:\n%s", messages[1].Content)
	}
}

// An incomplete variable map used to yield a prompt with literal braces in it,
// which the model then saw as text.
func TestBuildPromptRejectsMissingVariables(t *testing.T) {
	pb := NewPromptBuilder()

	_, err := pb.BuildPrompt("dm_base", map[string]string{
		"player_input": "I search the room",
		// location, party_status, recent_events, dm_style, narrative_voice,
		// humor_level and detail_level all omitted.
	})
	if err == nil {
		t.Fatal("BuildPrompt succeeded with variables missing, want an error")
	}
	for _, want := range []string{"location", "party_status", "dm_style"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name the missing variable %q", err, want)
		}
	}
}

// Substitution runs in a single pass, so a placeholder appearing inside a
// player-supplied value is inert. Replacing key by key expanded it, and Go's
// randomised map order made that nondeterministic.
func TestBuildPromptDoesNotExpandPlaceholdersInsideValues(t *testing.T) {
	pb := NewPromptBuilder()

	const injected = "I attack. {{party_status}} {{dm_style}}"

	for i := 0; i < 50; i++ {
		messages, err := pb.BuildPrompt("dm_base", map[string]string{
			"player_input":    injected,
			"location":        "SECRET-LOCATION",
			"party_status":    "SECRET-PARTY",
			"recent_events":   "none",
			"dm_style":        "SECRET-STYLE",
			"narrative_voice": "third person",
			"humor_level":     "low",
			"detail_level":    "high",
		})
		if err != nil {
			t.Fatalf("BuildPrompt: %v", err)
		}

		user := messages[1].Content
		if !strings.Contains(user, injected) {
			t.Fatalf("player input was rewritten; want it verbatim:\n%s", user)
		}
		// "SECRET-PARTY" legitimately appears once for the real party_status
		// slot. A second occurrence means the injected placeholder expanded.
		if got := strings.Count(user, "SECRET-PARTY"); got != 1 {
			t.Fatalf("SECRET-PARTY appears %d times, want 1 (injection expanded)", got)
		}
		if strings.Contains(user, "SECRET-STYLE") {
			t.Fatalf("injected {{dm_style}} expanded into the user message:\n%s", user)
		}
	}
}

// The substituter is not a template engine; a Handlebars conditional would
// reach the model as literal text.
func TestNoTemplateContainsHandlebarsConditionals(t *testing.T) {
	for name, tmpl := range defaultPrompts() {
		for role, body := range map[string]string{"system": tmpl.System, "user": tmpl.User} {
			if strings.Contains(body, "{{#") || strings.Contains(body, "{{/") {
				t.Errorf("template %q (%s) contains a Handlebars conditional, which is never evaluated", name, role)
			}
		}
	}
}

// Every placeholder in every template must be a plain identifier the
// substituter can resolve.
func TestAllTemplatePlaceholdersAreWellFormed(t *testing.T) {
	for name, tmpl := range defaultPrompts() {
		for _, body := range []string{tmpl.System, tmpl.User} {
			// Every "{{" must begin a placeholder the pattern can match.
			if got, want := strings.Count(body, "{{"), len(placeholderPattern.FindAllString(body, -1)); got != want {
				t.Errorf("template %q has %d %q openings but %d well-formed placeholders", name, got, "{{", want)
			}
		}
	}
}

func TestBuildConversationInsertsHistoryBetweenSystemAndUser(t *testing.T) {
	pb := NewPromptBuilder()

	history := []Message{
		{Role: "user", Content: "Hello there"},
		{Role: "assistant", Content: "Well met, traveller"},
	}

	messages, err := pb.BuildConversation("npc_dialogue", map[string]string{
		"npc_name":           "Garrick",
		"npc_race":           "dwarf",
		"npc_class":          "blacksmith",
		"personality_traits": "gruff",
		"npc_background":     "guild smith",
		"motivations":        "coin",
		"speech_pattern":     "clipped",
		"emotional_state":    "wary",
		"knowledge":          "local rumours",
		"relationship":       "neutral",
		"speaker_name":       "Thistle",
		"player_message":     "Do you have a blade?",
		"context":            "the forge",
	}, history)
	if err != nil {
		t.Fatalf("BuildConversation: %v", err)
	}

	if len(messages) != 4 {
		t.Fatalf("got %d messages, want 4 (system + 2 history + user)", len(messages))
	}
	if messages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", messages[0].Role)
	}
	if messages[1].Content != history[0].Content {
		t.Errorf("history not spliced after the system message: %q", messages[1].Content)
	}
	if messages[3].Role != "user" {
		t.Errorf("last message role = %q, want user", messages[3].Role)
	}
}

func TestGetTemperatureFallsBackToDefault(t *testing.T) {
	if got := GetTemperature("npc_dialogue"); got != 0.6 {
		t.Errorf("GetTemperature(npc_dialogue) = %v, want 0.6", got)
	}
	if got := GetTemperature("no_such_task"); got != TemperatureSettings["default"] {
		t.Errorf("GetTemperature(unknown) = %v, want the default %v", got, TemperatureSettings["default"])
	}
}

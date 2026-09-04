package ai

import (
	"context"
	"strings"
	"testing"
)

func TestSummarizeHistoryCompressesEvents(t *testing.T) {
	service, stub := NewStubService("The party cleared the goblin cave and freed Sildar.")

	text, err := service.SummarizeHistory(context.Background(), "",
		[]string{"Thistle picked the lock.", "A goblin ambushed the party.", "Sildar was freed."})
	if err != nil {
		t.Fatalf("SummarizeHistory: %v", err)
	}
	if text != "The party cleared the goblin cave and freed Sildar." {
		t.Errorf("text = %q, want the model's reply", text)
	}

	prompt := stub.LastPrompt()
	for _, want := range []string{"Thistle picked the lock", "Sildar was freed"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt is missing %q", want)
		}
	}
	if strings.Contains(prompt, "{{") {
		t.Errorf("the prompt has unsubstituted placeholders:\n%s", prompt)
	}
}

// Compaction is cumulative: the previous summary has to reach the prompt, or
// each pass forgets everything before it.
func TestSummarizeHistoryCarriesThePreviousSummary(t *testing.T) {
	service, stub := NewStubService("Acts one and two.")

	if _, err := service.SummarizeHistory(context.Background(),
		"Act one: the goblin cave.", []string{"They crossed the moor."}); err != nil {
		t.Fatalf("SummarizeHistory: %v", err)
	}

	if !strings.Contains(stub.LastPrompt(), "Act one: the goblin cave.") {
		t.Errorf("the previous summary did not reach the prompt:\n%s", stub.LastPrompt())
	}
}

// A first compaction has no previous summary, and an empty value in a prompt
// reads as an invitation to invent a past.
func TestSummarizeHistoryWithoutAPreviousSummaryUsesAPlaceholder(t *testing.T) {
	service, stub := NewStubService("A recap.")

	if _, err := service.SummarizeHistory(context.Background(), "", []string{"They set out."}); err != nil {
		t.Fatalf("SummarizeHistory: %v", err)
	}
	prompt := stub.LastPrompt()
	if strings.Contains(prompt, "{{") {
		t.Error("an absent summary left a placeholder in the prompt")
	}
	if !strings.Contains(prompt, "this is the first") {
		t.Errorf("no fallback sentence for a first compaction:\n%s", prompt)
	}
}

// Summarising nothing is a caller error, not a provider call to pay for.
func TestSummarizeHistoryRefusesAnEmptyBatch(t *testing.T) {
	service, stub := NewStubService("unused")

	if _, err := service.SummarizeHistory(context.Background(), "Act one.", nil); err == nil {
		t.Error("summarising no events should be an error")
	}
	if len(stub.Requests) != 0 {
		t.Errorf("the provider was called %d times for an empty batch", len(stub.Requests))
	}
}

// The summary is a compression, not a continuation: the contract has to forbid
// inventing events, or a recap quietly becomes new canon.
func TestSummaryPromptForbidsInvention(t *testing.T) {
	service, stub := NewStubService("A recap.")
	if _, err := service.SummarizeHistory(context.Background(), "", []string{"They set out."}); err != nil {
		t.Fatalf("SummarizeHistory: %v", err)
	}

	system := strings.ToLower(stub.Requests[0].Messages[0].Content)
	for _, want := range []string{"never invent", "do not continue"} {
		if !strings.Contains(system, want) {
			t.Errorf("the system prompt does not say %q:\n%s", want, system)
		}
	}
}

// Compression runs cool: a creative recap is a wrong one.
func TestSummaryTemperatureIsLow(t *testing.T) {
	if got := GetTemperature("history_summary"); got > 0.3 {
		t.Errorf("history_summary temperature = %v, want a low value", got)
	}
}

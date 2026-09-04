package ai

import (
	"context"
	"fmt"
	"strings"
)

// SummaryWordLimit caps a rolling summary.
//
// The point of the summary is to be cheaper than the events it replaces, and an
// unbounded recap of a long campaign is not. It also has to survive being
// re-summarised, so it must stay well inside whatever budget assembles it.
const SummaryWordLimit = 300

// SummarizeHistory compresses old story events into a rolling summary,
// carrying the previous summary forward so nothing is forgotten wholesale.
//
// The signature is deliberately plain strings rather than a request struct:
// application/memory depends on this method and should not have to import the
// provider layer to compact a log.
func (s *Service) SummarizeHistory(ctx context.Context, previous string, events []string) (string, error) {
	lines := make([]string, 0, len(events))
	for _, event := range events {
		if line := strings.TrimSpace(event); line != "" {
			lines = append(lines, "- "+line)
		}
	}
	// Summarising nothing is a caller error, not a provider call to pay for.
	if len(lines) == 0 {
		return "", fmt.Errorf("no events to summarise")
	}

	response, err := s.completeTemplate(ctx, "history_summary", SummaryWordLimit*2, map[string]string{
		"previous_summary": orPlaceholder(previous, "this is the first summary; there is nothing before these events"),
		"events":           strings.Join(lines, "\n"),
		"word_limit":       fmt.Sprintf("%d", SummaryWordLimit),
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(response.Text), nil
}

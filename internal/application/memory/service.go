package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// Defaults chosen so the service is useful before anyone tunes it.
const (
	// DefaultWindow is how many events are ever read for one prompt. It is a
	// ceiling on work, not on memory: anything older lives in the summary.
	DefaultWindow = 60

	// DefaultCompactAfter is how many un-summarised events trigger a compaction.
	DefaultCompactAfter = 40

	// DefaultRetain is how many events stay verbatim after a compaction. The
	// summary is a paraphrase; the last stretch should still be the real words.
	DefaultRetain = 15

	// DefaultMaxTokens is a conservative slice of a small context window,
	// leaving room for the system prompt, the character sheet and the reply.
	DefaultMaxTokens = 1200

	// DefaultMinRecent is the floor no budget may cut through.
	DefaultMinRecent = 3
)

// The stores this service needs, named as narrowly as it uses them.
type (
	// EventStore is the campaign's memory.
	EventStore interface {
		GetRecentEvents(ctx context.Context, campaignID string, limit int) ([]*models.StoryEvent, error)
		GetEventsSince(ctx context.Context, campaignID string, since time.Time, limit int) ([]*models.StoryEvent, error)
	}

	// CampaignStore holds the rolling summary. Optional: without it the
	// service still budgets recent events, it simply has no long memory.
	CampaignStore interface {
		GetCampaignByCampaignID(ctx context.Context, campaignID string) (*models.Campaign, error)
		UpdateSummary(ctx context.Context, campaignID string, summary models.CampaignSummary) error
	}

	// StoryStore is the DM's outstanding work: threads it opened and
	// consequences the party set in motion. Optional -- without it the memory
	// block is history only, and the DM will drop threads it opened.
	StoryStore interface {
		GetActiveArc(ctx context.Context, campaignID string) (*models.StoryArc, error)
		GetLiveThreads(ctx context.Context, campaignID string) ([]*models.PlotThread, error)
		GetPendingConsequences(ctx context.Context, campaignID string) ([]*models.Consequence, error)
	}

	// Summarizer compresses old history.
	//
	// It takes strings and returns one, rather than the ai request types, so
	// this package does not import the provider layer to compact a log.
	// ai.Service satisfies it directly.
	Summarizer interface {
		SummarizeHistory(ctx context.Context, previous string, events []string) (string, error)
	}
)

// Service builds the memory block for a prompt and compacts the log behind it.
type Service struct {
	events     EventStore
	campaigns  CampaignStore
	summarizer Summarizer

	// Story supplies open threads and pending consequences. Optional.
	Story StoryStore

	Budget       Budget
	Window       int
	CompactAfter int
	Retain       int
}

// New wires a memory service.
//
// campaigns and summarizer are optional and may be nil: the result then budgets
// recent events with no rolling summary, which is a complete and correct
// configuration for a young campaign and the one the turn service boots with.
func New(events EventStore, campaigns CampaignStore, summarizer Summarizer) *Service {
	return &Service{
		events: events, campaigns: campaigns, summarizer: summarizer,
		Budget:       Budget{MaxTokens: DefaultMaxTokens, MinRecent: DefaultMinRecent},
		Window:       DefaultWindow,
		CompactAfter: DefaultCompactAfter,
		Retain:       DefaultRetain,
	}
}

// Build assembles the memory block for a campaign.
func (s *Service) Build(ctx context.Context, campaignID string) (Context, error) {
	summary, events, err := s.load(ctx, campaignID)
	if err != nil {
		return Context{}, err
	}

	story, err := s.storyBlock(ctx, campaignID)
	if err != nil {
		return Context{}, err
	}

	return Assemble(Sources{
		Summary: summary.Text,
		Story:   story,
		Events:  events,
	}, s.Budget), nil
}

// storyBlock renders the DM's outstanding work, or "" when nothing tracks it.
//
// An empty string rather than the "nothing is outstanding" sentence: with no
// store there is no claim to make, and asserting that nothing is pending when
// nothing is tracking it would be a lie the DM would act on.
func (s *Service) storyBlock(ctx context.Context, campaignID string) (string, error) {
	if s.Story == nil {
		return "", nil
	}

	arc, err := s.Story.GetActiveArc(ctx, campaignID)
	if err != nil {
		return "", fmt.Errorf("reading the active arc: %w", err)
	}
	threads, err := s.Story.GetLiveThreads(ctx, campaignID)
	if err != nil {
		return "", fmt.Errorf("reading plot threads: %w", err)
	}
	consequences, err := s.Story.GetPendingConsequences(ctx, campaignID)
	if err != nil {
		return "", fmt.Errorf("reading pending consequences: %w", err)
	}
	if arc == nil && len(threads) == 0 && len(consequences) == 0 {
		return "", nil
	}

	// The arc goes first: it is the frame the rest sits inside, and it is what
	// tells the DM whether to build tension or land it.
	return models.ArcBlock(arc, threads) + "\n\n" + models.StoryBlock(threads, consequences), nil
}

// load reads the stored summary and every event it does not already cover.
func (s *Service) load(ctx context.Context, campaignID string) (models.CampaignSummary, []*models.StoryEvent, error) {
	var summary models.CampaignSummary

	if s.campaigns != nil {
		campaign, err := s.campaigns.GetCampaignByCampaignID(ctx, campaignID)
		if err != nil {
			return summary, nil, fmt.Errorf("reading campaign summary: %w", err)
		}
		if campaign != nil {
			summary = campaign.Summary
		}
	}

	// A zero watermark means nothing has been compacted, and asking for
	// "events after the zero time" would be a needlessly exotic query.
	if summary.Through.IsZero() {
		events, err := s.events.GetRecentEvents(ctx, campaignID, s.Window)
		if err != nil {
			return summary, nil, fmt.Errorf("reading recent events: %w", err)
		}
		return summary, events, nil
	}

	events, err := s.events.GetEventsSince(ctx, campaignID, summary.Through, s.Window)
	if err != nil {
		return summary, nil, fmt.Errorf("reading events since the summary: %w", err)
	}
	return summary, events, nil
}

// Compact folds the older un-summarised events into the rolling summary.
//
// It reports whether it did anything. Below the threshold it does nothing and
// calls no provider: an AI request per turn to summarise five events would cost
// more than sending the five events.
func (s *Service) Compact(ctx context.Context, campaignID string) (bool, error) {
	if s.campaigns == nil || s.summarizer == nil {
		return false, nil
	}

	summary, events, err := s.load(ctx, campaignID)
	if err != nil {
		return false, err
	}
	if len(events) <= s.CompactAfter {
		return false, nil
	}

	// Everything but the retained tail gets folded in. The tail stays verbatim
	// because a paraphrase of the last exchange is what makes a DM contradict
	// itself.
	cut := len(events) - s.Retain
	if cut < 1 {
		return false, nil
	}
	older := events[:cut]

	lines := make([]string, 0, len(older))
	for _, e := range older {
		if line := eventLine(e); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return false, nil
	}

	text, err := s.summarizer.SummarizeHistory(ctx, summary.Text, lines)
	if err != nil {
		return false, fmt.Errorf("summarising history: %w", err)
	}
	// An empty reply is a failure, not a summary. Storing it would advance the
	// watermark past events nothing remembers.
	if text = strings.TrimSpace(text); text == "" {
		return false, fmt.Errorf("summarising history: the model returned nothing")
	}

	updated := models.CampaignSummary{
		Text:       text,
		Through:    older[len(older)-1].Timestamp,
		EventCount: summary.EventCount + len(older),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.campaigns.UpdateSummary(ctx, campaignID, updated); err != nil {
		return false, fmt.Errorf("storing the summary: %w", err)
	}
	return true, nil
}

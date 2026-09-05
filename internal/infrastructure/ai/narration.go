package ai

import (
	"context"
	"fmt"
	"time"
)

// NarrationStyle is the campaign's voice, applied to every narrated outcome.
type NarrationStyle struct {
	NarrativeVoice string
	CombatTone     string
}

func (s NarrationStyle) orDefaults() NarrationStyle {
	if s.NarrativeVoice == "" {
		s.NarrativeVoice = "third person, present tense"
	}
	if s.CombatTone == "" {
		s.CombatTone = "grounded and quick"
	}
	return s
}

// NarrationRequest asks for prose describing an outcome the rules engine has
// already decided.
//
// Facts comes from rules.AttackResult.Facts() or rules.CheckResult.Facts() and
// is the complete set of values the prompt may reference. Passing the engine's
// own output rather than a hand-written summary is what keeps the prose and the
// game state from drifting apart.
type NarrationRequest struct {
	Facts   map[string]string
	Context string
	Style   NarrationStyle
}

// NarrationResponse is the prose and what it cost.
type NarrationResponse struct {
	Text           string
	TokensUsed     int
	Cost           float64
	ProcessingTime time.Duration
}

// narrate is the shared path for every "describe what already happened" call.
func (s *Service) narrate(
	ctx context.Context,
	template string,
	req *NarrationRequest,
	maxTokens int,
) (*NarrationResponse, error) {
	startTime := time.Now()

	if len(req.Facts) == 0 {
		return nil, fmt.Errorf("narration needs the engine's facts; refusing to narrate an outcome nothing decided")
	}

	style := req.Style.orDefaults()
	context_ := req.Context
	if context_ == "" {
		context_ = "no additional context"
	}

	// The facts are the base; style and context are layered on top. A fact can
	// never be overwritten by a style value, because the facts are copied last.
	variables := map[string]string{
		"narrative_voice": style.NarrativeVoice,
		"combat_tone":     style.CombatTone,
		"context":         context_,
	}
	for key, value := range req.Facts {
		variables[key] = value
	}

	messages, err := s.promptBuilder.BuildPrompt(template, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	resp, err := s.client.ChatCompletion(ctx, &ChatRequest{
		Messages:    messages,
		Model:       s.config.Model,
		Temperature: Float(GetTemperature(template)),
		MaxTokens:   maxTokens,
		TopP:        Float(0.9),
	})
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	return &NarrationResponse{
		Text:           resp.Choices[0].Message.Content,
		TokensUsed:     resp.Usage.TotalTokens,
		Cost:           s.calculateCost(resp.Usage),
		ProcessingTime: time.Since(startTime),
	}, nil
}

// NarrateAction describes an attack the engine has already resolved.
//
// Pass rules.AttackResult.Facts() straight through.
func (s *Service) NarrateAction(ctx context.Context, req *NarrationRequest) (*NarrationResponse, error) {
	return s.narrate(ctx, "action_narration", req, 300)
}

// NarrateCast describes a spell the engine has already resolved.
//
// Pass rules.CastResult.Facts() straight through.
func (s *Service) NarrateCast(ctx context.Context, req *NarrationRequest) (*NarrationResponse, error) {
	return s.narrate(ctx, "spell_narration", req, 300)
}

// NarrateCheck describes an ability check, skill check or saving throw the
// engine has already resolved.
//
// Pass rules.CheckResult.Facts() straight through.
func (s *Service) NarrateCheck(ctx context.Context, req *NarrationRequest) (*NarrationResponse, error) {
	return s.narrate(ctx, "check_narration", req, 250)
}

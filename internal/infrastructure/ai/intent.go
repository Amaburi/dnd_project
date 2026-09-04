package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dnd-campaign/manager/internal/domain/models"
)

// IntentRequest asks the parser to read one player sentence.
type IntentRequest struct {
	PlayerInput string
	Options     models.ActionOptions

	// Situation is a short line of context -- "in combat, round 2" or
	// "exploring the crypt" -- which changes how a sentence should be read.
	Situation string
}

// IntentResponse is a parsed intent plus what it cost.
type IntentResponse struct {
	Intent models.Intent

	// Repaired is set when the model's answer did not fit the situation and
	// was turned into a clarifying question rather than acted on.
	Repaired   bool
	RepairNote string

	TokensUsed     int
	Cost           float64
	ProcessingTime time.Duration
}

// ExtractIntent turns a player's sentence into a structured action.
//
// This is the first half of the two-call turn: parse cheaply and
// deterministically, let the rules engine decide, then narrate the verdict.
// Nothing here resolves anything -- the returned intent is a proposal.
func (s *Service) ExtractIntent(ctx context.Context, req *IntentRequest) (*IntentResponse, error) {
	startTime := time.Now()

	if strings.TrimSpace(req.PlayerInput) == "" {
		return nil, models.Invalid("player input is empty")
	}

	situation := req.Situation
	if situation == "" {
		situation = "no combat in progress"
	}

	messages, err := s.promptBuilder.BuildPrompt("intent_extraction", map[string]string{
		"player_input": req.PlayerInput,
		"options":      req.Options.Prompt(),
		"situation":    situation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Temperature 0 and JSON mode: a parser that answers differently each time
	// makes the whole turn unreproducible.
	resp, err := s.client.ChatCompletion(ctx, &ChatRequest{
		Messages:       messages,
		Model:          s.config.Model,
		Temperature:    Float(GetTemperature("intent_extraction")),
		MaxTokens:      400,
		ResponseFormat: JSONObjectFormat(),
	})
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	intent, err := ParseIntent(resp.Choices[0].Message.Content)
	if err != nil {
		return nil, err
	}
	intent.RawInput = req.PlayerInput
	if intent.Actor == "" {
		intent.Actor = req.Options.Actor
	}

	out := &IntentResponse{
		Intent:         intent,
		TokensUsed:     resp.Usage.TotalTokens,
		Cost:           s.calculateCost(resp.Usage),
		ProcessingTime: time.Since(startTime),
	}

	// A model will occasionally name a weapon the character is not carrying.
	// Turning that into a question is safer than resolving a fiction, and far
	// safer than letting it through to the engine.
	if err := intent.Validate(req.Options); err != nil {
		out.Repaired = true
		out.RepairNote = err.Error()
		out.Intent = intent.AsUnclear(clarificationFor(intent, err))
		out.Intent.RawInput = req.PlayerInput
	}

	return out, nil
}

// clarificationFor turns a validation failure into a question for the player.
func clarificationFor(intent models.Intent, err error) string {
	if intent.Clarification != "" {
		return intent.Clarification
	}
	return fmt.Sprintf(
		"I could not work out how to do that with what you have. Could you say it another way? (%v)",
		err)
}

// ParseIntent reads the JSON object a model returned.
//
// Models wrap JSON in markdown fences often enough that refusing to unwrap one
// would fail for a reason that has nothing to do with the answer's quality, so
// the fence and any prose either side of the object are stripped first.
func ParseIntent(content string) (models.Intent, error) {
	payload := extractJSONObject(content)
	if payload == "" {
		return models.Intent{}, models.Invalid("no JSON object found in the model's reply")
	}

	var intent models.Intent
	if err := json.Unmarshal([]byte(payload), &intent); err != nil {
		return models.Intent{}, models.Invalid("could not parse the model's reply as an intent: %v", err)
	}

	intent.Normalise()
	if !intent.Action.Valid() {
		return models.Intent{}, models.Invalid("model returned unknown action %q", intent.Action)
	}
	return intent, nil
}

// extractJSONObject pulls the first balanced {...} out of a reply, ignoring
// markdown fences and any commentary around it.
func extractJSONObject(content string) string {
	text := strings.TrimSpace(content)

	// Strip a ```json fence if there is one.
	if strings.HasPrefix(text, "```") {
		if end := strings.LastIndex(text, "```"); end > 3 {
			text = text[strings.Index(text, "\n")+1 : end]
		}
	}

	start := strings.Index(text, "{")
	if start < 0 {
		return ""
	}

	depth, inString, escaped := 0, false, false
	for i := start; i < len(text); i++ {
		c := text[i]
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
			// Braces inside a string are not structure.
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return ""
}

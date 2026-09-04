package ai

import (
	"context"
	"fmt"
	"time"
)

// Service provides high-level AI operations
type Service struct {
	client        Client
	promptBuilder *PromptBuilder
	config        ClientConfig
}

// NewService creates a new AI service for the configured provider.
func NewService(config ClientConfig) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &Service{
		client:        NewOpenAICompatibleClient(config),
		promptBuilder: NewPromptBuilder(),
		config:        config,
	}, nil
}

// NarrativeRequest represents a request for narrative generation
type NarrativeRequest struct {
	PlayerInput    string
	Location       string
	PartyStatus    string
	RecentEvents   string
	DMStyle        string
	NarrativeVoice string
	HumorLevel     string
	DetailLevel    string
}

// NarrativeResponse represents a narrative generation response
type NarrativeResponse struct {
	Narrative      string
	TokensUsed     int
	Cost           float64
	ProcessingTime time.Duration
}

// GenerateNarrative generates a narrative response
func (s *Service) GenerateNarrative(ctx context.Context, req *NarrativeRequest) (*NarrativeResponse, error) {
	startTime := time.Now()

	// Build prompt
	variables := map[string]string{
		"player_input":    req.PlayerInput,
		"location":        req.Location,
		"party_status":    req.PartyStatus,
		"recent_events":   req.RecentEvents,
		"dm_style":        req.DMStyle,
		"narrative_voice": req.NarrativeVoice,
		"humor_level":     req.HumorLevel,
		"detail_level":    req.DetailLevel,
	}

	messages, err := s.promptBuilder.BuildPrompt("dm_base", variables)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Create chat request
	chatReq := &ChatRequest{
		Messages:    messages,
		Model:       s.config.Model,
		Temperature: GetTemperature("narrative_description"),
		MaxTokens:   1000,
		TopP:        0.9,
	}

	// Call AI
	resp, err := s.client.ChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	// Extract response
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	processingTime := time.Since(startTime)
	cost := s.calculateCost(resp.Usage)

	return &NarrativeResponse{
		Narrative:      resp.Choices[0].Message.Content,
		TokensUsed:     resp.Usage.TotalTokens,
		Cost:           cost,
		ProcessingTime: processingTime,
	}, nil
}

// NPCDialogueRequest represents a request for NPC dialogue
type NPCDialogueRequest struct {
	NPCName             string
	NPCRace             string
	NPCClass            string
	PersonalityTraits   string
	Background          string
	Motivations         string
	SpeechPattern       string
	EmotionalState      string
	Knowledge           string
	Relationship        string
	SpeakerName         string
	PlayerMessage       string
	Context             string
	ConversationHistory []Message
}

// NPCDialogueResponse represents an NPC dialogue response
type NPCDialogueResponse struct {
	Dialogue       string
	TokensUsed     int
	Cost           float64
	ProcessingTime time.Duration
}

// GenerateNPCDialogue generates NPC dialogue
func (s *Service) GenerateNPCDialogue(ctx context.Context, req *NPCDialogueRequest) (*NPCDialogueResponse, error) {
	startTime := time.Now()

	// Build prompt
	variables := map[string]string{
		"npc_name":           req.NPCName,
		"npc_race":           req.NPCRace,
		"npc_class":          req.NPCClass,
		"personality_traits": req.PersonalityTraits,
		"npc_background":     req.Background,
		"motivations":        req.Motivations,
		"speech_pattern":     req.SpeechPattern,
		"emotional_state":    req.EmotionalState,
		"knowledge":          req.Knowledge,
		"relationship":       req.Relationship,
		"speaker_name":       req.SpeakerName,
		"player_message":     req.PlayerMessage,
		"context":            req.Context,
	}

	messages, err := s.promptBuilder.BuildConversation("npc_dialogue", variables, req.ConversationHistory)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Create chat request
	chatReq := &ChatRequest{
		Messages:    messages,
		Model:       s.config.Model,
		Temperature: GetTemperature("npc_dialogue"),
		MaxTokens:   500,
		TopP:        0.9,
	}

	// Call AI
	resp, err := s.client.ChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	// Extract response
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	processingTime := time.Since(startTime)
	cost := s.calculateCost(resp.Usage)

	return &NPCDialogueResponse{
		Dialogue:       resp.Choices[0].Message.Content,
		TokensUsed:     resp.Usage.TotalTokens,
		Cost:           cost,
		ProcessingTime: processingTime,
	}, nil
}

// DiceInterpretationRequest represents a request for dice interpretation
type DiceInterpretationRequest struct {
	RollType      string
	CharacterName string
	Skill         string
	Roll          int
	Modifier      int
	Total         int
	DC            int
	Outcome       string
	Context       string
}

// DiceInterpretationResponse represents a dice interpretation response
type DiceInterpretationResponse struct {
	Interpretation string
	TokensUsed     int
	Cost           float64
	ProcessingTime time.Duration
}

// InterpretDiceRoll interprets a dice roll
func (s *Service) InterpretDiceRoll(ctx context.Context, req *DiceInterpretationRequest) (*DiceInterpretationResponse, error) {
	startTime := time.Now()

	// Build prompt
	variables := map[string]string{
		"roll_type":      req.RollType,
		"character_name": req.CharacterName,
		"skill":          req.Skill,
		"roll":           fmt.Sprintf("%d", req.Roll),
		"modifier":       fmt.Sprintf("%d", req.Modifier),
		"total":          fmt.Sprintf("%d", req.Total),
		"dc":             fmt.Sprintf("%d", req.DC),
		"outcome":        req.Outcome,
		"context":        req.Context,
	}

	messages, err := s.promptBuilder.BuildPrompt("dice_interpretation", variables)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Create chat request
	chatReq := &ChatRequest{
		Messages:    messages,
		Model:       s.config.Model,
		Temperature: GetTemperature("dice_interpretation"),
		MaxTokens:   300,
		TopP:        0.9,
	}

	// Call AI
	resp, err := s.client.ChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	// Extract response
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	processingTime := time.Since(startTime)
	cost := s.calculateCost(resp.Usage)

	return &DiceInterpretationResponse{
		Interpretation: resp.Choices[0].Message.Content,
		TokensUsed:     resp.Usage.TotalTokens,
		Cost:           cost,
		ProcessingTime: processingTime,
	}, nil
}

// StreamNarrative generates a streaming narrative response
func (s *Service) StreamNarrative(ctx context.Context, req *NarrativeRequest) (<-chan string, <-chan error) {
	textChan := make(chan string, 10)
	errChan := make(chan error, 1)

	go func() {
		defer close(textChan)
		defer close(errChan)

		// Build prompt
		variables := map[string]string{
			"player_input":    req.PlayerInput,
			"location":        req.Location,
			"party_status":    req.PartyStatus,
			"recent_events":   req.RecentEvents,
			"dm_style":        req.DMStyle,
			"narrative_voice": req.NarrativeVoice,
			"humor_level":     req.HumorLevel,
			"detail_level":    req.DetailLevel,
		}

		messages, err := s.promptBuilder.BuildPrompt("dm_base", variables)
		if err != nil {
			errChan <- fmt.Errorf("failed to build prompt: %w", err)
			return
		}

		// Create chat request
		chatReq := &ChatRequest{
			Messages:    messages,
			Model:       s.config.Model,
			Temperature: GetTemperature("narrative_description"),
			MaxTokens:   1000,
			TopP:        0.9,
			Stream:      true,
		}

		// Call AI with streaming
		stream, err := s.client.StreamChatCompletion(ctx, chatReq)
		if err != nil {
			errChan <- fmt.Errorf("AI request failed: %w", err)
			return
		}

		// Forward stream to text channel
		for chunk := range stream.Chunks {
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					select {
					case textChan <- content:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		// A stream can fail after it opened; without this the caller would
		// treat a truncated narrative as a complete one.
		if err := stream.Err(); err != nil {
			errChan <- fmt.Errorf("AI stream failed: %w", err)
		}
	}()

	return textChan, errChan
}

// Close closes the AI service
func (s *Service) Close() error {
	return s.client.Close()
}

// calculateCost estimates the USD cost of one completion using the configured
// provider's rates. Providers price prompt and completion tokens differently,
// so a blended rate over total tokens misreports any request whose output
// length differs from its input length -- which is most of them.
func (s *Service) calculateCost(usage Usage) float64 {
	return s.config.Pricing.Cost(usage)
}

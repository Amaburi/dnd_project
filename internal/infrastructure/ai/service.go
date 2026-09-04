package ai

import (
	"context"
	"fmt"
	"strings"
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
		Temperature: Float(GetTemperature("narrative_description")),
		MaxTokens:   1000,
		TopP:        Float(0.9),
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
		Temperature: Float(GetTemperature("npc_dialogue")),
		MaxTokens:   500,
		TopP:        Float(0.9),
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
			Temperature: Float(GetTemperature("narrative_description")),
			MaxTokens:   1000,
			TopP:        Float(0.9),
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

// ---------------------------------------------------------------------------
// The remaining templates. Every prompt in defaultPrompts has a method here:
// a template with no way to call it is dead weight that drifts out of date.
// ---------------------------------------------------------------------------

// SceneRequest asks for a description of a place.
type SceneRequest struct {
	SceneDescription string
	NarrativeStyle   string
	DetailLevel      string
	TimeOfDay        string
	Weather          string
	PartyMood        string
	RecentEvents     string
}

// DescribeScene generates a scene description.
func (s *Service) DescribeScene(ctx context.Context, req *SceneRequest) (*NarrationResponse, error) {
	return s.completeTemplate(ctx, "narrative_generation", 600, map[string]string{
		"scene_description": orPlaceholder(req.SceneDescription, "an unremarkable room"),
		"narrative_style":   orPlaceholder(req.NarrativeStyle, "atmospheric"),
		"detail_level":      orPlaceholder(req.DetailLevel, "moderate"),
		"time_of_day":       orPlaceholder(req.TimeOfDay, "unknown"),
		"weather":           orPlaceholder(req.Weather, "indoors"),
		"party_mood":        orPlaceholder(req.PartyMood, "neutral"),
		"recent_events":     orPlaceholder(req.RecentEvents, "nothing of note"),
	})
}

// StoryAdaptationRequest asks how the story should react to a player choice.
type StoryAdaptationRequest struct {
	PlayerChoice  string
	StoryState    string
	RecentEvents  string
	CampaignTheme string
	CurrentArc    string
}

// AdaptStory proposes consequences for a player's choice.
func (s *Service) AdaptStory(ctx context.Context, req *StoryAdaptationRequest) (*NarrationResponse, error) {
	return s.completeTemplate(ctx, "story_adaptation", 800, map[string]string{
		"player_choice":  orPlaceholder(req.PlayerChoice, "the party pressed on"),
		"story_state":    orPlaceholder(req.StoryState, "the campaign has just begun"),
		"recent_events":  orPlaceholder(req.RecentEvents, "nothing of note"),
		"campaign_theme": orPlaceholder(req.CampaignTheme, "heroic fantasy"),
		"current_arc":    orPlaceholder(req.CurrentArc, "the opening arc"),
	})
}

// BackstoryRequest asks for a character history.
type BackstoryRequest struct {
	CharacterName     string
	Race              string
	Class             string
	Background        string
	Level             int
	Setting           string
	Tone              string
	AdditionalDetails string
}

// GenerateBackstory writes a character backstory.
func (s *Service) GenerateBackstory(ctx context.Context, req *BackstoryRequest) (*NarrationResponse, error) {
	return s.completeTemplate(ctx, "character_backstory", 900, map[string]string{
		"character_name":     orPlaceholder(req.CharacterName, "the character"),
		"race":               orPlaceholder(req.Race, "human"),
		"class":              orPlaceholder(req.Class, "fighter"),
		"background":         orPlaceholder(req.Background, "folk hero"),
		"level":              fmt.Sprintf("%d", maxInt(req.Level, 1)),
		"setting":            orPlaceholder(req.Setting, "a standard fantasy world"),
		"tone":               orPlaceholder(req.Tone, "grounded"),
		"additional_details": orPlaceholder(req.AdditionalDetails, "none"),
	})
}

// QuestRequest asks for a quest.
type QuestRequest struct {
	QuestType  string
	Location   string
	Difficulty string
	Setting    string
	PartyLevel int
	Themes     string
}

// GenerateQuest designs a quest.
func (s *Service) GenerateQuest(ctx context.Context, req *QuestRequest) (*NarrationResponse, error) {
	return s.completeTemplate(ctx, "quest_generation", 900, map[string]string{
		"quest_type":  orPlaceholder(req.QuestType, "investigation"),
		"location":    orPlaceholder(req.Location, "a nearby settlement"),
		"difficulty":  orPlaceholder(req.Difficulty, "medium"),
		"setting":     orPlaceholder(req.Setting, "a standard fantasy world"),
		"party_level": fmt.Sprintf("%d", maxInt(req.PartyLevel, 1)),
		"themes":      orPlaceholder(req.Themes, "adventure"),
	})
}

// completeTemplate runs a template that produces prose and nothing else.
func (s *Service) completeTemplate(
	ctx context.Context,
	template string,
	maxTokens int,
	variables map[string]string,
) (*NarrationResponse, error) {
	startTime := time.Now()

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

// orPlaceholder keeps every template variable populated: BuildPrompt rejects a
// missing one, and an empty string in a prompt reads as an instruction to
// invent something.
func orPlaceholder(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

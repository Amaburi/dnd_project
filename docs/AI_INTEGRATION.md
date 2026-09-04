# AI Integration Design

> **Provider-agnostic since 2026-09-04.** This document was written against DeepSeek, and
> DeepSeek-specific details below (endpoint, model name, pricing) are historical. The code
> talks to any OpenAI-compatible `/chat/completions` endpoint and defaults to **Groq**;
> the provider is `ai.base_url` in `configs/config.yaml`, not a Go symbol. Everything about
> prompts, temperature and context strategy applies unchanged.
>
> Semantic retrieval via embeddings is out of scope — see ARCHITECTURE.md §0. Build
> context from the last N story events plus a rolling summary instead.

## Overview

The AI DM is the core innovation of this project. It handles:
- **Narrative generation**: Describe scenes, events, and outcomes
- **NPC dialogue**: Generate realistic NPC personalities and responses
- **Story adaptation**: Adjust narrative based on player choices
- **Intent interpretation**: Understand player actions and map to game mechanics
- **Dice interpretation**: Provide meaningful narratives for dice results

---

## DeepSeek API Configuration

### API Endpoint
```
POST https://api.deepseek.com/chat/completions
```

### Recommended Model Settings

| Parameter | Value | Reasoning |
|-----------|-------|-----------|
| `model` | `deepseek-chat` | Best balance of capability and cost |
| `temperature` | 0.7 | Balanced creativity and consistency |
| `max_tokens` | 1000 | Adequate for most narratives |
| `top_p` | 0.9 | Good diversity without hallucination |
| `frequency_penalty` | 0.1 | Reduce repetition |
| `presence_penalty` | 0.1 | Encourage new topics |

### API Client Structure (Go)

```go
package ai

import (
    "context"
    "dnd-campaign/config"
    "encoding/json"
    "net/http"
    "net/url"
    "time"
)

type DeepSeekClient struct {
    apiKey     string
    baseURL    string
    httpClient *http.Client
    model      string
}

type ChatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatRequest struct {
    Model       string        `json:"model"`
    Messages    []ChatMessage `json:"messages"`
    Temperature float64       `json:"temperature,omitempty"`
    MaxTokens   int           `json:"max_tokens,omitempty"`
    TopP        float64       `json:"top_p,omitempty"`
    Stream      bool          `json:"stream,omitempty"`
}

type ChatResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

type Choice struct {
    Index        int       `json:"index"`
    Message      ChatMessage `json:"message"`
    FinishReason string    `json:"finish_reason"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

---

## System Prompts

### Base DM System Prompt

```
You are an expert Dungeon Master for D&D 5th Edition. Your role is to:

1. **Narrate**: Describe scenes, environments, and events in vivid detail
2. **Interpret**: Understand player intentions and translate them into game actions
3. **Adapt**: Adjust the story dynamically based on player choices
4. **Enforce**: Apply D&D 5e rules fairly but flexibly
5. **Entertain**: Create engaging, memorable moments

## Your Personality:
- Style: {dm_style} (Narrative-focused, Rules-focused, Improv-focused)
- Voice: {narrative_voice} (Third-person omniscient, First-person narrator)
- Humor: {humor_level} (None, Light, Moderate, Heavy)
- Detail: {detail_level} (Brief, Standard, Descriptive, Comprehensive)

## World Context:
{world_context}

## Current Session State:
{current_state}

## Rules Reminders:
- Use dice rolls sparingly and dramatically
- Make rulings that keep the game fun and moving
- Ask clarifying questions when player intent is unclear
- Reward creative problem-solving
- Maintain consistent NPC voices and behaviors

## Output Format:
Always structure your response as:
1. **Narrative**: The scene description or response
2. **Game State Changes**: Any updates to location, conditions, etc.
3. **Options**: 2-3 suggested actions for the player

Remember: You are creating an collaborative story with the players.
```

### NPC Dialogue System Prompt

```
You are {npc_name}, a {npc_race} {npc_class} with the following personality:

**Personality Traits**:
{personality_traits}

**Background**: 
{npc_background}

**Motivations**:
{motivations}

**Speech Pattern**:
{speech_pattern}

**Current Emotional State**:
{emotional_state}

**Knowledge**:
{knowledge}

**Relationship to Party**:
{relationship}

Respond in character as {npc_name}. Stay true to your personality and knowledge.
Do not break character. Keep responses appropriate to the situation.
```

---

## AI Service Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         AI DM Service Layer                             │
│                                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────┐   │
│  │ Narrative    │  │ NPC          │  │ Story        │  │ Dice     │   │
│  │ Generator    │  │ Dialogue     │  │ Adapter      │  │ Interpreter│  │
│  │              │  │ Generator    │  │              │  │          │   │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └────┬─────┘   │
│         │                 │                 │                │          │
│         └─────────────────┴─────────────────┴────────────────┘          │
│                                   │                                      │
│                          ┌───────▼───────┐                             │
│                          │ Prompt Builder │                             │
│                          │ & Context      │                             │
│                          │ Manager        │                             │
│                          └───────┬───────┘                             │
│                                  │                                      │
│                          ┌───────▼───────┐                             │
│                          │ DeepSeek API  │                             │
│                          │ Client        │                             │
│                          └───────────────┘                             │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Core AI Services

### 1. Narrative Generator

```go
type NarrativeRequest struct {
    CampaignID    string
    SessionID     string
    CharacterID   string
    PlayerInput   string
    Context       NarrativeContext
    Options       NarrativeOptions
}

type NarrativeContext struct {
    CurrentLocation   string
    TimeOfDay         string
    PartyStatus       string
    RecentEvents      []string
    ActiveQuests      []string
    EnvironmentalFactors []string
}

type NarrativeOptions struct {
    IncludeDice      bool
    NarrativeStyle   string  // brief, standard, descriptive
    MaxLength        int
    FocusOn          string  // combat, exploration, social, general
}

type NarrativeResponse struct {
    Narrative       string
    DMInterpretation string
    DiceResults     *DiceResult
    GameChanges     GameStateChanges
    SuggestedActions []string
    TokenUsage      TokenUsage
}
```

**Prompt Strategy**:
```go
func (s *NarrativeService) Generate(req *NarrativeRequest) (*NarrativeResponse, error) {
    prompt := BuildNarrativePrompt(
        SystemPrompts.DMBase,
        req.Context.PlayerInput,
        req.Context.CurrentState,
        req.Options,
    )
    
    response, err := s.client.ChatCompletion(ctx, prompt)
    return ParseNarrativeResponse(response)
}
```

### 2. NPC Dialogue Generator

```go
type NPCDialogueRequest struct {
    NPCID          string
    SpeakerID      string  // character ID of speaking player
    Context        DialogueContext
    ConversationHistory []DialogueTurn
}

type DialogueContext struct {
    Setting        string
    Mood           string
    Topic          string
    PartyDisposition string
}

type DialogueTurn struct {
    Speaker    string
    Message    string
    Intent     string
    Emotional  string
}
```

**NPC Interaction Flow**:
```
Player Message → Intent Classification → NPC Selection 
→ Context Building → Dialogue Generation → Response + NPC State Update
```

### 3. Story Adapter

Handles dynamic story adjustment based on player choices:

```go
type StoryAdapter struct {
    campaignRepo    CampaignRepository
    eventRepo       EventRepository
    narrativeService *NarrativeService
}

func (s *StoryAdapter) AdaptStory(
    campaignID string,
    playerChoice string,
    currentArc string,
) (*StoryAdaptation, error) {
    
    // Get recent story context
    recentEvents := s.eventRepo.GetRecent(campaignID, 10)
    
    // Determine story branch
    storyBranch := s.determineBranch(playerChoice, currentArc)
    
    // Generate adapted narrative
    adaptation := &StoryAdaptation{
        NewDirection: storyBranch,
        Narrative: s.generateBranchNarrative(storyBranch, recentEvents),
        Consequences: s.predictConsequences(storyBranch),
        NewQuestHooks: s.generateQuestHooks(storyBranch),
    }
    
    return adaptation, nil
}
```

### 4. Dice Interpreter

```go
type DiceInterpreter struct {
    rulesEngine *RulesEngine
    narrativeService *NarrativeService
}

func (d *DiceInterpreter) InterpretRoll(
    roll *DiceRoll,
    context RollContext,
) (*InterpretedRoll, error) {
    
    // Determine mechanical outcome
    outcome := d.rulesEngine.EvaluateRoll(roll, context)
    
    // Generate narrative
    narrative, err := d.narrativeService.GenerateDiceNarrative(
        roll,
        outcome,
        context,
    )
    
    return &InterpretedRoll{
        MechanicalResult: outcome,
        Narrative:        narrative,
        IsCritical:       roll.IsNatural20() || roll.IsNatural1(),
        IsSuccess:        outcome.IsSuccess,
    }, nil
}
```

---

## Context Management

### Building AI Context

```go
type ContextBuilder struct {
    maxContextTokens int
    tokenizer        Tokenizer
}

func (b *ContextBuilder) BuildGameContext(
    campaignID string,
    sessionID string,
    recentEventCount int,
) (*GameContext, error) {
    
    // Fetch campaign details
    campaign, err := b.campaignRepo.Get(campaignID)
    
    // Fetch recent story events
    recentEvents := b.eventRepo.GetRecent(sessionID, recentEventCount)
    
    // Fetch current party status
    partyStatus := b.characterRepo.GetPartyStatus(campaignID)
    
    // Fetch active NPCs
    activeNPCs := b.npcRepo.GetActiveNPCs(campaignID)
    
    // Build structured context
    context := &GameContext{
        Campaign:    campaign,
        RecentEvents: recentEvents,
        PartyStatus: partyStatus,
        ActiveNPCs:  activeNPCs,
        Timestamp:   time.Now(),
    }
    
    // Trim to token limit
    return b.trimToTokenLimit(context), nil
}
```

### Context Compression

For long sessions, compress historical context:

```go
func (b *ContextBuilder) CompressHistory(
    events []StoryEvent,
    maxTokens int,
) []CompressedEvent {
    
    // Use AI to summarize old events
    summary := b.summarizeEvents(events)
    
    // Keep recent events detailed
    recentDetailed := events[len(events)-5:]
    olderCompressed := CompressOlderEvents(events[:len(events)-5])
    
    return append(olderCompressed, recentDetailed...)
}
```

---

## Prompt Engineering Strategies

### 1. Few-Shot Examples

```go
SystemPrompt += `
## Examples of Good Responses:

**Example 1 - Combat**:
Player: "I attack the goblin with my sword!"
Response:
[Narrative] "The goblin snarls as it sees your blade coming. You strike with precision, the sword biting into its shoulder!"
[Game State] Goblin takes 12 damage, HP now 5/17
[Options] 
- Finish the goblin off
- Try to capture it
- Check if it has information

**Example 2 - Exploration**:
Player: "I want to search the room for traps"
Response:
[Narrative] "You carefully examine the stone floor and walls..."
[Game State] Roll: Investigation check, DC 14
[Options]
- Proceed with caution
- Use a 10-foot pole
- Cast a detection spell
`
```

### 2. Chain-of-Thought Reasoning

```go
SystemPrompt += `
## Reasoning Steps (Internal):
When processing player input:
1. What is the player's intent?
2. What game mechanics apply?
3. What is the dramatic outcome?
4. How does this affect the story?
5. What are interesting next steps?

Only output the final response, not the reasoning.
`
```

### 3. Temperature Tuning by Task

```go
func GetTemperatureForTask(taskType string) float64 {
    switch taskType {
    case "combat_resolution":      return 0.3  // Consistent, predictable
    case "narrative_description":   return 0.7  // Creative
    case "npc_dialogue":            return 0.6  // Consistent character voice
    case "story_adaptation":        return 0.8  // Creative, surprising
    case "dice_interpretation":     return 0.5  // Balanced
    default:                        return 0.7
    }
}
```

---

## Error Handling & Retry

```go
type AIError struct {
    Code       string
    Message    string
    Retriable  bool
    Backoff    time.Duration
}

func (s *AIManager) ExecuteWithRetry(
    ctx context.Context,
    request *ChatRequest,
    maxRetries int,
) (*ChatResponse, error) {
    
    for attempt := 0; attempt <= maxRetries; attempt++ {
        response, err := s.client.ChatCompletion(ctx, request)
        
        if err == nil {
            return response, nil
        }
        
        aiErr, ok := err.(AIError)
        if !ok || !aiErr.Retriable {
            return nil, err
        }
        
        if attempt < maxRetries {
            time.Sleep(aiErr.Backoff * time.Duration(attempt+1))
            request.Temperature *= 0.9  // Reduce temperature on retry
        }
    }
    
    return nil, fmt.Errorf("max retries exceeded")
}
```

---

## Cost Optimization

### Token Tracking

```go
type TokenTracker struct {
    dailyLimit    int
    monthlyLimit  int
    usedToday     int
    usedThisMonth int
}

func (t *TokenTracker) TrackUsage(response *ChatResponse) {
    t.usedToday += response.Usage.TotalTokens
    t.usedThisMonth += response.Usage.TotalTokens
    
    // Alert if approaching limits
    if t.usedToday > t.dailyLimit*0.8 {
        log.Warn("Approaching daily token limit")
    }
}
```

### Caching Strategy

```go
type ResponseCache struct {
    cache *lru.Cache  // LRU cache of common responses
}

func (c *ResponseCache) Get(request PromptKey) (*CachedResponse, bool) {
    key := c.hashPrompt(request)
    return c.cache.Get(key)
}

func (c *ResponseCache) Set(request PromptKey, response string) {
    key := c.hashPrompt(request)
    c.cache.Add(key, response)
}
```

Cacheable requests:
- Generic location descriptions
- Common NPC greetings
- Standard combat descriptions
- Rule clarifications

---

## Evaluation Metrics

### Quality Metrics

| Metric | Description | Target |
|--------|-------------|--------|
| Engagement Score | Player session duration | >30 min/session |
| Narrative Coherence | Story consistency rating | >4/5 |
| Player Satisfaction | Post-session survey | >4/5 |
| AI Cost per Session | USD per session | <$0.50 |
| Response Time | Latency for AI responses | <3 seconds |

### Automated Evaluation

```go
func (s *AIService) EvaluateNarrative(
    narrative string,
    context EvaluationContext,
) *EvaluationResult {
    
    return &EvaluationResult{
        Coherence:    s.llmJudge.EvaluateCoherence(narrative),
        Engagement:   s.llmJudge.EvaluateEngagement(narrative),
        Consistency:  s.checkConsistency(narrative, context),
        GrammarScore: s.checkGrammar(narrative),
    }
}
```

---

## Future AI Enhancements

1. **Fine-tuning**: Train custom model on successful D&D sessions
2. **Voice Synthesis**: Convert narratives to speech
3. **Image Generation**: Create visual scenes with Stable Diffusion
4. **Multi-modal**: Accept voice input, generate images
5. **Memory System**: Long-term memory for campaign continuity

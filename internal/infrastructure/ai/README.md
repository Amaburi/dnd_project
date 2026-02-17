# AI Infrastructure

This package provides AI integration for the D&D Campaign Manager, supporting narrative generation, NPC dialogue, and dice interpretation.

## Setup

### 1. Set API Key

Add your DeepSeek API key to environment variables:

```bash
export DEEPSEEK_API_KEY="your-api-key-here"
```

Or add it to your `.env` file:

```
DEEPSEEK_API_KEY=your-api-key-here
```

### 2. Configuration

The AI configuration is in `configs/config.yaml`:

```yaml
deepseek:
  api_key: "${DEEPSEEK_API_KEY}"
  base_url: "https://api.deepseek.com"
  model: "deepseek-chat"
  timeout: "30s"
  max_retries: 3
```

## Usage

### Initialize AI Service

```go
import (
    "github.com/dnd-campaign/manager/internal/infrastructure/ai"
    "github.com/dnd-campaign/manager/internal/infrastructure/config"
)

// Load config
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}

// Create AI service
aiService, err := ai.NewService(ai.ClientConfig{
    APIKey:     cfg.DeepSeek.APIKey,
    BaseURL:    cfg.DeepSeek.BaseURL,
    Model:      cfg.DeepSeek.Model,
    Timeout:    cfg.DeepSeek.Timeout,
    MaxRetries: cfg.DeepSeek.MaxRetries,
})
if err != nil {
    log.Fatal(err)
}
defer aiService.Close()
```

### Generate Narrative

```go
ctx := context.Background()

resp, err := aiService.GenerateNarrative(ctx, &ai.NarrativeRequest{
    PlayerInput:    "I want to search the room for traps",
    Location:       "Ancient Temple",
    PartyStatus:    "Full health, cautious",
    RecentEvents:   "Just defeated goblin guards",
    DMStyle:        "Narrative-focused",
    NarrativeVoice: "Third-person omniscient",
    HumorLevel:     "Light",
    DetailLevel:    "Descriptive",
})

if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Narrative)
fmt.Printf("Tokens used: %d, Cost: $%.4f\n", resp.TokensUsed, resp.Cost)
```

### Generate NPC Dialogue

```go
resp, err := aiService.GenerateNPCDialogue(ctx, &ai.NPCDialogueRequest{
    NPCName:           "Thorin Ironforge",
    NPCRace:           "Dwarf",
    NPCClass:          "Blacksmith",
    PersonalityTraits: "Gruff but kind-hearted",
    Background:        "Former adventurer, now runs a smithy",
    Motivations:       "Protect the town, craft legendary weapons",
    SpeechPattern:     "Gruff, uses dwarven accent",
    EmotionalState:    "Friendly but busy",
    Knowledge:         "Expert in weapons and armor",
    Relationship:      "Friendly acquaintance",
    SpeakerName:       "Adventurer",
    PlayerMessage:     "Can you repair my sword?",
    Context:           "In the smithy, morning",
})

if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Dialogue)
```

### Interpret Dice Roll

```go
resp, err := aiService.InterpretDiceRoll(ctx, &ai.DiceInterpretationRequest{
    RollType:      "ability_check",
    CharacterName: "Elara",
    Skill:         "Stealth",
    Roll:          18,
    Modifier:      5,
    Total:         23,
    DC:            15,
    Outcome:       "success",
    Context:       "Sneaking past guards in the castle",
})

if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Interpretation)
```

### Streaming Narrative

```go
textChan, errChan := aiService.StreamNarrative(ctx, &ai.NarrativeRequest{
    PlayerInput:    "I attack the dragon!",
    Location:       "Dragon's Lair",
    PartyStatus:    "Ready for combat",
    RecentEvents:   "Dragon just woke up",
    DMStyle:        "Dramatic",
    NarrativeVoice: "Third-person",
    HumorLevel:     "None",
    DetailLevel:    "Descriptive",
})

// Read stream
for {
    select {
    case text, ok := <-textChan:
        if !ok {
            return
        }
        fmt.Print(text)
    case err := <-errChan:
        if err != nil {
            log.Fatal(err)
        }
    }
}
```

## Available Prompt Templates

The service includes pre-built prompt templates:

- **`dm_base`** - General DM narrative generation
- **`npc_dialogue`** - NPC conversation
- **`narrative_generation`** - Scene descriptions
- **`combat_narration`** - Combat action descriptions
- **`dice_interpretation`** - Dice roll interpretation
- **`story_adaptation`** - Story branching based on choices
- **`character_backstory`** - Character backstory generation
- **`quest_generation`** - Quest creation

## Temperature Settings

Different tasks use different temperature settings for optimal results:

- **Combat Resolution**: 0.3 (consistent, predictable)
- **Narrative Description**: 0.7 (creative)
- **NPC Dialogue**: 0.6 (consistent character voice)
- **Story Adaptation**: 0.8 (creative, surprising)
- **Dice Interpretation**: 0.5 (balanced)

## Custom Prompts

You can add custom prompt templates:

```go
promptBuilder := ai.NewPromptBuilder()

promptBuilder.AddTemplate("custom_template", ai.PromptTemplate{
    System: "You are a {{role}}...",
    User:   "{{user_input}}",
})

messages, err := promptBuilder.BuildPrompt("custom_template", map[string]string{
    "role":       "treasure hunter",
    "user_input": "Find the hidden treasure",
})
```

## Error Handling

The AI client includes automatic retry logic with exponential backoff:

```go
resp, err := aiService.GenerateNarrative(ctx, req)
if err != nil {
    if aiErr, ok := err.(*ai.Error); ok {
        fmt.Printf("AI Error: %s (Code: %s, Retriable: %v)\n", 
            aiErr.Message, aiErr.Code, aiErr.IsRetriable())
    }
    return err
}
```

## Cost Tracking

All responses include token usage and estimated cost:

```go
resp, err := aiService.GenerateNarrative(ctx, req)
if err != nil {
    return err
}

fmt.Printf("Tokens: %d\n", resp.TokensUsed)
fmt.Printf("Cost: $%.4f\n", resp.Cost)
fmt.Printf("Processing Time: %v\n", resp.ProcessingTime)
```

## Alternative AI Providers

To use a different AI provider (e.g., OpenAI, Claude), implement the `Client` interface:

```go
type Client interface {
    ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    StreamChatCompletion(ctx context.Context, req *ChatRequest) (<-chan *ChatStreamResponse, error)
    Close() error
}
```

Then pass your custom client to the service:

```go
customClient := NewCustomAIClient(config)
service := &ai.Service{
    client:        customClient,
    promptBuilder: ai.NewPromptBuilder(),
    config:        config,
}
```

## Testing

Mock the AI service for testing:

```go
type MockAIService struct{}

func (m *MockAIService) GenerateNarrative(ctx context.Context, req *ai.NarrativeRequest) (*ai.NarrativeResponse, error) {
    return &ai.NarrativeResponse{
        Narrative:  "Mock narrative response",
        TokensUsed: 100,
        Cost:       0.00002,
    }, nil
}
```

## Rate Limiting

Configure rate limits in `config.yaml`:

```yaml
rate_limit:
  requests_per_minute: 60
  burst: 10
  ai_requests_per_hour: 100
```

## Best Practices

1. **Always use context with timeout** for AI requests
2. **Cache frequent requests** to reduce costs
3. **Monitor token usage** to control expenses
4. **Use streaming** for long-form content
5. **Implement fallbacks** for AI failures
6. **Log all AI interactions** for debugging
7. **Validate AI responses** before using them

## Troubleshooting

### API Key Not Found

```
Error: API key not set
```

**Solution**: Set the `DEEPSEEK_API_KEY` environment variable.

### Timeout Errors

```
Error: context deadline exceeded
```

**Solution**: Increase timeout in config or optimize prompts.

### Rate Limit Errors

```
Error: API error (429): Too many requests
```

**Solution**: Implement rate limiting or reduce request frequency.

## Support

For issues or questions:
- Check the [AI Integration docs](../../../docs/AI_INTEGRATION.md)
- Review the [API Design](../../../docs/API_DESIGN.md)
- Open an issue on GitHub

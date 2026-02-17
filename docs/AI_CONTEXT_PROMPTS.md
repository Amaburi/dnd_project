# AI Context & Prompt Engineering Guide

This document covers how to manage conversation context for the AI DM and effective prompt engineering strategies.

---

## Part 1: Conversation Context Management

### Overview

The AI DM needs to maintain coherent context across a campaign session. This includes:
- **Session Context**: Current location, party status, time of day
- **Conversation History**: Recent player actions and AI responses
- **Character Knowledge**: What NPCs know about the party
- **Story State**: Current plot threads and quest progress

### Context Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                      AI Context Management System                       │
│                                                                          │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────┐  │
│  │ Short-term      │    │ Medium-term     │    │ Long-term           │  │
│  │ Context         │    │ Context         │    │ Context             │  │
│  │ (Current Turn)  │    │ (Session)       │    │ (Campaign)          │  │
│  │                 │    │                 │    │                     │  │
│  │ - Player input  │    │ - Session events │    │ - Campaign lore     │  │
│  │ - Dice results  │    │ - Story arc     │    │ - NPC backstories   │  │
│  │ - Immediate     │    │ - Party status  │    │ - World history     │  │
│  │   NPCs nearby   │    │ - Quest hooks   │    │ - Major events      │  │
│  └────────┬────────┘    └────────┬────────┘    └──────────┬──────────┘  │
│           │                      │                         │              │
│           └──────────────────────┼─────────────────────────┘              │
│                                  ▼                                        │
│                    ┌─────────────────────────┐                           │
│                    │ Context Builder         │                           │
│                    │ - Assembles prompt      │                           │
│                    │ - Manages token limits  │                           │
│                    │ - Caches frequent       │                           │
│                    │   contexts              │                           │
│                    └───────────┬─────────────┘                           │
│                                ▼                                          │
│                    ┌─────────────────────────┐                           │
│                    │ DeepSeek API            │                           │
│                    └─────────────────────────┘                           │
└─────────────────────────────────────────────────────────────────────────┘
```

### Context Layers

#### 1. Short-Term Context (Current Turn)

```go
type ShortTermContext struct {
    TurnNumber      int
    CurrentLocation string
    CurrentTime     time.Time
    
    // Who just acted
    LastActor       *CharacterSummary
    LastAction      string
    LastDiceResult  *DiceResult
    
    // What's currently happening
    CombatStatus    *CombatSummary
    NPCsPresent     []NPCSummary
    
    // Player's exact input
    PlayerMessage   string
    PlayerIntent    IntentClassification
}
```

#### 2. Medium-Term Context (Current Session)

```go
type MediumTermContext struct {
    SessionID       string
    SessionStart    time.Time
    
    // Recent history (last 5-10 events)
    RecentEvents    []EventSummary
    
    // Party information
    PartyStatus     PartySummary
    PartyLocation   string
    
    // Story progress
    CurrentQuest    string
    PlotThreads     []PlotThread
    TensionLevel    TensionLevel
    
    // Resources used
    SpellsCast      []string
    ItemsUsed       []string
    EncountersFaced int
}
```

#### 3. Long-Term Context (Campaign)

```go
type LongTermContext struct {
    CampaignID      string
    CampaignStart   time.Time
    
    // World building
    WorldName       string
    MajorLocations  []LocationSummary
    Factions        []FactionSummary
    
    // NPC registry
    NPCsMet         []NPCSummary
    NPCRelationships map[string]Relationship
    
    // Story history
    MajorEvents     []MajorEvent
    QuestsCompleted []string
    QuestsFailed    []string
    
    // Party history
    Characters      []CharacterHistory
    Deaths          []CharacterDeath
    NotableMoments  []NotableMoment
}
```

### Context Building Process

```go
type ContextBuilder struct {
    maxTokens       int
    tokenBudget     TokenBudget
    historyWindow   int  // Number of events to keep detailed
    compressionRate float64
}

func (b *ContextBuilder) BuildContext(
    campaignID string,
    sessionID string,
    request *AIRequest,
) (*AIContext, error) {
    
    // Calculate available tokens
    availableTokens := b.calculateAvailableTokens(request)
    
    // Fetch and assemble context layers
    shortTerm := b.buildShortTermContext(request)
    mediumTerm := b.buildMediumTermContext(sessionID, availableTokens)
    longTerm := b.buildLongTermContext(campaignID, availableTokens)
    
    // Trim if over budget
    trimmed := b.trimToBudget(
        shortTerm, 
        mediumTerm, 
        longTerm, 
        availableTokens,
    )
    
    // Add to cache
    b.cache.Add(request.SessionID, trimmed)
    
    return trimmed, nil
}
```

### Token Management

```go
type TokenBudget struct {
    TotalLimit     int
    SystemPrompt   int
    ReservedForAI  int
    AvailableForContext int
}

func (b *ContextBuilder) calculateAvailableTokens(request *AIRequest) int {
    // Typical DeepSeek limits
    const maxContextTokens = 32768  // 32K context window
    
    // Estimate system prompt size
    systemTokens := estimateTokens(SystemPrompts.DMBase)
    
    // Reserve for response
    const responseReserve = 1000
    
    // Calculate available
    available := maxContextTokens - systemTokens - responseReserve
    
    return available
}

func estimateTokens(text string) int {
    // Rough estimate: 4 characters per token on average
    return len(text) / 4
}
```

### Context Compression

For long sessions, compress older events:

```go
type ContextCompressor struct {
    compressionModel string
    summaryLength    int
}

func (c *ContextCompressor) CompressEvents(
    events []StoryEvent,
    maxTokens int,
) []CompressedEvent {
    
    // Separate recent and old events
    recentEvents := events[len(events)-5:]
    oldEvents := events[:len(events)-5]
    
    // Summarize old events
    var compressed []CompressedEvent
    
    for i := 0; i < len(oldEvents); i += 3 {
        batch := oldEvents[i:min(i+3, len(oldEvents))]
        summary := c.summarizeBatch(batch)
        compressed = append(compressed, CompressedEvent{
            Summary:   summary,
            TokenCost: estimateTokens(summary),
        })
    }
    
    // Keep recent events detailed
    for _, event := range recentEvents {
        compressed = append(compressed, CompressedEvent{
            Detailed:  event,
            TokenCost: estimateTokens(formatEvent(event)),
        })
    }
    
    return compressed
}

func (c *ContextCompressor) summarizeBatch(events []StoryEvent) string {
    prompt := fmt.Sprintf(`Summarize these D&D session events in 2-3 sentences:
%s

Focus on: major discoveries, NPC interactions, plot developments, and quest progress.`,
        formatEventsForSummary(events))
    
    response, _ := c.aiClient.Complete(prompt)
    return response
}
```

### Conversation History Management

```go
type ConversationManager struct {
    maxHistoryLength  int
    windowSize        int  // How many turns to keep in detail
    summaryThreshold  int  // When to start summarizing
}

type ConversationTurn struct {
    PlayerMessage  string
    PlayerIntent   Intent
    AIResponse     string
    DiceResults    []DiceResult
    GameChanges    []GameChange
    Timestamp      time.Time
}

func (m *ConversationManager) AddTurn(turn ConversationTurn) {
    m.history = append(m.history, turn)
    
    // Summarize if too long
    if len(m.history) > m.summaryThreshold {
        m.summarizeOldestTurns()
    }
    
    // Prune if way too long
    if len(m.history) > m.maxHistoryLength {
        m.pruneOldestTurns()
    }
}

func (m *ConversationManager) GetRecentContext() []ConversationTurn {
    if len(m.history) <= m.windowSize {
        return m.history
    }
    
    // Keep recent detailed, older summarized
    recent := m.history[len(m.history)-m.windowSize:]
    
    // Add summary of older turns
    summary := m.generateSummary(m.history[:len(m.history)-m.windowSize])
    
    return append([]ConversationTurn{{
        Type:    "summary",
        Content: summary,
    }}, recent...)
}
```

---

## Part 2: Prompt Engineering Strategies

### Core Principles

1. **Clear Role Definition**: The AI must understand its role as DM
2. **Structured Output**: Define expected response format
3. **Context Injection**: Provide relevant game state
4. **Few-Shot Examples**: Show good examples of desired behavior
5. **Constraint Setting**: Boundaries for AI behavior

### System Prompt Templates

#### Base DM System Prompt

```go
const SystemPromptDM = `You are an expert Dungeon Master for D&D 5th Edition.

## Your Role
- Narrate scenes, environments, and events with vivid detail
- Interpret player intentions and translate to game mechanics
- Create engaging, dynamic stories that adapt to player choices
- Maintain consistent NPC voices and personalities
- Apply D&D 5e rules fairly but prioritize fun

## Current Campaign Context
World: {{.WorldName}}
Era: {{.Era}}
Tone: {{.Tone}}
Current Location: {{.Location}}
Recent Events: {{.RecentEvents}}

## Party Status
{{range .PartyMembers}}
- {{.Name}} ({{.Race}} {{.Class}}, HP: {{.HP}}/{{.MaxHP}})
{{end}}

## Rules Reminders
- Ask clarifying questions when player intent is unclear
- Use dice rolls to create dramatic moments
- Reward creative problem-solving
- Make rulings that keep the game moving

## Output Format
Always respond in this format:
1. **Narrative**: [Vivid description of the scene]
2. **Game Changes**: [Any changes to location, conditions, etc.]
3. **Options**: [2-3 suggested actions for players]

Remember: You are co-creating a story with the players.`
```

#### NPC Dialogue Prompt

```go
const SystemPromptNPC = `You are {{.NPCName}}, a {{.Race}} {{.Class}}.

## Personality
- Traits: {{.Traits}}
- Ideal: {{.Ideal}}
- Bond: {{.Bond}}
- Flaw: {{.Flaw}}

## Background
{{.Background}}

## Knowledge
{{.Knowledge}}

## Current Mood: {{.Mood}}
## Relationship to Party: {{.Relationship}}

## Speaking Style
{{.SpeechPattern}}

Respond in character. Stay true to your personality and knowledge.`
```

### Dynamic Prompt Injection

#### Campaign State Injection

```go
type PromptInjector struct {
    campaignRepo  CampaignRepository
    eventRepo     EventRepository
    characterRepo CharacterRepository
}

func (i *PromptInjector) InjectContext(
    campaignID string,
    currentTurn TurnContext,
) string {
    
    context := fmt.Sprintf(`## Current Session Context
Location: %s
Time: %s
Party Status: %s

## Recent Events (Last 3 turns)
%s

## Current Quest
%s

## Active Plot Threads
%s`,
        currentTurn.Location,
        currentTurn.TimeOfDay,
        formatPartyStatus(campaignID),
        formatRecentEvents(currentTurn.SessionID),
        getCurrentQuest(campaignID),
        getActivePlotThreads(campaignID),
    )
    
    return context
}
```

#### Character Context Injection

```go
func (i *PromptInjector) InjectCharacterContext(
    characterID string,
) string {
    
    char, _ := i.characterRepo.Get(characterID)
    
    return fmt.Sprintf(`## Character Information
Name: %s
Race: %s
Class: %s
Level: %d
HP: %d/%d
Stats: STR %d, DEX %d, CON %d, INT %d, WIS %d, CHA %d

Known Skills: %s
Equipment: %s

Personality: %s
Background: %s`,
        char.Name,
        char.Race,
        char.Class,
        char.Level,
        char.CurrentHP,
        char.MaxHP,
        char.Strength,
        char.Dexterity,
        char.Constitution,
        char.Intelligence,
        char.Wisdom,
        char.Charisma,
        formatSkills(char),
        formatEquipment(char),
        char.Personality,
        char.Background,
    )
}
```

### Few-Shot Learning

```go
const FewShotExamples = `
## Examples of Good AI DM Responses

### Example 1: Combat
**Player Input**: "I attack the goblin with my sword!"

[Narrative] "The goblin snarls, brandishing its crude dagger. You charge forward, your sword gleaming in the torchlight. With a powerful swing, you connect - the blade bites into the goblin's shoulder, drawing a pained screech!"

[Game State] Goblin takes 12 slashing damage (HP: 5/17). Goblin is bloodied but still fighting.

[Options]
- Finish the goblin off
- Attempt to knock it unconscious
- Check if it has information

### Example 2: Exploration
**Player Input**: "I want to search the room for traps."

[Narrative] "You carefully examine the stone floor, running your fingers along the ancient blocks. After a moment, you notice a subtle discoloration near the eastern wall - a pressure plate, nearly invisible. Your investigation reveals a tripwire connected to a mechanism in the ceiling."

[Game State] Roll: Investigation check, DC 14, Result: 18 - Success!

[Options]
- Carefully disarm the trap
- Mark the location and proceed cautiously
- Use a 10-foot pole to trigger it safely

### Example 3: Social Interaction
**Player Input**: "I try to persuade the guard to let us pass."

[Narrative] "The guard eyes you warily, hand resting on his spear. You appeal to his sense of duty, suggesting that a wise guard would let potentially dangerous individuals through without trouble - less trouble for him, less risk of injury."

[Game State] Roll: Persuasion check, DC 13, Result: 16 - Success!

The guard visibly relaxes. "Alright, pass through. And keep out of trouble."

[Options]
- Thank him and proceed
- Ask about other dangers in the area
- Offer a small bribe for information
`
```

### Temperature & Parameter Tuning

```go
type PromptParameters struct {
    Temperature    float64
    MaxTokens      int
    TopP           float64
    FrequencyPenalty float64
    PresencePenalty float64
}

func GetParametersForTask(taskType TaskType) PromptParameters {
    switch taskType {
    case TaskTypeCombatResolution:
        return PromptParameters{
            Temperature:    0.3,
            MaxTokens:      500,
            TopP:           0.9,
            FrequencyPenalty: 0.2,
            PresencePenalty: 0.1,
        }
        
    case TaskTypeNarrativeDescription:
        return PromptParameters{
            Temperature:    0.7,
            MaxTokens:      800,
            TopP:           0.9,
            FrequencyPenalty: 0.1,
            PresencePenalty: 0.1,
        }
        
    case TaskTypeNPCDialogue:
        return PromptParameters{
            Temperature:    0.6,
            MaxTokens:      400,
            TopP:           0.9,
            FrequencyPenalty: 0.15,
            PresencePenalty: 0.1,
        }
        
    case TaskTypeStoryAdaptation:
        return PromptParameters{
            Temperature:    0.8,
            MaxTokens:      1000,
            TopP:           0.95,
            FrequencyPenalty: 0.1,
            PresencePenalty: 0.15,
        }
        
    case TaskTypeDiceInterpretation:
        return PromptParameters{
            Temperature:    0.5,
            MaxTokens:      300,
            TopP:           0.9,
            FrequencyPenalty: 0.2,
            PresencePenalty: 0.1,
        }
        
    default:
        return PromptParameters{
            Temperature:    0.7,
            MaxTokens:      600,
            TopP:           0.9,
        }
    }
}
```

### Prompt Optimization

#### Instruction Ordering

```go
// Best practices for prompt structure:
// 1. Role definition first
// 2. Core instructions
// 3. Context injection
// 4. Constraints
// 5. Output format
// 6. Examples

const OptimizedPromptTemplate = `You are [ROLE - specific DM persona]

[CONTEXT - current game state]
[current location]
[party status]
[recent events]

[INSTRUCTIONS - what to do]
- Handle player input naturally
- Generate engaging narrative
- Apply D&D rules fairly

[CONSTRAINTS - what NOT to do]
- Don't break character
- Don't reveal plot twists prematurely
- Don't make rulings that contradict core rules

[OUTPUT FORMAT]
1. **Narrative**: [description]
2. **Game Changes**: [state updates]
3. **Options**: [suggestions]

[EXAMPLES]
[few-shot examples]`
```

#### Constraint Injection

```go
func InjectConstraints(
    basePrompt string,
    constraints []Constraint,
) string {
    
    constraintSection := "\n\n[CRITICAL CONSTRAINTS]\n"
    
    for _, c := range constraints {
        constraintSection += fmt.Sprintf("- %s\n", c.Description)
    }
    
    return basePrompt + constraintSection
}

type Constraint struct {
    Description string
    Priority    Priority
    Enforced    bool
}

var DefaultConstraints = []Constraint{
    {"Maintain consistent NPC voices", Priority.High},
    {"Use D&D 5e rules unless asked otherwise", Priority.High},
    {"Keep descriptions vivid but not excessive", Priority.Medium},
    {"Ask for clarification on unclear player intent", Priority.High},
    {"Provide options to keep game moving", Priority.Medium},
}
```

---

## Part 3: Testing & Iteration

### Prompt Testing Framework

```go
type PromptTester struct {
    testCases []TestCase
    evaluator *ResponseEvaluator
}

type TestCase struct {
    Name           string
    Input          PlayerInput
    ExpectedOutput OutputExpectation
    Context        TestContext
}

func (t *PromptTester) RunTests() []TestResult {
    var results []TestResult
    
    for _, tc := range t.testCases {
        // Build context
        ctx := t.buildContext(tc.Context)
        
        // Generate response
        response := t.generateResponse(tc.Input, ctx)
        
        // Evaluate
        result := t.evaluator.Evaluate(response, tc.ExpectedOutput)
        results = append(results, result)
    }
    
    return results
}

type TestResult struct {
    TestName      string
    Passed        bool
    Score         float64
    Issues        []string
    Suggestions   []string
}
```

### A/B Testing Prompts

```go
type PromptVariant struct {
    ID          string
    Name        string
    SystemPrompt string
    Parameters  PromptParameters
}

func (t *PromptTester) RunABTest(
    variantA PromptVariant,
    variantB PromptVariant,
    sessions []TestSession,
) ABTestResult {
    
    resultsA := t.runVariant(variantA, sessions)
    resultsB := t.runVariant(variantB, sessions)
    
    return ABTestResult{
        VariantA:   resultsA,
        VariantB:   resultsB,
        Winner:     determineWinner(resultsA, resultsB),
        Confidence: calculateConfidence(resultsA, resultsB),
    }
}
```

### Continuous Improvement

```go
type PromptOptimizer struct {
    feedbackRepo  FeedbackRepository
    metricsRepo   MetricsRepository
    model         *PromptImprovementModel
}

func (o *PromptOptimizer) AnalyzeAndImprove() {
    // Gather feedback
    feedback := o.feedbackRepo.GetRecent()
    
    // Gather metrics
    metrics := o.metricsRepo.GetSessionMetrics()
    
    // Identify issues
    issues := o.identifyIssues(feedback, metrics)
    
    // Generate improvements
    improvements := o.generateImprovements(issues)
    
    // Apply improvements
    o.applyImprovements(improvements)
}
```

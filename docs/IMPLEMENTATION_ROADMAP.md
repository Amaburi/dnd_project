# Implementation Roadmap

This roadmap outlines the phased implementation of the AI D&D Campaign Manager, from MVP to full feature set.

---

## Phase Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          IMPLEMENTATION PHASES                               │
├──────────┬──────────┬──────────┬──────────┬──────────┬─────────────────────┤
│ Phase 1  │ Phase 2  │ Phase 3  │ Phase 4  │ Phase 5  │ Phase 6 (Future)    │
│ MVP      │ Core     │ AI DM    │ Combat   │ Polish  │ Advanced Features   │
│ Foundation│ Features│ Integration│ System   │ & Scale │                     │
├──────────┼──────────┼──────────┼──────────┼──────────┼─────────────────────┤
│ 2-3 weeks│ 3-4 weeks│ 4-5 weeks│ 3-4 weeks│ 2-3 weeks│ Ongoing             │
└──────────┴──────────┴──────────┴──────────┴──────────┴─────────────────────┘
```

---

## Phase 1: MVP Foundation (Weeks 1-3)

### Goal
Establish project infrastructure, basic MongoDB integration, and core API endpoints.

### Deliverables

#### Week 1: Project Setup
- [x] Initialize Go module and project structure
- [x] Set up Gin HTTP server
- [x] Configure MongoDB connection
- [x] Create config.yaml with environment variables
- ~~Set up Docker Compose for local development~~ — cut, personal-use project
- [x] Configure logging (zerolog)

**Milestone**: Server runs locally, connects to MongoDB

#### Week 2: Data Layer
- [x] Implement MongoDB client and repositories
- [x] Create data models (Campaign, Character)
- [x] Implement collection initialization
- [x] Set up indexes for performance
- [x] Write unit tests for data layer

**Milestone**: Can create and retrieve campaigns and characters

#### Week 3: Basic API
- [x] Implement Campaign CRUD endpoints
- [x] Implement Character CRUD endpoints
- [x] Add request validation
- [x] Create error handling middleware  (`internal/api/middleware`: request id, structured logging, panic recovery, error handler, CORS, per-client rate limit)
- [x] Write integration tests for API

**Milestone**: REST API fully functional for campaigns and characters

### Technical Tasks

```
Week 1 Tasks:
├── Create project skeleton
│   ├── cmd/server/main.go
│   ├── internal/api/
│   ├── internal/infrastructure/
│   └── configs/
├── Set up dependencies
│   ├── go mod tidy
│   └── go.sum verification
├── Configure MongoDB
│   ├── mongodb/client.go
│   └── migrations/
└── Set up Docker
    ├── Dockerfile
    └── docker-compose.yml

Week 2 Tasks:
├── Implement models
│   ├── campaign.go
│   └── character.go
├── Implement repositories
│   ├── campaign_repo.go
│   └── character_repo.go
└── Create indexes
    └── index definitions

Week 3 Tasks:
├── API handlers
│   ├── campaign_handler.go
│   └── character_handler.go
├── Router setup
│   └── router.go
└── Testing
    ├── unit tests
    └── integration tests
```

### Success Criteria
- [x] Project compiles without errors
- [x] MongoDB connection established
- [x] API endpoints respond correctly
- [ ] Unit tests pass (>80% coverage)
- [ ] Code passes linter

---

## Phase 2: Core Features (Weeks 4-7)

### Goal
Implement session management, character creation, and AI-ready architecture.

### Deliverables

#### Week 4: Sessions & Story Events
- [x] Implement Session CRUD endpoints
- [x] Create Session model and repository
- [x] Implement Story Events model
- [x] Add session-campaign relationships
- [x] Implement basic story event logging

**Milestone**: Can track game sessions within campaigns

#### Week 5: Character Enhancement
- [x] Implement ability scores and modifiers
- [x] Add skills proficiency system
- [x] Create inventory management
- [x] Implement equipment tracking
- [x] Add character validation rules

**Milestone**: Full character sheet management

#### Week 6: Dice System
- [x] Implement dice expression parser
- [x] Create dice roller with advantage/disadvantage
- [x] Add probability calculations  (`internal/domain/dice/probability.go`: exact distributions by convolution, d20 check odds, attack odds with expected damage)
- [x] Implement dice history tracking
- [x] Create dice API endpoints  (`/api/v1/dice/*` -- roll, d20, damage, and three probability routes)

**Milestone**: Fully functional dice rolling system

#### Week 7: Rules Engine Foundation
- [x] Implement ability check resolution
- [x] Create saving throw system
- [x] Add skill check resolution
- [x] Implement proficiency handling
- [x] Add rules engine core

**Milestone**: Core D&D 5e mechanics working

### Technical Tasks

```
Week 4 Tasks:
├── Session management
│   ├── session model
│   ├── session handler
│   └── session repository
├── Story events
│   ├── event model
│   ├── event handler
│   └── event repository
└── Relationships
    └── campaign-session links

Week 5 Tasks:
├── Character stats
│   ├── ability scores
│   ├── modifiers
│   └── proficiency
├── Skills system
│   ├── skill list
│   ├── proficiency tracking
│   └── skill checks
└── Inventory
    ├── item model
    ├── inventory management
    └── equipment slots

Week 6 Tasks:
├── Dice parser
│   ├── expression parsing
│   ├── roll evaluation
│   └── result calculation
├── Dice roller
│   ├── random number generation
│   ├── advantage/disadvantage
│   └── history tracking
└── Dice API
    ├── roll endpoint
    ├── history endpoint
    └── statistics

Week 7 Tasks:
├── Rules engine
│   ├── check resolution
│   ├── saving throws
│   └── skill checks
├── Proficiency
│   ├── bonus calculation
│   ├── expertise
│   └── jack of all trades
└── Validation
    └── rules validation
```

### Success Criteria
- [x] Sessions can be created and tracked
- [x] Full character management working
- [x] Dice rolling working correctly
- [x] Basic rules engine implemented
- [x] Integration tests pass

---

## Phase 3: AI DM Integration (Weeks 8-12)

### Goal
Implement DeepSeek AI integration for narrative generation and DM functionality.

### Deliverables

#### Week 8: AI Infrastructure
- [x] Set up AI client (provider-agnostic; Groq by default, not only DeepSeek)
- [x] Implement prompt builder
- [x] Create system prompt templates
- [x] Add AI response parsing
- [x] Configure temperature/parameter tuning

**Milestone**: Can make API calls to DeepSeek

#### Week 9: Narrative Generation
- [x] Implement scene description generation
- [x] Create action interpretation
- [x] Add result narrative generation
- [x] Implement style customization
- [ ] Add narrative caching

**Milestone**: AI generates compelling narratives

#### Week 10: NPC System
- [x] Create NPC model with personality
- [x] Implement NPC dialogue generation
- [ ] Add NPC memory/relationships
- [ ] Implement NPC state tracking
- [ ] Create NPC appearance generation

**Milestone**: NPCs feel alive with unique personalities

#### Week 11: Story Adaptation
- [ ] Implement dynamic story branches
- [ ] Create consequence tracking
- [ ] Add plot thread management
- [ ] Implement story arc progression
- [x] Add player choice tracking

**Milestone**: Story adapts to player choices

#### Week 12: AI Integration Polish
- [x] Implement context management  (`internal/application/memory`: budgeted assembly, rolling summary, watermark)
- [x] Add conversation history  (the append-only event log feeds every prompt through `memory.Build`)
- [x] Optimize token usage  (`EstimateTokens` + `Budget`; compaction folds old events into `campaign.summary`)
- [ ] Implement rate limiting for AI  (HTTP-level limiting is done in `middleware.RateLimit`; the per-hour *provider* budget in `rate_limit.ai_requests_per_hour` is still unwired)
- [x] Add AI cost tracking

**Milestone**: Production-ready AI DM

### Technical Tasks

```
Week 8 Tasks:
├── DeepSeek client
│   ├── API client
│   ├── Request formatting
│   └── Response parsing
├── Prompt templates
│   ├── system prompts
│   ├── few-shot examples
│   └── instruction sets
└── Configuration
    ├── temperature settings
    ├── model selection
    └── timeout handling

Week 9 Tasks:
├── Narrative generation
│   ├── scene descriptions
│   ├── action results
│   └── environmental narration
├── Context injection
│   ├── character context
│   ├── location context
│   └── event context
└── Response parsing
    ├── narrative extraction
    └── game state parsing

Week 10 Tasks:
├── NPC system
│   ├── personality models
│   ├── dialogue generation
│   └── memory system
├── NPC lifecycle
    ├── creation
    ├── state tracking
    └── relationship updates
└── NPC integration
    ├── NPC endpoints
    └── NPC CRUD

Week 11 Tasks:
├── Story system
    ├── branch management
    ├── consequence tracking
    └── arc progression
├── Player tracking
    ├── choice history
    └── preference learning
└── Dynamic content
    ├── event generation
    └── encounter creation

Week 12 Tasks:
├── Context management
│   ├── history window
│   ├── compression
    └── relevance filtering
├── Optimization
    ├── token budgeting
    ├── caching strategy
    └── batch processing
└── Monitoring
    ├── cost tracking
    ├── latency metrics
    └── quality scoring
```

### Success Criteria
- [ ] AI generates coherent narratives
- [ ] NPCs have distinct personalities
- [ ] Story adapts to player choices
- [ ] Token usage within budget
- [ ] AI responses under 3 seconds

---

## Phase 4: Combat System (Weeks 13-16)

### Goal
Implement full combat encounter management with AI integration.

### Deliverables

#### Week 13: Combat Foundation
- [x] Create Combat model
- [x] Implement initiative tracking
- [x] Add combatant management
- [x] Create turn order system
- [x] Implement combat state machine

**Milestone**: Basic combat tracking working

#### Week 14: Combat Actions
- [x] Implement attack rolls
- [x] Add damage calculation
- [x] Create spell casting in combat  (this was marked done while only the engine primitives
      existed: there was no spell catalogue, so nothing knew what a spell *did*, and the turn
      service never called them. Closed for real 2026-09-05 -- `models/spells_srd.go`,
      `rules/spellcasting.go`, and `IntentCastSpell` wired into `turn.TakeAction`.)
- [x] Implement conditions/effects
- [x] Add movement mechanics

**Milestone**: Full combat action resolution

#### Week 15: Combat AI Integration
- [x] AI enemy tactics generation
- [x] Combat narration integration
- [x] Enemy action interpretation
- [ ] Combat outcome prediction
- [x] Victory/defeat handling

**Milestone**: AI runs combat encounters

#### Week 16: Combat Polish
- [x] Combat logging
- [x] Combat history tracking
- [x] Damage/healing formulas
- [x] Death and resurrection
- [x] Combat analytics

**Milestone**: Complete combat system

### Technical Tasks

```
Week 13 Tasks:
├── Combat model
│   ├── encounter structure
│   ├── combatant data
│   └── state management
├── Initiative
│   ├── roll handling
│   ├── sorting logic
│   └── turn tracking
└── State machine
    ├── phase transitions
    ├── round management
    └── end conditions

Week 14 Tasks:
├── Combat actions
│   ├── attack rolls
│   ├── damage calculation
│   └── critical handling
├── Spell casting
│   ├── spell resolution
│   ├── concentration
│   └── spell slots
├── Conditions
│   ├── status effects
│   ├── condition tracking
│   └── condition interaction
└── Movement
    ├── positioning
    ├── reach
    └── opportunity attacks

Week 15 Tasks:
├── AI combat
    ├── enemy tactics
    ├── action selection
    └── decision making
├── Combat narration
    ├── attack descriptions
    ├── damage narration
    └── dramatic moments
└── Integration
    ├── AI endpoints
    ├── context injection
    └── response parsing

Week 16 Tasks:
├── Combat logging
│   ├── action history
│   ├── damage logs
│   └── turn summary
├── Death mechanics
│   ├── death saves
│   ├── stabilization
│   └── revival
└── Analytics
    ├── combat stats
    ├── damage dealt/taken
    └── turn analysis
```

### Success Criteria
- [x] Initiative and turns work correctly
- [x] All combat actions resolve properly
- [x] AI controls enemies intelligently
- [x] Combat narrated appropriately
- [x] Combat data persists correctly

---

## Phase 5: Polish & Scale (Weeks 17-19)

### Goal
Improve reliability, performance, and developer experience.

### Deliverables

#### Week 17: Performance
- ~~Redis caching layer~~ — cut, see ARCHITECTURE.md §0
- [ ] Query optimization
- [ ] Connection pooling tuning
- [ ] Async processing setup
- [ ] Load testing

**Milestone**: Sub-100ms API responses

#### Week 18: Reliability
- [ ] Comprehensive test coverage (>90%)
- [ ] Error handling improvements
- [x] Retry mechanisms
- [ ] Circuit breakers
- [x] Health checks

**Milestone**: Production-ready reliability

#### Week 19: Developer Experience
- [ ] API documentation (OpenAPI)
- [ ] CLI tools
- [ ] Development scripts
- [ ] Debugging tools
- [ ] Example campaigns

**Milestone**: Easy to develop and test

### Technical Tasks

```
Week 17 Tasks:
├── Caching
│   ├── cache strategies
│   └── invalidation
├── Optimization
│   ├── query optimization
│   ├── index tuning
│   └── batching
└── Scalability
    ├── load testing
    ├── stress testing
    └── capacity planning

Week 18 Tasks:
├── Testing
│   ├── coverage increase
│   ├── integration tests
│   └── E2E tests
├── Resilience
    ├── retry logic
    ├── circuit breaker
    └── bulkheads
└── Monitoring
    ├── health checks
    ├── metrics
    └── alerting

Week 19 Tasks:
├── Documentation
│   ├── OpenAPI spec
│   ├── API docs
│   └── architecture docs
├── CLI tools
│   ├── seed command
│   ├── migrate command
│   └── admin tools
└── Examples
    ├── sample campaigns
    ├── API examples
    └── tutorials
```

### Success Criteria
- [ ] API response times <100ms
- [ ] Test coverage >90%
- [ ] Full API documentation
- [ ] Easy onboarding process
- [ ] Production deployment ready

---

## Phase 6: Advanced Features (Ongoing)

### Future Enhancements

#### Multiplayer Support
- ~~WebSocket real-time updates~~ — cut, see ARCHITECTURE.md §0
- [ ] Player presence tracking
- [ ] Turn-based locking
- [ ] Simultaneous actions

#### Voice Integration
- [ ] Speech-to-text input
- [ ] Text-to-speech output
- [ ] Voice cloning for NPCs

#### Visual Enhancements
- [ ] AI image generation
- [ ] Map generation
- [ ] Character portraits
- [ ] Environmental art

#### Advanced AI
- [ ] Custom fine-tuned model
- [ ] Long-term memory system
- [ ] Player preference learning
- [ ] Dynamic difficulty adjustment

#### Platform Expansion
- [ ] Mobile app
- [ ] Web interface
- [ ] Desktop client
- [ ] API access for third parties

---

## Milestone Timeline

```
Month 1 (Weeks 1-4):
├── Week 1: Project Setup ✓
├── Week 2: Data Layer ✓
├── Week 3: Basic API ✓
└── Week 4: Sessions ✓

Month 2 (Weeks 5-8):
├── Week 5: Character Enhancement ✓
├── Week 6: Dice System ✓
├── Week 7: Rules Engine ✓
└── Week 8: AI Infrastructure ✓

Month 3 (Weeks 9-12):
├── Week 9: Narrative Generation ✓
├── Week 10: NPC System ✓
├── Week 11: Story Adaptation ✓
└── Week 12: AI Integration Polish ✓

Month 4 (Weeks 13-16):
├── Week 13: Combat Foundation ✓
├── Week 14: Combat Actions ✓
├── Week 15: Combat AI ✓
└── Week 16: Combat Polish ✓

Month 5 (Weeks 17-19):
├── Week 17: Performance ✓
├── Week 18: Reliability ✓
└── Week 19: Developer Experience ✓
```

---

## Risk Assessment

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| AI cost overruns | High | Medium | Caching, rate limiting, prompt optimization |
| MongoDB performance | Medium | Low | Index optimization, caching, query tuning |
| AI quality issues | Medium | Medium | Prompt engineering, few-shot learning, evaluation |
| Scope creep | High | High | Strict phase boundaries, MVP focus |
| Technical debt | Medium | Medium | Code reviews, testing, refactoring sprints |

---

## Dependencies Between Phases

```
Phase 1 (Foundation)
    │
    ▼
Phase 2 (Core Features) ──────► Phase 4 (Combat)
    │                               │
    │                               ▼
    └───────────► Phase 3 ◄─────────┘
                    (AI DM)
                        │
                        ▼
                Phase 5 (Polish)
```

---

## Success Metrics

### Technical Metrics
- API uptime: >99.9%
- Response time: P95 <200ms
- Test coverage: >90%
- Code review: 100% coverage

### Business Metrics
- Session duration: >30 minutes
- AI response relevance: >4/5
- Player satisfaction: >4/5
- Cost per session: <$0.50

---

## Next Steps

After completing this roadmap, you'll have:
1. ✅ Production-ready AI D&D campaign manager
2. ✅ Scalable architecture
3. ✅ Comprehensive documentation
4. ✅ Foundation for future enhancements

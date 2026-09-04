# AI D&D Campaign Manager - Architecture Design

## 0. Scope decisions (2026-09-04)

Four subsystems this document originally specified are **deliberately out of scope**.
They were cut to keep the project finishable by one person; each solves a problem this
project does not yet have.

| Cut | Why | Do this instead |
|---|---|---|
| **Event sourcing** (replay, snapshots, undo) | Large, invasive complexity for a feature nobody has asked for. Every write path pays for it. | Append-only `story_events`, ordered by `sequence_number`. Gives the AI its history and the player a log — ~90% of the value. |
| **Vector embeddings / semantic retrieval** | Needs an embedding model, a vector store and a reindex on every edit. A single campaign's context already fits in a prompt window. | Last N story events plus a rolling summary, selected by recency. |
| **Redis** | A second datastore to run, configure and keep consistent, for a single-process app with no measured cache pressure. | Nothing. Add an in-process map behind a mutex if profiling ever shows a need. |
| **WebSocket / real-time** | Real-time is only meaningful with a second concurrent player. | Request/response over REST. Stream a single narration with SSE if latency ever feels wrong. |

Revisit any of these when the pressure is real and measured — not before. Sections below
that still describe them are retained for context and marked as out of scope.

## 1. System Architecture Overview

This project follows **Hexagonal Architecture** (Ports and Adapters) to ensure clean separation of concerns and testability.

### High-Level Architecture Diagram

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           Presentation Layer                               │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │  REST API (Gin Framework)                                          │   │
│  │  - Campaign CRUD endpoints                                          │   │
│  │  - Game session management                                          │   │
│  │  - AI interaction endpoints                                         │   │
│  │                                                                     │   │
│  └────────────────────────────────────────────────────────────────────┘   │
├────────────────────────────────────────────────────────────────────────────┤
│                         Application Layer                                   │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐  │
│  │ Campaign      │ │ Character    │ │ Combat       │ │ AIDM Service     │  │
│  │ Service       │ │ Service      │ │ Service      │ │ (DeepSeek)       │  │
│  ├──────────────┤ ├──────────────┤ ├──────────────┤ ├──────────────────┤  │
│  │ - Create/     │ │ - Create/     │ │ - Initiative │ │ - Generate       │  │
│  │   Update      │ │   Update      │ │   tracking   │ │   narratives     │  │
│  │ - Load/Save   │ │ - Stats       │ │ - Combat     │ │ - NPC dialogue   │  │
│  │ - List        │ │   management  │ │   rounds     │ │ - Story          │  │
│  │               │ │ - Background  │ │ - Damage     │ │   adaptation     │  │
│  │               │ │   generation  │ │   calculation │ │ - Dice results   │  │
│  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────────┘  │
├────────────────────────────────────────────────────────────────────────────┤
│                          Domain Layer                                       │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │  Core Game Logic                                                    │   │
│  │  - D&D 5e Rules Engine (simplified for MVP)                         │   │
│  │  - Dice probability calculations                                    │   │
│  │  - Character state management                                        │   │
│  │  - Combat flow logic                                                 │   │
│  │  - Story state machine                                               │   │
│  └────────────────────────────────────────────────────────────────────┘   │
├────────────────────────────────────────────────────────────────────────────┤
│                        Infrastructure Layer                                 │
│  ┌──────────────────┐ ┌──────────────────┐ ┌────────────────────────────┐  │
│  │ MongoDB           │ │ AI Provider      │ │ Configuration             │  │
│  │ - Campaign Store  │ │ - Chat completions│ │ - Environment vars       │  │
│  │ - Character Store │ │ - Streaming      │ │ - Config loader           │  │
│  │ - Session Store   │ │ - Model params   │ │ - Secret management       │  │
│  └──────────────────┘ └──────────────────┘ └────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
```

## 2. Component Breakdown

### 2.1 API Layer (Presentation)
- **Framework**: Gin for REST API
- **Protocol**: HTTP REST. WebSocket is out of scope (§0)
- **Authentication**: JWT-based (future) or API key for MVP

### 2.2 Application Services
Each service implements specific business logic:

#### Campaign Service
- Campaign lifecycle management
- Story arc tracking
- Session history
- World-building data

#### Character Service  
- Character creation (from AI-generated backgrounds)
- Stat management (STR, DEX, CON, INT, WIS, CHA)
- Inventory tracking
- Level progression

#### Combat Service
- Initiative tracking
- Turn management
- Damage/healing calculations
- Condition tracking (status effects)
- Combat log generation

#### AI DM Service (Core Innovation)
- Natural language understanding of player intent
- Narrative generation based on context
- Dynamic story adaptation
- NPC personality and dialogue
- Dice roll interpretation
- Game state context injection

### 2.3 Domain Layer (Core)
- **Rules Engine**: Simplified D&D 5e rules
- **Dice System**: Probabilistic dice with optional AI interpretation
- **State Management**: Campaign, session, and game state
- **Event Log**: Append-only `story_events` for AI context (not event sourcing — §0)

### 2.4 Infrastructure Layer
- **MongoDB**: Document database for flexible game data
- **AI provider**: any OpenAI-compatible `/chat/completions` endpoint. Groq by default;
  DeepSeek, OpenAI or a local server are a config change, not a code change. No Go code
  names a vendor.
- **Config**: Environment-based configuration

## 3. Data Flow

### 3.1 Player Action Flow

The AI never writes game state. It parses on the way in and describes on the way out;
the rules engine decides what actually happened and is the only thing that persists a
change. This is also what contains prompt injection: a player who talks the model into
proposing "set HP to 999" gets a rejected action, not free hit points.

```
Player Input
  → API
  → AI: intent extraction (structured output, low temperature)
  → Rules Engine: validate + resolve (deterministic, authoritative)
  → AI: narrate the resolved outcome (prose, higher temperature)
  → Response + State Update + story_event append
```

### 3.2 Campaign Creation Flow
```
Create Campaign → Define Settings → Generate World (AI) → Save to MongoDB 
→ Return Campaign ID
```

### 3.3 Character Creation Flow
```
Character Request → AI Background Generation → Validate Stats 
→ Create Character Record → Save to MongoDB
```

## 4. Key Design Decisions

### 4.1 Why MongoDB?
- **Flexible Schema**: Campaign data varies significantly
- **Document Model**: Nested game data (characters in campaigns, etc.)
- **Query Flexibility**: Complex queries for game state
- **Scalability**: Horizontal scaling for large campaigns

### 4.2 Why an OpenAI-compatible provider (Groq by default)?
- **Not locked in**: every major provider serves the same `/chat/completions` shape, so
  the vendor is `ai.base_url` in config. No Go code names one.
- **Latency is a product feature**: Groq returns in well under a second, which is what
  makes a two-call turn (parse intent → resolve → narrate) feel immediate rather than
  laggy. A slower provider would force one call doing both jobs badly.
- **Cheap enough to spend calls on correctness**: a small fast model can extract
  structured intent for a fraction of a cent, which is what keeps the rules engine
  authoritative (§3.1).
- **Free tier suits a personal project**, at the cost of tight rate limits — the client
  honours `Retry-After` for this reason.

Model IDs change: providers retire and rename models regularly. Keep the ID in config and
confirm it against the provider's `/models` endpoint rather than trusting a written note.

### 4.3 API Design Decision: REST vs GraphQL
**Decision**: Start with REST, add GraphQL later if needed
- REST is simpler to implement and debug
- Good for most CRUD operations
- WebSocket can supplement for real-time features
- GraphQL adds complexity, defer unless requirements demand it

### 4.4 State Management Strategy
- **Server-Side State**: Campaign state stored in MongoDB
- **Session State**: In-memory for active sessions
- **Event Log**: Append-only `story_events` for AI context. Full event sourcing with
  replay and undo is out of scope (§0).

## 5. Scalability Considerations

### 5.1 Horizontal Scaling
- Stateless API servers behind load balancer
- MongoDB replica set for read scalability
- No cache tier. Redis is out of scope (§0); revisit only under measured load.

### 5.2 AI Request Optimization
- Cache common AI responses
- Batch similar requests
- Implement rate limiting
- Use streaming responses for long narratives

## 6. Future Enhancements (Post-MVP)
- Multiplayer support with WebSocket (out of scope until a second concurrent player — §0)
- Voice input/output
- Image generation for visual content
- Advanced rules engine (full 5e compatibility)
- Plugin system for custom rules
- Campaign sharing and templates

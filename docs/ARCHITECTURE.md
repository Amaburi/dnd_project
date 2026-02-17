# AI D&D Campaign Manager - Architecture Design

## 1. System Architecture Overview

This project follows **Hexagonal Architecture** (Ports and Adapters) to ensure clean separation of concerns and testability.

### High-Level Architecture Diagram

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           Presentation Layer                               │
│  ┌────────────────────────────────────────────────────────────────────┐   │
│  │  REST API / WebSocket (Gin Framework)                              │   │
│  │  - Campaign CRUD endpoints                                          │   │
│  │  - Game session management                                          │   │
│  │  - AI interaction endpoints                                         │   │
│  │  - Real-time updates via WebSocket                                  │   │
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
│  │ MongoDB           │ │ DeepSeek API     │ │ Configuration             │  │
│  │ - Campaign Store  │ │ - Chat completions│ │ - Environment vars       │  │
│  │ - Character Store │ │ - Embeddings     │ │ - Config loader           │  │
│  │ - Session Store   │ │ - Model params   │ │ - Secret management       │  │
│  └──────────────────┘ └──────────────────┘ └────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────────┘
```

## 2. Component Breakdown

### 2.1 API Layer (Presentation)
- **Framework**: Gin (or Echo) for REST API
- **Protocol**: HTTP REST + optional WebSocket for real-time
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
- **Event Sourcing**: Track all game events for AI context

### 2.4 Infrastructure Layer
- **MongoDB**: Document database for flexible game data
- **DeepSeek**: AI model for natural language generation
- **Config**: Environment-based configuration

## 3. Data Flow

### 3.1 Player Action Flow
```
Player Input → API → AI DM Service → Context Retrieval → DeepSeek API 
→ Narrative Generation → Game Logic (if needed) → Response + State Update
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

### 4.2 Why DeepSeek?
- **Cost-Effective**: Lower cost than GPT-4 for high-volume requests
- **Good Reasoning**: Excellent for narrative and dialogue
- **Fast Inference**: Quick response times for game interactions
- **Large Context Window**: Can maintain comprehensive game state

### 4.3 API Design Decision: REST vs GraphQL
**Decision**: Start with REST, add GraphQL later if needed
- REST is simpler to implement and debug
- Good for most CRUD operations
- WebSocket can supplement for real-time features
- GraphQL adds complexity, defer unless requirements demand it

### 4.4 State Management Strategy
- **Server-Side State**: Campaign state stored in MongoDB
- **Session State**: In-memory cache for active sessions
- **Event Sourcing**: Log all events for AI context and undo capability

## 5. Scalability Considerations

### 5.1 Horizontal Scaling
- Stateless API servers behind load balancer
- MongoDB replica set for read scalability
- Redis cache for session data (future)

### 5.2 AI Request Optimization
- Cache common AI responses
- Batch similar requests
- Implement rate limiting
- Use streaming responses for long narratives

## 6. Future Enhancements (Post-MVP)
- Multiplayer support with WebSocket
- Voice input/output
- Image generation for visual content
- Advanced rules engine (full 5e compatibility)
- Plugin system for custom rules
- Campaign sharing and templates

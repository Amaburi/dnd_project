# AI D&D Campaign Manager

An AI-powered Dungeons & Dragons campaign management system built with Go, MongoDB, and any
OpenAI-compatible AI provider (Groq by default).

## Features

- 🎲 **Dice Rolling System**: Full D&D 5e dice mechanics with advantage/disadvantage
- 🤖 **AI Dungeon Master**: Dynamic narrative generation, provider-agnostic (Groq, DeepSeek, OpenAI, local)
- ⚔️ **Combat System**: Initiative tracking, turns, and combat resolution
- 👥 **Character Management**: Full character sheet support
- 📖 **Story Tracking**: Campaign narrative and session history
- 💾 **MongoDB Storage**: Persistent campaign data

## Quick Start

### Prerequisites

- Go 1.21+
- MongoDB 7.0+
- An API key for an OpenAI-compatible provider ([Groq](https://console.groq.com/keys) by default)

### Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd dnd-project
   ```

2. **Install dependencies**
   ```bash
   make deps
   ```

3. **Configure the application**
   ```bash
   cp configs/config.yaml config.yaml
   # Edit config.yaml with your settings
   ```

4. **Set environment variables**

   Copy `.env.example` to `.env` and fill it in. `make run` sources it automatically,
   because `configs/config.yaml` refers to these as `${VAR}` placeholders.
   ```bash
   cp .env.example .env
   # MONGODB_URI, GROQ_API_KEY
   ```

5. **Run the server**
   ```bash
   make run
   ```

## Project Structure

```
dnd-project/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── api/                     # HTTP API layer
│   ├── application/             # Business logic
│   ├── domain/                  # Core domain models
│   └── infrastructure/         # External integrations
├── pkg/                         # Reusable packages
├── configs/                     # Configuration files
├── docs/                        # Architecture docs
├── test/                        # Tests
├── go.mod
└── Makefile
```

## API Endpoints

### Campaigns
- `POST /api/v1/campaigns` - Create campaign
- `GET /api/v1/campaigns` - List campaigns
- `GET /api/v1/campaigns/:id` - Get campaign
- `PUT /api/v1/campaigns/:id` - Update campaign
- `DELETE /api/v1/campaigns/:id` - Delete campaign

### Characters
- `POST /api/v1/campaigns/:id/characters` - Create character
- `GET /api/v1/campaigns/:id/characters` - List characters (optional `?q=` name filter)
- `GET /api/v1/campaigns/:id/characters/:char_id` - Get character
- `PUT /api/v1/campaigns/:id/characters/:char_id` - Update character
- `DELETE /api/v1/campaigns/:id/characters/:char_id` - Delete character

### Monsters
- `POST /api/v1/campaigns/:id/monsters` - Create statblock
- `GET /api/v1/campaigns/:id/monsters` - List statblocks (`?q=` name, `?min_cr=`/`?max_cr=` range)
- `POST /api/v1/campaigns/:id/monsters/seed` - Copy the SRD catalogue into the campaign
- `GET /api/v1/campaigns/:id/monsters/:monster_id` - Get statblock
- `PUT /api/v1/campaigns/:id/monsters/:monster_id` - Update statblock
- `DELETE /api/v1/campaigns/:id/monsters/:monster_id` - Delete statblock

### Play
- `POST /api/v1/campaigns/:id/actions` - One player turn: parse, resolve, persist, narrate, log

```json
{ "character_id": "...", "input": "I stab the goblin", "scene": "a damp cellar" }
```

### Combat
- `POST /api/v1/campaigns/:id/encounters` - Create encounter (attached to the active session)
- `GET /api/v1/campaigns/:id/encounters` - List encounters
- `GET /api/v1/campaigns/:id/encounters/active` - The fight under way
- `GET|DELETE /api/v1/campaigns/:id/encounters/:encounter_id` - Get or delete
- `GET /api/v1/campaigns/:id/encounters/:encounter_id/stats` - After-action summary
- `POST /api/v1/campaigns/:id/encounters/:encounter_id/combatants` - Add a character or monster
- `POST /api/v1/campaigns/:id/encounters/:encounter_id/initiative` - Roll initiative and begin
- `POST /api/v1/campaigns/:id/encounters/:encounter_id/next-turn` - Advance (ends the fight if decided)
- `POST /api/v1/campaigns/:id/encounters/:encounter_id/end` - End with an outcome

### Sessions
- `POST /api/v1/campaigns/:id/sessions` - Create session (number auto-assigned)
- `GET /api/v1/campaigns/:id/sessions` - List sessions, newest first
- `GET /api/v1/campaigns/:id/sessions/active` - The session currently in progress
- `GET /api/v1/campaigns/:id/sessions/:session_id` - Get session
- `PUT /api/v1/campaigns/:id/sessions/:session_id` - Update session
- `DELETE /api/v1/campaigns/:id/sessions/:session_id` - Delete session and its events
- `POST /api/v1/campaigns/:id/sessions/:session_id/start` - Start (closes any other open session)
- `POST /api/v1/campaigns/:id/sessions/:session_id/end` - End

### Story Events (append-only log)
- `POST /api/v1/campaigns/:id/sessions/:session_id/events` - Append an event
- `GET /api/v1/campaigns/:id/sessions/:session_id/events` - Session log (`?type=` filter)
- `GET /api/v1/campaigns/:id/events/recent` - Recent events plus a rendered context block (`?limit=`)

### Health Check
- `GET /health` - API health status

## Development

### Running Tests
```bash
make test              # All tests
make test-unit         # Unit tests only
make test-integration  # Integration tests
```

### Code Linting
```bash
make lint
```

### Project Roadmap

See [docs/IMPLEMENTATION_ROADMAP.md](docs/IMPLEMENTATION_ROADMAP.md) for the detailed implementation plan.

## Architecture

Key design documents:
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Data Models](docs/DATA_MODELS.md)
- [API Design](docs/API_DESIGN.md)
- [AI Integration](docs/AI_INTEGRATION.md)
- [Game Engine](docs/GAME_ENGINE.md)

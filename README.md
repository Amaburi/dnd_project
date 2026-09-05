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
- 🎯 **Conditions that bite**: advantage from a helpless target, auto-crits, auto-failed saves
- ✨ **Spellcasting**: 85 SRD spells with real mechanics -- cantrip scaling, upcasting, saves, conditions
- 🧠 **Campaign memory**: budgeted history with a rolling summary, so a long campaign still fits
- 📊 **Dice probability**: exact odds and expected damage for encounter balance
- 🛡️ **Production middleware**: request IDs, JSON errors, panic recovery, CORS, per-client rate limiting

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
│   ├── api/                     # HTTP API layer (handlers + middleware)
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

## Browser clients (CORS)

The API refuses nothing, but it only sends `Access-Control-Allow-Origin` to origins you
list. Add your UI's origin to `configs/config.yaml`:

```yaml
cors:
  allowed_origins:
    - "http://localhost:5173"   # Vite
    - "http://localhost:3000"   # Next.js / CRA
  allow_credentials: false      # true only if you send cookies
```

Leave the list empty to disable CORS entirely. `["*"]` reflects whatever asks — fine
locally, wrong in production.

Every response carries an `X-Request-ID`; send your own to have it echoed back, which
makes a browser network log line and a server log line the same incident.

Requests are rate limited per client address. Over budget is `429` with a `Retry-After`
header saying how many seconds to wait:

```yaml
rate_limit:
  requests_per_minute: 60   # 0 disables
  burst: 10
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

### Dice

Not campaign-scoped — a roll is not campaign state. Rolls that matter to the story are
recorded by the action and combat endpoints instead.

- `POST /api/v1/dice/roll` - Roll any expression: `{"expression": "2d6+3"}`
- `POST /api/v1/dice/d20` - Roll a d20: `{"modifier": 5, "mode": "advantage", "dc": 15}`
  (`mode` and `dc` are optional; with a `dc` the response also carries the outcome and the odds)
- `POST /api/v1/dice/damage` - Roll damage: `{"expression": "1d8+3", "critical": true}`
  (a critical doubles the dice, never the modifier)

Every roll returns each individual die, not just the total: a log showing only a 7 hides
that the player rolled 19 and 7 at disadvantage, which was most of what made the moment.

### Dice probability

These roll nothing. The answers are exact, computed by convolution rather than sampled.

- `GET /api/v1/dice/probability?expression=2d6%2B3&target=10` - the full distribution,
  every total with its `probability`, `at_least` and `at_most`
- `POST /api/v1/dice/probability/check` - `{"dc": 15, "modifier": 5, "mode": "advantage"}`
  → chance of passing, and the lowest die face that does
- `POST /api/v1/dice/probability/attack` - `{"target_ac": 16, "modifier": 7, "damage": "1d8+4", "crit_range": 19}`
  → hit, crit, fumble and **expected damage per attack**, which is the number behind
  "how long does this fight last"

`crit_range` defaults to 20; pass 19 or 18 for a Champion.

### Health Check
- `GET /health` - API health status

## Development

### Running Tests

```bash
make test              # offline: no database, no AI provider, nothing skipped
make demo              # one full turn against a stub, free

# Integration tests need a MongoDB. No Docker required -- point it anywhere.
MONGODB_TEST_URI=mongodb://localhost:27017 make test-integration
MONGODB_TEST_URI=mongodb://localhost:27017 make coverage-all
```

Integration tests sit behind a `integration` build tag rather than skipping themselves, so
the default suite never quietly passes over them.

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

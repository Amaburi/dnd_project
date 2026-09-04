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

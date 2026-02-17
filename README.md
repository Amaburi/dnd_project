# AI D&D Campaign Manager

An AI-powered Dungeons & Dragons campaign management system built with Go, MongoDB, and DeepSeek AI.

## Features

- 🎲 **Dice Rolling System**: Full D&D 5e dice mechanics with advantage/disadvantage
- 🤖 **AI Dungeon Master**: Dynamic narrative generation using DeepSeek
- ⚔️ **Combat System**: Initiative tracking, turns, and combat resolution
- 👥 **Character Management**: Full character sheet support
- 📖 **Story Tracking**: Campaign narrative and session history
- 💾 **MongoDB Storage**: Persistent campaign data

## Quick Start

### Prerequisites

- Go 1.21+
- MongoDB 7.0+
- DeepSeek API key

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
   ```bash
   export MONGODB_PASSWORD="your-password"
   export DEEPSEEK_API_KEY="your-api-key"
   ```

5. **Run the server**
   ```bash
   make run
   ```

### Using Docker

```bash
docker-compose up -d
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
- `GET /api/v1/campaigns/:id/characters` - List characters
- `GET /api/v1/campaigns/:id/characters/:char_id` - Get character
- `PUT /api/v1/campaigns/:id/characters/:char_id` - Update character
- `DELETE /api/v1/campaigns/:id/characters/:char_id` - Delete character

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

## License

MIT

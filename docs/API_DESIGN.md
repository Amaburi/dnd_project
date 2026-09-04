# REST API Design

## Base URL
```
https://api.dnd-campaign-manager.com/v1
```

## API Versioning
- **Version**: `v1` in URL path
- **Breaking changes**: Version increment
- **Non-breaking additions**: New endpoints within version

---

## Authentication (Future)
```
Authorization: Bearer <jwt_token>
X-API-Key: <api_key>
```

---

## 1. Campaign Endpoints

### Campaign Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/campaigns` | Create new campaign |
| `GET` | `/campaigns` | List all campaigns |
| `GET` | `/campaigns/:id` | Get campaign details |
| `PUT` | `/campaigns/:id` | Update campaign |
| `DELETE` | `/campaigns/:id` | Delete campaign |

### Campaign Details

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/campaigns/:id/summary` | Get campaign summary |
| `PUT` | `/campaigns/:id/settings` | Update DM settings |
| `PUT` | `/campaigns/:id/ai-personality` | Update AI personality |

#### Create Campaign
```http
POST /campaigns
Content-Type: application/json

{
  "title": "The Lost Kingdom",
  "description": "A campaign in a fallen medieval kingdom",
  "setting": {
    "world_name": "Ethoria",
    "era": "Post-Apocalyptic",
    "magic_level": "Moderate"
  },
  "dm_settings": {
    "tone": "Serious",
    "pacing": "Balanced"
  },
  "ai_personality": {
    "dm_style": "Narrative-focused",
    "detail_level": "Descriptive"
  }
}
```

Response:
```json
{
  "campaign_id": "uuid-v4",
  "status": "created",
  "message": "Campaign created successfully",
  "created_at": "2024-01-15T10:30:00Z"
}
```

---

## 2. Character Endpoints

### Character CRUD

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/campaigns/:id/characters` | Create character |
| `GET` | `/campaigns/:id/characters` | List campaign characters |
| `GET` | `/campaigns/:id/characters/:char_id` | Get character details |
| `PUT` | `/campaigns/:id/characters/:char_id` | Update character |
| `DELETE` | `/campaigns/:id/characters/:char_id` | Delete character |

### AI Character Generation

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/campaigns/:id/characters/generate` | AI generate character background |
| `POST` | `/campaigns/:id/characters/:char_id/generate-backstory` | Generate character backstory |

#### Generate Character Background
```http
POST /campaigns/:id/characters/generate
Content-Type: application/json

{
  "race": "Dwarf",
  "class": "Fighter",
  "level": 5,
  "personality_hints": "Stoic, honorable",
  "backstory_length": "detailed"
}
```

Response:
```json
{
  "character": {
    "name": "Thorin Ironheart",
    "basic_info": {
      "race": "Dwarf",
      "class": "Fighter",
      "background": "Soldier"
    },
    "background_story": {
      "generated_by_ai": true,
      "backstory": "Thorin was once a captain...",
      "personality_traits": ["Stoic", "Honorable"],
      "ideals": ["Protection of the weak"]
    }
  },
  "ability_scores": {
    "strength": 16,
    "dexterity": 12,
    "constitution": 14
  }
}
```

### Character Stats & Inventory

| Method | Endpoint | Description |
|--------|----------|-------------|
| `PUT` | `/campaigns/:id/characters/:char_id/stats` | Update ability scores |
| `PUT` | `/campaigns/:id/characters/:char_id/hp` | Update HP |
| `POST` | `/campaigns/:id/characters/:char_id/inventory` | Add item |
| `PUT` | `/campaigns/:id/characters/:char_id/inventory/:item_id` | Update item |
| `DELETE` | `/campaigns/:id/characters/:char_id/inventory/:item_id` | Remove item |
| `POST` | `/campaigns/:id/characters/:char_id/status` | Add status effect |

---

## 3. Session Endpoints

### Session Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/campaigns/:id/sessions` | Create new session |
| `GET` | `/campaigns/:id/sessions` | List sessions |
| `GET` | `/campaigns/:id/sessions/:session_id` | Get session details |
| `PUT` | `/campaigns/:id/sessions/:session_id` | Update session |
| `PUT` | `/campaigns/:id/sessions/:session_id/status` | Update session status |

#### Session Lifecycle
```http
PUT /campaigns/:id/sessions/:session_id/status
Content-Type: application/json

{
  "status": "in_progress",
  "action": "start_session"
}
```

---

## 4. AI DM Interaction Endpoints

This is the core of your AI DM functionality.

### Send Player Action
```http
POST /campaigns/:id/sessions/:session_id/action
Content-Type: application/json

{
  "character_id": "uuid-v4",
  "player_input": "I want to investigate the mysterious chest in the corner",
  "context": {
    "current_location": "Dungeon Chamber",
    "time_of_day": "Evening",
    "party_status": "All conscious"
  },
  "options": {
    "include_dice": true,
    "narrative_style": "descriptive",
    "max_narrative_length": 500
  }
}
```

Response:
```json
{
  "event_id": "uuid-v4",
  "narrative": {
    "ai_generated_text": "As you approach the ancient chest...",
    "dm_interpretation": "Player investigating chest",
    "dice_results": {
      "roll_type": "ability_check",
      "skill": "Investigation",
      "modifier": 3,
      "roll": 15,
      "total": 18,
      "dc": 12,
      "outcome": "success"
    }
  },
  "game_state_changes": {
    "location_changed": false,
    "discovered": ["Trapped chest - Dex save DC 14"]
  },
  "continuation_options": [
    "Attempt to disarm the trap",
    "Open the chest anyway",
    "Leave it alone"
  ],
  "timestamp": "2024-01-15T10:35:00Z"
}
```

### AI DM Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/campaigns/:id/sessions/:session_id/action` | Send player action to AI DM |
| `POST` | `/campaigns/:id/sessions/:session_id/npc/:npc_id/dialogue` | Initiate NPC conversation |
| `POST` | `/campaigns/:id/sessions/:session_id/describe` | Request environment description |
| `POST` | `/campaigns/:id/sessions/:session_id/generate-encounter` | Generate random encounter |
| `PUT` | `/campaigns/:id/sessions/:session_id/story-thread` | Update story direction |

### Continue Story
```http
POST /campaigns/:id/sessions/:session_id/continue
Content-Type: application/json

{
  "choice": "Attempt to disarm the trap",
  "character_id": "uuid-v4"
}
```

---

## 5. Combat Endpoints

### Combat Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/campaigns/:id/sessions/:session_id/combat` | Start combat encounter |
| `GET` | `/campaigns/:id/sessions/:session_id/combat` | Get current combat state |
| `PUT` | `/campaigns/:id/sessions/:session_id/combat` | Update combat settings |
| `DELETE` | `/campaigns/:id/sessions/:session_id/combat` | End combat encounter |

### Combat Actions

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/campaigns/:id/sessions/:session_id/combat/initiative` | Roll initiative |
| `POST` | `/campaigns/:id/sessions/:session_id/combat/turn` | Take turn |
| `POST` | `/campaigns/:id/sessions/:session_id/combat/action` | Take combat action |
| `PUT` | `/campaigns/:id/sessions/:session_id/combat/combatant/:id` | Update combatant |
| `POST` | `/campaigns/:id/sessions/:session_id/combat/attack` | Make attack roll |

#### Combat Action Example
```http
POST /campaigns/:id/sessions/:session_id/combat/action
Content-Type: application/json

{
  "combatant_id": "uuid-v4",
  "action_type": "attack",
  "target_id": "dragon_id",
  "weapon": "longsword",
  "style": "standard",
  "roll_mode": "normal"
}
```

Response:
```json
{
  "event_id": "uuid-v4",
  "action_result": {
    "roll": 17,
    "natural": 17,
    "modifier": 5,
    "total": 22,
    "hit": true,
    "critical": false,
    "damage": {
      "type": "slashing",
      "roll": "1d8+5",
      "total": 13
    }
  },
  "narrative": "Thorin's sword strikes true, biting into the dragon's scales!",
  "target_status": {
    "current_hp": 87,
    "conditions": []
  }
}
```

### Combatant Management
```http
PUT /campaigns/:id/sessions/:session_id/combat/combatant/:id
Content-Type: application/json

{
  "action": "apply_damage",
  "damage": 13,
  "damage_type": "slashing"
}
```

---

## 6. Dice Endpoints

### Dice Rolling

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/dice/roll` | Roll dice (standalone) |
| `POST` | `/campaigns/:id/sessions/:session_id/dice/roll` | Roll dice in session |
| `POST` | `/dice/roll/advantage` | Roll with advantage |
| `POST` | `/dice/roll/disadvantage` | Roll with disadvantage |

#### Dice Roll Request
```http
POST /dice/roll
Content-Type: application/json

{
  "dice_expression": "2d20+5",
  "roll_type": "ability_check",
  "ability": "stealth",
  "context": {
    "character_id": "uuid-v4",
    "circumstances": "Moving silently through the shadows"
  },
  "options": {
    "include_narrative": true,
    "visualize": true
  }
}
```

Response:
```json
{
  "roll_id": "uuid-v4",
  "dice_expression": "2d20+5",
  "results": {
    "dice": [
      { "die": 20, "result": 18 },
      { "die": 20, "result": 12 }
    ],
    "modifier": 5,
    "total": 23,
    "natural_rolls": [18, 12],
    "highest": 18,
    "lowest": 12
  },
  "outcome": {
    "success": true,
    "critical_hit": false,
    "dc": 15
  },
  "narrative": "You move through the shadows like a ghost..."
}
```

---

## 7. Story & Narrative Endpoints

### Story Events

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/campaigns/:id/sessions/:session_id/events` | Get session events |
| `GET` | `/campaigns/:id/story` | Get campaign story timeline |
| `GET` | `/campaigns/:id/events/:event_id` | Get specific event |
| `PUT` | `/campaigns/:id/story-thread` | Update story direction |

### Story Generation
```http
POST /campaigns/:id/generate-story-element
Content-Type: application/json

{
  "element_type": "npc | location | plot_twist | treasure",
  "parameters": {
    "theme": "mystery",
    "tone": "eerie",
    "connection_to_existing": "related to ancient ruins"
  }
}
```

---

## 8. Error Responses

All error responses follow this format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid campaign ID format",
    "details": [
      {
        "field": "campaign_id",
        "issue": "Must be a valid UUID"
      }
    ]
  },
  "request_id": "uuid-v4",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `VALIDATION_ERROR` | 400 | Invalid input data |
| `NOT_FOUND` | 404 | Resource not found |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `RATE_LIMITED` | 429 | Too many requests |
| `AI_SERVICE_ERROR` | 503 | AI service unavailable |
| `INTERNAL_ERROR` | 500 | Server error |

---

## 9. Rate Limiting

| Tier | Requests/minute | AI Requests/hour |
|------|-----------------|-----------------|
| Free | 60 | 100 |
| Pro | 200 | 1000 |
| Enterprise | 1000 | Unlimited |

Headers:
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1640995200
```

---

## 10. API Versioning Strategy

### Version Lifecycle
1. **Active**: Current version, fully supported
2. **Deprecated**: Will be removed in future, notify users
3. **Sunset**: No longer available

### Version Headers
```
Accept: application/vnd.dnd-campaign.v1+json
```

---

## 11. WebSocket for Real-Time (OUT OF SCOPE)

> **Cut on 2026-09-04.** Real-time only earns its cost with a second concurrent player;
> see ARCHITECTURE.md §0. The sketch below is kept for whenever that changes. If a single
> long narration ever feels slow, stream it with SSE over the existing REST endpoint
> instead — `ai.Service.StreamNarrative` already produces that.

```javascript
// Connect to game session
const ws = new WebSocket('wss://api.dnd-campaign.com/v1/ws/session/:session_id');

// Send action
ws.send(JSON.stringify({
  type: 'player_action',
  character_id: 'uuid-v4',
  message: 'I attack the goblin!'
}));

// Receive updates
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  // Handle: narrative_update, dice_result, combat_update, etc.
};
```

---

## 12. Quick Reference Summary

### Endpoints by Category

**Campaigns**: `POST /campaigns`, `GET /campaigns`, `GET /campaigns/:id`, `PUT /campaigns/:id`, `DELETE /campaigns/:id`

**Characters**: `POST /campaigns/:id/characters`, `GET /campaigns/:id/characters`, `PUT /campaigns/:id/characters/:char_id`

**AI DM**: `POST /campaigns/:id/sessions/:session_id/action`, `POST /campaigns/:id/sessions/:session_id/npc/:id/dialogue`

**Combat**: `POST /campaigns/:id/sessions/:session_id/combat`, `POST /campaigns/:id/sessions/:session_id/combat/action`

**Dice**: `POST /dice/roll`, `POST /campaigns/:id/sessions/:session_id/dice/roll`

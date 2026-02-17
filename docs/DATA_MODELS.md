# MongoDB Data Models

## Collection Overview

| Collection | Purpose | Index Strategy |
|------------|---------|----------------|
| campaigns | Main campaign metadata and settings | campaign_id, user_id, created_at |
| characters | Player characters and NPCs | campaign_id, character_id |
| sessions | Game sessions within campaigns | campaign_id, session_date |
| story_events | Narrative events and AI interactions | campaign_id, timestamp |
| combat_encounters | Combat encounters and state | campaign_id, session_id, status |
| game_logs | Raw game logs for AI context | campaign_id, session_id, timestamp |

---

## 1. Campaign Collection

### Document Structure
```json
{
  "_id": "ObjectId",
  "campaign_id": "uuid-v4",
  "title": "The Lost Kingdom",
  "description": "A campaign set in a fallen medieval kingdom",
  "setting": {
    "world_name": "Ethoria",
    "era": "Post-Apocalyptic",
    "magic_level": "Moderate",
    "technology_level": "Medieval",
    "key_locations": [
      {
        "name": "Ironhold",
        "description": "Ancient dwarven fortress",
        "coordinates": {"x": 123, "y": 456}
      }
    ],
    "factions": [
      {
        "name": "The Silver Hand",
        "description": "Paladin order dedicated to restoring the kingdom",
        "alignment": "Lawful Good"
      }
    ]
  },
  "dm_settings": {
    "tone": "Serious with moments of levity",
    "pacing": "Balanced exploration and combat",
    "themes": ["Redemption", "Discovery", "Political Intrigue"]
  },
  "ai_personality": {
    "dm_style": "Narrative-focused",
    "narrative_voice": "Third-person omniscient",
    "humor_level": "Light",
    "detail_level": "Descriptive"
  },
  "status": "active",
  "created_at": "ISODate",
  "updated_at": "ISODate",
  "created_by": "user_id",
  "current_session_id": "session_id",
  "story_progress": {
    "main_quest_stage": 3,
    "completed_arcs": ["The Prologue", "Journey to Ironhold"],
    "active_plot_threads": ["The Mysterious Artifact"]
  }
}
```

### Indexes
```javascript
db.campaigns.createIndex({ "campaign_id": 1 }, { unique: true })
db.campaigns.createIndex({ "created_by": 1 })
db.campaigns.createIndex({ "status": 1 })
db.campaigns.createIndex({ "created_at": -1 })
```

---

## 2. Character Collection

### Document Structure
```json
{
  "_id": "ObjectId",
  "character_id": "uuid-v4",
  "campaign_id": "campaign_id",
  "type": "player | npc",
  "name": "Thorin Ironheart",
  "player_name": "John Doe",
  
  "basic_info": {
    "race": "Dwarf",
    "class": "Fighter",
    "background": "Soldier",
    "level": 5,
    "experience_points": 6500,
    "alignment": "Lawful Good"
  },
  
  "ability_scores": {
    "strength": 16,
    "dexterity": 12,
    "constitution": 14,
    "intelligence": 10,
    "wisdom": 13,
    "charisma": 8
  },
  
  "derived_stats": {
    "hit_points": {
      "current": 52,
      "maximum": 52,
      "temporary": 0
    },
    "armor_class": 16,
    "proficiency_bonus": 3,
    "initiative_modifier": 1,
    "speed": 25,
    "passive_perception": 11
  },
  
  "skills": {
    "acrobatics": false,
    "animal_handling": false,
    "arcana": false,
    "athletics": true,
    "deception": false,
    "history": false,
    "insight": false,
    "intimidation": false,
    "investigation": false,
    "medicine": false,
    "nature": false,
    "perception": true,
    "performance": false,
    "persuasion": false,
    "religion": false,
    "sleight_of_hand": false,
    "stealth": false,
    "survival": false
  },
  
  "saving_throws": {
    "strength": true,
    "dexterity": false,
    "constitution": true,
    "intelligence": false,
    "wisdom": false,
    "charisma": false
  },
  
  "inventory": [
    {
      "item_id": "item_uuid",
      "name": "Longsword",
      "quantity": 1,
      "weight": 3.0,
      "equipped": true,
      "description": "A masterwork steel longsword"
    }
  ],
  
  "equipment": {
    "armor": null,
    "shield": null,
    "weapons": [],
    "accessories": []
  },
  
  "features_and_abilities": [
    {
      "name": "Dwarven Resilience",
      "description": "Advantage on poison saves",
      "uses_per_day": null
    }
  ],
  
  "spells": {
    "spellcasting_ability": "wisdom",
    "spell_save_dc": 12,
    "spell_attack_modifier": 4,
    "cantrips_known": ["Guidance", "Light"],
    "spells_known": {
      "1st": ["Cure Wounds", "Shield of Faith"],
      "2nd": ["Lesser Restoration"]
    }
  },
  
  "background_story": {
    "generated_by_ai": true,
    "backstory": "Thorin was once a captain in the dwarven guard...",
    "personality_traits": ["Stubborn", "Honorable"],
    "ideals": ["Protection of the weak"],
    "bonds": ["His fallen comrades"],
    "flaws": ["Trusts too easily"]
  },
  
  "status_effects": [],
  
  "conditions": ["normal"],
  
  "ai_metadata": {
    "personality": "Stoic and honorable",
    "speech_pattern": "Formal, with dwarven accent",
    "motivation": "Redeem his fallen order",
    "relationships": {}
  },
  
  "created_at": "ISODate",
  "updated_at": "ISODate"
}
```

### Indexes
```javascript
db.characters.createIndex({ "character_id": 1 }, { unique: true })
db.characters.createIndex({ "campaign_id": 1 })
db.characters.createIndex({ "type": 1 })
db.characters.createIndex({ "basic_info.race": 1, "basic_info.class": 1 })
```

---

## 3. Session Collection

### Document Structure
```json
{
  "_id": "ObjectId",
  "session_id": "uuid-v4",
  "campaign_id": "campaign_id",
  "session_number": 7,
  "title": "The Dragon's Lair",
  
  "date": {
    "planned": "ISODate",
    "actual_start": "ISODate",
    "actual_end": "ISODate"
  },
  
  "participants": [
    {
      "character_id": "character_id",
      "character_name": "Thorin Ironheart",
      "player_name": "John Doe",
      "attendance": "present",
      "joined_at": "ISODate",
      "left_at": null
    }
  ],
  
  "location": {
    "current_location": "Dragon's Peak",
    "coordinates": null,
    "environment": "Mountain cave"
  },
  
  "session_summary": {
    "narrative_summary": "The party ascended the mountain...",
    "key_events": [
      "Discovered dragon tracks",
      "Rescued captured merchant",
      "Found ancient map"
    ],
    "treasure_found": [],
    "deaths": []
  },
  
  "dice_rolls_summary": {
    "total_rolls": 47,
    "natural_20s": 3,
    "natural_1s": 1,
    "average_roll": 12.4
  },
  
  "combat_encounters": ["encounter_id_1", "encounter_id_2"],
  
  "ai_interactions": {
    "total_prompts": 23,
    "total_tokens_used": 15000,
    "cost_estimate": 0.05
  },
  
  "status": "completed | in_progress | scheduled",
  
  "notes": "Great session, the dragon encounter was intense",
  
  "created_at": "ISODate"
}
```

### Indexes
```javascript
db.sessions.createIndex({ "session_id": 1 }, { unique: true })
db.sessions.createIndex({ "campaign_id": 1, "session_number": -1 })
db.sessions.createIndex({ "status": 1, "date.planned": 1 })
```

---

## 4. Story Event Collection (AI Interactions)

### Document Structure
```json
{
  "_id": "ObjectId",
  "event_id": "uuid-v4",
  "campaign_id": "campaign_id",
  "session_id": "session_id",
  "event_type": "narrative | dialogue | combat_action | dice_roll | exploration",
  
  "sequence_number": 42,
  
  "trigger": {
    "type": "player_action | ai_initiated | dice_result | time_passed",
    "player_input": "I want to sneak into the castle",
    "intent": "infiltration",
    "target": "Castle entrance"
  },
  
  "ai_context": {
    "prompt_tokens": 150,
    "completion_tokens": 500,
    "model": "deepseek-chat",
    "temperature": 0.7,
    "system_prompt_version": "v2.1"
  },
  
  "narrative": {
    "ai_generated_text": "As you approach the castle walls...",
    "dm_interpretation": "Player attempting Stealth check",
    "dice_results": {
      "roll_type": "ability_check",
      "skill": "Stealth",
      "modifier": 5,
      "roll": 14,
      "total": 19,
      "natural_roll": 14,
      "dc": 12,
      "outcome": "success"
    }
  },
  
  "game_state_changes": {
    "location_changed": true,
    "new_location": "Castle Courtyard",
    "characters_involved": ["character_id"],
    "conditions_applied": [],
    "items_gained": [],
    "hp_changes": []
  },
  
  "consequences": {
    "immediate": ["Successful infiltration"],
    "potential_future": ["Guard alerted", "Time消耗"]
  },
  
  "player_reactions": [],
  
  "metadata": {
    "processing_time_ms": 1200,
    "cost_usd": 0.002
  },
  
  "timestamp": "ISODate"
}
```

### Indexes
```javascript
db.story_events.createIndex({ "event_id": 1 }, { unique: true })
db.story_events.createIndex({ "campaign_id": 1, "session_id": 1, "sequence_number": 1 })
db.story_events.createIndex({ "campaign_id": 1, "timestamp": -1 })
db.story_events.createIndex({ "event_type": 1 })
```

---

## 5. Combat Encounter Collection

### Document Structure
```json
{
  "_id": "ObjectId",
  "encounter_id": "uuid-v4",
  "campaign_id": "campaign_id",
  "session_id": "session_id",
  
  "encounter_name": "Dragon Combat",
  "description": "Encounter with Young Red Dragon",
  
  "encounter_type": "combat | social | environmental",
  "status": "active | completed | aborted",
  
  "combatants": [
    {
      "combatant_id": "uuid-v4",
      "character_id": "character_id",
      "type": "player | enemy | npc",
      "name": "Thorin Ironheart",
      "initiative": 14,
      "initiative_modifier": 1,
      "turn_order": 1,
      "current_hp": 52,
      "max_hp": 52,
      "temporary_hp": 0,
      "armor_class": 16,
      "status": "active | unconscious | dead",
      "conditions": [],
      "actions_taken": 0,
      "bonus_actions_taken": 0,
      "reactions_remaining": 1,
      "movement_remaining": 30,
      "concentrating_on": null,
      "death_saves": {
        "successes": 0,
        "failures": 0
      }
    }
  ],
  
  "combat_state": {
    "round": 3,
    "turn": 2,
    "combat_started_at": "ISODate",
    "combat_ended_at": null,
    "duration_rounds": 8,
    "environment_conditions": []
  },
  
  "turn_history": [
    {
      "round": 1,
      "turn": 1,
      "combatant_id": "combatant_id",
      "actions": [
        {
          "action_type": "attack | spell | item | dash | dodge | help",
          "description": "Attacks with longsword",
          "target": "Dragon",
          "roll_result": {
            "roll": 18,
            "natural": 18,
            "modifier": 5,
            "total": 23,
            "hit": true,
            "critical": true
          },
          "damage": {
            "damage_type": "slashing",
            "damage_roll": "8+5",
            "damage_total": 13,
            "damage_modifier": "standard",
            "hit_points": 13
          }
        }
      ],
      "movement": {
        "distance": 30,
        "from": "Entrance",
        "to": "Flanking position"
      },
      "bonus_actions": [],
      "reactions": [],
      "free_actions": [],
      "notes": "Critically hit the dragon!"
    }
  ],
  
  "damage_log": [
    {
      "attacker": "Thorin Ironheart",
      "target": "Young Red Dragon",
      "damage": 13,
      "damage_type": "slashing",
      "round": 1,
      "timestamp": "ISODate"
    }
  ],
  
  "narrative_log": [
    {
      "text": "Thorin charges forward, sword gleaming...",
      "round": 1,
      "timestamp": "ISODate"
    }
  ],
  
  "victory_conditions": {
    "type": "elimination | capture | escape | negotiation",
    "criteria": "Defeat or flee the dragon",
    "outcome": null
  },
  
  "treasure": [],
  
  "ai_summary": {
    "narrative_summary": "Epic battle with the dragon...",
    "highlights": ["Critical hit", "Breath weapon dodge"],
    "pivot_moments": []
  },
  
  "created_at": "ISODate",
  "updated_at": "ISODate"
}
```

### Indexes
```javascript
db.combat_encounters.createIndex({ "encounter_id": 1 }, { unique: true })
db.combat_encounters.createIndex({ "campaign_id": 1, "session_id": 1 })
db.combat_encounters.createIndex({ "status": 1 })
```

---

## 6. Game Log Collection

### Document Structure
```json
{
  "_id": "ObjectId",
  "log_id": "uuid-v4",
  "campaign_id": "campaign_id",
  "session_id": "session_id",
  
  "timestamp": "ISODate",
  "log_type": "chat | dice | system | combat | exploration",
  
  "source": {
    "type": "player | dm | system | ai",
    "character_id": "character_id",
    "player_name": "John Doe"
  },
  
  "content": {
    "message_type": "text | action | dice_roll | command",
    "raw_message": "I attack the goblin with my sword!",
    "parsed_action": {
      "intent": "attack",
      "target": "goblin",
      "method": "melee_weapon"
    }
  },
  
  "ai_processing": {
    "was_processed_by_ai": true,
    "interpretation": "Player attacks goblin - generating combat response",
    "ai_response_id": "story_event_id"
  },
  
  "metadata": {
    "client_time": "ISODate",
    "server_time": "ISODate",
    "client_info": {
      "platform": "web",
      "version": "1.0.0"
    }
  }
}
```

### Indexes
```javascript
db.game_logs.createIndex({ "log_id": 1 }, { unique: true })
db.game_logs.createIndex({ "campaign_id": 1, "session_id": 1, "timestamp": 1 })
db.game_logs.createIndex({ "campaign_id": 1, "log_type": 1 })
```

---

## 7. Reference: Dice Roll Schema

```json
{
  "roll_id": "uuid-v4",
  "campaign_id": "campaign_id",
  "session_id": "session_id",
  
  "roll_type": "ability_check | saving_throw | attack_roll | damage_roll | skill_check",
  
  "dice_expression": "2d20 + 5",
  "dice_results": {
    "dice": [
      { "die": 20, "result": 15 },
      { "die": 20, "result": 18 }
    ],
    "modifier": 5,
    "total": 23,
    "natural_rolls": [15, 18]
  },
  
  "context": {
    "ability": "strength",
    "skill": null,
    "weapon": "Longsword",
    "target": "Dragon",
    "circumstances": "Advantage from flanking"
  },
  
  "outcome": {
    "success": true,
    "failure": false,
    "critical_hit": false,
    "critical_fail": false,
    "dc": 15,
    "notes": "Hit! Critical threat!"
  },
  
  "ai_generated_narrative": "Your sword connects with a thunderous blow!",
  
  "timestamp": "ISODate"
}
```

---

## 8. Data Relationships

```
Campaign (1) ───> (Many) Characters
Campaign (1) ───> (Many) Sessions
Session (1) ───> (Many) Story Events
Session (1) ───> (Many) Combat Encounters
Session (1) ───> (Many) Game Logs
Character (1) ───> (Many) Combatants (in different encounters)
Story Event (Many) ───> (1) AI Response
```

---

## 9. MongoDB Best Practices for This Project

1. **Use ObjectId for _id** but maintain uuid-v4 for business logic IDs
2. **Index strategically** on campaign_id for all collections
3. **Use TTL indexes** for temporary data (AI request logs)
4. **Embed vs Reference**:
   - Embed: Character inventory, equipment
   - Reference: Campaign → Characters, Session → Combat Encounters
5. **Use upsert patterns** for atomic updates to game state
6. **Implement soft deletes** with `is_deleted` flag for audit trail
7. **Use transactions** for multi-document updates (combat state changes)

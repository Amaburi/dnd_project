# MongoDB Setup & Migration Guide

## Overview

This guide covers MongoDB installation, configuration, database design, and migration strategies for the AI D&D Campaign Manager.

---

## 1. MongoDB Installation

### Local Development (macOS)

```bash
# Using Homebrew
brew install mongodb-community@7.0

# Start MongoDB as a background service
brew services start mongodb-community@7.0

# Verify installation
mongosh --version

# Connect to local instance
mongosh "mongodb://localhost:27017"
```

### Docker Setup

```bash
# Run MongoDB container
docker run -d \
  --name mongodb \
  -p 27017:27017 \
  -v mongodb_data:/data/db \
  -e MONGO_INITDB_ROOT_USERNAME=admin \
  -e MONGO_INITDB_ROOT_PASSWORD=${MONGO_PASSWORD} \
  mongo:7

# Connect using mongosh
docker exec -it mongodb mongosh -u admin -p ${MONGO_PASSWORD} --authenticationDatabase admin
```

### Production Deployment

```bash
# MongoDB Atlas (Cloud)
# 1. Create account at https://cloud.mongodb.com
# 2. Create cluster (M10+ recommended)
# 3. Create database user
# 4. Whitelist IP addresses
# 5. Get connection string
```

---

## 2. Database Configuration

### Connection Setup

```go
// internal/infrastructure/database/mongodb/client.go

package mongodb

import (
    "context"
    "fmt"
    "time"
    
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/mongo/readpref"
)

type Client struct {
    client   *mongo.Client
    database *mongo.Database
    config   *Config
}

type Config struct {
    URI          string
    Database     string
    Username     string
    Password     string
    AuthSource   string
    MaxPoolSize  uint64
    MinPoolSize  uint64
    ConnectTimeout time.Duration
}

func NewClient(config Config) (*Client, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Build connection URI
    uri := fmt.Sprintf(
        "mongodb://%s:%s@%s/%s?authSource=%s",
        config.Username,
        config.Password,
        config.URI,
        config.Database,
        config.AuthSource,
    )
    
    // Configure client options
    clientOptions := options.Client().
        ApplyURI(uri).
        SetMaxPoolSize(config.MaxPoolSize).
        SetMinPoolSize(config.MinPoolSize).
        SetConnectTimeout(config.ConnectTimeout)
    
    // Connect to MongoDB
    client, err := mongo.Connect(ctx, clientOptions)
    if err != nil {
        return nil, fmt.Errorf("failed to connect: %w", err)
    }
    
    // Verify connection
    if err := client.Ping(ctx, readpref.Primary()); err != nil {
        return nil, fmt.Errorf("failed to ping: %w", err)
    }
    
    return &Client{
        client:   client,
        database: client.Database(config.Database),
        config:   &config,
    }, nil
}

func (c *Client) Database() *mongo.Database {
    return c.database
}

func (c *Client) Collection(name string) *mongo.Collection {
    return c.database.Collection(name)
}

func (c *Client) Close(ctx context.Context) error {
    return c.client.Disconnect(ctx)
}
```

### Environment Configuration

```go
// internal/infrastructure/config/config.go

package config

import (
    "github.com/spf13/viper"
)

type MongoConfig struct {
    URI           string `mapstructure:"MONGODB_URI"`
    Database      string `mapstructure:"MONGODB_DATABASE"`
    Username      string `mapstructure:"MONGODB_USERNAME"`
    Password      string `mapstructure:"MONGODB_PASSWORD"`
    AuthSource    string `mapstructure:"MONGODB_AUTH_SOURCE"`
    MaxPoolSize   uint64 `mapstructure:"MONGODB_MAX_POOL_SIZE"`
    MinPoolSize   uint64 `mapstructure:"MONGODB_MIN_POOL_SIZE"`
}

func LoadMongoConfig() *MongoConfig {
    return &MongoConfig{
        URI:          viper.GetString("MONGODB_URI"),
        Database:     viper.GetString("MONGODB_DATABASE"),
        Username:     viper.GetString("MONGODB_USERNAME"),
        Password:     viper.GetString("MONGODB_PASSWORD"),
        AuthSource:   viper.GetString("MONGODB_AUTH_SOURCE"),
        MaxPoolSize:  viper.GetUint64("MONGODB_MAX_POOL_SIZE"),
        MinPoolSize:  viper.GetUint64("MONGODB_MIN_POOL_SIZE"),
    }
}
```

---

## 3. Collection Setup

### Initialize Collections

```go
// internal/infrastructure/database/mongodb/collections.go

package mongodb

import (
    "context"
    "fmt"
    
    "go.mongodb.org/mongo-driver/bson"
)

type CollectionName string

const (
    Campaigns       CollectionName = "campaigns"
    Characters      CollectionName = "characters"
    Sessions        CollectionName = "sessions"
    StoryEvents     CollectionName = "story_events"
    CombatEncounters CollectionName = "combat_encounters"
    GameLogs        CollectionName = "game_logs"
)

func (c *Client) InitializeCollections(ctx context.Context) error {
    collections := []CollectionName{
        Campaigns,
        Characters,
        Sessions,
        StoryEvents,
        CombatEncounters,
        GameLogs,
    }
    
    for _, name := range collections {
        if err := c.createCollection(ctx, name); err != nil {
            return fmt.Errorf("failed to create collection %s: %w", name, err)
        }
    }
    
    return nil
}

func (c *Client) createCollection(ctx context.Context, name CollectionName) error {
    // Check if collection exists
    collections, err := c.database.ListCollectionNames(ctx, bson.M{"name": string(name)})
    if err != nil {
        return err
    }
    
    if len(collections) > 0 {
        return nil // Collection already exists
    }
    
    // Create collection
    _, err = c.database.CreateCollection(ctx, string(name))
    return err
}
```

---

## 4. Indexes

### Create Indexes for Performance

```go
// internal/infrastructure/database/mongodb/indexes.go

package mongodb

import (
    "context"
    "fmt"
    
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func (c *Client) CreateIndexes(ctx context.Context) error {
    // Campaigns indexes
    if err := c.createCampaignIndexes(ctx); err != nil {
        return fmt.Errorf("failed to create campaign indexes: %w", err)
    }
    
    // Characters indexes
    if err := c.createCharacterIndexes(ctx); err != nil {
        return fmt.Errorf("failed to create character indexes: %w", err)
    }
    
    // Sessions indexes
    if err := c.createSessionIndexes(ctx); err != nil {
        return fmt.Errorf("failed to create session indexes: %w", err)
    }
    
    // Story events indexes
    if err := c.createStoryEventIndexes(ctx); err != nil {
        return fmt.Errorf("failed to create story event indexes: %w", err)
    }
    
    return nil
}

func (c *Client) createCampaignIndexes(ctx context.Context) error {
    coll := c.Collection(Campaigns)
    
    indexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "campaign_id", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{Key: "created_by", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "status", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "created_at", Value: -1}},
        },
        {
            Keys: bson.D{
                {Key: "title", Value: "text"},
                {Key: "description", Value: "text"},
            },
        },
    }
    
    _, err := coll.Indexes().CreateMany(ctx, indexes)
    return err
}

func (c *Client) createCharacterIndexes(ctx context.Context) error {
    coll := c.Collection(Characters)
    
    indexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "character_id", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{Key: "campaign_id", Value: 1}},
        },
        {
            Keys: bson.D{{Key: "type", Value: 1}},
        },
        {
            Keys: bson.D{
                {Key: "basic_info.race", Value: 1},
                {Key: "basic_info.class", Value: 1},
            },
        },
    }
    
    _, err := coll.Indexes().CreateMany(ctx, indexes)
    return err
}

func (c *Client) createSessionIndexes(ctx context.Context) error {
    coll := c.Collection(Sessions)
    
    indexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "session_id", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{
                {Key: "campaign_id", Value: 1},
                {Key: "session_number", Value: -1},
            },
        },
        {
            Keys: bson.D{
                {Key: "status", Value: 1},
                {Key: "date.planned", Value: 1},
            },
        },
    }
    
    _, err := coll.Indexes().CreateMany(ctx, indexes)
    return err
}

func (c *Client) createStoryEventIndexes(ctx context.Context) error {
    coll := c.Collection(StoryEvents)
    
    indexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{Key: "event_id", Value: 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{
                {Key: "campaign_id", Value: 1},
                {Key: "session_id", Value: 1},
                {Key: "sequence_number", Value: 1},
            },
        },
        {
            Keys: bson.D{
                {Key: "campaign_id", Value: 1},
                {Key: "timestamp", Value: -1},
            },
        },
        {
            Keys: bson.D{{Key: "event_type", Value: 1}},
        },
    }
    
    _, err := coll.Indexes().CreateMany(ctx, indexes)
    return err
}
```

---

## 5. Data Seeding

### Seed Initial Data

```go
// internal/infrastructure/database/mongodb/seed.go

package mongodb

import (
    "context"
    "fmt"
    
    "go.mongodb.org/mongo-driver/bson"
)

func (c *Client) SeedData(ctx context.Context) error {
    // Seed sample campaigns
    if err := c.seedSampleCampaigns(ctx); err != nil {
        return fmt.Errorf("failed to seed campaigns: %w", err)
    }
    
    // Seed sample characters
    if err := c.seedSampleCharacters(ctx); err != nil {
        return fmt.Errorf("failed to seed characters: %w", err)
    }
    
    return nil
}

func (c *Client) seedSampleCampaigns(ctx context.Context) error {
    coll := c.Collection(Campaigns)
    
    campaigns := []interface{}{
        bson.M{
            "campaign_id": "campaign-001",
            "title": "The Lost Kingdom",
            "description": "A campaign set in a fallen medieval kingdom",
            "setting": bson.M{
                "world_name": "Ethoria",
                "era": "Post-Apocalyptic",
                "magic_level": "Moderate",
            },
            "status": "active",
            "created_at": "2024-01-01T00:00:00Z",
            "updated_at": "2024-01-01T00:00:00Z",
        },
        bson.M{
            "campaign_id": "campaign-002",
            "title": "Shadow of the Spire",
            "description": "Dark fantasy adventure in a mysterious tower",
            "setting": bson.M{
                "world_name": "Midgard",
                "era": "Ancient",
                "magic_level": "High",
            },
            "status": "active",
            "created_at": "2024-01-15T00:00:00Z",
            "updated_at": "2024-01-15T00:00:00Z",
        },
    }
    
    _, err := coll.InsertMany(ctx, campaigns)
    return err
}

func (c *Client) seedSampleCharacters(ctx context.Context) error {
    coll := c.Collection(Characters)
    
    characters := []interface{}{
        bson.M{
            "character_id": "char-001",
            "campaign_id": "campaign-001",
            "type": "player",
            "name": "Thorin Ironheart",
            "basic_info": bson.M{
                "race": "Dwarf",
                "class": "Fighter",
                "level": 5,
            },
            "ability_scores": bson.M{
                "strength": 16,
                "dexterity": 12,
                "constitution": 14,
                "intelligence": 10,
                "wisdom": 13,
                "charisma": 8,
            },
            "created_at": "2024-01-01T00:00:00Z",
        },
    }
    
    _, err := coll.InsertMany(ctx, characters)
    return err
}
```

---

## 6. Migrations

### Migration Runner

```go
// internal/infrastructure/database/mongodb/migration.go

package mongodb

import (
    "context"
    "fmt"
    "sort"
    
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
)

type Migration struct {
    Version     int
    Name        string
    Up          func(ctx context.Context, db *mongo.Database) error
    Down        func(ctx context.Context, db *mongo.Database) error
}

type MigrationHistory struct {
    Version   int    `bson:"version"`
    Name      string `bson:"name"`
    AppliedAt string `bson:"applied_at"`
}

var migrations = []Migration{
    {
        Version: 1,
        Name:    "Initial schema",
        Up:      migrateV1Up,
        Down:    migrateV1Down,
    },
    {
        Version: 2,
        Name:    "Add campaign settings",
        Up:      migrateV2Up,
        Down:    migrateV2Down,
    },
}

func (c *Client) RunMigrations(ctx context.Context) error {
    // Ensure migrations collection exists
    if err := c.ensureMigrationCollection(ctx); err != nil {
        return err
    }
    
    // Get applied migrations
    applied, err := c.getAppliedMigrations(ctx)
    if err != nil {
        return fmt.Errorf("failed to get applied migrations: %w", err)
    }
    
    // Sort migrations by version
    sort.Slice(migrations, func(i, j int) bool {
        return migrations[i].Version < migrations[j].Version
    })
    
    // Run pending migrations
    for _, m := range migrations {
        if !applied.contains(m.Version) {
            fmt.Printf("Running migration %d: %s\n", m.Version, m.Name)
            
            if err := m.Up(ctx, c.database); err != nil {
                return fmt.Errorf("migration %d failed: %w", m.Version, err)
            }
            
            if err := c.recordMigration(ctx, m); err != nil {
                return fmt.Errorf("failed to record migration %d: %w", m.Version, err)
            }
        }
    }
    
    return nil
}

func (c *Client) ensureMigrationCollection(ctx context.Context) error {
    collections, err := c.database.ListCollectionNames(ctx, bson.M{"name": "migrations"})
    if err != nil {
        return err
    }
    
    if len(collections) == 0 {
        _, err := c.database.CreateCollection(ctx, "migrations")
        return err
    }
    
    return nil
}

func (c *Client) getAppliedMigrations(ctx context.Context) ([]MigrationHistory, error) {
    coll := c.database.Collection("migrations")
    
    cursor, err := coll.Find(ctx, bson.M{})
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)
    
    var history []MigrationHistory
    if err := cursor.All(ctx, &history); err != nil {
        return nil, err
    }
    
    return history, nil
}

func (c *Client) recordMigration(ctx context.Context, m Migration) error {
    coll := c.database.Collection("migrations")
    
    _, err := coll.InsertOne(ctx, MigrationHistory{
        Version:   m.Version,
        Name:      m.Name,
        AppliedAt: "2024-01-01T00:00:00Z",
    })
    
    return err
}

func (s *[]MigrationHistory) contains(version int) bool {
    for _, m := range *s {
        if m.Version == version {
            return true
        }
    }
    return false
}
```

### Migration Examples

```go
// Migration V1: Initial schema
func migrateV1Up(ctx context.Context, db *mongo.Database) error {
    // Create all collections
    collections := []string{
        "campaigns",
        "characters",
        "sessions",
        "story_events",
        "combat_encounters",
        "game_logs",
    }
    
    for _, name := range collections {
        if _, err := db.CreateCollection(ctx, name); err != nil {
            // Ignore "already exists" errors
            if !mongo.IsDuplicateKeyError(err) {
                return err
            }
        }
    }
    
    return nil
}

func migrateV1Down(ctx context.Context, db *mongo.Database) error {
    // Drop all collections
    collections := []string{
        "campaigns",
        "characters",
        "sessions",
        "story_events",
        "combat_encounters",
        "game_logs",
    }
    
    for _, name := range collections {
        if _, err := db.Collection(name).DeleteMany(ctx, bson.M{}); err != nil {
            return err
        }
    }
    
    return nil
}

// Migration V2: Add campaign settings
func migrateV2Up(ctx context.Context, db *mongo.Database) error {
    coll := db.Collection("campaigns")
    
    // Add new fields to existing documents
    _, err := coll.UpdateMany(ctx, bson.M{}, bson.M{
        "$set": bson.M{
            "settings.dice_rules": bson.M{
                "critical_hit": "double_damage",
                "critical_fail": "fumble",
            },
        },
    })
    
    return err
}

func migrateV2Down(ctx context.Context, db *mongo.Database) error {
    coll := db.Collection("campaigns")
    
    // Remove the new fields
    _, err := coll.UpdateMany(ctx, bson.M{}, bson.M{
        "$unset": bson.M{
            "settings.dice_rules": "",
        },
    })
    
    return err
}
```

---

## 7. Backup & Restore

### Backup Script

```bash
#!/bin/bash
# scripts/backup.sh

BACKUP_DIR="./backups"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="dnd_backup_${DATE}.gz"

# Create backup directory if not exists
mkdir -p $BACKUP_DIR

# MongoDB dump with gzip
mongodump \
  --uri="mongodb://${MONGODB_USER}:${MONGODB_PASSWORD}@localhost:27017/dnd_campaigns" \
  --archive="$BACKUP_DIR/$BACKUP_FILE" \
  --gzip \
  --oplog

echo "Backup created: $BACKUP_DIR/$BACKUP_FILE"

# Keep only last 7 backups
ls -t $BACKUP_DIR/dnd_backup_*.gz | tail -n +8 | xargs rm -f
```

### Restore Script

```bash
#!/bin/bash
# scripts/restore.sh

BACKUP_FILE=$1

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: ./restore.sh <backup_file.gz>"
    exit 1
fi

mongorestore \
  --uri="mongodb://${MONGODB_USER}:${MONGODB_PASSWORD}@localhost:27017/dnd_campaigns" \
  --archive="$BACKUP_FILE" \
  --gzip \
  --drop

echo "Restore completed from: $BACKUP_FILE"
```

---

## 8. Monitoring

### MongoDB Metrics

```go
// internal/infrastructure/monitoring/mongodb.go

package monitoring

import (
    "context"
    "time"
    
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
)

type MongoMetrics struct {
    Client *mongo.Client
}

func (m *MongoMetrics) GetStats(ctx context.Context) (map[string]interface{}, error) {
    adminDB := m.Client.Database("admin")
    
    var result bson.M
    err := adminDB.RunCommand(ctx, bson.M{"serverStatus": 1}).Decode(&result)
    if err != nil {
        return nil, err
    }
    
    return map[string]interface{}{
        "connections":        result["connections"],
        "network":           result["network"],
        "opcounters":        result["opcounters"],
        "memory":            result["mem"],
        "storage":           result["storageEngine"],
        "uptime_seconds":    result["uptime"],
    }, nil
}

func (m *MongoMetrics) GetCollectionStats(ctx context.Context, dbName, collName string) (map[string]interface{}, error) {
    db := m.Client.Database(dbName)
    
    var result bson.M
    err := db.RunCommand(ctx, bson.M{"collStats": collName}).Decode(&result)
    if err != nil {
        return nil, err
    }
    
    return map[string]interface{}{
        "count":         result["count"],
        "size":          result["size"],
        "avg_obj_size":  result["avgObjSize"],
        "storage_size":  result["storageSize"],
        "indexes":       result["nindexes"],
        "index_size":    result["totalIndexSize"],
    }, nil
}
```

---

## 9. Troubleshooting

### Common Issues

**Connection Refused**
```bash
# Check if MongoDB is running
brew services list | grep mongodb
# or
docker ps | grep mongodb
```

**Authentication Failed**
```bash
# Verify credentials
mongosh "mongodb://localhost:27017" -u admin -p your_password --authenticationDatabase admin
```

**Slow Queries**
```javascript
// Enable profiling in mongosh
db.setProfilingLevel(1, { slowms: 100 })

// Check slow queries
db.system.profile.find().pretty()
```

**Index Missing**
```javascript
// Check indexes
db.campaigns.getIndexes()

// Create missing index
db.campaigns.createIndex({ "campaign_id": 1 }, { unique: true })
```

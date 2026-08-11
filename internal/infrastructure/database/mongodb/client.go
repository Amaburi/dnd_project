package mongodb

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Client wraps the MongoDB client
type Client struct {
	client   *mongo.Client
	database *mongo.Database
	config   *Config
}

// Config holds MongoDB connection configuration
type Config struct {
	URI            string
	Database       string
	Username       string
	Password       string
	AuthSource     string
	MaxPoolSize    uint64
	MinPoolSize    uint64
	ConnectTimeout time.Duration
}

// NewClient creates a new MongoDB client
func NewClient(cfg Config) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	// Build connection URI
	uri, err := buildConnectionURI(cfg)
	if err != nil {
		return nil, err
	}

	// Configure client options
	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetConnectTimeout(cfg.ConnectTimeout)

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
		database: client.Database(cfg.Database),
		config:   &cfg,
	}, nil
}

// Database returns the database instance
func (c *Client) Database() *mongo.Database {
	return c.database
}

// Collection returns a collection by name
func (c *Client) Collection(name string) *mongo.Collection {
	return c.database.Collection(name)
}

// Close disconnects from MongoDB
func (c *Client) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

// Client returns the underlying mongo client
func (c *Client) Client() *mongo.Client {
	return c.client
}

// buildConnectionURI resolves the connection string to hand to the driver.
//
// A URI that already carries a scheme (the MongoDB Atlas case, which needs
// mongodb+srv:// plus its query parameters) is used verbatim. Only a bare
// host:port is assembled into a full URI from the individual settings.
func buildConnectionURI(cfg Config) (string, error) {
	if cfg.URI == "" {
		return "", fmt.Errorf("mongodb uri is not configured (set MONGODB_URI)")
	}

	if strings.HasPrefix(cfg.URI, "mongodb://") || strings.HasPrefix(cfg.URI, "mongodb+srv://") {
		return cfg.URI, nil
	}

	if cfg.Username == "" {
		return fmt.Sprintf("mongodb://%s/%s", cfg.URI, cfg.Database), nil
	}

	return fmt.Sprintf(
		"mongodb://%s:%s@%s/%s?authSource=%s",
		url.QueryEscape(cfg.Username),
		url.QueryEscape(cfg.Password),
		cfg.URI,
		cfg.Database,
		cfg.AuthSource,
	), nil
}

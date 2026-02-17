package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	MongoDB   MongoDBConfig   `mapstructure:"mongodb"`
	Redis     RedisConfig     `mapstructure:"redis"`
	DeepSeek  DeepSeekConfig  `mapstructure:"deepseek"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

// AppConfig holds application settings
type AppConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Environment string `mapstructure:"environment"`
	Debug       bool   `mapstructure:"debug"`
}

// MongoDBConfig holds MongoDB connection settings
type MongoDBConfig struct {
	URI            string        `mapstructure:"uri"`
	Database       string        `mapstructure:"database"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	AuthSource     string        `mapstructure:"auth_source"`
	MaxPoolSize    uint64        `mapstructure:"max_pool_size"`
	MinPoolSize    uint64        `mapstructure:"min_pool_size"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// DeepSeekConfig holds DeepSeek AI settings
type DeepSeekConfig struct {
	APIKey     string        `mapstructure:"api_key"`
	BaseURL    string        `mapstructure:"base_url"`
	Model      string        `mapstructure:"model"`
	Timeout    time.Duration `mapstructure:"timeout"`
	MaxRetries int           `mapstructure:"max_retries"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// RateLimitConfig holds rate limiting settings
type RateLimitConfig struct {
	RequestsPerMinute int `mapstructure:"requests_per_minute"`
	Burst             int `mapstructure:"burst"`
	AIRequestsPerHour int `mapstructure:"ai_requests_per_hour"`
}

// Load reads configuration from config.yaml and environment variables
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/dnd-campaign/")

	// Enable environment variable override
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("app.host", "0.0.0.0")
	viper.SetDefault("app.port", 8080)
	viper.SetDefault("app.environment", "development")
	viper.SetDefault("app.debug", true)

	viper.SetDefault("mongodb.max_pool_size", 100)
	viper.SetDefault("mongodb.min_pool_size", 10)
	viper.SetDefault("mongodb.connect_timeout", "10s")

	viper.SetDefault("redis.pool_size", 10)

	viper.SetDefault("deepseek.model", "deepseek-chat")
	viper.SetDefault("deepseek.timeout", "30s")
	viper.SetDefault("deepseek.max_retries", 3)

	viper.SetDefault("logging.level", "debug")
	viper.SetDefault("logging.format", "json")

	viper.SetDefault("rate_limit.requests_per_minute", 60)
	viper.SetDefault("rate_limit.burst", 10)
	viper.SetDefault("rate_limit.ai_requests_per_hour", 100)

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// GetMongoURI returns the MongoDB connection URI
func (c *MongoDBConfig) GetMongoURI() string {
	if c.Username != "" && c.Password != "" {
		return fmt.Sprintf("mongodb://%s:%s@%s/%s?authSource=%s",
			c.Username, c.Password, c.URI, c.Database, c.AuthSource)
	}
	return fmt.Sprintf("mongodb://%s/%s", c.URI, c.Database)
}

// GetRedisAddr returns the Redis address
func (c *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

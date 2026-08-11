package config

import (
	"fmt"
	"os"
	"strings"
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
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./configs")
	v.AddConfigPath("/etc/dnd-campaign/")

	// Enable environment variable override. The replacer maps nested keys onto
	// flat env var names, e.g. "mongodb.uri" -> MONGODB_URI. Without it
	// AutomaticEnv looks up "MONGODB.URI", which no shell can set.
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// AutomaticEnv only resolves keys viper already knows about, so bind the
	// secrets explicitly -- they must work even with no config file present.
	for key, env := range map[string]string{
		"mongodb.uri":      "MONGODB_URI",
		"deepseek.api_key": "DEEPSEEK_API_KEY",
		"redis.password":   "REDIS_PASSWORD",
	} {
		if err := v.BindEnv(key, env); err != nil {
			return nil, fmt.Errorf("failed to bind %s: %w", env, err)
		}
	}

	// Set defaults
	v.SetDefault("app.host", "0.0.0.0")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.debug", true)

	v.SetDefault("mongodb.max_pool_size", 100)
	v.SetDefault("mongodb.min_pool_size", 10)
	v.SetDefault("mongodb.connect_timeout", "10s")

	v.SetDefault("redis.pool_size", 10)

	v.SetDefault("deepseek.model", "deepseek-chat")
	v.SetDefault("deepseek.timeout", "30s")
	v.SetDefault("deepseek.max_retries", 3)

	v.SetDefault("logging.level", "debug")
	v.SetDefault("logging.format", "json")

	v.SetDefault("rate_limit.requests_per_minute", 60)
	v.SetDefault("rate_limit.burst", 10)
	v.SetDefault("rate_limit.ai_requests_per_hour", 100)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Viper does not interpolate "${VAR}" placeholders, so re-read the located
	// file with the environment expanded. An unset variable expands to "" --
	// never to the literal placeholder.
	if file := v.ConfigFileUsed(); file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}
		if err := v.ReadConfig(strings.NewReader(os.ExpandEnv(string(raw)))); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
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

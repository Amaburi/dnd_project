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
	AI        AIConfig        `mapstructure:"ai"`
	CORS      CORSConfig      `mapstructure:"cors"`
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

// AIConfig holds settings for the chat-completions provider.
//
// Any OpenAI-compatible endpoint works -- Groq, DeepSeek, OpenAI, a local
// server -- so the provider is chosen by BaseURL and Model rather than by
// code. Provider is a label for logs only.
type AIConfig struct {
	Provider   string        `mapstructure:"provider"`
	APIKey     string        `mapstructure:"api_key"`
	BaseURL    string        `mapstructure:"base_url"`
	Model      string        `mapstructure:"model"`
	Timeout    time.Duration `mapstructure:"timeout"`
	MaxRetries int           `mapstructure:"max_retries"`
	Pricing    PricingConfig `mapstructure:"pricing"`
}

// PricingConfig holds the provider's token rates in USD per million tokens.
//
// Rates vary by provider and model and change often, so they are configured
// rather than compiled in. Leaving both at 0 disables cost estimation.
type PricingConfig struct {
	PromptUSDPerMillion     float64 `mapstructure:"prompt_usd_per_million"`
	CompletionUSDPerMillion float64 `mapstructure:"completion_usd_per_million"`
}

// CORSConfig lists the browser origins allowed to call the API.
//
// Empty means no browser client, which is the safe default: a server that
// answers every origin is one a malicious page can drive with the user's
// credentials.
type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
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
	//
	// The AI key accepts several names so switching providers does not mean
	// renaming the variable in every shell profile and CI secret; viper takes
	// the first one that is set.
	for key, envs := range map[string][]string{
		"mongodb.uri": {"MONGODB_URI"},
		"ai.api_key":  {"GROQ_API_KEY", "DEEPSEEK_API_KEY", "OPENAI_API_KEY", "AI_API_KEY"},
	} {
		if err := v.BindEnv(append([]string{key}, envs...)...); err != nil {
			return nil, fmt.Errorf("failed to bind %s: %w", key, err)
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

	// No default provider, base URL or model: guessing one would silently
	// send traffic to an endpoint the operator never chose. ai.Validate
	// rejects an incomplete configuration at startup instead.
	v.SetDefault("ai.timeout", "30s")
	v.SetDefault("ai.max_retries", 3)

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

// The MongoDB URI is resolved by mongodb.buildConnectionURI, which passes a
// complete mongodb:// or mongodb+srv:// URI through untouched. There is
// deliberately no helper here that rebuilds one from parts.

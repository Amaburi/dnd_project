package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeConfigDir creates a configs/config.yaml alongside a temp working
// directory and chdirs into it, mirroring how the server is launched from
// the repository root.
func writeConfigDir(t *testing.T, yaml string) {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "configs", "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// clearAIKeyEnv removes every variable ai.api_key is bound to, so a key
// present in the developer's own shell cannot mask a test's expectations.
func clearAIKeyEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GROQ_API_KEY", "DEEPSEEK_API_KEY", "OPENAI_API_KEY", "AI_API_KEY"} {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
}

const placeholderConfig = `
mongodb:
  uri: "${MONGODB_URI}"
  database: "dnd_campaigns"
ai:
  provider: "groq"
  api_key: "${GROQ_API_KEY}"
  base_url: "https://api.groq.com/openai/v1"
  model: "llama-3.3-70b-versatile"
  pricing:
    prompt_usd_per_million: 0.59
    completion_usd_per_million: 0.79
`

func TestLoadReadsMongoURIFromEnvironment(t *testing.T) {
	writeConfigDir(t, placeholderConfig)
	t.Setenv("MONGODB_URI", "mongodb+srv://user:pw@cluster.mongodb.net/dnd")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.MongoDB.URI, "mongodb+srv://user:pw@cluster.mongodb.net/dnd"; got != want {
		t.Errorf("MongoDB.URI = %q, want %q", got, want)
	}
}

func TestLoadReadsAIKeyFromGroqEnvironment(t *testing.T) {
	writeConfigDir(t, placeholderConfig)
	clearAIKeyEnv(t)
	t.Setenv("GROQ_API_KEY", "gsk-test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.AI.APIKey, "gsk-test-key"; got != want {
		t.Errorf("AI.APIKey = %q, want %q", got, want)
	}
}

// Switching providers must not mean renaming the variable everywhere, so the
// older DeepSeek name still resolves when no Groq key is set.
func TestLoadAcceptsDeepSeekKeyAsFallback(t *testing.T) {
	writeConfigDir(t, `
mongodb:
  uri: "${MONGODB_URI}"
ai:
  base_url: "https://api.deepseek.com"
  model: "deepseek-chat"
`)
	clearAIKeyEnv(t)
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.AI.APIKey, "sk-deepseek-key"; got != want {
		t.Errorf("AI.APIKey = %q, want %q", got, want)
	}
}

// GROQ_API_KEY is bound first, so it wins when several are set.
func TestLoadPrefersGroqKeyOverFallbacks(t *testing.T) {
	writeConfigDir(t, placeholderConfig)
	clearAIKeyEnv(t)
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek-key")
	t.Setenv("GROQ_API_KEY", "gsk-groq-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.AI.APIKey, "gsk-groq-key"; got != want {
		t.Errorf("AI.APIKey = %q, want %q", got, want)
	}
}

func TestLoadReadsProviderRoutingAndPricing(t *testing.T) {
	writeConfigDir(t, placeholderConfig)
	clearAIKeyEnv(t)
	t.Setenv("GROQ_API_KEY", "gsk-test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.AI.BaseURL, "https://api.groq.com/openai/v1"; got != want {
		t.Errorf("AI.BaseURL = %q, want %q", got, want)
	}
	if got, want := cfg.AI.Model, "llama-3.3-70b-versatile"; got != want {
		t.Errorf("AI.Model = %q, want %q", got, want)
	}
	if got, want := cfg.AI.Pricing.PromptUSDPerMillion, 0.59; got != want {
		t.Errorf("prompt pricing = %v, want %v", got, want)
	}
	if got, want := cfg.AI.Pricing.CompletionUSDPerMillion, 0.79; got != want {
		t.Errorf("completion pricing = %v, want %v", got, want)
	}
}

// There is no default base URL or model: guessing one would silently send
// traffic to an endpoint nobody chose.
func TestLoadDoesNotInventAProvider(t *testing.T) {
	writeConfigDir(t, `
mongodb:
  uri: "${MONGODB_URI}"
`)
	clearAIKeyEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.AI.BaseURL != "" {
		t.Errorf("AI.BaseURL defaulted to %q, want empty", cfg.AI.BaseURL)
	}
	if cfg.AI.Model != "" {
		t.Errorf("AI.Model defaulted to %q, want empty", cfg.AI.Model)
	}
	// Timeout and retries are safe to default; routing is not.
	if cfg.AI.MaxRetries != 3 {
		t.Errorf("AI.MaxRetries = %d, want the default 3", cfg.AI.MaxRetries)
	}
}

// An unexpanded "${VAR}" placeholder must never reach the caller as a literal
// value -- that string was previously handed to the Mongo driver verbatim.
func TestLoadDoesNotLeakUnexpandedPlaceholder(t *testing.T) {
	writeConfigDir(t, placeholderConfig)
	os.Unsetenv("MONGODB_URI")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.MongoDB.URI == "${MONGODB_URI}" {
		t.Errorf("MongoDB.URI leaked the literal placeholder %q", cfg.MongoDB.URI)
	}
}

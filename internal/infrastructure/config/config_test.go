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

const placeholderConfig = `
mongodb:
  uri: "${MONGODB_URI}"
  database: "dnd_campaigns"
redis:
  password: "${REDIS_PASSWORD}"
deepseek:
  api_key: "${DEEPSEEK_API_KEY}"
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

func TestLoadReadsDeepSeekKeyFromEnvironment(t *testing.T) {
	writeConfigDir(t, placeholderConfig)
	t.Setenv("DEEPSEEK_API_KEY", "sk-test-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.DeepSeek.APIKey, "sk-test-key"; got != want {
		t.Errorf("DeepSeek.APIKey = %q, want %q", got, want)
	}
}

func TestLoadReadsRedisPasswordFromEnvironment(t *testing.T) {
	writeConfigDir(t, placeholderConfig)
	t.Setenv("REDIS_PASSWORD", "redis-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.Redis.Password, "redis-secret"; got != want {
		t.Errorf("Redis.Password = %q, want %q", got, want)
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

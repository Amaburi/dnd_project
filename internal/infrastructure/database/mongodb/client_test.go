package mongodb

import "testing"

func TestBuildConnectionURIPassesThroughAtlasSRVURI(t *testing.T) {
	cfg := Config{
		URI:        "mongodb+srv://user:pw@cluster.mongodb.net/dnd?retryWrites=true&w=majority",
		Database:   "dnd_campaigns",
		AuthSource: "admin",
	}

	got, err := buildConnectionURI(cfg)
	if err != nil {
		t.Fatalf("buildConnectionURI: %v", err)
	}

	if got != cfg.URI {
		t.Errorf("buildConnectionURI() = %q, want the URI unchanged: %q", got, cfg.URI)
	}
}

func TestBuildConnectionURIPassesThroughStandardURI(t *testing.T) {
	cfg := Config{
		URI:      "mongodb://user:pw@db.example.com:27017/dnd",
		Database: "dnd_campaigns",
	}

	got, err := buildConnectionURI(cfg)
	if err != nil {
		t.Fatalf("buildConnectionURI: %v", err)
	}

	if got != cfg.URI {
		t.Errorf("buildConnectionURI() = %q, want the URI unchanged: %q", got, cfg.URI)
	}
}

func TestBuildConnectionURIWrapsBareHostWithCredentials(t *testing.T) {
	cfg := Config{
		URI:        "localhost:27017",
		Database:   "dnd_campaigns",
		Username:   "admin",
		Password:   "secret",
		AuthSource: "admin",
	}

	got, err := buildConnectionURI(cfg)
	if err != nil {
		t.Fatalf("buildConnectionURI: %v", err)
	}

	want := "mongodb://admin:secret@localhost:27017/dnd_campaigns?authSource=admin"
	if got != want {
		t.Errorf("buildConnectionURI() = %q, want %q", got, want)
	}
}

// Without credentials the URI must not contain an empty "user:pass@" section --
// the driver rejects that with "username required if URI contains user info".
func TestBuildConnectionURIWrapsBareHostWithoutCredentials(t *testing.T) {
	cfg := Config{
		URI:      "localhost:27017",
		Database: "dnd_campaigns",
	}

	got, err := buildConnectionURI(cfg)
	if err != nil {
		t.Fatalf("buildConnectionURI: %v", err)
	}

	want := "mongodb://localhost:27017/dnd_campaigns"
	if got != want {
		t.Errorf("buildConnectionURI() = %q, want %q", got, want)
	}
}

func TestBuildConnectionURIRejectsEmptyURI(t *testing.T) {
	cfg := Config{Database: "dnd_campaigns"}

	_, err := buildConnectionURI(cfg)
	if err == nil {
		t.Fatal("buildConnectionURI() returned no error for an empty URI, want a configuration error")
	}
}

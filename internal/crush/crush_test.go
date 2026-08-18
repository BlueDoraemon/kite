package crush

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixtures writes a minimal providers.json and crush.json into a
// <parent>/crush directory and returns the parent.
func writeFixtures(t *testing.T, providersJSON, crushJSON string) string {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, "crush")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "providers.json"), []byte(providersJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "crush.json"), []byte(crushJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return parent
}

func TestLoadOpenAICompat(t *testing.T) {
	parent := writeFixtures(t,
		`[{"id":"openai","type":"openai","api_key":"$MY_KEY","api_endpoint":"https://api.example.com/v1"}]`,
		`{"providers":{"openai":{"api_key":"$MY_KEY"}},"models":{"large":{"model":"gpt-x","provider":"openai"}}}`,
	)
	// dataDir() joins XDG_DATA_HOME with "crush".
	t.Setenv("XDG_DATA_HOME", parent)
	t.Setenv("MY_KEY", "secret-value")

	imp, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if imp.Model != "gpt-x" || imp.Endpoint != "https://api.example.com/v1" || imp.APIKey != "secret-value" {
		t.Fatalf("imported = %+v", imp)
	}
}

func TestLoadRejectsHyper(t *testing.T) {
	parent := writeFixtures(t,
		`[{"id":"hyper","type":"hyper","api_endpoint":"https://hyper.example.com"}]`,
		`{"providers":{"hyper":{}},"models":{"large":{"model":"qwen","provider":"hyper"}}}`,
	)
	t.Setenv("XDG_DATA_HOME", parent)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "Hyper") {
		t.Fatalf("expected Hyper rejection, got %v", err)
	}
}

func TestLoadRejectsMissingEndpoint(t *testing.T) {
	parent := writeFixtures(t,
		`[{"id":"openai","type":"openai","api_key":"k"}]`,
		`{"providers":{"openai":{"api_key":"k"}},"models":{"large":{"model":"gpt-x","provider":"openai"}}}`,
	)
	t.Setenv("XDG_DATA_HOME", parent)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestLoadRejectsExpiredOAuth(t *testing.T) {
	parent := writeFixtures(t,
		`[{"id":"openai","type":"openai","api_endpoint":"https://api.example.com/v1"}]`,
		`{"providers":{"openai":{"oauth":{"expires_at":1000}}},"models":{"large":{"model":"gpt-x","provider":"openai"}}}`,
	)
	t.Setenv("XDG_DATA_HOME", parent)

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired OAuth error, got %v", err)
	}
}

func TestLoadSecretFreeErrors(t *testing.T) {
	// The provider has no credential anywhere; the error must not leak
	// any secret material.
	parent := writeFixtures(t,
		`[{"id":"openai","type":"openai","api_endpoint":"https://api.example.com/v1"}]`,
		`{"providers":{"openai":{}},"models":{"large":{"model":"gpt-x","provider":"openai"}}}`,
	)
	t.Setenv("XDG_DATA_HOME", parent)

	_, err := Load()
	if err == nil {
		t.Fatal("expected credential error")
	}
	if strings.Contains(err.Error(), "sk-") {
		t.Fatalf("error leaks a secret: %v", err)
	}
}

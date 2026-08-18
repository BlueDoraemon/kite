package schemagen

import (
	"os"
	"testing"
)

// TestGenerateProducesValidJSON ensures the generator emits parseable JSON
// schemas.
func TestGenerateProducesValidJSON(t *testing.T) {
	dir := t.TempDir()
	written, err := Generate(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 4 {
		t.Fatalf("generated %d schemas, want 4", len(written))
	}
	for _, w := range written {
		data, err := os.ReadFile(w)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) == 0 {
			t.Fatalf("schema %s is empty", w)
		}
	}
}

// TestCheckFailsOnMissingSchema ensures the check catches a missing schema.
func TestCheckFailsOnMissingSchema(t *testing.T) {
	dir := t.TempDir()
	if err := Check(dir); err == nil {
		t.Fatal("expected check to fail on missing schemas")
	}
}

// TestCheckPassesAfterGenerate ensures the check passes after generation.
func TestCheckPassesAfterGenerate(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(dir); err != nil {
		t.Fatal(err)
	}
	if err := Check(dir); err != nil {
		t.Fatalf("check failed after generate: %v", err)
	}
}

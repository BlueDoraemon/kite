package core_test

import (
	"context"
	"os"
	"testing"

	"github.com/BlueDoraemon/kite-core/internal/core"
	"github.com/BlueDoraemon/kite-core/internal/provider/openai"
	_ "github.com/BlueDoraemon/kite-core/internal/tools"
)

// TestLiveAcceptance runs against a real OpenAI-compatible endpoint. It is
// opt-in: set KITE_LIVE_TEST=1 and configure KITE_API_KEY, KITE_BASE_URL, and
// KITE_MODEL. It exercises a real prompt end-to-end.
func TestLiveAcceptance(t *testing.T) {
	if os.Getenv("KITE_LIVE_TEST") != "1" {
		t.Skip("set KITE_LIVE_TEST=1 to run the live acceptance test")
	}
	baseURL := os.Getenv("KITE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("KITE_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	apiKey := os.Getenv("KITE_API_KEY")
	if apiKey == "" {
		t.Fatal("KITE_API_KEY must be set for the live test")
	}

	dir := t.TempDir()
	p := openai.New(baseURL, apiKey, model)
	s, err := core.NewSession(core.Config{
		Provider:   p,
		Model:      model,
		WorkingDir: dir,
		DataDir:    dir,
		MaxTurns:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(context.Background(), "Reply with exactly the word ok.")
	if err != nil {
		t.Fatal(err)
	}
	var completed bool
	for ev := range ch {
		if ev.Type == core.EventSessionCompleted {
			completed = true
		}
	}
	if !completed {
		t.Fatal("session did not complete")
	}
}

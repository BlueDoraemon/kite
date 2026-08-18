package core_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BlueDoraemon/kite-core/internal/core"
	_ "github.com/BlueDoraemon/kite-core/internal/tools"
)

type integrationProvider struct {
	step int
}

func (p *integrationProvider) Complete(ctx context.Context, sess *core.Session, tools []core.Tool, onEvent func(core.ProviderEvent)) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	call := func(id, name, input string) {
		onEvent(core.ProviderEvent{ToolCall: &core.ToolCall{ID: id, Name: name, Input: input}})
	}
	switch p.step {
	case 0:
		call("c1", "bash", `{"command":"go build ./...","purpose":"verification"}`)
	case 1:
		call("c2", "read", `{"path":"main.go"}`)
	case 2:
		call("c3", "edit", `{"path":"main.go","old_text":"package main\n\nfunc main() {","new_text":"package main\n\nimport \"fmt\"\n\nfunc main() {"}`)
	case 3:
		call("c4", "bash", `{"command":"go build ./...","purpose":"verification"}`)
	default:
		onEvent(core.ProviderEvent{Text: "fixed it"})
		onEvent(core.ProviderEvent{Done: true})
	}
	p.step++
	return nil
}

func TestIntegrationBashFailReadEditVerifyPass(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("..", "..", "testdata", "broken-go-project")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	p := &integrationProvider{}
	s, err := core.NewSession(core.Config{
		Provider:   p,
		Model:      "m",
		WorkingDir: dir,
		DataDir:    filepath.Join(dir, "data"),
		MaxTurns:   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(context.Background(), "fix the build")
	if err != nil {
		t.Fatal(err)
	}

	var types []string
	var result *core.Result
	var verifications []*core.Verification
	var toolStarts, toolFinishes int
	for ev := range ch {
		types = append(types, ev.Type)
		switch ev.Type {
		case core.EventSessionCompleted:
			result = ev.Payload.(*core.SessionCompletedPayload).Result
		case core.EventVerification:
			verifications = append(verifications, ev.Payload.(*core.VerificationPayload).Verification)
		case core.EventToolStarted:
			toolStarts++
		case core.EventToolFinished:
			toolFinishes++
			p := ev.Payload.(*core.ToolFinishedPayload)
			fmt.Printf("tool %s finished: %q\n", p.Name, p.Output)
		}
	}

	if toolStarts == 0 || toolStarts != toolFinishes {
		t.Fatalf("tool starts=%d finishes=%d in event stream: %s", toolStarts, toolFinishes, strings.Join(types, ","))
	}
	if result == nil || result.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}
	if len(verifications) < 2 {
		t.Fatalf("verifications = %d, want at least 2", len(verifications))
	}
	if verifications[0].Status != "failed" {
		t.Fatalf("first verification = %+v, want failed", verifications[0])
	}
	last := verifications[len(verifications)-1]
	if last.Status != "passed" {
		t.Fatalf("last verification = %+v, want passed", last)
	}
	if result.Verification == nil || result.Verification.Status != "passed" || result.Verification.Stale {
		t.Fatalf("result verification = %+v, want current pass", result.Verification)
	}
}

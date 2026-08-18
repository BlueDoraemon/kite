package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptProvider is a scripted provider that returns a sequence of turns.
type scriptProvider struct {
	turns []turn
}

type turn struct {
	text      string
	toolCalls []ToolCall
	usage     *Usage
	err         *Error
}

func (s *scriptProvider) Complete(ctx context.Context, sess *Session, tools []Tool, onEvent func(ProviderEvent)) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(s.turns) == 0 {
		return errors.New("no turns left")
	}
	t := s.turns[0]
	s.turns = s.turns[1:]
	if t.err != nil {
		onEvent(ProviderEvent{Err: t.err})
		return nil
	}
	for _, c := range t.toolCalls {
		onEvent(ProviderEvent{ToolCall: &c})
	}
	if t.text != "" {
		onEvent(ProviderEvent{Text: t.text})
	}
	if t.usage != nil {
		onEvent(ProviderEvent{Usage: t.usage})
	}
	onEvent(ProviderEvent{Done: true})
	return nil
}

// echoTool returns a tool that echoes its input.
type echoTool struct{}

func (echoTool) Name() string        { return "echo" }
func (echoTool) Description() string { return "echo the input" }
func (echoTool) Schema() any         { return map[string]any{"type": "object"} }
func (echoTool) Run(ctx context.Context, input string) (string, error) {
	return "echo:" + input, nil
}

func TestRunToolCallThenFinishes(t *testing.T) {
	p := &scriptProvider{turns: []turn{
		{toolCalls: []ToolCall{{ID: "c1", Name: "echo", Input: `{"x":1}`}}},
		{text: "done", usage: &Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}},
	}}
	dir := t.TempDir()
	s, err := NewSession(Config{
		Provider:   p,
		Model:      "m",
		WorkingDir: dir,
		DataDir:    filepath.Join(dir, "data"),
		Tools:      []Tool{echoTool{}},
		MaxTurns:   5,
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	var result *Result
	for ev := range ch {
		types = append(types, ev.Type)
		if ev.Type == EventSessionCompleted {
			result = ev.Payload.(*SessionCompletedPayload).Result
		}
	}
	want := "session.started,user-message,model.started,usage,model.completed,tool.started,tool.finished,model.started,text.delta,usage,model.completed,session.completed"
	got := strings.Join(types, ",")
	if got != want {
		t.Fatalf("event sequence = %s, want %s", got, want)
	}
	if result == nil || result.Status != "completed" {
		t.Fatalf("result = %+v, want completed", result)
	}
	if result.Usage.TotalTokens != 15 {
		t.Fatalf("usage total = %d, want 15", result.Usage.TotalTokens)
	}
}

func TestRunMaxTurns(t *testing.T) {
	p := &scriptProvider{turns: []turn{
		{toolCalls: []ToolCall{{ID: "c1", Name: "echo", Input: "{}"}}},
		{toolCalls: []ToolCall{{ID: "c2", Name: "echo", Input: "{}"}}},
		{toolCalls: []ToolCall{{ID: "c3", Name: "echo", Input: "{}"}}},
		{toolCalls: []ToolCall{{ID: "c4", Name: "echo", Input: "{}"}}},
	}}
	dir := t.TempDir()
	s, err := NewSession(Config{
		Provider:   p,
		Model:      "m",
		WorkingDir: dir,
		DataDir:    filepath.Join(dir, "data"),
		Tools:      []Tool{echoTool{}},
		MaxTurns:   3,
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	var failed *Error
	for ev := range ch {
		if ev.Type == EventSessionFailed {
			failed = ev.Payload.(*SessionFailedPayload).Error
		}
	}
	if failed == nil || failed.Code != "max_turns" {
		t.Fatalf("expected max_turns failure, got %+v", failed)
	}
}

func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := &scriptProvider{}
	dir := t.TempDir()
	s, err := NewSession(Config{
		Provider:   p,
		Model:      "m",
		WorkingDir: dir,
		DataDir:    filepath.Join(dir, "data"),
		Tools:      []Tool{echoTool{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(ctx, "x")
	if err != nil {
		t.Fatal(err)
	}
	for ev := range ch {
		if ev.Type == EventSessionFailed {
			return // cancelled surfaces as a failure
		}
	}
}

func TestBuildContextDeterministic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("repo instructions"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(Config{
		Provider:   &scriptProvider{},
		Model:      "m",
		WorkingDir: dir,
		DataDir:    filepath.Join(dir, "data"),
		Tools:      []Tool{echoTool{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs := s.BuildContext()
	if len(msgs) < 2 {
		t.Fatalf("context messages = %d, want at least 2", len(msgs))
	}
	if msgs[0].Role != RoleSystem {
		t.Fatalf("first message role = %v, want system", msgs[0].Role)
	}
	if !strings.Contains(msgs[1].Content, "repo instructions") {
		t.Fatalf("second message = %q, want repo instructions", msgs[1].Content)
	}
}

func TestLoadInstructionsNearestAndSize(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only the root has AGENTS.md.
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	inst, err := LoadInstructions(sub)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Content != "root" {
		t.Fatalf("instructions = %q, want root", inst.Content)
	}
	if !strings.HasSuffix(inst.Path, "AGENTS.md") {
		t.Fatalf("path = %q", inst.Path)
	}

	// Oversized file is rejected.
	big := filepath.Join(sub, "AGENTS.md")
	if err := os.WriteFile(big, []byte(strings.Repeat("x", 65*1024)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadInstructions(sub); err == nil {
		t.Fatal("expected error for oversized AGENTS.md")
	}
}

func TestSessionPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	p := &scriptProvider{turns: []turn{
		{toolCalls: []ToolCall{{ID: "c1", Name: "echo", Input: `{"x":1}`}}},
		{text: "done"},
	}}
	s, err := NewSession(Config{
		Provider:   p,
		Model:      "m",
		WorkingDir: dir,
		DataDir:    dataDir,
		Tools:      []Tool{echoTool{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}

	// Reload the session from its durable log.
	loaded, err := LoadSession(Config{
		Provider:   &scriptProvider{},
		Model:      "m",
		WorkingDir: dir,
		DataDir:    dataDir,
		Tools:      []Tool{echoTool{}},
	}, s.ID)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if loaded.ID != s.ID {
		t.Fatalf("loaded id = %s, want %s", loaded.ID, s.ID)
	}
	if len(loaded.Messages) == 0 {
		t.Fatal("loaded session has no messages")
	}
	// The tool result should be replayed back.
	found := false
	for _, m := range loaded.Messages {
		if m.Role == RoleTool && m.ToolCallID == "c1" {
			found = true
		}
	}
	if !found {
		t.Fatal("tool result not replayed in loaded session")
	}
}

func TestStoreArtifactAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	content := []byte(strings.Repeat("x", 100))
	if err := store.StoreArtifact("s1", "a1", content); err != nil {
		t.Fatal(err)
	}
	data, err := store.LoadArtifact("s1", "a1", 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 50 {
		t.Fatalf("loaded = %d bytes, want 50", len(data))
	}
	// Offset read.
	data2, err := store.LoadArtifact("s1", "a1", 50, 50)
	if err != nil {
		t.Fatal(err)
	}
	if string(data2) != strings.Repeat("x", 50) {
		t.Fatalf("offset read = %q", string(data2))
	}
}

func TestLeaseAcquireAndRecover(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireLease("s1", 0); err != nil {
		t.Fatal(err)
	}
	// A second acquire within the TTL fails.
	if err := store.AcquireLease("s1", 0); err == nil {
		t.Fatal("expected lease conflict")
	}
	// Release and re-acquire works.
	if err := store.ReleaseLease("s1"); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireLease("s1", 0); err != nil {
		t.Fatal(err)
	}
}

func TestJSONLTruncatedRecordIgnored(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	ev := &Event{ID: "e1", Seq: 1, SessionID: "s1", Type: EventSessionStarted}
	if err := store.AppendEvent("s1", ev); err != nil {
		t.Fatal(err)
	}
	// Append a malformed record (simulating a crash mid-write).
	path := store.sessionPath("s1")
	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	fh.Write([]byte("{\"truncated\"\n"))
	fh.Close()
	evs, err := store.LoadEvents("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("events = %d, want 1 (truncated record ignored)", len(evs))
	}
}

func TestJSONMarshalEvent(t *testing.T) {
	ev := &Event{ID: "e1", Seq: 1, SessionID: "s1", Type: EventSessionStarted, Payload: &SessionStartedPayload{Prompt: "hi"}}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"session.started"`) {
		t.Fatalf("marshaled event = %s", string(data))
	}
}

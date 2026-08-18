package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testSessionID  = "sess_000000000000000000000001"
	testArtifactID = "art_000000000000000000000001"
)

// scriptProvider is a scripted provider that returns a sequence of turns.
type scriptProvider struct {
	turns []turn
}

type turn struct {
	text      string
	toolCalls []ToolCall
	usage     *Usage
	err       *Error
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

type failingAppendStore struct{ SessionStore }

func (failingAppendStore) AppendEvent(string, *Event) error { return errors.New("disk full") }

func (echoTool) Name() string        { return "echo" }
func (echoTool) Description() string { return "echo the input" }
func (echoTool) Schema() any         { return map[string]any{"type": "object"} }
func (echoTool) Run(ctx context.Context, input string) (string, error) {
	return "echo:" + input, nil
}

type testTool struct {
	name string
	run  func() (string, error)
}

func (t testTool) Name() string        { return t.name }
func (t testTool) Description() string { return t.name }
func (t testTool) Schema() any         { return map[string]any{"type": "object"} }
func (t testTool) Run(context.Context, string) (string, error) {
	return t.run()
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
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
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
	if err := store.StoreArtifact(testSessionID, testArtifactID, content); err != nil {
		t.Fatal(err)
	}
	data, err := store.LoadArtifact(testSessionID, testArtifactID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 50 {
		t.Fatalf("loaded = %d bytes, want 50", len(data))
	}
	// Offset read.
	data2, err := store.LoadArtifact(testSessionID, testArtifactID, 50, 50)
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
	if err := store.AcquireLease(testSessionID, 0); err != nil {
		t.Fatal(err)
	}
	// A second acquire within the TTL fails.
	if err := store.AcquireLease(testSessionID, 0); err == nil {
		t.Fatal("expected lease conflict")
	}
	// Release and re-acquire works.
	if err := store.ReleaseLease(testSessionID); err != nil {
		t.Fatal(err)
	}
	if err := store.AcquireLease(testSessionID, 0); err != nil {
		t.Fatal(err)
	}
}

func TestJSONLTruncatedRecordIgnored(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	ev := &Event{ID: "e1", Seq: 1, SessionID: testSessionID, Type: EventSessionStarted}
	if err := store.AppendEvent(testSessionID, ev); err != nil {
		t.Fatal(err)
	}
	// Append a malformed record (simulating a crash mid-write).
	path, err := store.sessionPath(testSessionID)
	if err != nil {
		t.Fatal(err)
	}
	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	fh.Write([]byte("{\"truncated\"\n"))
	fh.Close()
	evs, err := store.LoadEvents(testSessionID)
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

func TestResultJSONUsesVersionedFieldNames(t *testing.T) {
	data, err := json.Marshal(&Result{
		Status:               "failed",
		ChangedFiles:         []string{"a.go"},
		ChangedFilesComplete: true,
		Verification:         &Verification{Command: "go test", Status: "failed", ExitCode: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"status", "changed_files", "changed_files_complete", "verification"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("result JSON missing %q: %s", key, data)
		}
	}
	var verification map[string]json.RawMessage
	if err := json.Unmarshal(got["verification"], &verification); err != nil {
		t.Fatal(err)
	}
	if _, ok := verification["exit_code"]; !ok {
		t.Fatalf("verification JSON missing exit_code: %s", got["verification"])
	}
}

func TestProviderEventErrorIsSingleTerminalFailure(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSession(Config{Provider: &scriptProvider{turns: []turn{{err: &Error{Code: "upstream", Message: "failed"}}}}, Model: "m", WorkingDir: dir, DataDir: filepath.Join(dir, "data"), Tools: []Tool{}})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	var terminals []string
	for ev := range ch {
		if ev.Type == EventSessionFailed || ev.Type == EventSessionCompleted {
			terminals = append(terminals, ev.Type)
		}
	}
	if strings.Join(terminals, ",") != EventSessionFailed {
		t.Fatalf("terminal events = %v", terminals)
	}
}

func TestPersistenceFailureIsNotPublished(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSession(Config{Provider: NoopProvider{}, Model: "m", WorkingDir: dir, DataDir: filepath.Join(dir, "data"), Tools: []Tool{}})
	if err != nil {
		t.Fatal(err)
	}
	s.store = failingAppendStore{SessionStore: s.store}
	ch, err := s.Prompt(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	var events []Event
	for ev := range ch {
		events = append(events, ev)
	}
	if len(events) != 0 || len(s.events) != 0 {
		t.Fatalf("unpersisted events were published=%d retained=%d", len(events), len(s.events))
	}
}

func TestReplayPreservesSequenceAndToolOnlyTurn(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	s, err := NewSession(Config{
		Provider: &scriptProvider{turns: []turn{{toolCalls: []ToolCall{{ID: "c1", Name: "echo", Input: `{}`}}}, {text: "done"}}},
		Model:    "m", WorkingDir: dir, DataDir: dataDir, Tools: []Tool{echoTool{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, _ := s.Prompt(context.Background(), "first")
	for range ch {
	}
	prior := len(s.events)
	loaded, err := LoadSession(Config{Provider: &scriptProvider{turns: []turn{{text: "resumed"}}}, Model: "m", WorkingDir: dir, DataDir: dataDir, Tools: []Tool{echoTool{}}}, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundCall := false
	for _, msg := range loaded.Messages {
		if msg.Role == RoleAssistant && len(msg.ToolCalls) == 1 && msg.ToolCalls[0].ID == "c1" {
			foundCall = true
		}
	}
	if !foundCall {
		t.Fatal("tool-only assistant turn was not reconstructed")
	}
	ch, err = loaded.Prompt(context.Background(), "second")
	if err != nil {
		t.Fatal(err)
	}
	firstSeq := 0
	for ev := range ch {
		if firstSeq == 0 {
			firstSeq = ev.Seq
		}
	}
	if firstSeq != prior+1 {
		t.Fatalf("first resumed seq = %d, want %d", firstSeq, prior+1)
	}
}

func TestReplayDiscardsIncompleteModelText(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	events := []*Event{
		{ID: "e1", Seq: 1, SessionID: testSessionID, Type: EventUserMessage, Payload: &UserMessagePayload{Text: "hello"}},
		{ID: "e2", Seq: 2, SessionID: testSessionID, Type: EventModelStarted, Payload: &ModelStartedPayload{Turn: 1}},
		{ID: "e3", Seq: 3, SessionID: testSessionID, Type: EventTextDelta, Payload: &TextDeltaPayload{Text: "partial"}},
	}
	for _, ev := range events {
		if err := store.AppendEvent(testSessionID, ev); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := LoadSession(Config{Provider: NoopProvider{}, Model: "m", WorkingDir: dir, DataDir: filepath.Join(dir, "data"), Tools: []Tool{}}, testSessionID)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range loaded.Messages {
		if msg.Role == RoleAssistant {
			t.Fatalf("incomplete assistant message was replayed: %+v", msg)
		}
	}
}

func TestReplayKeepsToolBatchAndDurableInterruption(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	store, err := openStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	calls := []ToolCall{{ID: "c1", Name: "echo", Input: `{}`}, {ID: "c2", Name: "echo", Input: `{}`}}
	events := []*Event{
		{ID: "e1", Seq: 1, SessionID: testSessionID, Type: EventModelStarted, Payload: &ModelStartedPayload{Turn: 1}},
		{ID: "e2", Seq: 2, SessionID: testSessionID, Type: EventModelCompleted, Payload: &ModelCompletedPayload{Turn: 1}},
		{ID: "e3", Seq: 3, SessionID: testSessionID, Type: EventToolStarted, Payload: &ToolStartedPayload{CallID: "c1", Name: "echo", Input: `{}`}},
		{ID: "e4", Seq: 4, SessionID: testSessionID, Type: EventToolFinished, Payload: &ToolFinishedPayload{CallID: "c1", Name: "echo", Output: "one"}},
		{ID: "e5", Seq: 5, SessionID: testSessionID, Type: EventToolStarted, Payload: &ToolStartedPayload{CallID: "c2", Name: "echo", Input: `{}`}},
		{ID: "e6", Seq: 6, SessionID: testSessionID, Type: EventInterruptedTool, Payload: &InterruptedToolPayload{Call: &calls[1]}},
	}
	for _, ev := range events {
		if err := store.AppendEvent(testSessionID, ev); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := LoadSession(Config{Provider: NoopProvider{}, Model: "m", WorkingDir: dir, DataDir: dataDir, Tools: []Tool{}}, testSessionID)
	if err != nil {
		t.Fatal(err)
	}
	var assistants []Message
	toolResults := map[string]bool{}
	for _, msg := range loaded.Messages {
		if msg.Role == RoleAssistant {
			assistants = append(assistants, msg)
		}
		if msg.Role == RoleTool {
			toolResults[msg.ToolCallID] = true
		}
	}
	if len(assistants) != 1 || len(assistants[0].ToolCalls) != 2 {
		t.Fatalf("replayed assistant turns = %+v", assistants)
	}
	if !toolResults["c1"] || !toolResults["c2"] {
		t.Fatalf("replayed tool results = %+v", toolResults)
	}
}

func TestCustomMutationStalesVerification(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	mutated := filepath.Join(dir, "changed.txt")
	tools := []Tool{
		testTool{name: "bash", run: func() (string, error) { return "ok", nil }},
		testTool{name: "mutate", run: func() (string, error) {
			if err := os.WriteFile(mutated, []byte("changed"), 0o600); err != nil {
				return "", err
			}
			return "changed", nil
		}},
	}
	provider := &scriptProvider{turns: []turn{
		{toolCalls: []ToolCall{{ID: "v1", Name: "bash", Input: `{"command":"check","purpose":"verification"}`}}},
		{toolCalls: []ToolCall{{ID: "m1", Name: "mutate", Input: `{}`}}},
		{text: "done"},
	}}
	s, err := NewSession(Config{Provider: provider, Model: "m", WorkingDir: dir, DataDir: filepath.Join(dir, "data"), Tools: tools})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := s.Prompt(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}
	var result *Result
	for ev := range ch {
		if ev.Type == EventSessionCompleted {
			result = ev.Payload.(*SessionCompletedPayload).Result
		}
	}
	if result == nil || result.Status != "failed" || result.Verification == nil || !result.Verification.Stale {
		t.Fatalf("result after custom mutation = %+v", result)
	}
}

func TestSupersededLeaseOwnerCannotAppend(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	oldStore, err := openStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	newStore, err := openStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := oldStore.AcquireLease(testSessionID, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	if err := newStore.AcquireLease(testSessionID, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	ev := &Event{ID: "e1", Seq: 1, SessionID: testSessionID, Type: EventSessionStarted}
	if err := oldStore.AppendEvent(testSessionID, ev); err == nil {
		t.Fatal("superseded lease owner appended an event")
	}
	if err := newStore.AppendEvent(testSessionID, ev); err != nil {
		t.Fatalf("current lease owner append: %v", err)
	}
}

func TestPromptHonorsSessionLease(t *testing.T) {
	dir := t.TempDir()
	s, err := NewSession(Config{Provider: NoopProvider{}, Model: "m", WorkingDir: dir, DataDir: filepath.Join(dir, "data"), Tools: []Tool{}})
	if err != nil {
		t.Fatal(err)
	}
	store := s.store.(*FileStore)
	if err := store.AcquireLease(s.ID, time.Minute); err != nil {
		t.Fatal(err)
	}
	defer store.ReleaseLease(s.ID)
	if _, err := s.Prompt(context.Background(), "x"); err == nil {
		t.Fatal("expected prompt lease conflict")
	}
}

func TestWorktreeDiffDetectsOnlyChangesAfterBaseline(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	path := filepath.Join(dir, "already-dirty.txt")
	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", "already-dirty.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	baseline := snapshotWorktree(dir)
	if got, complete := diffWorktree(dir, baseline); !complete || len(got) != 0 {
		t.Fatalf("unchanged dirty baseline = %v, complete=%v", got, complete)
	}
	if err := os.WriteFile(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, complete := diffWorktree(dir, baseline)
	if !complete || len(got) != 1 || got[0] != "already-dirty.txt" {
		t.Fatalf("changed dirty baseline = %v, complete=%v", got, complete)
	}
}

func TestLoadInstructionsStopsAtRepositoryRoot(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "AGENTS.md"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(parent, "repo")
	work := filepath.Join(repo, "sub")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	inst, err := LoadInstructions(work)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Content != "" {
		t.Fatalf("loaded instructions outside repository: %q", inst.Content)
	}
}

func TestStoreRejectsTraversalIDs(t *testing.T) {
	store, err := openStore(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadEvents("../../outside"); err == nil {
		t.Fatal("expected invalid session ID rejection")
	}
	if err := store.StoreArtifact(testSessionID, "../../outside", []byte("x")); err == nil {
		t.Fatal("expected invalid artifact ID rejection")
	}
}

func TestArtifactPreviewIsBoundedHeadAndTail(t *testing.T) {
	output := strings.Repeat("a", 100) + strings.Repeat("z", 100)
	preview := outputPreview(output, 80)
	if len(preview) > 80 || !strings.HasPrefix(preview, "aaa") || !strings.HasSuffix(preview, "zzz") {
		t.Fatalf("preview length=%d content=%q", len(preview), preview)
	}
}

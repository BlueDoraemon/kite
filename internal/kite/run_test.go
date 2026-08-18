package kite

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeProvider returns a scripted sequence of replies: a tool call, then a
// final text reply.
type fakeProvider struct {
	replies []Reply
	gotSess []*Session
}

func (f *fakeProvider) Complete(ctx context.Context, session *Session, tools []Tool) (Reply, error) {
	if ctx.Err() != nil {
		return Reply{}, ctx.Err()
	}
	f.gotSess = append(f.gotSess, session)
	if len(f.replies) == 0 {
		return Reply{}, errors.New("no replies left")
	}
	r := f.replies[0]
	f.replies = f.replies[1:]
	return r, nil
}

// echoTool returns a tool that, when asked for its name or to echo, lets us
// exercise the tool-call path.
type echoTool struct{}

func (echoTool) Name() string        { return "echo" }
func (echoTool) Description() string { return "echo the input" }
func (echoTool) Schema() any         { return map[string]any{"type": "object"} }
func (echoTool) Run(ctx context.Context, input string) (string, error) {
	return "echo:" + input, nil
}

func TestRunDoesToolCallThenFinishes(t *testing.T) {
	p := &fakeProvider{replies: []Reply{
		{ToolCalls: []ToolCall{{ID: "call1", Name: "echo", Input: `{"x":1}`}}},
		{Text: "done", Finish: true},
	}}
	var events []Event
	err := Run(context.Background(), p, RunOptions{
		Session: &Session{Model: "m", Messages: []Message{{Role: RoleUser, Content: "hi"}}},
		Tools:   []Tool{echoTool{}},
		OnEvent: func(e Event) error { events = append(events, e); return nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n := len(events); n != 4 {
		t.Fatalf("expected 4 events, got %d", n)
	}
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
	}
	want := []string{"tool_call", "tool_result", "message", "finish"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("event sequence = %v, want %v", types, want)
	}

	// The tool result must be fed back into the session before the next turn.
	last := p.gotSess[len(p.gotSess)-1]
	found := false
	for _, m := range last.Messages {
		if m.Role == RoleTool && m.ToolCallID == "call1" && m.Content == "echo:{\"x\":1}" {
			found = true
		}
	}
	if !found {
		t.Fatalf("tool result not fed back to provider; session messages: %+v", last.Messages)
	}
}

func TestRunUnknownToolReportsError(t *testing.T) {
	var buf bytes.Buffer
	provider := &fakeProvider{replies: []Reply{
		{ToolCalls: []ToolCall{{ID: "c", Name: "nope", Input: "{}"}}},
		{Text: "ok", Finish: true},
	}}
	err := Run(context.Background(), provider, RunOptions{
		Session: &Session{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}},
		Stdout:  &buf,
		Print:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The unknown-tool error must be surfaced to the model as a tool result.
	last := provider.gotSess[len(provider.gotSess)-1]
	if !strings.Contains(_dump(last), "unknown tool") {
		t.Fatalf("expected unknown tool error fed back, got: %s", _dump(last))
	}
}

func _dump(s *Session) string {
	var b strings.Builder
	for _, m := range s.Messages {
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func TestRunRecordsAssistantToolCallsAndResults(t *testing.T) {
	p := &fakeProvider{replies: []Reply{
		{ToolCalls: []ToolCall{{ID: "call1", Name: "echo", Input: `{"x":1}`}}},
		{Text: "done", Finish: true},
	}}
	err := Run(context.Background(), p, RunOptions{
		Session: &Session{Model: "m", Messages: []Message{{Role: RoleUser, Content: "hi"}}},
		Tools:   []Tool{echoTool{}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The session should contain, in order: user, assistant(tool_calls),
	// tool(result), assistant(done).
	if n := len(p.gotSess[1].Messages); n != 4 {
		t.Fatalf("final session messages = %d, want 4", n)
	}
	msgs := p.gotSess[1].Messages
	if msgs[1].Role != RoleAssistant || len(msgs[1].ToolCalls) != 1 || msgs[1].ToolCalls[0].ID != "call1" {
		t.Fatalf("expected assistant message to record call1, got %+v", msgs[1])
	}
	if msgs[2].Role != RoleTool || msgs[2].ToolCallID != "call1" || msgs[2].Content != "echo:{\"x\":1}" {
		t.Fatalf("expected tool result for call1, got %+v", msgs[2])
	}
	if msgs[3].Role != RoleAssistant || msgs[3].Content != "done" {
		t.Fatalf("expected final assistant message, got %+v", msgs[3])
	}
}

func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, &fakeProvider{}, RunOptions{
		Session: &Session{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRunMaxTurns(t *testing.T) {
	// A provider that never finishes will trip the turn limit.
	err := Run(context.Background(), loopingProvider{}, RunOptions{
		Session:  &Session{Model: "m", Messages: []Message{{Role: RoleUser, Content: "x"}}},
		Tools:    []Tool{echoTool{}},
		MaxTurns: 3,
	})
	if err == nil || !strings.Contains(err.Error(), "max turns") {
		t.Fatalf("expected max turns error, got %v", err)
	}
}

// loopingProvider always asks for a tool call, so it never finishes.
type loopingProvider struct{}

func (loopingProvider) Complete(ctx context.Context, s *Session, tools []Tool) (Reply, error) {
	if ctx.Err() != nil {
		return Reply{}, ctx.Err()
	}
	return Reply{ToolCalls: []ToolCall{{ID: "c", Name: "echo", Input: "{}"}}}, nil
}

func TestPrintMirrorsText(t *testing.T) {
	var buf bytes.Buffer
	err := Run(context.Background(), &fakeProvider{replies: []Reply{
		{Text: "hello", Finish: true},
	}}, RunOptions{
		Session: &Session{Model: "m", Messages: []Message{{Role: RoleUser, Content: "hi"}}},
		Stdout:  &buf,
		Print:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "hello\n" {
		t.Fatalf("stdout = %q, want %q", buf.String(), "hello\n")
	}
}

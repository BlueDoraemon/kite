package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/BlueDoraemon/kite-core/internal/kite"
)

// stubTool is a minimal kite.Tool used to verify tool advertisement.
type stubTool struct{}

func (stubTool) Name() string        { return "my_tool" }
func (stubTool) Description() string { return "does things" }
func (stubTool) Schema() any {
	return map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}}
}
func (stubTool) Run(context.Context, string) (string, error) { return "", nil }

func TestCompleteSendsMessagesAndTools(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("authorization = %q, want Bearer key", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	defer srv.Close()

	p := New(srv.URL, "key", "model-1")
	msg := []kite.Message{{Role: kite.RoleUser, Content: "hello"}}
	reply, err := p.Complete(context.Background(), &kite.Session{Model: "model-1", Messages: msg}, []kite.Tool{stubTool{}})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if reply.Text != "hi" {
		t.Fatalf("reply text = %q, want %q", reply.Text, "hi")
	}

	if gotBody["model"] != "model-1" {
		t.Fatalf("model in request = %v, want model-1", gotBody["model"])
	}
	tools, ok := gotBody["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools in request = %v, want one tool", gotBody["tools"])
	}
	fn, ok := tools[0].(map[string]any)["function"].(map[string]any)
	if !ok {
		t.Fatalf("tools[0].function missing: %v", tools[0])
	}
	if fn["name"] != "my_tool" {
		t.Fatalf("function name = %v, want my_tool", fn["name"])
	}
}

func TestCompleteParsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{
			"role":"assistant",
			"content":"",
			"tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{\"path\":\"go.mod\"}"}}]
		}}]}`))
	}))
	defer srv.Close()

	p := New(srv.URL, "", "m")
	reply, err := p.Complete(context.Background(), &kite.Session{Model: "m"}, nil)
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if len(reply.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(reply.ToolCalls))
	}
	tc := reply.ToolCalls[0]
	if tc.ID != "c1" || tc.Name != "read" || !strings.Contains(tc.Input, "go.mod") {
		t.Fatalf("tool call = %+v", tc)
	}
}

func TestCompleteReportsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	p := New(srv.URL, "x", "m")
	_, err := p.Complete(context.Background(), &kite.Session{Model: "m"}, nil)
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
}

func TestCompleteNoChoicesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	p := New(srv.URL, "", "m")
	_, err := p.Complete(context.Background(), &kite.Session{Model: "m"}, nil)
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestCompleteRetriesOn503(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"busy"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := New(srv.URL, "", "m")
	p.MaxRetries = 3
	reply, err := p.Complete(context.Background(), &kite.Session{Model: "m"}, nil)
	if err != nil {
		t.Fatalf("complete failed after retries: %v", err)
	}
	if reply.Text != "ok" {
		t.Fatalf("reply text = %q, want ok", reply.Text)
	}
	if calls != 3 {
		t.Fatalf("server called %d times, want 3", calls)
	}
}

func TestCompleteNoRetryOn400(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()

	p := New(srv.URL, "", "m")
	p.MaxRetries = 3
	_, err := p.Complete(context.Background(), &kite.Session{Model: "m"}, nil)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected 400 error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("server called %d times, want 1 (no retry on 400)", calls)
	}
}

func TestTruncateKeepsUTF8Intact(t *testing.T) {
	// A multibyte rune straddling the cut point must not be split.
	s := "héllo wörld"
	out := truncate(s, 6)
	if !utf8.ValidString(out) {
		t.Fatalf("truncate produced invalid UTF-8: %q", out)
	}
	if !strings.HasSuffix(out, "...") {
		t.Fatalf("truncate output = %q, want ellipsis suffix", out)
	}
	if len(out) > 9 { // 6 bytes + "..."
		t.Fatalf("truncate output too long: %q (%d bytes)", out, len(out))
	}
	if truncate("short", 100) != "short" {
		t.Fatal("truncate should return short strings unchanged")
	}
}

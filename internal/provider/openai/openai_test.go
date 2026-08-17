package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BlueDoraemon/kite/internal/kite"
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

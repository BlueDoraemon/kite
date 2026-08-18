package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BlueDoraemon/kite-core/internal/core"
)

// stubTool is a minimal core.Tool used to verify tool advertisement.
type stubTool struct{}

func (stubTool) Name() string        { return "my_tool" }
func (stubTool) Description() string { return "does things" }
func (stubTool) Schema() any {
	return map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}}
}
func (stubTool) Run(context.Context, string) (string, error) { return "", nil }

func newSession() *core.Session {
	return &core.Session{ID: "s1", Model: "m"}
}

func TestCompleteStreamsTextAndUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		write := func(s string) { w.Write([]byte(s + "\n\n")) }
		write(`data: {"choices":[{"delta":{"content":"hel"}}]}`)
		write(`data: {"choices":[{"delta":{"content":"lo"}}]}`)
		write(`data: {"choices":[{"delta":{"content":""}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`)
		write("data: [DONE]")
	}))
	defer srv.Close()

	p := New(srv.URL, "key", "model-1")
	var text strings.Builder
	var usage *core.Usage
	done := false
	err := p.Complete(context.Background(), newSession(), []core.Tool{stubTool{}}, func(pe core.ProviderEvent) {
		text.WriteString(pe.Text)
		if pe.Usage != nil {
			usage = pe.Usage
		}
		if pe.Done {
			done = true
		}
	})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if text.String() != "hello" {
		t.Fatalf("text = %q, want hello", text.String())
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %+v, want total 5", usage)
	}
	if !done {
		t.Fatal("done not emitted")
	}
}

func TestCompleteFragmentedToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		write := func(s string) { w.Write([]byte(s + "\n\n")) }
		// The tool call arrives in fragments.
		write(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"read","arguments":"{\"pa"}}]}}]}`)
		write(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"th\":\"go.mod\"}"}}]}}]}`)
		write("data: [DONE]")
	}))
	defer srv.Close()

	p := New(srv.URL, "", "m")
	var calls []core.ToolCall
	err := p.Complete(context.Background(), newSession(), nil, func(pe core.ProviderEvent) {
		if pe.ToolCall != nil {
			calls = append(calls, *pe.ToolCall)
		}
	})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Name != "read" || !strings.Contains(calls[0].Input, "go.mod") {
		t.Fatalf("call = %+v", calls[0])
	}
}

func TestCompleteEmitsToolCallsByProviderIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":1,\"id\":\"c2\",\"function\":{\"name\":\"second\",\"arguments\":\"{}\"}},{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"first\",\"arguments\":\"{}\"}}]}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer srv.Close()
	p := New(srv.URL, "", "m")
	var names []string
	if err := p.Complete(context.Background(), newSession(), nil, func(pe core.ProviderEvent) {
		if pe.ToolCall != nil {
			names = append(names, pe.ToolCall.Name)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "first,second" {
		t.Fatalf("tool call order = %v", names)
	}
}

func TestCompleteAppliesConfiguredTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()
	p := New(srv.URL, "", "m")
	p.Timeout = 20 * time.Millisecond
	start := time.Now()
	err := p.Complete(context.Background(), newSession(), nil, func(core.ProviderEvent) {})
	if err == nil {
		t.Fatal("expected request timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("configured timeout took %s", elapsed)
	}
}

func TestCompleteMalformedSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {not json}\n\n"))
	}))
	defer srv.Close()

	p := New(srv.URL, "", "m")
	err := p.Complete(context.Background(), newSession(), nil, func(core.ProviderEvent) {})
	if err == nil {
		t.Fatal("expected error for malformed SSE")
	}
	var ke *core.Error
	if !asCoreError(err, &ke) || ke.Code != "malformed_sse" {
		t.Fatalf("error = %v, want malformed_sse", err)
	}
}

func asCoreError(err error, target **core.Error) bool {
	e, ok := err.(*core.Error)
	if !ok {
		return false
	}
	*target = e
	return true
}

func TestCompleteNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	p := New(srv.URL, "x", "m")
	err := p.Complete(context.Background(), newSession(), nil, func(core.ProviderEvent) {})
	if err == nil {
		t.Fatal("expected 401 error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %v, want 401", err)
	}
}

func TestCompleteCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"))
		// Hold the connection open.
		<-r.Context().Done()
	}))
	defer srv.Close()

	p := New(srv.URL, "", "m")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.Complete(ctx, newSession(), nil, func(core.ProviderEvent) {})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestBuildRequestSendsToolsAndStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true {
			t.Errorf("stream = %v, want true", body["stream"])
		}
		if body["model"] != "model-1" {
			t.Errorf("model = %v", body["model"])
		}
		tools, _ := body["tools"].([]any)
		if len(tools) != 1 {
			t.Errorf("tools = %v, want 1", tools)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: [DONE]\nn"))
	}))
	defer srv.Close()

	p := New(srv.URL, "", "model-1")
	err := p.Complete(context.Background(), newSession(), []core.Tool{stubTool{}}, func(core.ProviderEvent) {})
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}
}

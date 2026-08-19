package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BlueDoraemon/kite-core/internal/core"
	"github.com/BlueDoraemon/kite-core/internal/provider/openai"
	"github.com/BlueDoraemon/kite-core/internal/rpc"
)

func TestRPCPromptReturnsRuntimeFailure(t *testing.T) {
	t.Setenv("KITE_DATA_DIR", filepath.Join(t.TempDir(), "data"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()
	handler := &rpcHandler{provider: openai.New(server.URL, "", "m"), dir: t.TempDir()}
	params, _ := json.Marshal(rpc.PromptParams{Text: "hello"})
	resp, err := handler.Handle(&rpc.Request{ID: "1", Method: rpc.MethodPrompt, Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if resp.OK || resp.Error == nil || resp.Error.Code != "provider" {
		t.Fatalf("RPC response = %+v", resp)
	}
}

func TestRPCStatusReturnsRequestedSession(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	t.Setenv("KITE_DATA_DIR", dataDir)
	workDir := t.TempDir()
	session, err := core.NewSession(core.Config{Provider: core.NoopProvider{}, Model: "m", WorkingDir: workDir, DataDir: dataDir, Tools: []core.Tool{}})
	if err != nil {
		t.Fatal(err)
	}
	events, err := session.Prompt(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}
	handler := &rpcHandler{provider: openai.New("http://unused", "", "m"), dir: workDir}
	params, _ := json.Marshal(rpc.StatusParams{SessionID: session.ID})
	resp, err := handler.Handle(&rpc.Request{ID: "1", Method: rpc.MethodStatus, Params: params})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Fatalf("RPC response = %+v", resp)
	}
	var status rpc.StatusResult
	if err := json.Unmarshal(resp.Result, &status); err != nil {
		t.Fatal(err)
	}
	if status.SessionID != session.ID || status.Messages == 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestTUIRejectsUnknownThemeBeforeSessionSetup(t *testing.T) {
	if code := run([]string{"tui", "-theme", "ultraviolet"}); code != 2 {
		t.Fatalf("run(tui invalid theme) = %d, want 2", code)
	}
}

func TestTUIExecutableRunsPlainInteractiveSession(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "tui", "-plain", "-theme", "paper-trail")
	cmd.Env = append(os.Environ(), "KITE_DATA_DIR="+filepath.Join(t.TempDir(), "data"))
	cmd.Stdin = strings.NewReader("/theme high-contrast\n/quit\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kite tui failed: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{"KITE | session sess_", "theme paper-trail", "theme set to high-contrast", "session retained as sess_"} {
		if !strings.Contains(text, want) {
			t.Fatalf("kite tui output missing %q:\n%s", want, text)
		}
	}
	if strings.ContainsRune(text, '\x1b') {
		t.Fatalf("kite tui -plain emitted ANSI escapes: %q", text)
	}
}

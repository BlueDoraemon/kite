// Command kite is a minimal command-line agent that can explain and modify a
// repository. It drives a model through an OpenAI-compatible API and gives it
// read, edit, bash, and artifact tools to work with the current directory.
//
// Exit codes: 0 completed, 1 runtime or verification failure, 2 usage or
// configuration error.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/BlueDoraemon/kite-core/internal/core"
	"github.com/BlueDoraemon/kite-core/internal/crush"
	"github.com/BlueDoraemon/kite-core/internal/provider/openai"
	"github.com/BlueDoraemon/kite-core/internal/rpc"
	"github.com/BlueDoraemon/kite-core/internal/tools"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches the command and returns the exit code.
func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "run":
		return cmdRun(rest)
	case "resume":
		return cmdResume(rest)
	case "rpc":
		return cmdRPC(rest)
	case "status":
		return cmdStatus(rest)
	case "inspect":
		return cmdInspect(rest)
	case "artifact":
		return cmdArtifact(rest)
	case "context":
		return cmdContext(rest)
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "kite: unknown command %q\n", cmd)
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `kite: a minimal agent runtime for repositories

Usage:
  kite run [flags] <prompt>          Run a prompt in the current directory
  kite resume <session-id> [prompt]  Resume a session
  kite rpc                           Serve the NDJSON RPC protocol on stdin/stdout
  kite status [session-id]           Show session status
  kite inspect <tool-id>             Show a tool's schema
  kite artifact <artifact-id> [--offset N --limit N]  Retrieve an artifact
  kite context [session-id] [--full]  Show the session context

Environment:
  KITE_API_KEY, KITE_BASE_URL, KITE_MODEL, KITE_DATA_DIR
  --from-crush reads the Crush-selected large model, credential, and endpoint

Exit codes: 0 completed, 1 runtime/verification failure, 2 usage/config error
`)
}

// providerConfig resolves the provider configuration with precedence:
// explicit flags -> Kite environment -> Crush import -> defaults.
func providerConfig(baseURL, model *string, fromCrush bool) (*openai.Provider, error) {
	apiKey := os.Getenv("KITE_API_KEY")
	if fromCrush {
		imp, err := crush.Load()
		if err != nil {
			return nil, err
		}
		if *baseURL == "" {
			*baseURL = imp.Endpoint
		}
		if *model == "" {
			*model = imp.Model
		}
		if apiKey == "" {
			apiKey = imp.APIKey
		}
	}
	if *baseURL == "" {
		*baseURL = envOr("KITE_BASE_URL", "https://api.openai.com/v1")
	}
	if *model == "" {
		*model = envOr("KITE_MODEL", "gpt-4o-mini")
	}
	return openai.New(*baseURL, apiKey, *model), nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// sessionConfig builds the core.Config for a session.
func sessionConfig(p *openai.Provider, workingDir string, print bool) core.Config {
	return core.Config{
		Provider:   p,
		Model:      p.Model,
		WorkingDir: workingDir,
		DataDir:    envOr("KITE_DATA_DIR", ""),
		Stdout:     os.Stdout,
		Print:      print,
		MaxTurns:   50,
	}
}

// runPrompt runs a prompt on a session and returns the result or failure.
func runPrompt(ctx context.Context, sess *core.Session, prompt string, print bool) (*core.Result, *core.Error) {
	ch, err := sess.Prompt(ctx, prompt)
	if err != nil {
		return nil, &core.Error{Code: "setup", Message: err.Error()}
	}
	var result *core.Result
	var failed *core.Error
	for ev := range ch {
		switch ev.Type {
		case core.EventTextDelta:
			if print {
				fmt.Print(ev.Payload.(*core.TextDeltaPayload).Text)
			}
		case core.EventSessionCompleted:
			result = ev.Payload.(*core.SessionCompletedPayload).Result
		case core.EventSessionFailed:
			failed = ev.Payload.(*core.SessionFailedPayload).Error
		}
	}
	if result == nil && failed == nil {
		failed = &core.Error{Code: "runtime", Message: "session ended without a durable result"}
	}
	return result, failed
}

// cmdRun runs a prompt in the current directory.
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		baseURL   = fs.String("base-url", "", "OpenAI-compatible API base URL")
		model     = fs.String("model", "", "model to use")
		fromCrush = fs.Bool("from-crush", false, "import model, credential, and endpoint from Crush")
		noPrint   = fs.Bool("no-print", false, "do not mirror output to stdout")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "kite: usage: kite run [flags] <prompt>")
		return 2
	}
	prompt := strings.Join(rest, " ")

	provider, err := providerConfig(baseURL, model, *fromCrush)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}

	sess, err := core.NewSession(sessionConfig(provider, dir, !*noPrint))
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 2
	}

	result, failed := runPrompt(ctx, sess, prompt, !*noPrint)
	if failed != nil {
		fmt.Fprintln(os.Stderr, "kite:", failed.Message)
		return 1
	}
	if result != nil {
		fmt.Printf("\n--- result ---\nstatus: %s\n", result.Status)
		if len(result.ChangedFiles) > 0 {
			fmt.Printf("changed files: %s\n", strings.Join(result.ChangedFiles, ", "))
		}
		if result.Verification != nil {
			fmt.Printf("verification: %s (exit %d)\n", result.Verification.Status, result.Verification.ExitCode)
		}
		if result.Status == "failed" {
			return 1
		}
	}
	return 0
}

// cmdResume resumes a session.
func cmdResume(args []string) int {
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		baseURL   = fs.String("base-url", "", "OpenAI-compatible API base URL")
		model     = fs.String("model", "", "model to use")
		fromCrush = fs.Bool("from-crush", false, "import model, credential, and endpoint from Crush")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "kite: usage: kite resume <session-id> [prompt]")
		return 2
	}
	sessionID := rest[0]
	prompt := ""
	if len(rest) > 1 {
		prompt = strings.Join(rest[1:], " ")
	}

	provider, err := providerConfig(baseURL, model, *fromCrush)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}

	sess, err := core.LoadSession(sessionConfig(provider, dir, true), sessionID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 2
	}

	// The standard continuation instruction; an explicit prompt replaces it.
	if prompt == "" {
		prompt = "Continue from where the previous run left off. Do not repeat completed work."
	}

	result, failed := runPrompt(ctx, sess, prompt, true)
	if failed != nil {
		fmt.Fprintln(os.Stderr, "kite:", failed.Message)
		return 1
	}
	if result != nil {
		fmt.Printf("\n--- result ---\nstatus: %s\n", result.Status)
		if result.Status == "failed" {
			return 1
		}
	}
	return 0
}

// cmdRPC serves the NDJSON RPC protocol on stdin/stdout.
func cmdRPC(args []string) int {
	fs := flag.NewFlagSet("rpc", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		baseURL   = fs.String("base-url", "", "OpenAI-compatible API base URL")
		model     = fs.String("model", "", "model to use")
		fromCrush = fs.Bool("from-crush", false, "import model, credential, and endpoint from Crush")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	provider, err := providerConfig(baseURL, model, *fromCrush)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 2
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	handler := &rpcHandler{provider: provider, dir: dir}
	srv := rpc.NewServer(handler)
	// Protocol-only stdout: diagnostics go to stderr.
	if err := srv.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	return 0
}

// rpcHandler implements rpc.Handler for the CLI.
type rpcHandler struct {
	provider *openai.Provider
	dir      string
}

func (h *rpcHandler) Handle(req *rpc.Request) (*rpc.Response, error) {
	resp := &rpc.Response{ID: req.ID, Method: req.Method, OK: true}
	switch req.Method {
	case rpc.MethodPrompt:
		var p rpc.PromptParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcErr(req, "bad_params", "invalid params"), nil
		}
		sess, err := core.NewSession(sessionConfig(h.provider, h.dir, false))
		if err != nil {
			return rpcErr(req, "setup", err.Error()), nil
		}
		ch, err := sess.Prompt(context.Background(), p.Text)
		if err != nil {
			return rpcErr(req, "busy", err.Error()), nil
		}
		var result *core.Result
		var failed *core.Error
		for ev := range ch {
			switch ev.Type {
			case core.EventSessionCompleted:
				result = ev.Payload.(*core.SessionCompletedPayload).Result
			case core.EventSessionFailed:
				failed = ev.Payload.(*core.SessionFailedPayload).Error
			}
		}
		if failed != nil {
			return rpcErr(req, failed.Code, failed.Message), nil
		}
		if result == nil {
			return rpcErr(req, "runtime", "session ended without a durable result"), nil
		}
		out, err := json.Marshal(result)
		if err != nil {
			return rpcErr(req, "encode", err.Error()), nil
		}
		resp.Result = out
		return resp, nil
	case rpc.MethodStatus:
		var p rpc.StatusParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &p); err != nil {
				return rpcErr(req, "bad_params", "invalid params"), nil
			}
		}
		store, err := core.OpenStore()
		if err != nil {
			return rpcErr(req, "store", err.Error()), nil
		}
		if p.SessionID != "" {
			sess, err := core.LoadSession(sessionConfig(h.provider, h.dir, false), p.SessionID)
			if err != nil {
				return rpcErr(req, "session", err.Error()), nil
			}
			out, err := json.Marshal(&rpc.StatusResult{SessionID: sess.ID, Model: sess.Model, Turn: sess.Turn, Messages: len(sess.Messages), Interrupted: sess.Interrupted})
			if err != nil {
				return rpcErr(req, "encode", err.Error()), nil
			}
			resp.Result = out
			return resp, nil
		}
		sessions, err := store.ListSessions()
		if err != nil {
			return rpcErr(req, "store", err.Error()), nil
		}
		out, _ := json.Marshal(map[string]any{"sessions": sessions})
		resp.Result = out
		return resp, nil
	case rpc.MethodInspect:
		var p rpc.InspectParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcErr(req, "bad_params", "invalid params"), nil
		}
		for _, t := range (&tools.Set{Dir: h.dir}).All() {
			if t.Name() == p.ToolID {
				out, _ := json.Marshal(map[string]any{"name": t.Name(), "description": t.Description(), "schema": t.Schema()})
				resp.Result = out
				return resp, nil
			}
		}
		return rpcErr(req, "not_found", "tool not found"), nil
	case rpc.MethodArtifact:
		var p rpc.ArtifactParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcErr(req, "bad_params", "invalid params"), nil
		}
		store, err := core.OpenStore()
		if err != nil {
			return rpcErr(req, "store", err.Error()), nil
		}
		tool := (&tools.Set{Dir: h.dir, Store: store}).Artifact()
		out, err := tool.Run(context.Background(), fmt.Sprintf(`{"id":%q,"offset":%d,"limit":%d}`, p.ArtifactID, p.Offset, p.Limit))
		if err != nil {
			return rpcErr(req, "artifact", err.Error()), nil
		}
		resp.Result, _ = json.Marshal(map[string]any{"content": out})
		return resp, nil
	case rpc.MethodContext:
		var p rpc.ContextParams
		_ = json.Unmarshal(req.Params, &p)
		noopCfg := core.Config{
			Provider:   core.NoopProvider{},
			Model:      "noop",
			WorkingDir: h.dir,
			DataDir:    envOr("KITE_DATA_DIR", ""),
		}
		sess, err := core.LoadSession(noopCfg, p.SessionID)
		if err != nil {
			return rpcErr(req, "session", err.Error()), nil
		}
		msgs := sess.BuildContext()
		out, err := json.Marshal(map[string]any{"messages": msgs})
		if err != nil {
			return rpcErr(req, "encode", err.Error()), nil
		}
		resp.Result = out
		return resp, nil
	case rpc.MethodResume:
		var p rpc.ResumeParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcErr(req, "bad_params", "invalid params"), nil
		}
		sess, err := core.LoadSession(sessionConfig(h.provider, h.dir, false), p.SessionID)
		if err != nil {
			return rpcErr(req, "session", err.Error()), nil
		}
		if p.Prompt == "" {
			p.Prompt = "Continue from where the previous run left off. Do not repeat completed work."
		}
		ch, err := sess.Prompt(context.Background(), p.Prompt)
		if err != nil {
			return rpcErr(req, "busy", err.Error()), nil
		}
		var result *core.Result
		var failed *core.Error
		for ev := range ch {
			switch ev.Type {
			case core.EventSessionCompleted:
				result = ev.Payload.(*core.SessionCompletedPayload).Result
			case core.EventSessionFailed:
				failed = ev.Payload.(*core.SessionFailedPayload).Error
			}
		}
		if failed != nil {
			return rpcErr(req, failed.Code, failed.Message), nil
		}
		if result == nil {
			return rpcErr(req, "runtime", "session ended without a durable result"), nil
		}
		out, err := json.Marshal(result)
		if err != nil {
			return rpcErr(req, "encode", err.Error()), nil
		}
		resp.Result = out
		return resp, nil
	default:
		return rpcErr(req, "unknown_method", "unknown method"), nil
	}
}

func rpcErr(req *rpc.Request, code, msg string) *rpc.Response {
	return &rpc.Response{
		ID:     req.ID,
		Method: req.Method,
		OK:     false,
		Error:  &rpc.Error{Code: code, Message: msg},
	}
}

// cmdStatus shows session status.
func cmdStatus(args []string) int {
	store, err := core.OpenStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 2
	}
	if len(args) > 0 {
		// Show a specific session's status.
		evs, err := store.LoadEvents(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "kite:", err)
			return 2
		}
		fmt.Printf("session: %s\n", args[0])
		fmt.Printf("events: %d\n", len(evs))
		for _, ev := range evs {
			fmt.Printf("%d %s\n", ev.Seq, ev.Type)
		}
		return 0
	}
	sessions, err := store.ListSessions()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	for _, s := range sessions {
		fmt.Println(s)
	}
	return 0
}

// cmdInspect shows a tool's schema.
func cmdInspect(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "kite: usage: kite inspect <tool-id>")
		return 2
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	for _, t := range (&tools.Set{Dir: dir}).All() {
		if t.Name() == args[0] {
			out, _ := json.MarshalIndent(map[string]any{"name": t.Name(), "description": t.Description(), "schema": t.Schema()}, "", "  ")
			fmt.Println(string(out))
			return 0
		}
	}
	fmt.Fprintf(os.Stderr, "kite: tool %q not found\n", args[0])
	return 2
}

// cmdArtifact retrieves an artifact.
func cmdArtifact(args []string) int {
	fs := flag.NewFlagSet("artifact", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	offset := fs.Int("offset", 0, "byte offset")
	limit := fs.Int("limit", 0, "byte limit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "kite: usage: kite artifact <artifact-id>")
		return 2
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	store, err := core.OpenStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	tool := (&tools.Set{Dir: dir, Store: store}).Artifact()
	out, err := tool.Run(context.Background(), fmt.Sprintf(`{"id":%q,"offset":%d,"limit":%d}`, rest[0], *offset, *limit))
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}
	fmt.Print(out)
	return 0
}

// cmdContext shows the session context.
func cmdContext(args []string) int {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	full := fs.Bool("full", false, "show full context")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	var sessionID string
	if len(rest) > 0 {
		sessionID = rest[0]
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		return 1
	}

	var sess *core.Session
	noopCfg := core.Config{
		Provider:   core.NoopProvider{},
		Model:      "noop",
		WorkingDir: dir,
		DataDir:    envOr("KITE_DATA_DIR", ""),
	}
	if sessionID != "" {
		// Load a persisted session and show its context.
		sess, err = core.LoadSession(noopCfg, sessionID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kite:", err)
			return 2
		}
	} else {
		// Show the context for a fresh session (system + repo instructions).
		sess, err = core.NewSession(noopCfg)
		if err != nil {
			fmt.Fprintln(os.Stderr, "kite:", err)
			return 2
		}
	}

	msgs := sess.BuildContext()
	for i, m := range msgs {
		if !*full && m.Role == core.RoleSystem && i > 0 {
			// Skip the repository instructions in brief mode.
			fmt.Printf("--- system (repository instructions) ---\n")
			continue
		}
		fmt.Printf("--- %s ---\n%s\n", m.Role, m.Content)
	}
	return 0
}

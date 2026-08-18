package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/BlueDoraemon/kite-core/internal/core"
)

type scriptedSession struct {
	events []core.Event
	err    error
}

type verificationProvider struct{ turn int }

func (p *verificationProvider) Complete(_ context.Context, _ *core.Session, _ []core.Tool, onEvent func(core.ProviderEvent)) error {
	p.turn++
	if p.turn == 1 {
		onEvent(core.ProviderEvent{ToolCall: &core.ToolCall{ID: "call_1", Name: "bash", Input: `{"command":"go test ./...","purpose":"verification"}`}})
	} else {
		onEvent(core.ProviderEvent{Text: "done"})
	}
	onEvent(core.ProviderEvent{Done: true})
	return nil
}

type verificationTool struct{}

func (verificationTool) Name() string        { return "bash" }
func (verificationTool) Description() string { return "verify" }
func (verificationTool) Schema() any         { return map[string]any{"type": "object"} }
func (verificationTool) Run(context.Context, string) (string, error) {
	return "ok", nil
}

func (s scriptedSession) Prompt(context.Context, string) (<-chan core.Event, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan core.Event, len(s.events))
	for _, event := range s.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func TestAppRendersDurableEventLedger(t *testing.T) {
	result := &core.Result{Status: "completed", ChangedFiles: []string{"main.go"}, Usage: core.Usage{TotalTokens: 42}}
	events := []core.Event{
		{Seq: 1, Type: core.EventSessionStarted, Payload: &core.SessionStartedPayload{Prompt: "fix it"}},
		{Seq: 2, Type: core.EventUserMessage, Payload: &core.UserMessagePayload{Text: "fix it"}},
		{Seq: 3, Type: core.EventModelStarted, Payload: &core.ModelStartedPayload{Turn: 1}},
		{Seq: 4, Type: core.EventTextDelta, Payload: &core.TextDeltaPayload{Text: "I will inspect.\n"}},
		{Seq: 5, Type: core.EventToolStarted, Payload: &core.ToolStartedPayload{Name: "read", Input: `{"path":"main.go"}`}},
		{Seq: 6, Type: core.EventToolFinished, Payload: &core.ToolFinishedPayload{Name: "read", Output: "package main"}},
		{Seq: 7, Type: core.EventArtifactCreated, Payload: &core.ArtifactCreatedPayload{Artifact: &core.Artifact{ID: "art_123", Size: 20000}}},
		{Seq: 8, Type: core.EventVerification, Payload: &core.VerificationPayload{Verification: &core.Verification{Command: "go test ./...", Status: "passed"}}},
		{Seq: 9, Type: core.EventSessionCompleted, Payload: &core.SessionCompletedPayload{Result: result}},
	}
	var output bytes.Buffer
	app, err := New(Config{
		Session: scriptedSession{events: events}, SessionID: "sess_123", Model: "m",
		WorkDir: "/repo", Theme: "night-flight", In: strings.NewReader("fix it\n/quit\n"), Out: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"KITE | session sess_123", "YOU  > fix it", "[run] TURN 01", "KITE | I will inspect.",
		"+ TOOL read", "[ok] TOOL read", "[file] ARTIFACT art_123", "[ok] VERIFY go test ./...",
		"changed main.go", "session retained as sess_123",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, output.String())
		}
	}
}

func TestAppRendersActualVerificationAfterToolClosure(t *testing.T) {
	dir := t.TempDir()
	session, err := core.NewSession(core.Config{
		Provider: &verificationProvider{}, Model: "m", WorkingDir: dir,
		DataDir: filepath.Join(dir, "data"), Tools: []core.Tool{verificationTool{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	app, err := New(Config{
		Session: session, SessionID: session.ID, Model: session.Model,
		In: strings.NewReader("verify\n/quit\n"), Out: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	tool := strings.Index(output.String(), "[ok] TOOL bash")
	verification := strings.Index(output.String(), "[ok] VERIFY go test ./...")
	if tool < 0 || verification < 0 || verification < tool {
		t.Fatalf("verification did not seal the completed tool hunk:\n%s", output.String())
	}
}

func TestAppCommandsChangeThemeAndInspectContext(t *testing.T) {
	var output bytes.Buffer
	app, err := New(Config{
		Session: scriptedSession{}, SessionID: "sess_123", Model: "m", Theme: "night",
		In: strings.NewReader("/theme paper\n/context\n/quit\n"), Out: &output,
		Context: func() []core.Message { return []core.Message{{Role: core.RoleUser, Content: "hello"}} },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"theme set to paper-trail", "CONTEXT 1 MESSAGES", "USER      hello"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, output.String())
		}
	}
}

func TestAppKeepsRunningAfterPromptSetupError(t *testing.T) {
	var output bytes.Buffer
	app, err := New(Config{
		Session: scriptedSession{err: errors.New("busy")}, SessionID: "sess_123", Model: "m",
		In: strings.NewReader("first\n/quit\n"), Out: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "[fail] busy") || !strings.Contains(output.String(), "session retained") {
		t.Fatalf("output did not recover from setup error:\n%s", output.String())
	}
}

func TestThemesHaveStableCanonicalNames(t *testing.T) {
	got := strings.Join(ThemeNames(), ",")
	if got != "high-contrast,night-flight,paper-trail" {
		t.Fatalf("ThemeNames() = %q", got)
	}
	for alias, want := range map[string]string{"night": "night-flight", "paper": "paper-trail", "contrast": "high-contrast"} {
		theme, err := ParseTheme(alias)
		if err != nil {
			t.Fatal(err)
		}
		if theme.Name != want {
			t.Fatalf("ParseTheme(%q).Name = %q, want %q", alias, theme.Name, want)
		}
	}
}

func TestAppStripsTerminalControlSequencesFromEventContent(t *testing.T) {
	var output bytes.Buffer
	app, err := New(Config{
		Session: scriptedSession{events: []core.Event{
			{Seq: 1, Type: core.EventTextDelta, Payload: &core.TextDeltaPayload{Text: "safe\x1b[2Jtext\u009b31m\u202ereversed"}},
			{Seq: 2, Type: core.EventSessionFailed, Payload: &core.SessionFailedPayload{Error: &core.Error{Code: "bad", Message: "oops\x1b]0;owned\a"}}},
		}},
		SessionID: "sess_123", Model: "m", In: strings.NewReader("go\n/quit\n"), Out: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(output.String(), '\x1b') || strings.ContainsRune(output.String(), '\a') ||
		strings.ContainsRune(output.String(), '\u009b') || strings.ContainsRune(output.String(), '\u202e') {
		t.Fatalf("output contains terminal control bytes: %q", output.String())
	}
	if !strings.Contains(output.String(), "safe[2Jtext") || !strings.Contains(output.String(), "oops]0;owned") {
		t.Fatalf("sanitized content was not preserved visibly: %q", output.String())
	}
}

func TestAppCancelsWhileWaitingForInput(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	var output bytes.Buffer
	app, err := New(Config{Session: scriptedSession{}, SessionID: "sess_123", Model: "m", In: reader, Out: &output})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop while input was blocked")
	}
}

func TestOneLineTruncatesOnDisplayCellBoundaries(t *testing.T) {
	got := oneLine("界界界", 5)
	if got != "界..." || !utf8.ValidString(got) || displayWidth(got) != 5 {
		t.Fatalf("oneLine wide text = %q, valid=%v, width=%d", got, utf8.ValidString(got), displayWidth(got))
	}
	if got := oneLine("e\u0301clair", 5); !utf8.ValidString(got) || displayWidth(got) > 5 {
		t.Fatalf("oneLine combining text = %q, valid=%v, width=%d", got, utf8.ValidString(got), displayWidth(got))
	}
}

func TestThemeRolesMeetTextContrastFloor(t *testing.T) {
	for _, name := range ThemeNames() {
		theme, err := ParseTheme(name)
		if err != nil {
			t.Fatal(err)
		}
		roles := map[string]rgb{
			"text": theme.Text, "accent": theme.Accent, "success": theme.Success,
			"warning": theme.Warning, "failure": theme.Failure, "muted": theme.Muted,
		}
		for role, color := range roles {
			if ratio := contrastRatio(color, theme.Background); ratio < 4.5 {
				t.Errorf("theme %s role %s contrast = %.2f, want >= 4.5", name, role, ratio)
			}
		}
	}
}

func contrastRatio(a, b rgb) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func relativeLuminance(color rgb) float64 {
	channel := func(value int) float64 {
		v := float64(value) / 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(color.r) + 0.7152*channel(color.g) + 0.0722*channel(color.b)
}

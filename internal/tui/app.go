// Package tui implements Kite's terminal workspace.
//
// TUI DIRECTION CONTRACT (seed d8d08b44)
// THESIS: Durable event order is the navigation; refuse the dashboard of
// unrelated cards. OWN-WORLD: A source-control hunk ledger with one-cell rules,
// sequence gutters, square geometry, and invariant state markers across three
// palettes. STORY: Ask, watch turns and tools unfold, verify the result, then
// continue the same durable session. FIRST VIEWPORT: A slim identity rail, one
// dominant chronological ledger, and a prompt anchored after the latest sealed
// result. FORM: Single-ledger composition, selected from the fourth grounded
// direction. FINISH: unreviewed and undocumented is unfinished; this build ends
// with the finish review, the verdict, and DESIGN.md.
package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/BlueDoraemon/kite-core/internal/core"
)

const (
	defaultWidth = 88
	maxInput     = 1024 * 1024
)

// Prompter is the session seam used by the terminal interface.
type Prompter interface {
	Prompt(context.Context, string) (<-chan core.Event, error)
}

// Config configures an App without changing the underlying session runtime.
type Config struct {
	Session   Prompter
	SessionID string
	Model     string
	WorkDir   string
	Theme     string
	In        io.Reader
	Out       io.Writer
	Context   func() []core.Message
	Color     bool
	Clear     bool
	Width     int
}

// App is a line-oriented terminal workspace over one durable Kite session.
type App struct {
	session   Prompter
	sessionID string
	model     string
	workDir   string
	theme     Theme
	in        io.Reader
	out       io.Writer
	context   func() []core.Message
	color     bool
	clear     bool
	width     int
	textOpen  bool
	writeErr  error
}

type scanResult struct {
	text string
	err  error
	done bool
}

// New constructs a terminal app and validates its theme before any model work.
func New(cfg Config) (*App, error) {
	if cfg.Session == nil {
		return nil, fmt.Errorf("kite tui: nil session")
	}
	theme, err := ParseTheme(cfg.Theme)
	if err != nil {
		return nil, err
	}
	if cfg.In == nil {
		cfg.In = os.Stdin
	}
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.Width <= 0 {
		cfg.Width = widthFromEnvironment()
	}
	if cfg.Width < 48 {
		cfg.Width = 48
	}
	if cfg.Width > 120 {
		cfg.Width = 120
	}
	return &App{
		session: cfg.Session, sessionID: cfg.SessionID, model: cfg.Model,
		workDir: cfg.WorkDir, theme: theme, in: cfg.In, out: cfg.Out,
		context: cfg.Context, color: cfg.Color, clear: cfg.Clear, width: cfg.Width,
	}, nil
}

// SupportsANSI reports whether f is an interactive terminal with ANSI support.
func SupportsANSI(f *os.File) bool {
	if f == nil || os.Getenv("NO_COLOR") != "" || strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	info, err := f.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if runtime.GOOS != "windows" {
		return true
	}
	return os.Getenv("WT_SESSION") != "" || os.Getenv("ANSICON") != "" ||
		strings.EqualFold(os.Getenv("ConEmuANSI"), "ON") || strings.Contains(strings.ToLower(os.Getenv("TERM")), "xterm")
}

func widthFromEnvironment() int {
	if width, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && width > 0 {
		return width
	}
	return defaultWidth
}

// Run accepts prompts until EOF, cancellation, or /quit.
func (a *App) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a.begin()
	defer a.end()
	a.header()
	if a.writeErr != nil {
		return a.writeErr
	}

	input := scanLines(ctx, a.in)
	for {
		select {
		case <-ctx.Done():
			a.closeText()
			a.line(a.mark("[stop]", a.theme.Warning, true) + " session interrupted")
			return ctx.Err()
		default:
		}

		a.prompt()
		if a.writeErr != nil {
			return a.writeErr
		}
		var scanned scanResult
		select {
		case <-ctx.Done():
			a.closeText()
			a.line(a.mark("[stop]", a.theme.Warning, true) + " session interrupted")
			return ctx.Err()
		case scanned = <-input:
		}
		if scanned.err != nil {
			return fmt.Errorf("kite tui: read prompt: %w", scanned.err)
		}
		if scanned.done {
			return nil
		}
		line := strings.TrimSpace(scanned.text)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			quit := a.command(line)
			if a.writeErr != nil {
				return a.writeErr
			}
			if quit {
				return nil
			}
			continue
		}
		if err := a.runPrompt(ctx, line); err != nil {
			a.line(a.mark("[fail]", a.theme.Failure, true) + " " + oneLine(err.Error(), a.width-8))
		}
		if a.writeErr != nil {
			return a.writeErr
		}
	}
}

func scanLines(ctx context.Context, input io.Reader) <-chan scanResult {
	results := make(chan scanResult)
	go func() {
		defer close(results)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 64*1024), maxInput)
		for scanner.Scan() {
			result := scanResult{text: scanner.Text()}
			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
		}
		result := scanResult{err: scanner.Err(), done: scanner.Err() == nil}
		select {
		case results <- result:
		case <-ctx.Done():
		}
	}()
	return results
}

func (a *App) command(input string) bool {
	fields := strings.Fields(input)
	command := strings.ToLower(fields[0])
	switch command {
	case "/quit", "/exit":
		a.line(a.muted("session retained as " + oneLine(a.sessionID, a.width-20)))
		return true
	case "/help":
		a.line(a.strong("COMMANDS"))
		a.line("  /theme <name>  switch palette: " + strings.Join(ThemeNames(), ", "))
		a.line("  /context       show the bounded session context")
		a.line("  /clear         clear and redraw the workspace")
		a.line("  /quit          leave; the durable session is retained")
	case "/theme":
		if len(fields) != 2 {
			a.line("themes: " + strings.Join(ThemeNames(), ", "))
			return false
		}
		theme, err := ParseTheme(fields[1])
		if err != nil {
			a.line(a.mark("[fail]", a.theme.Failure, true) + " " + err.Error())
			return false
		}
		a.theme = theme
		if a.color {
			// Repaint subsequent rows without erasing the durable ledger the
			// operator is using as navigation.
			a.printf("%s", ansiCanvas(a.theme))
		}
		a.line(a.mark("[ok]", a.theme.Success, true) + " theme set to " + a.theme.Name)
	case "/clear":
		if a.color && a.clear {
			a.printf("\x1b[2J\x1b[H")
			a.header()
		} else {
			a.rule("CLEARED")
		}
	case "/context":
		a.renderContext()
	default:
		a.line(a.mark("[fail]", a.theme.Failure, true) + " unknown command " + oneLine(command, 32) + "; use /help")
	}
	return false
}

func (a *App) runPrompt(ctx context.Context, prompt string) error {
	ch, err := a.session.Prompt(ctx, prompt)
	if err != nil {
		return err
	}
	for event := range ch {
		a.event(event)
		if a.writeErr != nil {
			return a.writeErr
		}
	}
	a.closeText()
	return nil
}

func (a *App) event(event core.Event) {
	if event.Type != core.EventTextDelta {
		a.closeText()
	}
	prefix := fmt.Sprintf("%04d ", event.Seq)
	switch event.Type {
	case core.EventSessionStarted:
		a.rule("PROMPT")
	case core.EventUserMessage:
		if payload, ok := event.Payload.(*core.UserMessagePayload); ok {
			a.line(prefix + a.strong("YOU  > ") + safeText(payload.Text))
		}
	case core.EventModelStarted:
		if payload, ok := event.Payload.(*core.ModelStartedPayload); ok {
			a.line(prefix + a.mark("[run]", a.theme.Warning, true) + fmt.Sprintf(" TURN %02d", payload.Turn))
		}
	case core.EventTextDelta:
		if payload, ok := event.Payload.(*core.TextDeltaPayload); ok {
			a.assistant(safeText(payload.Text))
		}
	case core.EventToolStarted:
		if payload, ok := event.Payload.(*core.ToolStartedPayload); ok {
			a.line(prefix + a.mark("+ TOOL", a.theme.Accent, true) + " " + oneLine(payload.Name, 40))
			a.preview("     | input ", payload.Input, 3)
		}
	case core.EventToolFinished:
		if payload, ok := event.Payload.(*core.ToolFinishedPayload); ok {
			status, color := "[ok]", a.theme.Success
			if payload.Error != nil {
				status, color = "[fail]", a.theme.Failure
			}
			a.line(prefix + a.mark(status, color, true) + " TOOL " + oneLine(payload.Name, 40))
			a.preview("     | ", payload.Output, 4)
		}
	case core.EventArtifactCreated:
		if payload, ok := event.Payload.(*core.ArtifactCreatedPayload); ok && payload.Artifact != nil {
			a.line(prefix + a.mark("[file]", a.theme.Accent, true) + fmt.Sprintf(" ARTIFACT %s  %d bytes", oneLine(payload.Artifact.ID, 48), payload.Artifact.Size))
		}
	case core.EventVerification:
		if payload, ok := event.Payload.(*core.VerificationPayload); ok && payload.Verification != nil {
			status, color := "[fail]", a.theme.Failure
			if payload.Verification.Status == "passed" && !payload.Verification.Stale {
				status, color = "[ok]", a.theme.Success
			} else if payload.Verification.Stale {
				status, color = "[stale]", a.theme.Warning
			}
			a.line(prefix + a.mark(status, color, true) + fmt.Sprintf(" VERIFY %s (exit %d)", oneLine(payload.Verification.Command, a.width-24), payload.Verification.ExitCode))
		}
	case core.EventUsage:
		if payload, ok := event.Payload.(*core.UsagePayload); ok {
			a.line(prefix + a.muted(fmt.Sprintf("tokens %d total", payload.Usage.TotalTokens)))
		}
	case core.EventModelCompleted:
		if payload, ok := event.Payload.(*core.ModelCompletedPayload); ok {
			a.line(prefix + a.mark("[ok]", a.theme.Success, true) + fmt.Sprintf(" TURN %02d sealed", payload.Turn))
		}
	case core.EventSessionCompleted:
		if payload, ok := event.Payload.(*core.SessionCompletedPayload); ok && payload.Result != nil {
			a.result(prefix, payload.Result)
		}
	case core.EventSessionFailed:
		if payload, ok := event.Payload.(*core.SessionFailedPayload); ok && payload.Error != nil {
			a.line(prefix + a.mark("[fail]", a.theme.Failure, true) + " " + oneLine(payload.Error.Code, 32) + ": " + oneLine(payload.Error.Message, a.width-16))
		}
	case core.EventInterruptedTool:
		if payload, ok := event.Payload.(*core.InterruptedToolPayload); ok && payload.Call != nil {
			a.line(prefix + a.mark("[stop]", a.theme.Warning, true) + " interrupted tool " + oneLine(payload.Call.Name, 40) + " (not replayed)")
		}
	case core.EventResume:
		a.line(prefix + a.mark("[resume]", a.theme.Accent, true) + " durable session resumed")
	default:
		a.line(prefix + a.muted("[event] "+oneLine(event.Type, 64)))
	}
}

func (a *App) result(prefix string, result *core.Result) {
	a.rule("RESULT")
	status, color := "[ok]", a.theme.Success
	if result.Status != "completed" {
		status, color = "[fail]", a.theme.Failure
	}
	a.line(prefix + a.mark(status, color, true) + " " + oneLine(result.Status, 32))
	if len(result.ChangedFiles) > 0 {
		files := make([]string, 0, len(result.ChangedFiles))
		for _, file := range result.ChangedFiles {
			files = append(files, oneLine(file, a.width-16))
		}
		a.line("     | changed " + strings.Join(files, ", "))
	}
	if result.Verification != nil {
		label := result.Verification.Status
		if result.Verification.Stale {
			label += " (stale)"
		}
		a.line("     | verification " + label)
	}
	a.line("     | usage " + strconv.Itoa(result.Usage.TotalTokens) + " tokens")
}

func (a *App) renderContext() {
	if a.context == nil {
		a.line(a.mark("[fail]", a.theme.Failure, true) + " context inspection unavailable")
		return
	}
	messages := a.context()
	a.rule(fmt.Sprintf("CONTEXT %d MESSAGES", len(messages)))
	start := len(messages) - 8
	if start < 0 {
		start = 0
	}
	for _, message := range messages[start:] {
		content := strings.TrimSpace(safeText(message.Content))
		if content == "" && len(message.ToolCalls) > 0 {
			content = fmt.Sprintf("%d tool call(s)", len(message.ToolCalls))
		}
		a.line(fmt.Sprintf("%-9s %s", strings.ToUpper(string(message.Role)), oneLine(content, a.width-12)))
	}
}

func (a *App) begin() {
	if !a.color {
		return
	}
	if a.clear {
		a.printf("%s\x1b[2J\x1b[H", ansiCanvas(a.theme))
		return
	}
	a.printf("%s", ansiCanvas(a.theme))
}

func (a *App) end() {
	a.closeText()
	if a.color {
		a.printf("\x1b[0m")
	}
}

func (a *App) header() {
	a.line(a.strong("KITE") + a.muted(" | session ") + oneLine(a.sessionID, 36) + a.muted(" | model ") + oneLine(a.model, 32) + a.muted(" | theme ") + a.theme.Name)
	if a.workDir != "" {
		a.line(a.muted("repo ") + oneLine(a.workDir, a.width-5))
	}
	a.line(strings.Repeat("-", a.width))
	a.line(a.muted("/help commands  /theme palette  /context inspect  /quit retain session"))
}

func (a *App) prompt() {
	a.closeText()
	a.printf("\n%s ", a.mark("> Ask Kite", a.theme.Accent, true))
}

func (a *App) assistant(text string) {
	parts := strings.SplitAfter(text, "\n")
	for _, part := range parts {
		if part == "" {
			continue
		}
		if !a.textOpen {
			a.printf("     %s ", a.strong("KITE |"))
			a.textOpen = true
		}
		a.printf("%s", part)
		if strings.HasSuffix(part, "\n") {
			a.textOpen = false
		}
	}
}

func (a *App) closeText() {
	if a.textOpen {
		a.printf("\n")
		a.textOpen = false
	}
}

func (a *App) preview(prefix, value string, maxLines int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	lines := strings.Split(value, "\n")
	truncated := len(lines) > maxLines
	if truncated {
		lines = lines[:maxLines]
	}
	for i, line := range lines {
		linePrefix := prefix
		if i > 0 {
			linePrefix = strings.Repeat(" ", len(prefix))
		}
		a.line(a.muted(linePrefix) + oneLine(line, a.width-len(linePrefix)))
	}
	if truncated {
		a.line(a.muted(strings.Repeat(" ", len(prefix)) + "... output truncated; inspect the artifact or session for full content"))
	}
}

func oneLine(value string, limit int) string {
	value = safeText(value)
	value = strings.Join(strings.Fields(value), " ")
	if limit < 4 || displayWidth(value) <= limit {
		return value
	}
	var out strings.Builder
	width := 0
	for _, r := range value {
		cells := runeWidth(r)
		if width+cells > limit-3 {
			break
		}
		out.WriteRune(r)
		width += cells
	}
	return out.String() + "..."
}

// safeText removes terminal control bytes from model, tool, path, and event
// content before it reaches the ANSI renderer. Newlines remain structural.
func safeText(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n':
			return r
		case r == '\t':
			return ' '
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			return -1
		case r == 0x061c || r == 0x200e || r == 0x200f ||
			(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
			return -1
		default:
			return r
		}
	}, value)
}

func displayWidth(value string) int {
	width := 0
	for _, r := range value {
		width += runeWidth(r)
	}
	return width
}

func runeWidth(r rune) int {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) || (r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff) || (r >= 0x20000 && r <= 0x3fffd)) {
		return 2
	}
	return 1
}

func (a *App) rule(label string) {
	label = " " + label + " "
	remaining := a.width - len(label)
	if remaining < 1 {
		remaining = 1
	}
	a.line(a.strong(label) + strings.Repeat("-", remaining))
}

func (a *App) line(value string) { a.printf("%s\n", value) }

func (a *App) printf(format string, args ...any) {
	if a.writeErr != nil {
		return
	}
	_, a.writeErr = fmt.Fprintf(a.out, format, args...)
}

func (a *App) strong(value string) string {
	if !a.color {
		return value
	}
	return "\x1b[1m" + value + "\x1b[22m"
}

func (a *App) muted(value string) string { return a.mark(value, a.theme.Muted, false) }

func (a *App) mark(value string, color rgb, bold bool) string {
	if !a.color {
		return value
	}
	prefix := ansiForeground(color)
	if bold {
		prefix += "\x1b[1m"
	}
	return prefix + value + "\x1b[22m" + ansiForeground(a.theme.Text)
}

---
version: 1
slug: "internal-tui-app-go"
primary_target: "internal/tui/app.go"
related_targets: ["cmd/kite/main.go"]
---

# Kite TUI surface brief

- Scope and mode: production terminal interface for `kite tui`; Operate mode.
- Audience and job: developers supervise repeated agent prompts, tool calls,
  artifacts, verification, and failure recovery without leaving a repository.
- Chosen direction: a single chronological hunk ledger based on
  `.impeccable/mocks/kite-tui-b.png`; durable event order is the navigation.
- Memorable moment: every tool call opens and closes like a diff hunk, then the
  verification row seals the run before the prompt returns.
- Constraints: standard-library Go, Linux/macOS/Windows builds, keyboard-only,
  three palettes with invariant glyphs, `NO_COLOR`, plain-stream fallback, no
  alternate execution engine, and no changes to existing CLI/RPC contracts.

## Composition inventory

| Region | Grammar | Implementation medium |
| --- | --- | --- |
| Status rail | One line: product, session, model, theme | ANSI text |
| Event ledger | Sequence gutter plus durable event rows | ANSI text and box glyphs |
| Tool hunk | Start row, bounded input/output preview, finish row | ANSI text |
| Result seal | Explicit status, files, verification, usage | ANSI text |
| Composer | Prompt plus compact slash-command hint | Buffered line input |
| Themes | Night Flight, Paper Trail, High Contrast | ANSI SGR tokens |

Rules: square terminal geometry, one-cell rules, no elevation, no rounded
containers, one monospace size, hierarchy through spacing/bold/state markers,
and never color without a textual or glyph equivalent.

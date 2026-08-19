# Terminal UI

`kite tui` opens a keyboard-first workspace over one durable Kite session. It
uses the same provider, tools, event log, artifacts, and result construction as
`kite run`; the interface is a consumer of `Session.Prompt`, not a separate
agent runtime.

## Start or resume

```sh
kite tui
kite tui sess_...
kite tui --from-crush
```

A fresh invocation prints its session ID in the status rail. Passing that ID
later reconstructs the session from its durable events before accepting the
next prompt.

## Event ledger

The main surface is chronological. Sequence numbers are the durable event
numbers, model turns are hunk headers, tool starts and finishes form bounded
hunks, and results seal each prompt. State uses explicit markers:

| Marker | Meaning |
| --- | --- |
| `[run]` | Model turn is active |
| `+ TOOL` | Tool call started |
| `[ok]` | Turn, tool, verification, or result succeeded |
| `[fail]` | Runtime, tool, verification, or result failed |
| `[stale]` | Verification predates a later worktree change |
| `[file]` | Large output was retained as an artifact |
| `[stop]` | Tool or session was interrupted |

Tool input and output are bounded in the ledger. Full large output remains
available through `kite artifact`, and the complete history remains in the
session event log.

## Commands

Enter a prompt normally, or use a slash command at the prompt:

| Command | Action |
| --- | --- |
| `/help` | Show commands and theme names |
| `/theme <name>` | Switch the active palette |
| `/context` | Show the last eight bounded context messages |
| `/clear` | Clear and redraw on an interactive ANSI terminal |
| `/quit` or `/exit` | Leave while retaining the durable session |

## Themes

Kite ships three complete palettes over the same layout and state grammar:

- `night-flight` — ink navy, cobalt, mint, amber, and coral
- `paper-trail` — paper, charcoal, denim, leaf, ochre, and vermilion
- `high-contrast` — black, white, cyan, green, yellow, and red

Select one with `kite tui -theme paper-trail`, set `KITE_THEME`, or switch with
`/theme`. Short aliases `night`, `paper`, and `contrast` are accepted. Color is
never the only status signal.

## Plain and non-interactive output

Use `kite tui -plain` to disable ANSI color and screen clearing. Kite also
chooses the plain path when stdout is redirected, `NO_COLOR` is set, `TERM` is
`dumb`, or a Windows terminal does not advertise ANSI support. The event
ledger and all status labels remain available in plain text.

## Terminal safety

Model text, tool input and output, paths, event names, and errors are untrusted
terminal content. Kite removes control bytes before rendering them so an ANSI
or OSC sequence in that content cannot clear the screen, change the title, or
inject terminal commands. Kite's own styling is emitted only by the renderer.

## Cancellation and failures

Interrupting the process cancels the active prompt through its context. Durable
events already written remain available for inspection and resume. A runtime
failure is rendered in the ledger and returns control to the prompt; setup and
terminal I/O failures are reported as command failures.

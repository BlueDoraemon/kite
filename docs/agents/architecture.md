# Architecture

Kite's runtime flows through a small set of stages:

```text
session -> context -> provider -> tools -> artifacts -> events
```

## Session

A `Session` carries the conversation, the model, and durable state. It is
created with `NewSession(Config)` or loaded with `LoadSession(Config, id)`.
Only one prompt may be active per session; a second `Prompt` call while one is
running returns an error. Runtime failures during a prompt are delivered as
`session.failed` events, not returned from `Prompt`.

## Context

`BuildContext` produces the deterministic context a model sees:

1. Fixed system instructions.
2. The nearest `AGENTS.md` between the working directory and the repository
   root (max 64 KiB), with its absolute source path recorded.
3. Completed messages.
4. Bounded tool previews.

## Provider

The provider streams model replies over SSE. It emits text deltas, completed
tool calls (assembling fragmented calls), usage, and sanitised errors. Wire
types stay private to the provider package.

## Tools

Tools run with repository containment: paths are resolved through symlinks
and rejected if they escape the working directory. Outputs larger than the
inline cap are stored as artifacts.

## Artifacts

Large outputs are stored under the data directory and referenced by a
globally unique ID. The tool result carries a compact preview with the ID,
size, media type, and truncation metadata.

## Events

Every event is durable, sequence-numbered, and persisted before publication.
Consumers receive them on the `Prompt` channel and can replay a session from
its JSONL log.

## Result

When a prompt completes, Kite builds a structured `Result`: status, final
text, files changed during that prompt, verification, and usage.

## See also

- [Go API](go-api.md)
- [Events](events.md)
- [Sessions](sessions.md)

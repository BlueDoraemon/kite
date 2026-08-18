# Events

Events are durable, sequence-numbered units emitted by the agent loop. They
are persisted before publication, so a session can be replayed exactly from
its JSONL log.

## Lifecycle ordering

A successful prompt emits:

```text
session.started
user-message
model.started
text.delta*      (as the model streams)
usage
model.completed
tool.started*    (one per tool call)
tool.finished*
artifact.created* (when a large output is stored)
model.started    (next turn)
...
session.completed
```

A failed prompt ends with `session.failed` instead of `session.completed`.

## Required event types

- `session.started`
- `model.started`
- `text.delta`
- `tool.started`
- `tool.finished`
- `artifact.created`
- `session.completed`
- `session.failed`

## Supporting event types

- `user-message`
- `model-completed`
- `usage`
- `resume`
-- `verification`
- `interrupted-tool`

## IDs

Every event has a globally unique prefixed ID (`evt_...`). Every session
(`sess_...`) and artifact (`art_...`) is also globally unique, so inspection
commands do not require a session argument.

## Payloads

Each event type carries a typed payload. For example, `text.delta` carries
`{"text": "..."}` (the delta text), and `tool.finished` carries the call ID,
name, output, and an optional error.

## Usage and errors

`usage` events carry aggregated token counts. `session.failed` carries a
structured, sanitised `Error` with a code and message. Errors never carry
secrets.

## Compatibility rules

Consumers must ignore unknown JSON fields. Additive fields remain v1
compatible; removing fields or changing their meaning requires a new contract
version. The contract version is `kite.event/v1`.

## See also

- [Sessions](sessions.md) — how events persist
- [Schema](../schemas/v1/event.json) — the versioned event schema

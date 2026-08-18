# Context

The context a model sees is built deterministically.

## Deterministic ordering

1. Fixed system instructions.
2. The nearest `AGENTS.md` between the working directory and the repository
   root, with its absolute source path recorded.
3. Completed messages.
4. Bounded tool previews.

## AGENTS.md discovery

Kite walks from the working directory up to the repository root and loads the
nearest `AGENTS.md`. Files larger than 64 KiB are rejected. The absolute
source path is recorded so the model knows where the instructions came from.

## Provenance

`BuildContext` returns the exact messages that will be sent to the provider.
Consumers can inspect them to see what a model will see, including which
repository instructions were loaded and from where.

## Inspection

```sh
kite context                 # context for a fresh session in this directory
kite context sess_...        # context for a persisted session
kite context sess_... --full # include repository instructions
```

## See also

- [Architecture](architecture.md)
- [Go API](go-api.md) — `BuildContext`

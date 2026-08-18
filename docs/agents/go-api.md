# Go API

The root package `kite` is the public façade. It re-exports the neutral types
backed by `internal/core`.

## Session API

- `NewSession(Config) (*Session, error)`
- `LoadSession(Config, id string) (*Session, error)`
- `(*Session).Prompt(ctx, text) (<-chan Event, error)`
- `(*Session).BuildContext() []Message`
- `(*Session).DataDir() string`

`Config` selects the provider, model, working and data directories, tools,
inline and preview limits, and maximum turns. A nil `Config.Tools` installs
the four built-in tools.

## Public contracts

- Runtime interfaces and values: `Provider`, `Tool`, `NoopProvider`,
  `ProviderEvent`, `Message`, `ToolCall`, `Event`, `Artifact`, `Usage`,
  `Result`, `Verification`, and `Error`.
- Persistence and context: `SessionStore`, `LoadInstructions`,
  `SystemInstructions`, and `Context`.
- Typed event payloads, role and event constants, and the four contract
  version constants are also exported by the root package.

## Lifecycle

```go
sess, err := kite.NewSession(kite.Config{
    Provider:   provider,
    Model:      "gpt-4o-mini",
    WorkingDir: ".",
    DataDir:    "", // platform default
})
if err != nil { /* setup error */ }

ch, err := sess.Prompt(ctx, "explain this repository")
if err != nil { /* a prompt is already active */ }

for ev := range ch {
    switch ev.Type {
    case kite.EventTextDelta:
        fmt.Print(ev.Payload.(*kite.TextDeltaPayload).Text)
    case kite.EventSessionCompleted:
        res := ev.Payload.(*kite.SessionCompletedPayload).Result
        fmt.Println("status:", res.Status)
    case kite.EventSessionFailed:
        e := ev.Payload.(*kite.SessionFailedPayload).Error
        fmt.Println("failed:", e.Message)
    }
}
```

## Cancellation

Pass a cancellable context to `Prompt`. Cancelling it stops the provider
stream and marks the session failed. Unfinished tool calls are recorded as
interrupted and never replayed automatically.

## Custom providers

Implement `kite.Provider`:

```go
type Provider interface {
    Complete(ctx context.Context, session *kite.Session, tools []kite.Tool,
        onEvent func(kite.ProviderEvent)) error
}
```

Emit `ProviderEvent` values with text deltas, tool calls, usage, and errors as
they arrive.

## Custom tools

Implement `kite.Tool`:

```go
type Tool interface {
    Name() string
    Description() string
    Schema() any
    Run(ctx context.Context, input string) (string, error)
}
```

Pass your tools in `Config.Tools`. Nil installs the built-ins.

## Compiled examples

See `examples/agents/go-session` (session driver), `examples/agents/custom-tool`
(custom tool), and `examples/agents/rpc-client` (NDJSON RPC client).

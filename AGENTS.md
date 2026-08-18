# AGENTS.md

Guidance for contributor agents working in this repository.

## Project purpose

Kite is a minimal, standard-library-only agent runtime for Go, plus a CLI
(`kite run <prompt>`) that drives a model through an OpenAI-compatible API with
read, edit, bash, and artifact tools. The runtime is embeddable: the root
package `kite` exposes a small public API backed by `internal/core`.

## Current milestone

The full 15-step roadmap is implemented: streaming provider, durable events,
artifacts, sessions, resume, RPC, inspection commands, repository
instructions, and complete agent documentation. The milestone is release-ready
when `go build ./...`, `go vet ./...`, `go test ./...` pass and all release
targets cross-compile.

## Explicit non-goals

- No TUI, plugins, multi-agent orchestration, First Mate integration,
  database, remote execution, or CI-delivery product features.
- No automatic provider retries after streamed output.
- No replaying of interrupted tool calls.

## Package boundaries

- `kite` (root) — the public façade. Re-exports types from `internal/core`.
  Consumers depend on this package, never on `internal/*`.
- `internal/core` — neutral types and the agent loop. Depends on nothing
  internal except the tool registration hook.
- `internal/provider/openai` — the OpenAI-compatible SSE provider. Wire types
  stay here, never in core.
- `internal/tools` — the read, edit, bash, and artifact tools. Registers the
  built-in installer with core via `core.RegisterBuiltins` (no import cycle).
- `internal/persist` — (unused; persistence lives in `internal/core/store.go`).
- `internal/rpc` — the NDJSON RPC protocol.
- `internal/crush` — reads Crush's persisted config for `--from-crush`.
- `cmd/kite` — the CLI entry point.

## Public-contract rules

- Consumers must ignore unknown JSON fields. Additive fields remain v1
  compatible; removing fields or changing their meaning requires a new
  contract version (`kite.event/v1`, `kite.result/v1`,
  `kite.rpc.request/v1`, `kite.rpc.response/v1`).
- Machine contracts are versioned explicitly. Never change a v1 schema
  in place.
- The root `kite` package is the only public surface. New exported symbols
  there must be documented and covered by a compiled example.

## Standard-library constraint

Runtime code and documentation tooling are standard-library only. Do not add
third-party dependencies. Shell execution uses `os/exec` with build tags for
POSIX (`sh -c`, process groups) and Windows (`cmd.exe /C`, `taskkill /T`).

## Context, event, and artifact invariants

- Context is built deterministically: fixed system instructions, then the
  nearest AGENTS.md (max 64 KiB), then completed messages.
- Events are durable and sequence-numbered, persisted before publication.
  Required event types: `session.started`, `model.started`, `text.delta`,
  `tool.started`, `tool.finished`, `artifact.created`, `session.completed`,
  `session.failed`. Supporting: `user-message`, `model-completed`, `usage`,
  `resume`, `verification`, `interrupted-tool`.
- Artifact thresholds and retrieval limits are owned by
  `docs/agents/artifacts.md`.
- Session and artifact IDs are globally unique and prefixed.

## Cross-platform requirements

- Every target in the release workflow must build. Shell execution and
  process-tree termination are build-tagged.
- Data directory: `$KITE_DATA_DIR`, else XDG data storage (Unix) or
  LOCALAPPDATA (Windows). User-only permissions where supported.

## Safety rules

- Repository containment: read/edit resolve symlinks and reject paths that
  escape the working directory.
- Credentials: never log API keys or OAuth tokens. Errors must be
  secret-free.
- Shell: commands run with a 30-second timeout and their process tree is
  killed on timeout. Verification runs are marked with `purpose:
  "verification"`.
- Repository trust: AGENTS.md is loaded from the nearest repository root and
  its size is capped.

## Required validation commands

```sh
go build ./...
go vet ./...
go test ./...
# cross-compile every release target:
GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/kite
GOOS=linux GOARCH=arm64 go build -o /dev/null ./cmd/kite
GOOS=darwin GOARCH=amd64 go build -o /dev/null ./cmd/kite
GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/kite
GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/kite
# schema generation check:
go run ./cmd/schemagen
```

## How to extend without breaking compatibility

### Add a provider

Implement `core.Provider` (`Complete` streams `ProviderEvent`s). Keep wire
types private to the provider package. Register it in `cmd/kite` for the CLI.

### Add a tool

Implement `core.Tool` (Name, Description, Schema, Run). Add it to
`internal/tools.Set.All()` and to the docs in `docs/agents/tools.md`.

### Add an event field

Add the field to the typed payload struct and to the v1 schema
(`docs/schemas/v1/event.json`). Regenerate schemas with the generator. The
field must be optional for consumers.

### Add a CLI command

Add a case in `cmd/kite/main.go`'s dispatch, a usage line, and document it in
`docs/agents/cli.md`. Add the method to the RPC protocol if it is callable
over RPC.

### Change persistence

Keep the JSONL format append-only. Truncated trailing records must be
ignored on load. Update `docs/agents/sessions.md`.

## Test requirements

- Prefer behavioural tests: execute a public or executable interface and
  assert observable behaviour. Do not add tests that grep implementation
  source for strings.
- The integration test (`internal/core/integration_test.go`) drives a
  scripted provider through bash fail → read → edit → verification bash pass
  against `testdata/broken-go-project`.
- The live acceptance test is opt-in (`KITE_LIVE_TEST=1`).
- RPC framing is tested through its executable NDJSON interface.
- Documentation validation regenerates and compares schemas, compiles the Go
  examples, and checks internal Markdown links.

# Kite

A minimal, standard-library-only agent runtime for Go, plus a command-line
agent that can explain and modify a repository.

Kite provides the building blocks for running agents reliably: sessions, a
streaming model provider, tools, durable events, artifacts, context
management, resume, and an NDJSON RPC protocol. It is designed to work on its
own or underneath supervisors and orchestrators.

> Small core. Open interfaces. Easy to compose.

## Installation

```sh
go install github.com/BlueDoraemon/kite-core/cmd/kite@latest
```

Or build from source:

```sh
git clone git@github.com:BlueDoraemon/kite-core.git
cd kite-core
go build -o kite ./cmd/kite
```

## Five-minute quick start

Set the model endpoint and key, then run:

```sh
export KITE_API_KEY=sk-...
kite run "explain this repository"
kite run "add a --retries flag to the upload command"
```

To reuse the model, credential, and endpoint Crush has selected:

```sh
kite run --from-crush "explain this repository"
```

### Configuration

| Variable | Flag | Default | Purpose |
| --- | --- | --- | --- |
| `KITE_API_KEY` | (none) | unset | API key sent as a Bearer token |
| `KITE_BASE_URL` | `-base-url` | `https://api.openai.com/v1` | OpenAI-compatible API base URL |
| `KITE_MODEL` | `-model` | `gpt-4o-mini` | Model identifier |
| `KITE_DATA_DIR` | (none) | XDG/LOCALAPPDATA | Where sessions and artifacts are stored |

See [Providers](docs/agents/providers.md) for configuration precedence and
`--from-crush` behavior.

### Your first successful task

```sh
kite run "create a file called hello.txt containing 'hello kite'"
```

The agent reads, edits, and verifies within the current directory. When it
finishes, Kite prints a structured result: status, changed files, and any
verification outcome. The session is persisted under the configured data
directory (or the platform default) and can be resumed with
`kite resume <session-id>`.

## Architecture

```text
session -> context -> provider -> tools -> artifacts -> events
```

- **Session** — durable conversation state, resumable from its JSONL log.
- **Context** — deterministic: system instructions, nearest AGENTS.md,
  completed messages, bounded tool previews.
- **Provider** — streams model replies over SSE, assembling fragmented tool
  calls.
- **Tools** — read, edit, bash, artifact, with repository containment.
- **Artifacts** — large outputs stored and retrieved by ID.
- **Events** — durable, sequence-numbered, versioned contracts.

## Command index

| Command | Purpose |
| --- | --- |
| `kite run [flags] <prompt>` | Run a prompt in the current directory |
| `kite resume <session-id> [prompt]` | Resume a session |
| `kite rpc` | Serve the NDJSON RPC protocol on stdin/stdout |
| `kite status [session-id]` | Show session status |
| `kite inspect <tool-id>` | Show a tool's schema |
| `kite artifact [--offset N --limit N] <artifact-id>` | Retrieve an artifact |
| `kite context [--full] [session-id]` | Show the session context |

Exit codes: `0` completed, `1` runtime or verification failure, `2` usage or
configuration error.

## Tools the agent can use

- `read` — print a file with line numbers or list a directory; optional line
  range; large files stored as artifacts.
- `edit` — replace an exact block of text; atomic writes; preserved
  permissions.
- `bash` — run a shell command (30s timeout, process-tree kill); optional
  relative working directory; `purpose: "verification"` marks a verification
  run.
- `artifact` — retrieve a stored artifact by ID and byte offset (up to 32
  KiB).

## Layout

- `kite.go` — the root public façade
- `cmd/kite` — the CLI entry point
- `internal/core` — neutral types and the agent loop
- `internal/provider/openai` — the streaming OpenAI-compatible adapter
- `internal/tools` — the read, edit, bash, and artifact tools
- `internal/rpc` — the NDJSON RPC protocol
- `internal/crush` — the `--from-crush` import
- `docs/agents` — the agent documentation reference
- `docs/schemas/v1` — versioned JSON schemas
- `examples/agents` — executable examples

## Documentation

- [Quickstart](docs/agents/quickstart.md)
- [Architecture](docs/agents/architecture.md)
- [Go API](docs/agents/go-api.md)
- [CLI](docs/agents/cli.md)
- [Tools](docs/agents/tools.md)
- [Events](docs/agents/events.md)
- [Sessions](docs/agents/sessions.md)
- [Artifacts](docs/agents/artifacts.md)
- [Context](docs/agents/context.md)
- [RPC](docs/agents/rpc.md)
- [Providers](docs/agents/providers.md)
- [Security](docs/agents/security.md)
- [Troubleshooting](docs/agents/troubleshooting.md)
- [Recipes](docs/agents/recipes.md)

## Build and test

```sh
go build ./...
go vet ./...
go test ./...
```

Cross-compile every release target:

```sh
GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/kite
GOOS=linux GOARCH=arm64 go build -o /dev/null ./cmd/kite
GOOS=darwin GOARCH=amd64 go build -o /dev/null ./cmd/kite
GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/kite
GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/kite
```

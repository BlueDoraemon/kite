# Kite

A minimal, embeddable agent runtime for Go. Kite provides a small set of building blocks for running agents reliably: sessions, model providers, tools, structured events, artifacts, context management, policy, and execution. It is designed to work on its own or underneath supervisors and orchestrators.

> Small core. Open interfaces. Easy to compose.

## Why Kite?

Agent runtimes tend to grow quickly. Kite takes the opposite approach. The core handles the parts most agents need:

```text
Prompt ↓
Session ↓
Context ↓
Provider ↓
Tool calls ↓
Results
```

## kite

Kite is also a minimal command-line agent that can explain and modify a repository.
You give it a prompt and it drives a model that has read, edit, and bash tools
for working in the current directory.

### Usage

Set the model endpoint and key, then run:

```sh
export KITE_API_KEY=sk-...
kite run "explain this repository"
kite run "add a --retries flag to the upload command"
```

Configuration is via environment variables or flags:

| Variable | Flag | Default | Purpose |
| --- | --- | --- | --- |
| `KITE_API_KEY` | (none) | unset | API key sent as a Bearer token |
| `KITE_BASE_URL` | `-base-url` | `https://api.openai.com/v1` | OpenAI-compatible API base URL |
| `KITE_MODEL` | `-model` | `gpt-4o-mini` | Model identifier |

After a run, any uncommitted working-tree changes the agent made are shown as a
`git diff`.

### Tools the agent can use

- `read` print a file with line numbers, or list a directory
- `edit` replace an exact block of text in a file (optionally every occurrence)
- `bash` run a shell command in the working directory (30s timeout)

### Layout

- `cmd/kite` — the CLI entry point
- `internal/kite` — core types (`Session`, `Message`, `Tool`, `Provider`,
  `Reply`, `Event`) and the agent loop
- `internal/provider/openai` — an OpenAI-compatible chat completions adapter
  (the wire format stays here, not in the core)
- `internal/tools` — the read, edit, and bash tools

The agent loop depends only on the small `Provider` and `Tool` interfaces, never
on the CLI. Everything outside `cmd/` lives under `internal/`. Standard library
only, and `context.Context` is used for cancellation throughout.

### Build and test

```sh
go build ./...
go test ./...
```

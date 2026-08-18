# CLI

`kite` is the command-line agent.

## Commands

| Command | Purpose |
| --- | --- |
| `kite run [flags] <prompt>` | Run a prompt in the current directory |
| `kite resume <session-id> [prompt]` | Resume a session |
| `kite rpc` | Serve the NDJSON RPC protocol on stdin/stdout |
| `kite status [session-id]` | Show session status |
| `kite inspect <tool-id>` | Show a tool's schema |
| `kite artifact <artifact-id> [--offset N --limit N]` | Retrieve an artifact |
| `kite context [session-id] [--full]` | Show the session context |

## Flags

| Flag | Applies to | Purpose |
| --- | --- | --- |
| `-base-url <url>` | run, resume, rpc | OpenAI-compatible API base URL |
| `-model <id>` | run, resume, rpc | Model identifier |
| `-from-crush` | run, resume, rpc | Import model, credential, and endpoint from Crush |
| `-no-print` | run | Do not mirror output to stdout |
| `-offset N` | artifact | Byte offset to read from |
| `-limit N` | artifact | Maximum bytes to read |
| `-full` | context | Show full context including repository instructions |

## Environment variables

| Variable | Default | Purpose |
| --- | --- | --- |
| `KITE_API_KEY` | unset | API key sent as a Bearer token |
| `KITE_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible API base URL |
| `KITE_MODEL` | `gpt-4o-mini` | Model identifier |
| `KITE_DATA_DIR` | XDG/LOCALAPPDATA | Where sessions and artifacts are stored |

Precedence: explicit flags → Kite environment variables → `--from-crush`
import → defaults.

## Exit codes

- `0` — completed
- `1` — runtime or verification failure
- `2` — usage or configuration error

## Human output

`kite run` mirrors the model's text to stdout as it streams, then prints a
structured result block. `kite rpc` keeps stdout protocol-only; all
diagnostics go to stderr.

## `--from-crush`

Reads the persisted Crush-selected large model, credential, and cached
endpoint without executing crushrc. Supports OpenAI and OpenAI-compatible
providers. Rejects Hyper, unsupported providers, missing endpoints, and
expired or near-expiry OAuth credentials with actionable secret-free errors.

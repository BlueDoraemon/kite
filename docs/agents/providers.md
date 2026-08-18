# Providers

Kite ships one provider: an OpenAI-compatible chat completions adapter that
streams over Server-Sent Events.

## Configuration

| Variable | Flag | Default | Purpose |
| --- | --- | --- | --- |
| `KITE_API_KEY` | (none) | unset | API key sent as a Bearer token |
| `KITE_BASE_URL` | `-base-url` | `https://api.openai.com/v1` | API base URL |
| `KITE_MODEL` | `-model` | `gpt-4o-mini` | Model identifier |

Without `--from-crush`, explicit flags override Kite environment variables,
which override defaults. With `--from-crush`, explicit `-base-url` and
`-model` flags override imported values, and `KITE_API_KEY` overrides the
imported credential. `KITE_BASE_URL` and `KITE_MODEL` are only consulted when
Crush import is not active.

## Streaming behaviour

The provider sends a streaming chat completions request and parses the SSE
stream:

- Text arrives as `content` deltas.
- Tool calls arrive fragmented and are assembled by index.
- Usage arrives in the final chunk.
- `[DONE]` ends the stream.

## Errors

Errors are sanitised and structured:

- Non-200 responses carry the HTTP status.
- Malformed SSE chunks carry `malformed_sse`.
- Stream read failures carry `stream`.

There is no automatic retry after streamed output, by design.

## Crush import

`--from-crush` reads the persisted Crush-selected large model, credential, and
cached endpoint without executing crushrc.

Limitations:

- Supports OpenAI and OpenAI-compatible providers only.
- The Hyper provider is rejected.
- OAuth credentials that are expired or within 5 minutes of expiry are
  rejected.
- Errors are secret-free and actionable.

## See also

- [Troubleshooting](troubleshooting.md)
- [Security](security.md)

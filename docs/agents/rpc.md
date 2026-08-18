# RPC

Kite exposes an NDJSON RPC protocol on stdin/stdout via `kite rpc`.

## Framing

Requests and responses are newline-delimited JSON objects. A 1 MiB line limit
bounds each record. The server processes requests sequentially.

## Methods

| Method | Params | Result |
| --- | --- | --- |
| `prompt` | `{"text": "..."}` | `Result` |
| `resume` | `{"session_id": "...", "prompt": "..."}` | `Result` |
| `status` | `{"session_id": "..."}` | session status |
| `inspect` | `{"tool_id": "..."}` | tool name, description, schema |
| `artifact` | `{"artifact_id": "...", "offset": 0, "limit": 32768}` | artifact content |
| `context` | `{"session_id": "...", "full": false}` | context messages |

RPC always returns the complete context. The optional `full` field is accepted
for parity with the CLI contract but does not alter the RPC result in v1.

## Request

```json
{"id": "1", "method": "prompt", "params": {"text": "explain this repo"}}
```

## Response

```json
{"id": "1", "method": "prompt", "ok": true, "result": {"status": "completed", ...}}
{"id": "2", "method": "inspect", "ok": false, "error": {"code": "not_found", "message": "tool not found"}}
```

## Streaming responses

The `prompt` and `resume` methods block until the run completes and return the
structured result. Event-level streaming over RPC is not part of v1.

## Errors

Errors carry a `code` and a `message`. They are sanitised and never include
secrets.

## Limits

- 1 MiB per line.
- Sequential processing (one request at a time).
- Artifact retrieval capped at 32 KiB per call.

## Complete transcript

```text
$ printf '%s\n' \
  '{"id":"1","method":"inspect","params":{"tool_id":"read"}}' \
  '{"id":"2","method":"status"}' | kite rpc
{"id":"1","method":"inspect","ok":true,"result":{...}}
{"id":"2","method":"status","ok":true,"result":{"sessions":[...]}}
```

## See also

- [Schema](../schemas/v1/rpc-request.json)
- [Schema](../schemas/v1/rpc-response.json)
- [Examples](../../examples/agents/rpc-client)

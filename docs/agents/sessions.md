# Sessions

Sessions are durable and resumable. State lives in the data directory
(`$KITE_DATA_DIR`, else XDG data storage on Unix or LOCALAPPDATA on Windows)
under user-only permissions.

## Storage layout

```text
<data-dir>/
  sessions/<session-id>.jsonl    # append-only event log
  sessions/<session-id>.lease    # per-session write lease
  artifacts/<session-id>/<artifact-id>  # stored large outputs
```

## JSONL durability

Every event is appended to the session's JSONL log before it is published.
On load, truncated or malformed trailing records are ignored, so a crash
mid-write cannot corrupt the log.

## Leases

A cross-platform per-session lease with a heartbeat prevents concurrent
writers. Acquiring a lease for a session that is already leased fails. A
stale lease (from a crashed process) is recovered automatically after the TTL
expires. Lease heartbeats, event appends, and release validate an ownership
token, so a superseded writer cannot modify or release the current lease.

## Replay

`LoadSession` reconstructs a session from its durable event log: user
messages, assistant text, tool calls, and tool results are replayed in order.

## Interruption

Unfinished tool calls are recorded as interrupted and never replayed
automatically. A resumed session does not re-run interrupted tools.

## Resume

`kite resume <session-id>` appends a standard continuation instruction. An
optional prompt replaces it:

```sh
kite resume sess_... "now also add a README"
```

Resume only from complete durable turns.

`kite tui [session-id]` uses the same load and replay path, then keeps the
session open for multiple prompts. `/quit` leaves the append-only log intact.

## See also

- [Events](events.md)
- [Recipes](recipes.md) — resume after cancellation

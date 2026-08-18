# Troubleshooting

## Authentication

**Symptom:** `kite run` fails with a 401.

**Fix:** Set `KITE_API_KEY` to a valid key, or use `--from-crush`. Check the
base URL points at the right API root.

## Unsupported Hyper provider

**Symptom:** `--from-crush` fails with "the Hyper provider is not supported".

**Fix:** Kite supports OpenAI and OpenAI-compatible providers only. Configure
a supported provider in Crush, or set `KITE_API_KEY`/`KITE_BASE_URL`/`KITE_MODEL`
directly.

## Expired Crush tokens

**Symptom:** `--from-crush` fails with "the OAuth credential has expired or is
near expiry".

**Fix:** Refresh the credential in Crush and retry. Kite rejects OAuth
credentials within 5 minutes of expiry.

## Malformed SSE

**Symptom:** `kite run` fails with `malformed_sse`.

**Fix:** The endpoint returned a non-SSE or malformed stream. Check the base
URL is an OpenAI-compatible chat completions endpoint.

## Interrupted sessions

**Symptom:** A session was cancelled mid-run and some tool calls never
finished.

**Fix:** Interrupted tool calls are recorded and never replayed. Resume the
session with `kite resume <session-id>`; it continues from the last complete
durable turn.

## Stale leases

**Symptom:** `kite run` fails with "session is leased by another writer".

**Fix:** A previous process crashed while holding the lease. The lease is
recovered automatically after the TTL expires; wait a few minutes or remove
the `.lease` file if you are sure no other process is running.

## Verification failures

**Symptom:** The result reports a failed verification.

**Fix:** The verification command exited non-zero. Read the tool output, fix
the issue, and re-run. Worktree changes after a verification mark the result
stale until verification runs again.

## Platform shell differences

**Symptom:** A bash command behaves differently on Windows.

**Fix:** POSIX uses `sh -c`; Windows uses `cmd.exe /C`. Avoid shell-specific
syntax in commands you expect to run cross-platform.

## See also

- [Providers](providers.md)
- [Security](security.md)
- [Recipes](recipes.md)

# Security

Kite is designed with a small, explicit security model.

## Credential handling

- API keys are read from environment variables or `--from-crush`, never
  hard-coded.
- Credentials are never logged.
- Errors are sanitised and secret-free. A provider error never includes the
  API key, OAuth token, or full request body.

## Repository trust

- `AGENTS.md` is loaded from the nearest repository root, capped at 64 KiB.
- The absolute source path is recorded so instructions are auditable.

## Prompt injection

Repository instructions and tool outputs are treated as untrusted model
input. The fixed system instructions tell the model to work inside the
repository working directory only.

## Shell execution

- Commands run with a 30-second timeout.
- On timeout, the whole process tree is killed (POSIX process groups, Windows
  `taskkill /T`).
- Verification runs are marked with `purpose: "verification"`.

## Filesystem containment

- `read` and `edit` resolve symlinks and reject paths that escape the working
  directory.
- `edit` writes atomically and preserves permissions.
- Bash `working_dir` is resolved through the same containment check.

## Sensitive logs and artifacts

- Artifacts may contain repository content; they are stored with user-only
  permissions.
- Events and errors never include artifact contents.
- The data directory defaults to user-only XDG/LOCALAPPDATA storage.

## Redaction guarantees

- No API keys, OAuth tokens, or credentials in any event, error, log, or RPC
  response.
- No full request bodies in errors (bounded, truncated bodies only).

## See also

- [Troubleshooting](troubleshooting.md)
- [Providers](providers.md)

# Tools

The agent has four built-in tools. All paths are repository-relative and
symlink-safe: a path that resolves outside the working directory is rejected.

## read

Print a file with line numbers, or list a directory. Optional line range.

```json
{"path": "internal/core/run.go"}
{"path": "internal/core/run.go", "start_line": 10, "end_line": 30}
{"path": "."}
```

- Large results follow the shared [artifact limits](artifacts.md).

## edit

Replace an exact block of text. Writes are atomic and preserve permissions.

```json
{"path": "main.go", "old_text": "foo", "new_text": "bar"}
{"path": "main.go", "old_text": "foo", "new_text": "bar", "apply_all": true}
```

- `old_text` must match exactly, including whitespace.
- If `old_text` is not found, the tool reports a mismatch diagnostic.
- Containment is symlink-safe.

## bash

Run a shell command in the working directory. Optional relative working
directory. A 30-second timeout kills the whole process tree.

```json
{"command": "go test ./..."}
{"command": "pwd", "working_dir": "sub"}
{"command": "go build ./...", "purpose": "verification"}
```

- Separate stdout/stderr are captured together; a non-zero exit is reported
  as `exit status N` with the output.
- `purpose: "verification"` marks a verification run. Exit zero means passed;
  non-zero means failed. Later worktree changes make the result stale until
  verification runs again.
- POSIX uses `sh -c` and process groups; Windows uses `cmd.exe /C` and
  `taskkill /T`.

## artifact

Retrieve a stored artifact by ID and byte offset.

```json
{"id": "art_...", "offset": 0, "limit": 32768}
```

- Returns up to 32 KiB per call.
- IDs are globally unique, so no session argument is needed.

## Output limits

- Bash captures at most 32 KiB before the runtime applies the inline cap.
- Shared inline, preview, and retrieval limits are owned by
  [Artifacts](artifacts.md).

# Quickstart

This guide walks you through a first task end-to-end: run, inspect, fix,
verify, and interpret a result.

## 1. Run

```sh
export KITE_API_KEY=sk-...
kite run "create a file called hello.txt containing 'hello kite'"
```

Kite streams the model's output, runs tools, and prints a structured result
when it finishes:

```text
--- result ---
status: completed
changed files: hello.txt
verification: passed (exit 0)
```

## 2. Inspect

See what tools the agent can use and their exact schemas:

```sh
kite inspect bash
kite inspect read
kite inspect edit
kite inspect artifact
```

## 3. Fix

Ask the agent to fix a problem. It reads, edits, and verifies:

```sh
kite run "the build is failing; find the error and fix it"
```

## 4. Verify

Verification runs are bash commands with `purpose: "verification"`. A zero
exit means passed; a non-zero exit means failed. If the worktree changes after
a verification, the result is marked stale until verification runs again.

## 5. Interpret a result

The structured result contains:

- `status` — `completed` or `failed`
- `text` — the final assistant text
- `changed_files` — files modified during the current prompt
- `changed_files_complete` — whether worktree inspection found the complete set
- `verification` — the last verification command, status, exit code, and
  artifacts
- `usage` — aggregated token usage

## Next steps

- [Architecture](architecture.md) — how the pieces fit together
- [CLI](cli.md) — every command and flag
- [Recipes](recipes.md) — common workflows

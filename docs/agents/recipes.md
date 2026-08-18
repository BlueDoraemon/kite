# Recipes

## Test-and-fix

```sh
kite run "the test suite is failing; find the failures and fix them"
```

The agent reads, edits, and runs a verification bash command with
`purpose: "verification"`. A passing verification means the fix holds.

## Resume after cancellation

```sh
kite run "add a --retries flag to the upload command"   # cancelled mid-run
kite resume sess_...                                   # continue from the last complete turn
```

Interrupted tool calls are never replayed; the session continues from the
last complete durable turn.

## Inspect large output

```sh
kite run "list every file in this repository"           # output stored as an artifact
kite artifact --offset 0 --limit 32768 art_...          # page through it
kite artifact --offset 32768 --limit 32768 art_...      # next page
```

## Orchestrate through RPC

```sh
printf '%s\n' \
  '{"id":"1","method":"inspect","params":{"tool_id":"bash"}}' \
  '{"id":"2","method":"prompt","params":{"text":"explain this repo"}}' \
  '{"id":"3","method":"status"}' | kite rpc
```

## Inspect context

```sh
kite context                 # what a fresh session in this directory will see
kite context --full sess_... # full context including repository instructions
```

## Use --from-crush

```sh
kite run --from-crush "explain this repository"
```

Reuses the Crush-selected large model, credential, and endpoint without
executing crushrc. See [Providers](providers.md) for configuration precedence.

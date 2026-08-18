# Agent examples

Executable examples for the Kite runtime. All examples use placeholders and
never contain real credentials, local absolute paths, or persisted user data.

## go-session

A Go API session driver: create a session, drive a prompt, and read the
resulting events.

```sh
go run ./examples/agents/go-session
```

## custom-tool

A custom `kite.Tool` (uppercases its input) passed to a session.

```sh
go run ./examples/agents/custom-tool
```

## rpc-client

An NDJSON RPC client that talks to `kite rpc` over stdin/stdout.

```sh
KITE_API_KEY=sk-... go run ./examples/agents/rpc-client
```

## recipes.sh

Shell recipes for run, resume, artifact, context, and RPC.

```sh
sh ./examples/agents/recipes.sh
```

## Notes

- The Go API examples use small self-contained providers so they can be copied
  into another module. The RPC example and shell recipes use the CLI's
  OpenAI-compatible provider; configure it with `KITE_API_KEY`,
  `KITE_BASE_URL`, and `KITE_MODEL`, or `--from-crush` where shown.
- Examples are compiled and checked by the documentation validation suite.

#!/usr/bin/env sh
# recipes.sh demonstrates the kite CLI workflows: run, resume, artifact, and
# context. Set KITE_API_KEY (or use --from-crush) before running.
set -e

echo "== run =="
kite run "explain this repository"

echo "== inspect =="
kite inspect bash

echo "== context =="
kite context

echo "== run that produces an artifact =="
kite run "list every file in this repository"

echo "== list sessions and resume =="
kite status
# Replace SESSION_ID with a session from the status output.
# kite resume "$SESSION_ID"

echo "== retrieve an artifact =="
# Replace ART_ID with an artifact id from a tool result.
# kite artifact "$ART_ID" --offset 0 --limit 32768

echo "== rpc =="
printf '%s\n' \
  '{"id":"1","method":"inspect","params":{"tool_id":"read"}}' \
  '{"id":"2","method":"status"}' | kite rpc

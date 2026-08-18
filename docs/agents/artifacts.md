# Artifacts

Large tool outputs are stored as artifacts so they do not flood the model
context.

## Creation threshold

Outputs larger than 16 KiB are stored as artifacts. The tool result carries a
compact preview (8 KiB head/tail) with the artifact ID, size, media type, and
truncation metadata.

## Retrieval

Retrieve an artifact by ID and byte offset:

```sh
kite artifact art_... --offset 0 --limit 32768
```

```json
{"id": "art_...", "offset": 0, "limit": 32768}
```

Each call returns up to 32 KiB. IDs are globally unique, so no session
argument is needed.

## Storage

Artifacts are stored under `<data-dir>/artifacts/<session-id>/<artifact-id>`
with user-only permissions.

## Retention

Artifacts persist with the session. There is no automatic expiry; remove the
session's artifact directory to reclaim space.

## Sensitivity

Artifacts may contain repository content. They are stored with user-only
permissions and are never included in error messages or logs. Treat them as
sensitive to the repository they came from.

## See also

- [Tools](tools.md) — the artifact tool
- [Security](security.md) — sensitive artifacts and redaction guarantees

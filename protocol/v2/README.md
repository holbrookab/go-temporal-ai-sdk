# Temporal durable updates protocol v2

This directory is the language-neutral contract shared by
`go-temporal-ai-sdk` and `@holbrookab/temporal-ai-connectors`.
`go-ai` is deliberately outside this protocol: it produces AI SDK-compatible
model and tool parts, while the Temporal layer adds attempt, acceptance,
persistence, and replay semantics.

## Naming

- Exported Go and TypeScript symbols use `PascalCase`.
- JSON fields use `camelCase`.
- Wire discriminants, multiword statuses, and interaction types use
  `kebab-case`.
- Protocol fields never use `snake_case`.

## Events

Every live or replayed update is one of:

- `preview-begin`
- `preview-chunk`
- `preview-snapshot`
- `preview-end`
- `record-upsert`
- `stream-end`

`eventId` is the semantic idempotency key. Durable delivery also has a
store-assigned `cursor`; callers must never compare or paginate by `eventId`.
The store assigns a cursor once and returns the original cursor on an
idempotent retry.

### Preview ordering

- `sequence` is monotonic within one `attemptId`, beginning at zero.
- Duplicate event IDs and sequences at or below the applied sequence are
  ignored.
- `preview-end` with `succeeded` remains renderable until a record names that
  exact attempt in `acceptedAttemptId`.
- `failed` and `canceled` previews stop rendering immediately.
- Events received after an attempt is failed, canceled, or accepted are
  ignored.
- `targetRecordId` says what the preview may eventually become, but does not
  itself supersede anything.

### Record ordering and acceptance

- `recordVersion` is monotonic within one `recordId`, beginning at one.
- A lower version is stale and ignored. Reapplying the same version and
  `eventId` is idempotent success. Reusing a version with a different event ID
  or payload is a conflict.
- `acceptedAttemptId` is optional. When present, that attempt must target the
  same `recordId`; only that exact attempt is superseded.
- Records are complete current snapshots, not patches.
- Supported first-party record kinds are `message`, `tool`, `interaction`,
  `task`, and `subagent`.

### Stream termination

`preview-end` terminates one provider activity attempt. `stream-end` terminates
the overall subscription and is persisted only after all accepted records are
readable. A client that receives `stream-end` performs one final replay through
its cursor before closing.

## Replay

Replay uses `replay-response.schema.json` and contains:

- durable events after the requested cursor;
- current canonical record snapshots;
- active or succeeded, unsuperseded preview manifests;
- an optional persisted terminal `stream-end` event.

Failed, canceled, and superseded attempts are retained for a bounded audit TTL
but are excluded from ordinary replay.

## Projection rule

Preview text is transient application data and must not be emitted as native AI
SDK `text-*` chunks because those chunks cannot be rolled back. Only an accepted
`message` record becomes canonical native text. Tool and `tool-approval`
interaction records may additionally project to native AI SDK tool chunks.

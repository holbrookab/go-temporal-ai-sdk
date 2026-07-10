# Temporal preview and durable record guide

## Ownership boundary

`go-ai` is the provider/runtime layer. It knows model parts, tool calls,
structured output, provider metadata, and signed approval fields. It does not
know Temporal attempts or workflow acceptance.

`go-temporal-ai-sdk` owns:

- activity attempts and retry-distinct `attemptId` values;
- provisional preview publication and checkpoints;
- workflow-authored canonical records;
- idempotent record versions and deterministic event IDs;
- persisted stream termination.

The TypeScript connector owns client replay, projection, and AI SDK UI chunks.

## Wire naming

- Exported Go identifiers use `PascalCase`, such as `PreviewEndEvent`.
- JSON fields use `camelCase`, such as `acceptedAttemptId`.
- Wire discriminants use `kebab-case`, such as `record-upsert`.
- Multiword kinds, statuses, and interaction types use `kebab-case`.

The event union contains exactly `preview-begin`, `preview-chunk`,
`preview-snapshot`, `preview-end`, `record-upsert`, and `stream-end`.

## Provider attempts versus workflow acceptance

A model activity creates a retry-specific base attempt ID from the configured
attempt ID and the Temporal activity attempt number. Each output lane has its
own final attempt ID. Text, reasoning, object, and tool-input output are never
combined across retries.

`preview-end` means that one provider activity attempt ended. Its outcomes are
`succeeded`, `failed`, and `canceled`. `succeeded` does not mean the conversation
accepted the output. A workflow accepts output only by writing a
`record-upsert` with the receipt's exact `attemptId` in `acceptedAttemptId`.

The UI therefore keeps a succeeded preview visible while `WriteRecord` runs,
then replaces only that exact preview when the record arrives. Failed and
canceled attempts stop rendering immediately. Bundled stores retain their
hidden manifests for the configured audit TTL, one hour by default.

## Relay and receipts

Pass `updates.Options` in `ProviderOptions["temporal"]`. The activity removes
that provider option before invoking the real model.

```go
updates.Options{
    Visible:                 true,
    StreamID:                "conversation-1",
    TargetRecordID:          "message:assistant-1",
    AttemptID:               "turn-7",
    SnapshotEveryChunks:     16,
    SnapshotEveryCharacters: 1024,
    FailurePolicy:           updates.FailurePolicyStrict,
    Scope: updates.Scope{
        DisplayMode: updates.DisplayModeAssistant,
        AgentID:     "assistant",
    },
}
```

Model and object activity results contain `PreviewReceipts`. A receipt contains
the exact attempt and target record IDs, lane, final sequence, outcome,
snapshot, and scope. The workflow uses the receipt to author the record; the
activity never makes itself canonical.

## Canonical records

`WorkflowRecord` is a complete current snapshot, not a patch. It has a stable
`recordId`, monotonically increasing `recordVersion`, first-party `kind`,
status, data, scope, and `updatedAt`.

The first-party kinds are:

- `message`: accepted text, reasoning, object data, and message status;
- `tool`: input, output/error metadata, and lifecycle status;
- `interaction`: questions, choices, answers, origin, and gate status;
- `task`: task identity, dependencies, result, and lifecycle;
- `subagent`: child workflow identity, progress, result, and lifecycle.

Stores ignore lower record versions. The same version plus the same event ID
and payload is idempotent success. Reusing a version with a different payload
is `updates.ErrRecordConflict`.

The semantic event payload includes `acceptedAttemptId`. Reusing a deterministic
event ID with a different accepted attempt is rejected as
`updates.ErrEventConflict`; it can never supersede a second preview under an
already-persisted record version.

`temporalai.WriteRecord` schedules `activities.WriteRecordActivity` separately
from model/tool activities. This is the important cost and side-effect boundary:
a retry caused by persistence or fanout does not repeat the model or tool.

## Tool approvals

`temporalai.RequestToolApproval` writes a pending interaction record before it
waits. The interaction contains an explicit question with Approve and Deny
choices. After a response, timeout, or cancellation, it writes version two with
the answer and terminal status.

Tool execution itself contains no connector writes. The workflow writes tool
record version one before execution and version two after success, failure, or
denial. This keeps tool side effects outside the record retry envelope.

Provider-native signed approval request/response fields continue to round-trip
through `activities.Part` and `activities.StreamPart`. They are origin metadata,
not an implicit workflow human gate.

## Object, task, and subagent examples

Structured output uses the `object` lane and stores the latest partial object
and emitted elements in preview snapshots. Canonical object data can be written
as a `message` record with `data.object` and `data.elements`.

Tasks and subagents use ordinary versioned records:

```go
record := updates.WorkflowRecord{
    RecordID:      "task:research-1",
    RecordVersion: 3,
    Kind:          updates.RecordKindTask,
    Status:        "completed",
    Data: map[string]any{
        "taskId": "research-1",
        "title":  "Research",
        "result": map[string]any{"text": "Done"},
    },
    Scope: updates.Scope{DisplayMode: updates.DisplayModeTask, TaskID: "research-1"},
}
```

The existing child workflow query and signal APIs remain the orchestration
source of truth. Child agents also write each `SubagentSnapshot` as a
`subagent` record with `recordVersion` equal to its monotonic sequence before
signaling progress to the parent.

## Failure matrix

| Failure | Behavior |
| --- | --- |
| Preview begin/checkpoint/live publish | Strict by default; model activity may retry and incur provider cost. |
| Typed missing stream in best-effort mode | Preview relay disables itself; model result still returns. |
| Provider error | Activity emits `preview-end: failed`; Temporal may retry with a distinct attempt ID. |
| Activity cancellation | Caller may emit `preview-end: canceled`; the preview stops rendering. |
| Record persistence | Separate idempotent record activity retries; model/tool is not rerun. |
| Record live fanout | Composite connector persists first; activity retry reuses semantic IDs/cursors. |
| Duplicate record event | Same version/event/payload succeeds idempotently. |
| Stale record version | Ignored. |
| Conflicting same version | Returns `updates.ErrRecordConflict`. |
| `stream-end` delivery | Persisted terminal is replayable; clients perform one final replay before close. |

## Replay and adapters

The frozen replay response includes cursor events, current canonical records,
active/succeeded unsuperseded preview manifests, and an optional persisted
terminal event. Event IDs are semantic idempotency keys; cursors are
store-assigned ordering keys and are reused on idempotent retries.

Both bundled adapters store preview manifests with an audit TTL, canonical
record snapshots, durable cursor events, and terminal state in DynamoDB.
AppSync or Redis receives the same protocol-v2 JSON envelope without a second
adapter-specific chunk vocabulary.

The default DynamoDB replay indexes match the TypeScript connector:

- `updateStreamId-updateCursor-index`;
- `previewStreamId-previewUpdatedAt-index`;
- `recordStreamId-recordUpdatedAt-index`.

The attribute and index names remain configurable on both sides.

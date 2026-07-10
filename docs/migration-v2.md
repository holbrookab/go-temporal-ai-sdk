# v0.3 to v0.4 migration

v0.4 is an intentional clean break. The v1 `streaming` package and lifecycle
event vocabulary are removed.

| Removed v0.3 symbol or concept | v0.4 replacement |
| --- | --- |
| `streaming` package | `updates` package |
| `streaming.Connector` | `updates.Connector` |
| `streaming.Store` / `streaming.Publisher` | `updates.PreviewStore`, `updates.RecordStore`, `updates.LivePublisher` |
| `streaming.NewCompositeConnector` | `updates.NewCompositeConnector` |
| `activities.Options.StreamConnector` | `activities.Options.UpdateConnector` |
| `streaming.Options` | `updates.Options` |
| `streaming.Scope` | `updates.Scope` |
| `streaming.Lane*` | `updates.Lane*` |
| `StartAttempt` | `BeginPreview` |
| `PublishLiveChunk` | `PublishUpdate` with `PreviewChunkEvent` |
| `UpdateAttemptSnapshot` | `CheckpointPreview` |
| `CompleteAttempt` | `EndPreview` |
| `attempt-commit` / committed | `preview-end` / `succeeded` |
| attempt discard/fail/cancel variants | `preview-end` with `failed` or `canceled` |
| `PublishToolLifecycleEventActivity` | `WriteRecordActivity` |
| `PublishToolLifecycleEvent` | `temporalai.WriteRecord` |
| `PublishToolLifecycleEventArgs` | `activities.WriteRecordArgs` |
| `ToolLifecycleOptions` | workflow-authored `updates.WorkflowRecord` plus `updates.Scope` |
| `InvokeToolArgs.Lifecycle` | `InvokeToolArgs.Scope`; record writes move to the workflow |
| `InvokeToolArgs.SuppressInputLifecycle` | removed; tool activities never publish lifecycle records |
| `ActivityOptions.Stream` | `ActivityOptions.Record` |
| terminal control chunk | `temporalai.EndStream` / `stream-end` |
| adapter `PersistEphemeralChunks` | preview manifests with `TTL` |
| adapter attempt/ephemeral entity options | preview/record/terminal entity options |

Model/object results now expose `PreviewReceipts`. Use the successful receipt's
`AttemptID` as `acceptedAttemptId` when calling `temporalai.WriteRecord`.

`temporalai.RunAgent` and `temporalai.RequestToolApproval` protect their new
record activity commands with the Temporal change ID
`go-temporal-ai-sdk.durable-records-v2`. Existing histories take the
`DefaultVersion` branch and retain their v0.3 command sequence; new executions
write v2 records. Keep this version marker until no pre-v0.4 histories can
replay.

Wire migration details and fixtures are in [`../protocol/v2/MIGRATION.md`](../protocol/v2/MIGRATION.md).

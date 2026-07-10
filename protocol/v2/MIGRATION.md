# Protocol v1 to v2 migration map

| v1 concept | v2 replacement |
| --- | --- |
| `provider-live` phase | Preview events; phases are removed |
| `canonical` phase | `record-upsert` written by the workflow |
| `stream-start` | `preview-begin` |
| Provider delta events | `preview-chunk.chunk` |
| Attempt snapshot event | `preview-snapshot` or replay preview manifest |
| `attempt-commit` / `committed` | `preview-end` / `succeeded` |
| `attempt-discard` / `discarded` | `preview-end` / `failed` |
| `attempt-cancel` / `canceled` | `preview-end` / `canceled` |
| `attempt-fail` / `failed` | `preview-end` / `failed` |
| Terminal `__control: done` | Persisted `stream-end` |
| Tool lifecycle event family | Versioned `tool` records |
| Tool approval plus synthesized checkpoint | One `tool-approval` interaction record |
| `data-llm-stream` | `data-workflow-preview` |
| `data-human-checkpoint` | `data-workflow-record` with kind `interaction` |
| `data-task-event` | `data-workflow-record` with kind `task` |
| Durable assistant ID inferred by transport | Explicit `targetRecordId` and `recordId` |
| Event ID used as replay cursor | Separate semantic `eventId` and sortable `cursor` |
| Persisted ephemeral chunks | Periodic preview manifests with bounded audit TTL |

The Go and TypeScript implementations must extend this table with exact removed
public symbols before their breaking releases are tagged.

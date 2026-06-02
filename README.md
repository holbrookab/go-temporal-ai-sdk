# go-temporal-ai-sdk

Temporal-native runtime pieces for `github.com/holbrookab/go-ai`.

This package is not a direct TypeScript parity port. The goal is to make Temporal
the durable execution backend for Go AI agents, model calls, tool calls, and
visible streams.

## Current Surface

- `activities`: worker-side activities that call real `go-ai` providers.
- `temporalai`: workflow-safe helpers that schedule those activities.
- `streaming`: connector contracts for live/replay stream infrastructure.
- `connectors/appsync-dynamodb`: AppSync Events + DynamoDB connector adapter.
- `connectors/redis-dynamodb`: Redis live transport + DynamoDB replay adapter.

Go Temporal workflows use `workflow.Context`, while `go-ai` uses
`context.Context`, channels, and goroutines. Because of that, workflow code
should use `temporalai` helpers instead of calling `go-ai.GenerateText` or
`go-ai.StreamText` directly.

## Stream Model

The stream model has two phases:

- `provider-live`: provisional chunks published by the model activity while it
  reads the provider stream.
- `canonical`: workflow-owned final state after the activity returns buffered
  stream parts.

Workflows do not receive token-by-token callbacks. The activity streams live
chunks through a `streaming.Connector`, buffers all `go-ai` stream parts, and
returns the buffer to the workflow when the activity completes.

```text
ModelCall activity
  -> provider DoStream
  -> streaming.Connector publishes provider-live chunks
  -> returns buffered stream parts

Workflow
  -> receives completed activity result
  -> owns final state, retries, cancellation, and commits
```

## Connector Shape

```go
type Connector interface {
    StartAttempt(context.Context, AttemptRef) error
    PublishLiveChunk(context.Context, LiveChunk) error
    UpdateAttemptSnapshot(context.Context, AttemptSnapshot) error
    CompleteAttempt(context.Context, AttemptCompletion) error
    PersistToolLifecycleEvent(context.Context, ToolLifecycleInput) error
    PublishLiveToolLifecycleEvent(context.Context, ToolLifecycleInput) error
    PublishToolLifecycleEvent(context.Context, ToolLifecycleInput) error
}
```

`streaming.NewCompositeConnector` combines a durable `Store` and a live
`Publisher`, which matches the common shape of a replay database plus low-latency
fanout transport.

The bundled AppSync/DynamoDB adapter is intentionally generic: applications
decide how a `streamId` resolves to a live channel and replay attributes.

```go
import appsyncdynamodb "github.com/holbrookab/go-temporal-ai-sdk/connectors/appsync-dynamodb"

connector := appsyncdynamodb.New(appsyncdynamodb.Options{
    AWSConfig:          cfg,
    TableName:          "chat-production",
    AppSyncHTTPDomain:  "example.appsync-api.us-west-2.amazonaws.com",
    Resolver: appsyncdynamodb.NewDynamoDBResolver(appsyncdynamodb.DynamoDBResolverOptions{
        DynamoDB:  dynamodb.NewFromConfig(cfg),
        TableName: "chat-production",
    }),
})
```

The Redis/DynamoDB adapter uses the same DynamoDB attempt/replay shape, but sends
live frames through Redis. Use `ModePubSub` for low-latency fanout,
`ModeStream` for Redis Streams, or `ModeBoth` when consumers need Pub/Sub speed
and stream catch-up.

```go
import redisdynamodb "github.com/holbrookab/go-temporal-ai-sdk/connectors/redis-dynamodb"

connector := redisdynamodb.New(redisdynamodb.Options{
    AWSConfig: cfg,
    DynamoDB:  dynamodb.NewFromConfig(cfg),
    Redis: redis.NewUniversalClient(&redis.UniversalOptions{
        Addrs: []string{"localhost:6379"},
    }),
    TableName: "chat-production",
    Mode:      redisdynamodb.ModeBoth,
})
```

## Worker Registration

```go
acts := activities.New(activities.Options{
    ModelProvider:   provider,
    StreamConnector: connector,
    Sandbox:         sandbox,
    Tools: map[string]ai.Tool{
        "lookup": lookupTool,
    },
})

temporalai.RegisterActivities(worker, acts)
```

`Sandbox` is worker-owned configuration. It is passed to `go-ai` tool
executions as `ai.ToolExecutionOptions.Sandbox`, but it is not serialized through
workflow inputs because sandbox implementations are process-local interfaces.

## Workflow Usage

```go
result, err := temporalai.InvokeModel(ctx, "model-id", ai.LanguageModelCallOptions{
    Prompt: []ai.Message{ai.UserMessage("hello")},
})
```

Structured output calls use the same durable activity boundary. The workflow
passes a model ID separately from the serializable `GenerateObjectOptions`.

```go
profile, err := temporalai.GenerateObject(ctx, "model-id", ai.GenerateObjectOptions{
    Prompt:     "Return a user profile for Ada.",
    SchemaName: "profile",
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "name": map[string]any{"type": "string"},
        },
        "required": []any{"name"},
    },
})
```

Visible updates are standardized through the same `streaming.Connector`
configured on `activities.Options.StreamConnector`. Pass Temporal stream
metadata under `ProviderOptions["temporal"]`; activities strip that key before
calling the real model provider. If `Visible` is false or `StreamID` is empty,
the relay is a no-op even when a connector is configured.

`temporalai.InvokeModel`, `temporalai.GenerateObject`,
`temporalai.StreamObject`, and `temporalai.InvokeModelStream` all use this
opt-in shape:

- `InvokeModel` emits attempt start, final text/reasoning/tool/file/source
  chunks, finish, and committed completion.
- `GenerateObject` emits attempt start, optional reasoning, a final object
  snapshot, finish, and committed completion. It defaults to the object lane
  when no lane is provided.
- `StreamObject` wraps `ai.StreamObject`, emits object snapshots and element
  chunks as upstream object parts arrive, drains the upstream `Elements` channel,
  and returns buffered object stream parts/elements to the workflow. It defaults
  to the object lane when no lane is provided.
- `InvokeModelStream` emits provider stream chunks and partial JSON/object
  snapshots as they arrive. With a JSON response format this is still text
  streaming plus partial JSON parsing, not the object-native `StreamObject`
  contract.

```go
stream, err := temporalai.InvokeModelStream(ctx, "model-id", ai.LanguageModelCallOptions{
    ProviderOptions: ai.ProviderOptions{
        "temporal": streaming.Options{
            Visible:  true,
            StreamID: workflow.GetInfo(ctx).WorkflowExecution.ID,
            Lane:     streaming.LaneText,
        },
    },
})
```

The same option can be passed to non-streaming calls when the UI needs visible
progress or final snapshots without using a separate streaming activity.
By default visible streaming is strict: connector failures make the model
activity fail. Set `FailurePolicy` to
`streaming.StreamFailurePolicyBestEffort` when the stream is a telemetry/UI
side-channel and a missing stream row should degrade to no visible updates
instead of retrying the model activity. Best-effort mode only suppresses typed
`streaming.ErrStreamNotFound` errors; auth, throttling, publish, and other
connector failures still propagate.

```go
result, err := temporalai.InvokeModel(ctx, "model-id", ai.LanguageModelCallOptions{
    Prompt: []ai.Message{ai.UserMessage("summarize this")},
    ProviderOptions: ai.ProviderOptions{
        "temporal": streaming.Options{
            Visible:       true,
            StreamID:      workflow.GetInfo(ctx).WorkflowExecution.ID,
            Lane:          streaming.LaneText,
            FailurePolicy: streaming.StreamFailurePolicyBestEffort,
        },
    },
})

profile, err := temporalai.GenerateObject(ctx, "model-id", ai.GenerateObjectOptions{
    Prompt: "Return a user profile for Ada.",
    Schema: map[string]any{"type": "object"},
    ProviderOptions: ai.ProviderOptions{
        "temporal": streaming.Options{
            Visible:       true,
            StreamID:      workflow.GetInfo(ctx).WorkflowExecution.ID,
            FailurePolicy: streaming.StreamFailurePolicyBestEffort,
        },
    },
})
```

Use `StreamObject` when the model output is conceptually an object stream,
especially array output where individual elements should become visible as they
arrive.

```go
items, err := temporalai.StreamObject(ctx, "model-id", ai.StreamObjectOptions{
    GenerateObjectOptions: ai.GenerateObjectOptions{
        Output: ai.OutputArray,
        Prompt: "Return matching contacts.",
        Schema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "name": map[string]any{"type": "string"},
            },
            "required": []any{"name"},
        },
        ProviderOptions: ai.ProviderOptions{
            "temporal": streaming.Options{
                Visible:  true,
                StreamID: workflow.GetInfo(ctx).WorkflowExecution.ID,
            },
        },
    },
})
```

## Durable Agents

`temporalai.RunAgent` is a workflow-side agent loop. Each model step is a model
activity. Tool calls default to regular activities, and workflows can opt into
local tool activities for short idempotent tools by setting an agent default or
per-tool execution boundary. Tool lifecycle chunks are published from inside the
tool activity through the same stream connector used by model streaming. The
durable lifecycle write is part of the activity retry envelope; live fanout is
attempted after the durable write and does not make the activity fail.

```go
result, err := temporalai.RunAgent(ctx, temporalai.AgentInput{
    AgentID:      "researcher",
    ModelID:      "model-id",
    Instructions: "Use tools when useful.",
    Prompt:       "Find the latest durable execution notes.",
    Tools: activities.ToolDefinitionsFromAI(map[string]ai.Tool{
        "lookup": lookupTool,
    }),
    Stream: streaming.Options{
        Visible:  true,
        StreamID: workflow.GetInfo(ctx).WorkflowExecution.ID,
        Lane:     streaming.LaneText,
    },
})
```

The default tool execution mode is parallel. Set
`ToolExecution: temporalai.ToolExecutionSequential` when a workflow wants strict
one-at-a-time tool scheduling.

Tool execution boundaries are independent from parallel/sequential scheduling.
The SDK default is `activities.ToolExecutionBoundaryActivity`, which keeps tool
calls as regular Temporal activities. Use
`DefaultToolBoundary: activities.ToolExecutionBoundaryLocalActivity` when an
agent should run short, idempotent tools as local activities by default, and set
`ExecutionBoundary: activities.ToolExecutionBoundaryActivity` on individual
tools that should always use regular activities.

```go
tools := activities.ToolDefinitionsFromAI(map[string]ai.Tool{
    "lookup": lookupTool,
    "convertOfficeDocumentToPdf": convertOfficeTool,
})
for i := range tools {
    if tools[i].Name == "convertOfficeDocumentToPdf" {
        tools[i].ExecutionBoundary = activities.ToolExecutionBoundaryActivity
    }
}

result, err := temporalai.RunAgent(ctx, temporalai.AgentInput{
    AgentID:                  "researcher",
    ModelID:                  "model-id",
    Prompt:                   "Review these documents.",
    Tools:                    tools,
    DefaultToolBoundary:      activities.ToolExecutionBoundaryLocalActivity,
    LocalToolTimeoutFallback: temporalai.LocalToolTimeoutFallbackActivity,
}, temporalai.ActivityOptions{
    LocalTool: workflow.LocalActivityOptions{
        StartToCloseTimeout: 15 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 1,
        },
    },
})
```

Local tool activities skip the task queue, run on the same worker as the
workflow task, and are intended for short operations where the latency savings
matter. They are still retried and still need idempotency. When a local tool
times out, agents retry that same tool call as a regular activity by default.
Set `LocalToolTimeoutFallback: temporalai.LocalToolTimeoutFallbackNone` to
disable that fallback. Set `ActivityOptions.LocalTool.RetryPolicy` when you want
to control how many local attempts happen before the regular-activity fallback;
`MaximumAttempts: 1` makes the first local timeout switch immediately.

If a tool is slow, weakly idempotent, needs an isolated task queue, or needs a
durable checkpoint immediately after completion, keep it as a regular activity.

Approval-required tools can be marked before they reach the agent loop:

```go
tools := activities.ToolDefinitionsFromAI(map[string]ai.Tool{
    "create_worker": createWorkerTool,
})
for i := range tools {
    if tools[i].Name == "create_worker" {
        tools[i].RequiresApproval = true
    }
}

result, err := temporalai.RunAgent(ctx, temporalai.AgentInput{
    AgentID: "worker-admin",
    ModelID: "model-id",
    Prompt:  "Create the worker if the extracted details look right.",
    Tools:   tools,
    Stream: streaming.Options{
        Visible:  true,
        StreamID: workflow.GetInfo(ctx).WorkflowExecution.ID,
        Lane:     streaming.LaneText,
    },
})
```

When an approval-required tool is called, the workflow publishes AI SDK-shaped
`tool-input-available` and `tool-approval-request` chunks, waits for a Temporal
signal named by `temporalai.ToolApprovalResponseSignalName(approvalID)`, then
publishes `tool-approval-response`. Approved tools receive the approval state in
their activity input; denied or timed-out approvals publish `tool-output-denied`
and do not schedule the tool activity. Plan previews and richer feedback UI
should be represented as assistant text or `data-*` UI chunks; the SDK approval
path is the durable gate before executing a concrete tool call.

Short model calls can also opt into local activity execution. This is useful
for lightweight routing or classification calls where the latency savings are
worth holding the workflow task open briefly. Longer reasoning calls and
visible streaming calls should usually remain regular activities.

```go
route, err := temporalai.InvokeModel(ctx, "router-model", options, temporalai.ActivityOptions{
    LanguageModelBoundary: activities.ToolExecutionBoundaryLocalActivity,
    LocalLanguageModel: workflow.LocalActivityOptions{
        StartToCloseTimeout: 20 * time.Second,
        Summary:             "SelectSkill",
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 1,
        },
    },
})
```

Invoke helpers set Temporal activity summaries by default, so UI/CLI surfaces
can show labels such as `go-temporal-ai-sdk.InvokeModel` and
`go-temporal-ai-sdk.InvokeTool` when the server and UI expose user metadata.

Nested agents can be modeled as child workflows with
`temporalai.ExecuteAgentChildWorkflow`, keeping the parent agent history focused
on child workflow boundaries rather than every nested step.

Tool lifecycle event IDs are stable per tool call: `tool:<toolCallId>:input`,
`tool:<toolCallId>:approval-request`,
`tool:<toolCallId>:approval-response`, and `tool:<toolCallId>:terminal`.
Bundled durable connectors treat duplicate event IDs as idempotent success so
activity retries do not create duplicate start/end frames.

Non-agent Go activities can opt into the same behavior by wrapping their body
with `activities.RunWithToolLifecycle`. The activity input still has to provide
the stream and tool metadata; the SDK will not infer `streamId`, `toolCallId`,
`toolName`, input, or result mapping from an arbitrary activity signature.

## Development

Release commits should depend on tagged `go-ai` versions from `go.mod`. For
local sibling development, use a temporary workspace:

```bash
go work init . ../go-ai
```

Run tests with a workspace-local Go cache if the sandbox cannot write to the
default OS cache:

```bash
GOCACHE=$PWD/.cache/go-build go test ./...
```

`go-ai` provider packages are imported by workers directly. New providers in
`go-ai` releases, such as Anthropic or community OpenRouter, do not require an
SDK registry change as long as they satisfy the normal `ai.Provider` interface.

## License

This project is licensed under Apache-2.0. See [LICENSE](LICENSE).

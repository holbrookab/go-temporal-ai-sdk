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
    Tools: map[string]ai.Tool{
        "lookup": lookupTool,
    },
})

temporalai.RegisterActivities(worker, acts)
```

## Workflow Usage

```go
result, err := temporalai.InvokeModel(ctx, "model-id", ai.LanguageModelCallOptions{
    Prompt: []ai.Message{ai.UserMessage("hello")},
})
```

For visible streaming, pass Temporal stream metadata under
`ProviderOptions["temporal"]`; the activity strips that key before calling the
real model provider.

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

## Durable Agents

`temporalai.RunAgent` is a workflow-side agent loop. Each model step is a model
activity, and each tool call is a tool activity. Tool lifecycle chunks are
published through the same stream connector used by model streaming.

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

Nested agents can be modeled as child workflows with
`temporalai.ExecuteAgentChildWorkflow`, keeping the parent agent history focused
on child workflow boundaries rather than every nested step.

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

## License

This project is licensed under Apache-2.0. See [LICENSE](LICENSE).

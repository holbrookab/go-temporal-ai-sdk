# go-temporal-ai-sdk

Temporal-native runtime pieces for `github.com/holbrookab/go-ai`.

This package is not a direct TypeScript parity port. The goal is to make Temporal
the durable execution backend for Go AI agents, model calls, tool calls, and
visible streams.

## Current Surface

- `activities`: worker-side activities that call real `go-ai` providers.
- `temporalai`: workflow-safe helpers that schedule those activities.
- `streaming`: connector contracts for live/replay stream infrastructure.

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

## Worker Registration

```go
acts := activities.New(activities.Options{
    ModelProvider:   provider,
    StreamConnector: connector,
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

## Development

This module uses the local sibling `go-ai` checkout:

```go
replace github.com/holbrookab/go-ai => ../go-ai
```

Run tests with a workspace-local Go cache if the sandbox cannot write to the
default OS cache:

```bash
GOCACHE=$PWD/.cache/go-build go test ./...
```

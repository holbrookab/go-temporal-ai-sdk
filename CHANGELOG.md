# Changelog

## Unreleased

## 0.2.19 - 2026-06-01

- Added typed `streaming.ErrStreamNotFound` handling and
  `StreamFailurePolicyBestEffort` so visible streaming can degrade to no-op for
  missing stream rows without swallowing other connector errors.

## 0.2.18 - 2026-06-01

- Added connector-backed visible attempt updates for non-streaming
  `InvokeModel` and `GenerateObject` calls when temporal stream options are
  provided.
- Added `StreamObject` activity registration and the `temporalai.StreamObject`
  workflow helper for object-native streaming calls.

## 0.2.17 - 2026-06-01

- Added `GenerateObject` activity registration and the `temporalai.GenerateObject`
  workflow helper for durable structured-output calls.

## 0.2.16 - 2026-06-01

- Bumped `go-ai` to `v0.2.7` so SDK consumers inherit the OpenRouter
  structured-output schema fix.

## 0.2.15 - 2026-05-31

- Bumped `go-ai` to `v0.2.6`.
- Preserved streamed `reasoning-file` parts when compacting model stream
  results for workflow history.
- Preserved text/file provider metadata across the Temporal wire format for
  Vertex thought-signature replay.

## Historical Notes

- Added tool execution boundaries for durable agents. Tools still default to
  regular Temporal activities, but agents can opt into local tool activities by
  default or override the boundary per tool.
- Added `temporalai.ActivityOptions.LocalTool` and
  `temporalai.InvokeToolLocal` for configuring and invoking local tool
  activities.
- Added configurable local-tool timeout fallback. Local tool timeouts default to
  retrying the same tool call as a regular activity; agents can set
  `LocalToolTimeoutFallbackNone` to surface the local timeout instead.
- Added local language/embedding model invocation options for short routing or
  classification calls.
- Added default Temporal activity summaries for model, stream, embedding, tool,
  and lifecycle invoke helpers.
- Documented the regular-vs-local activity tradeoff for short idempotent tools
  versus slower or more durable tool work.

## 0.2.0 - 2026-05-01

- Set the `go-ai` dependency to `v0.2.0` for release publishing.
- Removed the local sibling `replace` directive from `go.mod`.
- Added Apache-2.0 licensing and README license guidance.

## 0.1.0

- Initial Temporal-native runtime for `go-ai` model calls, tool calls, agents, and visible streams.

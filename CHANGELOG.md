# Changelog

## Unreleased

- Added tool execution boundaries for durable agents. Tools still default to
  regular Temporal activities, but agents can opt into local tool activities by
  default or override the boundary per tool.
- Added `temporalai.ActivityOptions.LocalTool` and
  `temporalai.InvokeToolLocal` for configuring and invoking local tool
  activities.
- Added configurable local-tool timeout fallback. Local tool timeouts default to
  retrying the same tool call as a regular activity; agents can set
  `LocalToolTimeoutFallbackNone` to surface the local timeout instead.
- Documented the regular-vs-local activity tradeoff for short idempotent tools
  versus slower or more durable tool work.

## 0.2.0 - 2026-05-01

- Set the `go-ai` dependency to `v0.2.0` for release publishing.
- Removed the local sibling `replace` directive from `go.mod`.
- Added Apache-2.0 licensing and README license guidance.

## 0.1.0

- Initial Temporal-native runtime for `go-ai` model calls, tool calls, agents, and visible streams.

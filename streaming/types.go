package streaming

import "github.com/holbrookab/go-ai/packages/ai"

type Phase string

const (
	PhaseProviderLive Phase = "provider-live"
	PhaseCanonical    Phase = "canonical"
)

type Lane string

const (
	LaneText      Lane = "text"
	LaneReasoning Lane = "reasoning"
	LaneObject    Lane = "object"
	LaneToolInput Lane = "tool-input"
)

type Event string

const (
	EventStreamStart    Event = "stream-start"
	EventResponseMeta   Event = "response-metadata"
	EventTextDelta      Event = "text-delta"
	EventReasoningDelta Event = "reasoning-delta"
	EventToolInputDelta Event = "tool-input-delta"
	EventToolInputEnd   Event = "tool-input-end"
	EventToolCall       Event = "tool-call"
	EventElement        Event = "element"
	EventFile           Event = "file"
	EventSource         Event = "source"
	EventFinish         Event = "finish"
	EventAbort          Event = "abort"
	EventSnapshot       Event = "snapshot"
	EventAttemptCommit  Event = "attempt-commit"
	EventAttemptDiscard Event = "attempt-discard"
	EventAttemptCancel  Event = "attempt-cancel"
	EventAttemptFail    Event = "attempt-fail"
)

type AttemptStatus string

const (
	AttemptActive    AttemptStatus = "active"
	AttemptCommitted AttemptStatus = "committed"
	AttemptDiscarded AttemptStatus = "discarded"
	AttemptCanceled  AttemptStatus = "canceled"
	AttemptFailed    AttemptStatus = "failed"
)

type ToolLifecycleEvent string

const (
	ToolInputAvailable  ToolLifecycleEvent = "tool-input-available"
	ToolOutputAvailable ToolLifecycleEvent = "tool-output-available"
	ToolOutputError     ToolLifecycleEvent = "tool-output-error"
)

type Options struct {
	Visible                 bool   `json:"visible,omitempty"`
	StreamID                string `json:"streamId,omitempty"`
	Lane                    Lane   `json:"lane,omitempty"`
	AttemptID               string `json:"attemptId,omitempty"`
	SnapshotEveryChunks     int    `json:"snapshotEveryChunks,omitempty"`
	SnapshotEveryCharacters int    `json:"snapshotEveryChars,omitempty"`
	PersistEphemeralChunks  bool   `json:"persistEphemeralChunks,omitempty"`
}

type AttemptRef struct {
	StreamID   string `json:"streamId"`
	Phase      Phase  `json:"phase"`
	Lane       Lane   `json:"lane"`
	AttemptID  string `json:"attemptId"`
	PartID     string `json:"partId,omitempty"`
	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
}

type AttemptSnapshot struct {
	AttemptRef
	Sequence       int    `json:"sequence"`
	SnapshotText   string `json:"snapshotText,omitempty"`
	SnapshotObject any    `json:"snapshotObject,omitempty"`
}

type LiveChunk struct {
	AttemptRef
	Event        Event         `json:"event"`
	Sequence     int           `json:"sequence"`
	Delta        string        `json:"delta,omitempty"`
	Input        any           `json:"input,omitempty"`
	Element      any           `json:"element,omitempty"`
	ProviderPart ai.StreamPart `json:"providerPart"`
}

type EphemeralChunk = LiveChunk

type AttemptCompletion struct {
	AttemptRef
	Sequence       int           `json:"sequence"`
	Status         AttemptStatus `json:"status"`
	Reason         string        `json:"reason,omitempty"`
	SnapshotText   string        `json:"snapshotText,omitempty"`
	SnapshotObject any           `json:"snapshotObject,omitempty"`
}

type ToolLifecycleInput struct {
	EventID          string             `json:"eventId,omitempty"`
	StreamID         string             `json:"streamId"`
	Event            ToolLifecycleEvent `json:"event"`
	ToolCallID       string             `json:"toolCallId"`
	ToolName         string             `json:"toolName"`
	Input            any                `json:"input,omitempty"`
	Output           any                `json:"output,omitempty"`
	ErrorText        string             `json:"errorText,omitempty"`
	Dynamic          bool               `json:"dynamic,omitempty"`
	ProviderExecuted bool               `json:"providerExecuted,omitempty"`
	Preliminary      bool               `json:"preliminary,omitempty"`
	Metadata         map[string]any     `json:"metadata,omitempty"`
}

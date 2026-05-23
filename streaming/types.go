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
	EventStartStep      Event = "start-step"
	EventResponseMeta   Event = "response-metadata"
	EventTextDelta      Event = "text-delta"
	EventReasoningDelta Event = "reasoning-delta"
	EventToolInputDelta Event = "tool-input-delta"
	EventToolInputEnd   Event = "tool-input-end"
	EventToolCall       Event = "tool-call"
	EventElement        Event = "element"
	EventFile           Event = "file"
	EventSource         Event = "source"
	EventFinishStep     Event = "finish-step"
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
	ToolInputAvailable   ToolLifecycleEvent = "tool-input-available"
	ToolApprovalRequest  ToolLifecycleEvent = "tool-approval-request"
	ToolApprovalResponse ToolLifecycleEvent = "tool-approval-response"
	ToolOutputAvailable  ToolLifecycleEvent = "tool-output-available"
	ToolOutputError      ToolLifecycleEvent = "tool-output-error"
	ToolOutputDenied     ToolLifecycleEvent = "tool-output-denied"
)

type DisplayMode string

const (
	DisplayModeAssistant DisplayMode = "assistant"
	DisplayModeTask      DisplayMode = "task"
	DisplayModeHidden    DisplayMode = "hidden"
)

type Scope struct {
	DisplayMode DisplayMode `json:"displayMode,omitempty"`
	AgentID     string      `json:"agentId,omitempty"`
	TaskID      string      `json:"taskId,omitempty"`
	TaskTitle   string      `json:"taskTitle,omitempty"`
	SkillName   string      `json:"skillName,omitempty"`
	StepID      string      `json:"stepId,omitempty"`
	StepNumber  *int        `json:"stepNumber,omitempty"`
	StepType    string      `json:"stepType,omitempty"`
}

type Options struct {
	Visible                 bool   `json:"visible,omitempty"`
	StreamID                string `json:"streamId,omitempty"`
	Lane                    Lane   `json:"lane,omitempty"`
	AttemptID               string `json:"attemptId,omitempty"`
	SnapshotEveryChunks     int    `json:"snapshotEveryChunks,omitempty"`
	SnapshotEveryCharacters int    `json:"snapshotEveryChars,omitempty"`
	PersistEphemeralChunks  bool   `json:"persistEphemeralChunks,omitempty"`
	Scope
}

type AttemptRef struct {
	StreamID   string `json:"streamId"`
	Phase      Phase  `json:"phase"`
	Lane       Lane   `json:"lane"`
	AttemptID  string `json:"attemptId"`
	PartID     string `json:"partId,omitempty"`
	ToolCallID string `json:"toolCallId,omitempty"`
	ToolName   string `json:"toolName,omitempty"`
	Scope
}

type AttemptSnapshot struct {
	AttemptRef
	Sequence       int    `json:"sequence"`
	SnapshotText   string `json:"snapshotText,omitempty"`
	SnapshotObject any    `json:"snapshotObject,omitempty"`
}

type LiveChunk struct {
	AttemptRef
	Event          Event         `json:"event"`
	Sequence       int           `json:"sequence"`
	Delta          string        `json:"delta,omitempty"`
	Input          any           `json:"input,omitempty"`
	Element        any           `json:"element,omitempty"`
	SnapshotObject any           `json:"snapshotObject,omitempty"`
	ProviderPart   ai.StreamPart `json:"providerPart"`
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
	EventID          string              `json:"eventId,omitempty"`
	StreamID         string              `json:"streamId"`
	Event            ToolLifecycleEvent  `json:"event"`
	ToolCallID       string              `json:"toolCallId"`
	ToolName         string              `json:"toolName"`
	ApprovalID       string              `json:"approvalId,omitempty"`
	Approved         *bool               `json:"approved,omitempty"`
	Reason           string              `json:"reason,omitempty"`
	IsAutomatic      *bool               `json:"isAutomatic,omitempty"`
	Input            any                 `json:"input,omitempty"`
	Output           any                 `json:"output,omitempty"`
	ErrorText        string              `json:"errorText,omitempty"`
	Dynamic          bool                `json:"dynamic,omitempty"`
	ProviderExecuted bool                `json:"providerExecuted,omitempty"`
	Preliminary      bool                `json:"preliminary,omitempty"`
	ToolMetadata     ai.ProviderMetadata `json:"toolMetadata,omitempty"`
	Metadata         map[string]any      `json:"metadata,omitempty"`
	Scope
}

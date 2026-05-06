package appsyncdynamodb

import (
	"encoding/json"
	"fmt"

	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

const (
	maxStreamChunkValueBytes  = 24_000
	maxStreamChunkStringBytes = 24_000
)

func llmStreamChunk(event streaming.Event, input any) map[string]any {
	data := map[string]any{"event": event}
	id := ""
	switch value := input.(type) {
	case streaming.LiveChunk:
		id = chunkID(value.Lane, value.ToolCallID, value.AttemptID)
		data["streamId"] = value.StreamID
		data["phase"] = value.Phase
		data["lane"] = value.Lane
		data["attemptId"] = value.AttemptID
		data["partId"] = value.PartID
		data["toolCallId"] = value.ToolCallID
		data["toolName"] = value.ToolName
		data["sequence"] = value.Sequence
		data["delta"] = value.Delta
		data["input"] = compactStreamValue(value.Input)
		data["element"] = compactStreamValue(value.Element)
		data["snapshotObject"] = compactStreamValue(value.SnapshotObject)
		addScopeFields(data, value.Scope)
	case streaming.AttemptCompletion:
		id = chunkID(value.Lane, value.ToolCallID, value.AttemptID)
		data["streamId"] = value.StreamID
		data["phase"] = value.Phase
		data["lane"] = value.Lane
		data["attemptId"] = value.AttemptID
		data["partId"] = value.PartID
		data["toolCallId"] = value.ToolCallID
		data["toolName"] = value.ToolName
		data["sequence"] = value.Sequence
		data["status"] = value.Status
		data["reason"] = value.Reason
		data["snapshotText"] = value.SnapshotText
		data["snapshotObject"] = compactStreamValue(value.SnapshotObject)
		addScopeFields(data, value.Scope)
	}
	return map[string]any{
		"type":      "data-llm-stream",
		"id":        id,
		"transient": true,
		"data":      cleanChunkMap(data),
	}
}

func toolLifecycleChunk(input streaming.ToolLifecycleInput) map[string]any {
	chunk := map[string]any{
		"eventId":          input.EventID,
		"streamId":         input.StreamID,
		"type":             string(input.Event),
		"toolCallId":       input.ToolCallID,
		"toolName":         input.ToolName,
		"approvalId":       input.ApprovalID,
		"dynamic":          input.Dynamic,
		"providerExecuted": input.ProviderExecuted,
		"metadata":         input.Metadata,
	}
	addScopeFields(chunk, input.Scope)
	switch input.Event {
	case streaming.ToolInputAvailable:
		chunk["input"] = compactStreamValue(input.Input)
	case streaming.ToolApprovalRequest:
		chunk["isAutomatic"] = input.IsAutomatic
	case streaming.ToolApprovalResponse:
		chunk["approved"] = input.Approved
		chunk["reason"] = input.Reason
	case streaming.ToolOutputAvailable:
		chunk["output"] = compactStreamValue(input.Output)
		chunk["preliminary"] = input.Preliminary
	case streaming.ToolOutputError:
		chunk["errorText"] = compactStreamStringValue(input.ErrorText)
	case streaming.ToolOutputDenied:
	}
	return cleanChunkMap(chunk)
}

func compactStreamValue(value any) any {
	if value == nil {
		return nil
	}
	if text, ok := value.(string); ok {
		return truncateStringBytes(text, maxStreamChunkStringBytes)
	}
	originalBytes := jsonByteLength(value)
	if originalBytes <= maxStreamChunkValueBytes {
		return value
	}
	return map[string]any{
		"truncated":     true,
		"originalBytes": originalBytes,
		"preview":       truncateStringBytes(jsonString(value), maxStreamChunkValueBytes),
	}
}

func compactStreamStringValue(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	return truncateStringBytes(text, maxStreamChunkStringBytes)
}

func truncateStringBytes(value string, maxBytes int) string {
	if len([]byte(value)) <= maxBytes {
		return value
	}
	suffix := "\n\n[truncated for streaming]"
	limit := maxBytes - len([]byte(suffix))
	if limit < 0 {
		limit = 0
	}
	bytes := []byte(value)
	if limit > len(bytes) {
		limit = len(bytes)
	}
	return string(bytes[:limit]) + suffix
}

func jsonByteLength(value any) int {
	return len([]byte(jsonString(value)))
}

func jsonString(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(bytes)
}

func addScopeFields(data map[string]any, scope streaming.Scope) {
	data["displayMode"] = string(scope.DisplayMode)
	data["agentId"] = scope.AgentID
	data["taskId"] = scope.TaskID
	data["taskTitle"] = scope.TaskTitle
	data["skillName"] = scope.SkillName
	data["stepId"] = scope.StepID
	data["stepNumber"] = scope.StepNumber
	data["stepType"] = scope.StepType
}

func toolLifecycleEventID(input streaming.ToolLifecycleInput) string {
	if input.EventID != "" {
		return input.EventID
	}
	return newEventID()
}

func chunkID(lane streaming.Lane, toolCallID string, attemptID string) string {
	id := string(lane)
	if toolCallID != "" {
		id += ":" + toolCallID
	}
	if attemptID != "" {
		id += ":" + attemptID
	}
	return id
}

func cleanChunkMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		if value == nil || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

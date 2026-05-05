package appsyncdynamodb

import "github.com/holbrookab/go-temporal-ai-sdk/streaming"

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
		data["input"] = value.Input
		data["element"] = value.Element
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
		data["snapshotObject"] = value.SnapshotObject
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
		chunk["input"] = input.Input
	case streaming.ToolApprovalRequest:
		chunk["isAutomatic"] = input.IsAutomatic
	case streaming.ToolApprovalResponse:
		chunk["approved"] = input.Approved
		chunk["reason"] = input.Reason
	case streaming.ToolOutputAvailable:
		chunk["output"] = input.Output
		chunk["preliminary"] = input.Preliminary
	case streaming.ToolOutputError:
		chunk["errorText"] = input.ErrorText
	case streaming.ToolOutputDenied:
	}
	return cleanChunkMap(chunk)
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

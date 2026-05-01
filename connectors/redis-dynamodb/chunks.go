package redisdynamodb

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
		"type":             string(input.Event),
		"toolCallId":       input.ToolCallID,
		"toolName":         input.ToolName,
		"dynamic":          input.Dynamic,
		"providerExecuted": input.ProviderExecuted,
	}
	switch input.Event {
	case streaming.ToolInputAvailable:
		chunk["input"] = input.Input
	case streaming.ToolOutputAvailable:
		chunk["output"] = input.Output
		chunk["preliminary"] = input.Preliminary
	case streaming.ToolOutputError:
		chunk["errorText"] = input.ErrorText
	}
	return cleanChunkMap(chunk)
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

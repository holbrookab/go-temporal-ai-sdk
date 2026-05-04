package appsyncdynamodb

import (
	"testing"

	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

func TestLLMStreamChunkMatchesUIDataShape(t *testing.T) {
	chunk := llmStreamChunk(streaming.EventTextDelta, streaming.LiveChunk{
		AttemptRef: streaming.AttemptRef{
			StreamID:  "stream-1",
			Phase:     streaming.PhaseProviderLive,
			Lane:      streaming.LaneText,
			AttemptID: "attempt-1",
		},
		Sequence: 3,
		Delta:    "hel",
	})
	if chunk["type"] != "data-llm-stream" {
		t.Fatalf("type = %#v", chunk["type"])
	}
	data, ok := chunk["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v", chunk["data"])
	}
	if data["event"] != streaming.EventTextDelta || data["delta"] != "hel" || data["sequence"] != 3 {
		t.Fatalf("data = %#v", data)
	}
}

func TestToolLifecycleChunkMapsError(t *testing.T) {
	chunk := toolLifecycleChunk(streaming.ToolLifecycleInput{
		EventID:    "tool:call-1:terminal",
		Event:      streaming.ToolOutputError,
		ToolCallID: "call-1",
		ToolName:   "lookup",
		ErrorText:  "boom",
		Metadata: map[string]any{
			"taskId":    "task-1",
			"taskTitle": "Find records",
		},
	})
	if chunk["type"] != string(streaming.ToolOutputError) {
		t.Fatalf("type = %#v", chunk["type"])
	}
	if chunk["errorText"] != "boom" {
		t.Fatalf("chunk = %#v", chunk)
	}
	if chunk["eventId"] != "tool:call-1:terminal" {
		t.Fatalf("chunk = %#v", chunk)
	}
	metadata, ok := chunk["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %#v", chunk["metadata"])
	}
	if metadata["taskId"] != "task-1" || metadata["taskTitle"] != "Find records" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestToolLifecycleChunkMapsApproval(t *testing.T) {
	approved := true
	automatic := true
	request := toolLifecycleChunk(streaming.ToolLifecycleInput{
		EventID:     "tool:call-1:approval-request",
		Event:       streaming.ToolApprovalRequest,
		ToolCallID:  "call-1",
		ToolName:    "create_worker",
		ApprovalID:  "approval-1",
		IsAutomatic: &automatic,
		Metadata:    map[string]any{"taskId": "task-1"},
	})
	if request["type"] != string(streaming.ToolApprovalRequest) || request["approvalId"] != "approval-1" {
		t.Fatalf("request = %#v", request)
	}
	if request["isAutomatic"] != &automatic {
		t.Fatalf("request = %#v", request)
	}

	response := toolLifecycleChunk(streaming.ToolLifecycleInput{
		EventID:    "tool:call-1:approval-response",
		Event:      streaming.ToolApprovalResponse,
		ToolCallID: "call-1",
		ToolName:   "create_worker",
		ApprovalID: "approval-1",
		Approved:   &approved,
		Reason:     "looks good",
	})
	if response["type"] != string(streaming.ToolApprovalResponse) || response["approvalId"] != "approval-1" {
		t.Fatalf("response = %#v", response)
	}
	if response["approved"] != &approved || response["reason"] != "looks good" {
		t.Fatalf("response = %#v", response)
	}
}

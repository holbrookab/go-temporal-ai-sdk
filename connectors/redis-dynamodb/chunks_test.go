package redisdynamodb

import (
	"strings"
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

func TestToolLifecycleChunkMapsOutput(t *testing.T) {
	chunk := toolLifecycleChunk(streaming.ToolLifecycleInput{
		EventID:    "tool:call-1:terminal",
		Event:      streaming.ToolOutputAvailable,
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Output:     "ok",
		Metadata: map[string]any{
			"taskId":    "task-1",
			"taskTitle": "Find records",
		},
	})
	if chunk["type"] != string(streaming.ToolOutputAvailable) {
		t.Fatalf("type = %#v", chunk["type"])
	}
	if chunk["output"] != "ok" {
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

func TestToolLifecycleChunkCompactsLargeOutput(t *testing.T) {
	chunk := toolLifecycleChunk(streaming.ToolLifecycleInput{
		Event:      streaming.ToolOutputAvailable,
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Output: map[string]any{
			"value": strings.Repeat("x", maxStreamChunkValueBytes+1000),
		},
	})
	output, ok := chunk["output"].(map[string]any)
	if !ok {
		t.Fatalf("output = %#v", chunk["output"])
	}
	if output["truncated"] != true || output["preview"] == "" {
		t.Fatalf("output = %#v", output)
	}
}

func TestToolLifecycleChunkMapsApproval(t *testing.T) {
	approved := false
	response := toolLifecycleChunk(streaming.ToolLifecycleInput{
		EventID:    "tool:call-1:approval-response",
		Event:      streaming.ToolApprovalResponse,
		ToolCallID: "call-1",
		ToolName:   "create_worker",
		ApprovalID: "approval-1",
		Approved:   &approved,
		Reason:     "needs correction",
		Metadata:   map[string]any{"taskId": "task-1"},
	})
	if response["type"] != string(streaming.ToolApprovalResponse) || response["approvalId"] != "approval-1" {
		t.Fatalf("response = %#v", response)
	}
	if response["approved"] != &approved || response["reason"] != "needs correction" {
		t.Fatalf("response = %#v", response)
	}
	metadata, ok := response["metadata"].(map[string]any)
	if !ok || metadata["taskId"] != "task-1" {
		t.Fatalf("metadata = %#v", response["metadata"])
	}
}

func TestDefaultResolveBuildsRedisKeys(t *testing.T) {
	c := New(Options{})
	ref, err := c.resolve(nil, "stream-1")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Channel != defaultChannelPrefix+"stream-1" {
		t.Fatalf("channel = %q", ref.Channel)
	}
	if ref.RedisStream != defaultStreamPrefix+"stream-1" {
		t.Fatalf("stream = %q", ref.RedisStream)
	}
}

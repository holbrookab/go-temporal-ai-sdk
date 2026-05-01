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
}

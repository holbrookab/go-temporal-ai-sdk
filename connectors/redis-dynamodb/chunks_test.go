package redisdynamodb

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

func TestToolLifecycleChunkMapsOutput(t *testing.T) {
	chunk := toolLifecycleChunk(streaming.ToolLifecycleInput{
		EventID:    "tool:call-1:terminal",
		Event:      streaming.ToolOutputAvailable,
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Output:     "ok",
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

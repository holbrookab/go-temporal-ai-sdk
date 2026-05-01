package streaming

import (
	"context"
	"testing"

	"github.com/holbrookab/go-ai/packages/ai"
)

func TestStreamFromPartsReplaysInOrder(t *testing.T) {
	stream := StreamFromParts([]ai.StreamPart{
		{Type: "text-delta", TextDelta: "a"},
		{Type: "text-delta", TextDelta: "b"},
	})
	var got string
	for part := range stream {
		got += part.TextDelta
	}
	if got != "ab" {
		t.Fatalf("got %q", got)
	}
}

func TestRelayCreatesToolInputLanePerToolCall(t *testing.T) {
	connector := &recordingConnector{}
	relay := NewRelay(connector, Options{
		Visible:  true,
		StreamID: "stream-1",
		Lane:     LaneText,
	})

	if err := relay.Accept(context.Background(), ai.StreamPart{
		Type:       "tool-call",
		ToolCallID: "call-1",
		ToolName:   "search",
		ToolInput:  `{"q":"hello"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := relay.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(connector.starts) != 1 {
		t.Fatalf("starts = %d", len(connector.starts))
	}
	if connector.starts[0].Lane != LaneToolInput || connector.starts[0].ToolCallID != "call-1" {
		t.Fatalf("start = %#v", connector.starts[0])
	}
	if len(connector.completions) != 1 || connector.completions[0].Status != AttemptCommitted {
		t.Fatalf("completions = %#v", connector.completions)
	}
}

func TestRelayCompletionCarriesFinalSnapshot(t *testing.T) {
	connector := &recordingConnector{}
	relay := NewRelay(connector, Options{
		Visible:  true,
		StreamID: "stream-1",
		Lane:     LaneText,
	})

	if err := relay.Accept(context.Background(), ai.StreamPart{Type: "text-delta", TextDelta: "hel"}); err != nil {
		t.Fatal(err)
	}
	if err := relay.Accept(context.Background(), ai.StreamPart{Type: "text-delta", TextDelta: "lo"}); err != nil {
		t.Fatal(err)
	}
	if err := relay.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(connector.completions) != 1 {
		t.Fatalf("completions = %#v", connector.completions)
	}
	if connector.completions[0].SnapshotText != "hello" {
		t.Fatalf("completion snapshot = %#v", connector.completions[0])
	}
}

type recordingConnector struct {
	starts      []AttemptRef
	live        []LiveChunk
	snapshots   []AttemptSnapshot
	completions []AttemptCompletion
	tools       []ToolLifecycleInput
}

func (c *recordingConnector) StartAttempt(_ context.Context, input AttemptRef) error {
	c.starts = append(c.starts, input)
	return nil
}

func (c *recordingConnector) PublishLiveChunk(_ context.Context, input LiveChunk) error {
	c.live = append(c.live, input)
	return nil
}

func (c *recordingConnector) UpdateAttemptSnapshot(_ context.Context, input AttemptSnapshot) error {
	c.snapshots = append(c.snapshots, input)
	return nil
}

func (c *recordingConnector) CompleteAttempt(_ context.Context, input AttemptCompletion) error {
	c.completions = append(c.completions, input)
	return nil
}

func (c *recordingConnector) PublishToolLifecycleEvent(_ context.Context, input ToolLifecycleInput) error {
	c.tools = append(c.tools, input)
	return nil
}

func (c *recordingConnector) PersistToolLifecycleEvent(_ context.Context, input ToolLifecycleInput) error {
	c.tools = append(c.tools, input)
	return nil
}

func (c *recordingConnector) PublishLiveToolLifecycleEvent(_ context.Context, input ToolLifecycleInput) error {
	c.tools = append(c.tools, input)
	return nil
}

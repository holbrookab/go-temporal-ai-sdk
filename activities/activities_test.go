package activities

import (
	"context"
	"errors"
	"testing"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

func TestInvokeModelDelegatesToProvider(t *testing.T) {
	model := ai.NewMockLanguageModel("model-1")
	model.GenerateFunc = func(_ context.Context, opts ai.LanguageModelCallOptions) (*ai.LanguageModelGenerateResult, error) {
		if got := ai.TextFromParts(opts.Prompt[0].Content); got != "hello" {
			t.Fatalf("prompt = %q", got)
		}
		return &ai.LanguageModelGenerateResult{
			Content:      []ai.Part{ai.TextPart{Text: "hi"}},
			FinishReason: ai.FinishReason{Unified: ai.FinishStop},
		}, nil
	}
	acts := New(Options{ModelProvider: ai.CustomProvider{
		LanguageModels: map[string]ai.LanguageModel{"model-1": model},
	}})

	result, err := acts.InvokeModel(context.Background(), InvokeModelArgs{
		ModelID: "model-1",
		Options: LanguageModelCallOptionsFromAI(ai.LanguageModelCallOptions{
			Prompt: []ai.Message{ai.UserMessage("hello")},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ai.TextFromParts(result.ToAI().Content); got != "hi" {
		t.Fatalf("text = %q", got)
	}
}

func TestInvokeEmbeddingModelDelegatesToProvider(t *testing.T) {
	model := ai.NewMockEmbeddingModel("embed-1")
	acts := New(Options{ModelProvider: ai.CustomProvider{
		EmbeddingModels: map[string]ai.EmbeddingModel{"embed-1": model},
	}})

	result, err := acts.InvokeEmbeddingModel(context.Background(), InvokeEmbeddingModelArgs{
		ModelID: "embed-1",
		Values:  []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Embeddings) != 2 {
		t.Fatalf("embeddings = %d", len(result.Embeddings))
	}
}

func TestInvokeToolUsesRegisteredTool(t *testing.T) {
	acts := New(Options{
		Tools: map[string]ai.Tool{
			"lookup": {
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []any{"query"},
				},
				Execute: func(_ context.Context, call ai.ToolCall, _ ai.ToolExecutionOptions) (any, error) {
					return "found " + call.Input.(map[string]any)["query"].(string), nil
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Input:      map[string]any{"query": "temporal"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %#v", result)
	}
	if result.Output.Value != "found temporal" {
		t.Fatalf("output = %#v", result.Output)
	}
}

func TestInvokeModelStreamPublishesConnectorAttempt(t *testing.T) {
	model := ai.NewMockLanguageModel("stream-1")
	model.StreamFunc = func(_ context.Context, opts ai.LanguageModelCallOptions) (*ai.LanguageModelStreamResult, error) {
		if _, ok := opts.ProviderOptions[ProviderOptionsKey]; ok {
			t.Fatalf("temporal provider option leaked to model: %#v", opts.ProviderOptions)
		}
		ch := make(chan ai.StreamPart, 4)
		ch <- ai.StreamPart{Type: "raw", Raw: map[string]any{"event": "provider-frame"}}
		ch <- ai.StreamPart{Type: "text-delta", TextDelta: "hel"}
		ch <- ai.StreamPart{Type: "text-delta", TextDelta: "lo"}
		ch <- ai.StreamPart{Type: "finish", FinishReason: ai.FinishReason{Unified: ai.FinishStop}}
		close(ch)
		return &ai.LanguageModelStreamResult{Stream: ch}, nil
	}
	connector := &recordingConnector{}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"stream-1": model},
		},
		StreamConnector: connector,
	})

	result, err := acts.InvokeModelStream(context.Background(), InvokeModelStreamArgs{
		ModelID: "stream-1",
		Options: LanguageModelCallOptionsFromAI(ai.LanguageModelCallOptions{
			ProviderOptions: ai.ProviderOptions{
				ProviderOptionsKey: streaming.Options{
					Visible:   true,
					StreamID:  "stream-123",
					AttemptID: "turn-1",
					Lane:      streaming.LaneText,
				},
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.StreamParts) != 0 {
		t.Fatalf("stream parts should not be returned to workflow history: %#v", result.StreamParts)
	}
	if result.Result == nil {
		t.Fatal("expected compact stream result")
	}
	if got := ai.TextFromParts(result.Result.ToAI().Content); got != "hello" {
		t.Fatalf("result text = %q", got)
	}
	if result.Result.FinishReason.Unified != ai.FinishStop {
		t.Fatalf("finish reason = %#v", result.Result.FinishReason)
	}
	if len(connector.starts) != 1 {
		t.Fatalf("starts = %d", len(connector.starts))
	}
	if len(connector.live) != 3 {
		t.Fatalf("live chunks = %d", len(connector.live))
	}
	if len(connector.completions) != 1 || connector.completions[0].Status != streaming.AttemptCommitted {
		t.Fatalf("completions = %#v", connector.completions)
	}
	if connector.snapshots[len(connector.snapshots)-1].SnapshotText != "hello" {
		t.Fatalf("snapshot = %#v", connector.snapshots[len(connector.snapshots)-1])
	}
}

func TestInvokeModelStreamDiscardsErroredStream(t *testing.T) {
	model := ai.NewMockLanguageModel("stream-1")
	model.StreamFunc = func(context.Context, ai.LanguageModelCallOptions) (*ai.LanguageModelStreamResult, error) {
		ch := make(chan ai.StreamPart, 2)
		ch <- ai.StreamPart{Type: "text-delta", TextDelta: "partial"}
		ch <- ai.StreamPart{Type: "error", Err: errors.New("boom")}
		close(ch)
		return &ai.LanguageModelStreamResult{Stream: ch}, nil
	}
	connector := &recordingConnector{}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"stream-1": model},
		},
		StreamConnector: connector,
	})

	_, err := acts.InvokeModelStream(context.Background(), InvokeModelStreamArgs{
		ModelID: "stream-1",
		Options: LanguageModelCallOptionsFromAI(ai.LanguageModelCallOptions{
			ProviderOptions: ai.ProviderOptions{
				ProviderOptionsKey: streaming.Options{Visible: true, StreamID: "stream-123"},
			},
		}),
	})
	if err == nil {
		t.Fatal("expected stream error")
	}
	if len(connector.completions) != 1 || connector.completions[0].Status != streaming.AttemptDiscarded {
		t.Fatalf("completions = %#v", connector.completions)
	}
}

type recordingConnector struct {
	starts      []streaming.AttemptRef
	live        []streaming.LiveChunk
	snapshots   []streaming.AttemptSnapshot
	completions []streaming.AttemptCompletion
	tools       []streaming.ToolLifecycleInput
}

func (c *recordingConnector) StartAttempt(_ context.Context, input streaming.AttemptRef) error {
	c.starts = append(c.starts, input)
	return nil
}

func (c *recordingConnector) PublishLiveChunk(_ context.Context, input streaming.LiveChunk) error {
	c.live = append(c.live, input)
	return nil
}

func (c *recordingConnector) UpdateAttemptSnapshot(_ context.Context, input streaming.AttemptSnapshot) error {
	c.snapshots = append(c.snapshots, input)
	return nil
}

func (c *recordingConnector) CompleteAttempt(_ context.Context, input streaming.AttemptCompletion) error {
	c.completions = append(c.completions, input)
	return nil
}

func (c *recordingConnector) PublishToolLifecycleEvent(_ context.Context, input streaming.ToolLifecycleInput) error {
	c.tools = append(c.tools, input)
	return nil
}

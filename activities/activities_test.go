package activities

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestInvokeModelPublishesConnectorAttempt(t *testing.T) {
	model := ai.NewMockLanguageModel("model-1")
	model.GenerateFunc = func(_ context.Context, opts ai.LanguageModelCallOptions) (*ai.LanguageModelGenerateResult, error) {
		if _, ok := opts.ProviderOptions[ProviderOptionsKey]; ok {
			t.Fatalf("temporal provider option leaked to model: %#v", opts.ProviderOptions)
		}
		return &ai.LanguageModelGenerateResult{
			Content:      []ai.Part{ai.TextPart{Text: "hi"}},
			FinishReason: ai.FinishReason{Unified: ai.FinishStop},
		}, nil
	}
	connector := &recordingConnector{}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"model-1": model},
		},
		StreamConnector: connector,
	})

	result, err := acts.InvokeModel(context.Background(), InvokeModelArgs{
		ModelID: "model-1",
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
	if got := ai.TextFromParts(result.ToAI().Content); got != "hi" {
		t.Fatalf("text = %q", got)
	}
	if len(connector.starts) != 1 {
		t.Fatalf("starts = %d", len(connector.starts))
	}
	if len(connector.live) != 3 {
		t.Fatalf("live chunks = %#v", connector.live)
	}
	if connector.live[0].Event != streaming.EventStreamStart || connector.live[1].Delta != "hi" || connector.live[2].Event != streaming.EventFinish {
		t.Fatalf("live chunks = %#v", connector.live)
	}
	if len(connector.completions) != 1 || connector.completions[0].Status != streaming.AttemptCommitted {
		t.Fatalf("completions = %#v", connector.completions)
	}
	if connector.completions[0].SnapshotText != "hi" {
		t.Fatalf("completion = %#v", connector.completions[0])
	}
}

func TestGenerateObjectDelegatesToGenerateObject(t *testing.T) {
	model := ai.NewMockLanguageModel("model-1")
	model.GenerateFunc = func(_ context.Context, opts ai.LanguageModelCallOptions) (*ai.LanguageModelGenerateResult, error) {
		if opts.ResponseFormat == nil || opts.ResponseFormat.Type != "json" {
			t.Fatalf("response format = %#v", opts.ResponseFormat)
		}
		if opts.ProviderOptions["source"] != "test" {
			t.Fatalf("provider options = %#v", opts.ProviderOptions)
		}
		return &ai.LanguageModelGenerateResult{
			Content:      []ai.Part{ai.TextPart{Text: `{"name":"Ada"}`}},
			FinishReason: ai.FinishReason{Unified: ai.FinishStop, Raw: "stop"},
			Response:     ai.ResponseMetadata{ID: "response-1"},
		}, nil
	}
	acts := New(Options{ModelProvider: ai.CustomProvider{
		LanguageModels: map[string]ai.LanguageModel{"model-1": model},
	}})

	result, err := acts.GenerateObject(context.Background(), GenerateObjectArgs{
		ModelID: "model-1",
		Options: GenerateObjectOptionsFromAI(ai.GenerateObjectOptions{
			Prompt:          "return json",
			Schema:          map[string]any{"type": "object"},
			ProviderOptions: ai.ProviderOptions{"source": "test"},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	object, ok := result.Object.(map[string]any)
	if !ok || object["name"] != "Ada" {
		t.Fatalf("object = %#v", result.Object)
	}
	if result.Text != `{"name":"Ada"}` || result.Response.ID != "response-1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGenerateObjectPublishesConnectorObjectSnapshot(t *testing.T) {
	model := ai.NewMockLanguageModel("model-1")
	model.GenerateFunc = func(_ context.Context, opts ai.LanguageModelCallOptions) (*ai.LanguageModelGenerateResult, error) {
		if _, ok := opts.ProviderOptions[ProviderOptionsKey]; ok {
			t.Fatalf("temporal provider option leaked to model: %#v", opts.ProviderOptions)
		}
		return &ai.LanguageModelGenerateResult{
			Content:      []ai.Part{ai.TextPart{Text: `{"name":"Ada"}`}},
			FinishReason: ai.FinishReason{Unified: ai.FinishStop, Raw: "stop"},
		}, nil
	}
	connector := &recordingConnector{}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"model-1": model},
		},
		StreamConnector: connector,
	})

	result, err := acts.GenerateObject(context.Background(), GenerateObjectArgs{
		ModelID: "model-1",
		Options: GenerateObjectOptionsFromAI(ai.GenerateObjectOptions{
			Prompt: "return json",
			Schema: map[string]any{"type": "object"},
			ProviderOptions: ai.ProviderOptions{
				ProviderOptionsKey: streaming.Options{
					Visible:   true,
					StreamID:  "stream-123",
					AttemptID: "turn-1",
				},
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	object, ok := result.Object.(map[string]any)
	if !ok || object["name"] != "Ada" {
		t.Fatalf("object = %#v", result.Object)
	}
	if len(connector.starts) != 1 {
		t.Fatalf("starts = %d", len(connector.starts))
	}
	if connector.starts[0].Lane != streaming.LaneObject {
		t.Fatalf("start lane = %q", connector.starts[0].Lane)
	}
	if len(connector.live) != 3 {
		t.Fatalf("live chunks = %#v", connector.live)
	}
	if connector.live[1].Delta != "" {
		t.Fatalf("object lane delta = %q, want raw JSON suppressed", connector.live[1].Delta)
	}
	liveObject, ok := connector.live[1].SnapshotObject.(map[string]any)
	if !ok || liveObject["name"] != "Ada" {
		t.Fatalf("live object = %#v", connector.live[1].SnapshotObject)
	}
	completion, ok := connector.completions[0].SnapshotObject.(map[string]any)
	if !ok || completion["name"] != "Ada" {
		t.Fatalf("completion object = %#v", connector.completions[0].SnapshotObject)
	}
}

func TestGenerateObjectBestEffortStreamNotFoundStillSucceeds(t *testing.T) {
	modelCalled := false
	model := ai.NewMockLanguageModel("model-1")
	model.GenerateFunc = func(_ context.Context, _ ai.LanguageModelCallOptions) (*ai.LanguageModelGenerateResult, error) {
		modelCalled = true
		return &ai.LanguageModelGenerateResult{
			Content:      []ai.Part{ai.TextPart{Text: `{"name":"Ada"}`}},
			FinishReason: ai.FinishReason{Unified: ai.FinishStop, Raw: "stop"},
		}, nil
	}
	connector := &recordingConnector{startErr: streaming.NewStreamNotFoundError("stream-123")}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"model-1": model},
		},
		StreamConnector: connector,
	})

	result, err := acts.GenerateObject(context.Background(), GenerateObjectArgs{
		ModelID: "model-1",
		Options: GenerateObjectOptionsFromAI(ai.GenerateObjectOptions{
			Prompt: "return json",
			Schema: map[string]any{"type": "object"},
			ProviderOptions: ai.ProviderOptions{
				ProviderOptionsKey: streaming.Options{
					Visible:       true,
					StreamID:      "stream-123",
					AttemptID:     "turn-1",
					FailurePolicy: streaming.StreamFailurePolicyBestEffort,
				},
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !modelCalled {
		t.Fatal("model was not called")
	}
	object, ok := result.Object.(map[string]any)
	if !ok || object["name"] != "Ada" {
		t.Fatalf("object = %#v", result.Object)
	}
	if len(connector.live) != 0 || len(connector.completions) != 0 {
		t.Fatalf("connector was not disabled after missing stream: live=%#v completions=%#v", connector.live, connector.completions)
	}
}

func TestGenerateObjectStreamNotFoundIsStrictByDefault(t *testing.T) {
	tests := []struct {
		name   string
		policy streaming.StreamFailurePolicy
	}{
		{name: "default"},
		{name: "strict", policy: streaming.StreamFailurePolicyStrict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelCalled := false
			model := ai.NewMockLanguageModel("model-1")
			model.GenerateFunc = func(_ context.Context, _ ai.LanguageModelCallOptions) (*ai.LanguageModelGenerateResult, error) {
				modelCalled = true
				return &ai.LanguageModelGenerateResult{
					Content:      []ai.Part{ai.TextPart{Text: `{"name":"Ada"}`}},
					FinishReason: ai.FinishReason{Unified: ai.FinishStop},
				}, nil
			}
			acts := New(Options{
				ModelProvider: ai.CustomProvider{
					LanguageModels: map[string]ai.LanguageModel{"model-1": model},
				},
				StreamConnector: &recordingConnector{
					startErr: streaming.NewStreamNotFoundError("stream-123"),
				},
			})

			_, err := acts.GenerateObject(context.Background(), GenerateObjectArgs{
				ModelID: "model-1",
				Options: GenerateObjectOptionsFromAI(ai.GenerateObjectOptions{
					Prompt: "return json",
					Schema: map[string]any{"type": "object"},
					ProviderOptions: ai.ProviderOptions{
						ProviderOptionsKey: streaming.Options{
							Visible:       true,
							StreamID:      "stream-123",
							AttemptID:     "turn-1",
							FailurePolicy: tt.policy,
						},
					},
				}),
			})
			if !errors.Is(err, streaming.ErrStreamNotFound) {
				t.Fatalf("err = %v, want stream not found", err)
			}
			if modelCalled {
				t.Fatal("model should not be called when strict stream setup fails")
			}
		})
	}
}

func TestStreamObjectPublishesConnectorObjectAndElementSnapshots(t *testing.T) {
	model := ai.NewMockLanguageModel("model-1")
	model.StreamFunc = func(_ context.Context, opts ai.LanguageModelCallOptions) (*ai.LanguageModelStreamResult, error) {
		if _, ok := opts.ProviderOptions[ProviderOptionsKey]; ok {
			t.Fatalf("temporal provider option leaked to model: %#v", opts.ProviderOptions)
		}
		if opts.ResponseFormat == nil || opts.ResponseFormat.Type != "json" {
			t.Fatalf("response format = %#v, want json", opts.ResponseFormat)
		}
		ch := make(chan ai.StreamPart, 3)
		ch <- ai.StreamPart{Type: "text-delta", TextDelta: `{"elements":[{"name":"Ada"},`}
		ch <- ai.StreamPart{Type: "text-delta", TextDelta: `{"name":"Grace"}]}`}
		ch <- ai.StreamPart{Type: "finish", FinishReason: ai.FinishReason{Unified: ai.FinishStop}}
		close(ch)
		return &ai.LanguageModelStreamResult{Stream: ch}, nil
	}
	connector := &recordingConnector{}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"model-1": model},
		},
		StreamConnector: connector,
	})

	result, err := acts.StreamObject(context.Background(), StreamObjectArgs{
		ModelID: "model-1",
		Options: StreamObjectOptionsFromAI(ai.StreamObjectOptions{
			GenerateObjectOptions: ai.GenerateObjectOptions{
				Output: ai.OutputArray,
				Prompt: "return json",
				Schema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"name": map[string]any{"type": "string"}},
					"required":   []any{"name"},
				},
				ProviderOptions: ai.ProviderOptions{
					ProviderOptionsKey: streaming.Options{
						Visible:   true,
						StreamID:  "stream-123",
						AttemptID: "turn-1",
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Elements) != 2 {
		t.Fatalf("elements = %#v", result.Elements)
	}
	if len(result.StreamParts) == 0 {
		t.Fatal("expected object stream parts")
	}
	if len(connector.starts) != 1 {
		t.Fatalf("starts = %d", len(connector.starts))
	}
	if connector.starts[0].Lane != streaming.LaneObject {
		t.Fatalf("start lane = %q", connector.starts[0].Lane)
	}
	var elementChunks int
	for _, chunk := range connector.live {
		if chunk.Delta != "" {
			t.Fatalf("object stream emitted raw JSON delta: %#v", chunk)
		}
		if chunk.Event == streaming.EventElement {
			elementChunks++
		}
	}
	if elementChunks != 2 {
		t.Fatalf("element chunks = %d, live = %#v", elementChunks, connector.live)
	}
	completion, ok := connector.completions[0].SnapshotObject.([]any)
	if !ok || len(completion) != 2 {
		t.Fatalf("completion object = %#v", connector.completions[0].SnapshotObject)
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

func TestInvokeToolPropagatesToolMetadataAndSandbox(t *testing.T) {
	sandbox := testSandbox{}
	toolMetadata := ai.ProviderMetadata{"client": map[string]any{"source": "mcp"}}
	callMetadata := ai.ProviderMetadata{"call": "model"}
	var seenCall ai.ToolCall
	var seenOptions ai.ToolExecutionOptions
	acts := New(Options{
		Sandbox: sandbox,
		Tools: map[string]ai.Tool{
			"lookup": {
				ToolMetadata: toolMetadata,
				InputSchema:  map[string]any{"type": "object"},
				Execute: func(_ context.Context, call ai.ToolCall, opts ai.ToolExecutionOptions) (any, error) {
					seenCall = call
					seenOptions = opts
					return "ok", nil
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID:   "call-1",
		ToolName:     "lookup",
		Input:        map[string]any{},
		ToolMetadata: callMetadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seenCall.ToolMetadata["client"], toolMetadata["client"]) || seenCall.ToolMetadata["call"] != "model" {
		t.Fatalf("call tool metadata = %#v", seenCall.ToolMetadata)
	}
	if seenOptions.Sandbox != sandbox {
		t.Fatalf("sandbox = %#v", seenOptions.Sandbox)
	}
	if !reflect.DeepEqual(result.ToolMetadata, seenCall.ToolMetadata) {
		t.Fatalf("result tool metadata = %#v", result.ToolMetadata)
	}
}

func TestInvokeToolArtifactsCompactLargeOutput(t *testing.T) {
	store := &recordingArtifactStore{}
	big := strings.Repeat("x", 60_000)
	connector := &recordingConnector{}
	acts := New(Options{
		ArtifactStore:   store,
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"lookup": {
				InputSchema: map[string]any{"type": "object"},
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					return big, nil
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Input:      map[string]any{"query": "temporal"},
		Lifecycle: ToolLifecycleOptions{
			StreamID:        "stream-1",
			DurableRequired: true,
		},
		Artifacts: &ToolArtifactPolicy{
			Enabled:         true,
			WorkflowID:      "workflow-1",
			RunID:           "run-1",
			MaxInlineBytes:  1_024,
			MaxPreviewBytes: 32,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.writes) != 2 {
		t.Fatalf("artifact writes = %d, want result and output writes", len(store.writes))
	}
	if _, ok := result.Result.(ToolArtifactValue); !ok {
		t.Fatalf("result was not compacted: %#v", result.Result)
	}
	output, ok := result.Output.Value.(ToolArtifactValue)
	if !ok {
		t.Fatalf("output was not compacted: %#v", result.Output.Value)
	}
	if output.ArtifactRef.ArtifactID == "" || output.OriginalBytes == 0 || output.SHA256 == "" {
		t.Fatalf("artifact value = %#v", output)
	}
	if len(connector.toolDurable) != 2 {
		t.Fatalf("durable lifecycle = %#v", connector.toolDurable)
	}
	terminalOutput, ok := connector.toolDurable[1].Output.(ai.ToolResultOutput)
	if !ok {
		t.Fatalf("terminal lifecycle output = %#v", connector.toolDurable[1].Output)
	}
	terminal, ok := terminalOutput.Value.(ToolArtifactValue)
	if !ok {
		t.Fatalf("terminal lifecycle output = %#v", terminalOutput.Value)
	}
	if artifactJSONByteLength(terminal) > 2_000 {
		t.Fatalf("terminal lifecycle output is too large")
	}
}

func TestInvokeToolAddsLifecycleScopeToExecutionContext(t *testing.T) {
	var seen map[string]any
	stepNumber := 2
	acts := New(Options{
		Tools: map[string]ai.Tool{
			"lookup": {
				InputSchema: map[string]any{"type": "object"},
				Execute: func(_ context.Context, _ ai.ToolCall, opts ai.ToolExecutionOptions) (any, error) {
					var ok bool
					seen, ok = opts.Context.(map[string]any)
					if !ok {
						t.Fatalf("context = %T", opts.Context)
					}
					return "ok", nil
				},
			},
		},
	})

	_, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Input:      map[string]any{},
		Context:    map[string]any{"assistantMessageId": "msg-1"},
		Lifecycle: ToolLifecycleOptions{
			Scope: streaming.Scope{
				DisplayMode: streaming.DisplayModeTask,
				AgentID:     "agent-1",
				TaskID:      "task-1",
				TaskTitle:   "Verify license",
				SkillName:   "License validation",
				StepID:      "step-2",
				StepNumber:  &stepNumber,
				StepType:    "tool-result",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen["assistantMessageId"] != "msg-1" ||
		seen["taskId"] != "task-1" ||
		seen["skillName"] != "License validation" ||
		seen["stepId"] != "step-2" ||
		seen["stepNumber"] != stepNumber ||
		seen["stepType"] != "tool-result" {
		t.Fatalf("scoped context = %#v", seen)
	}
}

func TestToolDefinitionPreservesRequiresApproval(t *testing.T) {
	toolMetadata := ai.ProviderMetadata{"client": "mcp"}
	definition := ToolDefinitionFromAI("write", ai.Tool{RequiresApproval: true, ToolMetadata: toolMetadata})
	if !definition.RequiresApproval {
		t.Fatalf("requires approval was not copied: %#v", definition)
	}
	if !reflect.DeepEqual(definition.ToolMetadata, toolMetadata) {
		t.Fatalf("tool metadata was not copied: %#v", definition)
	}
	modelTool := definition.ToModelTool()
	if !reflect.DeepEqual(modelTool.ToolMetadata, toolMetadata) {
		t.Fatalf("model tool metadata was not copied: %#v", modelTool)
	}
	tool := definition.ToAI()
	if !tool.RequiresApproval {
		t.Fatalf("requires approval was not restored to AI tool")
	}
	if !reflect.DeepEqual(tool.ToolMetadata, toolMetadata) {
		t.Fatalf("tool metadata was not restored to AI tool: %#v", tool.ToolMetadata)
	}
}

func TestInvokeToolPublishesRequiredDurableLifecycle(t *testing.T) {
	connector := &recordingConnector{}
	toolMetadata := ai.ProviderMetadata{"client": "mcp"}
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"lookup": {
				ToolMetadata: toolMetadata,
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
		Lifecycle: ToolLifecycleOptions{
			StreamID:        "stream-1",
			Metadata:        map[string]any{"agentId": "agent-1"},
			DurableRequired: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if len(connector.toolDurable) != 2 {
		t.Fatalf("durable lifecycle = %#v", connector.toolDurable)
	}
	if len(connector.toolLive) != 2 {
		t.Fatalf("live lifecycle = %#v", connector.toolLive)
	}
	if connector.toolDurable[0].Event != streaming.ToolInputAvailable || connector.toolDurable[0].EventID != "tool:call-1:input" {
		t.Fatalf("input lifecycle = %#v", connector.toolDurable[0])
	}
	if !reflect.DeepEqual(connector.toolDurable[0].ToolMetadata, toolMetadata) {
		t.Fatalf("input lifecycle tool metadata = %#v", connector.toolDurable[0].ToolMetadata)
	}
	if connector.toolDurable[1].Event != streaming.ToolOutputAvailable || connector.toolDurable[1].EventID != "tool:call-1:terminal" {
		t.Fatalf("terminal lifecycle = %#v", connector.toolDurable[1])
	}
	if !reflect.DeepEqual(connector.toolDurable[1].ToolMetadata, toolMetadata) {
		t.Fatalf("terminal lifecycle tool metadata = %#v", connector.toolDurable[1].ToolMetadata)
	}
}

func TestInvokeToolPreservesToolResultFiles(t *testing.T) {
	connector := &recordingConnector{}
	file := ai.ToolResultFile{URL: "https://example.test/report.pdf", MediaType: "application/pdf", Filename: "report.pdf"}
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"report": {
				InputSchema: map[string]any{"type": "object"},
				ToModelOutput: func(string, any, any) (ai.ToolResultOutput, error) {
					return ai.ToolResultOutput{Type: "json", Value: map[string]any{"ok": true}, Files: []ai.ToolResultFile{file}}, nil
				},
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					return map[string]any{"ok": true}, nil
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "report",
		Input:      map[string]any{},
		Lifecycle:  ToolLifecycleOptions{StreamID: "stream-1", DurableRequired: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Output.Files, []ai.ToolResultFile{file}) {
		t.Fatalf("result files = %#v", result.Output.Files)
	}
	terminal, ok := connector.toolDurable[1].Output.(ai.ToolResultOutput)
	if !ok || !reflect.DeepEqual(terminal.Files, []ai.ToolResultFile{file}) {
		t.Fatalf("terminal files = %#v", connector.toolDurable[1].Output)
	}
}

func TestInvokeToolRequiresApprovalWithoutDecisionDeniesExecution(t *testing.T) {
	connector := &recordingConnector{}
	executed := false
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"write": {
				RequiresApproval: true,
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					executed = true
					return "written", nil
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "write",
		Input:      map[string]any{"id": "1"},
		Lifecycle:  ToolLifecycleOptions{StreamID: "stream-1", DurableRequired: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if executed {
		t.Fatal("tool executed without approval")
	}
	if result.Output.Type != "execution-denied" {
		t.Fatalf("result = %#v", result)
	}
	if connector.toolDurable[1].Event != streaming.ToolOutputDenied {
		t.Fatalf("terminal lifecycle = %#v", connector.toolDurable)
	}
}

func TestInvokeToolApprovedDecisionExecutesAndCanSuppressInputLifecycle(t *testing.T) {
	connector := &recordingConnector{}
	approved := true
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"write": {
				RequiresApproval: true,
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					return "written", nil
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID:             "call-1",
		ToolName:               "write",
		Input:                  map[string]any{"id": "1"},
		Lifecycle:              ToolLifecycleOptions{StreamID: "stream-1", DurableRequired: true},
		Approval:               &ToolApprovalState{ApprovalID: "approval-1", Approved: &approved},
		SuppressInputLifecycle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output.Value != "written" {
		t.Fatalf("result = %#v", result)
	}
	if len(connector.toolDurable) != 1 || connector.toolDurable[0].Event != streaming.ToolOutputAvailable {
		t.Fatalf("durable lifecycle = %#v", connector.toolDurable)
	}
}

func TestInvokeToolDurableInputFailurePreventsExecution(t *testing.T) {
	connector := &recordingConnector{
		toolPersistErrForEvent: map[streaming.ToolLifecycleEvent]error{
			streaming.ToolInputAvailable: errors.New("durable input failed"),
		},
	}
	var executed bool
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"lookup": {
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					executed = true
					return "found", nil
				},
			},
		},
	})

	_, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Input:      map[string]any{"query": "temporal"},
		Lifecycle:  ToolLifecycleOptions{StreamID: "stream-1", DurableRequired: true},
	})
	if err == nil {
		t.Fatal("expected durable lifecycle error")
	}
	if executed {
		t.Fatal("tool executed after durable input failure")
	}
}

func TestInvokeToolDurableTerminalFailureFailsActivity(t *testing.T) {
	connector := &recordingConnector{
		toolPersistErrForEvent: map[streaming.ToolLifecycleEvent]error{
			streaming.ToolOutputAvailable: errors.New("durable terminal failed"),
		},
	}
	var executions int
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"lookup": {
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					executions++
					return "found", nil
				},
			},
		},
	})

	_, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Input:      map[string]any{"query": "temporal"},
		Lifecycle:  ToolLifecycleOptions{StreamID: "stream-1", DurableRequired: true},
	})
	if err == nil {
		t.Fatal("expected durable lifecycle error")
	}
	if executions != 1 {
		t.Fatalf("executions = %d", executions)
	}
}

func TestInvokeToolLiveLifecycleFailureIsNonFatal(t *testing.T) {
	connector := &recordingConnector{toolLiveErr: errors.New("live failed")}
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"lookup": {
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					return "found", nil
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Input:      map[string]any{"query": "temporal"},
		Lifecycle:  ToolLifecycleOptions{StreamID: "stream-1", DurableRequired: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if len(connector.toolDurable) != 2 {
		t.Fatalf("durable lifecycle = %#v", connector.toolDurable)
	}
}

func TestInvokeToolPublishesErrorLifecycleResult(t *testing.T) {
	connector := &recordingConnector{}
	acts := New(Options{
		StreamConnector: connector,
		Tools: map[string]ai.Tool{
			"lookup": {
				Execute: func(context.Context, ai.ToolCall, ai.ToolExecutionOptions) (any, error) {
					return nil, errors.New("lookup failed")
				},
			},
		},
	})

	result, err := acts.InvokeTool(context.Background(), InvokeToolArgs{
		ToolCallID: "call-1",
		ToolName:   "lookup",
		Input:      map[string]any{"query": "temporal"},
		Lifecycle:  ToolLifecycleOptions{StreamID: "stream-1", DurableRequired: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("result = %#v", result)
	}
	if len(connector.toolDurable) != 2 {
		t.Fatalf("durable lifecycle = %#v", connector.toolDurable)
	}
	terminal := connector.toolDurable[1]
	if terminal.Event != streaming.ToolOutputError || terminal.ErrorText != "lookup failed" {
		t.Fatalf("terminal lifecycle = %#v", terminal)
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

func TestInvokeModelStreamPublishesPartialJSONOutputSnapshots(t *testing.T) {
	model := ai.NewMockLanguageModel("stream-1")
	model.StreamFunc = func(_ context.Context, opts ai.LanguageModelCallOptions) (*ai.LanguageModelStreamResult, error) {
		if opts.ResponseFormat == nil || opts.ResponseFormat.Type != "json" {
			t.Fatalf("response format = %#v, want json", opts.ResponseFormat)
		}
		ch := make(chan ai.StreamPart, 3)
		ch <- ai.StreamPart{Type: "text-delta", TextDelta: `{"status"`}
		ch <- ai.StreamPart{Type: "text-delta", TextDelta: `:"needs_user"}`}
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

	_, err := acts.InvokeModelStream(context.Background(), InvokeModelStreamArgs{
		ModelID: "stream-1",
		Options: LanguageModelCallOptionsFromAI(ai.LanguageModelCallOptions{
			ResponseFormat: &ai.ResponseFormat{Type: "json", Schema: map[string]any{"type": "object"}},
			ProviderOptions: ai.ProviderOptions{
				ProviderOptionsKey: streaming.Options{
					Visible:   true,
					StreamID:  "stream-123",
					AttemptID: "turn-1",
					Lane:      streaming.LaneObject,
				},
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(connector.live) != 2 {
		t.Fatalf("live chunks = %#v", connector.live)
	}
	if connector.live[0].Delta != "" {
		t.Fatalf("object lane delta = %q, want raw JSON suppressed", connector.live[0].Delta)
	}
	object, ok := connector.live[0].SnapshotObject.(map[string]any)
	if !ok || object["status"] != "needs_user" {
		t.Fatalf("live object = %#v", connector.live[0].SnapshotObject)
	}
	completion, ok := connector.completions[0].SnapshotObject.(map[string]any)
	if !ok || completion["status"] != "needs_user" {
		t.Fatalf("completion object = %#v", connector.completions[0].SnapshotObject)
	}
}

func TestInvokeModelStreamParsesToolCallInputRaw(t *testing.T) {
	model := ai.NewMockLanguageModel("stream-1")
	model.StreamFunc = func(context.Context, ai.LanguageModelCallOptions) (*ai.LanguageModelStreamResult, error) {
		ch := make(chan ai.StreamPart, 2)
		ch <- ai.StreamPart{Type: "tool-call", ToolCallID: "call-1", ToolName: "extractDocument", ToolInput: `{"s3Uri":"s3://bucket/resume.pdf","extractionType":"general"}`}
		ch <- ai.StreamPart{Type: "finish", FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls}}
		close(ch)
		return &ai.LanguageModelStreamResult{Stream: ch}, nil
	}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"stream-1": model},
		},
	})

	result, err := acts.InvokeModelStream(context.Background(), InvokeModelStreamArgs{
		ModelID: "stream-1",
		Options: LanguageModelCallOptionsFromAI(ai.LanguageModelCallOptions{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result == nil || len(result.Result.Content) != 1 {
		t.Fatalf("result content = %#v", result.Result)
	}
	call := result.Result.Content[0]
	input, ok := call.Input.(map[string]any)
	if !ok {
		t.Fatalf("tool input = %#v, want map", call.Input)
	}
	if input["s3Uri"] != "s3://bucket/resume.pdf" {
		t.Fatalf("s3Uri = %#v", input["s3Uri"])
	}
	aiCall, ok := call.ToAI().(ai.ToolCallPart)
	if !ok {
		t.Fatalf("ToAI = %#v", call.ToAI())
	}
	if aiCall.Input == nil {
		t.Fatalf("ToAI input is nil")
	}
}

func TestInvokeModelStreamPreservesReasoningFileParts(t *testing.T) {
	model := ai.NewMockLanguageModel("stream-1")
	model.StreamFunc = func(context.Context, ai.LanguageModelCallOptions) (*ai.LanguageModelStreamResult, error) {
		ch := make(chan ai.StreamPart, 2)
		ch <- ai.StreamPart{
			Type: "reasoning-file",
			Content: ai.ReasoningFilePart{
				Data:      ai.FileData{Type: "url", URL: "https://example.test/reasoning.png"},
				MediaType: "image/png",
			},
		}
		ch <- ai.StreamPart{Type: "finish", FinishReason: ai.FinishReason{Unified: ai.FinishStop}}
		close(ch)
		return &ai.LanguageModelStreamResult{Stream: ch}, nil
	}
	acts := New(Options{
		ModelProvider: ai.CustomProvider{
			LanguageModels: map[string]ai.LanguageModel{"stream-1": model},
		},
	})

	result, err := acts.InvokeModelStream(context.Background(), InvokeModelStreamArgs{
		ModelID: "stream-1",
		Options: LanguageModelCallOptionsFromAI(ai.LanguageModelCallOptions{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Result == nil || len(result.Result.Content) != 1 {
		t.Fatalf("result content = %#v", result.Result)
	}
	part := result.Result.Content[0]
	if part.Type != "reasoning-file" {
		t.Fatalf("part type = %q", part.Type)
	}
	aiPart, ok := part.ToAI().(ai.ReasoningFilePart)
	if !ok {
		t.Fatalf("ToAI = %#v", part.ToAI())
	}
	if aiPart.Data.URL != "https://example.test/reasoning.png" {
		t.Fatalf("reasoning file url = %q", aiPart.Data.URL)
	}
}

func TestWirePreservesTextAndFileProviderMetadata(t *testing.T) {
	metadata := ai.ProviderMetadata{"googleVertex": map[string]any{"thoughtSignature": "sig-1"}}

	textWire := PartFromAI(ai.TextPart{Text: "hello", ProviderMetadata: metadata})
	if !reflect.DeepEqual(textWire.ProviderMetadata, metadata) {
		t.Fatalf("text wire metadata = %#v", textWire.ProviderMetadata)
	}
	textAI, ok := textWire.ToAI().(ai.TextPart)
	if !ok || !reflect.DeepEqual(textAI.ProviderMetadata, metadata) {
		t.Fatalf("text ToAI metadata = %#v", textWire.ToAI())
	}

	fileWire := PartFromAI(ai.FilePart{
		Data:             ai.FileData{Type: "url", URL: "https://example.test/doc.txt"},
		MediaType:        "text/plain",
		Filename:         "doc.txt",
		ProviderMetadata: metadata,
	})
	if !reflect.DeepEqual(fileWire.ProviderMetadata, metadata) {
		t.Fatalf("file wire metadata = %#v", fileWire.ProviderMetadata)
	}
	fileAI, ok := fileWire.ToAI().(ai.FilePart)
	if !ok || !reflect.DeepEqual(fileAI.ProviderMetadata, metadata) {
		t.Fatalf("file ToAI metadata = %#v", fileWire.ToAI())
	}
}

func TestWirePreservesToolMetadataAndPerformance(t *testing.T) {
	toolMetadata := ai.ProviderMetadata{"client": "mcp"}
	providerMetadata := ai.ProviderMetadata{"provider": "mock"}

	callWire := PartFromAI(ai.ToolCallPart{
		ToolCallID:       "call-1",
		ToolName:         "lookup",
		Input:            map[string]any{"query": "temporal"},
		ToolMetadata:     toolMetadata,
		ProviderMetadata: providerMetadata,
	})
	if !reflect.DeepEqual(callWire.ToolMetadata, toolMetadata) {
		t.Fatalf("tool-call wire metadata = %#v", callWire.ToolMetadata)
	}
	callAI, ok := callWire.ToAI().(ai.ToolCallPart)
	if !ok || !reflect.DeepEqual(callAI.ToolMetadata, toolMetadata) {
		t.Fatalf("tool-call ToAI metadata = %#v", callWire.ToAI())
	}

	resultWire := PartFromAI(ai.ToolResultPart{
		ToolCallID:       "call-1",
		ToolName:         "lookup",
		Output:           ai.ToolResultOutput{Type: "text", Value: "ok"},
		ToolMetadata:     toolMetadata,
		ProviderMetadata: providerMetadata,
	})
	resultAI, ok := resultWire.ToAI().(ai.ToolResultPart)
	if !ok || !reflect.DeepEqual(resultAI.ToolMetadata, toolMetadata) {
		t.Fatalf("tool-result ToAI metadata = %#v", resultWire.ToAI())
	}

	performance := ai.StepPerformance{StepTime: 2 * time.Second, TimeToFirstOutputToken: 25 * time.Millisecond}
	streamWire := StreamPartFromAI(ai.StreamPart{
		Type:         "finish-step",
		Performance:  performance,
		ToolMetadata: toolMetadata,
	})
	if streamWire.Performance.StepTime != performance.StepTime || !reflect.DeepEqual(streamWire.ToolMetadata, toolMetadata) {
		t.Fatalf("stream wire = %#v", streamWire)
	}
	streamAI := streamWire.ToAI()
	if streamAI.Performance.StepTime != performance.StepTime || !reflect.DeepEqual(streamAI.ToolMetadata, toolMetadata) {
		t.Fatalf("stream ToAI = %#v", streamAI)
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
	starts                 []streaming.AttemptRef
	live                   []streaming.LiveChunk
	snapshots              []streaming.AttemptSnapshot
	completions            []streaming.AttemptCompletion
	tools                  []streaming.ToolLifecycleInput
	startErr               error
	liveErr                error
	snapshotErr            error
	completionErr          error
	toolDurable            []streaming.ToolLifecycleInput
	toolLive               []streaming.ToolLifecycleInput
	toolPersistErrForEvent map[streaming.ToolLifecycleEvent]error
	toolLiveErr            error
}

type testSandbox struct{}

func (testSandbox) RunCommand(context.Context, ai.SandboxCommand) (ai.SandboxCommandResult, error) {
	return ai.SandboxCommandResult{Stdout: "ok"}, nil
}

func (testSandbox) ReadFile(context.Context, string) ([]byte, error) {
	return []byte("ok"), nil
}

func (testSandbox) WriteFile(context.Context, string, []byte) error {
	return nil
}

type recordingArtifactStore struct {
	writes []ToolArtifactWriteInput
}

func (s *recordingArtifactStore) PutToolArtifact(_ context.Context, input ToolArtifactWriteInput) (*ToolArtifactRef, error) {
	s.writes = append(s.writes, input)
	return &ToolArtifactRef{
		ArtifactID:    fmt.Sprintf("%s/%s/%s/%s.json", input.WorkflowID, input.ToolCallID, input.Kind, input.SHA256),
		Kind:          input.Kind,
		OriginalBytes: input.OriginalBytes,
		ContentType:   input.ContentType,
		SHA256:        input.SHA256,
	}, nil
}

func (c *recordingConnector) StartAttempt(_ context.Context, input streaming.AttemptRef) error {
	c.starts = append(c.starts, input)
	return c.startErr
}

func (c *recordingConnector) PublishLiveChunk(_ context.Context, input streaming.LiveChunk) error {
	c.live = append(c.live, input)
	return c.liveErr
}

func (c *recordingConnector) UpdateAttemptSnapshot(_ context.Context, input streaming.AttemptSnapshot) error {
	c.snapshots = append(c.snapshots, input)
	return c.snapshotErr
}

func (c *recordingConnector) CompleteAttempt(_ context.Context, input streaming.AttemptCompletion) error {
	c.completions = append(c.completions, input)
	return c.completionErr
}

func (c *recordingConnector) PublishToolLifecycleEvent(_ context.Context, input streaming.ToolLifecycleInput) error {
	c.tools = append(c.tools, input)
	return nil
}

func (c *recordingConnector) PersistToolLifecycleEvent(_ context.Context, input streaming.ToolLifecycleInput) error {
	if c.toolPersistErrForEvent != nil {
		if err := c.toolPersistErrForEvent[input.Event]; err != nil {
			return err
		}
	}
	c.toolDurable = append(c.toolDurable, input)
	return nil
}

func (c *recordingConnector) PublishLiveToolLifecycleEvent(_ context.Context, input streaming.ToolLifecycleInput) error {
	if c.toolLiveErr != nil {
		return c.toolLiveErr
	}
	c.toolLive = append(c.toolLive, input)
	return nil
}

package temporalai

import (
	"context"
	"testing"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestRunAgentExecutesToolActivityAndContinues(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var modelCalls int
	var lifecycle []streaming.ToolLifecycleInput

	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			modelCalls++
			if modelCalls == 1 {
				if len(args.Options.Tools) != 1 || args.Options.Tools[0].Name != "lookup" {
					t.Fatalf("model tools = %#v", args.Options.Tools)
				}
				return &activities.InvokeModelResult{
					Content: []activities.Part{{
						Type:       "tool-call",
						ToolCallID: "call-1",
						ToolName:   "lookup",
						Input:      map[string]any{"query": "temporal"},
					}},
					FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls},
				}, nil
			}
			last := args.Options.Prompt[len(args.Options.Prompt)-1]
			if last.Role != ai.RoleTool || len(last.Content) != 1 {
				t.Fatalf("last prompt message = %#v", last)
			}
			return &activities.InvokeModelResult{
				Content:      []activities.Part{{Type: "text", Text: "Temporal result"}},
				FinishReason: ai.FinishReason{Unified: ai.FinishStop},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
			if args.ToolName != "lookup" || args.ToolCallID != "call-1" {
				t.Fatalf("tool args = %#v", args)
			}
			return &activities.InvokeToolResult{
				ToolCallID: args.ToolCallID,
				ToolName:   args.ToolName,
				Input:      args.Input,
				Output:     ai.ToolResultOutput{Type: "text", Value: "lookup output"},
				Result:     "lookup output",
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeToolActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, input activities.PublishToolLifecycleEventArgs) error {
			lifecycle = append(lifecycle, streaming.ToolLifecycleInput(input))
			return nil
		},
		activity.RegisterOptions{Name: activities.PublishToolLifecycleEventActivity},
	)

	env.ExecuteWorkflow(testAgentWorkflow, AgentInput{
		AgentID:       "agent-1",
		ModelID:       "model-1",
		Prompt:        "run lookup",
		Stream:        streaming.Options{StreamID: "stream-1"},
		ToolExecution: ToolExecutionSequential,
		Tools: []activities.ToolDefinition{{
			Name:        "lookup",
			Description: "Look something up",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
				},
			},
		}},
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result AgentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result.Text != "Temporal result" {
		t.Fatalf("text = %q", result.Text)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("steps = %d", len(result.Steps))
	}
	if modelCalls != 2 {
		t.Fatalf("model calls = %d", modelCalls)
	}
	if len(lifecycle) != 2 {
		t.Fatalf("lifecycle events = %#v", lifecycle)
	}
	if lifecycle[0].Event != streaming.ToolInputAvailable || lifecycle[1].Event != streaming.ToolOutputAvailable {
		t.Fatalf("lifecycle = %#v", lifecycle)
	}
}

func testAgentWorkflow(ctx workflow.Context, input AgentInput) (*AgentResult, error) {
	return RunAgent(ctx, input)
}

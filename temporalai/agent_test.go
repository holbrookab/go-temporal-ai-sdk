package temporalai

import (
	"context"
	"testing"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestRunAgentExecutesToolActivityAndContinues(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var modelCalls int

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
			if args.Lifecycle.StreamID != "stream-1" || !args.Lifecycle.DurableRequired {
				t.Fatalf("tool lifecycle = %#v", args.Lifecycle)
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
}

func TestRunAgentAppliesFirstToolChoiceOnlyOnFirstStep(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var choices []ai.ToolChoice

	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			choices = append(choices, args.Options.ToolChoice)
			if len(choices) == 1 {
				return &activities.InvokeModelResult{
					Content: []activities.Part{{
						Type:       "tool-call",
						ToolCallID: "call-1",
						ToolName:   "extractDocument",
						Input:      map[string]any{"s3Uri": "s3://bucket/resume.pdf"},
					}},
					FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls},
				}, nil
			}
			return &activities.InvokeModelResult{
				Content:      []activities.Part{{Type: "text", Text: "Done"}},
				FinishReason: ai.FinishReason{Unified: ai.FinishStop},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
			if args.Lifecycle.StreamID != "" {
				t.Fatalf("unexpected lifecycle without visible stream: %#v", args.Lifecycle)
			}
			return &activities.InvokeToolResult{
				ToolCallID: args.ToolCallID,
				ToolName:   args.ToolName,
				Input:      args.Input,
				Output:     ai.ToolResultOutput{Type: "json", Value: map[string]any{"text": "resume text"}},
				Result:     map[string]any{"text": "resume text"},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeToolActivity},
	)

	env.ExecuteWorkflow(testAgentWorkflow, AgentInput{
		AgentID:         "agent-1",
		ModelID:         "model-1",
		Prompt:          "open attached document",
		FirstToolChoice: ai.ToolChoiceFor("extractDocument"),
		Tools: []activities.ToolDefinition{{
			Name:        "extractDocument",
			Description: "Extract document text",
			InputSchema: map[string]any{"type": "object"},
		}},
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if len(choices) != 2 {
		t.Fatalf("tool choices = %#v", choices)
	}
	if choices[0].Type != "tool" || choices[0].ToolName != "extractDocument" {
		t.Fatalf("first tool choice = %#v", choices[0])
	}
	if choices[1].Type != "auto" {
		t.Fatalf("second tool choice = %#v, want auto", choices[1])
	}
}

func TestRunAgentUsesLocalToolBoundaryDefault(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var localToolStarts int

	env.SetOnLocalActivityStartedListener(func(info *activity.Info, _ context.Context, _ []interface{}) {
		if info.ActivityType.Name == activities.InvokeToolActivity {
			localToolStarts++
		}
	})
	registerOneToolAgentActivities(t, env)

	env.ExecuteWorkflow(testAgentWorkflow, AgentInput{
		AgentID:             "agent-1",
		ModelID:             "model-1",
		Prompt:              "run lookup",
		DefaultToolBoundary: activities.ToolExecutionBoundaryLocalActivity,
		Tools: []activities.ToolDefinition{{
			Name:        "lookup",
			Description: "Look something up",
			InputSchema: map[string]any{"type": "object"},
		}},
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if localToolStarts != 1 {
		t.Fatalf("local tool starts = %d, want 1", localToolStarts)
	}
}

func TestRunAgentPerToolActivityBoundaryOverridesLocalDefault(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var localToolStarts int
	var regularToolStarts int

	env.SetOnLocalActivityStartedListener(func(info *activity.Info, _ context.Context, _ []interface{}) {
		if info.ActivityType.Name == activities.InvokeToolActivity {
			localToolStarts++
		}
	})
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		if info.ActivityType.Name == activities.InvokeToolActivity {
			regularToolStarts++
		}
	})
	registerOneToolAgentActivities(t, env)

	env.ExecuteWorkflow(testAgentWorkflow, AgentInput{
		AgentID:             "agent-1",
		ModelID:             "model-1",
		Prompt:              "run lookup",
		DefaultToolBoundary: activities.ToolExecutionBoundaryLocalActivity,
		Tools: []activities.ToolDefinition{{
			Name:              "lookup",
			Description:       "Look something up",
			InputSchema:       map[string]any{"type": "object"},
			ExecutionBoundary: activities.ToolExecutionBoundaryActivity,
		}},
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if localToolStarts != 0 {
		t.Fatalf("local tool starts = %d, want 0", localToolStarts)
	}
	if regularToolStarts != 1 {
		t.Fatalf("regular tool starts = %d, want 1", regularToolStarts)
	}
}

func TestRunAgentMixedParallelToolBoundariesPreserveResultOrder(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var localToolStarts int
	var regularToolStarts int

	env.SetOnLocalActivityStartedListener(func(info *activity.Info, _ context.Context, _ []interface{}) {
		if info.ActivityType.Name == activities.InvokeToolActivity {
			localToolStarts++
		}
	})
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		if info.ActivityType.Name == activities.InvokeToolActivity {
			regularToolStarts++
		}
	})
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			return &activities.InvokeModelResult{
				Content: []activities.Part{
					{Type: "tool-call", ToolCallID: "call-local", ToolName: "localLookup", Input: map[string]any{"query": "local"}},
					{Type: "tool-call", ToolCallID: "call-regular", ToolName: "regularLookup", Input: map[string]any{"query": "regular"}},
				},
				FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
			return &activities.InvokeToolResult{
				ToolCallID: args.ToolCallID,
				ToolName:   args.ToolName,
				Input:      args.Input,
				Output:     ai.ToolResultOutput{Type: "text", Value: args.ToolName},
				Result:     args.ToolName,
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeToolActivity},
	)

	env.ExecuteWorkflow(testAgentWorkflow, AgentInput{
		AgentID:             "agent-1",
		ModelID:             "model-1",
		Prompt:              "run lookups",
		MaxSteps:            1,
		DefaultToolBoundary: activities.ToolExecutionBoundaryLocalActivity,
		Tools: []activities.ToolDefinition{
			{Name: "localLookup", InputSchema: map[string]any{"type": "object"}},
			{Name: "regularLookup", InputSchema: map[string]any{"type": "object"}, ExecutionBoundary: activities.ToolExecutionBoundaryActivity},
		},
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
	if localToolStarts != 1 {
		t.Fatalf("local tool starts = %d, want 1", localToolStarts)
	}
	if regularToolStarts != 1 {
		t.Fatalf("regular tool starts = %d, want 1", regularToolStarts)
	}
	if len(result.Steps) != 1 || len(result.Steps[0].ToolResults) != 2 {
		t.Fatalf("tool results = %#v", result.Steps)
	}
	if result.Steps[0].ToolResults[0].ToolName != "localLookup" || result.Steps[0].ToolResults[1].ToolName != "regularLookup" {
		t.Fatalf("tool result order = %#v", result.Steps[0].ToolResults)
	}
}

func registerOneToolAgentActivities(t *testing.T, env *testsuite.TestWorkflowEnvironment) {
	t.Helper()
	var modelCalls int
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			modelCalls++
			if modelCalls == 1 {
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
			return &activities.InvokeModelResult{
				Content:      []activities.Part{{Type: "text", Text: "Done"}},
				FinishReason: ai.FinishReason{Unified: ai.FinishStop},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
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
}

func testAgentWorkflow(ctx workflow.Context, input AgentInput) (*AgentResult, error) {
	return RunAgent(ctx, input)
}

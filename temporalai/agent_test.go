package temporalai

import (
	"context"
	"testing"
	"time"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestToolLifecycleMetadataFromContextCopiesTaskFields(t *testing.T) {
	metadata := toolLifecycleMetadataFromContext(struct {
		TaskID             string `json:"taskId"`
		TaskTitle          string `json:"taskTitle"`
		AssistantMessageID string `json:"assistantMessageId"`
	}{
		TaskID:             "task-1",
		TaskTitle:          "Find records",
		AssistantMessageID: "message-1",
	})
	if metadata["taskId"] != "task-1" || metadata["taskTitle"] != "Find records" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if _, ok := metadata["assistantMessageId"]; ok {
		t.Fatalf("metadata leaked persistence field: %#v", metadata)
	}
}

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

func TestRunAgentRequestsApprovalBeforeApprovalRequiredTool(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var lifecycle []streaming.ToolLifecycleInput
	var toolArgs activities.InvokeToolArgs

	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			return &activities.InvokeModelResult{
				Content: []activities.Part{{
					Type:       "tool-call",
					ToolCallID: "call-1",
					ToolName:   "create_worker",
					Input:      map[string]any{"name": "Ada"},
				}},
				FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
			toolArgs = args
			return &activities.InvokeToolResult{
				ToolCallID: args.ToolCallID,
				ToolName:   args.ToolName,
				Input:      args.Input,
				Output:     ai.ToolResultOutput{Type: "text", Value: "created"},
				Result:     "created",
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeToolActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.PublishToolLifecycleEventArgs) error {
			lifecycle = append(lifecycle, streaming.ToolLifecycleInput(args))
			return nil
		},
		activity.RegisterOptions{Name: activities.PublishToolLifecycleEventActivity},
	)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ToolApprovalResponseSignalName("call-1:approval"), ToolApprovalResponse{
			ApprovalID: "call-1:approval",
			Approved:   true,
			Reason:     "approved by user",
		})
	}, time.Second)

	env.ExecuteWorkflow(testAgentWorkflow, AgentInput{
		AgentID:  "agent-1",
		ModelID:  "model-1",
		Prompt:   "create worker",
		MaxSteps: 1,
		Stream:   streaming.Options{StreamID: "stream-1"},
		Tools: []activities.ToolDefinition{{
			Name:             "create_worker",
			InputSchema:      map[string]any{"type": "object"},
			RequiresApproval: true,
		}},
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if toolArgs.Approval == nil || toolArgs.Approval.Approved == nil || !*toolArgs.Approval.Approved {
		t.Fatalf("tool approval args = %#v", toolArgs.Approval)
	}
	if !toolArgs.SuppressInputLifecycle {
		t.Fatalf("expected input lifecycle to be suppressed after workflow published it")
	}
	if len(lifecycle) < 3 {
		t.Fatalf("lifecycle = %#v", lifecycle)
	}
	if lifecycle[0].Event != streaming.ToolInputAvailable ||
		lifecycle[1].Event != streaming.ToolApprovalRequest ||
		lifecycle[2].Event != streaming.ToolApprovalResponse {
		t.Fatalf("lifecycle = %#v", lifecycle)
	}
}

func TestRunAgentDeniesApprovalRequiredToolWhenUserDenies(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var lifecycle []streaming.ToolLifecycleInput
	var toolStarts int

	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			return &activities.InvokeModelResult{
				Content: []activities.Part{{
					Type:       "tool-call",
					ToolCallID: "call-1",
					ToolName:   "create_worker",
					Input:      map[string]any{"name": "Ada"},
				}},
				FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(context.Context, activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
			toolStarts++
			return nil, nil
		},
		activity.RegisterOptions{Name: activities.InvokeToolActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.PublishToolLifecycleEventArgs) error {
			lifecycle = append(lifecycle, streaming.ToolLifecycleInput(args))
			return nil
		},
		activity.RegisterOptions{Name: activities.PublishToolLifecycleEventActivity},
	)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(ToolApprovalResponseSignalName("call-1:approval"), ToolApprovalResponse{
			ApprovalID: "call-1:approval",
			Approved:   false,
			Reason:     "not yet",
		})
	}, time.Second)

	env.ExecuteWorkflow(testAgentWorkflow, AgentInput{
		AgentID:  "agent-1",
		ModelID:  "model-1",
		Prompt:   "create worker",
		MaxSteps: 1,
		Stream:   streaming.Options{StreamID: "stream-1"},
		Tools: []activities.ToolDefinition{{
			Name:             "create_worker",
			InputSchema:      map[string]any{"type": "object"},
			RequiresApproval: true,
		}},
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if toolStarts != 0 {
		t.Fatalf("tool starts = %d", toolStarts)
	}
	var result AgentResult
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if got := result.Steps[0].ToolResults[0].Output; got.Type != "execution-denied" || got.Reason != "not yet" {
		t.Fatalf("tool result = %#v", got)
	}
	if lifecycle[len(lifecycle)-1].Event != streaming.ToolOutputDenied {
		t.Fatalf("lifecycle = %#v", lifecycle)
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

func TestRunAgentFallsBackToActivityAfterLocalToolTimeout(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var localToolStarts int
	var regularToolStarts int
	var toolExecutions int

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
				Content: []activities.Part{{
					Type:       "tool-call",
					ToolCallID: "call-1",
					ToolName:   "lookup",
					Input:      map[string]any{"query": "temporal"},
				}},
				FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
			toolExecutions++
			if toolExecutions == 1 {
				return nil, temporal.NewTimeoutError(enumspb.TIMEOUT_TYPE_START_TO_CLOSE, nil)
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

	env.ExecuteWorkflow(testAgentWorkflowWithOneLocalToolAttempt, AgentInput{
		AgentID:             "agent-1",
		ModelID:             "model-1",
		Prompt:              "run lookup",
		MaxSteps:            1,
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
	if regularToolStarts != 1 {
		t.Fatalf("regular tool starts = %d, want 1", regularToolStarts)
	}
}

func TestRunAgentCanDisableLocalToolTimeoutFallback(t *testing.T) {
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
				Content: []activities.Part{{
					Type:       "tool-call",
					ToolCallID: "call-1",
					ToolName:   "lookup",
					Input:      map[string]any{"query": "temporal"},
				}},
				FinishReason: ai.FinishReason{Unified: ai.FinishToolCalls},
			}, nil
		},
		activity.RegisterOptions{Name: activities.InvokeModelActivity},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeToolArgs) (*activities.InvokeToolResult, error) {
			return nil, temporal.NewTimeoutError(enumspb.TIMEOUT_TYPE_START_TO_CLOSE, nil)
		},
		activity.RegisterOptions{Name: activities.InvokeToolActivity},
	)

	env.ExecuteWorkflow(testAgentWorkflowWithOneLocalToolAttempt, AgentInput{
		AgentID:                  "agent-1",
		ModelID:                  "model-1",
		Prompt:                   "run lookup",
		DefaultToolBoundary:      activities.ToolExecutionBoundaryLocalActivity,
		LocalToolTimeoutFallback: LocalToolTimeoutFallbackNone,
		Tools: []activities.ToolDefinition{{
			Name:        "lookup",
			Description: "Look something up",
			InputSchema: map[string]any{"type": "object"},
		}},
	})
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("workflow error is nil, want local timeout")
	}
	if localToolStarts != 1 {
		t.Fatalf("local tool starts = %d, want 1", localToolStarts)
	}
	if regularToolStarts != 0 {
		t.Fatalf("regular tool starts = %d, want 0", regularToolStarts)
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

func testAgentWorkflowWithOneLocalToolAttempt(ctx workflow.Context, input AgentInput) (*AgentResult, error) {
	return RunAgent(ctx, input, ActivityOptions{
		LocalTool: workflow.LocalActivityOptions{
			StartToCloseTimeout: time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		},
	})
}

package temporalai

import (
	"context"
	"testing"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestInvokeModelWorkflowHelper(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			if args.ModelID != "model-1" {
				t.Fatalf("model = %q", args.ModelID)
			}
			return &activities.InvokeModelResult{
				Content:      activities.PartsFromAI([]ai.Part{ai.TextPart{Text: "ok"}}),
				FinishReason: ai.FinishReason{Unified: ai.FinishStop},
			}, nil
		},
		activityRegisterOptions(activities.InvokeModelActivity),
	)

	env.ExecuteWorkflow(testInvokeModelWorkflow)
	if !env.IsWorkflowCompleted() {
		t.Fatal("workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result string
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Fatalf("result = %q", result)
	}
}

func testInvokeModelWorkflow(ctx workflow.Context) (string, error) {
	result, err := InvokeModel(ctx, "model-1", ai.LanguageModelCallOptions{})
	if err != nil {
		return "", err
	}
	return ai.TextFromParts(result.Content), nil
}

func activityRegisterOptions(name string) activity.RegisterOptions {
	return activity.RegisterOptions{Name: name}
}

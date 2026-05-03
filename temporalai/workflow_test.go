package temporalai

import (
	"context"
	"testing"
	"time"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
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

func TestInvokeModelCanRunAsLocalActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var regularStarts int
	var localStarts int
	env.SetOnActivityStartedListener(func(info *activity.Info, _ context.Context, _ converter.EncodedValues) {
		if info.ActivityType.Name == activities.InvokeModelActivity {
			regularStarts++
		}
	})
	env.SetOnLocalActivityStartedListener(func(info *activity.Info, _ context.Context, _ []interface{}) {
		if info.ActivityType.Name == activities.InvokeModelActivity {
			localStarts++
		}
	})
	env.RegisterActivityWithOptions(
		func(_ context.Context, args activities.InvokeModelArgs) (*activities.InvokeModelResult, error) {
			if args.ModelID != "model-1" {
				t.Fatalf("model = %q", args.ModelID)
			}
			return &activities.InvokeModelResult{
				Content:      activities.PartsFromAI([]ai.Part{ai.TextPart{Text: "local ok"}}),
				FinishReason: ai.FinishReason{Unified: ai.FinishStop},
			}, nil
		},
		activityRegisterOptions(activities.InvokeModelActivity),
	)

	env.ExecuteWorkflow(testInvokeModelLocalWorkflow)
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
	if result != "local ok" {
		t.Fatalf("result = %q", result)
	}
	if localStarts != 1 {
		t.Fatalf("local model starts = %d, want 1", localStarts)
	}
	if regularStarts != 0 {
		t.Fatalf("regular model starts = %d, want 0", regularStarts)
	}
}

func TestInvokeActivityOptionsDefaultSummaries(t *testing.T) {
	if got := languageModelActivityOptions(ActivityOptions{}).Summary; got != activities.InvokeModelActivity {
		t.Fatalf("language model summary = %q", got)
	}
	if got := streamModelActivityOptions(ActivityOptions{}).Summary; got != activities.InvokeModelStreamActivity {
		t.Fatalf("stream model summary = %q", got)
	}
	if got := embeddingModelActivityOptions(ActivityOptions{}).Summary; got != activities.InvokeEmbeddingModelActivity {
		t.Fatalf("embedding model summary = %q", got)
	}
	if got := toolActivityOptions(ActivityOptions{}).Summary; got != activities.InvokeToolActivity {
		t.Fatalf("tool summary = %q", got)
	}
	if got := streamActivityOptions(ActivityOptions{}).Summary; got != activities.PublishToolLifecycleEventActivity {
		t.Fatalf("stream summary = %q", got)
	}
	if got := localLanguageModelActivityOptions(ActivityOptions{}).Summary; got != activities.InvokeModelActivity {
		t.Fatalf("local language model summary = %q", got)
	}
	if got := localEmbeddingModelActivityOptions(ActivityOptions{}).Summary; got != activities.InvokeEmbeddingModelActivity {
		t.Fatalf("local embedding model summary = %q", got)
	}
	if got := localToolActivityOptions(ActivityOptions{}).Summary; got != activities.InvokeToolActivity {
		t.Fatalf("local tool summary = %q", got)
	}
}

func testInvokeModelWorkflow(ctx workflow.Context) (string, error) {
	result, err := InvokeModel(ctx, "model-1", ai.LanguageModelCallOptions{})
	if err != nil {
		return "", err
	}
	return ai.TextFromParts(result.Content), nil
}

func testInvokeModelLocalWorkflow(ctx workflow.Context) (string, error) {
	result, err := InvokeModel(ctx, "model-1", ai.LanguageModelCallOptions{}, ActivityOptions{
		LanguageModelBoundary: activities.ToolExecutionBoundaryLocalActivity,
		LocalLanguageModel: workflow.LocalActivityOptions{
			StartToCloseTimeout: time.Second,
		},
	})
	if err != nil {
		return "", err
	}
	return ai.TextFromParts(result.Content), nil
}

func activityRegisterOptions(name string) activity.RegisterOptions {
	return activity.RegisterOptions{Name: name}
}

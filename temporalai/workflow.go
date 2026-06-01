package temporalai

import (
	"time"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	"go.temporal.io/sdk/workflow"
)

type ActivityOptions struct {
	Default                workflow.ActivityOptions
	LanguageModel          workflow.ActivityOptions
	EmbeddingModel         workflow.ActivityOptions
	Tool                   workflow.ActivityOptions
	LocalLanguageModel     workflow.LocalActivityOptions
	LocalEmbeddingModel    workflow.LocalActivityOptions
	LocalTool              workflow.LocalActivityOptions
	Stream                 workflow.ActivityOptions
	LanguageModelBoundary  activities.ToolExecutionBoundary
	EmbeddingModelBoundary activities.ToolExecutionBoundary
}

func defaultActivityOptions(summary string) workflow.ActivityOptions {
	return workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, Summary: summary}
}

func languageModelActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(activities.InvokeModelActivity), mergeActivityOptions(options.Default, options.LanguageModel))
}

func generateObjectActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(activities.GenerateObjectActivity), mergeActivityOptions(options.Default, options.LanguageModel))
}

func embeddingModelActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(activities.InvokeEmbeddingModelActivity), mergeActivityOptions(options.Default, options.EmbeddingModel))
}

func toolActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(activities.InvokeToolActivity), mergeActivityOptions(options.Default, options.Tool))
}

func streamModelActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(activities.InvokeModelStreamActivity), mergeActivityOptions(options.Default, options.LanguageModel))
}

func defaultLocalLanguageModelActivityOptions() workflow.LocalActivityOptions {
	return workflow.LocalActivityOptions{StartToCloseTimeout: 10 * time.Minute, Summary: activities.InvokeModelActivity}
}

func localLanguageModelActivityOptions(options ActivityOptions) workflow.LocalActivityOptions {
	return mergeLocalActivityOptions(defaultLocalLanguageModelActivityOptions(), options.LocalLanguageModel)
}

func defaultLocalGenerateObjectActivityOptions() workflow.LocalActivityOptions {
	return workflow.LocalActivityOptions{StartToCloseTimeout: 10 * time.Minute, Summary: activities.GenerateObjectActivity}
}

func localGenerateObjectActivityOptions(options ActivityOptions) workflow.LocalActivityOptions {
	return mergeLocalActivityOptions(defaultLocalGenerateObjectActivityOptions(), options.LocalLanguageModel)
}

func defaultLocalEmbeddingModelActivityOptions() workflow.LocalActivityOptions {
	return workflow.LocalActivityOptions{StartToCloseTimeout: 10 * time.Minute, Summary: activities.InvokeEmbeddingModelActivity}
}

func localEmbeddingModelActivityOptions(options ActivityOptions) workflow.LocalActivityOptions {
	return mergeLocalActivityOptions(defaultLocalEmbeddingModelActivityOptions(), options.LocalEmbeddingModel)
}

func defaultLocalToolActivityOptions() workflow.LocalActivityOptions {
	return workflow.LocalActivityOptions{StartToCloseTimeout: 10 * time.Second, Summary: activities.InvokeToolActivity}
}

func localToolActivityOptions(options ActivityOptions) workflow.LocalActivityOptions {
	return mergeLocalActivityOptions(defaultLocalToolActivityOptions(), options.LocalTool)
}

func streamActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(activities.PublishToolLifecycleEventActivity), mergeActivityOptions(options.Default, options.Stream))
}

func InvokeModel(ctx workflow.Context, modelID string, options ai.LanguageModelCallOptions, activityOptions ...ActivityOptions) (*ai.LanguageModelGenerateResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	var wireResult activities.InvokeModelResult
	args := activities.InvokeModelArgs{
		ModelID: modelID,
		Options: activities.LanguageModelCallOptionsFromAI(options),
	}
	var err error
	if ao.LanguageModelBoundary == activities.ToolExecutionBoundaryLocalActivity {
		ctx = workflow.WithLocalActivityOptions(ctx, localLanguageModelActivityOptions(ao))
		err = workflow.ExecuteLocalActivity(ctx, activities.InvokeModelActivity, args).Get(ctx, &wireResult)
	} else {
		ctx = workflow.WithActivityOptions(ctx, languageModelActivityOptions(ao))
		err = workflow.ExecuteActivity(ctx, activities.InvokeModelActivity, args).Get(ctx, &wireResult)
	}
	if err != nil {
		return nil, err
	}
	result := wireResult.ToAI()
	return &result, nil
}

func GenerateObject(ctx workflow.Context, modelID string, options ai.GenerateObjectOptions, activityOptions ...ActivityOptions) (*ai.GenerateObjectResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	var wireResult activities.GenerateObjectResult
	args := activities.GenerateObjectArgs{
		ModelID: modelID,
		Options: activities.GenerateObjectOptionsFromAI(options),
	}
	var err error
	if ao.LanguageModelBoundary == activities.ToolExecutionBoundaryLocalActivity {
		ctx = workflow.WithLocalActivityOptions(ctx, localGenerateObjectActivityOptions(ao))
		err = workflow.ExecuteLocalActivity(ctx, activities.GenerateObjectActivity, args).Get(ctx, &wireResult)
	} else {
		ctx = workflow.WithActivityOptions(ctx, generateObjectActivityOptions(ao))
		err = workflow.ExecuteActivity(ctx, activities.GenerateObjectActivity, args).Get(ctx, &wireResult)
	}
	if err != nil {
		return nil, err
	}
	result := wireResult.ToAI()
	return &result, nil
}

func InvokeModelStream(ctx workflow.Context, modelID string, options ai.LanguageModelCallOptions, activityOptions ...ActivityOptions) (*activities.InvokeModelStreamAIResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	ctx = workflow.WithActivityOptions(ctx, streamModelActivityOptions(ao))
	var wireResult activities.InvokeModelStreamResult
	err := workflow.ExecuteActivity(ctx, activities.InvokeModelStreamActivity, activities.InvokeModelStreamArgs{
		ModelID: modelID,
		Options: activities.LanguageModelCallOptionsFromAI(options),
	}).Get(ctx, &wireResult)
	if err != nil {
		return nil, err
	}
	result := wireResult.ToAI()
	return &result, nil
}

func InvokeEmbeddingModel(ctx workflow.Context, modelID string, options ai.EmbeddingModelCallOptions, activityOptions ...ActivityOptions) (*ai.EmbeddingModelResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	var result ai.EmbeddingModelResult
	args := activities.InvokeEmbeddingModelArgs{
		ModelID:         modelID,
		Values:          options.Values,
		ProviderOptions: options.ProviderOptions,
		Headers:         options.Headers,
	}
	var err error
	if ao.EmbeddingModelBoundary == activities.ToolExecutionBoundaryLocalActivity {
		ctx = workflow.WithLocalActivityOptions(ctx, localEmbeddingModelActivityOptions(ao))
		err = workflow.ExecuteLocalActivity(ctx, activities.InvokeEmbeddingModelActivity, args).Get(ctx, &result)
	} else {
		ctx = workflow.WithActivityOptions(ctx, embeddingModelActivityOptions(ao))
		err = workflow.ExecuteActivity(ctx, activities.InvokeEmbeddingModelActivity, args).Get(ctx, &result)
	}
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func InvokeTool(ctx workflow.Context, args activities.InvokeToolArgs, activityOptions ...ActivityOptions) (*activities.InvokeToolResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	ctx = workflow.WithActivityOptions(ctx, toolActivityOptions(ao))
	var result activities.InvokeToolResult
	err := workflow.ExecuteActivity(ctx, activities.InvokeToolActivity, args).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func InvokeToolLocal(ctx workflow.Context, args activities.InvokeToolArgs, activityOptions ...ActivityOptions) (*activities.InvokeToolResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	ctx = workflow.WithLocalActivityOptions(ctx, localToolActivityOptions(ao))
	var result activities.InvokeToolResult
	err := workflow.ExecuteLocalActivity(ctx, activities.InvokeToolActivity, args).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func PublishToolLifecycleEvent(ctx workflow.Context, input streaming.ToolLifecycleInput, activityOptions ...ActivityOptions) error {
	if input.StreamID == "" {
		return nil
	}
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	ctx = workflow.WithActivityOptions(ctx, streamActivityOptions(ao))
	return workflow.ExecuteActivity(ctx, activities.PublishToolLifecycleEventActivity, activities.PublishToolLifecycleEventArgs(input)).Get(ctx, nil)
}

func mergeActivityOptions(base, override workflow.ActivityOptions) workflow.ActivityOptions {
	out := base
	if override.TaskQueue != "" {
		out.TaskQueue = override.TaskQueue
	}
	if override.ScheduleToCloseTimeout != 0 {
		out.ScheduleToCloseTimeout = override.ScheduleToCloseTimeout
	}
	if override.ScheduleToStartTimeout != 0 {
		out.ScheduleToStartTimeout = override.ScheduleToStartTimeout
	}
	if override.StartToCloseTimeout != 0 {
		out.StartToCloseTimeout = override.StartToCloseTimeout
	}
	if override.HeartbeatTimeout != 0 {
		out.HeartbeatTimeout = override.HeartbeatTimeout
	}
	if override.WaitForCancellation {
		out.WaitForCancellation = true
	}
	if override.ActivityID != "" {
		out.ActivityID = override.ActivityID
	}
	if override.RetryPolicy != nil {
		out.RetryPolicy = override.RetryPolicy
	}
	if override.Summary != "" {
		out.Summary = override.Summary
	}
	return out
}

func mergeLocalActivityOptions(base, override workflow.LocalActivityOptions) workflow.LocalActivityOptions {
	out := base
	if override.ScheduleToCloseTimeout != 0 {
		out.ScheduleToCloseTimeout = override.ScheduleToCloseTimeout
	}
	if override.StartToCloseTimeout != 0 {
		out.StartToCloseTimeout = override.StartToCloseTimeout
	}
	if override.RetryPolicy != nil {
		out.RetryPolicy = override.RetryPolicy
	}
	if override.Summary != "" {
		out.Summary = override.Summary
	}
	return out
}

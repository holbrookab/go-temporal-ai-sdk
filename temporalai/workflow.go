package temporalai

import (
	"time"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"go.temporal.io/sdk/workflow"
)

type ActivityOptions struct {
	Default        workflow.ActivityOptions
	LanguageModel  workflow.ActivityOptions
	EmbeddingModel workflow.ActivityOptions
}

func defaultActivityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute}
}

func languageModelActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(), mergeActivityOptions(options.Default, options.LanguageModel))
}

func embeddingModelActivityOptions(options ActivityOptions) workflow.ActivityOptions {
	return mergeActivityOptions(defaultActivityOptions(), mergeActivityOptions(options.Default, options.EmbeddingModel))
}

func InvokeModel(ctx workflow.Context, modelID string, options ai.LanguageModelCallOptions, activityOptions ...ActivityOptions) (*ai.LanguageModelGenerateResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	ctx = workflow.WithActivityOptions(ctx, languageModelActivityOptions(ao))
	var wireResult activities.InvokeModelResult
	err := workflow.ExecuteActivity(ctx, activities.InvokeModelActivity, activities.InvokeModelArgs{
		ModelID: modelID,
		Options: activities.LanguageModelCallOptionsFromAI(options),
	}).Get(ctx, &wireResult)
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
	ctx = workflow.WithActivityOptions(ctx, languageModelActivityOptions(ao))
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
	ctx = workflow.WithActivityOptions(ctx, embeddingModelActivityOptions(ao))
	var result ai.EmbeddingModelResult
	err := workflow.ExecuteActivity(ctx, activities.InvokeEmbeddingModelActivity, activities.InvokeEmbeddingModelArgs{
		ModelID:         modelID,
		Values:          options.Values,
		ProviderOptions: options.ProviderOptions,
		Headers:         options.Headers,
	}).Get(ctx, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
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
	return out
}

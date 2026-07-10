package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	"go.temporal.io/sdk/activity"
)

type Options struct {
	ModelProvider   ai.Provider
	StreamConnector streaming.Connector
	Tools           map[string]ai.Tool
	ArtifactStore   ToolArtifactStore
	Sandbox         ai.Sandbox
}

type Activities struct {
	provider  ai.Provider
	connector streaming.Connector
	tools     map[string]ai.Tool
	artifacts ToolArtifactStore
	sandbox   ai.Sandbox
}

func New(opts Options) *Activities {
	connector := opts.StreamConnector
	if connector == nil {
		connector = streaming.NoopConnector{}
	}
	return &Activities{
		provider:  opts.ModelProvider,
		connector: connector,
		tools:     opts.Tools,
		artifacts: opts.ArtifactStore,
		sandbox:   opts.Sandbox,
	}
}

func (a *Activities) InvokeModel(ctx context.Context, args InvokeModelArgs) (*InvokeModelResult, error) {
	model, err := a.languageModel(args.ModelID)
	if err != nil {
		return nil, err
	}
	options, streamOptions := extractStreamOptions(args.Options.ToAI())
	relay := streaming.NewRelay(a.connector, withActivityAttempt(ctx, streamOptions))
	if err := relay.Accept(ctx, ai.StreamPart{Type: "stream-start"}); err != nil {
		return nil, err
	}
	result, err := model.DoGenerate(ctx, options)
	if err != nil {
		failRelay(ctx, relay, err)
		return nil, err
	}
	if result == nil {
		err := errors.New("model returned nil generate result")
		failRelay(ctx, relay, err)
		return nil, err
	}
	if err := relayGenerateResult(ctx, relay, result); err != nil {
		_ = relay.Discard(ctx, err.Error())
		return nil, err
	}
	if err := relay.Commit(ctx); err != nil {
		return nil, err
	}
	return (*InvokeModelResult)(GenerateResultFromAI(result)), nil
}

func (a *Activities) GenerateObject(ctx context.Context, args GenerateObjectArgs) (*GenerateObjectResult, error) {
	model, err := a.languageModel(args.ModelID)
	if err != nil {
		return nil, err
	}
	options, streamOptions := extractGenerateObjectStreamOptions(args.Options.ToAI(model))
	if streamOptions.Lane == "" {
		streamOptions.Lane = streaming.LaneObject
	}
	relay := streaming.NewRelay(a.connector, withActivityAttempt(ctx, streamOptions))
	if err := relay.Accept(ctx, ai.StreamPart{Type: "stream-start"}); err != nil {
		return nil, err
	}
	result, err := ai.GenerateObject(ctx, options)
	if err != nil {
		failRelay(ctx, relay, err)
		return nil, err
	}
	if result == nil {
		err := errors.New("model returned nil object result")
		failRelay(ctx, relay, err)
		return nil, err
	}
	if err := relayGenerateObjectResult(ctx, relay, result); err != nil {
		_ = relay.Discard(ctx, err.Error())
		return nil, err
	}
	if err := relay.Commit(ctx); err != nil {
		return nil, err
	}
	return GenerateObjectResultFromAI(result), nil
}

func (a *Activities) StreamObject(ctx context.Context, args StreamObjectArgs) (*StreamObjectResult, error) {
	model, err := a.languageModel(args.ModelID)
	if err != nil {
		return nil, err
	}
	options, streamOptions := extractStreamObjectStreamOptions(args.Options.ToAI(model))
	if streamOptions.Lane == "" {
		streamOptions.Lane = streaming.LaneObject
	}
	relay := streaming.NewRelay(a.connector, withActivityAttempt(ctx, streamOptions))
	if err := relay.Accept(ctx, ai.StreamPart{Type: "stream-start"}); err != nil {
		return nil, err
	}
	streamResult, err := ai.StreamObject(ctx, options)
	if err != nil {
		failRelay(ctx, relay, err)
		return nil, err
	}
	if streamResult == nil {
		err := errors.New("model returned nil object stream result")
		failRelay(ctx, relay, err)
		return nil, err
	}
	if streamResult.Stream == nil {
		err := errors.New("model returned nil object stream")
		failRelay(ctx, relay, err)
		return nil, err
	}
	return a.consumeObjectStream(ctx, relay, streamResult)
}

func (a *Activities) InvokeEmbeddingModel(ctx context.Context, args InvokeEmbeddingModelArgs) (*InvokeEmbeddingModelResult, error) {
	provider, ok := a.provider.(ai.EmbeddingProvider)
	if !ok {
		return nil, errors.New("provider does not support embeddings")
	}
	model := provider.EmbeddingModel(args.ModelID)
	if model == nil {
		return nil, fmt.Errorf("embedding model %q not found", args.ModelID)
	}
	result, err := model.DoEmbed(ctx, ai.EmbeddingModelCallOptions{
		Values:          args.Values,
		ProviderOptions: args.ProviderOptions,
		Headers:         args.Headers,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("model returned nil embedding result")
	}
	return (*InvokeEmbeddingModelResult)(result), nil
}

func (a *Activities) InvokeModelStream(ctx context.Context, args InvokeModelStreamArgs) (*InvokeModelStreamResult, error) {
	model, err := a.languageModel(args.ModelID)
	if err != nil {
		return nil, err
	}
	options, streamOptions := extractStreamOptions(args.Options.ToAI())
	streamResult, err := model.DoStream(ctx, options)
	if err != nil {
		return nil, err
	}
	if streamResult == nil {
		return nil, errors.New("model returned nil stream result")
	}
	if streamResult.Stream == nil {
		return nil, errors.New("model returned nil stream")
	}

	relay := streaming.NewRelay(a.connector, withActivityAttempt(ctx, streamOptions))
	outputTracker := newPartialOutputTracker(options.ResponseFormat)
	parts := []ai.StreamPart{}
	outputSeen := false
	for {
		select {
		case <-ctx.Done():
			reason := ctx.Err().Error()
			_ = relay.Cancel(context.Background(), reason)
			return nil, ctx.Err()
		case part, ok := <-streamResult.Stream:
			if !ok {
				if !outputSeen {
					err := ai.NewNoOutputGeneratedError("Model stream ended without producing output.", nil)
					_ = relay.Discard(ctx, err.Error())
					return nil, err
				}
				if err := relay.Commit(ctx); err != nil {
					return nil, err
				}
				result := GenerateResultFromAIStreamParts(parts, streamResult.Request, streamResult.Response)
				return &InvokeModelStreamResult{
					Result: result,
				}, nil
			}
			part, extraParts := outputTracker.enrich(part)
			if isGeneratedOutputPart(part) {
				outputSeen = true
			}
			if isReturnedStreamPart(part) {
				parts = append(parts, part)
			}
			if part.Type == "error" {
				reason := "provider stream error"
				if part.Err != nil {
					reason = part.Err.Error()
				}
				_ = relay.Discard(ctx, reason)
				if part.Err != nil {
					return nil, part.Err
				}
				return nil, errors.New(reason)
			}
			if err := relay.Accept(ctx, part); err != nil {
				_ = relay.Discard(ctx, err.Error())
				return nil, err
			}
			for _, extra := range extraParts {
				if err := relay.Accept(ctx, extra); err != nil {
					_ = relay.Discard(ctx, err.Error())
					return nil, err
				}
			}
		}
	}
}

func isReturnedStreamPart(part ai.StreamPart) bool {
	switch part.Type {
	case "text-delta",
		"reasoning-delta",
		"tool-input-delta",
		"tool-input-end",
		"tool-call",
		"tool-approval-request",
		"tool-approval-response",
		"file",
		"reasoning-file",
		"source",
		"finish":
		return true
	default:
		return false
	}
}

func isGeneratedOutputPart(part ai.StreamPart) bool {
	switch part.Type {
	case "text-delta":
		return part.TextDelta != ""
	case "reasoning-delta":
		return part.ReasoningDelta != ""
	case "tool-input-delta":
		return part.ToolInputDelta != ""
	case "file", "reasoning-file", "tool-call":
		return true
	default:
		return false
	}
}

func (a *Activities) PublishToolLifecycleEvent(ctx context.Context, args PublishToolLifecycleEventArgs) error {
	if args.StreamID == "" {
		return nil
	}
	return a.connector.PublishToolLifecycleEvent(ctx, args)
}

func (a *Activities) languageModel(modelID string) (ai.LanguageModel, error) {
	if a == nil || a.provider == nil {
		return nil, errors.New("model provider is required")
	}
	model := a.provider.LanguageModel(modelID)
	if model == nil {
		return nil, fmt.Errorf("language model %q not found", modelID)
	}
	return model, nil
}

func withActivityAttempt(ctx context.Context, options streaming.Options) streaming.Options {
	base := options.AttemptID
	if base == "" {
		base = "attempt"
	}
	attempt := activityAttempt(ctx)
	options.AttemptID = fmt.Sprintf("%s:activity-%d", base, attempt)
	return options
}

func activityAttempt(ctx context.Context) (attempt int) {
	attempt = 1
	defer func() {
		if recover() != nil {
			attempt = 1
		}
	}()
	if info := activity.GetInfo(ctx); info.Attempt > 0 {
		return int(info.Attempt)
	}
	return attempt
}

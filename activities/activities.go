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
}

type Activities struct {
	provider  ai.Provider
	connector streaming.Connector
}

func New(opts Options) *Activities {
	connector := opts.StreamConnector
	if connector == nil {
		connector = streaming.NoopConnector{}
	}
	return &Activities{
		provider:  opts.ModelProvider,
		connector: connector,
	}
}

func (a *Activities) InvokeModel(ctx context.Context, args InvokeModelArgs) (*InvokeModelResult, error) {
	model, err := a.languageModel(args.ModelID)
	if err != nil {
		return nil, err
	}
	result, err := model.DoGenerate(ctx, args.Options.ToAI())
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("model returned nil generate result")
	}
	return (*InvokeModelResult)(GenerateResultFromAI(result)), nil
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
	parts := []StreamPart{}
	for {
		select {
		case <-ctx.Done():
			reason := ctx.Err().Error()
			_ = relay.Cancel(context.Background(), reason)
			return nil, ctx.Err()
		case part, ok := <-streamResult.Stream:
			if !ok {
				if err := relay.Commit(ctx); err != nil {
					return nil, err
				}
				return &InvokeModelStreamResult{
					StreamParts: parts,
					Request:     streamResult.Request,
					Response:    ResponseMetadataFromAI(streamResult.Response),
				}, nil
			}
			parts = append(parts, StreamPartFromAI(part))
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
		}
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

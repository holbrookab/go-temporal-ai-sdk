package activities

import (
	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

const (
	InvokeModelActivity               = "go-temporal-ai-sdk.InvokeModel"
	InvokeModelStreamActivity         = "go-temporal-ai-sdk.InvokeModelStream"
	InvokeEmbeddingModelActivity      = "go-temporal-ai-sdk.InvokeEmbeddingModel"
	PublishToolLifecycleEventActivity = "go-temporal-ai-sdk.PublishToolLifecycleEvent"
)

type InvokeModelArgs struct {
	ModelID string                   `json:"modelId"`
	Options LanguageModelCallOptions `json:"options"`
}

type InvokeModelResult = LanguageModelGenerateResult

type InvokeModelStreamArgs struct {
	ModelID string                   `json:"modelId"`
	Options LanguageModelCallOptions `json:"options"`
}

type InvokeModelStreamResult struct {
	StreamParts []StreamPart       `json:"streamParts"`
	Request     ai.RequestMetadata `json:"request"`
	Response    ResponseMetadata   `json:"response"`
}

type InvokeEmbeddingModelArgs struct {
	ModelID         string             `json:"modelId"`
	Values          []string           `json:"values"`
	ProviderOptions ai.ProviderOptions `json:"providerOptions,omitempty"`
	Headers         map[string]string  `json:"headers,omitempty"`
}

type InvokeEmbeddingModelResult = ai.EmbeddingModelResult

type PublishToolLifecycleEventArgs = streaming.ToolLifecycleInput

package activities

import (
	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

const (
	InvokeModelActivity               = "go-temporal-ai-sdk.InvokeModel"
	InvokeModelStreamActivity         = "go-temporal-ai-sdk.InvokeModelStream"
	InvokeEmbeddingModelActivity      = "go-temporal-ai-sdk.InvokeEmbeddingModel"
	InvokeToolActivity                = "go-temporal-ai-sdk.InvokeTool"
	PublishToolLifecycleEventActivity = "go-temporal-ai-sdk.PublishToolLifecycleEvent"
)

type ToolExecutionBoundary string

const (
	ToolExecutionBoundaryAuto          ToolExecutionBoundary = "auto"
	ToolExecutionBoundaryActivity      ToolExecutionBoundary = "activity"
	ToolExecutionBoundaryLocalActivity ToolExecutionBoundary = "local-activity"
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
	Result      *LanguageModelGenerateResult `json:"result,omitempty"`
	StreamParts []StreamPart                 `json:"streamParts,omitempty"`
	Request     *ai.RequestMetadata          `json:"request,omitempty"`
	Response    *ResponseMetadata            `json:"response,omitempty"`
}

type InvokeEmbeddingModelArgs struct {
	ModelID         string             `json:"modelId"`
	Values          []string           `json:"values"`
	ProviderOptions ai.ProviderOptions `json:"providerOptions,omitempty"`
	Headers         map[string]string  `json:"headers,omitempty"`
}

type InvokeEmbeddingModelResult = ai.EmbeddingModelResult

type ToolDefinition struct {
	Name              string                `json:"name"`
	Title             string                `json:"title,omitempty"`
	Description       string                `json:"description,omitempty"`
	InputSchema       any                   `json:"inputSchema,omitempty"`
	OutputSchema      any                   `json:"outputSchema,omitempty"`
	InputExamples     []any                 `json:"inputExamples,omitempty"`
	Strict            *bool                 `json:"strict,omitempty"`
	ProviderOptions   ai.ProviderOptions    `json:"providerOptions,omitempty"`
	ProviderMetadata  ai.ProviderMetadata   `json:"providerMetadata,omitempty"`
	Type              string                `json:"type,omitempty"`
	ID                string                `json:"id,omitempty"`
	Args              any                   `json:"args,omitempty"`
	ExecutionBoundary ToolExecutionBoundary `json:"executionBoundary,omitempty"`
}

type InvokeToolArgs struct {
	ToolCallID string               `json:"toolCallId"`
	ToolName   string               `json:"toolName"`
	Input      any                  `json:"input,omitempty"`
	Messages   []Message            `json:"messages,omitempty"`
	Context    any                  `json:"context,omitempty"`
	Lifecycle  ToolLifecycleOptions `json:"lifecycle,omitempty"`
}

type ToolLifecycleOptions struct {
	StreamID        string         `json:"streamId,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	DurableRequired bool           `json:"durableRequired,omitempty"`
}

type InvokeToolResult struct {
	ToolCallID       string              `json:"toolCallId"`
	ToolName         string              `json:"toolName"`
	Input            any                 `json:"input,omitempty"`
	Output           ai.ToolResultOutput `json:"output"`
	Result           any                 `json:"result,omitempty"`
	IsError          bool                `json:"isError,omitempty"`
	Dynamic          bool                `json:"dynamic,omitempty"`
	ProviderExecuted bool                `json:"providerExecuted,omitempty"`
	Preliminary      bool                `json:"preliminary,omitempty"`
	ProviderMetadata ai.ProviderMetadata `json:"providerMetadata,omitempty"`
}

type PublishToolLifecycleEventArgs = streaming.ToolLifecycleInput

package activities

import (
	"time"

	"github.com/holbrookab/go-ai/packages/ai"
)

type LanguageModelCallOptions struct {
	Prompt           []Message          `json:"prompt,omitempty"`
	MaxOutputTokens  *int               `json:"maxOutputTokens,omitempty"`
	Temperature      *float64           `json:"temperature,omitempty"`
	TopP             *float64           `json:"topP,omitempty"`
	TopK             *float64           `json:"topK,omitempty"`
	PresencePenalty  *float64           `json:"presencePenalty,omitempty"`
	FrequencyPenalty *float64           `json:"frequencyPenalty,omitempty"`
	StopSequences    []string           `json:"stopSequences,omitempty"`
	Seed             *int               `json:"seed,omitempty"`
	Reasoning        string             `json:"reasoning,omitempty"`
	ResponseFormat   *ai.ResponseFormat `json:"responseFormat,omitempty"`
	Tools            []ai.ModelTool     `json:"tools,omitempty"`
	ToolChoice       ai.ToolChoice      `json:"toolChoice,omitempty"`
	ProviderOptions  ai.ProviderOptions `json:"providerOptions,omitempty"`
	Headers          map[string]string  `json:"headers,omitempty"`
}

type Message struct {
	Role            ai.Role            `json:"role"`
	Content         []Part             `json:"content,omitempty"`
	Text            string             `json:"text,omitempty"`
	ProviderOptions ai.ProviderOptions `json:"providerOptions,omitempty"`
}

type Part struct {
	Type string `json:"type"`

	Text             string              `json:"text,omitempty"`
	Data             ai.FileData         `json:"data,omitempty"`
	MediaType        string              `json:"mediaType,omitempty"`
	Filename         string              `json:"filename,omitempty"`
	ToolCallID       string              `json:"toolCallId,omitempty"`
	ToolName         string              `json:"toolName,omitempty"`
	Input            any                 `json:"input,omitempty"`
	InputRaw         string              `json:"inputRaw,omitempty"`
	Output           ai.ToolResultOutput `json:"output,omitempty"`
	Result           any                 `json:"result,omitempty"`
	IsError          bool                `json:"isError,omitempty"`
	ProviderExecuted bool                `json:"providerExecuted,omitempty"`
	Dynamic          bool                `json:"dynamic,omitempty"`
	Invalid          bool                `json:"invalid,omitempty"`
	ErrorText        string              `json:"errorText,omitempty"`
	Preliminary      bool                `json:"preliminary,omitempty"`
	Title            string              `json:"title,omitempty"`
	ID               string              `json:"id,omitempty"`
	SourceType       string              `json:"sourceType,omitempty"`
	URL              string              `json:"url,omitempty"`

	ProviderMetadata ai.ProviderMetadata `json:"providerMetadata,omitempty"`
	ProviderOptions  ai.ProviderOptions  `json:"providerOptions,omitempty"`
}

type LanguageModelGenerateResult struct {
	Content          []Part              `json:"content,omitempty"`
	FinishReason     ai.FinishReason     `json:"finishReason,omitempty"`
	Usage            ai.Usage            `json:"usage,omitempty"`
	Warnings         []ai.Warning        `json:"warnings,omitempty"`
	ProviderMetadata ai.ProviderMetadata `json:"providerMetadata,omitempty"`
	Request          RequestMetadata     `json:"request,omitempty"`
	Response         ResponseMetadata    `json:"response,omitempty"`
}

type RequestMetadata = ai.RequestMetadata

type ResponseMetadata struct {
	ID         string            `json:"id,omitempty"`
	Timestamp  string            `json:"timestamp,omitempty"`
	ModelID    string            `json:"modelId,omitempty"`
	StatusCode int               `json:"statusCode,omitempty"`
	StatusText string            `json:"statusText,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       any               `json:"body,omitempty"`
	Messages   []Message         `json:"messages,omitempty"`
}

type StreamPart struct {
	Type             string              `json:"type"`
	ID               string              `json:"id,omitempty"`
	TextDelta        string              `json:"textDelta,omitempty"`
	PartialOutput    any                 `json:"partialOutput,omitempty"`
	Element          any                 `json:"element,omitempty"`
	ReasoningDelta   string              `json:"reasoningDelta,omitempty"`
	ToolCallID       string              `json:"toolCallId,omitempty"`
	ToolName         string              `json:"toolName,omitempty"`
	ToolInputDelta   string              `json:"toolInputDelta,omitempty"`
	ToolInput        string              `json:"toolInput,omitempty"`
	Content          *Part               `json:"content,omitempty"`
	FinishReason     ai.FinishReason     `json:"finishReason,omitempty"`
	Usage            ai.Usage            `json:"usage,omitempty"`
	Warnings         []ai.Warning        `json:"warnings,omitempty"`
	Request          RequestMetadata     `json:"request,omitempty"`
	Response         ResponseMetadata    `json:"response,omitempty"`
	ProviderMetadata ai.ProviderMetadata `json:"providerMetadata,omitempty"`
	Raw              any                 `json:"raw,omitempty"`
	AbortReason      string              `json:"abortReason,omitempty"`
	ErrorText        string              `json:"errorText,omitempty"`
}

func ResponseMetadataFromAI(metadata ai.ResponseMetadata) ResponseMetadata {
	timestamp := ""
	if !metadata.Timestamp.IsZero() {
		timestamp = metadata.Timestamp.Format(time.RFC3339Nano)
	}
	return ResponseMetadata{
		ID:         metadata.ID,
		Timestamp:  timestamp,
		ModelID:    metadata.ModelID,
		StatusCode: metadata.StatusCode,
		StatusText: metadata.StatusText,
		Headers:    metadata.Headers,
		Body:       metadata.Body,
		Messages:   MessagesFromAI(metadata.Messages),
	}
}

func (metadata ResponseMetadata) ToAI() ai.ResponseMetadata {
	var timestamp time.Time
	if metadata.Timestamp != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, metadata.Timestamp); err == nil {
			timestamp = parsed
		}
	}
	return ai.ResponseMetadata{
		ID:         metadata.ID,
		Timestamp:  timestamp,
		ModelID:    metadata.ModelID,
		StatusCode: metadata.StatusCode,
		StatusText: metadata.StatusText,
		Headers:    metadata.Headers,
		Body:       metadata.Body,
		Messages:   MessagesToAI(metadata.Messages),
	}
}

func LanguageModelCallOptionsFromAI(options ai.LanguageModelCallOptions) LanguageModelCallOptions {
	return LanguageModelCallOptions{
		Prompt:           MessagesFromAI(options.Prompt),
		MaxOutputTokens:  options.MaxOutputTokens,
		Temperature:      options.Temperature,
		TopP:             options.TopP,
		TopK:             options.TopK,
		PresencePenalty:  options.PresencePenalty,
		FrequencyPenalty: options.FrequencyPenalty,
		StopSequences:    options.StopSequences,
		Seed:             options.Seed,
		Reasoning:        options.Reasoning,
		ResponseFormat:   options.ResponseFormat,
		Tools:            options.Tools,
		ToolChoice:       options.ToolChoice,
		ProviderOptions:  options.ProviderOptions,
		Headers:          options.Headers,
	}
}

func (options LanguageModelCallOptions) ToAI() ai.LanguageModelCallOptions {
	return ai.LanguageModelCallOptions{
		Prompt:           MessagesToAI(options.Prompt),
		MaxOutputTokens:  options.MaxOutputTokens,
		Temperature:      options.Temperature,
		TopP:             options.TopP,
		TopK:             options.TopK,
		PresencePenalty:  options.PresencePenalty,
		FrequencyPenalty: options.FrequencyPenalty,
		StopSequences:    options.StopSequences,
		Seed:             options.Seed,
		Reasoning:        options.Reasoning,
		ResponseFormat:   options.ResponseFormat,
		Tools:            options.Tools,
		ToolChoice:       options.ToolChoice,
		ProviderOptions:  options.ProviderOptions,
		Headers:          options.Headers,
	}
}

func MessagesFromAI(messages []ai.Message) []Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, Message{
			Role:            message.Role,
			Content:         PartsFromAI(message.Content),
			Text:            message.Text,
			ProviderOptions: message.ProviderOptions,
		})
	}
	return out
}

func MessagesToAI(messages []Message) []ai.Message {
	if len(messages) == 0 {
		return nil
	}
	out := make([]ai.Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, ai.Message{
			Role:            message.Role,
			Content:         PartsToAI(message.Content),
			Text:            message.Text,
			ProviderOptions: message.ProviderOptions,
		})
	}
	return out
}

func PartsFromAI(parts []ai.Part) []Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]Part, 0, len(parts))
	for _, part := range parts {
		out = append(out, PartFromAI(part))
	}
	return out
}

func PartsToAI(parts []Part) []ai.Part {
	if len(parts) == 0 {
		return nil
	}
	out := make([]ai.Part, 0, len(parts))
	for _, part := range parts {
		if converted := part.ToAI(); converted != nil {
			out = append(out, converted)
		}
	}
	return out
}

func PartFromAI(part ai.Part) Part {
	switch part := part.(type) {
	case ai.TextPart:
		return Part{Type: "text", Text: part.Text, ProviderOptions: part.ProviderOptions}
	case ai.FilePart:
		return Part{Type: "file", Data: part.Data, MediaType: part.MediaType, Filename: part.Filename, ProviderOptions: part.ProviderOptions}
	case ai.ReasoningPart:
		return Part{Type: "reasoning", Text: part.Text, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case ai.ReasoningFilePart:
		return Part{Type: "reasoning-file", Data: part.Data, MediaType: part.MediaType, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case ai.ToolCallPart:
		errorText := ""
		if part.Error != nil {
			errorText = part.Error.Error()
		}
		return Part{Type: "tool-call", ToolCallID: part.ToolCallID, ToolName: part.ToolName, Input: part.Input, InputRaw: part.InputRaw, ProviderExecuted: part.ProviderExecuted, Dynamic: part.Dynamic, Invalid: part.Invalid, ErrorText: errorText, Title: part.Title, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case ai.ToolResultPart:
		return Part{Type: "tool-result", ToolCallID: part.ToolCallID, ToolName: part.ToolName, Input: part.Input, Output: part.Output, Result: part.Result, IsError: part.IsError, ProviderExecuted: part.ProviderExecuted, Dynamic: part.Dynamic, Preliminary: part.Preliminary, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case ai.SourcePart:
		return Part{Type: "source", ID: part.ID, SourceType: part.SourceType, URL: part.URL, Title: part.Title}
	default:
		return Part{Type: part.PartType()}
	}
}

func (part Part) ToAI() ai.Part {
	switch part.Type {
	case "text":
		return ai.TextPart{Text: part.Text, ProviderOptions: part.ProviderOptions}
	case "file":
		return ai.FilePart{Data: part.Data, MediaType: part.MediaType, Filename: part.Filename, ProviderOptions: part.ProviderOptions}
	case "reasoning":
		return ai.ReasoningPart{Text: part.Text, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case "reasoning-file":
		return ai.ReasoningFilePart{Data: part.Data, MediaType: part.MediaType, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case "tool-call":
		return ai.ToolCallPart{ToolCallID: part.ToolCallID, ToolName: part.ToolName, Input: part.Input, InputRaw: part.InputRaw, ProviderExecuted: part.ProviderExecuted, Dynamic: part.Dynamic, Invalid: part.Invalid, Title: part.Title, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case "tool-result":
		return ai.ToolResultPart{ToolCallID: part.ToolCallID, ToolName: part.ToolName, Input: part.Input, Output: part.Output, Result: part.Result, IsError: part.IsError, ProviderExecuted: part.ProviderExecuted, Dynamic: part.Dynamic, Preliminary: part.Preliminary, ProviderMetadata: part.ProviderMetadata, ProviderOptions: part.ProviderOptions}
	case "source":
		return ai.SourcePart{ID: part.ID, SourceType: part.SourceType, URL: part.URL, Title: part.Title}
	default:
		return nil
	}
}

func GenerateResultFromAI(result *ai.LanguageModelGenerateResult) *LanguageModelGenerateResult {
	if result == nil {
		return nil
	}
	return &LanguageModelGenerateResult{
		Content:          PartsFromAI(result.Content),
		FinishReason:     result.FinishReason,
		Usage:            result.Usage,
		Warnings:         result.Warnings,
		ProviderMetadata: result.ProviderMetadata,
		Request:          result.Request,
		Response:         ResponseMetadataFromAI(result.Response),
	}
}

func GenerateResultFromAIStreamParts(parts []ai.StreamPart, request ai.RequestMetadata, response ai.ResponseMetadata) *LanguageModelGenerateResult {
	result := &LanguageModelGenerateResult{
		Request:  request,
		Response: ResponseMetadataFromAI(response),
	}
	var text string
	var reasoning string
	toolInputs := map[string]string{}
	for _, part := range parts {
		switch part.Type {
		case "text-delta":
			text += part.TextDelta
		case "reasoning-delta":
			reasoning += part.ReasoningDelta
		case "tool-input-delta":
			toolInputs[part.ToolCallID] += part.ToolInputDelta
		case "tool-input-end":
			if part.ToolInput != "" {
				toolInputs[part.ToolCallID] = part.ToolInput
			}
		case "tool-call":
			input := part.ToolInput
			if input == "" {
				input = toolInputs[part.ToolCallID]
			}
			result.Content = append(result.Content, Part{
				Type:             "tool-call",
				ToolCallID:       part.ToolCallID,
				ToolName:         part.ToolName,
				InputRaw:         input,
				ProviderMetadata: part.ProviderMetadata,
			})
		case "file", "source":
			if part.Content != nil {
				result.Content = append(result.Content, PartFromAI(part.Content))
			}
		case "finish":
			result.FinishReason = part.FinishReason
			result.Usage = part.Usage
			result.Warnings = append(result.Warnings, part.Warnings...)
			result.ProviderMetadata = part.ProviderMetadata
		}
	}
	if text != "" {
		result.Content = append([]Part{{Type: "text", Text: text}}, result.Content...)
	}
	if reasoning != "" {
		result.Content = append([]Part{{Type: "reasoning", Text: reasoning}}, result.Content...)
	}
	return result
}

func (result LanguageModelGenerateResult) ToAI() ai.LanguageModelGenerateResult {
	return ai.LanguageModelGenerateResult{
		Content:          PartsToAI(result.Content),
		FinishReason:     result.FinishReason,
		Usage:            result.Usage,
		Warnings:         result.Warnings,
		ProviderMetadata: result.ProviderMetadata,
		Request:          result.Request,
		Response:         result.Response.ToAI(),
	}
}

func StreamPartFromAI(part ai.StreamPart) StreamPart {
	var content *Part
	if part.Content != nil {
		converted := PartFromAI(part.Content)
		content = &converted
	}
	errorText := ""
	if part.Err != nil {
		errorText = part.Err.Error()
	}
	return StreamPart{
		Type:             part.Type,
		ID:               part.ID,
		TextDelta:        part.TextDelta,
		PartialOutput:    part.PartialOutput,
		Element:          part.Element,
		ReasoningDelta:   part.ReasoningDelta,
		ToolCallID:       part.ToolCallID,
		ToolName:         part.ToolName,
		ToolInputDelta:   part.ToolInputDelta,
		ToolInput:        part.ToolInput,
		Content:          content,
		FinishReason:     part.FinishReason,
		Usage:            part.Usage,
		Warnings:         part.Warnings,
		Request:          part.Request,
		Response:         ResponseMetadataFromAI(part.Response),
		ProviderMetadata: part.ProviderMetadata,
		Raw:              part.Raw,
		AbortReason:      part.AbortReason,
		ErrorText:        errorText,
	}
}

func StreamPartsToAI(parts []StreamPart) []ai.StreamPart {
	if len(parts) == 0 {
		return nil
	}
	out := make([]ai.StreamPart, 0, len(parts))
	for _, part := range parts {
		out = append(out, part.ToAI())
	}
	return out
}

func (part StreamPart) ToAI() ai.StreamPart {
	var content ai.Part
	if part.Content != nil {
		content = part.Content.ToAI()
	}
	return ai.StreamPart{
		Type:             part.Type,
		ID:               part.ID,
		TextDelta:        part.TextDelta,
		PartialOutput:    part.PartialOutput,
		Element:          part.Element,
		ReasoningDelta:   part.ReasoningDelta,
		ToolCallID:       part.ToolCallID,
		ToolName:         part.ToolName,
		ToolInputDelta:   part.ToolInputDelta,
		ToolInput:        part.ToolInput,
		Content:          content,
		FinishReason:     part.FinishReason,
		Usage:            part.Usage,
		Warnings:         part.Warnings,
		Request:          part.Request,
		Response:         part.Response.ToAI(),
		ProviderMetadata: part.ProviderMetadata,
		Raw:              part.Raw,
		AbortReason:      part.AbortReason,
	}
}

func (result InvokeModelStreamResult) ToAI() InvokeModelStreamAIResult {
	out := InvokeModelStreamAIResult{
		StreamParts: StreamPartsToAI(result.StreamParts),
		Request:     result.Request,
		Response:    result.Response.ToAI(),
	}
	if result.Result != nil {
		generated := result.Result.ToAI()
		out.Result = &generated
	}
	return out
}

type InvokeModelStreamAIResult struct {
	StreamParts []ai.StreamPart
	Request     ai.RequestMetadata
	Response    ai.ResponseMetadata
	Result      *ai.LanguageModelGenerateResult
}

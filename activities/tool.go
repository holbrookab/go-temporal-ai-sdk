package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	"go.temporal.io/sdk/activity"
)

func normalizeSchema(schema any) any {
	switch value := schema.(type) {
	case nil:
		return map[string]any{"type": "object", "properties": map[string]any{}}
	case json.RawMessage:
		var out any
		if err := json.Unmarshal(value, &out); err == nil {
			return out
		}
		return value
	default:
		return value
	}
}

func ToolDefinitionFromAI(name string, tool ai.Tool) ToolDefinition {
	if tool.Name != "" {
		name = tool.Name
	}
	return ToolDefinition{
		Name:             name,
		Title:            tool.Title,
		Description:      tool.Description,
		InputSchema:      tool.InputSchema,
		OutputSchema:     tool.OutputSchema,
		InputExamples:    tool.InputExamples,
		Strict:           tool.Strict,
		ProviderOptions:  tool.ProviderOptions,
		ProviderMetadata: tool.ProviderMetadata,
		Type:             tool.Type,
		ID:               tool.ID,
		Args:             tool.Args,
	}
}

func ToolDefinitionsFromAI(tools map[string]ai.Tool) []ToolDefinition {
	if len(tools) == 0 {
		return nil
	}
	definitions := make([]ToolDefinition, 0, len(tools))
	for name, tool := range tools {
		definitions = append(definitions, ToolDefinitionFromAI(name, tool))
	}
	return definitions
}

func (definition ToolDefinition) ToModelTool() ai.ModelTool {
	toolType := definition.Type
	if toolType == "" || toolType == "function" || toolType == "dynamic" {
		return ai.ModelTool{
			Type:            "function",
			Name:            definition.Name,
			Description:     definition.Description,
			InputSchema:     normalizeSchema(definition.InputSchema),
			InputExamples:   definition.InputExamples,
			Strict:          definition.Strict,
			ProviderOptions: definition.ProviderOptions,
		}
	}
	return ai.ModelTool{
		Type: "provider",
		Name: definition.Name,
		ID:   definition.ID,
		Args: definition.Args,
	}
}

func (definition ToolDefinition) ToAI() ai.Tool {
	return ai.Tool{
		Name:             definition.Name,
		Title:            definition.Title,
		Description:      definition.Description,
		InputSchema:      definition.InputSchema,
		OutputSchema:     definition.OutputSchema,
		InputExamples:    definition.InputExamples,
		Strict:           definition.Strict,
		ProviderOptions:  definition.ProviderOptions,
		ProviderMetadata: definition.ProviderMetadata,
		Type:             definition.Type,
		ID:               definition.ID,
		Args:             definition.Args,
	}
}

func ModelToolsFromDefinitions(definitions []ToolDefinition, choice ai.ToolChoice) []ai.ModelTool {
	if len(definitions) == 0 || choice.Type == "none" {
		return nil
	}
	out := make([]ai.ModelTool, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Name == "" {
			continue
		}
		if choice.Type == "tool" && choice.ToolName != "" && choice.ToolName != definition.Name {
			continue
		}
		out = append(out, definition.ToModelTool())
	}
	return out
}

func (a *Activities) InvokeTool(ctx context.Context, args InvokeToolArgs) (*InvokeToolResult, error) {
	if args.ToolCallID == "" {
		return nil, fmt.Errorf("toolCallId is required")
	}
	if args.ToolName == "" {
		return nil, fmt.Errorf("toolName is required")
	}
	tool, ok := a.tool(args.ToolName)
	dynamic := ok && tool.Type == "dynamic"
	metadata := ai.ProviderMetadata(nil)
	if ok {
		metadata = tool.ProviderMetadata
	}
	if err := a.publishToolLifecycleInput(ctx, args, dynamic); err != nil {
		return nil, err
	}
	if !ok {
		return a.finishToolLifecycle(ctx, args, toolErrorResult(args, fmt.Errorf("tool %q is not registered", args.ToolName), nil))
	}
	call := ai.ToolCall{
		ToolCallID:       args.ToolCallID,
		ToolName:         args.ToolName,
		Input:            args.Input,
		Dynamic:          dynamic,
		ProviderMetadata: metadata,
	}
	if err := ai.ValidateToolInput(tool, args.Input); err != nil {
		return a.finishToolLifecycle(ctx, args, toolErrorResult(args, err, call.ProviderMetadata))
	}
	if tool.NeedsApproval != nil {
		decision, err := tool.NeedsApproval(ctx, call)
		if err != nil {
			return a.finishToolLifecycle(ctx, args, toolErrorResult(args, err, call.ProviderMetadata))
		}
		if decision.Type == "denied" || decision.Type == "user-approval" {
			return a.finishToolLifecycle(ctx, args, &InvokeToolResult{
				ToolCallID:       args.ToolCallID,
				ToolName:         args.ToolName,
				Input:            args.Input,
				Output:           ai.ToolResultOutput{Type: "execution-denied", Reason: decision.Reason},
				IsError:          decision.Type == "denied",
				Dynamic:          call.Dynamic,
				ProviderMetadata: call.ProviderMetadata,
			})
		}
	}
	if tool.Execute == nil {
		return a.finishToolLifecycle(ctx, args, toolErrorResult(args, fmt.Errorf("tool %q has no execute function", args.ToolName), call.ProviderMetadata))
	}
	output, err := tool.Execute(ctx, call, ai.ToolExecutionOptions{
		ToolCallID: args.ToolCallID,
		Messages:   MessagesToAI(args.Messages),
		Context:    args.Context,
	})
	isError := err != nil
	modelOutputInput := output
	if isError {
		modelOutputInput = err.Error()
	} else if err := ai.ValidateToolOutput(tool, output); err != nil {
		isError = true
		modelOutputInput = err.Error()
	}
	modelOutput, modelErr := ai.CreateToolModelOutput(tool, args.ToolCallID, args.Input, modelOutputInput, isError)
	if modelErr != nil {
		isError = true
		modelOutput = ai.ToolResultOutput{Type: "error-text", Value: modelErr.Error()}
	}
	return a.finishToolLifecycle(ctx, args, &InvokeToolResult{
		ToolCallID:       args.ToolCallID,
		ToolName:         args.ToolName,
		Input:            args.Input,
		Output:           modelOutput,
		Result:           output,
		IsError:          isError,
		Dynamic:          call.Dynamic,
		ProviderMetadata: call.ProviderMetadata,
	})
}

func toolErrorResult(args InvokeToolArgs, err error, metadata ai.ProviderMetadata) *InvokeToolResult {
	text := "tool execution failed"
	if err != nil {
		text = err.Error()
	}
	return &InvokeToolResult{
		ToolCallID:       args.ToolCallID,
		ToolName:         args.ToolName,
		Input:            args.Input,
		Output:           ai.ToolResultOutput{Type: "error-text", Value: text},
		IsError:          true,
		ProviderMetadata: metadata,
	}
}

func (a *Activities) tool(name string) (ai.Tool, bool) {
	if a == nil || len(a.tools) == 0 {
		return ai.Tool{}, false
	}
	tool, ok := a.tools[name]
	return tool, ok
}

func (a *Activities) publishToolLifecycleInput(ctx context.Context, args InvokeToolArgs, dynamic bool) error {
	input, ok := toolLifecycleInput(args, streaming.ToolInputAvailable)
	if !ok {
		return nil
	}
	input.Input = args.Input
	input.Dynamic = dynamic
	return a.publishToolLifecycle(ctx, args, input)
}

func (a *Activities) finishToolLifecycle(ctx context.Context, args InvokeToolArgs, result *InvokeToolResult) (*InvokeToolResult, error) {
	input, ok := toolLifecycleInput(args, toolLifecycleEventForResult(result))
	if !ok {
		return result, nil
	}
	input.Input = result.Input
	input.Output = result.Output
	input.Dynamic = result.Dynamic
	input.ProviderExecuted = result.ProviderExecuted
	input.Preliminary = result.Preliminary
	if result.IsError {
		input.ErrorText = toolLifecycleErrorText(result)
		input.Output = nil
	}
	if err := a.publishToolLifecycle(ctx, args, input); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *Activities) publishToolLifecycle(ctx context.Context, args InvokeToolArgs, input streaming.ToolLifecycleInput) error {
	return publishLifecycleEvent(ctx, a.connector, args.Lifecycle.DurableRequired, input)
}

func toolLifecycleInput(args InvokeToolArgs, event streaming.ToolLifecycleEvent) (streaming.ToolLifecycleInput, bool) {
	if args.Lifecycle.StreamID == "" {
		return streaming.ToolLifecycleInput{}, false
	}
	phase := "terminal"
	if event == streaming.ToolInputAvailable {
		phase = "input"
	}
	return streaming.ToolLifecycleInput{
		EventID:    toolLifecycleStableEventID(args.ToolCallID, phase),
		StreamID:   args.Lifecycle.StreamID,
		Event:      event,
		ToolCallID: args.ToolCallID,
		ToolName:   args.ToolName,
		Metadata:   args.Lifecycle.Metadata,
	}, true
}

func toolLifecycleEventForResult(result *InvokeToolResult) streaming.ToolLifecycleEvent {
	if result != nil && result.IsError {
		return streaming.ToolOutputError
	}
	return streaming.ToolOutputAvailable
}

func toolLifecycleErrorText(result *InvokeToolResult) string {
	if result == nil {
		return "tool execution failed"
	}
	if text, ok := result.Output.Value.(string); ok && text != "" {
		return text
	}
	if result.Output.Reason != "" {
		return result.Output.Reason
	}
	return "tool execution failed"
}

func logToolLifecycleLiveError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	activity.GetLogger(ctx).Warn("tool lifecycle live publish failed", "error", err)
}

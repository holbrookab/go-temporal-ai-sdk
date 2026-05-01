package activities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/holbrookab/go-ai/packages/ai"
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
	if !ok {
		return toolErrorResult(args, fmt.Errorf("tool %q is not registered", args.ToolName), nil), nil
	}
	call := ai.ToolCall{
		ToolCallID:       args.ToolCallID,
		ToolName:         args.ToolName,
		Input:            args.Input,
		Dynamic:          tool.Type == "dynamic",
		ProviderMetadata: tool.ProviderMetadata,
	}
	if err := ai.ValidateToolInput(tool, args.Input); err != nil {
		return toolErrorResult(args, err, call.ProviderMetadata), nil
	}
	if tool.NeedsApproval != nil {
		decision, err := tool.NeedsApproval(ctx, call)
		if err != nil {
			return toolErrorResult(args, err, call.ProviderMetadata), nil
		}
		if decision.Type == "denied" || decision.Type == "user-approval" {
			return &InvokeToolResult{
				ToolCallID:       args.ToolCallID,
				ToolName:         args.ToolName,
				Input:            args.Input,
				Output:           ai.ToolResultOutput{Type: "execution-denied", Reason: decision.Reason},
				IsError:          decision.Type == "denied",
				Dynamic:          call.Dynamic,
				ProviderMetadata: call.ProviderMetadata,
			}, nil
		}
	}
	if tool.Execute == nil {
		return toolErrorResult(args, fmt.Errorf("tool %q has no execute function", args.ToolName), call.ProviderMetadata), nil
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
	return &InvokeToolResult{
		ToolCallID:       args.ToolCallID,
		ToolName:         args.ToolName,
		Input:            args.Input,
		Output:           modelOutput,
		Result:           output,
		IsError:          isError,
		Dynamic:          call.Dynamic,
		ProviderMetadata: call.ProviderMetadata,
	}, nil
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

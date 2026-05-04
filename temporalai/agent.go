package temporalai

import (
	"encoding/json"
	"fmt"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	defaultAgentMaxSteps = 20

	ToolExecutionParallel   = "parallel"
	ToolExecutionSequential = "sequential"

	LocalToolTimeoutFallbackActivity LocalToolTimeoutFallback = "activity"
	LocalToolTimeoutFallbackNone     LocalToolTimeoutFallback = "none"
)

type LocalToolTimeoutFallback string

type AgentInput struct {
	AgentID                  string                              `json:"agentId,omitempty"`
	ModelID                  string                              `json:"modelId"`
	Instructions             string                              `json:"instructions,omitempty"`
	Prompt                   string                              `json:"prompt,omitempty"`
	Messages                 []activities.Message                `json:"messages,omitempty"`
	Tools                    []activities.ToolDefinition         `json:"tools,omitempty"`
	ToolChoice               ai.ToolChoice                       `json:"toolChoice,omitempty"`
	FirstToolChoice          ai.ToolChoice                       `json:"firstToolChoice,omitempty"`
	MaxSteps                 int                                 `json:"maxSteps,omitempty"`
	ModelOptions             activities.LanguageModelCallOptions `json:"modelOptions,omitempty"`
	Stream                   streaming.Options                   `json:"stream,omitempty"`
	UseStreamingModel        bool                                `json:"useStreamingModel,omitempty"`
	ToolContext              any                                 `json:"toolContext,omitempty"`
	ToolExecution            string                              `json:"toolExecution,omitempty"`
	ToolApproval             AgentToolApprovalOptions            `json:"toolApproval,omitempty"`
	DefaultToolBoundary      activities.ToolExecutionBoundary    `json:"defaultToolBoundary,omitempty"`
	LocalToolTimeoutFallback LocalToolTimeoutFallback            `json:"localToolTimeoutFallback,omitempty"`
}

type AgentResult struct {
	AgentID          string               `json:"agentId,omitempty"`
	ModelID          string               `json:"modelId"`
	Text             string               `json:"text,omitempty"`
	FinishReason     string               `json:"finishReason,omitempty"`
	RawFinishReason  string               `json:"rawFinishReason,omitempty"`
	Usage            ai.Usage             `json:"usage,omitempty"`
	Warnings         []ai.Warning         `json:"warnings,omitempty"`
	ProviderMetadata ai.ProviderMetadata  `json:"providerMetadata,omitempty"`
	Messages         []activities.Message `json:"messages,omitempty"`
	Steps            []AgentStep          `json:"steps,omitempty"`
}

type AgentStep struct {
	StepNumber  int                                    `json:"stepNumber"`
	ModelResult activities.LanguageModelGenerateResult `json:"modelResult"`
	Text        string                                 `json:"text,omitempty"`
	ToolCalls   []AgentToolCall                        `json:"toolCalls,omitempty"`
	ToolResults []activities.InvokeToolResult          `json:"toolResults,omitempty"`
}

type AgentToolCall struct {
	ToolCallID       string              `json:"toolCallId"`
	ToolName         string              `json:"toolName"`
	Input            any                 `json:"input,omitempty"`
	InputRaw         string              `json:"inputRaw,omitempty"`
	ProviderExecuted bool                `json:"providerExecuted,omitempty"`
	Dynamic          bool                `json:"dynamic,omitempty"`
	Invalid          bool                `json:"invalid,omitempty"`
	ErrorText        string              `json:"errorText,omitempty"`
	ProviderMetadata ai.ProviderMetadata `json:"providerMetadata,omitempty"`
}

func AgentWorkflow(ctx workflow.Context, input AgentInput) (*AgentResult, error) {
	return RunAgent(ctx, input)
}

func RunAgent(ctx workflow.Context, input AgentInput, activityOptions ...ActivityOptions) (*AgentResult, error) {
	if input.ModelID == "" {
		return nil, fmt.Errorf("modelId is required")
	}
	maxSteps := input.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultAgentMaxSteps
	}
	messages := initialAgentMessages(input)
	result := &AgentResult{
		AgentID:  input.AgentID,
		ModelID:  input.ModelID,
		Messages: append([]activities.Message(nil), messages...),
	}
	for stepNumber := 0; stepNumber < maxSteps; stepNumber++ {
		callOptions := input.ModelOptions
		callOptions.Prompt = append([]activities.Message(nil), messages...)
		toolChoice := input.ToolChoice
		if stepNumber == 0 && input.FirstToolChoice.Type != "" {
			toolChoice = input.FirstToolChoice
		}
		callOptions.Tools = activities.ModelToolsFromDefinitions(input.Tools, toolChoice)
		if toolChoice.Type != "" {
			callOptions.ToolChoice = toolChoice
		} else {
			callOptions.ToolChoice = ai.AutoToolChoice()
		}
		if input.Stream.Visible || input.UseStreamingModel {
			callOptions.ProviderOptions = withAgentStreamOptions(ctx, input, stepNumber, callOptions.ProviderOptions)
		}

		modelResult, err := invokeAgentModel(ctx, input, callOptions, activityOptions...)
		if err != nil {
			return nil, err
		}
		step := AgentStep{
			StepNumber:  stepNumber,
			ModelResult: *modelResult,
			Text:        textFromWireParts(modelResult.Content),
			ToolCalls:   extractToolCalls(modelResult.Content),
		}
		result.Text = step.Text
		result.FinishReason = modelResult.FinishReason.Unified
		result.RawFinishReason = modelResult.FinishReason.Raw
		result.Usage = ai.AddUsage(result.Usage, modelResult.Usage)
		result.Warnings = append(result.Warnings, modelResult.Warnings...)
		result.ProviderMetadata = modelResult.ProviderMetadata

		messages = append(messages, activities.Message{Role: ai.RoleAssistant, Content: modelResult.Content})
		result.Messages = append([]activities.Message(nil), messages...)
		if len(step.ToolCalls) == 0 {
			result.Steps = append(result.Steps, step)
			return result, nil
		}
		toolResults, err := executeAgentTools(ctx, input, messages, step.ToolCalls, activityOptions...)
		if err != nil {
			return nil, err
		}
		step.ToolResults = toolResults
		result.Steps = append(result.Steps, step)
		if len(toolResults) == 0 {
			return result, nil
		}
		messages = append(messages, activities.Message{Role: ai.RoleTool, Content: toolResultParts(toolResults)})
		result.Messages = append([]activities.Message(nil), messages...)
	}
	return result, nil
}

func ExecuteAgentChildWorkflow(ctx workflow.Context, workflowType any, input AgentInput, options ...workflow.ChildWorkflowOptions) (*AgentResult, error) {
	if len(options) > 0 {
		ctx = workflow.WithChildOptions(ctx, options[0])
	}
	var result AgentResult
	if err := workflow.ExecuteChildWorkflow(ctx, workflowType, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func invokeAgentModel(ctx workflow.Context, input AgentInput, options activities.LanguageModelCallOptions, activityOptions ...ActivityOptions) (*activities.LanguageModelGenerateResult, error) {
	if input.UseStreamingModel || input.Stream.Visible {
		streamResult, err := InvokeModelStream(ctx, input.ModelID, options.ToAI(), activityOptions...)
		if err != nil {
			return nil, err
		}
		return generateResultFromStream(streamResult), nil
	}
	result, err := InvokeModel(ctx, input.ModelID, options.ToAI(), activityOptions...)
	if err != nil {
		return nil, err
	}
	wire := activities.GenerateResultFromAI(result)
	return wire, nil
}

func executeAgentTools(ctx workflow.Context, input AgentInput, messages []activities.Message, calls []AgentToolCall, activityOptions ...ActivityOptions) ([]activities.InvokeToolResult, error) {
	if input.ToolExecution == ToolExecutionSequential {
		return executeAgentToolsSequential(ctx, input, messages, calls, activityOptions...)
	}
	return executeAgentToolsParallel(ctx, input, messages, calls, activityOptions...)
}

func executeAgentToolsSequential(ctx workflow.Context, input AgentInput, messages []activities.Message, calls []AgentToolCall, activityOptions ...ActivityOptions) ([]activities.InvokeToolResult, error) {
	results := make([]activities.InvokeToolResult, 0, len(calls))
	for _, call := range calls {
		if call.ProviderExecuted {
			continue
		}
		result, err := executeOneAgentTool(ctx, input, messages, call, activityOptions...)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}
	return results, nil
}

func executeAgentToolsParallel(ctx workflow.Context, input AgentInput, messages []activities.Message, calls []AgentToolCall, activityOptions ...ActivityOptions) ([]activities.InvokeToolResult, error) {
	type toolOutcome struct {
		index  int
		result *activities.InvokeToolResult
		err    error
	}
	count := 0
	outcomes := workflow.NewChannel(ctx)
	for _, call := range calls {
		if call.ProviderExecuted {
			continue
		}
		index := count
		toolCall := call
		count++
		workflow.Go(ctx, func(ctx workflow.Context) {
			result, err := executeOneAgentTool(ctx, input, messages, toolCall, activityOptions...)
			outcomes.Send(ctx, toolOutcome{index: index, result: result, err: err})
		})
	}
	results := make([]activities.InvokeToolResult, count)
	for i := 0; i < count; i++ {
		var outcome toolOutcome
		outcomes.Receive(ctx, &outcome)
		if outcome.err != nil {
			return nil, outcome.err
		}
		if outcome.result != nil {
			results[outcome.index] = *outcome.result
		}
	}
	return results, nil
}

func executeOneAgentTool(ctx workflow.Context, input AgentInput, messages []activities.Message, call AgentToolCall, activityOptions ...ActivityOptions) (*activities.InvokeToolResult, error) {
	ao := ActivityOptions{}
	if len(activityOptions) > 0 {
		ao = activityOptions[0]
	}
	approval, inputPublished, deniedResult, err := approveAgentToolIfRequired(ctx, input, call, ao)
	if err != nil {
		return nil, err
	}
	if deniedResult != nil {
		return deniedResult, nil
	}
	future, boundary := executeAgentToolFuture(ctx, input, messages, call, approval, inputPublished, ao)
	return agentToolResultFromFuture(ctx, input, messages, call, ao, future, boundary, approval, inputPublished)
}

func executeAgentToolFuture(ctx workflow.Context, input AgentInput, messages []activities.Message, call AgentToolCall, approval *activities.ToolApprovalState, suppressInputLifecycle bool, options ActivityOptions) (workflow.Future, activities.ToolExecutionBoundary) {
	args := activities.InvokeToolArgs{
		ToolCallID:             call.ToolCallID,
		ToolName:               call.ToolName,
		Input:                  call.Input,
		Messages:               messages,
		Context:                input.ToolContext,
		Lifecycle:              toolLifecycleOptions(ctx, input),
		Approval:               approval,
		SuppressInputLifecycle: suppressInputLifecycle,
	}
	boundary := toolExecutionBoundary(input, call.ToolName)
	switch boundary {
	case activities.ToolExecutionBoundaryLocalActivity:
		toolCtx := workflow.WithLocalActivityOptions(ctx, localToolActivityOptions(options))
		return workflow.ExecuteLocalActivity(toolCtx, activities.InvokeToolActivity, args), boundary
	default:
		return executeAgentToolActivityFuture(ctx, args, options), boundary
	}
}

func agentToolResultFromFuture(ctx workflow.Context, input AgentInput, messages []activities.Message, call AgentToolCall, options ActivityOptions, future workflow.Future, boundary activities.ToolExecutionBoundary, approval *activities.ToolApprovalState, suppressInputLifecycle bool) (*activities.InvokeToolResult, error) {
	var result activities.InvokeToolResult
	if err := future.Get(ctx, &result); err != nil {
		if shouldFallbackLocalToolTimeout(input, boundary, err) {
			args := activities.InvokeToolArgs{
				ToolCallID:             call.ToolCallID,
				ToolName:               call.ToolName,
				Input:                  call.Input,
				Messages:               messages,
				Context:                input.ToolContext,
				Lifecycle:              toolLifecycleOptions(ctx, input),
				Approval:               approval,
				SuppressInputLifecycle: suppressInputLifecycle,
			}
			var fallbackResult activities.InvokeToolResult
			if fallbackErr := executeAgentToolActivityFuture(ctx, args, options).Get(ctx, &fallbackResult); fallbackErr != nil {
				return nil, fallbackErr
			}
			return &fallbackResult, nil
		}
		return nil, err
	}
	return &result, nil
}

func executeAgentToolActivityFuture(ctx workflow.Context, args activities.InvokeToolArgs, options ActivityOptions) workflow.Future {
	toolCtx := workflow.WithActivityOptions(ctx, toolActivityOptions(options))
	return workflow.ExecuteActivity(toolCtx, activities.InvokeToolActivity, args)
}

func shouldFallbackLocalToolTimeout(input AgentInput, boundary activities.ToolExecutionBoundary, err error) bool {
	return boundary == activities.ToolExecutionBoundaryLocalActivity &&
		localToolTimeoutFallback(input) == LocalToolTimeoutFallbackActivity &&
		temporal.IsTimeoutError(err)
}

func approveAgentToolIfRequired(ctx workflow.Context, input AgentInput, call AgentToolCall, options ActivityOptions) (*activities.ToolApprovalState, bool, *activities.InvokeToolResult, error) {
	definition, ok := agentToolDefinition(input, call.ToolName)
	if !ok || !definition.RequiresApproval {
		return nil, false, nil, nil
	}
	lifecycle := toolLifecycleOptions(ctx, input)
	metadata := mergeApprovalMetadata(lifecycle.Metadata, input.ToolApproval.Metadata)
	inputPublished := false
	if lifecycle.StreamID != "" {
		if err := PublishToolLifecycleEvent(ctx, streaming.ToolLifecycleInput{
			EventID:    toolApprovalEventID(call.ToolCallID, "input"),
			StreamID:   lifecycle.StreamID,
			Event:      streaming.ToolInputAvailable,
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Input:      call.Input,
			Dynamic:    call.Dynamic,
			Metadata:   metadata,
		}, options); err != nil {
			return nil, false, nil, err
		}
		inputPublished = true
	}
	approvalID := fmt.Sprintf("%s:approval", call.ToolCallID)
	response, err := RequestToolApproval(ctx, ToolApprovalRequest{
		StreamID:        lifecycle.StreamID,
		ApprovalID:      approvalID,
		ToolCallID:      call.ToolCallID,
		ToolName:        call.ToolName,
		Input:           call.Input,
		Metadata:        metadata,
		DurableRequired: lifecycle.DurableRequired,
		Timeout:         input.ToolApproval.Timeout,
		SignalName:      input.ToolApproval.SignalName,
	}, options)
	if err != nil {
		return nil, inputPublished, nil, err
	}
	approval := toolApprovalState(response)
	if response.Approved {
		return approval, inputPublished, nil, nil
	}
	result := deniedAgentToolResult(call, response.Reason)
	if lifecycle.StreamID != "" {
		if err := PublishToolLifecycleEvent(ctx, streaming.ToolLifecycleInput{
			EventID:    toolApprovalEventID(call.ToolCallID, "terminal"),
			StreamID:   lifecycle.StreamID,
			Event:      streaming.ToolOutputDenied,
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Metadata:   metadata,
		}, options); err != nil {
			return nil, inputPublished, nil, err
		}
	}
	return approval, inputPublished, result, nil
}

func deniedAgentToolResult(call AgentToolCall, reason string) *activities.InvokeToolResult {
	return &activities.InvokeToolResult{
		ToolCallID:       call.ToolCallID,
		ToolName:         call.ToolName,
		Input:            call.Input,
		Output:           ai.ToolResultOutput{Type: "execution-denied", Reason: reason},
		Dynamic:          call.Dynamic,
		ProviderMetadata: call.ProviderMetadata,
	}
}

func agentToolDefinition(input AgentInput, toolName string) (activities.ToolDefinition, bool) {
	for _, tool := range input.Tools {
		if tool.Name == toolName {
			return tool, true
		}
	}
	return activities.ToolDefinition{}, false
}

func mergeApprovalMetadata(base map[string]any, extra map[string]any) map[string]any {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func localToolTimeoutFallback(input AgentInput) LocalToolTimeoutFallback {
	if input.LocalToolTimeoutFallback == "" {
		return LocalToolTimeoutFallbackActivity
	}
	return input.LocalToolTimeoutFallback
}

func aoFromActivityOptions(activityOptions ...ActivityOptions) ActivityOptions {
	if len(activityOptions) > 0 {
		return activityOptions[0]
	}
	return ActivityOptions{}
}

func toolExecutionBoundary(input AgentInput, toolName string) activities.ToolExecutionBoundary {
	for _, tool := range input.Tools {
		if tool.Name != toolName {
			continue
		}
		if tool.ExecutionBoundary != "" && tool.ExecutionBoundary != activities.ToolExecutionBoundaryAuto {
			return tool.ExecutionBoundary
		}
		break
	}
	if input.DefaultToolBoundary != "" && input.DefaultToolBoundary != activities.ToolExecutionBoundaryAuto {
		return input.DefaultToolBoundary
	}
	return activities.ToolExecutionBoundaryActivity
}

func initialAgentMessages(input AgentInput) []activities.Message {
	messages := make([]activities.Message, 0, len(input.Messages)+2)
	if input.Instructions != "" {
		messages = append(messages, activities.Message{Role: ai.RoleSystem, Text: input.Instructions})
	}
	messages = append(messages, input.Messages...)
	if input.Prompt != "" {
		messages = append(messages, activities.Message{
			Role:    ai.RoleUser,
			Content: []activities.Part{{Type: "text", Text: input.Prompt}},
		})
	}
	return messages
}

func withAgentStreamOptions(ctx workflow.Context, input AgentInput, stepNumber int, providerOptions ai.ProviderOptions) ai.ProviderOptions {
	out := ai.ProviderOptions{}
	for key, value := range providerOptions {
		out[key] = value
	}
	options := input.Stream
	if options.StreamID == "" {
		options.StreamID = streamID(ctx, "")
	}
	if options.AttemptID == "" {
		agentID := input.AgentID
		if agentID == "" {
			agentID = "agent"
		}
		options.AttemptID = fmt.Sprintf("%s:step-%d", agentID, stepNumber)
	}
	if !options.Visible && input.UseStreamingModel {
		options.Visible = true
	}
	out[activities.ProviderOptionsKey] = options
	return out
}

func streamID(ctx workflow.Context, configured string) string {
	if configured != "" {
		return configured
	}
	return workflow.GetInfo(ctx).WorkflowExecution.ID
}

func toolLifecycleStreamID(ctx workflow.Context, input AgentInput) string {
	if !input.Stream.Visible && input.Stream.StreamID == "" && !input.UseStreamingModel {
		return ""
	}
	return streamID(ctx, input.Stream.StreamID)
}

func toolLifecycleOptions(ctx workflow.Context, input AgentInput) activities.ToolLifecycleOptions {
	streamID := toolLifecycleStreamID(ctx, input)
	if streamID == "" {
		return activities.ToolLifecycleOptions{}
	}
	metadata := map[string]any{"agentId": input.AgentID}
	for key, value := range toolLifecycleMetadataFromContext(input.ToolContext) {
		metadata[key] = value
	}
	return activities.ToolLifecycleOptions{
		StreamID:        streamID,
		Metadata:        metadata,
		DurableRequired: true,
	}
}

func toolLifecycleMetadataFromContext(context any) map[string]any {
	if context == nil {
		return nil
	}
	var raw map[string]any
	switch value := context.(type) {
	case map[string]any:
		raw = value
	default:
		payload, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(payload, &raw); err != nil {
			return nil
		}
	}
	metadata := map[string]any{}
	copyStringMetadata(metadata, raw, "taskId")
	copyStringMetadata(metadata, raw, "taskTitle")
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func copyStringMetadata(out map[string]any, raw map[string]any, key string) {
	value, ok := raw[key].(string)
	if !ok || value == "" {
		return
	}
	out[key] = value
}

func extractToolCalls(parts []activities.Part) []AgentToolCall {
	calls := []AgentToolCall{}
	for _, part := range parts {
		if part.Type != "tool-call" {
			continue
		}
		input := part.Input
		errorText := part.ErrorText
		invalid := part.Invalid
		if input == nil && part.InputRaw != "" {
			var parsed any
			if err := json.Unmarshal([]byte(part.InputRaw), &parsed); err != nil {
				input = part.InputRaw
				errorText = err.Error()
				invalid = true
			} else {
				input = parsed
			}
		}
		calls = append(calls, AgentToolCall{
			ToolCallID:       part.ToolCallID,
			ToolName:         part.ToolName,
			Input:            input,
			InputRaw:         part.InputRaw,
			ProviderExecuted: part.ProviderExecuted,
			Dynamic:          part.Dynamic,
			Invalid:          invalid,
			ErrorText:        errorText,
			ProviderMetadata: part.ProviderMetadata,
		})
	}
	return calls
}

func toolResultParts(results []activities.InvokeToolResult) []activities.Part {
	parts := make([]activities.Part, 0, len(results))
	for _, result := range results {
		parts = append(parts, activities.Part{
			Type:             "tool-result",
			ToolCallID:       result.ToolCallID,
			ToolName:         result.ToolName,
			Input:            result.Input,
			Output:           result.Output,
			Result:           result.Result,
			IsError:          result.IsError,
			Dynamic:          result.Dynamic,
			ProviderExecuted: result.ProviderExecuted,
			Preliminary:      result.Preliminary,
			ProviderMetadata: result.ProviderMetadata,
		})
	}
	return parts
}

func textFromWireParts(parts []activities.Part) string {
	var out string
	for _, part := range parts {
		if part.Type == "text" {
			out += part.Text
		}
	}
	return out
}

func generateResultFromStream(result *activities.InvokeModelStreamAIResult) *activities.LanguageModelGenerateResult {
	if result.Result != nil {
		return activities.GenerateResultFromAI(result.Result)
	}
	out := &activities.LanguageModelGenerateResult{
		Request:  result.Request,
		Response: activities.ResponseMetadataFromAI(result.Response),
	}
	var text string
	var reasoning string
	toolInputs := map[string]string{}
	for _, part := range result.StreamParts {
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
			out.Content = append(out.Content, activities.Part{
				Type:             "tool-call",
				ToolCallID:       part.ToolCallID,
				ToolName:         part.ToolName,
				Input:            toolCallInputFromRaw(input),
				InputRaw:         input,
				ProviderMetadata: part.ProviderMetadata,
			})
		case "finish":
			out.FinishReason = part.FinishReason
			out.Usage = part.Usage
			out.Warnings = append(out.Warnings, part.Warnings...)
			out.ProviderMetadata = part.ProviderMetadata
		}
	}
	if text != "" {
		out.Content = append([]activities.Part{{Type: "text", Text: text}}, out.Content...)
	}
	if reasoning != "" {
		out.Content = append([]activities.Part{{Type: "reasoning", Text: reasoning}}, out.Content...)
	}
	return out
}

func toolCallInputFromRaw(inputRaw string) any {
	if inputRaw == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(inputRaw), &parsed); err != nil {
		return nil
	}
	return parsed
}

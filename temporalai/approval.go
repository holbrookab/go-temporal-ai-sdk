package temporalai

import (
	"fmt"
	"time"

	"github.com/holbrookab/go-ai/packages/ai"
	"github.com/holbrookab/go-temporal-ai-sdk/activities"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
	"go.temporal.io/sdk/workflow"
)

const (
	ToolApprovalSignalName = "go-temporal-ai-sdk.tool-approval-response"
)

type AgentToolApprovalOptions struct {
	SignalName string         `json:"signalName,omitempty"`
	Timeout    time.Duration  `json:"timeout,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ToolApprovalRequest struct {
	StreamID        string              `json:"streamId,omitempty"`
	ApprovalID      string              `json:"approvalId"`
	ToolCallID      string              `json:"toolCallId"`
	ToolName        string              `json:"toolName"`
	Input           any                 `json:"input,omitempty"`
	ToolMetadata    ai.ProviderMetadata `json:"toolMetadata,omitempty"`
	Metadata        map[string]any      `json:"metadata,omitempty"`
	DurableRequired bool                `json:"durableRequired,omitempty"`
	Timeout         time.Duration       `json:"timeout,omitempty"`
	SignalName      string              `json:"signalName,omitempty"`
	streaming.Scope
}

type ToolApprovalResponse struct {
	ApprovalID string `json:"approvalId"`
	ToolCallID string `json:"toolCallId,omitempty"`
	Approved   bool   `json:"approved"`
	Reason     string `json:"reason,omitempty"`
	TimedOut   bool   `json:"timedOut,omitempty"`
	Canceled   bool   `json:"canceled,omitempty"`
}

func RequestToolApproval(ctx workflow.Context, request ToolApprovalRequest, activityOptions ...ActivityOptions) (*ToolApprovalResponse, error) {
	if request.ApprovalID == "" {
		return nil, fmt.Errorf("approvalId is required")
	}
	if request.ToolCallID == "" {
		return nil, fmt.Errorf("toolCallId is required")
	}
	if request.ToolName == "" {
		return nil, fmt.Errorf("toolName is required")
	}
	if request.StreamID != "" {
		if err := PublishToolLifecycleEvent(ctx, streaming.ToolLifecycleInput{
			EventID:      toolApprovalEventID(request.ToolCallID, "approval-request"),
			StreamID:     request.StreamID,
			Event:        streaming.ToolApprovalRequest,
			ToolCallID:   request.ToolCallID,
			ToolName:     request.ToolName,
			ApprovalID:   request.ApprovalID,
			Input:        request.Input,
			ToolMetadata: request.ToolMetadata,
			Metadata:     request.Metadata,
			Scope:        request.Scope,
		}, activityOptions...); err != nil {
			return nil, err
		}
	}
	response := waitForToolApprovalResponse(ctx, request)
	if request.StreamID != "" {
		if err := PublishToolLifecycleEvent(ctx, streaming.ToolLifecycleInput{
			EventID:      toolApprovalEventID(request.ToolCallID, "approval-response"),
			StreamID:     request.StreamID,
			Event:        streaming.ToolApprovalResponse,
			ToolCallID:   request.ToolCallID,
			ToolName:     request.ToolName,
			ApprovalID:   request.ApprovalID,
			Approved:     &response.Approved,
			Reason:       response.Reason,
			ToolMetadata: request.ToolMetadata,
			Metadata:     request.Metadata,
			Scope:        request.Scope,
		}, activityOptions...); err != nil {
			return nil, err
		}
	}
	return &response, nil
}

func waitForToolApprovalResponse(ctx workflow.Context, request ToolApprovalRequest) ToolApprovalResponse {
	signalName := request.SignalName
	if signalName == "" {
		signalName = ToolApprovalResponseSignalName(request.ApprovalID)
	}
	signalCh := workflow.GetSignalChannel(ctx, signalName)
	var response ToolApprovalResponse
	for {
		selector := workflow.NewSelector(ctx)
		received := false
		selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, &response)
			received = true
		})
		timedOut := false
		if request.Timeout > 0 {
			timer := workflow.NewTimer(ctx, request.Timeout)
			selector.AddFuture(timer, func(workflow.Future) {
				timedOut = true
			})
		}
		selector.Select(ctx)
		if timedOut {
			return ToolApprovalResponse{
				ApprovalID: request.ApprovalID,
				ToolCallID: request.ToolCallID,
				Approved:   false,
				Reason:     "approval timed out",
				TimedOut:   true,
			}
		}
		if !received || response.ApprovalID != request.ApprovalID {
			continue
		}
		if response.ToolCallID == "" {
			response.ToolCallID = request.ToolCallID
		}
		return response
	}
}

func ToolApprovalResponseSignalName(approvalID string) string {
	if approvalID == "" {
		return ToolApprovalSignalName
	}
	return fmt.Sprintf("%s.%s", ToolApprovalSignalName, approvalID)
}

func toolApprovalState(response *ToolApprovalResponse) *activities.ToolApprovalState {
	if response == nil {
		return nil
	}
	return &activities.ToolApprovalState{
		ApprovalID: response.ApprovalID,
		Approved:   &response.Approved,
		Reason:     response.Reason,
	}
}

func toolApprovalEventID(toolCallID string, phase string) string {
	return fmt.Sprintf("tool:%s:%s", toolCallID, phase)
}

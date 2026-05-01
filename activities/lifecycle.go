package activities

import (
	"context"
	"fmt"

	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

type ToolLifecycleActivityOptions struct {
	Connector       streaming.Connector
	Start           streaming.ToolLifecycleInput
	DurableRequired bool
	MapTerminal     ToolLifecycleTerminalMapper
}

type ToolLifecycleTerminal struct {
	Output           any
	ErrorText        string
	IsError          bool
	Dynamic          bool
	ProviderExecuted bool
	Preliminary      bool
}

type ToolLifecycleTerminalMapper func(result any, err error) ToolLifecycleTerminal

func RunWithToolLifecycle(ctx context.Context, opts ToolLifecycleActivityOptions, execute func(context.Context) (any, error)) (any, error) {
	connector := opts.Connector
	if connector == nil {
		connector = streaming.NoopConnector{}
	}
	start := opts.Start
	if start.StreamID != "" {
		start.Event = streaming.ToolInputAvailable
		if start.EventID == "" {
			start.EventID = toolLifecycleStableEventID(start.ToolCallID, "input")
		}
		if err := publishLifecycleEvent(ctx, connector, opts.DurableRequired, start); err != nil {
			return nil, err
		}
	}

	result, executeErr := execute(ctx)
	if start.StreamID == "" {
		return result, executeErr
	}
	terminal := mapToolLifecycleTerminal(opts.MapTerminal, result, executeErr)
	event := streaming.ToolOutputAvailable
	if terminal.IsError {
		event = streaming.ToolOutputError
	}
	output := terminal.Output
	if terminal.IsError {
		output = nil
	}
	input := streaming.ToolLifecycleInput{
		EventID:          toolLifecycleStableEventID(start.ToolCallID, "terminal"),
		StreamID:         start.StreamID,
		Event:            event,
		ToolCallID:       start.ToolCallID,
		ToolName:         start.ToolName,
		Input:            start.Input,
		Output:           output,
		ErrorText:        terminal.ErrorText,
		Dynamic:          terminal.Dynamic,
		ProviderExecuted: terminal.ProviderExecuted,
		Preliminary:      terminal.Preliminary,
		Metadata:         start.Metadata,
	}
	if err := publishLifecycleEvent(ctx, connector, opts.DurableRequired, input); err != nil {
		return nil, err
	}
	return result, executeErr
}

func mapToolLifecycleTerminal(mapper ToolLifecycleTerminalMapper, result any, err error) ToolLifecycleTerminal {
	if mapper != nil {
		return mapper(result, err)
	}
	if err != nil {
		return ToolLifecycleTerminal{IsError: true, ErrorText: err.Error()}
	}
	return ToolLifecycleTerminal{Output: result}
}

func publishLifecycleEvent(ctx context.Context, connector streaming.Connector, durableRequired bool, input streaming.ToolLifecycleInput) error {
	if durableRequired {
		if err := connector.PersistToolLifecycleEvent(ctx, input); err != nil {
			return err
		}
		if err := connector.PublishLiveToolLifecycleEvent(ctx, input); err != nil {
			logToolLifecycleLiveError(ctx, err)
		}
		return nil
	}
	return connector.PublishToolLifecycleEvent(ctx, input)
}

func toolLifecycleStableEventID(toolCallID string, phase string) string {
	return fmt.Sprintf("tool:%s:%s", toolCallID, phase)
}

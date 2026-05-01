package streaming

import "context"

type NoopConnector struct{}

func (NoopConnector) StartAttempt(context.Context, AttemptRef) error {
	return nil
}

func (NoopConnector) PublishLiveChunk(context.Context, LiveChunk) error {
	return nil
}

func (NoopConnector) UpdateAttemptSnapshot(context.Context, AttemptSnapshot) error {
	return nil
}

func (NoopConnector) CompleteAttempt(context.Context, AttemptCompletion) error {
	return nil
}

func (NoopConnector) PublishToolLifecycleEvent(context.Context, ToolLifecycleInput) error {
	return nil
}

package streaming

import "context"

type Connector interface {
	StartAttempt(context.Context, AttemptRef) error
	PublishLiveChunk(context.Context, LiveChunk) error
	UpdateAttemptSnapshot(context.Context, AttemptSnapshot) error
	CompleteAttempt(context.Context, AttemptCompletion) error
	PublishToolLifecycleEvent(context.Context, ToolLifecycleInput) error
}

type Store interface {
	StartAttempt(context.Context, AttemptRef) error
	PersistEphemeralChunk(context.Context, EphemeralChunk) error
	UpdateAttemptSnapshot(context.Context, AttemptSnapshot) error
	CompleteAttempt(context.Context, AttemptCompletion) error
	PublishToolLifecycleEvent(context.Context, ToolLifecycleInput) error
}

type Publisher interface {
	PublishLiveChunk(context.Context, LiveChunk) error
	PublishToolLifecycleEvent(context.Context, ToolLifecycleInput) error
	PublishAttemptCompletion(context.Context, AttemptCompletion) error
}

type CompositeOptions struct {
	Store                  Store
	Publisher              Publisher
	PersistEphemeralChunks bool
}

type CompositeConnector struct {
	store                  Store
	publisher              Publisher
	persistEphemeralChunks bool
}

func NewCompositeConnector(opts CompositeOptions) *CompositeConnector {
	return &CompositeConnector{
		store:                  opts.Store,
		publisher:              opts.Publisher,
		persistEphemeralChunks: opts.PersistEphemeralChunks,
	}
}

func (c *CompositeConnector) StartAttempt(ctx context.Context, ref AttemptRef) error {
	if c == nil || c.store == nil {
		return nil
	}
	return c.store.StartAttempt(ctx, ref)
}

func (c *CompositeConnector) PublishLiveChunk(ctx context.Context, chunk LiveChunk) error {
	if c == nil {
		return nil
	}
	if c.publisher != nil {
		if err := c.publisher.PublishLiveChunk(ctx, chunk); err != nil {
			return err
		}
	}
	if c.persistEphemeralChunks && c.store != nil {
		return c.store.PersistEphemeralChunk(ctx, chunk)
	}
	return nil
}

func (c *CompositeConnector) UpdateAttemptSnapshot(ctx context.Context, snapshot AttemptSnapshot) error {
	if c == nil || c.store == nil {
		return nil
	}
	return c.store.UpdateAttemptSnapshot(ctx, snapshot)
}

func (c *CompositeConnector) CompleteAttempt(ctx context.Context, completion AttemptCompletion) error {
	if c == nil {
		return nil
	}
	if c.store != nil {
		if err := c.store.CompleteAttempt(ctx, completion); err != nil {
			return err
		}
	}
	if c.publisher != nil {
		return c.publisher.PublishAttemptCompletion(ctx, completion)
	}
	return nil
}

func (c *CompositeConnector) PublishToolLifecycleEvent(ctx context.Context, input ToolLifecycleInput) error {
	if c == nil {
		return nil
	}
	if c.store != nil {
		if err := c.store.PublishToolLifecycleEvent(ctx, input); err != nil {
			return err
		}
	}
	if c.publisher != nil {
		return c.publisher.PublishToolLifecycleEvent(ctx, input)
	}
	return nil
}

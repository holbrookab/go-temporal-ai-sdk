package updates

import "context"

type PreviewStore interface {
	BeginPreview(context.Context, PreviewBeginEvent) error
	CheckpointPreview(context.Context, PreviewSnapshotEvent) error
	EndPreview(context.Context, PreviewEndEvent) error
}

type RecordStore interface {
	UpsertRecord(context.Context, RecordUpsertEvent) error
	EndStream(context.Context, StreamEndEvent) error
}

type LivePublisher interface {
	PublishUpdate(context.Context, UpdateEvent) error
}

type Connector interface {
	PreviewStore
	RecordStore
	LivePublisher
}

type CompositeOptions struct {
	PreviewStore  PreviewStore
	RecordStore   RecordStore
	LivePublisher LivePublisher
}

type CompositeConnector struct {
	previews  PreviewStore
	records   RecordStore
	publisher LivePublisher
}

func NewCompositeConnector(options CompositeOptions) *CompositeConnector {
	return &CompositeConnector{previews: options.PreviewStore, records: options.RecordStore, publisher: options.LivePublisher}
}

func (c *CompositeConnector) BeginPreview(ctx context.Context, event PreviewBeginEvent) error {
	if c != nil && c.previews != nil {
		if err := c.previews.BeginPreview(ctx, event); err != nil {
			return err
		}
	}
	return c.PublishUpdate(ctx, event)
}

func (c *CompositeConnector) CheckpointPreview(ctx context.Context, event PreviewSnapshotEvent) error {
	if c != nil && c.previews != nil {
		if err := c.previews.CheckpointPreview(ctx, event); err != nil {
			return err
		}
	}
	return c.PublishUpdate(ctx, event)
}

func (c *CompositeConnector) EndPreview(ctx context.Context, event PreviewEndEvent) error {
	if c != nil && c.previews != nil {
		if err := c.previews.EndPreview(ctx, event); err != nil {
			return err
		}
	}
	return c.PublishUpdate(ctx, event)
}

func (c *CompositeConnector) UpsertRecord(ctx context.Context, event RecordUpsertEvent) error {
	if c != nil && c.records != nil {
		if err := c.records.UpsertRecord(ctx, event); err != nil {
			return err
		}
	}
	return c.PublishUpdate(ctx, event)
}

func (c *CompositeConnector) EndStream(ctx context.Context, event StreamEndEvent) error {
	if c != nil && c.records != nil {
		if err := c.records.EndStream(ctx, event); err != nil {
			return err
		}
	}
	return c.PublishUpdate(ctx, event)
}

func (c *CompositeConnector) PublishUpdate(ctx context.Context, event UpdateEvent) error {
	if c == nil || c.publisher == nil {
		return nil
	}
	return c.publisher.PublishUpdate(ctx, event)
}

type NoopConnector struct{}

func (NoopConnector) BeginPreview(context.Context, PreviewBeginEvent) error         { return nil }
func (NoopConnector) CheckpointPreview(context.Context, PreviewSnapshotEvent) error { return nil }
func (NoopConnector) EndPreview(context.Context, PreviewEndEvent) error             { return nil }
func (NoopConnector) UpsertRecord(context.Context, RecordUpsertEvent) error         { return nil }
func (NoopConnector) EndStream(context.Context, StreamEndEvent) error               { return nil }
func (NoopConnector) PublishUpdate(context.Context, UpdateEvent) error              { return nil }

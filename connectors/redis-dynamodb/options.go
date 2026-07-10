package redisdynamodb

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/redis/go-redis/v9"
)

const (
	ModePubSub = "pubsub"
	ModeStream = "stream"
	ModeBoth   = "both"

	defaultTTL              = time.Hour
	defaultEventEntity      = "STREAM_EVENT"
	defaultPreviewEntity    = "STREAM_PREVIEW"
	defaultRecordEntity     = "WORKFLOW_RECORD"
	defaultTerminalEntity   = "STREAM_TERMINAL"
	defaultPartitionKeyName = "id"
	defaultSortKeyName      = "createdAt"
	defaultChannelPrefix    = "temporal-ai:live:"
	defaultStreamPrefix     = "temporal-ai:stream:"
	defaultEventStreamKey   = "updateStreamId"
	defaultEventCursorKey   = "updateCursor"
	defaultPreviewStreamKey = "previewStreamId"
	defaultPreviewTimeKey   = "previewUpdatedAt"
	defaultRecordStreamKey  = "recordStreamId"
	defaultRecordTimeKey    = "recordUpdatedAt"
)

type Options struct {
	AWSConfig aws.Config
	DynamoDB  *dynamodb.Client
	Redis     redis.UniversalClient

	TableName               string
	PartitionKeyName        string
	SortKeyName             string
	StateSortKey            int64
	TTL                     time.Duration
	EventEntityType         string
	PreviewEntityType       string
	RecordEntityType        string
	TerminalEntityType      string
	EventStreamKeyName      string
	EventCursorKeyName      string
	PreviewStreamKeyName    string
	PreviewUpdatedAtKeyName string
	RecordStreamKeyName     string
	RecordUpdatedAtKeyName  string

	Resolver Resolver
	Disabled bool

	Mode            string
	ChannelPrefix   string
	StreamPrefix    string
	MaxStreamLength int64
}

type Resolver interface {
	ResolveStream(context.Context, string) (StreamRef, error)
}

type StreamRef struct {
	Channel          string
	RedisStream      string
	ReplayAttributes map[string]any
}

func (o Options) ttl() time.Duration {
	if o.TTL > 0 {
		return o.TTL
	}
	return defaultTTL
}

func (o Options) partitionKeyName() string {
	if o.PartitionKeyName != "" {
		return o.PartitionKeyName
	}
	return defaultPartitionKeyName
}

func (o Options) sortKeyName() string {
	if o.SortKeyName != "" {
		return o.SortKeyName
	}
	return defaultSortKeyName
}

func (o Options) eventEntityType() string {
	if o.EventEntityType != "" {
		return o.EventEntityType
	}
	return defaultEventEntity
}

func (o Options) previewEntityType() string {
	if o.PreviewEntityType != "" {
		return o.PreviewEntityType
	}
	return defaultPreviewEntity
}

func (o Options) recordEntityType() string {
	if o.RecordEntityType != "" {
		return o.RecordEntityType
	}
	return defaultRecordEntity
}

func (o Options) terminalEntityType() string {
	if o.TerminalEntityType != "" {
		return o.TerminalEntityType
	}
	return defaultTerminalEntity
}

func (o Options) stateSortKey() int64 {
	return o.StateSortKey
}

func (o Options) mode() string {
	switch o.Mode {
	case ModeStream, ModeBoth:
		return o.Mode
	default:
		return ModePubSub
	}
}

func (o Options) channelPrefix() string {
	if o.ChannelPrefix != "" {
		return o.ChannelPrefix
	}
	return defaultChannelPrefix
}

func (o Options) streamPrefix() string {
	if o.StreamPrefix != "" {
		return o.StreamPrefix
	}
	return defaultStreamPrefix
}

func (o Options) eventStreamKeyName() string {
	if o.EventStreamKeyName != "" {
		return o.EventStreamKeyName
	}
	return defaultEventStreamKey
}

func (o Options) eventCursorKeyName() string {
	if o.EventCursorKeyName != "" {
		return o.EventCursorKeyName
	}
	return defaultEventCursorKey
}

func (o Options) previewStreamKeyName() string {
	if o.PreviewStreamKeyName != "" {
		return o.PreviewStreamKeyName
	}
	return defaultPreviewStreamKey
}

func (o Options) previewUpdatedAtKeyName() string {
	if o.PreviewUpdatedAtKeyName != "" {
		return o.PreviewUpdatedAtKeyName
	}
	return defaultPreviewTimeKey
}

func (o Options) recordStreamKeyName() string {
	if o.RecordStreamKeyName != "" {
		return o.RecordStreamKeyName
	}
	return defaultRecordStreamKey
}

func (o Options) recordUpdatedAtKeyName() string {
	if o.RecordUpdatedAtKeyName != "" {
		return o.RecordUpdatedAtKeyName
	}
	return defaultRecordTimeKey
}

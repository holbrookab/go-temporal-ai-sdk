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
	defaultAttemptEntity    = "STREAM_ATTEMPT"
	defaultEphemeralEntity  = "STREAM_EPHEMERAL"
	defaultPartitionKeyName = "id"
	defaultSortKeyName      = "createdAt"
	defaultChannelPrefix    = "temporal-ai:live:"
	defaultStreamPrefix     = "temporal-ai:stream:"
)

type Options struct {
	AWSConfig aws.Config
	DynamoDB  *dynamodb.Client
	Redis     redis.UniversalClient

	TableName           string
	PartitionKeyName    string
	SortKeyName         string
	AttemptSortKey      int64
	TTL                 time.Duration
	EventEntityType     string
	AttemptEntityType   string
	EphemeralEntityType string

	Resolver Resolver
	Disabled bool

	Mode                   string
	ChannelPrefix          string
	StreamPrefix           string
	MaxStreamLength        int64
	PersistEphemeralChunks bool
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

func (o Options) attemptEntityType() string {
	if o.AttemptEntityType != "" {
		return o.AttemptEntityType
	}
	return defaultAttemptEntity
}

func (o Options) ephemeralEntityType() string {
	if o.EphemeralEntityType != "" {
		return o.EphemeralEntityType
	}
	return defaultEphemeralEntity
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

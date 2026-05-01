package appsyncdynamodb

import (
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const (
	defaultTTL              = time.Hour
	defaultNamespace        = "chat"
	defaultEventEntity      = "STREAM_EVENT"
	defaultAttemptEntity    = "STREAM_ATTEMPT"
	defaultEphemeralEntity  = "STREAM_EPHEMERAL"
	defaultPartitionKeyName = "id"
	defaultSortKeyName      = "createdAt"
)

type Options struct {
	AWSConfig aws.Config
	DynamoDB  *dynamodb.Client
	Signer    *v4.Signer

	HTTPClient *http.Client

	TableName           string
	AppSyncHTTPDomain   string
	ChannelNamespace    string
	PartitionKeyName    string
	SortKeyName         string
	AttemptSortKey      int64
	TTL                 time.Duration
	EventEntityType     string
	AttemptEntityType   string
	EphemeralEntityType string

	Resolver Resolver
	Disabled bool

	PersistEphemeralChunks bool
}

type Resolver interface {
	ResolveStream(context.Context, string) (StreamRef, error)
}

type StreamRef struct {
	Channel          string
	ReplayAttributes map[string]any
}

func (o Options) ttl() time.Duration {
	if o.TTL > 0 {
		return o.TTL
	}
	return defaultTTL
}

func (o Options) namespace() string {
	if o.ChannelNamespace != "" {
		return trimSlashes(o.ChannelNamespace)
	}
	return defaultNamespace
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

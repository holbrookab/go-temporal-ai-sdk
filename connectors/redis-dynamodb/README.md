# Redis DynamoDB Connector

`connectors/redis-dynamodb` publishes live stream frames through Redis and stores
replayable stream state in DynamoDB.

The import path contains a hyphen for readability, while the Go package name
remains `redisdynamodb` because package identifiers cannot contain hyphens.

```go
import redisdynamodb "github.com/holbrookab/go-temporal-ai-sdk/connectors/redis-dynamodb"
```

## Use

```go
connector := redisdynamodb.New(redisdynamodb.Options{
    AWSConfig: cfg,
    DynamoDB:  dynamodb.NewFromConfig(cfg),
    Redis: redis.NewUniversalClient(&redis.UniversalOptions{
        Addrs: []string{"localhost:6379"},
    }),
    TableName: "chat-production",
    Mode:      redisdynamodb.ModeBoth,
    Resolver: redisdynamodb.NewDynamoDBResolver(redisdynamodb.DynamoDBResolverOptions{
        DynamoDB:  dynamodb.NewFromConfig(cfg),
        TableName: "chat-production",
    }),
})
```

Pass the connector to `activities.Options.StreamConnector` when registering
Temporal activities.

## Modes

- `ModePubSub`: publish live frames to Redis Pub/Sub only.
- `ModeStream`: append live frames to Redis Streams only.
- `ModeBoth`: publish to Pub/Sub and append to Redis Streams.

`ModePubSub` is the default. Use `ModeBoth` when consumers need low-latency live
updates and stream catch-up.

## Behavior

- `PublishLiveChunk` sends provisional provider-live frames to Redis.
- `CompleteAttempt` persists the final attempt status and publishes the terminal
  attempt event.
- Replay data is stored in DynamoDB using `id` and `createdAt` by default.
- `PersistEphemeralChunks` also writes provisional provider-live chunks to
  DynamoDB with a TTL.

## Stream Resolution

The connector uses a `Resolver` to map a Temporal stream ID to:

- the Redis Pub/Sub channel
- the Redis Stream key
- replay attributes copied onto DynamoDB stream records

`NewDynamoDBResolver` reads the stream row from DynamoDB. By default it looks for
`ownerUserId` or `scopeId`, then `parentConversationId` or `conversationId`.
Override field names, prefixes, or formatter functions when your application
uses different routing.

## Required Options

- `DynamoDB` or `AWSConfig`: DynamoDB client/config for replay persistence.
- `Redis`: Redis universal client for live publishing.
- `TableName`: DynamoDB table for stream attempts/events.
- `Resolver`: stream ID resolver for Redis routing and replay attributes.

Optional fields customize table key names, entity type strings, TTL, Redis
prefixes, Redis Stream max length, and whether ephemeral chunks are durably
persisted.

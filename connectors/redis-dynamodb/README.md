# Redis and DynamoDB connector

This adapter implements `updates.Connector`. It publishes the common protocol-v2
JSON envelope through Redis and stores replay state in DynamoDB.

```go
connector := redisdynamodb.New(redisdynamodb.Options{
    AWSConfig: cfg,
    DynamoDB:  dynamodb.NewFromConfig(cfg),
    Redis: redis.NewUniversalClient(&redis.UniversalOptions{
        Addrs: []string{"localhost:6379"},
    }),
    TableName: "chat-production",
    Mode:      redisdynamodb.ModeBoth,
})

acts := activities.New(activities.Options{UpdateConnector: connector})
```

Modes:

- `ModePubSub` publishes low-latency live updates (the default).
- `ModeStream` appends updates to Redis Streams.
- `ModeBoth` does both.

DynamoDB behavior matches the AppSync adapter: bounded preview manifests,
monotonic complete record snapshots, semantic cursor events, exact accepted
attempt supersession, and persisted terminal state. Redis Stream entries carry
`eventId`, `cursor`, `streamId`, and the raw v2 JSON payload.

The default DynamoDB replay attributes are `updateStreamId`/`updateCursor`,
`previewStreamId`/`previewUpdatedAt`, and
`recordStreamId`/`recordUpdatedAt`, matching the TypeScript replay store; their
names are configurable.

The resolver maps `streamId` to Pub/Sub and Redis Stream keys plus DynamoDB
replay attributes. A missing stream returns `updates.ErrStreamNotFound`.

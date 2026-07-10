# AppSync Events and DynamoDB connector

This adapter implements `updates.Connector`. It publishes the common protocol-v2
JSON envelope to AppSync Events and stores replay state in DynamoDB.

```go
connector := appsyncdynamodb.New(appsyncdynamodb.Options{
    AWSConfig:         cfg,
    DynamoDB:          dynamodb.NewFromConfig(cfg),
    TableName:         "chat-production",
    AppSyncHTTPDomain: "example.appsync-api.us-west-2.amazonaws.com",
    Resolver: appsyncdynamodb.NewDynamoDBResolver(appsyncdynamodb.DynamoDBResolverOptions{
        DynamoDB:  dynamodb.NewFromConfig(cfg),
        TableName: "chat-production",
    }),
})

acts := activities.New(activities.Options{UpdateConnector: connector})
```

Behavior:

- `preview-begin`, snapshots, and terminal attempt state update one preview
  manifest keyed by stream and attempt. Failed, canceled, succeeded, and
  superseded manifests use the configured audit `TTL` (one hour by default).
- Preview chunks are live only; replay uses bounded manifests rather than every
  token delta.
- `record-upsert` writes the monotonic current record, then a semantic durable
  event with a store-assigned cursor, then publishes live.
- Idempotent retries reuse the stored cursor. Lower record versions are ignored;
  conflicting reuse of one version returns `updates.ErrRecordConflict`.
- `stream-end` is persisted as both a cursor event and terminal state before it
  is published.

The resolver maps `streamId` to an AppSync channel and replay attributes. A
missing row returns `updates.ErrStreamNotFound`, which the preview relay can
handle only when explicitly configured with `updates.FailurePolicyBestEffort`.

Table key names, state sort key, entity type strings, TTL, HTTP client, and
SigV4 signer are configurable. The default replay attributes are
`updateStreamId`/`updateCursor`, `previewStreamId`/`previewUpdatedAt`, and
`recordStreamId`/`recordUpdatedAt`, matching the TypeScript replay store;
their names are configurable. Canonical record and terminal rows do not use the
preview audit TTL.

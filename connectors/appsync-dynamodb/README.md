# AppSync DynamoDB Connector

`connectors/appsync-dynamodb` publishes live stream frames to AppSync Events and
stores replayable stream state in DynamoDB.

The import path contains a hyphen for readability, while the Go package name
remains `appsyncdynamodb` because package identifiers cannot contain hyphens.

```go
import appsyncdynamodb "github.com/holbrookab/go-temporal-ai-sdk/connectors/appsync-dynamodb"
```

## Use

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
```

Pass the connector to `activities.Options.StreamConnector` when registering
Temporal activities.

## Behavior

- `PublishLiveChunk` signs and posts live frames to AppSync Events.
- `CompleteAttempt` persists the final attempt status and publishes the terminal
  attempt event.
- Replay data is stored in DynamoDB using `id` and `createdAt` by default.
- `PersistEphemeralChunks` also writes provisional provider-live chunks to
  DynamoDB with a TTL.

## Stream Resolution

The connector uses a `Resolver` to map a Temporal stream ID to:

- the AppSync channel to publish to
- replay attributes copied onto DynamoDB stream records

`NewDynamoDBResolver` reads the stream row from DynamoDB. By default it looks for
`ownerUserId` or `scopeId`, then `parentConversationId` or `conversationId`.
Override field names or `ChannelFormatter` when your application uses different
table attributes or channel routing.

## Required Options

- `AWSConfig`: AWS SDK config used when a DynamoDB client is not supplied.
- `TableName`: DynamoDB table for stream attempts/events.
- `AppSyncHTTPDomain`: AppSync Events HTTP domain without a scheme.
- `Resolver`: stream ID resolver for live channel and replay attributes.

Optional fields customize table key names, entity type strings, TTL, HTTP client,
SigV4 signer, and whether ephemeral chunks are durably persisted.

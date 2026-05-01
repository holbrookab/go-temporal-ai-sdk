package appsyncdynamodb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/holbrookab/go-temporal-ai-sdk/streaming"
)

type Connector struct {
	options Options
	ddb     *dynamodb.Client
	signer  *v4.Signer
	http    *http.Client
}

func New(options Options) *Connector {
	ddb := options.DynamoDB
	if ddb == nil {
		ddb = dynamodb.NewFromConfig(options.AWSConfig)
	}
	signer := options.Signer
	if signer == nil {
		signer = v4.NewSigner()
	}
	client := options.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &Connector{options: options, ddb: ddb, signer: signer, http: client}
}

func (c *Connector) StartAttempt(ctx context.Context, ref streaming.AttemptRef) error {
	return c.upsertAttempt(ctx, ref, attemptUpdate{Status: streaming.AttemptActive, Sequence: 0})
}

func (c *Connector) PublishLiveChunk(ctx context.Context, chunk streaming.LiveChunk) error {
	if c == nil {
		return nil
	}
	if err := c.publishLive(ctx, chunk.StreamID, llmStreamChunk(chunk.Event, chunk)); err != nil {
		return err
	}
	if c.options.PersistEphemeralChunks {
		return c.PersistEphemeralChunk(ctx, chunk)
	}
	return nil
}

func (c *Connector) PersistEphemeralChunk(ctx context.Context, chunk streaming.EphemeralChunk) error {
	ref, err := c.resolve(ctx, chunk.StreamID)
	if err != nil {
		return err
	}
	now := time.Now()
	item := c.baseItem(ref, "ephemeral#"+attemptStorageKey(chunk.AttemptRef)+"#"+fmt.Sprint(chunk.Sequence), now)
	item["entityType"] = c.options.ephemeralEntityType()
	item["streamId"] = chunk.StreamID
	item["lane"] = chunk.Lane
	item["attemptId"] = chunk.AttemptID
	item["partId"] = chunk.PartID
	item["toolCallId"] = chunk.ToolCallID
	item["toolName"] = chunk.ToolName
	item["sequence"] = chunk.Sequence
	item["ephemeralAttemptId"] = attemptStorageKey(chunk.AttemptRef)
	item["ephemeralSequence"] = chunk.Sequence
	item["chunk"] = llmStreamChunk(chunk.Event, chunk)
	item["expiresAt"] = now.Add(c.options.ttl()).Unix()
	return c.putItem(ctx, item, "attribute_not_exists(#pk)")
}

func (c *Connector) UpdateAttemptSnapshot(ctx context.Context, snapshot streaming.AttemptSnapshot) error {
	return c.upsertAttempt(ctx, snapshot.AttemptRef, attemptUpdate{
		Status:         streaming.AttemptActive,
		Sequence:       snapshot.Sequence,
		SnapshotText:   snapshot.SnapshotText,
		SnapshotObject: snapshot.SnapshotObject,
	})
}

func (c *Connector) CompleteAttempt(ctx context.Context, completion streaming.AttemptCompletion) error {
	if err := c.upsertAttempt(ctx, completion.AttemptRef, attemptUpdate{
		Status:         completion.Status,
		Sequence:       completion.Sequence,
		SnapshotText:   completion.SnapshotText,
		SnapshotObject: completion.SnapshotObject,
		Reason:         completion.Reason,
	}); err != nil {
		return err
	}
	event := streaming.EventAttemptDiscard
	if completion.Status == streaming.AttemptCommitted {
		event = streaming.EventAttemptCommit
	} else if completion.Status == streaming.AttemptCanceled {
		event = streaming.EventAttemptCancel
	} else if completion.Status == streaming.AttemptFailed {
		event = streaming.EventAttemptFail
	}
	return c.publishChunk(ctx, completion.StreamID, llmStreamChunk(event, completion))
}

func (c *Connector) PublishToolLifecycleEvent(ctx context.Context, input streaming.ToolLifecycleInput) error {
	if input.EventID == "" {
		input.EventID = newEventID()
	}
	if err := c.PersistToolLifecycleEvent(ctx, input); err != nil {
		return err
	}
	return c.PublishLiveToolLifecycleEvent(ctx, input)
}

func (c *Connector) PersistToolLifecycleEvent(ctx context.Context, input streaming.ToolLifecycleInput) error {
	if input.StreamID == "" {
		return nil
	}
	return c.persistEvent(ctx, input.StreamID, toolLifecycleEventID(input), toolLifecycleChunk(input))
}

func (c *Connector) PublishLiveToolLifecycleEvent(ctx context.Context, input streaming.ToolLifecycleInput) error {
	if input.StreamID == "" {
		return nil
	}
	return c.publishLiveEvent(ctx, input.StreamID, toolLifecycleEventID(input), toolLifecycleChunk(input))
}

type attemptUpdate struct {
	Status         streaming.AttemptStatus
	Sequence       int
	SnapshotText   string
	SnapshotObject any
	Reason         string
}

func (c *Connector) upsertAttempt(ctx context.Context, attempt streaming.AttemptRef, update attemptUpdate) error {
	ref, err := c.resolve(ctx, attempt.StreamID)
	if err != nil {
		return err
	}
	now := time.Now()
	key := map[string]any{
		c.options.partitionKeyName(): "attempt#" + attemptStorageKey(attempt),
		c.options.sortKeyName():      c.options.AttemptSortKey,
	}
	values := c.cleanMap(map[string]any{
		":entityType":       c.options.attemptEntityType(),
		":streamId":         attempt.StreamID,
		":phase":            attempt.Phase,
		":lane":             attempt.Lane,
		":attemptId":        attempt.AttemptID,
		":partId":           attempt.PartID,
		":toolCallId":       attempt.ToolCallID,
		":toolName":         attempt.ToolName,
		":status":           update.Status,
		":updatedAt":        now.UnixMilli(),
		":attemptStreamId":  attempt.StreamID,
		":attemptUpdatedAt": now.UnixMilli(),
		":snapshotSequence": update.Sequence,
		":snapshotText":     update.SnapshotText,
		":snapshotObject":   update.SnapshotObject,
		":discardReason":    update.Reason,
		":completedAt":      completedAt(update.Status, now),
		":expiresAt":        now.Add(c.options.ttl()).Unix(),
		":activeStatus":     streaming.AttemptActive,
	})
	for k, v := range ref.ReplayAttributes {
		values[":"+k] = v
	}
	sets := []string{
		"entityType = :entityType",
		"streamId = :streamId",
		"phase = :phase",
		"lane = :lane",
		"attemptId = :attemptId",
		"#status = :status",
		"updatedAt = :updatedAt",
		"attemptStreamId = :attemptStreamId",
		"attemptUpdatedAt = :attemptUpdatedAt",
		"snapshotSequence = :snapshotSequence",
		"expiresAt = :expiresAt",
	}
	for key := range ref.ReplayAttributes {
		if reservedAttemptField(key) {
			continue
		}
		sets = append(sets, key+" = :"+key)
	}
	if values[":partId"] != nil {
		sets = append(sets, "partId = :partId")
	}
	if values[":toolCallId"] != nil {
		sets = append(sets, "toolCallId = :toolCallId")
	}
	if values[":toolName"] != nil {
		sets = append(sets, "toolName = :toolName")
	}
	if values[":snapshotText"] != nil {
		sets = append(sets, "snapshotText = :snapshotText")
	}
	if values[":snapshotObject"] != nil {
		sets = append(sets, "snapshotObject = :snapshotObject")
	}
	if values[":discardReason"] != nil {
		sets = append(sets, "discardReason = :discardReason")
	}
	if values[":completedAt"] != nil {
		sets = append(sets, "completedAt = :completedAt")
	}
	avKey, err := marshalMap(key)
	if err != nil {
		return err
	}
	avValues, err := marshalMap(values)
	if err != nil {
		return err
	}
	condition := "(attribute_not_exists(snapshotSequence) OR snapshotSequence <= :snapshotSequence) AND (attribute_not_exists(#status) OR #status = :activeStatus OR :status <> :activeStatus)"
	_, err = c.ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(c.options.TableName),
		Key:                       avKey,
		UpdateExpression:          aws.String("SET " + strings.Join(sets, ", ")),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: avValues,
		ConditionExpression:       aws.String(condition),
	})
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return nil
	}
	return err
}

func reservedAttemptField(key string) bool {
	switch key {
	case "entityType", "streamId", "phase", "lane", "attemptId", "partId", "toolCallId", "toolName", "status", "updatedAt", "attemptStreamId", "attemptUpdatedAt", "snapshotSequence", "snapshotText", "snapshotObject", "discardReason", "completedAt", "expiresAt":
		return true
	default:
		return false
	}
}

func completedAt(status streaming.AttemptStatus, now time.Time) any {
	if status == streaming.AttemptActive {
		return nil
	}
	return now.UnixMilli()
}

func (c *Connector) publishChunk(ctx context.Context, streamID string, chunk any) error {
	eventID := newEventID()
	if err := c.persistEvent(ctx, streamID, eventID, chunk); err != nil {
		return err
	}
	return c.publishLiveEvent(ctx, streamID, eventID, chunk)
}

func (c *Connector) publishLive(ctx context.Context, streamID string, chunk any) error {
	return c.publishLiveEvent(ctx, streamID, newEventID(), chunk)
}

func (c *Connector) persistEvent(ctx context.Context, streamID string, eventID string, chunk any) error {
	ref, err := c.resolve(ctx, streamID)
	if err != nil {
		return err
	}
	now := time.Now()
	item := c.baseItem(ref, "event#"+streamID+"#"+eventID, now)
	item["entityType"] = c.options.eventEntityType()
	item["streamId"] = streamID
	item["eventId"] = eventID
	item["durableStreamId"] = streamID
	item["durableEventId"] = eventID
	item["chunk"] = chunk
	item["expiresAt"] = now.Add(c.options.ttl()).Unix()
	err = c.putItem(ctx, item, "attribute_not_exists(#pk)")
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return nil
	}
	return err
}

func (c *Connector) publishLiveEvent(ctx context.Context, streamID string, eventID string, chunk any) error {
	if c == nil || c.options.Disabled || c.options.AppSyncHTTPDomain == "" {
		return nil
	}
	ref, err := c.resolve(ctx, streamID)
	if err != nil {
		return err
	}
	channel := ref.Channel
	if channel == "" {
		return fmt.Errorf("stream %q resolved without a channel", streamID)
	}
	eventBytes, err := json.Marshal(map[string]any{"eventId": eventID, "chunk": chunk})
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"channel": channel,
		"events":  []string{string(eventBytes)},
	})
	if err != nil {
		return err
	}
	endpoint := "https://" + strings.TrimPrefix(strings.TrimPrefix(c.options.AppSyncHTTPDomain, "https://"), "http://") + "/event"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	creds, err := c.options.AWSConfig.Credentials.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("retrieving AWS credentials: %w", err)
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	if err := c.signer.SignHTTP(ctx, creds, req, hash, "appsync", c.options.AWSConfig.Region, time.Now()); err != nil {
		return fmt.Errorf("signing AppSync event publish: %w", err)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("publishing AppSync event: %w", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("publishing AppSync event: status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Connector) baseItem(ref StreamRef, id string, now time.Time) map[string]any {
	item := map[string]any{
		c.options.partitionKeyName(): id,
		c.options.sortKeyName():      now.UnixMilli(),
		"createdAt":                  now.UnixMilli(),
		"updatedAt":                  now.UnixMilli(),
	}
	for key, value := range ref.ReplayAttributes {
		item[key] = value
	}
	return item
}

func (c *Connector) putItem(ctx context.Context, item map[string]any, condition string) error {
	av, err := marshalMap(c.cleanMap(item))
	if err != nil {
		return err
	}
	input := &dynamodb.PutItemInput{
		TableName: aws.String(c.options.TableName),
		Item:      av,
		ExpressionAttributeNames: map[string]string{
			"#pk": c.options.partitionKeyName(),
		},
	}
	if condition != "" {
		input.ConditionExpression = aws.String(condition)
	}
	_, err = c.ddb.PutItem(ctx, input)
	return err
}

func (c *Connector) resolve(ctx context.Context, streamID string) (StreamRef, error) {
	if streamID == "" {
		return StreamRef{}, fmt.Errorf("streamId is required")
	}
	if c.options.Resolver != nil {
		return c.options.Resolver.ResolveStream(ctx, streamID)
	}
	ref := StreamRef{
		Channel: "/" + c.options.namespace() + "/" + streamID,
		ReplayAttributes: map[string]any{
			"streamId": streamID,
		},
	}
	return ref, nil
}

func (c *Connector) cleanMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		if value == nil || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func marshalMap(input map[string]any) (map[string]types.AttributeValue, error) {
	return attributevalue.MarshalMapWithOptions(input, func(options *attributevalue.EncoderOptions) {
		options.TagKey = "dynamodbav"
	})
}

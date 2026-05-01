package streaming

import (
	"context"
	"fmt"
	"strings"

	"github.com/holbrookab/go-ai/packages/ai"
)

const (
	defaultSnapshotEveryChunks     = 16
	defaultSnapshotEveryCharacters = 1024
)

type Relay struct {
	connector              Connector
	options                Options
	attempts               map[string]*attemptState
	snapshotEveryChunks    int
	snapshotEveryChars     int
	persistEphemeralChunks bool
}

type attemptState struct {
	ref                    AttemptRef
	sequence               int
	text                   strings.Builder
	lastSnapshotSequence   int
	lastSnapshotTextLength int
}

func NewRelay(connector Connector, options Options) *Relay {
	if connector == nil {
		connector = NoopConnector{}
	}
	snapshotEveryChunks := options.SnapshotEveryChunks
	if snapshotEveryChunks <= 0 {
		snapshotEveryChunks = defaultSnapshotEveryChunks
	}
	snapshotEveryChars := options.SnapshotEveryCharacters
	if snapshotEveryChars <= 0 {
		snapshotEveryChars = defaultSnapshotEveryCharacters
	}
	return &Relay{
		connector:              connector,
		options:                options,
		attempts:               map[string]*attemptState{},
		snapshotEveryChunks:    snapshotEveryChunks,
		snapshotEveryChars:     snapshotEveryChars,
		persistEphemeralChunks: options.PersistEphemeralChunks,
	}
}

func (r *Relay) Accept(ctx context.Context, part ai.StreamPart) error {
	if r == nil || !r.options.Visible || r.options.StreamID == "" {
		return nil
	}
	event, lane, meta, ok := classifyPart(r.options.Lane, part)
	if !ok {
		return nil
	}
	state, err := r.ensureAttempt(ctx, lane, meta)
	if err != nil {
		return err
	}
	state.sequence++
	if meta.delta != "" {
		state.text.WriteString(meta.delta)
	}
	chunk := LiveChunk{
		AttemptRef:   state.ref,
		Event:        event,
		Sequence:     state.sequence,
		Delta:        meta.delta,
		Input:        meta.input,
		Element:      meta.element,
		ProviderPart: part,
	}
	if err := r.connector.PublishLiveChunk(ctx, chunk); err != nil {
		return err
	}
	if r.snapshotDue(state) {
		return r.flushSnapshot(ctx, state)
	}
	return nil
}

func (r *Relay) Commit(ctx context.Context) error {
	return r.complete(ctx, AttemptCommitted, "")
}

func (r *Relay) Discard(ctx context.Context, reason string) error {
	return r.complete(ctx, AttemptDiscarded, reason)
}

func (r *Relay) Cancel(ctx context.Context, reason string) error {
	return r.complete(ctx, AttemptCanceled, reason)
}

func (r *Relay) Fail(ctx context.Context, reason string) error {
	return r.complete(ctx, AttemptFailed, reason)
}

func (r *Relay) complete(ctx context.Context, status AttemptStatus, reason string) error {
	if r == nil || !r.options.Visible || r.options.StreamID == "" {
		return nil
	}
	for _, state := range r.attempts {
		if err := r.flushSnapshot(ctx, state); err != nil {
			return err
		}
		if err := r.connector.CompleteAttempt(ctx, AttemptCompletion{
			AttemptRef: state.ref,
			Sequence:   state.sequence,
			Status:     status,
			Reason:     reason,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Relay) ensureAttempt(ctx context.Context, lane Lane, meta partMeta) (*attemptState, error) {
	key := string(lane)
	if meta.toolCallID != "" {
		key += ":" + meta.toolCallID
	}
	if state := r.attempts[key]; state != nil {
		if state.ref.ToolName == "" && meta.toolName != "" {
			state.ref.ToolName = meta.toolName
		}
		return state, nil
	}
	attemptID := r.options.AttemptID
	if attemptID == "" {
		attemptID = "attempt"
	}
	if meta.toolCallID != "" {
		attemptID = fmt.Sprintf("%s:%s:%s", attemptID, lane, sanitize(meta.toolCallID))
	} else {
		attemptID = fmt.Sprintf("%s:%s", attemptID, lane)
	}
	ref := AttemptRef{
		StreamID:   r.options.StreamID,
		Phase:      PhaseProviderLive,
		Lane:       lane,
		AttemptID:  attemptID,
		PartID:     meta.partID,
		ToolCallID: meta.toolCallID,
		ToolName:   meta.toolName,
	}
	if err := r.connector.StartAttempt(ctx, ref); err != nil {
		return nil, err
	}
	state := &attemptState{ref: ref}
	r.attempts[key] = state
	if err := r.flushSnapshot(ctx, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (r *Relay) snapshotDue(state *attemptState) bool {
	if state == nil {
		return false
	}
	if state.sequence-state.lastSnapshotSequence >= r.snapshotEveryChunks {
		return true
	}
	return state.text.Len()-state.lastSnapshotTextLength >= r.snapshotEveryChars
}

func (r *Relay) flushSnapshot(ctx context.Context, state *attemptState) error {
	if state == nil {
		return nil
	}
	state.lastSnapshotSequence = state.sequence
	state.lastSnapshotTextLength = state.text.Len()
	return r.connector.UpdateAttemptSnapshot(ctx, AttemptSnapshot{
		AttemptRef:   state.ref,
		Sequence:     state.sequence,
		SnapshotText: state.text.String(),
	})
}

type partMeta struct {
	partID     string
	toolCallID string
	toolName   string
	delta      string
	input      any
	element    any
}

func classifyPart(defaultLane Lane, part ai.StreamPart) (Event, Lane, partMeta, bool) {
	textLane := LaneText
	if defaultLane == LaneObject {
		textLane = LaneObject
	}
	switch part.Type {
	case "stream-start":
		return EventStreamStart, textLane, partMeta{partID: part.ID}, true
	case "response-metadata":
		return EventResponseMeta, textLane, partMeta{partID: part.ID}, true
	case "text-delta":
		return EventTextDelta, textLane, partMeta{partID: part.ID, delta: part.TextDelta}, true
	case "reasoning-delta":
		return EventReasoningDelta, LaneReasoning, partMeta{partID: part.ID, delta: part.ReasoningDelta}, true
	case "tool-input-delta":
		return EventToolInputDelta, LaneToolInput, partMeta{partID: part.ID, toolCallID: part.ToolCallID, toolName: part.ToolName, delta: part.ToolInputDelta}, true
	case "tool-input-end":
		return EventToolInputEnd, LaneToolInput, partMeta{partID: part.ID, toolCallID: part.ToolCallID, toolName: part.ToolName, delta: part.ToolInput}, true
	case "tool-call":
		return EventToolCall, LaneToolInput, partMeta{partID: part.ID, toolCallID: part.ToolCallID, toolName: part.ToolName, input: part.ToolInput}, true
	case "element":
		return EventElement, LaneObject, partMeta{partID: part.ID, element: part.Element}, true
	case "file":
		return EventFile, textLane, partMeta{partID: part.ID}, true
	case "source":
		return EventSource, textLane, partMeta{partID: part.ID}, true
	case "finish":
		return EventFinish, textLane, partMeta{partID: part.ID}, true
	case "abort":
		return EventAbort, textLane, partMeta{partID: part.ID}, true
	default:
		return "", "", partMeta{}, false
	}
}

func sanitize(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

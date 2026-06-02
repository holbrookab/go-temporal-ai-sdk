package streaming

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

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
	liveQueue              chan livePublish
	liveOnce               sync.Once
	liveCloseOnce          sync.Once
	liveWG                 sync.WaitGroup
	liveErrMu              sync.Mutex
	liveErr                error
	disabled               bool
}

type attemptState struct {
	ref                    AttemptRef
	sequence               int
	text                   strings.Builder
	object                 any
	lastSnapshotSequence   int
	lastSnapshotTextLength int
}

type livePublish struct {
	ctx   context.Context
	chunk LiveChunk
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
	if r.isDisabled() {
		return nil
	}
	event, lane, meta, ok := classifyPart(r.options.Lane, part)
	if !ok {
		return nil
	}
	meta.scope = mergeScope(r.options.Scope, meta.scope)
	state, err := r.ensureAttempt(ctx, lane, meta)
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	state.sequence++
	if meta.delta != "" {
		state.text.WriteString(meta.delta)
	}
	if meta.hasObject {
		state.object = meta.object
	}
	chunk := LiveChunk{
		AttemptRef:     state.ref,
		Event:          event,
		Sequence:       state.sequence,
		Delta:          meta.delta,
		Input:          meta.input,
		Element:        meta.element,
		SnapshotObject: meta.object,
		ProviderPart:   part,
	}
	if err := r.publishLiveChunk(ctx, chunk); err != nil {
		return err
	}
	if meta.hasObject {
		return r.flushSnapshot(ctx, state)
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
	if r.isDisabled() {
		return nil
	}
	if err := r.flushLiveChunks(); err != nil {
		return err
	}
	if r.isDisabled() {
		return nil
	}
	for _, state := range r.attempts {
		if err := r.flushSnapshot(ctx, state); err != nil {
			return err
		}
		if r.isDisabled() {
			return nil
		}
		if err := r.connector.CompleteAttempt(ctx, AttemptCompletion{
			AttemptRef:     state.ref,
			Sequence:       state.sequence,
			Status:         status,
			Reason:         reason,
			SnapshotText:   state.text.String(),
			SnapshotObject: state.object,
		}); err != nil {
			if r.disableIfBestEffortStreamNotFound(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

func (r *Relay) publishLiveChunk(ctx context.Context, chunk LiveChunk) error {
	if r == nil {
		return nil
	}
	if r.isDisabled() {
		return nil
	}
	if err := r.livePublishError(); err != nil {
		return err
	}
	r.liveOnce.Do(func() {
		r.liveQueue = make(chan livePublish, 128)
		r.liveWG.Add(1)
		go r.publishLiveChunks()
	})
	select {
	case r.liveQueue <- livePublish{ctx: ctx, chunk: chunk}:
		return r.livePublishError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Relay) publishLiveChunks() {
	defer r.liveWG.Done()
	for item := range r.liveQueue {
		if r.isDisabled() {
			continue
		}
		if err := r.connector.PublishLiveChunk(item.ctx, item.chunk); err != nil {
			r.setLivePublishError(err)
		}
	}
}

func (r *Relay) flushLiveChunks() error {
	if r == nil {
		return nil
	}
	r.liveCloseOnce.Do(func() {
		if r.liveQueue != nil {
			close(r.liveQueue)
		}
	})
	r.liveWG.Wait()
	return r.livePublishError()
}

func (r *Relay) setLivePublishError(err error) {
	if err == nil {
		return
	}
	if r.disableIfBestEffortStreamNotFound(err) {
		return
	}
	r.liveErrMu.Lock()
	defer r.liveErrMu.Unlock()
	if r.liveErr == nil {
		r.liveErr = err
	}
}

func (r *Relay) livePublishError() error {
	r.liveErrMu.Lock()
	defer r.liveErrMu.Unlock()
	return r.liveErr
}

func (r *Relay) isDisabled() bool {
	if r == nil {
		return false
	}
	r.liveErrMu.Lock()
	defer r.liveErrMu.Unlock()
	return r.disabled
}

func (r *Relay) disableIfBestEffortStreamNotFound(err error) bool {
	if r == nil || err == nil || r.options.FailurePolicy != StreamFailurePolicyBestEffort {
		return false
	}
	if !errors.Is(err, ErrStreamNotFound) {
		return false
	}
	r.liveErrMu.Lock()
	r.disabled = true
	r.liveErrMu.Unlock()
	return true
}

func (r *Relay) ensureAttempt(ctx context.Context, lane Lane, meta partMeta) (*attemptState, error) {
	if r.isDisabled() {
		return nil, nil
	}
	key := string(lane)
	if meta.scope.StepID != "" {
		key += ":step:" + meta.scope.StepID
	}
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
		Scope:      meta.scope,
	}
	if err := r.connector.StartAttempt(ctx, ref); err != nil {
		if r.disableIfBestEffortStreamNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	state := &attemptState{ref: ref}
	r.attempts[key] = state
	if err := r.flushSnapshot(ctx, state); err != nil {
		return nil, err
	}
	if r.isDisabled() {
		return nil, nil
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
	if r.isDisabled() {
		return nil
	}
	state.lastSnapshotSequence = state.sequence
	state.lastSnapshotTextLength = state.text.Len()
	if err := r.connector.UpdateAttemptSnapshot(ctx, AttemptSnapshot{
		AttemptRef:     state.ref,
		Sequence:       state.sequence,
		SnapshotText:   state.text.String(),
		SnapshotObject: state.object,
	}); err != nil {
		if r.disableIfBestEffortStreamNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

type partMeta struct {
	partID     string
	toolCallID string
	toolName   string
	delta      string
	input      any
	element    any
	object     any
	hasObject  bool
	scope      Scope
}

func classifyPart(defaultLane Lane, part ai.StreamPart) (Event, Lane, partMeta, bool) {
	textLane := LaneText
	if defaultLane == LaneObject {
		textLane = LaneObject
	}
	scope := scopeFromStreamPart(part)
	switch part.Type {
	case "stream-start":
		return EventStreamStart, textLane, partMeta{partID: part.ID, scope: scope}, true
	case "start-step":
		return EventStartStep, textLane, partMeta{partID: part.ID, scope: scope}, true
	case "response-metadata":
		return EventResponseMeta, textLane, partMeta{partID: part.ID, scope: scope}, true
	case "text-delta":
		if part.PartialOutput != nil {
			if defaultLane == LaneObject {
				return EventTextDelta, LaneObject, partMeta{partID: part.ID, object: part.PartialOutput, hasObject: true, scope: scope}, true
			}
			return EventTextDelta, textLane, partMeta{partID: part.ID, delta: part.TextDelta, object: part.PartialOutput, hasObject: true, scope: scope}, true
		}
		if defaultLane == LaneObject {
			return "", "", partMeta{}, false
		}
		return EventTextDelta, textLane, partMeta{partID: part.ID, delta: part.TextDelta, scope: scope}, true
	case "reasoning-delta":
		return EventReasoningDelta, LaneReasoning, partMeta{partID: part.ID, delta: part.ReasoningDelta, scope: scope}, true
	case "tool-input-delta":
		return EventToolInputDelta, LaneToolInput, partMeta{partID: part.ID, toolCallID: part.ToolCallID, toolName: part.ToolName, delta: part.ToolInputDelta, scope: scope}, true
	case "tool-input-end":
		return EventToolInputEnd, LaneToolInput, partMeta{partID: part.ID, toolCallID: part.ToolCallID, toolName: part.ToolName, delta: part.ToolInput, scope: scope}, true
	case "tool-call":
		return EventToolCall, LaneToolInput, partMeta{partID: part.ID, toolCallID: part.ToolCallID, toolName: part.ToolName, input: part.ToolInput, scope: scope}, true
	case "element":
		return EventElement, LaneObject, partMeta{partID: part.ID, element: part.Element, scope: scope}, true
	case "file":
		return EventFile, textLane, partMeta{partID: part.ID, scope: scope}, true
	case "source":
		return EventSource, textLane, partMeta{partID: part.ID, scope: scope}, true
	case "finish-step":
		return EventFinishStep, textLane, partMeta{partID: part.ID, scope: scope}, true
	case "finish":
		return EventFinish, textLane, partMeta{partID: part.ID, scope: scope}, true
	case "abort":
		return EventAbort, textLane, partMeta{partID: part.ID, scope: scope}, true
	default:
		return "", "", partMeta{}, false
	}
}

func scopeFromStreamPart(part ai.StreamPart) Scope {
	scope := Scope{
		StepID:   part.StepID,
		StepType: part.StepType,
	}
	if part.StepID != "" || part.StepType != "" || part.StepNumber != 0 {
		stepNumber := part.StepNumber
		scope.StepNumber = &stepNumber
	}
	return scope
}

func mergeScope(base Scope, override Scope) Scope {
	out := base
	if override.DisplayMode != "" {
		out.DisplayMode = override.DisplayMode
	}
	if override.AgentID != "" {
		out.AgentID = override.AgentID
	}
	if override.TaskID != "" {
		out.TaskID = override.TaskID
	}
	if override.TaskTitle != "" {
		out.TaskTitle = override.TaskTitle
	}
	if override.SkillName != "" {
		out.SkillName = override.SkillName
	}
	if override.StepID != "" {
		out.StepID = override.StepID
	}
	if override.StepNumber != nil {
		out.StepNumber = override.StepNumber
	}
	if override.StepType != "" {
		out.StepType = override.StepType
	}
	return out
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

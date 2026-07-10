package updates

import "fmt"

func NewRecordUpsertEvent(streamID string, record WorkflowRecord, acceptedAttemptID string, occurredAt int64) RecordUpsertEvent {
	return RecordUpsertEvent{
		BaseEvent: BaseEvent{
			ProtocolVersion: ProtocolVersion,
			Type:            EventTypeRecordUpsert,
			EventID:         fmt.Sprintf("%s:v%d", record.RecordID, record.RecordVersion),
			StreamID:        streamID,
			OccurredAt:      occurredAt,
		},
		AcceptedAttemptID: acceptedAttemptID,
		Record:            record,
	}
}

func NewStreamEndEvent(streamID string, outcome StreamOutcome, errorText string, occurredAt int64) StreamEndEvent {
	return StreamEndEvent{
		BaseEvent: BaseEvent{
			ProtocolVersion: ProtocolVersion,
			Type:            EventTypeStreamEnd,
			EventID:         fmt.Sprintf("stream:%s:end:%s", streamID, outcome),
			StreamID:        streamID,
			OccurredAt:      occurredAt,
		},
		Outcome: outcome,
		Error:   errorText,
	}
}

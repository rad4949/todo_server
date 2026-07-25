package outbox

import (
	"encoding/json"
	"time"

	"todo_server/internal/model"
)

type Message struct {
	EventID       string                `json:"event_id"`
	AggregateType string                `json:"aggregate_type"`
	AggregateID   string                `json:"aggregate_id"`
	EventType     model.OutboxEventType `json:"event_type"`
	EventVersion  int                   `json:"event_version"`
	OccurredAt    time.Time             `json:"occurred_at"`
	Payload       json.RawMessage       `json:"payload"`
}

func MessageFromEvent(
	event model.OutboxEvent,
) Message {
	return Message{
		EventID:       event.ID,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		EventVersion:  event.EventVersion,
		OccurredAt:    event.CreatedAt,
		Payload:       event.Payload,
	}
}

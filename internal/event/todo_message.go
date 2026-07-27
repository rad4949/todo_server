package event

import (
	"encoding/json"
	"time"

	"todo_server/internal/model"
)

type TodoMessage struct {
	EventID       string                `json:"event_id"`
	AggregateType string                `json:"aggregate_type"`
	AggregateID   string                `json:"aggregate_id"`
	EventType     model.OutboxEventType `json:"event_type"`
	EventVersion  int                   `json:"event_version"`
	OccurredAt    time.Time             `json:"occurred_at"`
	Payload       json.RawMessage       `json:"payload"`
}
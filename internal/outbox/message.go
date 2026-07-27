package outbox

import (
	"todo_server/internal/model"
	"todo_server/internal/event"
)

type Message = event.TodoMessage

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

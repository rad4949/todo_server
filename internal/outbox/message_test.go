package outbox

import (
	"encoding/json"
	"testing"
	"time"

	"todo_server/internal/model"
)

func TestMessageFromEvent(t *testing.T) {
	createdAt := time.Date(
		2026,
		time.July,
		25,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	payload := json.RawMessage(`{
		"id": "todo-1",
		"title": "Test Kafka message",
		"completed": true,
		"user_id": "user-1",
		"user_email": "user@example.com"
	}`)

	event := model.OutboxEvent{
		ID:            "event-1",
		AggregateType: "todo",
		AggregateID:   "todo-1",
		EventType:     model.OutboxEventTodoUpdated,
		EventVersion:  2,
		Payload:       payload,
		Status:        model.OutboxEventStatusProcessing,
		Attempts:      2,
		CreatedAt:     createdAt,
	}

	message := MessageFromEvent(event)

	if message.EventID != event.ID {
		t.Fatalf(
			"event ID = %q, want %q",
			message.EventID,
			event.ID,
		)
	}

	if message.AggregateType != event.AggregateType {
		t.Fatalf(
			"aggregate type = %q, want %q",
			message.AggregateType,
			event.AggregateType,
		)
	}

	if message.AggregateID != event.AggregateID {
		t.Fatalf(
			"aggregate ID = %q, want %q",
			message.AggregateID,
			event.AggregateID,
		)
	}

	if message.EventType != event.EventType {
		t.Fatalf(
			"event type = %q, want %q",
			message.EventType,
			event.EventType,
		)
	}

	if message.EventVersion != event.EventVersion {
		t.Fatalf(
			"event version = %d, want %d",
			message.EventVersion,
			event.EventVersion,
		)
	}

	if !message.OccurredAt.Equal(event.CreatedAt) {
		t.Fatalf(
			"occurred at = %v, want %v",
			message.OccurredAt,
			event.CreatedAt,
		)
	}

	if string(message.Payload) != string(event.Payload) {
		t.Fatalf(
			"payload = %s, want %s",
			message.Payload,
			event.Payload,
		)
	}
}

func TestMessageJSONDoesNotExposeOutboxState(
	t *testing.T,
) {
	event := model.OutboxEvent{
		ID:            "event-1",
		AggregateType: "todo",
		AggregateID:   "todo-1",
		EventType:     model.OutboxEventTodoUpdated,
		EventVersion:  2,
		Payload: json.RawMessage(
			`{"id":"todo-1"}`,
		),
		Status:   model.OutboxEventStatusProcessing,
		Attempts: 3,
		CreatedAt: time.Date(
			2026,
			time.July,
			25,
			12,
			0,
			0,
			0,
			time.UTC,
		),
	}

	data, err := json.Marshal(
		MessageFromEvent(event),
	)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	var fields map[string]json.RawMessage

	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}

	requiredFields := []string{
		"event_id",
		"aggregate_type",
		"aggregate_id",
		"event_type",
		"event_version",
		"occurred_at",
		"payload",
	}

	for _, field := range requiredFields {
		if _, exists := fields[field]; !exists {
			t.Errorf(
				"required field %q is missing",
				field,
			)
		}
	}

	forbiddenFields := []string{
		"status",
		"attempts",
		"processed_at",
		"next_attempt_at",
		"last_error",
		"locked_at",
		"locked_by",
	}

	for _, field := range forbiddenFields {
		if _, exists := fields[field]; exists {
			t.Errorf(
				"internal field %q is exposed",
				field,
			)
		}
	}
}
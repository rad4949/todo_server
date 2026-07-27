package notification

import (
	"encoding/json"
	"testing"
	"time"
	"strings"

	"todo_server/internal/event"
	"todo_server/internal/model"
)

func TestDecodeTodoNotification(
	t *testing.T,
) {
	userID := "user-123"
	userEmail := "user@example.com"

	payload, err := json.Marshal(
		model.TodoEventPayload{
			ID:        "todo-123",
			Title:     "Learn Kafka consumer",
			Completed: false,
			UserID:    &userID,
			UserEmail: &userEmail,
		},
	)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	message := event.TodoMessage{
		EventID:       "event-123",
		AggregateType: "todo",
		AggregateID:   "todo-123",
		EventType:     model.OutboxEventTodoCreated,
		EventVersion:  model.TodoEventVersion,
		OccurredAt: time.Date(
			2026,
			time.July,
			26,
			12,
			0,
			0,
			0,
			time.UTC,
		),
		Payload: payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	notification, err := DecodeTodoNotification(data)
	if err != nil {
		t.Fatalf(
			"DecodeTodoNotification() error: %v",
			err,
		)
	}

	if notification.Message.EventID != message.EventID {
		t.Errorf(
			"EventID = %q, want %q",
			notification.Message.EventID,
			message.EventID,
		)
	}

	if notification.Message.EventType !=
		model.OutboxEventTodoCreated {
		t.Errorf(
			"EventType = %q, want %q",
			notification.Message.EventType,
			model.OutboxEventTodoCreated,
		)
	}

	if notification.Message.EventVersion !=
		model.TodoEventVersion {
		t.Errorf(
			"EventVersion = %d, want %d",
			notification.Message.EventVersion,
			model.TodoEventVersion,
		)
	}

	if notification.Payload.ID != "todo-123" {
		t.Errorf(
			"Payload.ID = %q, want %q",
			notification.Payload.ID,
			"todo-123",
		)
	}

	if notification.Payload.UserEmail == nil {
		t.Fatal("Payload.UserEmail is nil")
	}

	if *notification.Payload.UserEmail != userEmail {
		t.Errorf(
			"Payload.UserEmail = %q, want %q",
			*notification.Payload.UserEmail,
			userEmail,
		)
	}
}

func todoNotificationTestData(
	t *testing.T,
	version int,
	eventType model.OutboxEventType,
	userEmail *string,
) []byte {
	t.Helper()

	userID := "user-123"

	payload, err := json.Marshal(
		model.TodoEventPayload{
			ID:        "todo-123",
			Title:     "Test notification",
			Completed: false,
			UserID:    &userID,
			UserEmail: userEmail,
		},
	)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	message := event.TodoMessage{
		EventID:       "event-123",
		AggregateType: "todo",
		AggregateID:   "todo-123",
		EventType:     eventType,
		EventVersion:  version,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	return data
}

func TestDecodeTodoNotificationReturnsValidationError(
	t *testing.T,
) {
	email := "user@example.com"

	tests := []struct {
		name      string
		data      func(t *testing.T) []byte
		wantError string
	}{
		{
			name: "invalid JSON",
			data: func(_ *testing.T) []byte {
				return []byte(`{"event_id":`)
			},
			wantError: "decode todo notification message",
		},
		{
			name: "unsupported event version",
			data: func(t *testing.T) []byte {
				return todoNotificationTestData(
					t,
					1,
					model.OutboxEventTodoCreated,
					&email,
				)
			},
			wantError: "unsupported event version 1",
		},
		{
			name: "missing user email",
			data: func(t *testing.T) []byte {
				return todoNotificationTestData(
					t,
					model.TodoEventVersion,
					model.OutboxEventTodoCreated,
					nil,
				)
			},
			wantError: "user email is required",
		},
		{
			name: "unsupported event type",
			data: func(t *testing.T) []byte {
				return todoNotificationTestData(
					t,
					model.TodoEventVersion,
					model.OutboxEventType("TodoArchived"),
					&email,
				)
			},
			wantError: `unsupported event type "TodoArchived"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notification, err := DecodeTodoNotification(
				test.data(t),
			)

			if err == nil {
				t.Fatal(
					"expected DecodeTodoNotification() error, got nil",
				)
			}

			if !strings.Contains(
				err.Error(),
				test.wantError,
			) {
				t.Errorf(
					"error = %q, want it to contain %q",
					err,
					test.wantError,
				)
			}

			if notification.Message.EventID != "" &&
				test.name == "invalid JSON" {
				t.Errorf(
					"unexpected decoded EventID %q",
					notification.Message.EventID,
				)
			}
		})
	}
}
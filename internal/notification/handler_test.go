package notification

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"todo_server/internal/event"
	"todo_server/internal/model"
)

func TestLoggingHandlerHandle(t *testing.T) {
	var logBuffer bytes.Buffer

	logger := slog.New(
		slog.NewJSONHandler(
			&logBuffer,
			nil,
		),
	)

	handler, err := NewLoggingHandler(logger)
	if err != nil {
		t.Fatalf(
			"NewLoggingHandler() error: %v",
			err,
		)
	}

	userEmail := "user@example.com"

	notification := TodoNotification{
		Message: event.TodoMessage{
			EventID:       "event-123",
			AggregateType: "todo",
			AggregateID:   "todo-123",
			EventType:     model.OutboxEventTodoUpdated,
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
		},
		Payload: model.TodoEventPayload{
			ID:        "todo-123",
			Title:     "Updated todo",
			Completed: true,
			UserEmail: &userEmail,
		},
	}

	err = handler.Handle(
		context.Background(),
		notification,
	)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	logOutput := logBuffer.String()

	expectedValues := []string{
		"todo notification received",
		"event-123",
		"TodoUpdated",
		"todo-123",
		"Updated todo",
		"user@example.com",
	}

	for _, expectedValue := range expectedValues {
		if !strings.Contains(
			logOutput,
			expectedValue,
		) {
			t.Errorf(
				"log output does not contain %q:\n%s",
				expectedValue,
				logOutput,
			)
		}
	}
}

func TestLoggingHandlerHandleRequiresUserEmail(t *testing.T) {
	logger := slog.New(
		slog.NewTextHandler(
			&bytes.Buffer{},
			nil,
		),
	)

	handler, err := NewLoggingHandler(logger)
	if err != nil {
		t.Fatalf(
			"NewLoggingHandler() error: %v",
			err,
		)
	}

	notification := TodoNotification{
		Message: event.TodoMessage{
			EventID: "event-123",
		},
		Payload: model.TodoEventPayload{
			ID: "todo-123",
		},
	}

	err = handler.Handle(
		context.Background(),
		notification,
	)
	if err == nil {
		t.Fatal("expected Handle() error, got nil")
	}

	if !strings.Contains(
		err.Error(),
		"user email is required",
	) {
		t.Errorf(
			"error = %q, want email validation error",
			err,
		)
	}
}

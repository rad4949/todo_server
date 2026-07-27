package notification

import (
	"encoding/json"
	"fmt"
	"strings"

	"todo_server/internal/event"
	"todo_server/internal/model"
)

type TodoNotification struct {
	Message event.TodoMessage
	Payload model.TodoEventPayload
}

func DecodeTodoNotification(
	data []byte,
) (TodoNotification, error) {
	var notification TodoNotification

	if len(data) == 0 {
		return notification, fmt.Errorf(
			"decode todo notification: message is empty",
		)
	}

	if err := json.Unmarshal(
		data,
		&notification.Message,
	); err != nil {
		return notification, fmt.Errorf(
			"decode todo notification message: %w",
			err,
		)
	}

	if notification.Message.EventID == "" {
		return notification, fmt.Errorf(
			"decode todo notification: event ID is required",
		)
	}

	if notification.Message.AggregateType != "todo" {
		return notification, fmt.Errorf(
			"decode todo notification: unsupported aggregate type %q",
			notification.Message.AggregateType,
		)
	}

	if notification.Message.AggregateID == "" {
		return notification, fmt.Errorf(
			"decode todo notification: aggregate ID is required",
		)
	}

	if notification.Message.EventVersion != model.TodoEventVersion {
		return notification, fmt.Errorf(
			"decode todo notification: unsupported event version %d",
			notification.Message.EventVersion,
		)
	}

	switch notification.Message.EventType {
	case model.OutboxEventTodoCreated,
		model.OutboxEventTodoUpdated,
		model.OutboxEventTodoDeleted:
		// Підтримувані типи подій.

	default:
		return notification, fmt.Errorf(
			"decode todo notification: unsupported event type %q",
			notification.Message.EventType,
		)
	}

	if err := json.Unmarshal(
		notification.Message.Payload,
		&notification.Payload,
	); err != nil {
		return notification, fmt.Errorf(
			"decode todo notification payload: %w",
			err,
		)
	}

	if notification.Payload.ID == "" {
		return notification, fmt.Errorf(
			"decode todo notification: todo ID is required",
		)
	}

	if notification.Payload.ID != notification.Message.AggregateID {
		return notification, fmt.Errorf(
			"decode todo notification: payload todo ID %q does not match aggregate ID %q",
			notification.Payload.ID,
			notification.Message.AggregateID,
		)
	}

	if notification.Payload.UserEmail == nil ||
		strings.TrimSpace(*notification.Payload.UserEmail) == "" {
		return notification, fmt.Errorf(
			"decode todo notification: user email is required",
		)
	}

	return notification, nil
}
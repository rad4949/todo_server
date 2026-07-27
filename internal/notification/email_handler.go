package notification

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"todo_server/internal/model"
)

type EmailHandler struct {
	sender EmailSender
	logger *slog.Logger
}

var _ Handler = (*EmailHandler)(nil)

func NewEmailHandler(
	sender EmailSender,
	logger *slog.Logger,
) (*EmailHandler, error) {
	if sender == nil {
		return nil, fmt.Errorf(
			"create email handler: sender is required",
		)
	}

	if logger == nil {
		return nil, fmt.Errorf(
			"create email handler: logger is required",
		)
	}

	return &EmailHandler{
		sender: sender,
		logger: logger,
	}, nil
}

func (h *EmailHandler) Handle(
	ctx context.Context,
	notification TodoNotification,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"handle email notification: %w",
			err,
		)
	}

	if notification.Payload.UserEmail == nil {
		return fmt.Errorf(
			"handle email notification: user email is required",
		)
	}

	recipient := strings.TrimSpace(
		*notification.Payload.UserEmail,
	)
	if recipient == "" {
		return fmt.Errorf(
			"handle email notification: user email is required",
		)
	}

	email, err := buildTodoEmail(notification)
	if err != nil {
		return err
	}

	email.To = recipient

	if err := h.sender.Send(ctx, email); err != nil {
		return fmt.Errorf(
			"send email for event %s: %w",
			notification.Message.EventID,
			err,
		)
	}

	h.logger.InfoContext(
		ctx,
		"todo notification email sent",
		"event_id",
		notification.Message.EventID,
		"event_type",
		notification.Message.EventType,
		"todo_id",
		notification.Payload.ID,
		"user_email",
		recipient,
	)

	return nil
}

func buildTodoEmail(
	notification TodoNotification,
) (Email, error) {
	title := strings.TrimSpace(
		notification.Payload.Title,
	)
	if title == "" {
		title = "Untitled todo"
	}

	var action string

	switch notification.Message.EventType {
	case model.OutboxEventTodoCreated:
		action = "created"

	case model.OutboxEventTodoUpdated:
		action = "updated"

	case model.OutboxEventTodoDeleted:
		action = "deleted"

	default:
		return Email{}, fmt.Errorf(
			"build todo email: unsupported event type %q",
			notification.Message.EventType,
		)
	}

	subject := fmt.Sprintf(
		"Todo %s: %s",
		action,
		title,
	)

	textBody := fmt.Sprintf(
		"Your todo was %s.\n\n"+
			"Title: %s\n"+
			"Completed: %t\n"+
			"Todo ID: %s\n"+
			"Event ID: %s\n",
		action,
		title,
		notification.Payload.Completed,
		notification.Payload.ID,
		notification.Message.EventID,
	)

	htmlBody := fmt.Sprintf(
		`<!doctype html>
<html>
<body>
	<h2>Todo %s</h2>
	<p>Your todo was <strong>%s</strong>.</p>
	<ul>
		<li><strong>Title:</strong> %s</li>
		<li><strong>Completed:</strong> %t</li>
		<li><strong>Todo ID:</strong> %s</li>
		<li><strong>Event ID:</strong> %s</li>
	</ul>
</body>
</html>`,
		html.EscapeString(action),
		html.EscapeString(action),
		html.EscapeString(title),
		notification.Payload.Completed,
		html.EscapeString(notification.Payload.ID),
		html.EscapeString(notification.Message.EventID),
	)

	return Email{
		Subject:  subject,
		TextBody: textBody,
		HTMLBody: htmlBody,
	}, nil
}
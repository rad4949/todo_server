package notification

import (
	"context"
	"fmt"
	"log/slog"
)

type Handler interface {
	Handle(
		ctx context.Context,
		notification TodoNotification,
	) error
}

type LoggingHandler struct {
	logger *slog.Logger
}

var _ Handler = (*LoggingHandler)(nil)

func NewLoggingHandler(
	logger *slog.Logger,
) (*LoggingHandler, error) {
	if logger == nil {
		return nil, fmt.Errorf(
			"create notification logging handler: logger is required",
		)
	}

	return &LoggingHandler{
		logger: logger,
	}, nil
}

func (h *LoggingHandler) Handle(
	ctx context.Context,
	notification TodoNotification,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"handle todo notification: %w",
			err,
		)
	}

	if notification.Payload.UserEmail == nil {
		return fmt.Errorf(
			"handle todo notification: user email is required",
		)
	}

	h.logger.InfoContext(
		ctx,
		"todo notification received",
		"event_id",
		notification.Message.EventID,
		"event_type",
		notification.Message.EventType,
		"event_version",
		notification.Message.EventVersion,
		"todo_id",
		notification.Payload.ID,
		"title",
		notification.Payload.Title,
		"completed",
		notification.Payload.Completed,
		"user_email",
		*notification.Payload.UserEmail,
		"occurred_at",
		notification.Message.OccurredAt,
	)

	return nil
}
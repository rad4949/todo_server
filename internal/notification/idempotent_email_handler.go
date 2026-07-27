package notification

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"todo_server/internal/repository"
)

const deliveryStatusUpdateTimeout = 5 * time.Second

type IdempotentEmailHandler struct {
	next       Handler
	deliveries repository.NotificationDeliveryRepository
	logger     *slog.Logger
}

var _ Handler = (*IdempotentEmailHandler)(nil)

func NewIdempotentEmailHandler(
	next Handler,
	deliveries repository.NotificationDeliveryRepository,
	logger *slog.Logger,
) (*IdempotentEmailHandler, error) {
	if next == nil {
		return nil, fmt.Errorf(
			"create idempotent email handler: next handler is required",
		)
	}

	if deliveries == nil {
		return nil, fmt.Errorf(
			"create idempotent email handler: delivery repository is required",
		)
	}

	if logger == nil {
		return nil, fmt.Errorf(
			"create idempotent email handler: logger is required",
		)
	}

	return &IdempotentEmailHandler{
		next:       next,
		deliveries: deliveries,
		logger:     logger,
	}, nil
}

func (h *IdempotentEmailHandler) Handle(
	ctx context.Context,
	notification TodoNotification,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"handle idempotent email notification: %w",
			err,
		)
	}

	eventID := strings.TrimSpace(
		notification.Message.EventID,
	)
	if eventID == "" {
		return fmt.Errorf(
			"handle idempotent email notification: event ID is required",
		)
	}

	eventType := strings.TrimSpace(
		string(notification.Message.EventType),
	)
	if eventType == "" {
		return fmt.Errorf(
			"handle idempotent email notification: event type is required",
		)
	}

	if notification.Payload.UserEmail == nil {
		return fmt.Errorf(
			"handle idempotent email notification: user email is required",
		)
	}

	recipient := strings.TrimSpace(
		*notification.Payload.UserEmail,
	)
	if recipient == "" {
		return fmt.Errorf(
			"handle idempotent email notification: user email is required",
		)
	}

	started, err := h.deliveries.TryStart(
		ctx,
		eventID,
		eventType,
		recipient,
	)
	if err != nil {
		return fmt.Errorf(
			"start email delivery for event %s: %w",
			eventID,
			err,
		)
	}

	if !started {
		h.logger.InfoContext(
			ctx,
			"duplicate email notification skipped",
			"event_id",
			eventID,
			"event_type",
			eventType,
			"user_email",
			recipient,
		)

		return nil
	}

	if err := h.next.Handle(ctx, notification); err != nil {
		if markErr := h.markFailed(
			eventID,
			err.Error(),
		); markErr != nil {
			return fmt.Errorf(
				"handle email for event %s: %v; mark delivery failed: %w",
				eventID,
				err,
				markErr,
			)
		}

		h.logger.ErrorContext(
			ctx,
			"todo notification email failed",
			"event_id",
			eventID,
			"event_type",
			eventType,
			"user_email",
			recipient,
			"error",
			err,
		)

		return fmt.Errorf(
			"handle email for event %s: %w",
			eventID,
			err,
		)
	}

	if err := h.markSent(eventID); err != nil {
		return fmt.Errorf(
			"mark email delivery sent for event %s: %w",
			eventID,
			err,
		)
	}

	h.logger.InfoContext(
		ctx,
		"todo notification delivery recorded",
		"event_id",
		eventID,
		"event_type",
		eventType,
		"user_email",
		recipient,
	)

	return nil
}

func (h *IdempotentEmailHandler) markSent(
	eventID string,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		deliveryStatusUpdateTimeout,
	)
	defer cancel()

	return h.deliveries.MarkSent(
		ctx,
		eventID,
	)
}

func (h *IdempotentEmailHandler) markFailed(
	eventID string,
	lastError string,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		deliveryStatusUpdateTimeout,
	)
	defer cancel()

	return h.deliveries.MarkFailed(
		ctx,
		eventID,
		lastError,
	)
}
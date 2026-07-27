package repository

import (
	"context"

	"todo_server/internal/model"
)

type NotificationDeliveryRepository interface {
	TryStart(
		ctx context.Context,
		eventID string,
		eventType string,
		recipient string,
	) (bool, error)

	MarkSent(
		ctx context.Context,
		eventID string,
	) error

	MarkFailed(
		ctx context.Context,
		eventID string,
		lastError string,
	) error

	GetByEventID(
		ctx context.Context,
		eventID string,
	) (model.NotificationDelivery, error)
}
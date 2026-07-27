package outbox

import (
	"context"

	"todo_server/internal/model"
)

type Publisher interface {
	Publish(
		ctx context.Context,
		event model.OutboxEvent,
	) error
}
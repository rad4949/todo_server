package repository

import (
	"context"
	"database/sql"
	"fmt"

	"todo_server/internal/model"
)

type PostgresOutboxRepository struct {
	db *sql.DB
}

var _ OutboxRepository = (*PostgresOutboxRepository)(nil)

func NewPostgresOutboxRepository(
	db *sql.DB,
) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{
		db: db,
	}
}

func (r *PostgresOutboxRepository) Create(
	ctx context.Context,
	exec DBExecutor,
	event model.OutboxEvent,
) error {
	const query = `
		INSERT INTO outbox_events (
			id,
			aggregate_type,
			aggregate_id,
			event_type,
			event_version,
			payload
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := exec.ExecContext(
		ctx,
		query,
		event.ID,
		event.AggregateType,
		event.AggregateID,
		string(event.EventType),
		event.EventVersion,
		event.Payload,
	)
	if err != nil {
		return fmt.Errorf("create outbox event: %w", err)
	}

	return nil
}

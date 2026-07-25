package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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

func (r *PostgresOutboxRepository) ClaimPending(
	ctx context.Context,
	limit int,
	workerID string,
	staleBefore time.Time,
) ([]model.OutboxEvent, error) {
	if limit <= 0 {
		return nil, fmt.Errorf(
			"claim pending outbox events: limit must be positive",
		)
	}

	if workerID == "" {
		return nil, fmt.Errorf(
			"claim pending outbox events: worker ID is required",
		)
	}

	const query = `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE (
				status = 'pending'
				AND next_attempt_at <= NOW()
			)
			OR (
				status = 'processing'
				AND (
					locked_at IS NULL
					OR locked_at < $1
				)
			)
			ORDER BY created_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events AS event
		SET
			status = 'processing',
			locked_at = NOW(),
			locked_by = $3
		FROM candidates
		WHERE event.id = candidates.id
		RETURNING
			event.id,
			event.aggregate_type,
			event.aggregate_id,
			event.event_type,
			event.event_version,
			event.payload,
			event.status,
			event.attempts,
			event.created_at,
			event.processed_at,
			event.next_attempt_at,
			event.last_error,
			event.locked_at,
			event.locked_by
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		staleBefore,
		limit,
		workerID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"claim pending outbox events: %w",
			err,
		)
	}
	defer rows.Close()

	events := make([]model.OutboxEvent, 0, limit)

	for rows.Next() {
		var event model.OutboxEvent
		var eventType string
		var status string
		var payload []byte

		err := rows.Scan(
			&event.ID,
			&event.AggregateType,
			&event.AggregateID,
			&eventType,
			&event.EventVersion,
			&payload,
			&status,
			&event.Attempts,
			&event.CreatedAt,
			&event.ProcessedAt,
			&event.NextAttemptAt,
			&event.LastError,
			&event.LockedAt,
			&event.LockedBy,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"scan claimed outbox event: %w",
				err,
			)
		}

		event.EventType = model.OutboxEventType(eventType)
		event.Status = model.OutboxEventStatus(status)
		event.Payload = payload

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate claimed outbox events: %w",
			err,
		)
	}

	return events, nil
}

func (r *PostgresOutboxRepository) MarkProcessed(
	ctx context.Context,
	eventID string,
	workerID string,
) error {
	if eventID == "" {
		return fmt.Errorf(
			"mark outbox event processed: event ID is required",
		)
	}

	if workerID == "" {
		return fmt.Errorf(
			"mark outbox event processed: worker ID is required",
		)
	}

	const query = `
		UPDATE outbox_events
		SET
			status = 'processed',
			processed_at = NOW(),
			last_error = NULL,
			locked_at = NULL,
			locked_by = NULL
		WHERE id = $1
		  AND status = 'processing'
		  AND locked_by = $2
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		eventID,
		workerID,
	)
	if err != nil {
		return fmt.Errorf(
			"mark outbox event processed: %w",
			err,
		)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"get marked processed rows count: %w",
			err,
		)
	}

	if rowsAffected == 0 {
		return fmt.Errorf(
			"mark outbox event processed: event is not claimed by worker %q",
			workerID,
		)
	}

	return nil
}

func (r *PostgresOutboxRepository) MarkFailed(
	ctx context.Context,
	eventID string,
	workerID string,
	errorMessage string,
	nextAttemptAt time.Time,
	maxAttempts int,
) error {
	if eventID == "" {
		return fmt.Errorf(
			"mark outbox event failed: event ID is required",
		)
	}

	if workerID == "" {
		return fmt.Errorf(
			"mark outbox event failed: worker ID is required",
		)
	}

	if errorMessage == "" {
		return fmt.Errorf(
			"mark outbox event failed: error message is required",
		)
	}

	if maxAttempts <= 0 {
		return fmt.Errorf(
			"mark outbox event failed: max attempts must be positive",
		)
	}

	const query = `
		UPDATE outbox_events
		SET
			attempts = attempts + 1,
			status = CASE
				WHEN attempts + 1 >= $4
					THEN 'failed'
				ELSE 'pending'
			END,
			last_error = $3,
			next_attempt_at = $5,
			processed_at = NULL,
			locked_at = NULL,
			locked_by = NULL
		WHERE id = $1
		  AND status = 'processing'
		  AND locked_by = $2
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		eventID,
		workerID,
		errorMessage,
		maxAttempts,
		nextAttemptAt,
	)
	if err != nil {
		return fmt.Errorf(
			"mark outbox event failed: %w",
			err,
		)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"get marked failed rows count: %w",
			err,
		)
	}

	if rowsAffected == 0 {
		return fmt.Errorf(
			"mark outbox event failed: event is not claimed by worker %q",
			workerID,
		)
	}

	return nil
}

package repository

import (
	"context"
	"database/sql"
	"time"

	"todo_server/internal/model"
)

type DBExecutor interface {
	ExecContext(
		ctx context.Context,
		query string,
		args ...any,
	) (sql.Result, error)
}

type OutboxRepository interface {
	Create(
		ctx context.Context,
		exec DBExecutor,
		event model.OutboxEvent,
	) error

	ClaimPending(
		ctx context.Context,
		limit int,
		workerID string,
		staleBefore time.Time,
	) ([]model.OutboxEvent, error)

	MarkProcessed(
		ctx context.Context,
		eventID string,
		workerID string,
	) error

	MarkFailed(
		ctx context.Context,
		eventID string,
		workerID string,
		errorMessage string,
		nextAttemptAt time.Time,
		maxAttempts int,
	) error
}

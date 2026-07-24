package repository

import (
	"context"
	"database/sql"

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
}
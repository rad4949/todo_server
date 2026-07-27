package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"todo_server/internal/model"

	"github.com/google/uuid"
)

type PostgresTodoRepository struct {
	db     *sql.DB
	outbox OutboxRepository
}

func NewPostgresTodoRepository(
	db *sql.DB,
	outbox OutboxRepository,
) *PostgresTodoRepository {
	return &PostgresTodoRepository{
		db:     db,
		outbox: outbox,
	}
}

func newTodoOutboxEvent(
	todo model.Todo,
	userEmail *string,
	eventType model.OutboxEventType,
) (model.OutboxEvent, error) {
	payload, err := json.Marshal(model.TodoEventPayload{
		ID:        todo.ID,
		Title:     todo.Title,
		Completed: todo.Completed,
		UserID:    todo.UserID,
		UserEmail: userEmail,
	})
	if err != nil {
		return model.OutboxEvent{}, fmt.Errorf(
			"marshal todo event payload: %w",
			err,
		)
	}

	return model.OutboxEvent{
		ID:            uuid.NewString(),
		AggregateType: "todo",
		AggregateID:   todo.ID,
		EventType:     eventType,
		EventVersion:  model.TodoEventVersion,
		Payload:       payload,
		Status:        model.OutboxEventStatusPending,
	}, nil
}

func getUserEmail(
	ctx context.Context,
	tx *sql.Tx,
	userID *string,
) (*string, error) {
	if userID == nil {
		return nil, nil
	}

	const query = `
		SELECT email
		FROM users
		WHERE id = $1
	`

	var email string

	err := tx.QueryRowContext(
		ctx,
		query,
		*userID,
	).Scan(&email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf(
				"user not found for todo event: %w",
				err,
			)
		}

		return nil, fmt.Errorf(
			"get user email for todo event: %w",
			err,
		)
	}

	return &email, nil
}

func (r *PostgresTodoRepository) Create(ctx context.Context, title string, userID *string) (model.Todo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Todo{}, fmt.Errorf(
			"begin create todo transaction: %w",
			err,
		)
	}
	defer tx.Rollback()

	id := uuid.New().String()

	query := `
		INSERT INTO todos (id, title, completed, user_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, title, completed, user_id
	`

	var todo model.Todo

	err = tx.QueryRowContext(ctx, query, id, title, false, userID).
		Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.UserID)
	if err != nil {
		return model.Todo{}, fmt.Errorf("create todo: %w", err)
	}

	userEmail, err := getUserEmail(
		ctx,
		tx,
		todo.UserID,
	)
	if err != nil {
		return model.Todo{}, err
	}

	event, err := newTodoOutboxEvent(
		todo,
		userEmail,
		model.OutboxEventTodoCreated,
	)
	if err != nil {
		return model.Todo{}, err
	}

	if err := r.outbox.Create(ctx, tx, event); err != nil {
		return model.Todo{}, fmt.Errorf(
			"create TodoCreated outbox event: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return model.Todo{}, fmt.Errorf(
			"commit create todo transaction: %w",
			err,
		)
	}

	return todo, nil
}

func (r *PostgresTodoRepository) GetAll(ctx context.Context) ([]model.Todo, error) {
	query := `
		SELECT id, title, completed, user_id
		FROM todos
		ORDER BY title
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all todos: %w", err)
	}
	defer rows.Close()

	todos := []model.Todo{}

	for rows.Next() {
		var todo model.Todo
		err := rows.Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.UserID)
		if err != nil {
			return nil, fmt.Errorf("scan todo: %w", err)
		}
		todos = append(todos, todo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate todos: %w", err)
	}

	return todos, nil
}

func (r *PostgresTodoRepository) GetByID(ctx context.Context, id string) (*model.Todo, error) {
	query := `
		SELECT id, title, completed, user_id
		FROM todos
		WHERE id = $1
	`

	var todo model.Todo
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	return &todo, nil
}

func (r *PostgresTodoRepository) Update(ctx context.Context, id string, title string, completed bool) (*model.Todo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"begin update todo transaction: %w",
			err,
		)
	}
	defer tx.Rollback()

	const query = `
		UPDATE todos
		SET title = $1,
		    completed = $2
		WHERE id = $3
		RETURNING id, title, completed, user_id
	`

	var todo model.Todo

	err = tx.QueryRowContext(
		ctx,
		query,
		title,
		completed,
		id,
	).Scan(
		&todo.ID,
		&todo.Title,
		&todo.Completed,
		&todo.UserID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf(
				"todo not found: %w",
				err,
			)
		}

		return nil, fmt.Errorf(
			"update todo: %w",
			err,
		)
	}

	userEmail, err := getUserEmail(
		ctx,
		tx,
		todo.UserID,
	)
	if err != nil {
		return nil, err
	}

	event, err := newTodoOutboxEvent(
		todo,
		userEmail,
		model.OutboxEventTodoUpdated,
	)
	if err != nil {
		return nil, err
	}

	if err := r.outbox.Create(ctx, tx, event); err != nil {
		return nil, fmt.Errorf(
			"create TodoUpdated outbox event: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf(
			"commit update todo transaction: %w",
			err,
		)
	}

	return &todo, nil
}

func (r *PostgresTodoRepository) Delete(
	ctx context.Context,
	id string,
) (*model.Todo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf(
			"begin delete todo transaction: %w",
			err,
		)
	}
	defer tx.Rollback()

	const query = `
		DELETE FROM todos
		WHERE id = $1
		RETURNING id, title, completed, user_id
	`

	var todo model.Todo

	err = tx.QueryRowContext(
		ctx,
		query,
		id,
	).Scan(
		&todo.ID,
		&todo.Title,
		&todo.Completed,
		&todo.UserID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf(
				"todo not found: %w",
				err,
			)
		}

		return nil, fmt.Errorf(
			"delete todo: %w",
			err,
		)
	}

	userEmail, err := getUserEmail(
		ctx,
		tx,
		todo.UserID,
	)
	if err != nil {
		return nil, err
	}

	event, err := newTodoOutboxEvent(
		todo,
		userEmail,
		model.OutboxEventTodoDeleted,
	)
	if err != nil {
		return nil, err
	}

	if err := r.outbox.Create(ctx, tx, event); err != nil {
		return nil, fmt.Errorf(
			"create TodoDeleted outbox event: %w",
			err,
		)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf(
			"commit delete todo transaction: %w",
			err,
		)
	}

	return &todo, nil
}

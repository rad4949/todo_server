package repository

import (
	"database/sql"
	"fmt"
	"todo_server/internal/model"

	"github.com/google/uuid"
)

type PostgresTodoRepository struct {
	db *sql.DB
}

func NewPostgresTodoRepository(db *sql.DB) *PostgresTodoRepository {
	return &PostgresTodoRepository{
		db: db,
	}
}

func (r *PostgresTodoRepository) Create(title string, userID *string) (model.Todo, error) {
	id := uuid.New().String()

	query := `
		INSERT INTO todos (id, title, completed, user_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, title, completed, user_id
	`

	var todo model.Todo
	err := r.db.QueryRow(query, id, title, false, userID).
		Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.UserID)
	if err != nil {
		return model.Todo{}, fmt.Errorf("create todo: %w", err)
	}

	return todo, nil
}

func (r *PostgresTodoRepository) GetAll() []model.Todo {
	query := `
		SELECT id, title, completed, user_id
		FROM todos
		ORDER BY title
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return []model.Todo{}
	}
	defer rows.Close()

	todos := []model.Todo{}

	for rows.Next() {
		var todo model.Todo
		err := rows.Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.UserID)
		if err != nil {
			return []model.Todo{}
		}
		todos = append(todos, todo)
	}

	return todos
}

func (r *PostgresTodoRepository) GetByID(id string) (*model.Todo, error) {
	query := `
		SELECT id, title, completed, user_id
		FROM todos
		WHERE id = $1
	`

	var todo model.Todo
	err := r.db.QueryRow(query, id).
		Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	return &todo, nil
}

func (r *PostgresTodoRepository) Update(id string, title string, completed bool) (*model.Todo, error) {
	query := `
		UPDATE todos
		SET title = $1, completed = $2
		WHERE id = $3
		RETURNING id, title, completed, user_id
	`

	var todo model.Todo
	err := r.db.QueryRow(query, title, completed, id).
		Scan(&todo.ID, &todo.Title, &todo.Completed, &todo.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get todo: %w", err)
	}

	return &todo, nil
}

func (r *PostgresTodoRepository) Delete(id string) error {
	query := `
		DELETE FROM todos
		WHERE id = $1
	`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete todo: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check deleted row: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("not found: %w", err)
	}

	return nil
}
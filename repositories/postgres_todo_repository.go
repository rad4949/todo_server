package repositories

import (
	"database/sql"
	"errors"
	"todo_server/models"

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

func (r *PostgresTodoRepository) Create(title string) (models.Todo, error) {
	id := uuid.New().String()

	todo := models.Todo{
		ID:        id,
		Title:     title,
		Completed: false,
	}
	query := `
		INSERT INTO todos (id, title, completed)
		VALUES ($1, $2, $3)
	`

	_, err := r.db.Exec(query, todo.ID, todo.Title, todo.Completed)
	if err != nil {
		return models.Todo{}, errors.New("failed to create todo")
	}

	return todo, nil
}

func (r *PostgresTodoRepository) GetAll() []models.Todo {
	query := `
		SELECT id, title, completed
		FROM todos
		ORDER BY title
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return []models.Todo{}
	}
	defer rows.Close()

	todos := []models.Todo{}

	for rows.Next() {
		var todo models.Todo

		err := rows.Scan(&todo.ID, &todo.Title, &todo.Completed)
		if err != nil {
			return []models.Todo{}
		}

		todos = append(todos, todo)
	}

	return todos
}

func (r *PostgresTodoRepository) GetByID(id string) (*models.Todo, error) {
	query := `
		SELECT id, title, completed
		FROM todos
		WHERE id = $1
	`

	var todo models.Todo

	err := r.db.QueryRow(query, id).Scan(&todo.ID, &todo.Title, &todo.Completed)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("todo not found")
		}
		return nil, errors.New("failed to get todo")
	}

	return &todo, nil
}

func (r *PostgresTodoRepository) Update(id string, title string, completed bool) (*models.Todo, error) {
	query := `
		UPDATE todos
		SET title = $1, completed = $2
		WHERE id = $3
		RETURNING id, title, completed
	`

	var todo models.Todo

	err := r.db.QueryRow(query, title, completed, id).Scan(&todo.ID, &todo.Title, &todo.Completed)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("todo not found")
		}
		return nil, errors.New("failed to update todo")
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
		return errors.New("failed to delete todo")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.New("failed to check deleted rows")
	}

	if rowsAffected == 0 {
		return errors.New("todo not found")
	}

	return nil
}

package repository

import (
	"context"
	"errors"

	"todo_server/internal/model"

	"github.com/google/uuid"
)

type InMemoryTodoRepository struct {
	todos map[string]model.Todo
}

func NewInMemoryTodoRepository() TodoRepository {
	return &InMemoryTodoRepository{
		todos: make(map[string]model.Todo),
	}
}

func (r *InMemoryTodoRepository) GetAll(_ context.Context) ([]model.Todo, error) {
	todos := make([]model.Todo, 0, len(r.todos))
	for _, todo := range r.todos {
		todos = append(todos, todo)
	}
	return todos, nil
}

func (r *InMemoryTodoRepository) GetByID(_ context.Context, id string) (*model.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, errors.New("todo with this ID not found")
	}
	return &todo, nil
}

func (r *InMemoryTodoRepository) Create(_ context.Context, title string, userID *string) (model.Todo, error) {
	id := uuid.New().String()
	todo := model.Todo{
		ID:        id,
		Title:     title,
		Completed: false,
		UserID:    userID,
	}
	r.todos[id] = todo
	return todo, nil
}

func (r *InMemoryTodoRepository) Update(_ context.Context, id string, title string, completed bool) (*model.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, errors.New("todo with this ID not found")
	}
	todo.Title = title
	todo.Completed = completed
	r.todos[id] = todo
	return &todo, nil
}

func (r *InMemoryTodoRepository) Delete(_ context.Context, id string) (*model.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, errors.New("todo with this ID not found")
	}
	delete(r.todos, id)
	return &todo, nil
}

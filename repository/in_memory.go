package repository

import (
	"errors"
	"fmt"
	"todo_server/model"

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

func (r *InMemoryTodoRepository) GetAll() []model.Todo {
	todos := make([]model.Todo, 0, len(r.todos))

	for _, todo := range r.todos {
		todos = append(todos, todo)
	}

	return todos
}

func (r *InMemoryTodoRepository) GetByID(id string) (*model.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, fmt.Errorf("todo with this ID not found: %w", exists)
	}

	return &todo, nil
}

func (r *InMemoryTodoRepository) Create(title string) (model.Todo, error) {
	id := uuid.New().String()
	if _, exist := r.todos[id]; exist {
		return model.Todo{}, fmt.Errorf("todo with this ID already exists: %w", exist)
	}

	todo := model.Todo{
		ID:        id,
		Title:     title,
		Completed: false,
	}

	r.todos[id] = todo

	return todo, nil
}

func (r *InMemoryTodoRepository) Update(id string, title string, completed bool) (*model.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, fmt.Errorf("todo with this ID not found: %w", exists)
	}

	todo.Title = title
	todo.Completed = completed

	r.todos[id] = todo

	return &todo, nil
}

func (r *InMemoryTodoRepository) Delete(id string) error {
	if _, exist := r.todos[id]; !exist {
		return fmt.Errorf("todo with this ID not found: %w", exist)
	}

	delete(r.todos, id)

	return nil
}

package repositories

import (
	"errors"
	"todo_server/models"

	"github.com/google/uuid"
)

type InMemoryTodoRepository struct {
	todos map[string]models.Todo
}

func NewInMemoryTodoRepository() TodoRepository {
	return &InMemoryTodoRepository{
		todos: make(map[string]models.Todo),
	}
}

func (r *InMemoryTodoRepository) GetAll() []models.Todo {
	todos := make([]models.Todo, 0, len(r.todos))

	for _, todo := range r.todos {
		todos = append(todos, todo)
	}

	return todos
}

func (r *InMemoryTodoRepository) GetByID(id string) (*models.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, errors.New("todo with this ID not found")
	}

	return &todo, nil
}

func (r *InMemoryTodoRepository) Create(title string) (models.Todo, error) {
	id := uuid.New().String()
	if _, exist := r.todos[id]; exist {
		return models.Todo{}, errors.New("todo with this ID already exists")
	}

	todo := models.Todo{
		ID:        id,
		Title:     title,
		Completed: false,
	}

	r.todos[id] = todo

	return todo, nil
}

func (r *InMemoryTodoRepository) Update(id string, title string, completed bool) (*models.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, errors.New("todo with this ID not found")
	}

	todo.Title = title
	todo.Completed = completed

	r.todos[id] = todo

	return &todo, nil
}

func (r *InMemoryTodoRepository) Delete(id string) error {
	if _, exist := r.todos[id]; !exist {
		return errors.New("todo not found")
	}

	delete(r.todos, id)

	return nil
}

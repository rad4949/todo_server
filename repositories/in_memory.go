package repositories

import (
	"errors"
	"todo_server/models"
)

type InMemoryTodoRepository struct {
	todos  map[int]models.Todo
	nextID int
}

func NewInMemoryTodoRepository() TodoRepository {
	return &InMemoryTodoRepository{
		todos:  make(map[int]models.Todo),
		nextID: 1,
	}
}

func (r *InMemoryTodoRepository) GetAll() []models.Todo {
	todos := make([]models.Todo, 0, len(r.todos))

	for _, todo := range r.todos {
		todos = append(todos, todo)
	}

	return todos
}

func (r *InMemoryTodoRepository) GetByID(id int) (*models.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, errors.New("todo with this ID not found")
	}

	return &todo, nil
}

func (r *InMemoryTodoRepository) Create(title string) (models.Todo, error) {
	id := r.nextID
	if _, exist := r.todos[id]; exist {
		return models.Todo{}, errors.New("todo with this ID already exists")
	}

	todo := models.Todo{
		ID:        r.nextID,
		Title:     title,
		Completed: false,
	}

	r.todos[r.nextID] = todo
	r.nextID++

	return todo, nil
}

func (r *InMemoryTodoRepository) Update(id int, title string, completed bool) (*models.Todo, error) {
	todo, exists := r.todos[id]
	if !exists {
		return nil, errors.New("todo with this ID not found")
	}

	todo.Title = title
	todo.Completed = completed

	r.todos[id] = todo

	return &todo, nil
}

func (r *InMemoryTodoRepository) Delete(id int) error {
	if _, exist := r.todos[id]; !exist {
		return errors.New("todo not found")
	}

	delete(r.todos, id)

	return nil
}

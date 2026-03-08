package services

import (
	"errors"
	"todo_server/models"
)

type TodoService struct {
	todos  []models.Todo
	nextID int
}

func NewTodoService() *TodoService {
	return &TodoService{
		todos:  []models.Todo{},
		nextID: 1,
	}
}

func (s *TodoService) GetAll() []models.Todo {
	return s.todos
}

func (s *TodoService) GetByID(id int) (*models.Todo, error) {
	for i := range s.todos {
		if s.todos[i].ID == id {
			return &s.todos[i], nil
		}
	}
	return nil, errors.New("todo not found")
}

func (s *TodoService) Create(title string) models.Todo {
	todo := models.Todo{
		ID:        s.nextID,
		Title:     title,
		Completed: false,
	}
	s.todos = append(s.todos, todo)
	s.nextID++
	return todo
}

func (s *TodoService) Update(id int, title string, completed bool) (*models.Todo, error) {
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos[i].Title = title
			s.todos[i].Completed = completed
			return &s.todos[i], nil
		}
	}
	return nil, errors.New("todo not found")
}

func (s *TodoService) Delete(id int) error {
	for i := range s.todos {
		if s.todos[i].ID == id {
			s.todos = append(s.todos[:i], s.todos[i+1:]...)
			return nil
		}
	}
	return errors.New("todo not found")
}

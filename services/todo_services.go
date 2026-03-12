package services

import (
	"errors"
	"todo_server/models"
)

// перенести залежності в окремий пакет
type TodoService struct {
	todos  map[int]models.Todo
	nextID int
}

// передавати репо в конструктор
func NewTodoService() *TodoService {
	return &TodoService{
		todos:  make(map[int]models.Todo),
		nextID: 1,
	}
}

func (s *TodoService) GetAll() map[int]models.Todo {
	return s.todos
}

func (s *TodoService) GetByID(id int) (*models.Todo, error) {
	todo, ok := s.todos[id]
	if !ok {
		return nil, errors.New("todo not found")
	}
	return &todo, nil
}

func (s *TodoService) Create(title string) (models.Todo, error) {
	id := s.nextID
	if _, exist := s.todos[id]; exist {
		return models.Todo{}, errors.New("todo with this ID already exists")
	}

	todo := models.Todo{
		ID:        id,
		Title:     title,
		Completed: false,
	}

	s.todos[id] = todo
	s.nextID++

	return todo, nil
}

func (s *TodoService) Update(id int, title string, completed bool) (*models.Todo, error) {
	todo, exist := s.todos[id]
	if !exist {
		return nil, errors.New("todo not found")
	}

	todo.Title = title
	todo.Completed = completed

	s.todos[id] = todo

	return &todo, nil
}

func (s *TodoService) Delete(id int) error {
	if _, exist := s.todos[id]; !exist {
		return errors.New("todo not found")
	}

	delete(s.todos, id)

	return nil
}

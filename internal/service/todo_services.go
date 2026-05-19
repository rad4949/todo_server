package service

import (
	"todo_server/internal/model"
	"todo_server/internal/repository"
)

type TodoService struct {
	repo repository.TodoRepository
}

func NewTodoService(repo repository.TodoRepository) *TodoService {
	return &TodoService{
		repo: repo,
	}
}

func (s *TodoService) GetAll() []model.Todo {
	return s.repo.GetAll()
}

func (s *TodoService) GetByID(id string) (*model.Todo, error) {
	return s.repo.GetByID(id)
}

func (s *TodoService) Create(title string, userID *string) (model.Todo, error) { // ← додали userID *string
	return s.repo.Create(title, userID)
}

func (s *TodoService) Update(id string, title string, completed bool) (*model.Todo, error) {
	return s.repo.Update(id, title, completed)
}

func (s *TodoService) Delete(id string) error {
	return s.repo.Delete(id)
}
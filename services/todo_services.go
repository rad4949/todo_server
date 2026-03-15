package services

import (
	"todo_server/models"
	"todo_server/repositories"
)

type TodoService struct {
	repo repositories.TodoRepository
}

func NewTodoService(repo repositories.TodoRepository) *TodoService {
	return &TodoService{
		repo: repo,
	}
}

func (s *TodoService) GetAll() []models.Todo {
	return s.repo.GetAll()
}

func (s *TodoService) GetByID(id int) (*models.Todo, error) {
	return s.repo.GetByID(id)
}

func (s *TodoService) Create(title string) (models.Todo, error) {
	return s.repo.Create(title)
}

func (s *TodoService) Update(id int, title string, completed bool) (*models.Todo, error) {
	return s.repo.Update(id, title, completed)
}

func (s *TodoService) Delete(id int) error {
	return s.repo.Delete(id)
}

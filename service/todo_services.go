package service

import (
	"todo_server/model"
	"todo_server/repository"
)

// test: unit, integration, e2e
// connect Postgre
// create config
// using for e2e test containers
// create docker file for project
// + use uuid for id
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

func (s *TodoService) Create(title string) (model.Todo, error) {
	return s.repo.Create(title)
}

func (s *TodoService) Update(id string, title string, completed bool) (*model.Todo, error) {
	return s.repo.Update(id, title, completed)
}

func (s *TodoService) Delete(id string) error {
	return s.repo.Delete(id)
}

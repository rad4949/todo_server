package service

import (
	"context"
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

func (s *TodoService) GetAll(ctx context.Context) ([]model.Todo, error) {
	return s.repo.GetAll(ctx)
}

func (s *TodoService) GetByID(ctx context.Context, id string) (*model.Todo, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TodoService) Create(ctx context.Context, title string, userID *string) (model.Todo, error) {
	return s.repo.Create(ctx, title, userID)
}

func (s *TodoService) Update(ctx context.Context, id string, title string, completed bool) (*model.Todo, error) {
	return s.repo.Update(ctx, id, title, completed)
}

func (s *TodoService) Delete(ctx context.Context, id string) (*model.Todo, error) {
	return s.repo.Delete(ctx, id)
}

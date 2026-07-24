package repository

import (
	"context"
	"todo_server/internal/model"
)

type TodoRepository interface {
	GetAll(ctx context.Context) ([]model.Todo, error)
	GetByID(ctx context.Context, id string) (*model.Todo, error)
	Create(ctx context.Context, title string, userID *string) (model.Todo, error)
	Update(ctx context.Context, id string, title string, completed bool) (*model.Todo, error)
	Delete(ctx context.Context, id string) (*model.Todo, error)
}

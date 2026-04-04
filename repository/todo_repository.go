package repository

import (
	"todo_server/model"
)

type TodoRepository interface {
	GetAll() []model.Todo
	GetByID(id string) (*model.Todo, error)
	Create(title string) (model.Todo, error)
	Update(id string, title string, completed bool) (*model.Todo, error)
	Delete(id string) error
}

package repositories

import (
	"todo_server/models"
)

type TodoRepository interface {
	GetAll() []models.Todo
	GetByID(id string) (*models.Todo, error)
	Create(title string) (models.Todo, error)
	Update(id string, title string, completed bool) (*models.Todo, error)
	Delete(id string) error
}

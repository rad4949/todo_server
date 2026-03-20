package repositories

import (
	"todo_server/models"
)

type TodoRepository interface {
	GetAll() []models.Todo
	GetByID(id int) (*models.Todo, error)
	Create(title string) (models.Todo, error)
	Update(id int, title string, completed bool) (*models.Todo, error)
	Delete(id int) error
}

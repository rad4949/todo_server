package repository

import "todo_server/internal/model"

type UserRepository interface {
	Create(id, username, email, hashedPassword string) (model.User, error)
	GetAll() ([]model.User, error)
	GetByID(id string) (*model.User, error)
	GetByUsername(username string) (*model.User, error) // для логіну
	Update(id, username, email string) (*model.User, error)
	Delete(id string) error
}
package service

import (
	"fmt"
	"todo_server/internal/model"
	"todo_server/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Register(username, email, password string) (model.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}

	id := uuid.New().String()

	return s.repo.Create(id, username, email, string(hashedPassword))
}

func (s *UserService) GetAll() ([]model.User, error) {
	return s.repo.GetAll()
}

func (s *UserService) GetByID(id string) (*model.User, error) {
	return s.repo.GetByID(id)
}

func (s *UserService) Update(id, username, email string) (*model.User, error) {
	return s.repo.Update(id, username, email)
}

func (s *UserService) Delete(id string) error {
	return s.repo.Delete(id)
}

func (s *UserService) Authenticate(username, password string) (*model.User, error) {
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}
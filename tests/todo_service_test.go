package tests

import (
	"testing"
	"todo_server/internal/repository"
	"todo_server/internal/service"

	"github.com/stretchr/testify/suite"
)

type TodoServiceSuite struct {
	BaseSuite
	svc *service.TodoService
}

// SetupTest викликається автоматично перед кожним тестом.
func (s *TodoServiceSuite) SetupTest() {
	s.CleanDB()
	repo := repository.NewPostgresTodoRepository(s.DB)
	s.svc = service.NewTodoService(repo)
}

func (s *TodoServiceSuite) TestCreate() {
	todo, err := s.svc.Create("купити молоко", nil)

	s.NoError(err)
	s.Equal("купити молоко", todo.Title)
	s.False(todo.Completed)
	s.NotEmpty(todo.ID)
}

func (s *TodoServiceSuite) TestGetByID() {
	created, err := s.svc.Create("знайди мене", nil)
	s.NoError(err)

	found, err := s.svc.GetByID(created.ID)

	s.NoError(err)
	s.Equal(created.ID, found.ID)
	s.Equal("знайди мене", found.Title)
}

func (s *TodoServiceSuite) TestGetByID_NotFound() {
	_, err := s.svc.GetByID("неіснуючий-id")

	s.Error(err)
}

func (s *TodoServiceSuite) TestGetAll() {
	s.svc.Create("перший", nil)
	s.svc.Create("другий", nil)
	s.svc.Create("третій", nil)

	todos := s.svc.GetAll()

	s.Len(todos, 3)
}

func (s *TodoServiceSuite) TestUpdate() {
	created, err := s.svc.Create("старий заголовок", nil)
	s.NoError(err)

	updated, err := s.svc.Update(created.ID, "новий заголовок", true)

	s.NoError(err)
	s.Equal("новий заголовок", updated.Title)
	s.True(updated.Completed)
}

func (s *TodoServiceSuite) TestDelete() {
	created, err := s.svc.Create("видали мене", nil)
	s.NoError(err)

	err = s.svc.Delete(created.ID)
	s.NoError(err)

	_, err = s.svc.GetByID(created.ID)
	s.Error(err)
}

func (s *TodoServiceSuite) TestDelete_NotFound() {
	err := s.svc.Delete("неіснуючий-id")

	s.Error(err)
}

// Точка входу для запуску suite
func TestTodoServiceSuite(t *testing.T) {
	suite.Run(t, new(TodoServiceSuite))
}
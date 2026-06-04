package tests

import (
	"testing"
	"todo_server/internal/repository"
	"todo_server/internal/service"

	"github.com/stretchr/testify/suite"
)

type UserServiceSuite struct {
	BaseSuite
	svc *service.UserService
}

// SetupTest is called automatically before each test.
func (s *UserServiceSuite) SetupTest() {
	s.CleanDB()
	repo := repository.NewPostgresUserRepository(s.DB)
	s.svc = service.NewUserService(repo)
}

func (s *UserServiceSuite) TestRegister() {
	user, err := s.svc.Register("igor", "igor@test.com", "1234")

	s.NoError(err)
	s.Equal("igor", user.Username)
	s.Equal("igor@test.com", user.Email)
	s.NotEmpty(user.ID)
}

func (s *UserServiceSuite) TestRegister_DuplicateUsername() {
	_, err := s.svc.Register("igor", "igor@test.com", "1234")
	s.NoError(err)

	_, err = s.svc.Register("igor", "igor2@test.com", "1234")
	s.Error(err)
}

func (s *UserServiceSuite) TestAuthenticate_Success() {
	_, err := s.svc.Register("igor", "igor@test.com", "1234")
	s.NoError(err)

	user, err := s.svc.Authenticate("igor", "1234")

	s.NoError(err)
	s.Equal("igor", user.Username)
}

func (s *UserServiceSuite) TestAuthenticate_WrongPassword() {
	_, err := s.svc.Register("igor", "igor@test.com", "1234")
	s.NoError(err)

	_, err = s.svc.Authenticate("igor", "wrong_password")

	s.Error(err)
}

func (s *UserServiceSuite) TestAuthenticate_UserNotFound() {
	_, err := s.svc.Authenticate("nonexistent", "1234")

	s.Error(err)
}

func (s *UserServiceSuite) TestGetByID() {
	created, err := s.svc.Register("igor", "igor@test.com", "1234")
	s.NoError(err)

	found, err := s.svc.GetByID(created.ID)

	s.NoError(err)
	s.Equal(created.ID, found.ID)
	s.Equal("igor", found.Username)
}

func (s *UserServiceSuite) TestUpdate() {
	created, err := s.svc.Register("igor", "igor@test.com", "1234")
	s.NoError(err)

	updated, err := s.svc.Update(created.ID, "igor_new", "new@test.com")

	s.NoError(err)
	s.Equal("igor_new", updated.Username)
	s.Equal("new@test.com", updated.Email)
}

func (s *UserServiceSuite) TestDelete() {
	created, err := s.svc.Register("igor", "igor@test.com", "1234")
	s.NoError(err)

	err = s.svc.Delete(created.ID)
	s.NoError(err)

	_, err = s.svc.GetByID(created.ID)
	s.Error(err)
}

// Entry point for running the suite.
func TestUserServiceSuite(t *testing.T) {
	suite.Run(t, new(UserServiceSuite))
}
package tests

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"todo_server/internal/repository"
	"todo_server/internal/service"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type UserServiceSuite struct {
	suite.Suite
	DB        *sql.DB
	container *postgres.PostgresContainer
	svc       *service.UserService
}

func (s *UserServiceSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("todo_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		panic(fmt.Errorf("failed to start postgres container: %w", err))
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic(fmt.Errorf("failed to get connection string: %w", err))
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(fmt.Errorf("failed to open db: %w", err))
	}

	if err := db.Ping(); err != nil {
		panic(fmt.Errorf("failed to ping db: %w", err))
	}

	if err := runMigrations(db); err != nil {
		panic(fmt.Errorf("failed to run migrations: %w", err))
	}

	s.DB = db
	s.container = container
}

func (s *UserServiceSuite) TearDownSuite() {
	s.DB.Close()
	s.container.Terminate(context.Background())
}

func (s *UserServiceSuite) SetupTest() {
	repo := repository.NewPostgresUserRepository(s.DB)
	s.svc = service.NewUserService(repo)
}

func (s *UserServiceSuite) TearDownTest() {
	s.DB.Exec(`TRUNCATE TABLE todos, users CASCADE`)
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

func TestUserServiceSuite(t *testing.T) {
	suite.Run(t, new(UserServiceSuite))
}
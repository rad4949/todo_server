package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"todo_server/internal/repository"
	"todo_server/internal/service"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TodoServiceSuite struct {
	suite.Suite
	DB        *sql.DB
	container *postgres.PostgresContainer
	svc       *service.TodoService
}

func (s *TodoServiceSuite) SetupSuite() {
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

func (s *TodoServiceSuite) TearDownSuite() {
	s.DB.Close()
	s.container.Terminate(context.Background())
}

func (s *TodoServiceSuite) SetupTest() {
	repo := repository.NewPostgresTodoRepository(s.DB)
	s.svc = service.NewTodoService(repo)
}

func (s *TodoServiceSuite) TearDownTest() {
	s.DB.Exec(`TRUNCATE TABLE todos, users CASCADE`)
}

func (s *TodoServiceSuite) TestCreate() {
	todo, err := s.svc.Create("buy milk", nil)

	s.NoError(err)
	s.Equal("buy milk", todo.Title)
	s.False(todo.Completed)
	s.NotEmpty(todo.ID)
}

func (s *TodoServiceSuite) TestGetByID() {
	created, err := s.svc.Create("find me", nil)
	s.NoError(err)

	found, err := s.svc.GetByID(created.ID)

	s.NoError(err)
	s.Equal(created.ID, found.ID)
	s.Equal("find me", found.Title)
}

func (s *TodoServiceSuite) TestGetByID_NotFound() {
	_, err := s.svc.GetByID("non-existing-id")

	s.Error(err)
}

func (s *TodoServiceSuite) TestGetAll() {
	s.svc.Create("first", nil)
	s.svc.Create("second", nil)
	s.svc.Create("third", nil)

	todos := s.svc.GetAll()

	s.Len(todos, 3)
}

func (s *TodoServiceSuite) TestUpdate() {
	created, err := s.svc.Create("old title", nil)
	s.NoError(err)

	updated, err := s.svc.Update(created.ID, "new title", true)

	s.NoError(err)
	s.Equal("new title", updated.Title)
	s.True(updated.Completed)
}

func (s *TodoServiceSuite) TestDelete() {
	created, err := s.svc.Create("delete me", nil)
	s.NoError(err)

	err = s.svc.Delete(created.ID)
	s.NoError(err)

	_, err = s.svc.GetByID(created.ID)
	s.Error(err)
}

func (s *TodoServiceSuite) TestDelete_NotFound() {
	err := s.svc.Delete("non-existing-id")

	s.Error(err)
}

func TestTodoServiceSuite(t *testing.T) {
	suite.Run(t, new(TodoServiceSuite))
}

func runMigrations(db *sql.DB) error {
	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "../internal/db/migrations")

	files, err := filepath.Glob(filepath.Join(migrationsPath, "*.up.sql"))
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", file, err)
		}
	}

	return nil
}
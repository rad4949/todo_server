package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type BaseSuite struct {
	suite.Suite
	DB        *sql.DB
	container *postgres.PostgresContainer
}

func (s *BaseSuite) SetupSuite() {
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

func (s *BaseSuite) TearDownSuite() {
	s.DB.Close()
	s.container.Terminate(context.Background())
}

func (s *BaseSuite) CleanDB() {
	if _, err := s.DB.Exec(`TRUNCATE TABLE todos, users CASCADE`); err != nil {
		panic(fmt.Errorf("failed to clean db: %w", err))
	}
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
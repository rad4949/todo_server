package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
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

	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(fmt.Errorf("failed to open db: %w", err))
	}

	if err := testDB.Ping(); err != nil {
		panic(fmt.Errorf("failed to ping db: %w", err))
	}

	if err := runMigrations(testDB); err != nil {
		panic(fmt.Errorf("failed to run migrations: %w", err))
	}

	code := m.Run()

	testDB.Close()
	container.Terminate(ctx)

	os.Exit(code)
}

func runMigrations(db *sql.DB) error {
	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "../../db/migrations")

	pattern := filepath.Join(migrationsPath, "*.up.sql")
	files, err := filepath.Glob(pattern)
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

func cleanDB(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(`TRUNCATE TABLE todos, users CASCADE`)
	if err != nil {
		t.Fatalf("failed to clean db: %v", err)
	}
}
package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"todo_server/internal/model"
	"todo_server/internal/repository"
	"todo_server/internal/service"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type failingOutboxRepository struct {
	err error
}

func (r *failingOutboxRepository) Create(
	_ context.Context,
	_ repository.DBExecutor,
	_ model.OutboxEvent,
) error {
	return r.err
}

type storedOutboxEvent struct {
	AggregateType string
	AggregateID   string
	EventType     string
	EventVersion  int
	Status        string
	Attempts      int
	Payload       model.TodoEventPayload
}

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
	outboxRepo := repository.NewPostgresOutboxRepository(s.DB)
	repo := repository.NewPostgresTodoRepository(
		s.DB,
		outboxRepo,
	)
	s.svc = service.NewTodoService(repo)
}

func (s *TodoServiceSuite) TearDownTest() {
	s.DB.Exec(`
	TRUNCATE TABLE outbox_events, todos, users CASCADE
	`)
}

func (s *TodoServiceSuite) TestCreate() {
	todo, err := s.svc.Create(context.Background(), "buy milk", nil)

	s.NoError(err)
	s.Equal("buy milk", todo.Title)
	s.False(todo.Completed)
	s.NotEmpty(todo.ID)

	event := s.requireOutboxEvent(todo.ID, string(model.OutboxEventTodoCreated))
	s.Equal(1, s.countOutboxEvents(todo.ID, string(model.OutboxEventTodoCreated)))
	s.Equal("todo", event.AggregateType)
	s.Equal(todo.ID, event.AggregateID)
	s.Equal(model.TodoEventVersion, event.EventVersion)
	s.Equal(string(model.OutboxEventStatusPending), event.Status)
	s.Zero(event.Attempts)
	s.Equal(todo.ID, event.Payload.ID)
	s.Equal(todo.Title, event.Payload.Title)
	s.False(event.Payload.Completed)
}

func (s *TodoServiceSuite) TestGetByID() {
	created, err := s.svc.Create(context.Background(), "find me", nil)
	s.NoError(err)

	found, err := s.svc.GetByID(context.Background(), created.ID)

	s.NoError(err)
	s.Equal(created.ID, found.ID)
	s.Equal("find me", found.Title)
}

func (s *TodoServiceSuite) TestGetByID_NotFound() {
	_, err := s.svc.GetByID(context.Background(), "non-existing-id")

	s.Error(err)
}

func (s *TodoServiceSuite) TestGetAll() {
	s.svc.Create(context.Background(), "first", nil)
	s.svc.Create(context.Background(), "second", nil)
	s.svc.Create(context.Background(), "third", nil)

	todos, err := s.svc.GetAll(context.Background())

	s.NoError(err)
	s.Len(todos, 3)
}

func (s *TodoServiceSuite) TestUpdate() {
	created, err := s.svc.Create(context.Background(), "old title", nil)
	s.NoError(err)

	updated, err := s.svc.Update(context.Background(), created.ID, "new title", true)

	s.NoError(err)
	s.Equal("new title", updated.Title)
	s.True(updated.Completed)

	event := s.requireOutboxEvent(created.ID, string(model.OutboxEventTodoUpdated))
	s.Equal(1, s.countOutboxEvents(created.ID, string(model.OutboxEventTodoUpdated)))
	s.Equal(created.ID, event.Payload.ID)
	s.Equal("new title", event.Payload.Title)
	s.True(event.Payload.Completed)
}

func (s *TodoServiceSuite) TestDelete() {
	created, err := s.svc.Create(context.Background(), "delete me", nil)
	s.NoError(err)

	deleted, err := s.svc.Delete(context.Background(), created.ID)
	s.NoError(err)
	s.Equal(created.ID, deleted.ID)
	s.Equal(created.Title, deleted.Title)

	_, err = s.svc.GetByID(context.Background(), created.ID)
	s.Error(err)

	event := s.requireOutboxEvent(created.ID, string(model.OutboxEventTodoDeleted))
	s.Equal(1, s.countOutboxEvents(created.ID, string(model.OutboxEventTodoDeleted)))
	s.Equal(created.ID, event.Payload.ID)
	s.Equal(created.Title, event.Payload.Title)
	s.False(event.Payload.Completed)
}

func (s *TodoServiceSuite) TestDelete_NotFound() {
	_, err := s.svc.Delete(context.Background(), "non-existing-id")

	s.Error(err)
	s.Equal(0, s.countRows("outbox_events"))
}

func (s *TodoServiceSuite) TestCreateRollsBackWhenOutboxInsertFails() {
	failingService := s.newServiceWithFailingOutbox()

	_, err := failingService.Create(context.Background(), "must rollback", nil)

	s.Error(err)
	s.Contains(err.Error(), "create TodoCreated outbox event")
	s.Equal(0, s.countRows("todos"))
	s.Equal(0, s.countRows("outbox_events"))
}

func (s *TodoServiceSuite) TestUpdateRollsBackWhenOutboxInsertFails() {
	created, err := s.svc.Create(context.Background(), "original", nil)
	s.Require().NoError(err)
	failingService := s.newServiceWithFailingOutbox()

	_, err = failingService.Update(
		context.Background(),
		created.ID,
		"must rollback",
		true,
	)

	s.Error(err)
	s.Contains(err.Error(), "create TodoUpdated outbox event")
	found, getErr := s.svc.GetByID(context.Background(), created.ID)
	s.Require().NoError(getErr)
	s.Equal("original", found.Title)
	s.False(found.Completed)
	s.Equal(1, s.countRows("outbox_events"), "only TodoCreated must remain")
}

func (s *TodoServiceSuite) TestDeleteRollsBackWhenOutboxInsertFails() {
	created, err := s.svc.Create(context.Background(), "must survive", nil)
	s.Require().NoError(err)
	failingService := s.newServiceWithFailingOutbox()

	_, err = failingService.Delete(context.Background(), created.ID)

	s.Error(err)
	s.Contains(err.Error(), "create TodoDeleted outbox event")
	found, getErr := s.svc.GetByID(context.Background(), created.ID)
	s.Require().NoError(getErr)
	s.Equal(created.ID, found.ID)
	s.Equal("must survive", found.Title)
	s.Equal(1, s.countRows("outbox_events"), "only TodoCreated must remain")
}

func (s *TodoServiceSuite) newServiceWithFailingOutbox() *service.TodoService {
	outbox := &failingOutboxRepository{
		err: errors.New("forced outbox failure"),
	}
	repo := repository.NewPostgresTodoRepository(s.DB, outbox)
	return service.NewTodoService(repo)
}

func (s *TodoServiceSuite) countRows(table string) int {
	allowedTables := map[string]bool{
		"todos":         true,
		"outbox_events": true,
	}
	s.Require().True(allowedTables[table], "unexpected table name")

	var count int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *TodoServiceSuite) requireOutboxEvent(
	aggregateID string,
	eventType string,
) storedOutboxEvent {
	var event storedOutboxEvent
	var payload []byte

	err := s.DB.QueryRow(`
		SELECT
			aggregate_type,
			aggregate_id,
			event_type,
			event_version,
			status,
			attempts,
			payload
		FROM outbox_events
		WHERE aggregate_id = $1
		  AND event_type = $2
	`, aggregateID, eventType).Scan(
		&event.AggregateType,
		&event.AggregateID,
		&event.EventType,
		&event.EventVersion,
		&event.Status,
		&event.Attempts,
		&payload,
	)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(payload, &event.Payload))

	return event
}

func (s *TodoServiceSuite) countOutboxEvents(
	aggregateID string,
	eventType string,
) int {
	var count int
	err := s.DB.QueryRow(`
		SELECT COUNT(*)
		FROM outbox_events
		WHERE aggregate_id = $1
		  AND event_type = $2
	`, aggregateID, eventType).Scan(&count)
	s.Require().NoError(err)
	return count
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

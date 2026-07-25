package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"todo_server/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
)

type stubSQLResult struct{}

func (stubSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (stubSQLResult) RowsAffected() (int64, error) { return 1, nil }

type recordingExecutor struct {
	query string
	args  []any
	err   error
}

func (e *recordingExecutor) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = args
	if e.err != nil {
		return nil, e.err
	}
	return stubSQLResult{}, nil
}

func TestPostgresOutboxRepositoryCreate(t *testing.T) {
	exec := &recordingExecutor{}
	repo := NewPostgresOutboxRepository(nil)
	payload := json.RawMessage(`{"id":"todo-1","title":"test"}`)
	event := model.OutboxEvent{
		ID:            "event-1",
		AggregateType: "todo",
		AggregateID:   "todo-1",
		EventType:     model.OutboxEventTodoCreated,
		EventVersion:  model.TodoEventVersion,
		Payload:       payload,
	}

	if err := repo.Create(context.Background(), exec, event); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.Contains(exec.query, "INSERT INTO outbox_events") {
		t.Fatalf("query does not insert into outbox_events: %q", exec.query)
	}
	if len(exec.args) != 6 {
		t.Fatalf("argument count = %d, want 6", len(exec.args))
	}
	if exec.args[0] != event.ID || exec.args[2] != event.AggregateID {
		t.Fatalf("unexpected arguments: %#v", exec.args)
	}
	if exec.args[3] != string(model.OutboxEventTodoCreated) {
		t.Fatalf("event type = %#v", exec.args[3])
	}
}

func TestPostgresOutboxRepositoryCreateWrapsExecutorError(t *testing.T) {
	exec := &recordingExecutor{err: errors.New("database unavailable")}
	repo := NewPostgresOutboxRepository(nil)

	err := repo.Create(context.Background(), exec, model.OutboxEvent{})
	if err == nil {
		t.Fatal("Create() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "create outbox event") {
		t.Fatalf("Create() error = %q, want wrapped context", err)
	}
}

func TestNewTodoOutboxEvent(t *testing.T) {
	userID := "user-1"
	userEmail := "user@example.com"
	todo := model.Todo{
		ID:        "todo-1",
		Title:     "Learn outbox",
		Completed: true,
		UserID:    &userID,
	}

	event, err := newTodoOutboxEvent(todo, &userEmail, model.OutboxEventTodoUpdated)
	if err != nil {
		t.Fatalf("newTodoOutboxEvent() error = %v", err)
	}
	if event.ID == "" {
		t.Fatal("event ID is empty")
	}
	if event.AggregateType != "todo" || event.AggregateID != todo.ID {
		t.Fatalf("unexpected aggregate: type=%q id=%q", event.AggregateType, event.AggregateID)
	}
	if event.EventType != model.OutboxEventTodoUpdated {
		t.Fatalf("event type = %q", event.EventType)
	}
	if event.EventVersion != model.TodoEventVersion {
		t.Fatalf("event version = %d", event.EventVersion)
	}
	if event.EventVersion != 2 {
		t.Fatalf(
			"event version = %d, want 2",
			event.EventVersion,
		)
	}
	if event.Status != model.OutboxEventStatusPending {
		t.Fatalf("event status = %q", event.Status)
	}

	var payload model.TodoEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.ID != todo.ID || payload.Title != todo.Title || !payload.Completed {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.UserID == nil || *payload.UserID != userID {
		t.Fatalf("payload user ID = %v", payload.UserID)
	}
	if payload.UserEmail == nil {
		t.Fatal("payload user email is nil")
	}

	if *payload.UserEmail != userEmail {
		t.Fatalf(
			"payload user email = %q, want %q",
			*payload.UserEmail,
			userEmail,
		)
	}
}

func TestNewTodoOutboxEventOmitsUserEmailWhenNil(
	t *testing.T,
) {
	userID := "user-1"

	todo := model.Todo{
		ID:        "todo-1",
		Title:     "Todo without email",
		Completed: false,
		UserID:    &userID,
	}

	event, err := newTodoOutboxEvent(
		todo,
		nil,
		model.OutboxEventTodoCreated,
	)
	if err != nil {
		t.Fatalf(
			"newTodoOutboxEvent() error = %v",
			err,
		)
	}

	var payload map[string]json.RawMessage

	if err := json.Unmarshal(
		event.Payload,
		&payload,
	); err != nil {
		t.Fatalf(
			"unmarshal payload: %v",
			err,
		)
	}

	if _, exists := payload["user_email"]; exists {
		t.Fatal(
			"user_email exists in payload, want omitted",
		)
	}

	if _, exists := payload["user_id"]; !exists {
		t.Fatal(
			"user_id is missing from payload",
		)
	}
}

func TestPostgresOutboxRepositoryClaimPending(
	t *testing.T,
) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresOutboxRepository(db)

	now := time.Now().UTC()
	staleBefore := now.Add(-5 * time.Minute)
	lockedAt := now
	workerID := "worker-1"

	payload := []byte(
		`{"id":"todo-1","title":"Kafka test"}`,
	)

	rows := sqlmock.NewRows([]string{
		"id",
		"aggregate_type",
		"aggregate_id",
		"event_type",
		"event_version",
		"payload",
		"status",
		"attempts",
		"created_at",
		"processed_at",
		"next_attempt_at",
		"last_error",
		"locked_at",
		"locked_by",
	}).AddRow(
		"11111111-1111-4111-8111-111111111111",
		"todo",
		"todo-1",
		"TodoUpdated",
		2,
		payload,
		"processing",
		0,
		now,
		nil,
		now,
		nil,
		lockedAt,
		workerID,
	)

	mock.ExpectQuery(
		`WITH candidates AS`,
	).
		WithArgs(
			staleBefore,
			10,
			workerID,
		).
		WillReturnRows(rows)

	events, err := repo.ClaimPending(
		context.Background(),
		10,
		workerID,
		staleBefore,
	)
	if err != nil {
		t.Fatalf("ClaimPending() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf(
			"ClaimPending() event count = %d, want 1",
			len(events),
		)
	}

	event := events[0]

	if event.ID != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("event ID = %q", event.ID)
	}

	if event.AggregateType != "todo" {
		t.Fatalf(
			"aggregate type = %q, want todo",
			event.AggregateType,
		)
	}

	if event.AggregateID != "todo-1" {
		t.Fatalf(
			"aggregate ID = %q, want todo-1",
			event.AggregateID,
		)
	}

	if event.EventType != model.OutboxEventTodoUpdated {
		t.Fatalf(
			"event type = %q",
			event.EventType,
		)
	}

	if event.EventVersion != 2 {
		t.Fatalf(
			"event version = %d, want 2",
			event.EventVersion,
		)
	}

	if event.Status != model.OutboxEventStatusProcessing {
		t.Fatalf(
			"event status = %q, want processing",
			event.Status,
		)
	}

	if event.LockedBy == nil ||
		*event.LockedBy != workerID {
		t.Fatalf(
			"locked by = %v, want %q",
			event.LockedBy,
			workerID,
		)
	}

	if event.LockedAt == nil {
		t.Fatal("locked at is nil")
	}

	if string(event.Payload) != string(payload) {
		t.Fatalf(
			"payload = %s, want %s",
			event.Payload,
			payload,
		)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf(
			"unmet SQL expectations: %v",
			err,
		)
	}
}

func TestPostgresOutboxRepositoryMarkProcessed(
	t *testing.T,
) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresOutboxRepository(db)

	eventID := "11111111-1111-4111-8111-111111111111"
	workerID := "worker-1"

	mock.ExpectExec(
		`UPDATE outbox_events`,
	).
		WithArgs(
			eventID,
			workerID,
		).
		WillReturnResult(
			sqlmock.NewResult(0, 1),
		)

	err = repo.MarkProcessed(
		context.Background(),
		eventID,
		workerID,
	)
	if err != nil {
		t.Fatalf(
			"MarkProcessed() error = %v",
			err,
		)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf(
			"unmet SQL expectations: %v",
			err,
		)
	}
}

func TestPostgresOutboxRepositoryMarkFailed(
	t *testing.T,
) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresOutboxRepository(db)

	eventID := "11111111-1111-4111-8111-111111111111"
	workerID := "worker-1"
	errorMessage := "Kafka broker is unavailable"
	maxAttempts := 3
	nextAttemptAt := time.Now().
		UTC().
		Add(10 * time.Second)

	mock.ExpectExec(
		`UPDATE outbox_events`,
	).
		WithArgs(
			eventID,
			workerID,
			errorMessage,
			maxAttempts,
			nextAttemptAt,
		).
		WillReturnResult(
			sqlmock.NewResult(0, 1),
		)

	err = repo.MarkFailed(
		context.Background(),
		eventID,
		workerID,
		errorMessage,
		nextAttemptAt,
		maxAttempts,
	)
	if err != nil {
		t.Fatalf(
			"MarkFailed() error = %v",
			err,
		)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf(
			"unmet SQL expectations: %v",
			err,
		)
	}
}

func TestPostgresOutboxRepositoryMarkProcessedRejectsUnownedEvent(
	t *testing.T,
) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresOutboxRepository(db)

	eventID := "11111111-1111-4111-8111-111111111111"
	workerID := "wrong-worker"

	mock.ExpectExec(
		`UPDATE outbox_events`,
	).
		WithArgs(
			eventID,
			workerID,
		).
		WillReturnResult(
			sqlmock.NewResult(0, 0),
		)

	err = repo.MarkProcessed(
		context.Background(),
		eventID,
		workerID,
	)
	if err == nil {
		t.Fatal(
			"MarkProcessed() error = nil, want ownership error",
		)
	}

	if !strings.Contains(
		err.Error(),
		"event is not claimed by worker",
	) {
		t.Fatalf(
			"MarkProcessed() error = %q",
			err.Error(),
		)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf(
			"unmet SQL expectations: %v",
			err,
		)
	}
}

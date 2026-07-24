package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"todo_server/internal/model"
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
	todo := model.Todo{
		ID:        "todo-1",
		Title:     "Learn outbox",
		Completed: true,
		UserID:    &userID,
	}

	event, err := newTodoOutboxEvent(todo, model.OutboxEventTodoUpdated)
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
}

package model

import (
	"encoding/json"
	"time"
)

type OutboxEventStatus string

const (
	OutboxEventStatusPending    OutboxEventStatus = "pending"
	OutboxEventStatusProcessing OutboxEventStatus = "processing"
	OutboxEventStatusProcessed  OutboxEventStatus = "processed"
	OutboxEventStatusFailed     OutboxEventStatus = "failed"
)

type OutboxEventType string

const (
	OutboxEventTodoCreated OutboxEventType = "TodoCreated"
	OutboxEventTodoUpdated OutboxEventType = "TodoUpdated"
	OutboxEventTodoDeleted OutboxEventType = "TodoDeleted"
)

const TodoEventVersion = 1

type OutboxEvent struct {
	ID            string            `json:"id"`
	AggregateType string            `json:"aggregate_type"`
	AggregateID   string            `json:"aggregate_id"`
	EventType     OutboxEventType   `json:"event_type"`
	EventVersion  int               `json:"event_version"`
	Payload       json.RawMessage   `json:"payload"`
	Status        OutboxEventStatus `json:"status"`
	Attempts      int               `json:"attempts"`
	CreatedAt     time.Time         `json:"created_at"`
	ProcessedAt   *time.Time        `json:"processed_at,omitempty"`
	NextAttemptAt time.Time         `json:"next_attempt_at"`
	LastError     *string           `json:"last_error,omitempty"`
}

type TodoEventPayload struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Completed bool    `json:"completed"`
	UserID    *string `json:"user_id,omitempty"`
}

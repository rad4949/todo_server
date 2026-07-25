package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"todo_server/internal/model"

	"github.com/twmb/franz-go/pkg/kgo"
)

type fakeKafkaProducer struct {
	record     *kgo.Record
	produceErr error
	pingErr    error
	closed     bool
}

func (f *fakeKafkaProducer) ProduceSync(
	_ context.Context,
	records ...*kgo.Record,
) kgo.ProduceResults {
	if len(records) > 0 {
		f.record = records[0]
	}

	return kgo.ProduceResults{
		{
			Record: f.record,
			Err:    f.produceErr,
		},
	}
}

func (f *fakeKafkaProducer) Ping(_ context.Context) error {
	return f.pingErr
}

func (f *fakeKafkaProducer) Close() {
	f.closed = true
}

func TestKafkaPublisherPublish(t *testing.T) {
	createdAt := time.Date(
		2026, time.July, 25,
		12, 0, 0, 0,
		time.UTC,
	)

	userID := "user-123"
	userEmail := "user@example.com"

	payload, err := json.Marshal(model.TodoEventPayload{
		ID:        "todo-123",
		Title:     "Learn Kafka",
		Completed: false,
		UserID:    &userID,
		UserEmail: &userEmail,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	producer := &fakeKafkaProducer{}

	publisher := &KafkaPublisher{
		client: producer,
		topic:  "todo.events.v2",
	}

	event := model.OutboxEvent{
		ID:            "event-123",
		AggregateType: "todo",
		AggregateID:   "todo-123",
		EventType:     model.OutboxEventTodoCreated,
		EventVersion:  model.TodoEventVersion,
		Payload:       payload,
		CreatedAt:     createdAt,
	}

	err = publisher.Publish(context.Background(), event)
	if err != nil {
		t.Fatalf("Publish() error: %v", err)
	}

	if producer.record == nil {
		t.Fatal("expected Kafka record, got nil")
	}

	if producer.record.Topic != "todo.events.v2" {
		t.Errorf(
			"Topic = %q, want %q",
			producer.record.Topic,
			"todo.events.v2",
		)
	}

	if string(producer.record.Key) != event.AggregateID {
		t.Errorf(
			"Key = %q, want %q",
			producer.record.Key,
			event.AggregateID,
		)
	}

	if !producer.record.Timestamp.Equal(createdAt) {
		t.Errorf(
			"Timestamp = %v, want %v",
			producer.record.Timestamp,
			createdAt,
		)
	}

	var message Message

	if err := json.Unmarshal(producer.record.Value, &message); err != nil {
		t.Fatalf("unmarshal Kafka message: %v", err)
	}

	if message.EventID != event.ID {
		t.Errorf(
			"EventID = %q, want %q",
			message.EventID,
			event.ID,
		)
	}

	if message.AggregateType != event.AggregateType {
		t.Errorf(
			"AggregateType = %q, want %q",
			message.AggregateType,
			event.AggregateType,
		)
	}

	if message.AggregateID != event.AggregateID {
		t.Errorf(
			"AggregateID = %q, want %q",
			message.AggregateID,
			event.AggregateID,
		)
	}

	if message.EventType != event.EventType {
		t.Errorf(
			"EventType = %q, want %q",
			message.EventType,
			event.EventType,
		)
	}

	if message.EventVersion != model.TodoEventVersion {
		t.Errorf(
			"EventVersion = %d, want %d",
			message.EventVersion,
			model.TodoEventVersion,
		)
	}

	var messagePayload model.TodoEventPayload

	if err := json.Unmarshal(message.Payload, &messagePayload); err != nil {
		t.Fatalf("unmarshal todo payload: %v", err)
	}

	if messagePayload.UserEmail == nil {
		t.Fatal("UserEmail is nil")
	}

	if *messagePayload.UserEmail != userEmail {
		t.Errorf(
			"UserEmail = %q, want %q",
			*messagePayload.UserEmail,
			userEmail,
		)
	}

	if got := headerValue(
		producer.record.Headers,
		"event_id",
	); got != event.ID {
		t.Errorf(
			"event_id header = %q, want %q",
			got,
			event.ID,
		)
	}

	if got := headerValue(
		producer.record.Headers,
		"event_type",
	); got != string(event.EventType) {
		t.Errorf(
			"event_type header = %q, want %q",
			got,
			event.EventType,
		)
	}

	if got := headerValue(
		producer.record.Headers,
		"event_version",
	); got != "2" {
		t.Errorf(
			"event_version header = %q, want %q",
			got,
			"2",
		)
	}
}

func headerValue(
	headers []kgo.RecordHeader,
	key string,
) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}

	return ""
}

func TestKafkaPublisherPublishReturnsProducerError(t *testing.T) {
	producer := &fakeKafkaProducer{
		produceErr: errors.New("Kafka unavailable"),
	}

	publisher := &KafkaPublisher{
		client: producer,
		topic:  "todo.events.v2",
	}

	event := model.OutboxEvent{
		ID:            "event-123",
		AggregateType: "todo",
		AggregateID:   "todo-123",
		EventType:     model.OutboxEventTodoUpdated,
		EventVersion:  model.TodoEventVersion,
		Payload:       json.RawMessage(`{"id":"todo-123"}`),
		CreatedAt:     time.Now(),
	}

	err := publisher.Publish(context.Background(), event)
	if err == nil {
		t.Fatal("expected Publish() error, got nil")
	}

	if !strings.Contains(err.Error(), "event-123") {
		t.Errorf(
			"error %q does not contain event ID",
			err,
		)
	}

	if !strings.Contains(err.Error(), "Kafka unavailable") {
		t.Errorf(
			"error %q does not contain producer error",
			err,
		)
	}
}

func TestKafkaPublisherPublishValidatesEvent(t *testing.T) {
	tests := []struct {
		name      string
		event     model.OutboxEvent
		wantError string
	}{
		{
			name: "missing event ID",
			event: model.OutboxEvent{
				AggregateID: "todo-123",
			},
			wantError: "event ID is required",
		},
		{
			name: "missing aggregate ID",
			event: model.OutboxEvent{
				ID: "event-123",
			},
			wantError: "aggregate ID is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			producer := &fakeKafkaProducer{}

			publisher := &KafkaPublisher{
				client: producer,
				topic:  "todo.events.v2",
			}

			err := publisher.Publish(
				context.Background(),
				test.event,
			)
			if err == nil {
				t.Fatal("expected Publish() error, got nil")
			}

			if !strings.Contains(err.Error(), test.wantError) {
				t.Errorf(
					"error = %q, want it to contain %q",
					err,
					test.wantError,
				)
			}

			if producer.record != nil {
				t.Error(
					"producer received a record for invalid event",
				)
			}
		})
	}
}
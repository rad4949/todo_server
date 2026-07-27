package notification

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"todo_server/internal/model"

	"github.com/twmb/franz-go/pkg/kgo"
)

type fakeConsumerClient struct {
	mu sync.Mutex

	fetches    []kgo.Fetches
	fetchIndex int

	committedRecords []*kgo.Record
	commitErr        error
	pingErr          error
	closed           bool
	commitSignal     chan struct{}
}

func (f *fakeConsumerClient) PollRecords(
	ctx context.Context,
	_ int,
) kgo.Fetches {
	f.mu.Lock()

	if f.fetchIndex < len(f.fetches) {
		fetches := f.fetches[f.fetchIndex]
		f.fetchIndex++
		f.mu.Unlock()

		return fetches
	}

	f.mu.Unlock()
	<-ctx.Done()

	return nil
}

func (f *fakeConsumerClient) CommitRecords(
	_ context.Context,
	records ...*kgo.Record,
) error {
	f.mu.Lock()
	f.committedRecords = append(
		f.committedRecords,
		records...,
	)
	commitErr := f.commitErr
	commitSignal := f.commitSignal
	f.mu.Unlock()

	if commitSignal != nil {
		select {
		case commitSignal <- struct{}{}:
		default:
		}
	}

	return commitErr
}

func (f *fakeConsumerClient) Ping(
	_ context.Context,
) error {
	return f.pingErr
}

func (f *fakeConsumerClient) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true
}

func (f *fakeConsumerClient) committed() []*kgo.Record {
	f.mu.Lock()
	defer f.mu.Unlock()

	records := make(
		[]*kgo.Record,
		len(f.committedRecords),
	)
	copy(records, f.committedRecords)

	return records
}

type fakeNotificationHandler struct {
	mu sync.Mutex

	notifications []TodoNotification
	handleErr     error
}

func (f *fakeNotificationHandler) Handle(
	_ context.Context,
	notification TodoNotification,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.notifications = append(
		f.notifications,
		notification,
	)

	return f.handleErr
}

func (f *fakeNotificationHandler) handled() []TodoNotification {
	f.mu.Lock()
	defer f.mu.Unlock()

	notifications := make(
		[]TodoNotification,
		len(f.notifications),
	)
	copy(notifications, f.notifications)

	return notifications
}

func fetchesWithRecords(
	records ...*kgo.Record,
) kgo.Fetches {
	if len(records) == 0 {
		return nil
	}

	return kgo.Fetches{
		{
			Topics: []kgo.FetchTopic{
				{
					Topic: records[0].Topic,
					Partitions: []kgo.FetchPartition{
						{
							Partition: records[0].Partition,
							Records:   records,
						},
					},
				},
			},
		},
	}
}

func TestConsumerRunHandlesAndCommitsValidRecord(t *testing.T) {
	email := "user@example.com"
	record := &kgo.Record{
		Topic:     "todo.events.v2",
		Partition: 0,
		Offset:    15,
		Value: todoNotificationTestData(
			t,
			model.TodoEventVersion,
			model.OutboxEventTodoCreated,
			&email,
		),
	}

	commitSignal := make(chan struct{}, 1)
	client := &fakeConsumerClient{
		fetches: []kgo.Fetches{
			fetchesWithRecords(record),
		},
		commitSignal: commitSignal,
	}
	handler := &fakeNotificationHandler{}
	consumer := &Consumer{
		client:    client,
		handler:   handler,
		batchSize: 10,
	}

	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)

	go func() {
		runResult <- consumer.Run(ctx)
	}()

	select {
	case <-commitSignal:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("timed out waiting for record commit")
	}

	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("consumer did not stop after context cancellation")
	}

	handled := handler.handled()
	if len(handled) != 1 {
		t.Fatalf(
			"handled notifications = %d, want 1",
			len(handled),
		)
	}

	if handled[0].Message.EventID != "event-123" {
		t.Errorf(
			"handled EventID = %q, want %q",
			handled[0].Message.EventID,
			"event-123",
		)
	}

	committed := client.committed()
	if len(committed) != 1 {
		t.Fatalf(
			"committed records = %d, want 1",
			len(committed),
		)
	}

	if committed[0] != record {
		t.Error("committed record does not match consumed record")
	}
}

func TestConsumerRunDoesNotCommitWhenHandlerFails(t *testing.T) {
	email := "user@example.com"

	record := &kgo.Record{
		Topic:     "todo.events.v2",
		Partition: 0,
		Offset:    16,
		Value: todoNotificationTestData(
			t,
			model.TodoEventVersion,
			model.OutboxEventTodoUpdated,
			&email,
		),
	}

	client := &fakeConsumerClient{
		fetches: []kgo.Fetches{
			fetchesWithRecords(record),
		},
	}

	handler := &fakeNotificationHandler{
		handleErr: errors.New("SMTP unavailable"),
	}

	consumer := &Consumer{
		client:    client,
		handler:   handler,
		batchSize: 10,
	}

	err := consumer.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run() error, got nil")
	}

	if !strings.Contains(
		err.Error(),
		"SMTP unavailable",
	) {
		t.Errorf(
			"error = %q, want handler error",
			err,
		)
	}

	if !strings.Contains(
		err.Error(),
		"event-123",
	) {
		t.Errorf(
			"error = %q, want event ID",
			err,
		)
	}

	handled := handler.handled()
	if len(handled) != 1 {
		t.Errorf(
			"handled notifications = %d, want 1",
			len(handled),
		)
	}

	committed := client.committed()
	if len(committed) != 0 {
		t.Errorf(
			"committed records = %d, want 0",
			len(committed),
		)
	}
}

func TestConsumerRunCommitsInvalidEventWithoutCallingHandler(t *testing.T) {
	email := "user@example.com"
	record := &kgo.Record{
		Topic:     "todo.events.v2",
		Partition: 0,
		Offset:    5,
		Value: todoNotificationTestData(
			t,
			1,
			model.OutboxEventTodoCreated,
			&email,
		),
	}

	commitSignal := make(chan struct{}, 1)
	client := &fakeConsumerClient{
		fetches: []kgo.Fetches{
			fetchesWithRecords(record),
		},
		commitSignal: commitSignal,
	}
	handler := &fakeNotificationHandler{}
	consumer := &Consumer{
		client:    client,
		handler:   handler,
		batchSize: 10,
	}

	ctx, cancel := context.WithCancel(context.Background())
	runResult := make(chan error, 1)

	go func() {
		runResult <- consumer.Run(ctx)
	}()

	select {
	case <-commitSignal:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("timed out waiting for invalid record commit")
	}

	select {
	case err := <-runResult:
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("consumer did not stop after context cancellation")
	}

	if handled := handler.handled(); len(handled) != 0 {
		t.Errorf(
			"handled notifications = %d, want 0",
			len(handled),
		)
	}

	committed := client.committed()
	if len(committed) != 1 {
		t.Fatalf(
			"committed records = %d, want 1",
			len(committed),
		)
	}

	if committed[0] != record {
		t.Error("committed record does not match invalid record")
	}
}

func TestConsumerRunReturnsCommitError(t *testing.T) {
	email := "user@example.com"
	record := &kgo.Record{
		Topic:     "todo.events.v2",
		Partition: 0,
		Offset:    17,
		Value: todoNotificationTestData(
			t,
			model.TodoEventVersion,
			model.OutboxEventTodoCreated,
			&email,
		),
	}

	client := &fakeConsumerClient{
		fetches: []kgo.Fetches{
			fetchesWithRecords(record),
		},
		commitErr: errors.New("commit unavailable"),
	}
	handler := &fakeNotificationHandler{}
	consumer := &Consumer{
		client:    client,
		handler:   handler,
		batchSize: 10,
	}

	err := consumer.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run() error, got nil")
	}

	if !strings.Contains(err.Error(), "commit unavailable") {
		t.Errorf("error = %q, want commit error", err)
	}

	if !strings.Contains(err.Error(), "offset=17") {
		t.Errorf("error = %q, want record offset", err)
	}

	if handled := handler.handled(); len(handled) != 1 {
		t.Errorf(
			"handled notifications = %d, want 1",
			len(handled),
		)
	}

	if attempted := client.committed(); len(attempted) != 1 {
		t.Errorf(
			"commit attempts = %d, want 1",
			len(attempted),
		)
	}
}

func TestConsumerRunReturnsPollError(t *testing.T) {
	client := &fakeConsumerClient{
		fetches: []kgo.Fetches{
			kgo.NewErrFetch(errors.New("broker unavailable")),
		},
	}
	handler := &fakeNotificationHandler{}
	consumer := &Consumer{
		client:    client,
		handler:   handler,
		batchSize: 10,
	}

	err := consumer.Run(context.Background())
	if err == nil {
		t.Fatal("expected Run() error, got nil")
	}

	if !strings.Contains(err.Error(), "broker unavailable") {
		t.Errorf("error = %q, want poll error", err)
	}

	if len(handler.handled()) != 0 {
		t.Error("handler must not be called after poll error")
	}

	if len(client.committed()) != 0 {
		t.Error("record must not be committed after poll error")
	}
}

func TestConsumerPingAndClose(t *testing.T) {
	client := &fakeConsumerClient{
		pingErr: errors.New("ping unavailable"),
	}
	consumer := &Consumer{
		client:    client,
		handler:   &fakeNotificationHandler{},
		batchSize: 10,
	}

	err := consumer.Ping(context.Background())
	if err == nil {
		t.Fatal("expected Ping() error, got nil")
	}

	if !strings.Contains(err.Error(), "ping unavailable") {
		t.Errorf("error = %q, want ping error", err)
	}

	consumer.Close()

	client.mu.Lock()
	closed := client.closed
	client.mu.Unlock()

	if !closed {
		t.Error("client was not closed")
	}
}

func TestNewConsumerValidatesConfig(t *testing.T) {
	validConfig := ConsumerConfig{
		Brokers:   []string{"localhost:9092"},
		Topic:     "todo.events.v2",
		GroupID:   "todo-notification-v1",
		ClientID:  "notification-test",
		BatchSize: 10,
	}
	validHandler := &fakeNotificationHandler{}

	tests := []struct {
		name      string
		config    ConsumerConfig
		handler   Handler
		wantError string
	}{
		{
			name: "missing brokers",
			config: func() ConsumerConfig {
				config := validConfig
				config.Brokers = nil
				return config
			}(),
			handler:   validHandler,
			wantError: "at least one broker is required",
		},
		{
			name: "missing topic",
			config: func() ConsumerConfig {
				config := validConfig
				config.Topic = ""
				return config
			}(),
			handler:   validHandler,
			wantError: "topic is required",
		},
		{
			name: "missing group ID",
			config: func() ConsumerConfig {
				config := validConfig
				config.GroupID = ""
				return config
			}(),
			handler:   validHandler,
			wantError: "group ID is required",
		},
		{
			name: "missing client ID",
			config: func() ConsumerConfig {
				config := validConfig
				config.ClientID = ""
				return config
			}(),
			handler:   validHandler,
			wantError: "client ID is required",
		},
		{
			name: "invalid batch size",
			config: func() ConsumerConfig {
				config := validConfig
				config.BatchSize = 0
				return config
			}(),
			handler:   validHandler,
			wantError: "batch size must be positive",
		},
		{
			name:      "missing handler",
			config:    validConfig,
			handler:   nil,
			wantError: "handler is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			consumer, err := NewConsumer(
				test.config,
				test.handler,
			)
			if consumer != nil {
				consumer.Close()
				t.Fatal("expected nil consumer")
			}

			if err == nil {
				t.Fatal("expected NewConsumer() error, got nil")
			}

			if !strings.Contains(err.Error(), test.wantError) {
				t.Errorf(
					"error = %q, want it to contain %q",
					err,
					test.wantError,
				)
			}
		})
	}
}

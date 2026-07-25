package outbox

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"todo_server/internal/model"
)

type fakeWorkerRepository struct {
	events []model.OutboxEvent

	claimErr         error
	markProcessedErr error
	markFailedErr    error

	claimLimit       int
	claimWorkerID    string
	claimStaleBefore time.Time

	processedEventIDs []string
	processedWorkerID string

	failedEventIDs []string
	failedWorkerID string
	failedMessages []string
	nextAttemptAt  []time.Time
	maxAttempts    int

	claimSignal chan struct{}
}

func newTestWorker(
	repository WorkerRepository,
	publisher Publisher,
) *Worker {
	worker, err := NewWorker(
		repository,
		publisher,
		WorkerConfig{
			WorkerID:       "worker-1",
			BatchSize:      10,
			PollInterval:   time.Second,
			LockTimeout:    time.Minute,
			MaxAttempts:    5,
			RetryBaseDelay: time.Second,
		},
	)
	if err != nil {
		panic(err)
	}

	return worker
}

func (f *fakeWorkerRepository) ClaimPending(
	_ context.Context,
	limit int,
	workerID string,
	staleBefore time.Time,
) ([]model.OutboxEvent, error) {

	if f.claimSignal != nil {
		select {
		case f.claimSignal <- struct{}{}:
		default:
		}
	}
	f.claimLimit = limit
	f.claimWorkerID = workerID
	f.claimStaleBefore = staleBefore

	if f.claimErr != nil {
		return nil, f.claimErr
	}

	return f.events, nil
}

func (f *fakeWorkerRepository) MarkProcessed(
	_ context.Context,
	eventID string,
	workerID string,
) error {
	f.processedEventIDs = append(
		f.processedEventIDs,
		eventID,
	)
	f.processedWorkerID = workerID

	return f.markProcessedErr
}

func (f *fakeWorkerRepository) MarkFailed(
	_ context.Context,
	eventID string,
	workerID string,
	errorMessage string,
	nextAttemptAt time.Time,
	maxAttempts int,
) error {
	f.failedEventIDs = append(
		f.failedEventIDs,
		eventID,
	)
	f.failedWorkerID = workerID
	f.failedMessages = append(
		f.failedMessages,
		errorMessage,
	)
	f.nextAttemptAt = append(
		f.nextAttemptAt,
		nextAttemptAt,
	)
	f.maxAttempts = maxAttempts

	return f.markFailedErr
}

type fakePublisher struct {
	publishedEvents []model.OutboxEvent
	publishErr      error
}

func (f *fakePublisher) Publish(
	_ context.Context,
	event model.OutboxEvent,
) error {
	f.publishedEvents = append(
		f.publishedEvents,
		event,
	)

	return f.publishErr
}

func TestWorkerProcessBatchPublishesAndMarksProcessed(
	t *testing.T,
) {
	event := model.OutboxEvent{
		ID:            "event-123",
		AggregateType: "todo",
		AggregateID:   "todo-123",
		EventType:     model.OutboxEventTodoCreated,
		EventVersion:  model.TodoEventVersion,
		Payload:       []byte(`{"id":"todo-123"}`),
		Attempts:      0,
		CreatedAt:     time.Now().UTC(),
	}

	repository := &fakeWorkerRepository{
		events: []model.OutboxEvent{
			event,
		},
	}

	publisher := &fakePublisher{}
	worker := newTestWorker(repository, publisher)

	processedCount, err := worker.ProcessBatch(
		context.Background(),
	)
	if err != nil {
		t.Fatalf("ProcessBatch() error: %v", err)
	}

	if processedCount != 1 {
		t.Errorf(
			"processed count = %d, want 1",
			processedCount,
		)
	}

	if repository.claimLimit != 10 {
		t.Errorf(
			"claim limit = %d, want 10",
			repository.claimLimit,
		)
	}

	if repository.claimWorkerID != "worker-1" {
		t.Errorf(
			"claim worker ID = %q, want %q",
			repository.claimWorkerID,
			"worker-1",
		)
	}

	if len(publisher.publishedEvents) != 1 {
		t.Fatalf(
			"published events count = %d, want 1",
			len(publisher.publishedEvents),
		)
	}

	if publisher.publishedEvents[0].ID != event.ID {
		t.Errorf(
			"published event ID = %q, want %q",
			publisher.publishedEvents[0].ID,
			event.ID,
		)
	}

	if len(repository.processedEventIDs) != 1 {
		t.Fatalf(
			"processed event IDs count = %d, want 1",
			len(repository.processedEventIDs),
		)
	}

	if repository.processedEventIDs[0] != event.ID {
		t.Errorf(
			"processed event ID = %q, want %q",
			repository.processedEventIDs[0],
			event.ID,
		)
	}

	if repository.processedWorkerID != "worker-1" {
		t.Errorf(
			"processed worker ID = %q, want %q",
			repository.processedWorkerID,
			"worker-1",
		)
	}

	if len(repository.failedEventIDs) != 0 {
		t.Errorf(
			"MarkFailed called %d times, want 0",
			len(repository.failedEventIDs),
		)
	}
}

func TestWorkerProcessBatchMarksFailedWhenPublishFails(
	t *testing.T,
) {
	event := model.OutboxEvent{
		ID:            "event-456",
		AggregateType: "todo",
		AggregateID:   "todo-456",
		EventType:     model.OutboxEventTodoUpdated,
		EventVersion:  model.TodoEventVersion,
		Payload:       []byte(`{"id":"todo-456"}`),
		Attempts:      2,
		CreatedAt:     time.Now().UTC(),
	}

	repository := &fakeWorkerRepository{
		events: []model.OutboxEvent{
			event,
		},
	}

	publisher := &fakePublisher{
		publishErr: errors.New("Kafka unavailable"),
	}

	worker := newTestWorker(repository, publisher)

	beforeProcess := time.Now().UTC()

	processedCount, err := worker.ProcessBatch(
		context.Background(),
	)

	afterProcess := time.Now().UTC()

	if err == nil {
		t.Fatal("expected ProcessBatch() error, got nil")
	}

	if !strings.Contains(err.Error(), "Kafka unavailable") {
		t.Errorf(
			"error = %q, want Kafka error",
			err,
		)
	}

	if processedCount != 1 {
		t.Errorf(
			"processed count = %d, want 1",
			processedCount,
		)
	}

	if len(repository.processedEventIDs) != 0 {
		t.Errorf(
			"MarkProcessed called %d times, want 0",
			len(repository.processedEventIDs),
		)
	}

	if len(repository.failedEventIDs) != 1 {
		t.Fatalf(
			"MarkFailed called %d times, want 1",
			len(repository.failedEventIDs),
		)
	}

	if repository.failedEventIDs[0] != event.ID {
		t.Errorf(
			"failed event ID = %q, want %q",
			repository.failedEventIDs[0],
			event.ID,
		)
	}

	if repository.failedWorkerID != "worker-1" {
		t.Errorf(
			"failed worker ID = %q, want %q",
			repository.failedWorkerID,
			"worker-1",
		)
	}

	if repository.maxAttempts != 5 {
		t.Errorf(
			"max attempts = %d, want 5",
			repository.maxAttempts,
		)
	}

	if len(repository.failedMessages) != 1 {
		t.Fatalf(
			"failed messages count = %d, want 1",
			len(repository.failedMessages),
		)
	}

	if !strings.Contains(
		repository.failedMessages[0],
		"Kafka unavailable",
	) {
		t.Errorf(
			"failed message = %q, want Kafka error",
			repository.failedMessages[0],
		)
	}

	if len(repository.nextAttemptAt) != 1 {
		t.Fatalf(
			"next attempt times count = %d, want 1",
			len(repository.nextAttemptAt),
		)
	}

	// Attempts = 2, тому затримка:
	// 1s × 2² = 4s.
	minimumNextAttempt := beforeProcess.Add(4 * time.Second)
	maximumNextAttempt := afterProcess.Add(4 * time.Second)

	if repository.nextAttemptAt[0].Before(minimumNextAttempt) {
		t.Errorf(
			"next attempt = %v, want >= %v",
			repository.nextAttemptAt[0],
			minimumNextAttempt,
		)
	}

	if repository.nextAttemptAt[0].After(maximumNextAttempt) {
		t.Errorf(
			"next attempt = %v, want <= %v",
			repository.nextAttemptAt[0],
			maximumNextAttempt,
		)
	}
}

func TestWorkerProcessBatchReturnsClaimError(
	t *testing.T,
) {
	repository := &fakeWorkerRepository{
		claimErr: errors.New("PostgreSQL unavailable"),
	}

	publisher := &fakePublisher{}
	worker := newTestWorker(repository, publisher)

	processedCount, err := worker.ProcessBatch(
		context.Background(),
	)

	if err == nil {
		t.Fatal("expected ProcessBatch() error, got nil")
	}

	if !strings.Contains(
		err.Error(),
		"claim pending outbox events",
	) {
		t.Errorf(
			"error = %q, want claim context",
			err,
		)
	}

	if !strings.Contains(
		err.Error(),
		"PostgreSQL unavailable",
	) {
		t.Errorf(
			"error = %q, want repository error",
			err,
		)
	}

	if processedCount != 0 {
		t.Errorf(
			"processed count = %d, want 0",
			processedCount,
		)
	}

	if len(publisher.publishedEvents) != 0 {
		t.Errorf(
			"publisher called %d times, want 0",
			len(publisher.publishedEvents),
		)
	}

	if len(repository.processedEventIDs) != 0 {
		t.Errorf(
			"MarkProcessed called %d times, want 0",
			len(repository.processedEventIDs),
		)
	}

	if len(repository.failedEventIDs) != 0 {
		t.Errorf(
			"MarkFailed called %d times, want 0",
			len(repository.failedEventIDs),
		)
	}
}

func TestWorkerProcessBatchReturnsMarkProcessedError(
	t *testing.T,
) {
	event := model.OutboxEvent{
		ID:            "event-789",
		AggregateType: "todo",
		AggregateID:   "todo-789",
		EventType:     model.OutboxEventTodoCreated,
		EventVersion:  model.TodoEventVersion,
		Payload:       []byte(`{"id":"todo-789"}`),
		CreatedAt:     time.Now().UTC(),
	}

	repository := &fakeWorkerRepository{
		events: []model.OutboxEvent{
			event,
		},
		markProcessedErr: errors.New(
			"database update failed",
		),
	}

	publisher := &fakePublisher{}
	worker := newTestWorker(repository, publisher)

	processedCount, err := worker.ProcessBatch(
		context.Background(),
	)

	if err == nil {
		t.Fatal("expected ProcessBatch() error, got nil")
	}

	if !strings.Contains(
		err.Error(),
		"mark outbox event event-789 as processed",
	) {
		t.Errorf(
			"error = %q, want MarkProcessed context",
			err,
		)
	}

	if !strings.Contains(
		err.Error(),
		"database update failed",
	) {
		t.Errorf(
			"error = %q, want repository error",
			err,
		)
	}

	if processedCount != 1 {
		t.Errorf(
			"processed count = %d, want 1",
			processedCount,
		)
	}

	if len(publisher.publishedEvents) != 1 {
		t.Fatalf(
			"published events count = %d, want 1",
			len(publisher.publishedEvents),
		)
	}

	if publisher.publishedEvents[0].ID != event.ID {
		t.Errorf(
			"published event ID = %q, want %q",
			publisher.publishedEvents[0].ID,
			event.ID,
		)
	}

	if len(repository.processedEventIDs) != 1 {
		t.Fatalf(
			"MarkProcessed calls = %d, want 1",
			len(repository.processedEventIDs),
		)
	}

	if len(repository.failedEventIDs) != 0 {
		t.Errorf(
			"MarkFailed calls = %d, want 0",
			len(repository.failedEventIDs),
		)
	}
}

func TestWorkerRunPollsAndStopsAfterContextCancellation(
	t *testing.T,
) {
	claimSignal := make(chan struct{}, 10)

	repository := &fakeWorkerRepository{
		claimSignal: claimSignal,
	}

	publisher := &fakePublisher{}

	worker, err := NewWorker(
		repository,
		publisher,
		WorkerConfig{
			WorkerID:       "worker-run-test",
			BatchSize:      10,
			PollInterval:   10 * time.Millisecond,
			LockTimeout:    time.Minute,
			MaxAttempts:    5,
			RetryBaseDelay: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewWorker() error: %v", err)
	}

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	workerStopped := make(chan struct{})

	go func() {
		worker.Run(ctx)
		close(workerStopped)
	}()

	for call := 1; call <= 2; call++ {
		select {
		case <-claimSignal:
			// Worker викликав ClaimPending.

		case <-time.After(time.Second):
			cancel()

			t.Fatalf(
				"timed out waiting for ClaimPending call %d",
				call,
			)
		}
	}

	cancel()

	select {
	case <-workerStopped:
		// Worker коректно завершився.

	case <-time.After(time.Second):
		t.Fatal(
			"worker did not stop after context cancellation",
		)
	}
}
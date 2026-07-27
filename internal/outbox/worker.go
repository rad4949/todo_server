package outbox

import (
	"context"
	"fmt"
	"time"
	"errors"
	"log/slog"

	"todo_server/internal/model"
)

type WorkerRepository interface {
	ClaimPending(
		ctx context.Context,
		limit int,
		workerID string,
		staleBefore time.Time,
	) ([]model.OutboxEvent, error)

	MarkProcessed(
		ctx context.Context,
		eventID string,
		workerID string,
	) error

	MarkFailed(
		ctx context.Context,
		eventID string,
		workerID string,
		errorMessage string,
		nextAttemptAt time.Time,
		maxAttempts int,
	) error
}

type WorkerConfig struct {
	WorkerID       string
	BatchSize      int
	PollInterval   time.Duration
	LockTimeout    time.Duration
	MaxAttempts    int
	RetryBaseDelay time.Duration
}

type Worker struct {
	repository WorkerRepository
	publisher  Publisher
	config     WorkerConfig
}

func NewWorker(
	repository WorkerRepository,
	publisher Publisher,
	config WorkerConfig,
) (*Worker, error) {
	if repository == nil {
		return nil, fmt.Errorf(
			"create outbox worker: repository is required",
		)
	}

	if publisher == nil {
		return nil, fmt.Errorf(
			"create outbox worker: publisher is required",
		)
	}

	if config.WorkerID == "" {
		return nil, fmt.Errorf(
			"create outbox worker: worker ID is required",
		)
	}

	if config.BatchSize <= 0 {
		return nil, fmt.Errorf(
			"create outbox worker: batch size must be positive",
		)
	}

	if config.PollInterval <= 0 {
		return nil, fmt.Errorf(
			"create outbox worker: poll interval must be positive",
		)
	}

	if config.LockTimeout <= 0 {
		return nil, fmt.Errorf(
			"create outbox worker: lock timeout must be positive",
		)
	}

	if config.MaxAttempts <= 0 {
		return nil, fmt.Errorf(
			"create outbox worker: max attempts must be positive",
		)
	}

	if config.RetryBaseDelay <= 0 {
		return nil, fmt.Errorf(
			"create outbox worker: retry base delay must be positive",
		)
	}

	return &Worker{
		repository: repository,
		publisher:  publisher,
		config:     config,
	}, nil
}

func (w *Worker) ProcessBatch(
	ctx context.Context,
) (int, error) {
	now := time.Now().UTC()
	staleBefore := now.Add(-w.config.LockTimeout)

	events, err := w.repository.ClaimPending(
		ctx,
		w.config.BatchSize,
		w.config.WorkerID,
		staleBefore,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"claim pending outbox events: %w",
			err,
		)
	}

	var processingErrors []error

	for _, event := range events {
		if err := ctx.Err(); err != nil {
			processingErrors = append(
				processingErrors,
				err,
			)
			break
		}

		if err := w.processEvent(ctx, event); err != nil {
			processingErrors = append(
				processingErrors,
				err,
			)
		}
	}

	return len(events), errors.Join(processingErrors...)
}

func (w *Worker) Run(
	ctx context.Context,
) {
	w.processAndLog(ctx)

	ticker := time.NewTicker(
		w.config.PollInterval,
	)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info(
				"outbox worker stopped",
				"worker_id",
				w.config.WorkerID,
			)

			return

		case <-ticker.C:
			w.processAndLog(ctx)
		}
	}
}

func (w *Worker) processAndLog(
	ctx context.Context,
) {
	claimedCount, err := w.ProcessBatch(ctx)
	if err != nil {
		slog.Error(
			"outbox batch processing failed",
			"worker_id",
			w.config.WorkerID,
			"claimed_count",
			claimedCount,
			"error",
			err,
		)

		return
	}

	if claimedCount > 0 {
		slog.Info(
			"outbox batch processed",
			"worker_id",
			w.config.WorkerID,
			"claimed_count",
			claimedCount,
		)
	}
}

func (w *Worker) processEvent(
	ctx context.Context,
	event model.OutboxEvent,
) error {
	err := w.publisher.Publish(ctx, event)
	if err != nil {
		nextAttemptAt := time.Now().
			UTC().
			Add(w.retryDelay(event.Attempts))

		markErr := w.repository.MarkFailed(
			ctx,
			event.ID,
			w.config.WorkerID,
			err.Error(),
			nextAttemptAt,
			w.config.MaxAttempts,
		)
		if markErr != nil {
			return errors.Join(
				fmt.Errorf(
					"publish outbox event %s: %w",
					event.ID,
					err,
				),
				fmt.Errorf(
					"mark outbox event %s as failed: %w",
					event.ID,
					markErr,
				),
			)
		}

		return fmt.Errorf(
			"publish outbox event %s: %w",
			event.ID,
			err,
		)
	}

	if err := w.repository.MarkProcessed(
		ctx,
		event.ID,
		w.config.WorkerID,
	); err != nil {
		return fmt.Errorf(
			"mark outbox event %s as processed: %w",
			event.ID,
			err,
		)
	}

	return nil
}

func (w *Worker) retryDelay(
	attempts int,
) time.Duration {
	if attempts <= 0 {
		return w.config.RetryBaseDelay
	}

	delay := w.config.RetryBaseDelay

	for attempt := 0; attempt < attempts; attempt++ {
		if delay > 30*time.Minute {
			return time.Hour
		}

		delay *= 2
	}

	return delay
}
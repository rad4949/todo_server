package notification

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"todo_server/internal/model"
	"todo_server/internal/repository"
)

type fakeDeliveryRepository struct {
	started       bool
	tryStartErr   error
	markSentErr   error
	markFailedErr error

	tryStartCalls   int
	markSentCalls   int
	markFailedCalls int
	eventID         string
	eventType       string
	recipient       string
	lastError       string
}

func (f *fakeDeliveryRepository) TryStart(
	_ context.Context,
	eventID string,
	eventType string,
	recipient string,
) (bool, error) {
	f.tryStartCalls++
	f.eventID = eventID
	f.eventType = eventType
	f.recipient = recipient
	return f.started, f.tryStartErr
}

func (f *fakeDeliveryRepository) MarkSent(
	_ context.Context,
	eventID string,
) error {
	f.markSentCalls++
	f.eventID = eventID
	return f.markSentErr
}

func (f *fakeDeliveryRepository) MarkFailed(
	_ context.Context,
	eventID string,
	lastError string,
) error {
	f.markFailedCalls++
	f.eventID = eventID
	f.lastError = lastError
	return f.markFailedErr
}

func (f *fakeDeliveryRepository) GetByEventID(
	_ context.Context,
	_ string,
) (model.NotificationDelivery, error) {
	return model.NotificationDelivery{}, nil
}

type recordingNotificationHandler struct {
	calls int
	err   error
}

func (h *recordingNotificationHandler) Handle(
	_ context.Context,
	_ TodoNotification,
) error {
	h.calls++
	return h.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestIdempotentEmailHandler(
	t *testing.T,
	next Handler,
	deliveries *fakeDeliveryRepository,
) *IdempotentEmailHandler {
	t.Helper()

	handler, err := NewIdempotentEmailHandler(
		next,
		deliveries,
		discardLogger(),
	)
	if err != nil {
		t.Fatalf("NewIdempotentEmailHandler() error = %v", err)
	}
	return handler
}

func TestNewIdempotentEmailHandlerValidation(t *testing.T) {
	validHandler := &recordingNotificationHandler{}
	validRepository := &fakeDeliveryRepository{}
	validLogger := discardLogger()

	tests := []struct {
		name          string
		next          Handler
		deliveries    repository.NotificationDeliveryRepository
		logger        *slog.Logger
		wantErrorText string
	}{
		{"missing next handler", nil, validRepository, validLogger, "next handler is required"},
		{"missing repository", validHandler, nil, validLogger, "delivery repository is required"},
		{"missing logger", validHandler, validRepository, nil, "logger is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIdempotentEmailHandler(tt.next, tt.deliveries, tt.logger)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrorText) {
				t.Fatalf("NewIdempotentEmailHandler() error = %v, want %q", err, tt.wantErrorText)
			}
		})
	}
}

func TestIdempotentEmailHandlerHandleSuccess(t *testing.T) {
	next := &recordingNotificationHandler{}
	deliveries := &fakeDeliveryRepository{started: true}
	handler := newTestIdempotentEmailHandler(t, next, deliveries)

	err := handler.Handle(
		context.Background(),
		todoEmailNotification(model.OutboxEventTodoCreated),
	)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if deliveries.tryStartCalls != 1 || deliveries.markSentCalls != 1 {
		t.Fatalf(
			"repository calls: TryStart=%d MarkSent=%d, want 1 and 1",
			deliveries.tryStartCalls,
			deliveries.markSentCalls,
		)
	}
	if deliveries.markFailedCalls != 0 {
		t.Fatalf("MarkFailed calls = %d, want 0", deliveries.markFailedCalls)
	}
	if next.calls != 1 {
		t.Fatalf("next handler calls = %d, want 1", next.calls)
	}
	if deliveries.eventID != "event-123" ||
		deliveries.eventType != string(model.OutboxEventTodoCreated) ||
		deliveries.recipient != "user@example.com" {
		t.Fatalf(
			"TryStart arguments = eventID=%q eventType=%q recipient=%q",
			deliveries.eventID,
			deliveries.eventType,
			deliveries.recipient,
		)
	}
}

func TestIdempotentEmailHandlerHandleSkipsDuplicate(t *testing.T) {
	next := &recordingNotificationHandler{}
	deliveries := &fakeDeliveryRepository{started: false}
	handler := newTestIdempotentEmailHandler(t, next, deliveries)

	err := handler.Handle(
		context.Background(),
		todoEmailNotification(model.OutboxEventTodoUpdated),
	)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if next.calls != 0 {
		t.Fatalf("next handler calls = %d, want 0", next.calls)
	}
	if deliveries.markSentCalls != 0 || deliveries.markFailedCalls != 0 {
		t.Fatalf(
			"status calls: MarkSent=%d MarkFailed=%d, want 0 and 0",
			deliveries.markSentCalls,
			deliveries.markFailedCalls,
		)
	}
}

func TestIdempotentEmailHandlerHandleMarksFailed(t *testing.T) {
	sendErr := errors.New("SMTP unavailable")
	next := &recordingNotificationHandler{err: sendErr}
	deliveries := &fakeDeliveryRepository{started: true}
	handler := newTestIdempotentEmailHandler(t, next, deliveries)

	err := handler.Handle(
		context.Background(),
		todoEmailNotification(model.OutboxEventTodoDeleted),
	)
	if !errors.Is(err, sendErr) {
		t.Fatalf("Handle() error = %v, want wrapped SMTP error", err)
	}
	if deliveries.markFailedCalls != 1 {
		t.Fatalf("MarkFailed calls = %d, want 1", deliveries.markFailedCalls)
	}
	if deliveries.markSentCalls != 0 {
		t.Fatalf("MarkSent calls = %d, want 0", deliveries.markSentCalls)
	}
	if deliveries.lastError != sendErr.Error() {
		t.Fatalf("MarkFailed error = %q, want %q", deliveries.lastError, sendErr)
	}
}

func TestIdempotentEmailHandlerHandleRepositoryErrors(t *testing.T) {
	t.Run("TryStart error prevents email", func(t *testing.T) {
		dbErr := errors.New("database unavailable")
		next := &recordingNotificationHandler{}
		deliveries := &fakeDeliveryRepository{
			started:     false,
			tryStartErr: dbErr,
		}
		handler := newTestIdempotentEmailHandler(t, next, deliveries)

		err := handler.Handle(
			context.Background(),
			todoEmailNotification(model.OutboxEventTodoCreated),
		)
		if !errors.Is(err, dbErr) {
			t.Fatalf("Handle() error = %v, want TryStart error", err)
		}
		if next.calls != 0 {
			t.Fatalf("next handler calls = %d, want 0", next.calls)
		}
	})

	t.Run("MarkSent error is returned", func(t *testing.T) {
		markErr := errors.New("cannot mark sent")
		next := &recordingNotificationHandler{}
		deliveries := &fakeDeliveryRepository{
			started:     true,
			markSentErr: markErr,
		}
		handler := newTestIdempotentEmailHandler(t, next, deliveries)

		err := handler.Handle(
			context.Background(),
			todoEmailNotification(model.OutboxEventTodoCreated),
		)
		if !errors.Is(err, markErr) {
			t.Fatalf("Handle() error = %v, want MarkSent error", err)
		}
	})

	t.Run("MarkFailed error includes both failures", func(t *testing.T) {
		sendErr := errors.New("SMTP unavailable")
		markErr := errors.New("cannot mark failed")
		next := &recordingNotificationHandler{err: sendErr}
		deliveries := &fakeDeliveryRepository{
			started:       true,
			markFailedErr: markErr,
		}
		handler := newTestIdempotentEmailHandler(t, next, deliveries)

		err := handler.Handle(
			context.Background(),
			todoEmailNotification(model.OutboxEventTodoCreated),
		)
		if !errors.Is(err, markErr) || !strings.Contains(err.Error(), sendErr.Error()) {
			t.Fatalf("Handle() error = %v, want send and MarkFailed errors", err)
		}
	})
}

func TestIdempotentEmailHandlerHandleValidation(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*TodoNotification)
		wantError string
	}{
		{
			name:      "missing event ID",
			modify:    func(n *TodoNotification) { n.Message.EventID = " " },
			wantError: "event ID is required",
		},
		{
			name:      "missing event type",
			modify:    func(n *TodoNotification) { n.Message.EventType = "" },
			wantError: "event type is required",
		},
		{
			name:      "missing email",
			modify:    func(n *TodoNotification) { n.Payload.UserEmail = nil },
			wantError: "user email is required",
		},
		{
			name: "blank email",
			modify: func(n *TodoNotification) {
				email := "  "
				n.Payload.UserEmail = &email
			},
			wantError: "user email is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &recordingNotificationHandler{}
			deliveries := &fakeDeliveryRepository{started: true}
			handler := newTestIdempotentEmailHandler(t, next, deliveries)
			notification := todoEmailNotification(model.OutboxEventTodoCreated)
			tt.modify(&notification)

			err := handler.Handle(context.Background(), notification)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Handle() error = %v, want %q", err, tt.wantError)
			}
			if deliveries.tryStartCalls != 0 || next.calls != 0 {
				t.Fatalf(
					"calls after validation failure: TryStart=%d next=%d, want 0 and 0",
					deliveries.tryStartCalls,
					next.calls,
				)
			}
		})
	}

	t.Run("canceled context", func(t *testing.T) {
		next := &recordingNotificationHandler{}
		deliveries := &fakeDeliveryRepository{started: true}
		handler := newTestIdempotentEmailHandler(t, next, deliveries)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := handler.Handle(
			ctx,
			todoEmailNotification(model.OutboxEventTodoCreated),
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Handle() error = %v, want context.Canceled", err)
		}
		if deliveries.tryStartCalls != 0 || next.calls != 0 {
			t.Fatal("dependencies called for canceled context")
		}
	})
}

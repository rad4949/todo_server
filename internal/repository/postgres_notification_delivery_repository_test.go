package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"todo_server/internal/model"

	"github.com/DATA-DOG/go-sqlmock"
)

func newNotificationDeliveryRepositoryMock(t *testing.T) (
	*PostgresNotificationDeliveryRepository,
	sqlmock.Sqlmock,
) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return NewPostgresNotificationDeliveryRepository(db), mock
}

func requireSQLExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestPostgresNotificationDeliveryRepositoryTryStart(t *testing.T) {
	t.Run("starts new delivery", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectQuery("INSERT INTO notification_deliveries").
			WithArgs("event-1", "TodoCreated", "user@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"started"}).AddRow(true))

		started, err := repo.TryStart(
			context.Background(),
			"event-1",
			"TodoCreated",
			"user@example.com",
		)
		if err != nil {
			t.Fatalf("TryStart() error = %v", err)
		}
		if !started {
			t.Fatal("TryStart() started = false, want true")
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("skips duplicate delivery", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectQuery("INSERT INTO notification_deliveries").
			WithArgs("event-1", "TodoCreated", "user@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"started"}))

		started, err := repo.TryStart(
			context.Background(),
			"event-1",
			"TodoCreated",
			"user@example.com",
		)
		if err != nil {
			t.Fatalf("TryStart() error = %v", err)
		}
		if started {
			t.Fatal("TryStart() started = true, want false")
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("wraps database error", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectQuery("INSERT INTO notification_deliveries").
			WithArgs("event-1", "TodoCreated", "user@example.com").
			WillReturnError(errors.New("database unavailable"))

		started, err := repo.TryStart(
			context.Background(),
			"event-1",
			"TodoCreated",
			"user@example.com",
		)
		if err == nil || !strings.Contains(err.Error(), "start notification delivery") {
			t.Fatalf("TryStart() error = %v, want wrapped error", err)
		}
		if started {
			t.Fatal("TryStart() started = true after database error")
		}
		requireSQLExpectations(t, mock)
	})
}

func TestPostgresNotificationDeliveryRepositoryTryStartValidation(t *testing.T) {
	tests := []struct {
		name      string
		eventID   string
		eventType string
		recipient string
		want      string
	}{
		{"missing event ID", "", "TodoCreated", "user@example.com", "event ID is required"},
		{"missing event type", "event-1", "", "user@example.com", "event type is required"},
		{"missing recipient", "event-1", "TodoCreated", "", "recipient is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newNotificationDeliveryRepositoryMock(t)
			started, err := repo.TryStart(
				context.Background(), tt.eventID, tt.eventType, tt.recipient,
			)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("TryStart() error = %v, want %q", err, tt.want)
			}
			if started {
				t.Fatal("TryStart() started = true for invalid input")
			}
			requireSQLExpectations(t, mock)
		})
	}
}

func TestPostgresNotificationDeliveryRepositoryMarkSent(t *testing.T) {
	t.Run("marks processing delivery as sent", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := repo.MarkSent(context.Background(), "event-1"); err != nil {
			t.Fatalf("MarkSent() error = %v", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("rejects non-processing delivery", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.MarkSent(context.Background(), "event-1")
		if err == nil || !strings.Contains(err.Error(), "is not processing") {
			t.Fatalf("MarkSent() error = %v, want state error", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("wraps execution error", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1").
			WillReturnError(errors.New("database unavailable"))

		err := repo.MarkSent(context.Background(), "event-1")
		if err == nil || !strings.Contains(err.Error(), "mark notification delivery sent") {
			t.Fatalf("MarkSent() error = %v, want wrapped error", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("wraps rows affected error", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1").
			WillReturnResult(sqlmock.NewErrorResult(errors.New("result unavailable")))

		err := repo.MarkSent(context.Background(), "event-1")
		if err == nil || !strings.Contains(err.Error(), "rows count") {
			t.Fatalf("MarkSent() error = %v, want rows count error", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("validates event ID", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		err := repo.MarkSent(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "event ID is required") {
			t.Fatalf("MarkSent() error = %v, want validation error", err)
		}
		requireSQLExpectations(t, mock)
	})
}

func TestPostgresNotificationDeliveryRepositoryMarkFailed(t *testing.T) {
	t.Run("marks processing delivery as failed", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1", "SMTP unavailable").
			WillReturnResult(sqlmock.NewResult(0, 1))

		if err := repo.MarkFailed(
			context.Background(), "event-1", "SMTP unavailable",
		); err != nil {
			t.Fatalf("MarkFailed() error = %v", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("rejects non-processing delivery", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1", "SMTP unavailable").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.MarkFailed(context.Background(), "event-1", "SMTP unavailable")
		if err == nil || !strings.Contains(err.Error(), "is not processing") {
			t.Fatalf("MarkFailed() error = %v, want state error", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("wraps execution error", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1", "SMTP unavailable").
			WillReturnError(errors.New("database unavailable"))

		err := repo.MarkFailed(context.Background(), "event-1", "SMTP unavailable")
		if err == nil || !strings.Contains(err.Error(), "mark notification delivery failed") {
			t.Fatalf("MarkFailed() error = %v, want wrapped error", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("wraps rows affected error", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectExec("UPDATE notification_deliveries").
			WithArgs("event-1", "SMTP unavailable").
			WillReturnResult(sqlmock.NewErrorResult(errors.New("result unavailable")))

		err := repo.MarkFailed(context.Background(), "event-1", "SMTP unavailable")
		if err == nil || !strings.Contains(err.Error(), "rows count") {
			t.Fatalf("MarkFailed() error = %v, want rows count error", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("validates input", func(t *testing.T) {
		tests := []struct {
			name      string
			eventID   string
			lastError string
			want      string
		}{
			{"missing event ID", "", "SMTP unavailable", "event ID is required"},
			{"missing error", "event-1", "", "error message is required"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				repo, mock := newNotificationDeliveryRepositoryMock(t)
				err := repo.MarkFailed(
					context.Background(), tt.eventID, tt.lastError,
				)
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("MarkFailed() error = %v, want %q", err, tt.want)
				}
				requireSQLExpectations(t, mock)
			})
		}
	})
}

func TestPostgresNotificationDeliveryRepositoryGetByEventID(t *testing.T) {
	t.Run("returns delivery", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		createdAt := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
		updatedAt := createdAt.Add(time.Minute)
		sentAt := updatedAt.Add(time.Minute)

		mock.ExpectQuery("SELECT(.|[[:space:]])+FROM notification_deliveries").
			WithArgs("event-1").
			WillReturnRows(sqlmock.NewRows([]string{
				"event_id", "event_type", "recipient", "status", "attempts",
				"last_error", "created_at", "updated_at", "sent_at",
			}).AddRow(
				"event-1", "TodoCreated", "user@example.com", "sent", 1,
				nil, createdAt, updatedAt, sentAt,
			))

		delivery, err := repo.GetByEventID(context.Background(), "event-1")
		if err != nil {
			t.Fatalf("GetByEventID() error = %v", err)
		}
		if delivery.EventID != "event-1" ||
			delivery.EventType != "TodoCreated" ||
			delivery.Recipient != "user@example.com" ||
			delivery.Status != model.NotificationDeliveryStatusSent ||
			delivery.Attempts != 1 {
			t.Fatalf("GetByEventID() delivery = %#v", delivery)
		}
		if delivery.LastError != nil {
			t.Fatalf("GetByEventID() LastError = %q, want nil", *delivery.LastError)
		}
		if delivery.SentAt == nil || !delivery.SentAt.Equal(sentAt) {
			t.Fatalf("GetByEventID() SentAt = %v, want %v", delivery.SentAt, sentAt)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("returns failed delivery details", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
		mock.ExpectQuery("SELECT(.|[[:space:]])+FROM notification_deliveries").
			WithArgs("event-2").
			WillReturnRows(sqlmock.NewRows([]string{
				"event_id", "event_type", "recipient", "status", "attempts",
				"last_error", "created_at", "updated_at", "sent_at",
			}).AddRow(
				"event-2", "TodoUpdated", "user@example.com", "failed", 2,
				"SMTP unavailable", now, now, nil,
			))

		delivery, err := repo.GetByEventID(context.Background(), "event-2")
		if err != nil {
			t.Fatalf("GetByEventID() error = %v", err)
		}
		if delivery.Status != model.NotificationDeliveryStatusFailed ||
			delivery.LastError == nil || *delivery.LastError != "SMTP unavailable" ||
			delivery.SentAt != nil {
			t.Fatalf("GetByEventID() delivery = %#v", delivery)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("wraps not found", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectQuery("SELECT(.|[[:space:]])+FROM notification_deliveries").
			WithArgs("missing-event").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetByEventID(context.Background(), "missing-event")
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("GetByEventID() error = %v, want sql.ErrNoRows", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("wraps database error", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		mock.ExpectQuery("SELECT(.|[[:space:]])+FROM notification_deliveries").
			WithArgs("event-1").
			WillReturnError(errors.New("database unavailable"))

		_, err := repo.GetByEventID(context.Background(), "event-1")
		if err == nil || !strings.Contains(err.Error(), "get notification delivery") {
			t.Fatalf("GetByEventID() error = %v, want wrapped error", err)
		}
		requireSQLExpectations(t, mock)
	})

	t.Run("validates event ID", func(t *testing.T) {
		repo, mock := newNotificationDeliveryRepositoryMock(t)
		_, err := repo.GetByEventID(context.Background(), "")
		if err == nil || !strings.Contains(err.Error(), "event ID is required") {
			t.Fatalf("GetByEventID() error = %v, want validation error", err)
		}
		requireSQLExpectations(t, mock)
	})
}

package repository

import (
	"context"
	"database/sql"
	"fmt"

	"todo_server/internal/model"
)

type PostgresNotificationDeliveryRepository struct {
	db *sql.DB
}

var _ NotificationDeliveryRepository = (*PostgresNotificationDeliveryRepository)(nil)

func NewPostgresNotificationDeliveryRepository(
	db *sql.DB,
) *PostgresNotificationDeliveryRepository {
	return &PostgresNotificationDeliveryRepository{
		db: db,
	}
}

func (r *PostgresNotificationDeliveryRepository) TryStart(
	ctx context.Context,
	eventID string,
	eventType string,
	recipient string,
) (bool, error) {
	if eventID == "" {
		return false, fmt.Errorf(
			"start notification delivery: event ID is required",
		)
	}

	if eventType == "" {
		return false, fmt.Errorf(
			"start notification delivery: event type is required",
		)
	}

	if recipient == "" {
		return false, fmt.Errorf(
			"start notification delivery: recipient is required",
		)
	}

	const query = `
		INSERT INTO notification_deliveries (
			event_id,
			event_type,
			recipient,
			status,
			attempts
		)
		VALUES ($1, $2, $3, 'processing', 1)
		ON CONFLICT (event_id) DO UPDATE
		SET
			status = 'processing',
			attempts = notification_deliveries.attempts + 1,
			last_error = NULL,
			updated_at = NOW()
		WHERE notification_deliveries.status = 'failed'
		RETURNING TRUE
	`

	var started bool

	err := r.db.QueryRowContext(
		ctx,
		query,
		eventID,
		eventType,
		recipient,
	).Scan(&started)

	if err == sql.ErrNoRows {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf(
			"start notification delivery: %w",
			err,
		)
	}

	return started, nil
}

func (r *PostgresNotificationDeliveryRepository) MarkSent(
	ctx context.Context,
	eventID string,
) error {
	if eventID == "" {
		return fmt.Errorf(
			"mark notification delivery sent: event ID is required",
		)
	}

	const query = `
		UPDATE notification_deliveries
		SET
			status = 'sent',
			sent_at = NOW(),
			updated_at = NOW(),
			last_error = NULL
		WHERE event_id = $1
		  AND status = 'processing'
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		eventID,
	)
	if err != nil {
		return fmt.Errorf(
			"mark notification delivery sent: %w",
			err,
		)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"get marked notification delivery rows count: %w",
			err,
		)
	}

	if rowsAffected == 0 {
		return fmt.Errorf(
			"mark notification delivery sent: event %q is not processing",
			eventID,
		)
	}

	return nil
}

func (r *PostgresNotificationDeliveryRepository) GetByEventID(
	ctx context.Context,
	eventID string,
) (model.NotificationDelivery, error) {
	if eventID == "" {
		return model.NotificationDelivery{}, fmt.Errorf(
			"get notification delivery: event ID is required",
		)
	}

	const query = `
		SELECT
			event_id,
			event_type,
			recipient,
			status,
			attempts,
			last_error,
			created_at,
			updated_at,
			sent_at
		FROM notification_deliveries
		WHERE event_id = $1
	`

	var delivery model.NotificationDelivery
	var status string

	err := r.db.QueryRowContext(
		ctx,
		query,
		eventID,
	).Scan(
		&delivery.EventID,
		&delivery.EventType,
		&delivery.Recipient,
		&status,
		&delivery.Attempts,
		&delivery.LastError,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
		&delivery.SentAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.NotificationDelivery{}, fmt.Errorf(
				"get notification delivery %q: %w",
				eventID,
				sql.ErrNoRows,
			)
		}

		return model.NotificationDelivery{}, fmt.Errorf(
			"get notification delivery: %w",
			err,
		)
	}

	delivery.Status = model.NotificationDeliveryStatus(status)

	return delivery, nil
}

func (r *PostgresNotificationDeliveryRepository) MarkFailed(
	ctx context.Context,
	eventID string,
	lastError string,
) error {
	if eventID == "" {
		return fmt.Errorf(
			"mark notification delivery failed: event ID is required",
		)
	}

	if lastError == "" {
		return fmt.Errorf(
			"mark notification delivery failed: error message is required",
		)
	}

	const query = `
		UPDATE notification_deliveries
		SET
			status = 'failed',
			last_error = $2,
			updated_at = NOW(),
			sent_at = NULL
		WHERE event_id = $1
		  AND status = 'processing'
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		eventID,
		lastError,
	)
	if err != nil {
		return fmt.Errorf(
			"mark notification delivery failed: %w",
			err,
		)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"get failed notification delivery rows count: %w",
			err,
		)
	}

	if rowsAffected == 0 {
		return fmt.Errorf(
			"mark notification delivery failed: event %q is not processing",
			eventID,
		)
	}

	return nil
}
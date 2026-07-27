package model

import "time"

type NotificationDeliveryStatus string

const (
	NotificationDeliveryStatusProcessing NotificationDeliveryStatus = "processing"
	NotificationDeliveryStatusSent       NotificationDeliveryStatus = "sent"
	NotificationDeliveryStatusFailed     NotificationDeliveryStatus = "failed"
)

type NotificationDelivery struct {
	EventID   string
	EventType string
	Recipient string
	Status    NotificationDeliveryStatus
	Attempts  int
	LastError *string
	CreatedAt time.Time
	UpdatedAt time.Time
	SentAt    *time.Time
}
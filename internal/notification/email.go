package notification

import "context"

type Email struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

type EmailSender interface {
	Send(
		ctx context.Context,
		email Email,
	) error
}
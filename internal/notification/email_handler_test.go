package notification

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"todo_server/internal/event"
	"todo_server/internal/model"
)

type fakeEmailSender struct {
	emails  []Email
	sendErr error
}

func (f *fakeEmailSender) Send(
	_ context.Context,
	email Email,
) error {
	f.emails = append(f.emails, email)

	return f.sendErr
}

func todoEmailNotification(
	eventType model.OutboxEventType,
) TodoNotification {
	email := " user@example.com "

	return TodoNotification{
		Message: event.TodoMessage{
			EventID:       "event-123",
			AggregateType: "todo",
			AggregateID:   "todo-123",
			EventType:     eventType,
			EventVersion:  model.TodoEventVersion,
			OccurredAt:    time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC),
		},
		Payload: model.TodoEventPayload{
			ID:        "todo-123",
			Title:     "Learn email notifications",
			Completed: true,
			UserEmail: &email,
		},
	}
}

func TestBuildTodoEmailSupportsTodoEventTypes(t *testing.T) {
	tests := []struct {
		name      string
		eventType model.OutboxEventType
		action    string
	}{
		{
			name:      "created",
			eventType: model.OutboxEventTodoCreated,
			action:    "created",
		},
		{
			name:      "updated",
			eventType: model.OutboxEventTodoUpdated,
			action:    "updated",
		},
		{
			name:      "deleted",
			eventType: model.OutboxEventTodoDeleted,
			action:    "deleted",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email, err := buildTodoEmail(
				todoEmailNotification(test.eventType),
			)
			if err != nil {
				t.Fatalf("buildTodoEmail() error: %v", err)
			}

			if !strings.Contains(email.Subject, test.action) {
				t.Errorf(
					"Subject = %q, want action %q",
					email.Subject,
					test.action,
				)
			}

			if !strings.Contains(email.TextBody, "todo-123") {
				t.Errorf("TextBody does not contain todo ID: %q", email.TextBody)
			}

			if !strings.Contains(email.TextBody, "event-123") {
				t.Errorf("TextBody does not contain event ID: %q", email.TextBody)
			}

			if !strings.Contains(email.HTMLBody, test.action) {
				t.Errorf("HTMLBody does not contain action: %q", email.HTMLBody)
			}
		})
	}
}

func TestEmailHandlerHandleSendsEmail(t *testing.T) {
	sender := &fakeEmailSender{}
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))

	handler, err := NewEmailHandler(sender, logger)
	if err != nil {
		t.Fatalf("NewEmailHandler() error: %v", err)
	}

	err = handler.Handle(
		context.Background(),
		todoEmailNotification(model.OutboxEventTodoCreated),
	)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}

	if len(sender.emails) != 1 {
		t.Fatalf("sent emails = %d, want 1", len(sender.emails))
	}

	sent := sender.emails[0]
	if sent.To != "user@example.com" {
		t.Errorf("To = %q, want user@example.com", sent.To)
	}

	if !strings.Contains(sent.Subject, "Todo created") {
		t.Errorf("Subject = %q, want created subject", sent.Subject)
	}

	logOutput := logBuffer.String()
	for _, expected := range []string{
		"todo notification email sent",
		"event-123",
		"user@example.com",
	} {
		if !strings.Contains(logOutput, expected) {
			t.Errorf("log output does not contain %q: %s", expected, logOutput)
		}
	}
}

func TestEmailHandlerHandleReturnsSenderError(t *testing.T) {
	sender := &fakeEmailSender{
		sendErr: errors.New("SMTP unavailable"),
	}
	handler, err := NewEmailHandler(
		sender,
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	)
	if err != nil {
		t.Fatalf("NewEmailHandler() error: %v", err)
	}

	err = handler.Handle(
		context.Background(),
		todoEmailNotification(model.OutboxEventTodoUpdated),
	)
	if err == nil {
		t.Fatal("expected Handle() error, got nil")
	}

	if !strings.Contains(err.Error(), "SMTP unavailable") {
		t.Errorf("error = %q, want SMTP error", err)
	}

	if !strings.Contains(err.Error(), "event-123") {
		t.Errorf("error = %q, want event ID", err)
	}
}

func TestEmailHandlerValidatesDependenciesAndEmail(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	if handler, err := NewEmailHandler(nil, logger); err == nil || handler != nil {
		t.Fatal("expected missing sender error")
	}

	if handler, err := NewEmailHandler(&fakeEmailSender{}, nil); err == nil || handler != nil {
		t.Fatal("expected missing logger error")
	}

	handler, err := NewEmailHandler(&fakeEmailSender{}, logger)
	if err != nil {
		t.Fatalf("NewEmailHandler() error: %v", err)
	}

	notification := todoEmailNotification(model.OutboxEventTodoCreated)
	notification.Payload.UserEmail = nil

	err = handler.Handle(context.Background(), notification)
	if err == nil || !strings.Contains(err.Error(), "user email is required") {
		t.Errorf("error = %v, want user email validation error", err)
	}
}

func TestBuildTodoEmailEscapesHTML(t *testing.T) {
	notification := todoEmailNotification(model.OutboxEventTodoUpdated)
	notification.Payload.Title = `<script>alert("x")</script>`

	email, err := buildTodoEmail(notification)
	if err != nil {
		t.Fatalf("buildTodoEmail() error: %v", err)
	}

	if strings.Contains(email.HTMLBody, "<script>") {
		t.Errorf("HTMLBody contains unescaped script tag: %s", email.HTMLBody)
	}

	if !strings.Contains(email.HTMLBody, "&lt;script&gt;") {
		t.Errorf("HTMLBody does not contain escaped title: %s", email.HTMLBody)
	}
}

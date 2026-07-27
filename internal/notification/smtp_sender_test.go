package notification

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func validSMTPConfig() SMTPConfig {
	return SMTPConfig{
		Host:     "smtp.gmail.com",
		Port:     587,
		Username: "sender@example.com",
		Password: "app-password",
		From:     "sender@example.com",
		FromName: "Todo Notifications",
		Timeout:  15 * time.Second,
	}
}

func TestNewSMTPSenderValidatesConfig(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*SMTPConfig)
		wantError string
	}{
		{
			name: "missing host",
			mutate: func(config *SMTPConfig) {
				config.Host = " "
			},
			wantError: "host is required",
		},
		{
			name: "invalid port",
			mutate: func(config *SMTPConfig) {
				config.Port = 0
			},
			wantError: "port must be between 1 and 65535",
		},
		{
			name: "missing username",
			mutate: func(config *SMTPConfig) {
				config.Username = ""
			},
			wantError: "username is required",
		},
		{
			name: "missing password",
			mutate: func(config *SMTPConfig) {
				config.Password = ""
			},
			wantError: "password is required",
		},
		{
			name: "missing from address",
			mutate: func(config *SMTPConfig) {
				config.From = ""
			},
			wantError: "from address is required",
		},
		{
			name: "invalid from address",
			mutate: func(config *SMTPConfig) {
				config.From = "not-an-email"
			},
			wantError: "invalid from address",
		},
		{
			name: "invalid timeout",
			mutate: func(config *SMTPConfig) {
				config.Timeout = 0
			},
			wantError: "timeout must be positive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validSMTPConfig()
			test.mutate(&config)

			sender, err := NewSMTPSender(config)
			if sender != nil {
				t.Fatal("expected nil sender")
			}

			if err == nil {
				t.Fatal("expected NewSMTPSender() error, got nil")
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

func TestNewSMTPSenderNormalizesConfig(t *testing.T) {
	config := validSMTPConfig()
	config.Host = " smtp.gmail.com "
	config.Username = " sender@example.com "
	config.From = " Todo Sender <sender@example.com> "
	config.FromName = " "

	sender, err := NewSMTPSender(config)
	if err != nil {
		t.Fatalf("NewSMTPSender() error: %v", err)
	}

	if sender.config.Host != "smtp.gmail.com" {
		t.Errorf("Host = %q, want smtp.gmail.com", sender.config.Host)
	}

	if sender.config.Username != "sender@example.com" {
		t.Errorf(
			"Username = %q, want sender@example.com",
			sender.config.Username,
		)
	}

	if sender.config.From != "sender@example.com" {
		t.Errorf("From = %q, want sender@example.com", sender.config.From)
	}

	if sender.config.FromName != "Todo Notifications" {
		t.Errorf(
			"FromName = %q, want Todo Notifications",
			sender.config.FromName,
		)
	}
}

func TestSMTPSenderBuildMessageCreatesMultipartEmail(t *testing.T) {
	sender, err := NewSMTPSender(validSMTPConfig())
	if err != nil {
		t.Fatalf("NewSMTPSender() error: %v", err)
	}

	messageBytes, err := sender.buildMessage(
		"recipient@example.com",
		Email{
			Subject:  "Todo оновлено",
			TextBody: "Todo was updated",
			HTMLBody: "<strong>Todo was updated</strong>",
		},
	)
	if err != nil {
		t.Fatalf("buildMessage() error: %v", err)
	}

	message, err := mail.ReadMessage(
		bufio.NewReader(bytes.NewReader(messageBytes)),
	)
	if err != nil {
		t.Fatalf("read MIME message: %v", err)
	}

	if !strings.Contains(message.Header.Get("From"), "sender@example.com") {
		t.Errorf("From header = %q", message.Header.Get("From"))
	}

	if message.Header.Get("To") != "<recipient@example.com>" {
		t.Errorf(
			"To header = %q, want recipient@example.com",
			message.Header.Get("To"),
		)
	}

	decodedSubject, err := (&mime.WordDecoder{}).DecodeHeader(
		message.Header.Get("Subject"),
	)
	if err != nil {
		t.Fatalf("decode subject: %v", err)
	}

	if decodedSubject != "Todo оновлено" {
		t.Errorf("Subject = %q, want Todo оновлено", decodedSubject)
	}

	mediaType, params, err := mime.ParseMediaType(
		message.Header.Get("Content-Type"),
	)
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}

	if mediaType != "multipart/alternative" {
		t.Errorf("media type = %q, want multipart/alternative", mediaType)
	}

	reader := multipart.NewReader(message.Body, params["boundary"])
	parts := make(map[string]string)

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read MIME part: %v", err)
		}

		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read MIME part body: %v", err)
		}

		partType, _, err := mime.ParseMediaType(
			part.Header.Get("Content-Type"),
		)
		if err != nil {
			t.Fatalf("parse MIME part type: %v", err)
		}

		parts[partType] = string(content)
	}

	if parts["text/plain"] != "Todo was updated" {
		t.Errorf("text body = %q", parts["text/plain"])
	}

	if parts["text/html"] != "<strong>Todo was updated</strong>" {
		t.Errorf("HTML body = %q", parts["text/html"])
	}
}

func TestSMTPSenderSendValidatesBeforeConnecting(t *testing.T) {
	sender, err := NewSMTPSender(validSMTPConfig())
	if err != nil {
		t.Fatalf("NewSMTPSender() error: %v", err)
	}

	tests := []struct {
		name      string
		ctx       func() context.Context
		email     Email
		wantError string
	}{
		{
			name: "cancelled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			email: Email{
				To:       "recipient@example.com",
				Subject:  "Subject",
				TextBody: "Body",
			},
			wantError: "context canceled",
		},
		{
			name: "invalid recipient",
			ctx:  context.Background,
			email: Email{
				To:       "invalid",
				Subject:  "Subject",
				TextBody: "Body",
			},
			wantError: "invalid recipient",
		},
		{
			name: "missing subject",
			ctx:  context.Background,
			email: Email{
				To:       "recipient@example.com",
				TextBody: "Body",
			},
			wantError: "subject is required",
		},
		{
			name: "missing body",
			ctx:  context.Background,
			email: Email{
				To:      "recipient@example.com",
				Subject: "Subject",
			},
			wantError: "body is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := sender.Send(test.ctx(), test.email)
			if err == nil {
				t.Fatal("expected Send() error, got nil")
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

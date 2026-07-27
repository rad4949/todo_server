package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	Timeout  time.Duration
}

type SMTPSender struct {
	config SMTPConfig
}

var _ EmailSender = (*SMTPSender)(nil)

func NewSMTPSender(
	config SMTPConfig,
) (*SMTPSender, error) {
	config.Host = strings.TrimSpace(config.Host)
	config.Username = strings.TrimSpace(
		config.Username,
	)
	config.From = strings.TrimSpace(config.From)
	config.FromName = strings.TrimSpace(
		config.FromName,
	)

	if config.Host == "" {
		return nil, fmt.Errorf(
			"create SMTP sender: host is required",
		)
	}

	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf(
			"create SMTP sender: port must be between 1 and 65535",
		)
	}

	if config.Username == "" {
		return nil, fmt.Errorf(
			"create SMTP sender: username is required",
		)
	}

	if config.Password == "" {
		return nil, fmt.Errorf(
			"create SMTP sender: password is required",
		)
	}

	if config.From == "" {
		return nil, fmt.Errorf(
			"create SMTP sender: from address is required",
		)
	}

	fromAddress, err := mail.ParseAddress(
		config.From,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create SMTP sender: invalid from address: %w",
			err,
		)
	}

	// Store only the normalized email address.
	config.From = fromAddress.Address

	if config.FromName == "" {
		config.FromName = "Todo Notifications"
	}

	if config.Timeout <= 0 {
		return nil, fmt.Errorf(
			"create SMTP sender: timeout must be positive",
		)
	}

	return &SMTPSender{
		config: config,
	}, nil
}

func (s *SMTPSender) Send(
	ctx context.Context,
	email Email,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"send SMTP email: %w",
			err,
		)
	}

	recipient, err := mail.ParseAddress(
		strings.TrimSpace(email.To),
	)
	if err != nil {
		return fmt.Errorf(
			"send SMTP email: invalid recipient: %w",
			err,
		)
	}

	if strings.TrimSpace(email.Subject) == "" {
		return fmt.Errorf(
			"send SMTP email: subject is required",
		)
	}

	if email.TextBody == "" &&
		email.HTMLBody == "" {
		return fmt.Errorf(
			"send SMTP email: body is required",
		)
	}

	message, err := s.buildMessage(
		recipient.Address,
		email,
	)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(
		s.config.Host,
		strconv.Itoa(s.config.Port),
	)

	dialer := net.Dialer{
		Timeout: s.config.Timeout,
	}

	connection, err := dialer.DialContext(
		ctx,
		"tcp",
		address,
	)
	if err != nil {
		return fmt.Errorf(
			"connect to SMTP server %s: %w",
			address,
			err,
		)
	}
	defer connection.Close()

	deadline := time.Now().Add(
		s.config.Timeout,
	)

	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}

	if err := connection.SetDeadline(deadline); err != nil {
		return fmt.Errorf(
			"set SMTP connection deadline: %w",
			err,
		)
	}

	client, err := smtp.NewClient(
		connection,
		s.config.Host,
	)
	if err != nil {
		return fmt.Errorf(
			"create SMTP client: %w",
			err,
		)
	}
	defer client.Close()

	if supported, _ := client.Extension(
		"STARTTLS",
	); !supported {
		return fmt.Errorf(
			"SMTP server does not support STARTTLS",
		)
	}

	if err := client.StartTLS(
		&tls.Config{
			ServerName: s.config.Host,
			MinVersion: tls.VersionTLS12,
		},
	); err != nil {
		return fmt.Errorf(
			"start SMTP TLS: %w",
			err,
		)
	}

	auth := smtp.PlainAuth(
		"",
		s.config.Username,
		s.config.Password,
		s.config.Host,
	)

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf(
			"authenticate with SMTP server: %w",
			err,
		)
	}

	if err := client.Mail(s.config.From); err != nil {
		return fmt.Errorf(
			"set SMTP sender: %w",
			err,
		)
	}

	if err := client.Rcpt(
		recipient.Address,
	); err != nil {
		return fmt.Errorf(
			"set SMTP recipient: %w",
			err,
		)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf(
			"start SMTP message: %w",
			err,
		)
	}

	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()

		return fmt.Errorf(
			"write SMTP message: %w",
			err,
		)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf(
			"finish SMTP message: %w",
			err,
		)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf(
			"quit SMTP session: %w",
			err,
		)
	}

	return nil
}

func (s *SMTPSender) buildMessage(
	recipient string,
	email Email,
) ([]byte, error) {
	var body bytes.Buffer

	multipartWriter := multipart.NewWriter(&body)

	from := (&mail.Address{
		Name:    s.config.FromName,
		Address: s.config.From,
	}).String()

	to := (&mail.Address{
		Address: recipient,
	}).String()

	subject := mime.QEncoding.Encode(
		"UTF-8",
		email.Subject,
	)

	fmt.Fprintf(
		&body,
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Date: %s\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: multipart/alternative; boundary=%q\r\n"+
			"\r\n",
		from,
		to,
		subject,
		time.Now().UTC().Format(time.RFC1123Z),
		multipartWriter.Boundary(),
	)

	if email.TextBody != "" {
		headers := textproto.MIMEHeader{}
		headers.Set(
			"Content-Type",
			`text/plain; charset="UTF-8"`,
		)
		headers.Set(
			"Content-Transfer-Encoding",
			"8bit",
		)

		part, err := multipartWriter.CreatePart(
			headers,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create text email part: %w",
				err,
			)
		}

		if _, err := part.Write(
			[]byte(email.TextBody),
		); err != nil {
			return nil, fmt.Errorf(
				"write text email part: %w",
				err,
			)
		}
	}

	if email.HTMLBody != "" {
		headers := textproto.MIMEHeader{}
		headers.Set(
			"Content-Type",
			`text/html; charset="UTF-8"`,
		)
		headers.Set(
			"Content-Transfer-Encoding",
			"8bit",
		)

		part, err := multipartWriter.CreatePart(
			headers,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create HTML email part: %w",
				err,
			)
		}

		if _, err := part.Write(
			[]byte(email.HTMLBody),
		); err != nil {
			return nil, fmt.Errorf(
				"write HTML email part: %w",
				err,
			)
		}
	}

	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf(
			"finish MIME email: %w",
			err,
		)
	}

	return body.Bytes(), nil
}

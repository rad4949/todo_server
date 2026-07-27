package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo_server/internal/config"
	"todo_server/internal/notification"
	"todo_server/internal/repository"

	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		slog.Error(
			"notification service stopped with error",
			"error",
			err,
		)

		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf(
			"load notification config: %w",
			err,
		)
	}

	logger := slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level: slog.LevelInfo,
			},
		),
	)

	db, err := openPostgres(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	logger.Info(
		"notification service connected to PostgreSQL",
		"host",
		cfg.DBHost,
		"port",
		cfg.DBPort,
		"database",
		cfg.DBName,
	)

	deliveryRepository :=
		repository.NewPostgresNotificationDeliveryRepository(db)

	smtpSender, err := notification.NewSMTPSender(
		notification.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			FromName: cfg.SMTPFromName,
			Timeout:  cfg.SMTPTimeout,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"create SMTP sender: %w",
			err,
		)
	}

	emailHandler, err := notification.NewEmailHandler(
		smtpSender,
		logger,
	)
	if err != nil {
		return fmt.Errorf(
			"create email notification handler: %w",
			err,
		)
	}

	idempotentEmailHandler, err :=
		notification.NewIdempotentEmailHandler(
			emailHandler,
			deliveryRepository,
			logger,
		)
	if err != nil {
		return fmt.Errorf(
			"create idempotent email handler: %w",
			err,
		)
	}

	consumer, err := notification.NewConsumer(
		notification.ConsumerConfig{
			Brokers:   cfg.KafkaBrokers,
			Topic:     cfg.KafkaTopic,
			GroupID:   cfg.KafkaConsumerGroup,
			ClientID:  cfg.KafkaConsumerID,
			BatchSize: cfg.KafkaConsumerBatchSize,
		},
		idempotentEmailHandler,
	)
	if err != nil {
		return fmt.Errorf(
			"create notification consumer: %w",
			err,
		)
	}
	defer consumer.Close()

	pingCtx, cancelPing := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)

	err = consumer.Ping(pingCtx)
	cancelPing()

	if err != nil {
		return fmt.Errorf(
			"connect notification consumer to Kafka: %w",
			err,
		)
	}

	logger.Info(
		"notification consumer connected to Kafka",
		"brokers",
		cfg.KafkaBrokers,
		"topic",
		cfg.KafkaTopic,
		"group_id",
		cfg.KafkaConsumerGroup,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	logger.Info(
		"notification consumer started",
		"topic",
		cfg.KafkaTopic,
		"group_id",
		cfg.KafkaConsumerGroup,
	)

	if err := consumer.Run(ctx); err != nil {
		return fmt.Errorf(
			"run notification consumer: %w",
			err,
		)
	}

	logger.Info(
		"notification consumer stopped",
	)

	return nil
}

func openPostgres(
	cfg *config.Config,
) (*sql.DB, error) {
	connectionString := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	db, err := sql.Open(
		"postgres",
		connectionString,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open notification PostgreSQL connection: %w",
			err,
		)
	}

	pingCtx, cancelPing := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancelPing()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()

		return nil, fmt.Errorf(
			"connect notification service to PostgreSQL: %w",
			err,
		)
	}

	return db, nil
}
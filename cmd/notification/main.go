package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todo_server/internal/config"
	"todo_server/internal/notification"
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

	handler, err := notification.NewLoggingHandler(
		logger,
	)
	if err != nil {
		return fmt.Errorf(
			"create notification handler: %w",
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
		handler,
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

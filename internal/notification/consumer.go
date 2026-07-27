package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
)

var ErrInvalidNotification = errors.New(
	"invalid todo notification",
)

type ConsumerConfig struct {
	Brokers   []string
	Topic     string
	GroupID   string
	ClientID  string
	BatchSize int
}

type consumerClient interface {
	PollRecords(
		ctx context.Context,
		maxRecords int,
	) kgo.Fetches

	CommitRecords(
		ctx context.Context,
		records ...*kgo.Record,
	) error

	Ping(ctx context.Context) error
	Close()
}

type Consumer struct {
	client    consumerClient
	handler   Handler
	batchSize int
}

func NewConsumer(
	config ConsumerConfig,
	handler Handler,
) (*Consumer, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf(
			"create notification consumer: at least one broker is required",
		)
	}

	if config.Topic == "" {
		return nil, fmt.Errorf(
			"create notification consumer: topic is required",
		)
	}

	if config.GroupID == "" {
		return nil, fmt.Errorf(
			"create notification consumer: group ID is required",
		)
	}

	if config.ClientID == "" {
		return nil, fmt.Errorf(
			"create notification consumer: client ID is required",
		)
	}

	if config.BatchSize <= 0 {
		return nil, fmt.Errorf(
			"create notification consumer: batch size must be positive",
		)
	}

	if handler == nil {
		return nil, fmt.Errorf(
			"create notification consumer: handler is required",
		)
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(config.Brokers...),
		kgo.ClientID(config.ClientID),
		kgo.ConsumerGroup(config.GroupID),
		kgo.ConsumeTopics(config.Topic),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(
			kgo.NewOffset().AtEnd(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create Kafka notification client: %w",
			err,
		)
	}

	return &Consumer{
		client:    client,
		handler:   handler,
		batchSize: config.BatchSize,
	}, nil
}

func (c *Consumer) Ping(
	ctx context.Context,
) error {
	if err := c.client.Ping(ctx); err != nil {
		return fmt.Errorf(
			"ping Kafka notification consumer: %w",
			err,
		)
	}

	return nil
}

func (c *Consumer) Close() {
	c.client.Close()
}

func (c *Consumer) processRecord(
	ctx context.Context,
	record *kgo.Record,
) error {
	if record == nil {
		return fmt.Errorf(
			"process Kafka notification record: record is nil",
		)
	}

	notification, err := DecodeTodoNotification(
		record.Value,
	)
	if err != nil {
		return fmt.Errorf(
			"%w: topic=%s partition=%d offset=%d: %v",
			ErrInvalidNotification,
			record.Topic,
			record.Partition,
			record.Offset,
			err,
		)
	}

	if err := c.handler.Handle(
		ctx,
		notification,
	); err != nil {
		return fmt.Errorf(
			"handle Kafka notification event %s topic=%s partition=%d offset=%d: %w",
			notification.Message.EventID,
			record.Topic,
			record.Partition,
			record.Offset,
			err,
		)
	}

	return nil
}

func (c *Consumer) processAndCommitRecord(
	ctx context.Context,
	record *kgo.Record,
) error {
	if err := c.processRecord(
		ctx,
		record,
	); err != nil {
		return err
	}

	if err := c.client.CommitRecords(
		ctx,
		record,
	); err != nil {
		return fmt.Errorf(
			"commit Kafka notification record topic=%s partition=%d offset=%d: %w",
			record.Topic,
			record.Partition,
			record.Offset,
			err,
		)
	}

	return nil
}

func (c *Consumer) Run(
	ctx context.Context,
) error {
	for {
		fetches := c.client.PollRecords(
			ctx,
			c.batchSize,
		)

		if err := ctx.Err(); err != nil {
			return nil
		}

		fetchErrors := fetches.Errors()
		if len(fetchErrors) > 0 {
			fetchError := fetchErrors[0]

			return fmt.Errorf(
				"poll Kafka notification records topic=%s partition=%d: %w",
				fetchError.Topic,
				fetchError.Partition,
				fetchError.Err,
			)
		}

		iterator := fetches.RecordIter()

		for !iterator.Done() {
			record := iterator.Next()

			err := c.processAndCommitRecord(
				ctx,
				record,
			)
			if err == nil {
				continue
			}

			if !errors.Is(
				err,
				ErrInvalidNotification,
			) {
				return err
			}

			slog.ErrorContext(
				ctx,
				"invalid Kafka notification skipped",
				"topic",
				record.Topic,
				"partition",
				record.Partition,
				"offset",
				record.Offset,
				"error",
				err,
			)

			if commitErr := c.client.CommitRecords(
				ctx,
				record,
			); commitErr != nil {
				return fmt.Errorf(
					"commit invalid Kafka notification topic=%s partition=%d offset=%d: %w",
					record.Topic,
					record.Partition,
					record.Offset,
					commitErr,
				)
			}
		}
	}
}

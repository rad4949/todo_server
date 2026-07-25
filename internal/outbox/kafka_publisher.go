package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"todo_server/internal/model"

	"github.com/twmb/franz-go/pkg/kgo"
)

type kafkaProducer interface {
	ProduceSync(
		ctx context.Context,
		records ...*kgo.Record,
	) kgo.ProduceResults

	Ping(ctx context.Context) error
	Close()
}

type KafkaPublisher struct {
	client kafkaProducer
	topic  string
}

var _ Publisher = (*KafkaPublisher)(nil)

func NewKafkaPublisher(
	brokers []string,
	topic string,
) (*KafkaPublisher, error) {
	if len(brokers) == 0 {
		return nil, fmt.Errorf(
			"create Kafka publisher: at least one broker is required",
		)
	}

	if topic == "" {
		return nil, fmt.Errorf(
			"create Kafka publisher: topic is required",
		)
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ClientID("todo-outbox-publisher"),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create Kafka client: %w",
			err,
		)
	}

	return &KafkaPublisher{
		client: client,
		topic:  topic,
	}, nil
}

func (p *KafkaPublisher) Publish(
	ctx context.Context,
	event model.OutboxEvent,
) error {
	if event.ID == "" {
		return fmt.Errorf(
			"publish Kafka event: event ID is required",
		)
	}

	if event.AggregateID == "" {
		return fmt.Errorf(
			"publish Kafka event: aggregate ID is required",
		)
	}

	message := MessageFromEvent(event)

	value, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf(
			"marshal Kafka message: %w",
			err,
		)
	}

	record := &kgo.Record{
		Topic:     p.topic,
		Key:       []byte(event.AggregateID),
		Value:     value,
		Timestamp: event.CreatedAt,
		Headers: []kgo.RecordHeader{
			{
				Key:   "event_id",
				Value: []byte(event.ID),
			},
			{
				Key: "event_type",
				Value: []byte(
					string(event.EventType),
				),
			},
			{
				Key: "event_version",
				Value: []byte(
					strconv.Itoa(event.EventVersion),
				),
			},
		},
	}

	if err := p.client.
		ProduceSync(ctx, record).
		FirstErr(); err != nil {
		return fmt.Errorf(
			"publish event %s to Kafka: %w",
			event.ID,
			err,
		)
	}

	return nil
}

func (p *KafkaPublisher) Ping(
	ctx context.Context,
) error {
	if err := p.client.Ping(ctx); err != nil {
		return fmt.Errorf(
			"ping Kafka broker: %w",
			err,
		)
	}

	return nil
}

func (p *KafkaPublisher) Close() {
	p.client.Close()
}
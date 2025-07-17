package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log-processor/model"
	"log/slog"

	"github.com/IBM/sarama"
)

type KafkaConsumer struct {
	consumer sarama.ConsumerGroup
	topic    string
	ready    chan bool
	logger   *slog.Logger
	batch    []model.LogEntry
	batchCh  chan model.LogEntry
}

func NewKafkaConsumer(brokers []string, topic, group string, logger *slog.Logger) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()

	consumer, err := sarama.NewConsumerGroup(brokers, group, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return &KafkaConsumer{
		consumer: consumer,
		topic:    topic,
		ready:    make(chan bool),
		logger:   logger,
		batch:    make([]model.LogEntry, 0),
		batchCh:  make(chan model.LogEntry, 10000),
	}, nil
}

func (kc *KafkaConsumer) Start(ctx context.Context) error {
	go func() {
		for {
			if err := kc.consumer.Consume(ctx, []string{kc.topic}, kc); err != nil {
				kc.logger.Error("Error from consumer", "error", err)
				return
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	<-kc.ready
	kc.logger.Info("Kafka consumer started")
	return nil
}

func (kc *KafkaConsumer) Setup(sarama.ConsumerGroupSession) error {
	close(kc.ready)
	return nil
}

func (kc *KafkaConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (kc *KafkaConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			var logEntry model.LogEntry
			if err := json.Unmarshal(message.Value, &logEntry); err != nil {
				kc.logger.Error("Failed to unmarshal log entry", "error", err)
				session.MarkMessage(message, "")
				continue
			}

			select {
			case kc.batchCh <- logEntry:
				session.MarkMessage(message, "")
			case <-session.Context().Done():
				return nil
			}

		case <-session.Context().Done():
			return nil
		}
	}
}

func (kc *KafkaConsumer) GetBatchChannel() <-chan model.LogEntry {
	return kc.batchCh
}

func (kc *KafkaConsumer) Close() error {
	close(kc.batchCh)
	return kc.consumer.Close()
}

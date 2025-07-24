package consumer

import (
	"context"
	"encoding/json"
	"eventer/model"
	"fmt"
	"log/slog"
	"time"

	"github.com/IBM/sarama"
)

type KafkaConsumer struct {
	consumer sarama.ConsumerGroup
	topic    string
	ready    chan bool
	logger   *slog.Logger
	batchCh  chan model.LogEntry
}

func NewKafkaConsumer(brokers []string, topic, group string, logger *slog.Logger) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRoundRobin()

	config.Consumer.Fetch.Min = 1024 * 1024          // 1MB minimum fetch size
	config.Consumer.Fetch.Default = 10 * 1024 * 1024 // 10MB default fetch size
	config.Consumer.MaxProcessingTime = 30 * time.Second

	consumer, err := sarama.NewConsumerGroup(brokers, group, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return &KafkaConsumer{
		consumer: consumer,
		topic:    topic,
		ready:    make(chan bool),
		logger:   logger,
		batchCh:  make(chan model.LogEntry, 50000),
	}, nil
}

func (kc *KafkaConsumer) Start(ctx context.Context) error {
	go func() {
		for err := range kc.consumer.Errors() {
			kc.logger.Error("Kafka consumer error", "error", err)
		}
	}()

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

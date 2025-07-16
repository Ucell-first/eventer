package kafka

import (
	"context"
	"encoding/json"
	"log-processor/config"
	"log-processor/model"
	"log/slog"
	"strings"

	"github.com/IBM/sarama"
)

type Consumer struct {
	consumer sarama.Consumer
	logger   *slog.Logger
	config   *config.Config
}

func NewConsumer(cfg *config.Config, logger *slog.Logger) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	consumer, err := sarama.NewConsumer(strings.Split(cfg.Kafka.BrokerURL, ","), config)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		consumer: consumer,
		logger:   logger,
		config:   cfg,
	}, nil
}

func (c *Consumer) Consume(ctx context.Context, logChan chan<- model.LogEntry) error {
	partitionConsumer, err := c.consumer.ConsumePartition(c.config.Kafka.Topic, 0, sarama.OffsetNewest)
	if err != nil {
		return err
	}
	defer partitionConsumer.Close()

	for {
		select {
		case message := <-partitionConsumer.Messages():
			var logEntry model.LogEntry
			if err := json.Unmarshal(message.Value, &logEntry); err != nil {
				c.logger.Error("failed to parse log message", "error", err)
				continue
			}

			select {
			case logChan <- logEntry:
			case <-ctx.Done():
				return ctx.Err()
			}

		case err := <-partitionConsumer.Errors():
			c.logger.Error("kafka consumer error", "error", err)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Consumer) Close() error {
	return c.consumer.Close()
}

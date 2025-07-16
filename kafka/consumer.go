package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log-processor/config"
	"log-processor/model"
	"log/slog"
	"net"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
	config *config.Config
	log    *slog.Logger
}

// JSON xabar formati (Kafka'dan keladi)
type LogMessage struct {
	Timestamp string `json:"timestamp"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    uint16 `json:"status"`
	LatencyMs uint32 `json:"latency_ms"`
	IP        string `json:"ip"`
}

// Yangi Kafka consumer yaratish
func NewConsumer(cfg *config.Config, log *slog.Logger) *Consumer {
	// Kafka reader sozlash
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{cfg.Kafka.BrokerURL}, // Kafka server address
		Topic:    cfg.Kafka.Topic,               // Topic nomi
		GroupID:  cfg.Kafka.ConsumerGroup,       // Consumer group
		MinBytes: 10e3,                          // 10KB minimum
		MaxBytes: 10e6,                          // 10MB maximum
	})

	return &Consumer{
		reader: reader,
		config: cfg,
		log:    log,
	}
}

// Kafka'dan xabarlarni o'qish
func (c *Consumer) Consume(ctx context.Context, logChan chan<- model.LogEntry) error {
	c.log.Info("Starting Kafka consumer", "topic", c.config.Kafka.Topic)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Kafka consumer stopped")
			return nil
		default:
			// Kafka'dan xabar o'qish
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				c.log.Error("Error reading message from Kafka", "error", err)
				continue
			}

			// JSON'ni LogEntry'ga o'tkazish
			logEntry, err := c.parseMessage(msg.Value)
			if err != nil {
				c.log.Error("Failed to parse message", "error", err)
				continue
			}

			// Batch processor'ga yuborish
			select {
			case logChan <- *logEntry:
				c.log.Debug("Message sent to batch processor")
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// JSON xabarni LogEntry struct'ga o'tkazish
func (c *Consumer) parseMessage(data []byte) (*model.LogEntry, error) {
	var logMsg LogMessage

	// JSON parse qilish
	if err := json.Unmarshal(data, &logMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Timestamp parse qilish
	timestamp, err := time.Parse(time.RFC3339, logMsg.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// IP address parse qilish
	ip := net.ParseIP(logMsg.IP)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", logMsg.IP)
	}

	// LogEntry struct yaratish
	return &model.LogEntry{
		Timestamp: timestamp,
		Method:    logMsg.Method,
		Path:      logMsg.Path,
		Status:    logMsg.Status,
		LatencyMs: logMsg.LatencyMs,
		IP:        ip,
	}, nil
}

// Consumer'ni yopish
func (c *Consumer) Close() error {
	return c.reader.Close()
}

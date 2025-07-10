package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

type Config struct {
	Kafka      KafkaConfig
	ClickHouse ClickHouseConfig
	Batch      BatchConfig
	Server     ServerConfig
	Logging    LoggingConfig
}

type KafkaConfig struct {
	BrokerURL     string
	Topic         string
	ConsumerGroup string
	Username      string
	Password      string
}

type ClickHouseConfig struct {
	URL      string
	Database string
	Username string
	Password string
	Table    string
}

type BatchConfig struct {
	Size          int
	FlushInterval time.Duration
	MaxRetries    int
}

type ServerConfig struct {
	Port        string
	MetricsPort string
	MetricsPath string
	HealthPath  string
}

type LoggingConfig struct {
	Level  string
	Format string
}

func Load() *Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("error while loading .env file: %v", err)
	}

	flushInterval, err := time.ParseDuration(cast.ToString(coalesce("FLUSH_INTERVAL", "5s")))
	if err != nil {
		log.Printf("invalid FLUSH_INTERVAL, using default 5s: %v", err)
		flushInterval = 5 * time.Second
	}

	return &Config{
		Kafka: KafkaConfig{
			BrokerURL:     cast.ToString(coalesce("BROKER_URL", "localhost:9092")),
			Topic:         cast.ToString(coalesce("TOPIC", "logs")),
			ConsumerGroup: cast.ToString(coalesce("CONSUMER_GROUP", "log-processor")),
			Username:      cast.ToString(coalesce("KAFKA_USERNAME", "")),
			Password:      cast.ToString(coalesce("KAFKA_PASSWORD", "")),
		},
		ClickHouse: ClickHouseConfig{
			URL:      cast.ToString(coalesce("CLICKHOUSE_URL", "http://localhost:8123")),
			Database: cast.ToString(coalesce("CLICKHOUSE_DB", "default")),
			Username: cast.ToString(coalesce("CLICKHOUSE_USER", "default")),
			Password: cast.ToString(coalesce("CLICKHOUSE_PASSWORD", "")),
			Table:    cast.ToString(coalesce("CLICKHOUSE_TABLE", "logs")),
		},
		Batch: BatchConfig{
			Size:          cast.ToInt(coalesce("BATCH_SIZE", 1000)),
			FlushInterval: flushInterval,
			MaxRetries:    cast.ToInt(coalesce("MAX_RETRIES", 3)),
		},
		Server: ServerConfig{
			Port:        cast.ToString(coalesce("SERVER_PORT", ":8080")),
			MetricsPort: cast.ToString(coalesce("METRICS_PORT", ":8081")),
			MetricsPath: cast.ToString(coalesce("METRICS_PATH", "/metrics")),
			HealthPath:  cast.ToString(coalesce("HEALTH_PATH", "/health")),
		},
		Logging: LoggingConfig{
			Level:  cast.ToString(coalesce("LOG_LEVEL", "info")),
			Format: cast.ToString(coalesce("LOG_FORMAT", "json")),
		},
	}
}

func coalesce(key string, value interface{}) interface{} {
	val, exist := os.LookupEnv(key)
	if exist {
		return val
	}
	return value
}

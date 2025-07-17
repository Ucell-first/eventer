package main

import (
	"context"
	"log"
	"log-processor/config"
	"log-processor/consumer"
	"log-processor/db"
	"log-processor/db/clickhouse"
	"log-processor/logs"
	"log-processor/metrics"
	"log-processor/processor"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()
	logger := logs.NewLogger()
	conn, err := clickhouse.ConnectionCH()
	if err != nil {
		logger.Error("Failed to connect to ClickHouse", "error", err)
		log.Fatal(err)
	} else {
		logger.Info("connected to clickhouse")
	}
	defer conn.Close()
	storage := db.NewStorage(conn, logger)

	ctx := context.Background()
	if err := storage.ClickHouseCrud().CreateTable(ctx); err != nil {
		logger.Error("Failed to create table", "error", err)
		log.Fatal(err)
	}

	// Initialize Kafka consumer
	brokers := strings.Split(cfg.Kafka.BrokerURL, ",")
	kafkaConsumer, err := consumer.NewKafkaConsumer(brokers, cfg.Kafka.Topic, cfg.Kafka.ConsumerGroup, logger)
	if err != nil {
		logger.Error("Failed to create Kafka consumer", "error", err)
		log.Fatal(err)
	}
	defer kafkaConsumer.Close()

	// Initialize batch processor
	batchProcessor := processor.NewBatchProcessor(
		storage.ClickHouseCrud(),
		logger,
		cfg.Batch.Size,
		cfg.Batch.FlushInterval,
		cfg.Batch.MaxRetries,
	)

	// Start metrics server
	metrics.StartMetricsServer(cfg.Server.MetricsPort)
	logger.Info("Metrics server started", "port", cfg.Server.MetricsPort)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Start Kafka consumer
	go func() {
		if err := kafkaConsumer.Start(ctx); err != nil {
			logger.Error("Failed to start Kafka consumer", "error", err)
			cancel()
		}
	}()

	// Start batch processor
	go batchProcessor.Start(ctx, kafkaConsumer.GetBatchChannel())

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	logger.Info("Log processor started successfully")

	<-c
	logger.Info("Shutting down gracefully...")

	// Cancel context to stop all goroutines
	cancel()

	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)

	logger.Info("Shutdown complete")
}

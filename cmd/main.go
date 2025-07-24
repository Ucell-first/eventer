package main

import (
	"context"
	"eventer/config"
	"eventer/consumer"
	"eventer/db"
	"eventer/db/clickhouse"
	"eventer/logs"
	"eventer/metrics"
	"eventer/processor"
	"log"
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
		log.Printf("fatal error: %v", err)
		return
	} else {
		logger.Info("connected to clickhouse")
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("error closing connection: %v", err)
		}
	}()

	storage := db.NewStorage(conn, logger)

	ctx := context.Background()
	if err := storage.ClickHouseCrud().CreateTable(ctx); err != nil {
		logger.Error("Failed to create table", "error", err)
		log.Printf("fatal error: %v", err)
		return
	}

	// Initialize Kafka consumer
	brokers := strings.Split(cfg.Kafka.BrokerURL, ",")
	kafkaConsumer, err := consumer.NewKafkaConsumer(brokers, cfg.Kafka.Topic, cfg.Kafka.ConsumerGroup, logger)
	if err != nil {
		logger.Error("Failed to create Kafka consumer", "error", err)
		log.Printf("fatal error: %v", err)
		return
	}
	defer func() {
		if err := kafkaConsumer.Close(); err != nil {
			log.Printf("error closing Kafka consumer: %v", err)
		}
	}()

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

package main

import (
	"context"
	"log-processor/config"
	"log-processor/db"
	"log-processor/db/clickhouse"
	"log-processor/kafka"
	"log-processor/logs"
	"log-processor/model"
	"log-processor/processor"
	"log-processor/server"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func main() {
	// Load config
	cfg := config.Load()

	// Initialize logger
	logger := logs.NewLogger()
	logger.Info("starting log processor")

	// Wait for ClickHouse to be ready
	logger.Info("waiting for ClickHouse connection")
	var conn driver.Conn
	var err error

	for i := 0; i < 30; i++ {
		conn, err = clickhouse.ConnectionCH()
		if err == nil {
			break
		}
		logger.Error("failed to connect to ClickHouse, retrying...", "error", err, "attempt", i+1)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		logger.Error("failed to connect to ClickHouse after retries", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	logger.Info("connected to ClickHouse")

	// Initialize storage
	storage := db.NewStorage(conn, logger)

	// Create table if not exists
	ctx := context.Background()
	if err := storage.ClickHouseCrud().CreateTable(ctx); err != nil {
		logger.Error("failed to create table", "error", err)
		os.Exit(1)
	}

	logger.Info("table created/verified")

	// Initialize Kafka consumer
	logger.Info("initializing kafka consumer")
	consumer, err := kafka.NewConsumer(cfg, logger)
	if err != nil {
		logger.Error("failed to create kafka consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	logger.Info("kafka consumer initialized")

	// Initialize batch processor
	batchProcessor := processor.NewBatchProcessor(storage.ClickHouseCrud(), logger, cfg)

	// Initialize HTTP server
	httpServer := server.NewServer(storage.ClickHouseCrud(), logger, cfg)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel for log entries
	logChan := make(chan model.LogEntry, cfg.Batch.Size)

	// WaitGroup for goroutines
	var wg sync.WaitGroup

	// Start Kafka consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("starting kafka consumer")
		if err := consumer.Consume(ctx, logChan); err != nil && err != context.Canceled {
			logger.Error("kafka consumer error", "error", err)
		}
	}()

	// Start batch processor
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("starting batch processor")
		if err := batchProcessor.Process(ctx, logChan); err != nil && err != context.Canceled {
			logger.Error("batch processor error", "error", err)
		}
	}()

	// Start HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("starting http server")
		if err := httpServer.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("http server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("log processor started successfully")
	<-sigChan
	logger.Info("shutting down...")

	// Cancel context to stop all goroutines
	cancel()

	// Stop HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	httpServer.Stop(shutdownCtx)

	// Wait for all goroutines to finish
	wg.Wait()

	logger.Info("log processor stopped")
}

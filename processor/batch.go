package processor

import (
	"context"
	"log-processor/config"
	"log-processor/db/repo"
	"log-processor/model"
	"log/slog"
	"time"
)

type BatchProcessor struct {
	storage repo.IClickHouseCrudStorage
	logger  *slog.Logger
	config  *config.Config
	batch   []model.LogEntry
}

func NewBatchProcessor(storage repo.IClickHouseCrudStorage, logger *slog.Logger, cfg *config.Config) *BatchProcessor {
	return &BatchProcessor{
		storage: storage,
		logger:  logger,
		config:  cfg,
		batch:   make([]model.LogEntry, 0, cfg.Batch.Size),
	}
}

func (bp *BatchProcessor) Process(ctx context.Context, logChan <-chan model.LogEntry) error {
	ticker := time.NewTicker(bp.config.Batch.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case log := <-logChan:
			bp.batch = append(bp.batch, log)

			if len(bp.batch) >= bp.config.Batch.Size {
				bp.flush(ctx)
			}

		case <-ticker.C:
			if len(bp.batch) > 0 {
				bp.flush(ctx)
			}

		case <-ctx.Done():
			if len(bp.batch) > 0 {
				bp.flush(context.Background()) // Force flush on exit
			}
			return ctx.Err()
		}
	}
}

func (bp *BatchProcessor) flush(ctx context.Context) {
	if len(bp.batch) == 0 {
		return
	}

	retries := 0
	for retries < bp.config.Batch.MaxRetries {
		err := bp.storage.InsertLogsBatch(ctx, bp.batch)
		if err == nil {
			bp.logger.Info("batch inserted successfully", "count", len(bp.batch))
			bp.batch = bp.batch[:0] // Clear batch
			return
		}

		retries++
		bp.logger.Error("failed to insert batch", "error", err, "retry", retries)

		if retries < bp.config.Batch.MaxRetries {
			time.Sleep(time.Second * time.Duration(retries))
		}
	}

	bp.logger.Error("max retries reached, dropping batch", "count", len(bp.batch))
	bp.batch = bp.batch[:0]
}

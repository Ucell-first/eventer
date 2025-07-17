package processor

import (
	"context"
	"log-processor/db/repo"
	"log-processor/model"
	"log/slog"
	"sync"
	"time"
)

type BatchProcessor struct {
	storage       repo.IClickHouseCrudStorage
	logger        *slog.Logger
	batchSize     int
	flushInterval time.Duration
	maxRetries    int
	batch         []model.LogEntry
	mu            sync.RWMutex
}

func NewBatchProcessor(storage repo.IClickHouseCrudStorage, logger *slog.Logger, batchSize int, flushInterval time.Duration, maxRetries int) *BatchProcessor {
	return &BatchProcessor{
		storage:       storage,
		logger:        logger,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		maxRetries:    maxRetries,
		batch:         make([]model.LogEntry, 0, batchSize),
	}
}

func (bp *BatchProcessor) Start(ctx context.Context, logCh <-chan model.LogEntry) {
	ticker := time.NewTicker(bp.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case log := <-logCh:
			bp.addToBatch(log)
			if bp.shouldFlush() {
				bp.flush(ctx)
			}

		case <-ticker.C:
			bp.flush(ctx)

		case <-ctx.Done():
			bp.logger.Info("Shutting down batch processor")
			bp.flush(ctx)
			return
		}
	}
}

func (bp *BatchProcessor) addToBatch(log model.LogEntry) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.batch = append(bp.batch, log)
}

func (bp *BatchProcessor) shouldFlush() bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return len(bp.batch) >= bp.batchSize
}

func (bp *BatchProcessor) flush(ctx context.Context) {
	bp.mu.Lock()
	if len(bp.batch) == 0 {
		bp.mu.Unlock()
		return
	}

	batch := make([]model.LogEntry, len(bp.batch))
	copy(batch, bp.batch)
	bp.batch = bp.batch[:0]
	bp.mu.Unlock()

	bp.insertWithRetry(ctx, batch)
}

func (bp *BatchProcessor) insertWithRetry(ctx context.Context, batch []model.LogEntry) {
	for i := 0; i < bp.maxRetries; i++ {
		if err := bp.storage.InsertLogsBatch(ctx, batch); err != nil {
			bp.logger.Error("Failed to insert batch", "error", err, "attempt", i+1)
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		bp.logger.Info("Batch inserted successfully", "count", len(batch))
		return
	}

	bp.logger.Error("Failed to insert batch after retries", "count", len(batch))
}

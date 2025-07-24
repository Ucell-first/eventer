package processor

import (
	"context"
	"eventer/db/repo"
	"eventer/model"
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

	currentBatch []model.LogEntry
	batchCh      chan []model.LogEntry
	workerPool   int
	wg           sync.WaitGroup

	timer       *time.Timer
	timerActive bool
	mu          sync.Mutex
}

func NewBatchProcessor(
	storage repo.IClickHouseCrudStorage,
	logger *slog.Logger,
	batchSize int,
	flushInterval time.Duration,
	maxRetries int,
) *BatchProcessor {
	return &BatchProcessor{
		storage:       storage,
		logger:        logger,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		maxRetries:    maxRetries,
		currentBatch:  make([]model.LogEntry, 0, batchSize),
		batchCh:       make(chan []model.LogEntry, 10),
		workerPool:    3,
	}
}

func (bp *BatchProcessor) Start(ctx context.Context, logCh <-chan model.LogEntry) {
	for i := 0; i < bp.workerPool; i++ {
		bp.wg.Add(1)
		go bp.insertWorker(ctx)
	}
	for {
		select {
		case log, ok := <-logCh:
			if !ok {
				bp.logger.Info("Log channel closed")
				bp.flushCurrentBatch()
				close(bp.batchCh)
				bp.wg.Wait()
				return
			}

			bp.addToBatch(log)

		case <-ctx.Done():
			bp.logger.Info("Shutting down batch processor")
			bp.flushCurrentBatch()
			close(bp.batchCh)
			bp.wg.Wait()
			return
		}
	}
}

func (bp *BatchProcessor) addToBatch(log model.LogEntry) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.currentBatch = append(bp.currentBatch, log)
	if len(bp.currentBatch) == 1 && !bp.timerActive {
		bp.startTimer()
	}
	if len(bp.currentBatch) >= bp.batchSize {
		bp.flushAndReset()
	}
}

func (bp *BatchProcessor) startTimer() {
	bp.timerActive = true
	bp.timer = time.AfterFunc(bp.flushInterval, func() {
		bp.mu.Lock()
		defer bp.mu.Unlock()

		if len(bp.currentBatch) > 0 {
			bp.flushAndReset()
		}
		bp.timerActive = false
	})
}

func (bp *BatchProcessor) flushAndReset() {
	if len(bp.currentBatch) == 0 {
		return
	}
	batch := make([]model.LogEntry, len(bp.currentBatch))
	copy(batch, bp.currentBatch)

	select {
	case bp.batchCh <- batch:
		bp.logger.Debug("Batch queued for processing", "size", len(batch))
	default:
		bp.logger.Warn("Worker queue full, processing batch synchronously", "size", len(batch))
		go bp.insertWithRetry(context.Background(), batch)
	}
	bp.currentBatch = bp.currentBatch[:0]
	if bp.timerActive && bp.timer != nil {
		bp.timer.Stop()
		bp.timerActive = false
	}
}

func (bp *BatchProcessor) flushCurrentBatch() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.flushAndReset()
}

func (bp *BatchProcessor) insertWorker(ctx context.Context) {
	defer bp.wg.Done()

	for {
		select {
		case batch, ok := <-bp.batchCh:
			if !ok {
				return
			}
			bp.insertWithRetry(ctx, batch)

		case <-ctx.Done():
			return
		}
	}
}

func (bp *BatchProcessor) insertWithRetry(ctx context.Context, batch []model.LogEntry) {
	for attempt := 1; attempt <= bp.maxRetries; attempt++ {
		if err := bp.storage.InsertLogsBatch(ctx, batch); err != nil {
			bp.logger.Error("Failed to insert batch",
				"error", err,
				"attempt", attempt,
				"batch_size", len(batch))

			if attempt < bp.maxRetries {
				backoff := time.Duration(attempt) * time.Second
				time.Sleep(backoff)
				continue
			}
			bp.logger.Error("Failed to insert batch after all retries",
				"batch_size", len(batch),
				"max_retries", bp.maxRetries)
			return
		}
		bp.logger.Info("Batch inserted successfully",
			"batch_size", len(batch),
			"attempt", attempt)
		return
	}
}

package processor

import (
	"context"
	"log-processor/config"
	"log-processor/db"
	"log-processor/model"
	"log/slog"
	"time"
)

type BatchProcessor struct {
	storage db.IStorage      // ClickHouse storage
	config  *config.Config   // Konfiguratsiya
	log     *slog.Logger     // Logger
	batch   []model.LogEntry // Loglar uchun batch
}

// Yangi batch processor yaratish
func NewBatchProcessor(storage db.IStorage, cfg *config.Config, log *slog.Logger) *BatchProcessor {
	return &BatchProcessor{
		storage: storage,
		config:  cfg,
		log:     log,
		batch:   make([]model.LogEntry, 0, cfg.Batch.Size), // Bo'sh batch yaratish
	}
}

// Batch processing'ni boshlash
func (bp *BatchProcessor) Process(ctx context.Context, logChan <-chan model.LogEntry) {
	// Timer yaratish (har X sekundda flush qilish uchun)
	ticker := time.NewTicker(bp.config.Batch.FlushInterval)
	defer ticker.Stop()

	bp.log.Info("Starting batch processor",
		"batch_size", bp.config.Batch.Size,
		"flush_interval", bp.config.Batch.FlushInterval)

	for {
		select {
		case <-ctx.Done():
			// Dastur to'xtatilganda qolgan loglarni flush qilish
			bp.log.Info("Batch processor stopping, flushing remaining logs", "remaining", len(bp.batch))
			bp.flush()
			return

		case logEntry := <-logChan:
			// Kelgan logni batch'ga qo'shish
			bp.batch = append(bp.batch, logEntry)
			bp.log.Debug("Added log to batch", "current_size", len(bp.batch))

			// Batch to'lganmi tekshirish
			if len(bp.batch) >= bp.config.Batch.Size {
				bp.log.Info("Batch size reached, flushing", "size", len(bp.batch))
				bp.flush()
			}

		case <-ticker.C:
			// Vaqt o'tganmi tekshirish
			if len(bp.batch) > 0 {
				bp.log.Info("Flush interval reached, flushing", "size", len(bp.batch))
				bp.flush()
			}
		}
	}
}

// Batch'ni ClickHouse'ga yozish
func (bp *BatchProcessor) flush() {
	if len(bp.batch) == 0 {
		return // Bo'sh batch'ni flush qilmaslik
	}

	// Timeout bilan context yaratish
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Retry logic - bir necha marta urinish
	for attempt := 1; attempt <= bp.config.Batch.MaxRetries; attempt++ {
		err := bp.storage.ClickHouseCrud().InsertLogsBatch(ctx, bp.batch)

		if err == nil {
			// Muvaffaqiyatli yozildi
			bp.log.Info("Successfully flushed batch", "size", len(bp.batch))
			bp.batch = bp.batch[:0] // Batch'ni tozalash
			return
		}

		// Xatolik bo'ldi
		bp.log.Error("Failed to flush batch",
			"attempt", attempt,
			"max_retries", bp.config.Batch.MaxRetries,
			"error", err)

		// Oxirgi urinish bo'lmasa, kutish
		if attempt < bp.config.Batch.MaxRetries {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	// Barcha urinishlar muvaffaqiyatsiz
	bp.log.Error("Failed to flush batch after all retries", "lost_logs", len(bp.batch))
	bp.batch = bp.batch[:0] // Memory leak oldini olish uchun tozalash
}

package repo

import (
	"context"
	"eventer/model"
	"time"
)

type IClickHouseCrudStorage interface {
	CreateTable(ctx context.Context) error
	InsertLog(ctx context.Context, log *model.LogEntry) error
	InsertLogsBatch(ctx context.Context, logs []model.LogEntry) error
	GetLogs(ctx context.Context, limit int, offset int) ([]model.LogEntry, error)
	GetLogsByMethod(ctx context.Context, method string, limit int) ([]model.LogEntry, error)
	GetLogStats(ctx context.Context) (map[string]interface{}, error)
	DeleteOldLogs(ctx context.Context, olderThan time.Duration) (int64, error)
}

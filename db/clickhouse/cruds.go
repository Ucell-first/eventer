package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type LogEntry struct {
	Timestamp time.Time `ch:"timestamp"`
	Method    string    `ch:"method"`
	Path      string    `ch:"path"`
	Status    uint16    `ch:"status"`
	LatencyMs uint32    `ch:"latency_ms"`
	IP        net.IP    `ch:"ip"`
}

type LogRepository struct {
	Conn driver.Conn
	Log  *slog.Logger
}

func NewLogRepository(conn driver.Conn, log *slog.Logger) *LogRepository {
	return &LogRepository{Conn: conn, Log: log}
}

func (r *LogRepository) CreateTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS logs (
			timestamp   DateTime,
			method      LowCardinality(String),
			path        String,
			status      UInt16,
			latency_ms  UInt32,
			ip          IPv4
		) ENGINE = MergeTree()
		ORDER BY timestamp
	`

	if err := r.Conn.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create logs table: %w", err)
	}

	return nil
}

func (r *LogRepository) InsertLog(ctx context.Context, log *LogEntry) error {
	query := `
		INSERT INTO logs (timestamp, method, path, status, latency_ms, ip)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	if err := r.Conn.Exec(ctx, query,
		log.Timestamp,
		log.Method,
		log.Path,
		log.Status,
		log.LatencyMs,
		log.IP,
	); err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	return nil
}

func (r *LogRepository) InsertLogsBatch(ctx context.Context, logs []LogEntry) error {
	if len(logs) == 0 {
		return nil
	}

	batch, err := r.Conn.PrepareBatch(ctx, "INSERT INTO logs")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, log := range logs {
		if err := batch.Append(
			log.Timestamp,
			log.Method,
			log.Path,
			log.Status,
			log.LatencyMs,
			log.IP,
		); err != nil {
			return fmt.Errorf("failed to append log to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	return nil
}

func (r *LogRepository) GetLogs(ctx context.Context, limit int, offset int) ([]LogEntry, error) {
	query := `
		SELECT timestamp, method, path, status, latency_ms, ip
		FROM logs
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.Conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			r.Log.Error(fmt.Sprintf("error while closing rows: %v", err))
		}
	}()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		if err := rows.Scan(
			&log.Timestamp,
			&log.Method,
			&log.Path,
			&log.Status,
			&log.LatencyMs,
			&log.IP,
		); err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return logs, nil
}

func (r *LogRepository) GetLogsByMethod(ctx context.Context, method string, limit int) ([]LogEntry, error) {
	query := `
		SELECT timestamp, method, path, status, latency_ms, ip
		FROM logs
		WHERE method = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := r.Conn.Query(ctx, query, method, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs by method: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			r.Log.Error(fmt.Sprintf("error while closing rows: %v", err))
		}
	}()

	var logs []LogEntry
	for rows.Next() {
		var log LogEntry
		if err := rows.Scan(
			&log.Timestamp,
			&log.Method,
			&log.Path,
			&log.Status,
			&log.LatencyMs,
			&log.IP,
		); err != nil {
			return nil, fmt.Errorf("failed to scan log row: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (r *LogRepository) GetLogStats(ctx context.Context) (map[string]interface{}, error) {
	query := `
		SELECT
			count() as total_requests,
			countIf(status >= 400) as error_count,
			avg(latency_ms) as avg_latency,
			max(latency_ms) as max_latency,
			min(timestamp) as first_log,
			max(timestamp) as last_log
		FROM logs
	`

	row := r.Conn.QueryRow(ctx, query)

	var stats = make(map[string]interface{})
	var totalRequests, errorCount uint64
	var avgLatency, maxLatency float64
	var firstLog, lastLog time.Time

	if err := row.Scan(
		&totalRequests,
		&errorCount,
		&avgLatency,
		&maxLatency,
		&firstLog,
		&lastLog,
	); err != nil {
		return nil, fmt.Errorf("failed to scan stats: %w", err)
	}

	stats["total_requests"] = totalRequests
	stats["error_count"] = errorCount
	stats["avg_latency"] = avgLatency
	stats["max_latency"] = maxLatency
	stats["first_log"] = firstLog
	stats["last_log"] = lastLog

	return stats, nil
}

func (r *LogRepository) DeleteOldLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)

	query := `
		ALTER TABLE logs DELETE WHERE timestamp < ?
	`

	if err := r.Conn.Exec(ctx, query, cutoffTime); err != nil {
		return 0, fmt.Errorf("failed to delete old logs: %w", err)
	}
	return 0, nil
}

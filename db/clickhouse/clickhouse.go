package clickhouse

import (
	"context"
	"fmt"
	"log-processor/config"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func ConnectionCH() (driver.Conn, error) {
	conf := config.Load()

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{conf.ClickHouse.URL},
		Auth: clickhouse.Auth{
			Database: conf.ClickHouse.Database,
			Username: conf.ClickHouse.Username,
			Password: conf.ClickHouse.Password,
		},
		DialTimeout: 30 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	return conn, nil
}

package db

import (
	"log-processor/db/clickhouse"
	"log-processor/db/repo"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type IStorage interface {
	ClickHouseCrud() repo.IClickHouseCrudStorage
}

type databaseStorage struct {
	clh driver.Conn
	log *slog.Logger
}

func NewStorage(clh driver.Conn, log *slog.Logger) IStorage {
	return &databaseStorage{
		clh: clh,
		log: log,
	}
}

func (p *databaseStorage) ClosePDB() error {
	err := p.clh.Close()
	if err != nil {
		return err
	}
	return nil
}

func (p *databaseStorage) ClickHouseCrud() repo.IClickHouseCrudStorage {
	return clickhouse.NewLogRepository(p.clh, p.log)
}

package clickhouse

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/samber/do/v2"

	"metis/internal/config"
)

// ClickHouseClient wraps a ClickHouse database/sql connection.
type ClickHouseClient struct {
	DB *sql.DB
}

// NewClickHouseClient creates a ClickHouse client from config.
// Returns nil (not error) if ClickHouse is not configured — callers should check for nil.
func NewClickHouseClient(i do.Injector) (*ClickHouseClient, error) {
	cfg := do.MustInvoke[*config.MetisConfig](i)
	if cfg.ClickHouse == nil || cfg.ClickHouse.DSN == "" {
		slog.Warn("ClickHouse not configured — APM features will be unavailable")
		return nil, nil
	}

	db, err := sql.Open("clickhouse", cfg.ClickHouse.DSN)
	if err != nil {
		return nil, fmt.Errorf("open ClickHouse at %s: %w", cfg.ClickHouse.DSN, err)
	}

	// Verify connectivity
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping ClickHouse at %s: %w", cfg.ClickHouse.DSN, err)
	}

	slog.Info("ClickHouse connected", "dsn", cfg.ClickHouse.DSN)
	return &ClickHouseClient{DB: db}, nil
}

// Shutdown closes the ClickHouse connection pool.
func (c *ClickHouseClient) Shutdown() error {
	if c != nil && c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

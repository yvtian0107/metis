package runtime

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FalkorDB/falkordb-go/v2"
	"github.com/samber/do/v2"

	"metis/internal/config"
)

// FalkorDBClient wraps the FalkorDB connection and provides graph selection.
type FalkorDBClient struct {
	db *falkordb.FalkorDB
}

// NewFalkorDBClient creates a FalkorDB client from the config.
// Returns nil (not error) if FalkorDB is not configured — callers should check for nil.
func NewFalkorDBClient(i do.Injector) (*FalkorDBClient, error) {
	cfg := do.MustInvoke[*config.MetisConfig](i)
	if cfg.FalkorDB == nil || cfg.FalkorDB.Addr == "" {
		slog.Warn("FalkorDB not configured — knowledge graph features will be unavailable")
		return nil, nil
	}

	db, err := falkordb.FalkorDBNew(&falkordb.ConnectionOption{
		Addr:     cfg.FalkorDB.Addr,
		Password: cfg.FalkorDB.Password,
		DB:       cfg.FalkorDB.Database,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to FalkorDB at %s: %w", cfg.FalkorDB.Addr, err)
	}

	// Verify connectivity
	if _, err := db.Conn.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("ping FalkorDB at %s: %w", cfg.FalkorDB.Addr, err)
	}

	slog.Info("FalkorDB connected", "addr", cfg.FalkorDB.Addr)
	return &FalkorDBClient{db: db}, nil
}

// GraphFor returns the FalkorDB graph for a given knowledge graph asset ID.
func (c *FalkorDBClient) GraphFor(kgID uint) *falkordb.Graph {
	return c.db.SelectGraph(fmt.Sprintf("kg_%d", kgID))
}

// DeleteGraph deletes the graph for a knowledge graph asset.
func (c *FalkorDBClient) DeleteGraph(kgID uint) error {
	graph := c.GraphFor(kgID)
	err := graph.Delete()
	// Ignore error if graph doesn't exist (never compiled)
	if err != nil && err.Error() == "ERR Invalid graph operation on empty key" {
		return nil
	}
	return err
}

// Available returns true if FalkorDB is configured and connected.
func (c *FalkorDBClient) Available() bool {
	return c != nil && c.db != nil
}

// Shutdown closes the FalkorDB connection.
func (c *FalkorDBClient) Shutdown() error {
	if c != nil && c.db != nil && c.db.Conn != nil {
		return c.db.Conn.Close()
	}
	return nil
}

package database

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"metis/internal/config"
	"metis/internal/model"
)

// DB wraps gorm.DB to implement do.Shutdowner.
type DB struct {
	*gorm.DB
}

// New creates a DB from MetisConfig in the IOC container.
// If no config is provided, falls back to default SQLite.
func New(i do.Injector) (*DB, error) {
	cfg, err := do.InvokeAs[*config.MetisConfig](i)
	if err != nil {
		// No config in container — use default SQLite (install mode)
		cfg = config.DefaultSQLiteConfig()
	}
	return Open(cfg.DBDriver, cfg.DBDSN)
}

// Open connects to a database with the given driver and DSN.
// Used by both normal startup and install wizard (for connection testing).
func Open(driver, dsn string) (*DB, error) {
	var dialector gorm.Dialector
	isSQLite := false
	switch driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "sqlite", "":
		isSQLite = true
		if dsn == "" {
			dsn = "metis.db?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
		}
		dsn = ensureSQLiteBusyTimeout(dsn)
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if isSQLite {
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to configure sqlite connection pool: %w", err)
		}
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}

	// OpenTelemetry: auto-trace all DB queries (noop when OTel is disabled)
	if err := db.Use(otelgorm.NewPlugin(otelgorm.WithoutQueryVariables())); err != nil {
		return nil, fmt.Errorf("otelgorm plugin failed: %w", err)
	}

	slog.Info("database connected", "driver", driver, "dsn", dsn)
	return &DB{DB: db}, nil
}

func ensureSQLiteBusyTimeout(dsn string) string {
	if strings.Contains(strings.ToLower(dsn), "busy_timeout") {
		return dsn
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "_pragma=busy_timeout(5000)"
}

// AutoMigrateKernel runs AutoMigrate for all kernel models.
func AutoMigrateKernel(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.SystemConfig{},
		&model.Role{},
		&model.RoleDeptScope{},
		&model.Menu{},
		&model.User{},
		&model.RefreshToken{},
		&model.AuthProvider{},
		&model.UserConnection{},
		&model.TwoFactorSecret{},
		&model.TaskState{},
		&model.TaskExecution{},
		&model.Notification{},
		&model.NotificationRead{},
		&model.MessageChannel{},
		&model.AuditLog{},
		&model.IdentitySource{},
	)
}

// Shutdown implements do.ShutdownerWithError for graceful cleanup.
func (d *DB) Shutdown() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	slog.Info("closing database connection")
	return sqlDB.Close()
}

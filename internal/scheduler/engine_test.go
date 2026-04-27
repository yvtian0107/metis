package scheduler

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/database"
)

func TestNewSQLiteEngineUsesSingleAsyncWorker(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	injector := do.New()
	do.ProvideValue(injector, &database.DB{DB: db})

	eng, err := New(injector)
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	if eng.maxWorkers != 1 {
		t.Fatalf("expected SQLite scheduler max workers 1, got %d", eng.maxWorkers)
	}
}

func TestMaxWorkersForDialectorKeepsDefaultForNonSQLite(t *testing.T) {
	if got := maxWorkersForDialector("postgres"); got != defaultMaxWorkers {
		t.Fatalf("expected postgres max workers %d, got %d", defaultMaxWorkers, got)
	}
}

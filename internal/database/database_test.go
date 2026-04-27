package database

import "testing"

func TestOpenSQLiteAddsBusyTimeoutWhenMissing(t *testing.T) {
	db, err := Open("sqlite", "file:busy_timeout_missing?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() {
		if err := db.Shutdown(); err != nil {
			t.Fatalf("shutdown db: %v", err)
		}
	}()

	var timeout int
	if err := db.Raw("PRAGMA busy_timeout").Scan(&timeout).Error; err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if timeout <= 0 {
		t.Fatalf("expected positive busy_timeout, got %d", timeout)
	}
}

func TestOpenSQLiteLimitsConnectionPoolToSingleWriter(t *testing.T) {
	db, err := Open("sqlite", "file:single_writer?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() {
		if err := db.Shutdown(); err != nil {
			t.Fatalf("shutdown db: %v", err)
		}
	}()

	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("expected max open connections to be 1, got %d", got)
	}
}

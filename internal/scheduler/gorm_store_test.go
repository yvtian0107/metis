package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSQLiteBusyRetryRetriesTransientLockErrors(t *testing.T) {
	attempts := 0
	err := withSQLiteBusyRetry(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return errors.New("database is locked (5) (SQLITE_BUSY)")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestSQLiteBusyRetryDoesNotRetryPermanentErrors(t *testing.T) {
	attempts := 0
	want := errors.New("validation failed")
	err := withSQLiteBusyRetry(context.Background(), func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected original error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestSQLiteBusyRetryStopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := withSQLiteBusyRetry(ctx, func() error {
		attempts++
		cancel()
		return errors.New("database is locked (5) (SQLITE_BUSY)")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt before cancellation, got %d", attempts)
	}
}

func TestSQLiteBusyRetryHonorsContextDeadlineDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	err := withSQLiteBusyRetry(ctx, func() error {
		return errors.New("database is locked (5) (SQLITE_BUSY)")
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

package scheduler

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"metis/internal/model"
)

// GormStore implements Store using GORM (SQLite + PostgreSQL compatible).
type GormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

func (s *GormStore) SaveTaskState(ctx context.Context, state *model.TaskState) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return s.db.WithContext(ctx).Save(state).Error
	})
}

func (s *GormStore) GetTaskState(ctx context.Context, name string) (*model.TaskState, error) {
	var state model.TaskState
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *GormStore) ListTaskStates(ctx context.Context, taskType string) ([]*model.TaskState, error) {
	var states []*model.TaskState
	q := s.db.WithContext(ctx)
	if taskType != "" {
		q = q.Where("type = ?", taskType)
	}
	if err := q.Order("name").Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}

func (s *GormStore) Enqueue(ctx context.Context, exec *model.TaskExecution) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return s.db.WithContext(ctx).Create(exec).Error
	})
}

func (s *GormStore) Dequeue(ctx context.Context, limit int) ([]*model.TaskExecution, error) {
	var execs []*model.TaskExecution
	err := withSQLiteBusyRetry(ctx, func() error {
		execs = nil
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("status = ?", ExecPending).
				Order("created_at").
				Limit(limit).
				Find(&execs).Error; err != nil {
				return err
			}
			for _, e := range execs {
				now := time.Now()
				e.Status = ExecRunning
				e.StartedAt = &now
				if err := tx.Save(e).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
	return execs, err
}

func (s *GormStore) UpdateExecution(ctx context.Context, exec *model.TaskExecution) error {
	return withSQLiteBusyRetry(ctx, func() error {
		return s.db.WithContext(ctx).Save(exec).Error
	})
}

func (s *GormStore) ListExecutions(ctx context.Context, filter ExecutionFilter) ([]*model.TaskExecution, int64, error) {
	var execs []*model.TaskExecution
	var total int64

	q := s.db.WithContext(ctx).Model(&model.TaskExecution{})
	if filter.TaskName != "" {
		q = q.Where("task_name = ?", filter.TaskName)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	if err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&execs).Error; err != nil {
		return nil, 0, err
	}

	return execs, total, nil
}

func (s *GormStore) GetExecution(ctx context.Context, id uint) (*model.TaskExecution, error) {
	var exec model.TaskExecution
	if err := s.db.WithContext(ctx).First(&exec, id).Error; err != nil {
		return nil, err
	}
	return &exec, nil
}

func (s *GormStore) GetLastExecution(ctx context.Context, taskName string) (*model.TaskExecution, error) {
	var exec model.TaskExecution
	err := s.db.WithContext(ctx).
		Where("task_name = ?", taskName).
		Order("created_at DESC").
		First(&exec).Error
	if err != nil {
		return nil, err
	}
	return &exec, nil
}

func (s *GormStore) Stats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{}

	// Total registered tasks
	s.db.WithContext(ctx).Model(&model.TaskState{}).Count(new(int64))
	var totalTasks int64
	s.db.WithContext(ctx).Model(&model.TaskState{}).Count(&totalTasks)
	stats.TotalTasks = int(totalTasks)

	// Pending
	var pending int64
	s.db.WithContext(ctx).Model(&model.TaskExecution{}).Where("status = ?", ExecPending).Count(&pending)
	stats.Pending = int(pending)

	// Running
	var running int64
	s.db.WithContext(ctx).Model(&model.TaskExecution{}).Where("status = ?", ExecRunning).Count(&running)
	stats.Running = int(running)

	// Completed today
	today := time.Now().Truncate(24 * time.Hour)
	var completedToday int64
	s.db.WithContext(ctx).Model(&model.TaskExecution{}).
		Where("status = ? AND finished_at >= ?", ExecCompleted, today).
		Count(&completedToday)
	stats.CompletedToday = int(completedToday)

	// Failed today
	var failedToday int64
	s.db.WithContext(ctx).Model(&model.TaskExecution{}).
		Where("status IN (?, ?) AND created_at >= ?", ExecFailed, ExecTimeout, today).
		Count(&failedToday)
	stats.FailedToday = int(failedToday)

	return stats, nil
}

func (s *GormStore) Close() error {
	return nil
}

// DeleteOlderThan removes execution records older than the given time.
func (s *GormStore) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	var rows int64
	err := withSQLiteBusyRetry(ctx, func() error {
		result := s.db.WithContext(ctx).
			Where("created_at < ?", before).
			Delete(&model.TaskExecution{})
		rows = result.RowsAffected
		return result.Error
	})
	return rows, err
}

func withSQLiteBusyRetry(ctx context.Context, fn func() error) error {
	const maxAttempts = 5
	delay := 10 * time.Millisecond
	var err error
	if ctx == nil {
		ctx = context.Background()
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		err = fn()
		if err == nil || !isSQLiteBusyError(err) || attempt == maxAttempts {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		delay *= 2
	}
	return err
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "sqlite_busy") ||
		strings.Contains(msg, "sqlite_locked")
}

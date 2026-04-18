package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"metis/internal/model"
)

// ErrNotReady is returned by task handlers when the task is not yet ready to execute
// (e.g., a timer that hasn't reached its execute_after time). The scheduler will
// re-enqueue the task for the next poll cycle without marking it as completed or failed.
var ErrNotReady = errors.New("task not ready")

// Task types.
const (
	TypeScheduled = "scheduled"
	TypeAsync     = "async"
	TypeStartup   = "startup" // runs once at engine start
)

// Task runtime statuses.
const (
	StatusActive = "active"
	StatusPaused = "paused"
)

// Execution statuses.
const (
	ExecPending   = "pending"
	ExecRunning   = "running"
	ExecCompleted = "completed"
	ExecFailed    = "failed"
	ExecTimeout   = "timeout"
	ExecStale     = "stale"
)

// Execution trigger sources.
const (
	TriggerCron   = "cron"
	TriggerManual = "manual"
	TriggerAPI    = "api"
)

// HandlerFunc is the function signature for task handlers.
type HandlerFunc func(ctx context.Context, payload json.RawMessage) error

// TaskDef is a task definition registered in code.
type TaskDef struct {
	Name        string
	Type        string // TypeScheduled or TypeAsync
	Description string
	CronExpr    string        // only for scheduled tasks
	Timeout     time.Duration // default 30s
	MaxRetries  int           // default 3
	Handler     HandlerFunc
}

// QueueStats holds aggregate statistics.
type QueueStats struct {
	TotalTasks     int `json:"totalTasks"`
	Pending        int `json:"pending"`
	Running        int `json:"running"`
	CompletedToday int `json:"completedToday"`
	FailedToday    int `json:"failedToday"`
}

// ExecutionFilter for querying execution history.
type ExecutionFilter struct {
	TaskName string
	Status   string
	Page     int
	PageSize int
}

// TaskInfo combines a TaskState with its last execution for API responses.
type TaskInfo struct {
	model.TaskState
	LastExecution *LastExecution `json:"lastExecution,omitempty"`
}

// LastExecution is a summary of the most recent run.
type LastExecution struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	Duration  int64     `json:"duration"` // milliseconds
}

// Store is the pluggable persistence interface.
type Store interface {
	// Task state management
	SaveTaskState(ctx context.Context, state *model.TaskState) error
	GetTaskState(ctx context.Context, name string) (*model.TaskState, error)
	ListTaskStates(ctx context.Context, taskType string) ([]*model.TaskState, error)

	// Async queue
	Enqueue(ctx context.Context, exec *model.TaskExecution) error
	Dequeue(ctx context.Context, limit int) ([]*model.TaskExecution, error)
	UpdateExecution(ctx context.Context, exec *model.TaskExecution) error

	// Execution history
	ListExecutions(ctx context.Context, filter ExecutionFilter) ([]*model.TaskExecution, int64, error)
	GetExecution(ctx context.Context, id uint) (*model.TaskExecution, error)

	// Get last execution for a task
	GetLastExecution(ctx context.Context, taskName string) (*model.TaskExecution, error)

	// Statistics
	Stats(ctx context.Context) (*QueueStats, error)

	// Lifecycle
	Close() error
}

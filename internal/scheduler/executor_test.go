package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"metis/internal/model"
)

// mockStore is a minimal Store implementation for executor tests.
type mockStore struct {
	updated  *model.TaskExecution
	enqueued []*model.TaskExecution
}

func (m *mockStore) Enqueue(_ context.Context, exec *model.TaskExecution) error {
	m.enqueued = append(m.enqueued, exec)
	return nil
}

func (m *mockStore) UpdateExecution(_ context.Context, exec *model.TaskExecution) error {
	m.updated = exec
	return nil
}

// Unused interface methods.
func (m *mockStore) Dequeue(context.Context, int) ([]*model.TaskExecution, error) { return nil, nil }
func (m *mockStore) SaveTaskState(context.Context, *model.TaskState) error       { return nil }
func (m *mockStore) GetTaskState(context.Context, string) (*model.TaskState, error) {
	return nil, nil
}
func (m *mockStore) ListTaskStates(context.Context, string) ([]*model.TaskState, error) {
	return nil, nil
}
func (m *mockStore) ListExecutions(context.Context, ExecutionFilter) ([]*model.TaskExecution, int64, error) {
	return nil, 0, nil
}
func (m *mockStore) GetExecution(context.Context, uint) (*model.TaskExecution, error) {
	return nil, nil
}
func (m *mockStore) GetLastExecution(context.Context, string) (*model.TaskExecution, error) {
	return nil, nil
}
func (m *mockStore) Stats(context.Context) (*QueueStats, error) { return nil, nil }
func (m *mockStore) Close() error                               { return nil }

// helper to build an executor with one registered task.
func setupExecutor(store *mockStore, taskName string, handler HandlerFunc) *executor {
	registry := map[string]*TaskDef{
		taskName: {
			Name:       taskName,
			Type:       TypeAsync,
			Timeout:    5 * time.Second,
			MaxRetries: 3,
			Handler:    handler,
		},
	}
	return newExecutor(1, store, registry)
}

func TestRun_ErrNotReady_ReenqueuesPending(t *testing.T) {
	store := &mockStore{}
	exec := &model.TaskExecution{
		ID:       1,
		TaskName: "test-task",
		Trigger:  TriggerCron,
		Status:   ExecRunning,
		Payload:  `{}`,
	}

	e := setupExecutor(store, "test-task", func(_ context.Context, _ json.RawMessage) error {
		return ErrNotReady
	})

	e.run(exec)

	if store.updated == nil {
		t.Fatal("expected UpdateExecution to be called")
	}

	if store.updated.Status != ExecPending {
		t.Errorf("status: got %q, want %q", store.updated.Status, ExecPending)
	}

	if store.updated.FinishedAt != nil {
		t.Errorf("FinishedAt: got %v, want nil", store.updated.FinishedAt)
	}

	if len(store.enqueued) != 0 {
		t.Errorf("retry enqueued: got %d, want 0 (no retry for ErrNotReady)", len(store.enqueued))
	}

	if store.updated.Error != "" {
		t.Errorf("Error field: got %q, want empty", store.updated.Error)
	}
}

func TestRun_Success_Completed(t *testing.T) {
	store := &mockStore{}
	exec := &model.TaskExecution{
		ID:       2,
		TaskName: "test-task",
		Trigger:  TriggerManual,
		Status:   ExecRunning,
		Payload:  `{"key":"value"}`,
	}

	e := setupExecutor(store, "test-task", func(_ context.Context, _ json.RawMessage) error {
		return nil
	})

	e.run(exec)

	if store.updated == nil {
		t.Fatal("expected UpdateExecution to be called")
	}

	if store.updated.Status != ExecCompleted {
		t.Errorf("status: got %q, want %q", store.updated.Status, ExecCompleted)
	}

	if store.updated.FinishedAt == nil {
		t.Error("FinishedAt: got nil, want non-nil")
	}

	if len(store.enqueued) != 0 {
		t.Errorf("retry enqueued: got %d, want 0", len(store.enqueued))
	}
}

func TestRun_GenericError_FailedWithRetry(t *testing.T) {
	store := &mockStore{}
	exec := &model.TaskExecution{
		ID:         3,
		TaskName:   "test-task",
		Trigger:    TriggerAPI,
		Status:     ExecRunning,
		Payload:    `{}`,
		RetryCount: 0,
	}

	handlerErr := errors.New("something broke")
	e := setupExecutor(store, "test-task", func(_ context.Context, _ json.RawMessage) error {
		return handlerErr
	})

	e.run(exec)

	if store.updated == nil {
		t.Fatal("expected UpdateExecution to be called")
	}

	if store.updated.Status != ExecFailed {
		t.Errorf("status: got %q, want %q", store.updated.Status, ExecFailed)
	}

	if store.updated.FinishedAt == nil {
		t.Error("FinishedAt: got nil, want non-nil")
	}

	if store.updated.Error != handlerErr.Error() {
		t.Errorf("Error field: got %q, want %q", store.updated.Error, handlerErr.Error())
	}

	if len(store.enqueued) != 1 {
		t.Fatalf("retry enqueued: got %d, want 1", len(store.enqueued))
	}

	retry := store.enqueued[0]
	if retry.Status != ExecPending {
		t.Errorf("retry status: got %q, want %q", retry.Status, ExecPending)
	}
	if retry.RetryCount != 1 {
		t.Errorf("retry RetryCount: got %d, want 1", retry.RetryCount)
	}
	if retry.TaskName != "test-task" {
		t.Errorf("retry TaskName: got %q, want %q", retry.TaskName, "test-task")
	}
}

func TestRun_GenericError_NoRetryWhenMaxRetriesExhausted(t *testing.T) {
	store := &mockStore{}
	exec := &model.TaskExecution{
		ID:         4,
		TaskName:   "test-task",
		Trigger:    TriggerAPI,
		Status:     ExecRunning,
		Payload:    `{}`,
		RetryCount: 3, // already at max
	}

	e := setupExecutor(store, "test-task", func(_ context.Context, _ json.RawMessage) error {
		return errors.New("still broken")
	})

	e.run(exec)

	if store.updated.Status != ExecFailed {
		t.Errorf("status: got %q, want %q", store.updated.Status, ExecFailed)
	}

	if len(store.enqueued) != 0 {
		t.Errorf("retry enqueued: got %d, want 0 (retries exhausted)", len(store.enqueued))
	}
}

func TestRun_WrappedErrNotReady_StillReenqueues(t *testing.T) {
	store := &mockStore{}
	exec := &model.TaskExecution{
		ID:       5,
		TaskName: "test-task",
		Trigger:  TriggerCron,
		Status:   ExecRunning,
		Payload:  `{}`,
	}

	e := setupExecutor(store, "test-task", func(_ context.Context, _ json.RawMessage) error {
		return fmt.Errorf("waiting on dependency: %w", ErrNotReady)
	})

	e.run(exec)

	if store.updated.Status != ExecPending {
		t.Errorf("status: got %q, want %q (wrapped ErrNotReady should still re-enqueue)", store.updated.Status, ExecPending)
	}

	if store.updated.FinishedAt != nil {
		t.Errorf("FinishedAt: got %v, want nil", store.updated.FinishedAt)
	}

	if len(store.enqueued) != 0 {
		t.Errorf("retry enqueued: got %d, want 0", len(store.enqueued))
	}
}

package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"metis/internal/model"
)

// executor manages a bounded goroutine pool for running task handlers.
type executor struct {
	maxWorkers int
	sem        chan struct{}
	store      Store
	registry   map[string]*TaskDef
	wg         sync.WaitGroup
}

func newExecutor(maxWorkers int, store Store, registry map[string]*TaskDef) *executor {
	return &executor{
		maxWorkers: maxWorkers,
		sem:        make(chan struct{}, maxWorkers),
		store:      store,
		registry:   registry,
	}
}

// Submit submits a task execution to the pool. Blocks if all workers are busy.
func (e *executor) Submit(exec *model.TaskExecution) {
	e.sem <- struct{}{} // acquire slot
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() { <-e.sem }() // release slot
		e.run(exec)
	}()
}

// Wait waits for all running tasks to finish, with a timeout.
func (e *executor) Wait(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("executor: timeout waiting for tasks to finish")
	}
}

func (e *executor) run(exec *model.TaskExecution) {
	taskDef, ok := e.registry[exec.TaskName]
	if !ok {
		e.fail(exec, fmt.Sprintf("task %q not found in registry", exec.TaskName))
		return
	}

	timeout := taskDef.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	slog.Debug("scheduler: executing task", "task", exec.TaskName, "trigger", exec.Trigger, "execId", exec.ID)

	errCh := make(chan error, 1)
	go func() {
		errCh <- taskDef.Handler(ctx, json.RawMessage(exec.Payload))
	}()

	var err error
	timedOut := false
	select {
	case err = <-errCh:
	case <-ctx.Done():
		timedOut = true
		err = ctx.Err()
	}

	now := time.Now()
	exec.FinishedAt = &now

	if timedOut {
		exec.Status = ExecTimeout
		exec.Error = "execution timed out"
		slog.Error("scheduler: task timed out", "task", exec.TaskName, "execId", exec.ID, "timeout", timeout)
	} else if err != nil {
		if errors.Is(err, ErrNotReady) {
			// Task is not ready yet — re-enqueue as pending for the next poll cycle.
			// Do not count as a failure or increment retry count.
			exec.Status = ExecPending
			exec.FinishedAt = nil
			slog.Debug("scheduler: task not ready, re-enqueuing", "task", exec.TaskName, "execId", exec.ID)
		} else if ctx.Err() == context.DeadlineExceeded {
			exec.Status = ExecTimeout
			exec.Error = "execution timed out"
		} else {
			// Check retry
			maxRetries := taskDef.MaxRetries
			if maxRetries <= 0 {
				maxRetries = 3
			}
			if exec.RetryCount < maxRetries {
				// Re-enqueue for retry
				retry := &model.TaskExecution{
					TaskName:   exec.TaskName,
					Trigger:    exec.Trigger,
					Status:     ExecPending,
					Payload:    exec.Payload,
					RetryCount: exec.RetryCount + 1,
					CreatedAt:  time.Now(),
				}
				if enqErr := e.store.Enqueue(context.Background(), retry); enqErr != nil {
					slog.Error("scheduler: failed to enqueue retry", "task", exec.TaskName, "error", enqErr)
				} else {
					slog.Debug("scheduler: retrying task", "task", exec.TaskName, "retry", retry.RetryCount)
				}
			}
			exec.Status = ExecFailed
			exec.Error = err.Error()
		}
		if !errors.Is(err, ErrNotReady) {
			slog.Error("scheduler: task failed", "task", exec.TaskName, "status", exec.Status, "error", exec.Error)
		}
	} else {
		exec.Status = ExecCompleted
		slog.Debug("scheduler: task completed", "task", exec.TaskName, "execId", exec.ID)
	}

	if updErr := e.store.UpdateExecution(context.Background(), exec); updErr != nil {
		slog.Error("scheduler: failed to update execution", "task", exec.TaskName, "error", updErr)
	}
}

func (e *executor) fail(exec *model.TaskExecution, errMsg string) {
	now := time.Now()
	exec.Status = ExecFailed
	exec.Error = errMsg
	exec.FinishedAt = &now
	if updErr := e.store.UpdateExecution(context.Background(), exec); updErr != nil {
		slog.Error("scheduler: failed to update execution", "error", updErr)
	}
}

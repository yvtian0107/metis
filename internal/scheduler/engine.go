package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/samber/do/v2"

	"metis/internal/database"
	"metis/internal/model"
)

const (
	defaultMaxWorkers   = 5
	defaultPollInterval = 3 * time.Second
	shutdownTimeout     = 30 * time.Second
)

// Engine is the core task scheduler.
type Engine struct {
	store      Store
	registry   map[string]*TaskDef
	cron       *cron.Cron
	cronIDs    map[string]cron.EntryID // task name -> cron entry ID
	executor   *executor
	maxWorkers int
	notify     chan struct{}
	stopCh     chan struct{}
	stopped    bool
	mu         sync.RWMutex
}

// New creates a new Engine from the IOC container.
func New(i do.Injector) (*Engine, error) {
	db := do.MustInvoke[*database.DB](i)
	store := NewGormStore(db.DB)
	maxWorkers := maxWorkersForDialector(db.DB.Dialector.Name())

	return &Engine{
		store:      store,
		registry:   make(map[string]*TaskDef),
		cron:       cron.New(),
		cronIDs:    make(map[string]cron.EntryID),
		maxWorkers: maxWorkers,
		notify:     make(chan struct{}, 1),
		stopCh:     make(chan struct{}),
	}, nil
}

func maxWorkersForDialector(dialector string) int {
	if dialector == "sqlite" {
		return 1
	}
	return defaultMaxWorkers
}

// Register adds a task definition to the registry. Must be called before Start().
func (e *Engine) Register(def *TaskDef) {
	if def.Name == "" {
		panic("scheduler: task name is required")
	}
	if def.Handler == nil {
		panic(fmt.Sprintf("scheduler: handler is required for task %q", def.Name))
	}
	if _, exists := e.registry[def.Name]; exists {
		panic(fmt.Sprintf("scheduler: duplicate task name %q", def.Name))
	}
	if def.Timeout <= 0 {
		def.Timeout = 30 * time.Second
	}
	if def.MaxRetries <= 0 {
		def.MaxRetries = 3
	}
	e.registry[def.Name] = def
}

// Enqueue adds an async task execution to the queue.
func (e *Engine) Enqueue(name string, payload any) error {
	e.mu.RLock()
	if e.stopped {
		e.mu.RUnlock()
		return fmt.Errorf("scheduler: engine is stopped")
	}
	e.mu.RUnlock()

	def, ok := e.registry[name]
	if !ok {
		return fmt.Errorf("scheduler: task %q not registered", name)
	}
	if def.Type != TypeAsync {
		return fmt.Errorf("scheduler: task %q is not an async task", name)
	}

	var payloadStr string
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("scheduler: failed to marshal payload: %w", err)
		}
		payloadStr = string(data)
	}

	exec := &model.TaskExecution{
		TaskName:  name,
		Trigger:   TriggerAPI,
		Status:    ExecPending,
		Payload:   payloadStr,
		CreatedAt: time.Now(),
	}

	if err := e.store.Enqueue(context.Background(), exec); err != nil {
		return err
	}

	// Non-blocking notify to wake poller
	select {
	case e.notify <- struct{}{}:
	default:
	}

	return nil
}

// Start syncs state, starts cron, and begins the queue poller.
func (e *Engine) Start() error {
	ctx := context.Background()

	// Sync task definitions to DB
	for _, def := range e.registry {
		state := &model.TaskState{
			Name:        def.Name,
			Type:        def.Type,
			Description: def.Description,
			CronExpr:    def.CronExpr,
			TimeoutMs:   int(def.Timeout.Milliseconds()),
			MaxRetries:  def.MaxRetries,
		}

		existing, err := e.store.GetTaskState(ctx, def.Name)
		if err != nil {
			// New task — default to active
			state.Status = StatusActive
			if err := e.store.SaveTaskState(ctx, state); err != nil {
				return fmt.Errorf("scheduler: failed to save task state %q: %w", def.Name, err)
			}
			slog.Info("scheduler: registered task", "name", def.Name, "type", def.Type)
		} else {
			// Update definition fields but preserve runtime status
			state.Status = existing.Status
			if err := e.store.SaveTaskState(ctx, state); err != nil {
				return fmt.Errorf("scheduler: failed to update task state %q: %w", def.Name, err)
			}
		}
	}

	// Initialize executor
	e.executor = newExecutor(e.maxWorkers, e.store, e.registry)

	// Schedule active cron tasks
	for _, def := range e.registry {
		if def.Type != TypeScheduled || def.CronExpr == "" {
			continue
		}
		state, _ := e.store.GetTaskState(ctx, def.Name)
		if state != nil && state.Status == StatusPaused {
			slog.Info("scheduler: skipping paused task", "name", def.Name)
			continue
		}
		if err := e.addCronEntry(def); err != nil {
			return err
		}
	}

	// Mark stale executions
	e.recoverStale(ctx)

	// Run startup tasks (fire-and-forget in background)
	for _, def := range e.registry {
		if def.Type != TypeStartup {
			continue
		}
		go func(d *TaskDef) {
			slog.Info("scheduler: running startup task", "name", d.Name)
			if err := d.Handler(context.Background(), nil); err != nil {
				slog.Error("scheduler: startup task failed", "name", d.Name, "error", err)
			} else {
				slog.Info("scheduler: startup task completed", "name", d.Name)
			}
		}(def)
	}

	// Start cron
	e.cron.Start()

	// Start poller
	go e.poller()

	slog.Info("scheduler: engine started", "tasks", len(e.registry))
	return nil
}

// Stop gracefully shuts down the engine.
func (e *Engine) Stop() {
	e.mu.Lock()
	e.stopped = true
	e.mu.Unlock()

	close(e.stopCh)
	e.cron.Stop()
	e.executor.Wait(shutdownTimeout)
	e.store.Close()

	slog.Info("scheduler: engine stopped")
}

// Shutdown implements do.Shutdowner for IOC container integration.
func (e *Engine) Shutdown() error {
	e.Stop()
	return nil
}

// PauseTask pauses a scheduled task's cron entry.
func (e *Engine) PauseTask(name string) error {
	def, ok := e.registry[name]
	if !ok {
		return fmt.Errorf("task %q not found", name)
	}
	if def.Type != TypeScheduled {
		return fmt.Errorf("only scheduled tasks can be paused")
	}

	ctx := context.Background()
	state, err := e.store.GetTaskState(ctx, name)
	if err != nil {
		return err
	}
	if state.Status == StatusPaused {
		return fmt.Errorf("task %q is already paused", name)
	}

	// Remove cron entry
	if id, ok := e.cronIDs[name]; ok {
		e.cron.Remove(id)
		delete(e.cronIDs, name)
	}

	state.Status = StatusPaused
	return e.store.SaveTaskState(ctx, state)
}

// ResumeTask resumes a paused scheduled task.
func (e *Engine) ResumeTask(name string) error {
	def, ok := e.registry[name]
	if !ok {
		return fmt.Errorf("task %q not found", name)
	}
	if def.Type != TypeScheduled {
		return fmt.Errorf("only scheduled tasks can be resumed")
	}

	ctx := context.Background()
	state, err := e.store.GetTaskState(ctx, name)
	if err != nil {
		return err
	}
	if state.Status == StatusActive {
		return fmt.Errorf("task %q is already active", name)
	}

	// Re-add cron entry
	if err := e.addCronEntry(def); err != nil {
		return err
	}

	state.Status = StatusActive
	return e.store.SaveTaskState(ctx, state)
}

// TriggerTask manually triggers any registered task.
func (e *Engine) TriggerTask(name string) (*model.TaskExecution, error) {
	if _, ok := e.registry[name]; !ok {
		return nil, fmt.Errorf("task %q not found", name)
	}

	exec := &model.TaskExecution{
		TaskName:  name,
		Trigger:   TriggerManual,
		Status:    ExecPending,
		CreatedAt: time.Now(),
	}

	if err := e.store.Enqueue(context.Background(), exec); err != nil {
		return nil, err
	}

	// Wake poller
	select {
	case e.notify <- struct{}{}:
	default:
	}

	return exec, nil
}

// GetStore returns the store for use by handlers.
func (e *Engine) GetStore() Store {
	return e.store
}

// GetRegistry returns the task registry.
func (e *Engine) GetRegistry() map[string]*TaskDef {
	return e.registry
}

func (e *Engine) addCronEntry(def *TaskDef) error {
	taskName := def.Name
	id, err := e.cron.AddFunc(def.CronExpr, func() {
		exec := &model.TaskExecution{
			TaskName:  taskName,
			Trigger:   TriggerCron,
			Status:    ExecPending,
			CreatedAt: time.Now(),
		}
		if err := e.store.Enqueue(context.Background(), exec); err != nil {
			slog.Error("scheduler: failed to enqueue cron task", "task", taskName, "error", err)
			return
		}
		// Wake poller
		select {
		case e.notify <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return fmt.Errorf("scheduler: invalid cron expression for %q: %w", def.Name, err)
	}
	e.cronIDs[def.Name] = id
	slog.Info("scheduler: cron scheduled", "task", def.Name, "cron", def.CronExpr)
	return nil
}

func (e *Engine) poller() {
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-e.notify:
			e.poll()
		case <-ticker.C:
			e.poll()
		}
	}
}

func (e *Engine) poll() {
	execs, err := e.store.Dequeue(context.Background(), e.maxWorkers)
	if err != nil {
		slog.Error("scheduler: poll error", "error", err)
		return
	}
	for _, exec := range execs {
		e.executor.Submit(exec)
	}
}

func (e *Engine) recoverStale(ctx context.Context) {
	// Find executions that were running when the process stopped
	execs, _, err := e.store.ListExecutions(ctx, ExecutionFilter{Status: ExecRunning, Page: 1, PageSize: 1000})
	if err != nil {
		slog.Error("scheduler: failed to recover stale executions", "error", err)
		return
	}
	now := time.Now()
	for _, exec := range execs {
		exec.Status = ExecStale
		exec.Error = "process restarted while task was running"
		exec.FinishedAt = &now
		if err := e.store.UpdateExecution(ctx, exec); err != nil {
			slog.Error("scheduler: failed to mark stale", "execId", exec.ID, "error", err)
		}
	}
	if len(execs) > 0 {
		slog.Info("scheduler: recovered stale executions", "count", len(execs))
	}
}

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metis/internal/scheduler"
)

var (
	recoverySubmissions   = make(map[uint]time.Time)
	recoverySubmissionsMu sync.Mutex
)

// ActionExecutePayload is the async task payload for itsm-action-execute.
type ActionExecutePayload struct {
	TicketID   uint `json:"ticket_id"`
	ActivityID uint `json:"activity_id"`
	ActionID   uint `json:"action_id"`
}

// WaitTimerPayload is the async task payload for itsm-wait-timer.
type WaitTimerPayload struct {
	TicketID     uint   `json:"ticket_id"`
	ActivityID   uint   `json:"activity_id"`
	ExecuteAfter string `json:"execute_after"` // RFC3339
}

// HandleActionExecute is the scheduler task handler for itsm-action-execute.
// It executes the HTTP webhook and then calls Progress on the engine.
// For classic engine tickets, it uses classicEngine.Progress() (with token-based workflow).
// For smart engine tickets, it directly marks the activity as completed (no execution tokens).
// If the action fails and a b_error boundary event is attached, it triggers
// the boundary error path instead of calling Progress with "failed".
func HandleActionExecute(db *gorm.DB, classicEngine *ClassicEngine, smartEngine *SmartEngine) func(ctx context.Context, payload json.RawMessage) error {
	executor := NewActionExecutor(db)

	return func(ctx context.Context, payload json.RawMessage) error {
		var p ActionExecutePayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		slog.Info("executing action", "ticketID", p.TicketID, "activityID", p.ActivityID, "actionID", p.ActionID)

		err := executor.Execute(ctx, p.TicketID, p.ActivityID, p.ActionID)

		outcome := "success"
		if err != nil {
			outcome = "failed"
			slog.Error("action execution failed", "error", err, "ticketID", p.TicketID, "actionID", p.ActionID)
		}

		// Smart engine tickets: directly mark activity completed (no execution tokens).
		var ticket ticketModel
		if err := db.First(&ticket, p.TicketID).Error; err != nil {
			return fmt.Errorf("ticket %d not found: %w", p.TicketID, err)
		}
		if ticket.EngineType == "smart" {
			now := time.Now()
			if err := db.Model(&activityModel{}).Where("id = ?", p.ActivityID).Updates(map[string]any{
				"status":             ActivityCompleted,
				"transition_outcome": outcome,
				"finished_at":        now,
			}).Error; err != nil {
				return err
			}

			if smartEngine != nil {
				event := NewActionFinishedEvent(p.TicketID, p.ActivityID, outcome)
				smartEngine.DispatchDecisionAsync(event.TicketID, event.CompletedActivityID, event.TriggerReason)
			}
			return nil
		}

		// Classic engine: on failure, check for b_error boundary event before calling Progress
		if outcome == "failed" {
			if handled, bErr := tryHandleBoundaryError(ctx, db, classicEngine, p.TicketID, p.ActivityID); handled {
				if bErr != nil {
					slog.Error("boundary error handling failed", "error", bErr, "ticketID", p.TicketID)
					return bErr
				}
				return nil // b_error path handled the failure
			}
			// No b_error — fall through to Progress("failed")
		}

		// Progress the workflow with the outcome
		if progressErr := db.Transaction(func(tx *gorm.DB) error {
			return classicEngine.Progress(ctx, tx, ProgressParams{
				TicketID:   p.TicketID,
				ActivityID: p.ActivityID,
				Outcome:    outcome,
				OperatorID: 0, // system
			})
		}); progressErr != nil {
			slog.Error("failed to progress after action", "error", progressErr, "ticketID", p.TicketID)
			return progressErr
		}

		return nil
	}
}

// tryHandleBoundaryError checks if the action node has a b_error boundary event.
// Returns (true, nil) if handled successfully, (true, err) if handling failed,
// (false, nil) if no b_error exists.
func tryHandleBoundaryError(ctx context.Context, db *gorm.DB, classicEngine *ClassicEngine, ticketID, activityID uint) (bool, error) {
	// Load activity to find the node ID and token
	var activity activityModel
	if err := db.First(&activity, activityID).Error; err != nil {
		return false, nil
	}
	if activity.TokenID == nil {
		return false, nil
	}

	var hostToken executionTokenModel
	if err := db.First(&hostToken, *activity.TokenID).Error; err != nil {
		return false, nil
	}

	// Load workflow to find b_error boundary nodes
	var ticket ticketModel
	if err := db.First(&ticket, ticketID).Error; err != nil {
		return false, nil
	}

	def, nodeMap, outEdges, err := resolveWorkflowContext(db, &hostToken, ticket.WorkflowJSON)
	if err != nil {
		return false, nil
	}

	boundaryMap := def.BuildBoundaryMap()
	boundaries := boundaryMap[activity.NodeID]

	// Find a b_error boundary node
	var bErrorNode *WFNode
	for _, bNode := range boundaries {
		if bNode.Type == NodeBError {
			bErrorNode = bNode
			break
		}
	}

	if bErrorNode == nil {
		return false, nil // no b_error — not handled
	}

	// Trigger boundary error within a transaction
	txErr := db.Transaction(func(tx *gorm.DB) error {
		return classicEngine.triggerBoundaryError(ctx, tx, def, nodeMap, outEdges, ticketID, activityID, &hostToken, bErrorNode)
	})

	return true, txErr
}

// HandleWaitTimer is the scheduler task handler for itsm-wait-timer.
// It checks if the execute_after time has been reached and triggers Progress.
func HandleWaitTimer(db *gorm.DB, classicEngine *ClassicEngine) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p WaitTimerPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		executeAfter, err := time.Parse(time.RFC3339, p.ExecuteAfter)
		if err != nil {
			return fmt.Errorf("invalid execute_after time: %w", err)
		}

		// Not yet time — return ErrNotReady so scheduler retains the task
		if time.Now().Before(executeAfter) {
			return scheduler.ErrNotReady
		}

		slog.Info("wait timer expired", "ticketID", p.TicketID, "activityID", p.ActivityID)

		// Verify the activity is still active (with lock to prevent concurrent progress)
		var activity activityModel
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&activity, p.ActivityID).Error; err != nil {
			return nil // activity gone, skip
		}
		if activity.Status != ActivityPending && activity.Status != ActivityInProgress {
			return nil // already handled
		}

		return db.Transaction(func(tx *gorm.DB) error {
			return classicEngine.Progress(ctx, tx, ProgressParams{
				TicketID:   p.TicketID,
				ActivityID: p.ActivityID,
				Outcome:    "timeout",
				OperatorID: 0, // system
			})
		})
	}
}

// SmartProgressPayload is the async task payload for itsm-smart-progress.
type SmartProgressPayload struct {
	TicketID            uint   `json:"ticket_id"`
	CompletedActivityID *uint  `json:"completed_activity_id"`
	TriggerReason       string `json:"trigger_reason,omitempty"`
}

// BoundaryTimerPayload is the async task payload for itsm-boundary-timer.
type BoundaryTimerPayload struct {
	TicketID        uint   `json:"ticket_id"`
	BoundaryTokenID uint   `json:"boundary_token_id"`
	BoundaryNodeID  string `json:"boundary_node_id"`
	HostTokenID     uint   `json:"host_token_id"`
	ExecuteAfter    string `json:"execute_after"` // RFC3339
}

// HandleBoundaryTimer is the scheduler task handler for itsm-boundary-timer.
// It checks if the boundary token is still suspended (host not yet completed)
// and, if so, interrupts the host and continues via the boundary path.
func HandleBoundaryTimer(db *gorm.DB, classicEngine *ClassicEngine) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p BoundaryTimerPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		executeAfter, err := time.Parse(time.RFC3339, p.ExecuteAfter)
		if err != nil {
			return fmt.Errorf("invalid execute_after time: %w", err)
		}

		// Not yet time — return ErrNotReady so scheduler retains the task
		if time.Now().Before(executeAfter) {
			return scheduler.ErrNotReady
		}

		// Load boundary token with lock — if not suspended, host already completed
		var bToken executionTokenModel
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&bToken, p.BoundaryTokenID).Error; err != nil {
			return nil // token gone, skip
		}
		if bToken.Status != TokenSuspended {
			return nil // already handled (host completed or cancelled)
		}

		slog.Info("boundary timer expired, interrupting host",
			"ticketID", p.TicketID, "boundaryTokenID", p.BoundaryTokenID, "boundaryNodeID", p.BoundaryNodeID)

		return db.Transaction(func(tx *gorm.DB) error {
			// Cancel the host activity (find active/pending activity on the host token)
			tx.Model(&activityModel{}).
				Where("token_id = ? AND status IN ?", p.HostTokenID, []string{ActivityPending, ActivityInProgress}).
				Updates(map[string]any{
					"status":      ActivityCancelled,
					"finished_at": time.Now(),
				})

			// Cancel the host token
			tx.Model(&executionTokenModel{}).
				Where("id = ?", p.HostTokenID).
				Update("status", TokenCancelled)

			// Cancel all other boundary tokens on the same host (multi-timer: first wins)
			tx.Model(&executionTokenModel{}).
				Where("parent_token_id = ? AND token_type = ? AND status = ? AND id != ?",
					p.HostTokenID, TokenBoundary, TokenSuspended, p.BoundaryTokenID).
				Update("status", TokenCancelled)

			// Activate the boundary token
			tx.Model(&executionTokenModel{}).
				Where("id = ?", p.BoundaryTokenID).
				Update("status", TokenActive)
			bToken.Status = TokenActive

			// Load workflow and continue from boundary node's outgoing edge
			var ticket ticketModel
			if err := tx.First(&ticket, p.TicketID).Error; err != nil {
				return fmt.Errorf("ticket %d not found: %w", p.TicketID, err)
			}

			// Use host token to resolve workflow context (handles subprocess case)
			var hostToken executionTokenModel
			if err := tx.First(&hostToken, p.HostTokenID).Error; err != nil {
				return fmt.Errorf("host token %d not found: %w", p.HostTokenID, err)
			}

			def, nodeMap, outEdges, err := resolveWorkflowContext(tx, &hostToken, ticket.WorkflowJSON)
			if err != nil {
				return fmt.Errorf("resolve workflow context: %w", err)
			}

			edges := outEdges[p.BoundaryNodeID]
			if len(edges) == 0 {
				return fmt.Errorf("boundary node %s has no outgoing edge", p.BoundaryNodeID)
			}

			targetNode, ok := nodeMap[edges[0].Target]
			if !ok {
				return fmt.Errorf("boundary node target %q not found", edges[0].Target)
			}

			classicEngine.recordTimeline(tx, p.TicketID, nil, 0, "boundary_timer_fired",
				"边界定时器超时，流程已转向边界路径")

			return classicEngine.processNode(ctx, tx, def, nodeMap, outEdges, &bToken, 0, targetNode, 0)
		})
	}
}

// HandleSmartRecovery scans in_progress smart tickets and resubmits decision cycles
// for any that have no active activities and haven't been circuit-broken.
// Runs periodically (@every 10m) to recover from server restarts or lost decision cycles.
// A per-ticket dedup map prevents resubmitting the same ticket within 10 minutes.
func HandleSmartRecovery(db *gorm.DB, smartEngine *SmartEngine) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, _ json.RawMessage) error {
		// Prune dedup entries older than 10 minutes
		recoverySubmissionsMu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for id, ts := range recoverySubmissions {
			if ts.Before(cutoff) {
				delete(recoverySubmissions, id)
			}
		}
		recoverySubmissionsMu.Unlock()

		// Find all orphaned decisioning smart tickets
		type ticketRow struct {
			ID             uint
			Code           string
			AIFailureCount int
		}
		var tickets []ticketRow
		if err := db.Table("itsm_tickets").
			Where("engine_type = ? AND status IN ? AND deleted_at IS NULL", "smart", []string{
				TicketStatusApprovedDecisioning,
				TicketStatusRejectedDecisioning,
				TicketStatusDecisioning,
			}).
			Select("id, code, ai_failure_count").
			Find(&tickets).Error; err != nil {
			return fmt.Errorf("smart recovery: query tickets: %w", err)
		}

		if len(tickets) == 0 {
			slog.Info("smart recovery: no orphaned decisioning smart tickets found")
			return nil
		}

		recovered := 0
		for _, t := range tickets {
			// Skip circuit-broken tickets
			if t.AIFailureCount >= MaxAIFailureCount {
				slog.Debug("smart recovery: skipping circuit-broken ticket", "ticketID", t.ID, "code", t.Code)
				continue
			}

			// Check if there are active (pending/in_progress) activities
			var activeCount int64
			db.Table("itsm_ticket_activities").
				Where("ticket_id = ? AND status IN ?", t.ID, []string{ActivityPending, ActivityInProgress}).
				Count(&activeCount)

			if activeCount > 0 {
				slog.Debug("smart recovery: skipping ticket with active activities", "ticketID", t.ID, "code", t.Code, "activeCount", activeCount)
				continue
			}

			// Dedup: skip if submitted within the last 10 minutes
			recoverySubmissionsMu.Lock()
			if lastSubmit, ok := recoverySubmissions[t.ID]; ok && time.Since(lastSubmit) < 10*time.Minute {
				recoverySubmissionsMu.Unlock()
				slog.Debug("smart recovery: skipping recently submitted ticket", "ticketID", t.ID, "code", t.Code)
				continue
			}
			recoverySubmissionsMu.Unlock()

			smartEngine.DispatchDecisionAsync(t.ID, nil, TriggerReasonRecovery)

			// Record submission time
			recoverySubmissionsMu.Lock()
			recoverySubmissions[t.ID] = time.Now()
			recoverySubmissionsMu.Unlock()

			recovered++
			slog.Info("smart recovery: submitted progress task for orphaned ticket", "ticketID", t.ID, "code", t.Code)
		}

		slog.Info("smart recovery: completed", "scanned", len(tickets), "recovered", recovered)
		return nil
	}
}

// HandleSmartProgress is the scheduler task handler for itsm-smart-progress.
// It runs the AI decision cycle for smart engine tickets.
func HandleSmartProgress(db *gorm.DB, smartEngine *SmartEngine) func(ctx context.Context, payload json.RawMessage) error {
	return func(ctx context.Context, payload json.RawMessage) error {
		var p SmartProgressPayload
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		slog.Info("smart-progress: running decision cycle", "ticketID", p.TicketID, "completedActivityID", p.CompletedActivityID, "triggerReason", p.TriggerReason)

		err := smartEngine.RunDecisionCycleForTicket(ctx, db.WithContext(ctx), p.TicketID, p.CompletedActivityID, p.TriggerReason)

		if err != nil {
			// Decision failures are handled internally (failure count incremented),
			// so we log but don't propagate to avoid retries of already-handled failures.
			if err == ErrAIDecisionFailed || err == ErrAIDisabled {
				slog.Warn("smart-progress: decision cycle ended with handled error", "error", err, "ticketID", p.TicketID)
				return nil
			}
			slog.Error("smart-progress: decision cycle failed", "error", err, "ticketID", p.TicketID)
			return err
		}

		slog.Info("smart-progress: decision cycle completed", "ticketID", p.TicketID, "completedActivityID", p.CompletedActivityID)
		return nil
	}
}

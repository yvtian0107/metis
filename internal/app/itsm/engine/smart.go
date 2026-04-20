package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metis/internal/app"
	"metis/internal/llm"
)

// --- Interfaces for AI App dependency injection ---

// KnowledgeSearcher searches knowledge bases for context.
type KnowledgeSearcher interface {
	Search(kbIDs []uint, query string, limit int) ([]KnowledgeResult, error)
}

// KnowledgeResult is a single result from knowledge search.
type KnowledgeResult struct {
	Title   string
	Content string
	Score   float64
}

// UserProvider provides user information for policy snapshots.
type UserProvider interface {
	// ListActiveUsers returns all active users (id, name).
	ListActiveUsers() ([]ParticipantCandidate, error)
}

// EngineConfigProvider provides engine-level configuration to the SmartEngine.
type EngineConfigProvider interface {
	// FallbackAssigneeID returns the user ID of the fallback assignee (0 = not configured).
	FallbackAssigneeID() uint
	// DecisionMode returns the decision mode ("direct_first" or "ai_only").
	DecisionMode() string
	// DecisionAgentID returns the configured decision agent ID (0 = not configured).
	DecisionAgentID() uint
}

// ParticipantCandidate is a user available for assignment.
type ParticipantCandidate struct {
	UserID     uint   `json:"user_id"`
	Name       string `json:"name"`
	Department string `json:"department,omitempty"`
	Position   string `json:"position,omitempty"`
}

// --- Smart engine configuration types ---

// SmartServiceConfig is parsed from ServiceDefinition.AgentConfig JSON.
type SmartServiceConfig struct {
	ConfidenceThreshold    float64 `json:"confidence_threshold"`
	DecisionTimeoutSeconds int     `json:"decision_timeout_seconds"`
	FallbackStrategy       string  `json:"fallback_strategy"`
}

// ParseSmartServiceConfig parses agent_config JSON with defaults.
func ParseSmartServiceConfig(raw string) SmartServiceConfig {
	cfg := SmartServiceConfig{
		ConfidenceThreshold:    DefaultConfidenceThreshold,
		DecisionTimeoutSeconds: DefaultDecisionTimeoutSeconds,
		FallbackStrategy:       "manual_queue",
	}
	if raw != "" {
		json.Unmarshal([]byte(raw), &cfg)
	}
	if cfg.ConfidenceThreshold <= 0 {
		cfg.ConfidenceThreshold = DefaultConfidenceThreshold
	}
	if cfg.DecisionTimeoutSeconds <= 0 {
		cfg.DecisionTimeoutSeconds = DefaultDecisionTimeoutSeconds
	}
	return cfg
}

// --- Decision plan types ---

// DecisionPlan is the structured output from the AI agent.
type DecisionPlan struct {
	NextStepType  string             `json:"next_step_type"` // approve|process|action|notify|form|complete|escalate
	ExecutionMode string             `json:"execution_mode"` // ""|"single"|"parallel"
	Activities    []DecisionActivity `json:"activities"`
	Reasoning     string             `json:"reasoning"`
	Confidence    float64            `json:"confidence"`
}

// DecisionActivity is a single activity within a decision plan.
type DecisionActivity struct {
	Type            string `json:"type"`
	ParticipantType string `json:"participant_type,omitempty"`
	ParticipantID   *uint  `json:"participant_id,omitempty"`
	PositionCode    string `json:"position_code,omitempty"`
	DepartmentCode  string `json:"department_code,omitempty"`
	ActionID        *uint  `json:"action_id,omitempty"`
	Instructions    string `json:"instructions,omitempty"`
}

// Allowed next_step_types for smart engine.
var AllowedSmartStepTypes = map[string]bool{
	"approve": true, "process": true, "action": true,
	"notify": true, "form": true, "complete": true, "escalate": true,
}

// --- SmartEngine ---

// SmartEngine implements WorkflowEngine via AI Agent-driven decisions.
type SmartEngine struct {
	decisionExecutor  app.AIDecisionExecutor
	knowledgeSearcher KnowledgeSearcher
	userProvider      UserProvider
	resolver          *ParticipantResolver
	scheduler         TaskSubmitter
	configProvider    EngineConfigProvider
	actionExecutor    *ActionExecutor
}

// NewSmartEngine creates a SmartEngine with optional AI dependencies.
func NewSmartEngine(
	decisionExecutor app.AIDecisionExecutor,
	knowledgeSearcher KnowledgeSearcher,
	userProvider UserProvider,
	resolver *ParticipantResolver,
	scheduler TaskSubmitter,
	configProvider EngineConfigProvider,
) *SmartEngine {
	return &SmartEngine{
		decisionExecutor:  decisionExecutor,
		knowledgeSearcher: knowledgeSearcher,
		userProvider:      userProvider,
		resolver:          resolver,
		scheduler:         scheduler,
		configProvider:    configProvider,
	}
}

// IsAvailable returns true if the smart engine has AI dependencies.
func (e *SmartEngine) IsAvailable() bool {
	return e.decisionExecutor != nil
}

// SetActionExecutor injects the action executor for decision.execute_action tool support.
func (e *SmartEngine) SetActionExecutor(executor *ActionExecutor) {
	e.actionExecutor = executor
}

// Start initialises the workflow for a smart-engine ticket.
func (e *SmartEngine) Start(ctx context.Context, tx *gorm.DB, params StartParams) error {
	if !e.IsAvailable() {
		return ErrSmartEngineUnavailable
	}

	// Load service definition for agent config
	svcInfo, err := e.loadServiceForTicket(tx, params.TicketID)
	if err != nil {
		return fmt.Errorf("load service: %w", err)
	}

	if svcInfo.AgentID == nil || *svcInfo.AgentID == 0 {
		return fmt.Errorf("智能服务未绑定 Agent")
	}

	// Update ticket status to in_progress
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).
		Update("status", "in_progress").Error; err != nil {
		return err
	}

	// Record timeline: workflow started
	e.recordTimeline(tx, params.TicketID, nil, params.RequesterID, "workflow_started", "智能流程已启动", "")

	// Execute the decision cycle
	return e.runDecisionCycle(ctx, tx, params.TicketID, nil, svcInfo)
}

// Progress advances the workflow after an activity is completed.
func (e *SmartEngine) Progress(ctx context.Context, tx *gorm.DB, params ProgressParams) error {
	if !e.IsAvailable() {
		return ErrSmartEngineUnavailable
	}

	// Load and complete the activity
	var activity activityModel
	if err := tx.First(&activity, params.ActivityID).Error; err != nil {
		return ErrActivityNotFound
	}
	if activity.Status != ActivityPending && activity.Status != ActivityInProgress {
		return ErrActivityNotActive
	}

	now := time.Now()
	updates := map[string]any{
		"status":             ActivityCompleted,
		"transition_outcome": params.Outcome,
		"finished_at":        now,
	}
	if len(params.Result) > 0 {
		updates["form_data"] = string(params.Result)
	}
	if err := tx.Model(&activityModel{}).Where("id = ?", params.ActivityID).Updates(updates).Error; err != nil {
		return err
	}

	e.recordTimeline(tx, params.TicketID, &params.ActivityID, params.OperatorID, "activity_completed",
		fmt.Sprintf("活动 [%s] 完成，结果: %s", activity.Name, params.Outcome), "")

	// Load ticket for ensureContinuation
	var ticket ticketModel
	if err := tx.First(&ticket, params.TicketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	e.ensureContinuation(tx, &ticket, params.ActivityID)
	return nil
}

// Cancel terminates all active activities and marks the ticket cancelled.
func (e *SmartEngine) Cancel(ctx context.Context, tx *gorm.DB, params CancelParams) error {
	// Cancel all active activities
	if err := tx.Model(&activityModel{}).
		Where("ticket_id = ? AND status IN ?", params.TicketID,
			[]string{ActivityPending, ActivityPendingApproval, ActivityInProgress}).
		Updates(map[string]any{
			"status":      ActivityCancelled,
			"finished_at": time.Now(),
		}).Error; err != nil {
		return err
	}

	// Cancel all pending assignments
	if err := tx.Model(&assignmentModel{}).
		Where("ticket_id = ? AND status = ?", params.TicketID, "pending").
		Update("status", "cancelled").Error; err != nil {
		return err
	}

	// Update ticket
	now := time.Now()
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).Updates(map[string]any{
		"status":      "cancelled",
		"finished_at": now,
	}).Error; err != nil {
		return err
	}

	eventType := params.EventType
	if eventType == "" {
		eventType = "ticket_cancelled"
	}
	msg := params.Message
	if msg == "" {
		msg = "工单已取消"
		if params.Reason != "" {
			msg = "工单已取消: " + params.Reason
		}
	}
	return e.recordTimeline(tx, params.TicketID, nil, params.OperatorID, eventType, msg, "")
}

// --- Decision cycle ---

// runDecisionCycle executes the full AI decision cycle for a ticket.
func (e *SmartEngine) runDecisionCycle(ctx context.Context, tx *gorm.DB, ticketID uint, completedActivityID *uint, svcInfo *serviceModel) error {
	// Check if AI is disabled for this ticket
	var ticket ticketModel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	if ticket.AIFailureCount >= MaxAIFailureCount {
		e.recordTimeline(tx, ticketID, nil, 0, "ai_disabled",
			fmt.Sprintf("AI 决策已停用（连续 %d 次失败），请手动处理", ticket.AIFailureCount), "")
		return ErrAIDisabled
	}

	cfg := ParseSmartServiceConfig(svcInfo.AgentConfig)

	// Check terminal state
	switch ticket.Status {
	case "completed", "cancelled", "failed":
		return nil
	}

	var activeCount int64
	if err := tx.Model(&activityModel{}).
		Where("ticket_id = ? AND status IN ?", ticketID, []string{ActivityPending, ActivityInProgress, ActivityPendingApproval}).
		Count(&activeCount).Error; err != nil {
		return err
	}
	if activeCount > 0 {
		slog.Info("smart-progress: active activities already exist, skipping duplicate continuation",
			"ticketID", ticketID, "activeCount", activeCount, "completedActivityID", completedActivityID)
		return nil
	}

	// Call agentic decision with timeout
	timeout := time.Duration(cfg.DecisionTimeoutSeconds) * time.Second
	decisionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	plan, err := e.agenticDecision(decisionCtx, tx, ticketID, svcInfo)
	if err != nil {
		reason := fmt.Sprintf("AI 决策失败: %v", err)
		if decisionCtx.Err() == context.DeadlineExceeded {
			reason = fmt.Sprintf("AI 决策超时（%ds）", cfg.DecisionTimeoutSeconds)
		}
		return e.handleDecisionFailure(tx, ticketID, reason)
	}

	// Validate decision plan
	if err := e.validateDecisionPlan(tx, plan, svcInfo); err != nil {
		return e.handleDecisionFailure(tx, ticketID, fmt.Sprintf("AI 决策校验不通过: %v", err))
	}

	// Reset failure count on success
	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("ai_failure_count", 0)

	// Handle "complete" decision
	if plan.NextStepType == "complete" {
		return e.handleComplete(tx, ticketID, plan)
	}

	// Evaluate confidence
	if plan.Confidence >= cfg.ConfidenceThreshold {
		return e.executeDecisionPlan(tx, ticketID, plan)
	}
	return e.pendApprovalDecisionPlan(tx, ticketID, plan)
}

// handleDecisionFailure increments failure count and records timeline.
func (e *SmartEngine) handleDecisionFailure(tx *gorm.DB, ticketID uint, reason string) error {
	tx.Model(&ticketModel{}).Where("id = ?", ticketID).
		UpdateColumn("ai_failure_count", gorm.Expr("ai_failure_count + 1"))

	e.recordTimeline(tx, ticketID, nil, 0, "ai_decision_failed", reason, "")

	// Check if we should disable AI
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err == nil {
		if ticket.AIFailureCount+1 >= MaxAIFailureCount {
			e.recordTimeline(tx, ticketID, nil, 0, "ai_disabled",
				fmt.Sprintf("AI 决策已停用（连续 %d 次失败），请手动处理", ticket.AIFailureCount+1), "")
		}
	}

	return ErrAIDecisionFailed
}

// handleComplete finishes the ticket when agent decides to complete.
func (e *SmartEngine) handleComplete(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	now := time.Now()

	// Create a completed end activity
	act := &activityModel{
		TicketID:          ticketID,
		Name:              "流程完结",
		ActivityType:      "complete",
		Status:            ActivityCompleted,
		AIDecision:        mustJSON(plan),
		AIReasoning:       plan.Reasoning,
		AIConfidence:      plan.Confidence,
		TransitionOutcome: "completed",
		StartedAt:         &now,
		FinishedAt:        &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	if err := tx.Model(&ticketModel{}).Where("id = ?", ticketID).Updates(map[string]any{
		"status":              "completed",
		"finished_at":         now,
		"current_activity_id": act.ID,
	}).Error; err != nil {
		return err
	}

	return e.recordTimeline(tx, ticketID, &act.ID, 0, "workflow_completed",
		"智能流程已完结", plan.Reasoning)
}

// executeDecisionPlan creates activities for a high-confidence decision.
func (e *SmartEngine) executeDecisionPlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	if plan.ExecutionMode == "parallel" {
		return e.executeParallelPlan(tx, ticketID, plan)
	}
	return e.executeSinglePlan(tx, ticketID, plan)
}

// executeParallelPlan creates a group of parallel activities sharing the same activity_group_id.
func (e *SmartEngine) executeParallelPlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	now := time.Now()
	planJSON := mustJSON(plan)
	groupID := uuid.New().String()

	var firstActID uint
	for i, da := range plan.Activities {
		status := ActivityPending
		act := &activityModel{
			TicketID:        ticketID,
			Name:            decisionActivityName(da),
			ActivityType:    da.Type,
			Status:          status,
			ExecutionMode:   "parallel",
			ActivityGroupID: groupID,
			AIDecision:      planJSON,
			AIReasoning:     plan.Reasoning,
			AIConfidence:    plan.Confidence,
			StartedAt:       &now,
		}
		if err := tx.Create(act).Error; err != nil {
			return err
		}

		if i == 0 {
			firstActID = act.ID
			tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)
		}

		// Create assignment
		if da.ParticipantID != nil && *da.ParticipantID > 0 {
			assignment := &assignmentModel{
				TicketID:        ticketID,
				ActivityID:      act.ID,
				ParticipantType: "user",
				UserID:          da.ParticipantID,
				AssigneeID:      da.ParticipantID,
				Status:          "pending",
				IsCurrent:       true,
			}
			tx.Create(assignment)
		} else if da.ParticipantType == "position_department" && da.PositionCode != "" && da.DepartmentCode != "" {
			e.createPositionAssignment(tx, ticketID, act.ID, da.PositionCode, da.DepartmentCode)
		} else if da.Type == "approve" || da.Type == "process" || da.Type == "form" {
			e.tryFallbackAssignment(tx, ticketID, act.ID)
		}

		// Submit action task if applicable
		if da.Type == "action" && da.ActionID != nil && e.scheduler != nil {
			payload, _ := json.Marshal(map[string]any{
				"ticket_id":   ticketID,
				"activity_id": act.ID,
				"action_id":   *da.ActionID,
			})
			e.scheduler.SubmitTask("itsm-action-execute", payload)
		}

		e.recordTimeline(tx, ticketID, &act.ID, 0, "ai_decision_executed",
			fmt.Sprintf("AI 并签活动：%s（组 %s，信心 %.0f%%）", decisionActivityName(da), groupID[:8], plan.Confidence*100),
			plan.Reasoning)
	}

	_ = firstActID
	return nil
}

// executeSinglePlan creates only the current activity for single-mode plans.
func (e *SmartEngine) executeSinglePlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	if len(plan.Activities) == 0 {
		return fmt.Errorf("decision plan has no activities")
	}

	now := time.Now()
	planJSON := mustJSON(plan)
	da := plan.Activities[0]
	status := ActivityInProgress
	if da.Type == "approve" || da.Type == "form" || da.Type == "process" {
		status = ActivityPending
	}

	act := &activityModel{
		TicketID:     ticketID,
		Name:         decisionActivityName(da),
		ActivityType: da.Type,
		Status:       status,
		AIDecision:   planJSON,
		AIReasoning:  plan.Reasoning,
		AIConfidence: plan.Confidence,
		StartedAt:    &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}
	if err := tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID).Error; err != nil {
		return err
	}

	if da.ParticipantID != nil && *da.ParticipantID > 0 {
		assignment := &assignmentModel{
			TicketID:        ticketID,
			ActivityID:      act.ID,
			ParticipantType: "user",
			UserID:          da.ParticipantID,
			AssigneeID:      da.ParticipantID,
			Status:          "pending",
			IsCurrent:       true,
		}
		if err := tx.Create(assignment).Error; err != nil {
			return err
		}
		if err := tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", *da.ParticipantID).Error; err != nil {
			return err
		}
	} else if da.ParticipantType == "position_department" && da.PositionCode != "" && da.DepartmentCode != "" {
		e.createPositionAssignment(tx, ticketID, act.ID, da.PositionCode, da.DepartmentCode)
	} else if da.Type == "approve" || da.Type == "process" || da.Type == "form" {
		e.tryFallbackAssignment(tx, ticketID, act.ID)
	}

	if da.Type == "action" && da.ActionID != nil && e.scheduler != nil {
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":   ticketID,
			"activity_id": act.ID,
			"action_id":   *da.ActionID,
		})
		e.scheduler.SubmitTask("itsm-action-execute", payload)
	}

	e.recordTimeline(tx, ticketID, &act.ID, 0, "ai_decision_executed",
		fmt.Sprintf("AI 自动执行：%s（信心 %.0f%%）", decisionActivityName(da), plan.Confidence*100),
		plan.Reasoning)

	return nil
}

// tryFallbackAssignment checks engine config for a fallback assignee and creates
// an assignment if the fallback user is valid. Records timeline events.
func (e *SmartEngine) tryFallbackAssignment(tx *gorm.DB, ticketID uint, activityID uint) {
	if e.configProvider == nil {
		return
	}
	fallbackID := e.configProvider.FallbackAssigneeID()
	if fallbackID == 0 {
		return
	}

	// Verify the fallback user exists and is active
	var user struct {
		Username string
		IsActive bool
	}
	if err := tx.Table("users").Where("id = ? AND deleted_at IS NULL", fallbackID).
		Select("username, is_active").First(&user).Error; err != nil || !user.IsActive {
		e.recordTimeline(tx, ticketID, &activityID, 0, "participant_fallback_warning",
			fmt.Sprintf("兜底处理人无效（ID=%d），请检查引擎配置", fallbackID), "")
		return
	}

	assignment := &assignmentModel{
		TicketID:        ticketID,
		ActivityID:      activityID,
		ParticipantType: "user",
		UserID:          &fallbackID,
		AssigneeID:      &fallbackID,
		Status:          "pending",
		IsCurrent:       true,
	}
	if err := tx.Create(assignment).Error; err != nil {
		slog.Error("failed to create fallback assignment", "error", err, "ticketID", ticketID)
		return
	}
	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", fallbackID)
	e.recordTimeline(tx, ticketID, &activityID, 0, "participant_fallback",
		fmt.Sprintf("参与者缺失，已转派兜底处理人（%s）", user.Username), "")
}

// createPositionAssignment resolves position_department participant type to actual
// users and creates the assignment with position/department IDs.
func (e *SmartEngine) createPositionAssignment(tx *gorm.DB, ticketID, activityID uint, positionCode, departmentCode string) {
	if e.resolver == nil || e.resolver.orgResolver == nil {
		slog.Warn("position assignment skipped: org resolver not available", "ticketID", ticketID)
		return
	}

	// Resolve user IDs via org service
	userIDs, err := e.resolver.orgResolver.FindUsersByPositionAndDepartment(positionCode, departmentCode)
	if err != nil || len(userIDs) == 0 {
		slog.Warn("position assignment: no users found", "positionCode", positionCode, "departmentCode", departmentCode)
	}

	// Look up position and department IDs (best-effort, tables may not exist in tests)
	var positionID, departmentID uint
	tx.Table("positions").Where("code = ?", positionCode).Select("id").Scan(&positionID)
	tx.Table("departments").Where("code = ?", departmentCode).Select("id").Scan(&departmentID)

	assignment := &assignmentModel{
		TicketID:        ticketID,
		ActivityID:      activityID,
		ParticipantType: "position_department",
		Status:          "pending",
		IsCurrent:       true,
	}
	if positionID > 0 {
		assignment.PositionID = &positionID
	}
	if departmentID > 0 {
		assignment.DepartmentID = &departmentID
	}
	if len(userIDs) > 0 {
		assignment.UserID = &userIDs[0]
	}

	if err := tx.Create(assignment).Error; err != nil {
		slog.Error("failed to create position assignment", "error", err, "ticketID", ticketID)
		return
	}

	if len(userIDs) > 0 {
		tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", userIDs[0])
	}
}

// pendApprovalDecisionPlan creates a pending_approval activity for low-confidence decision.
func (e *SmartEngine) pendApprovalDecisionPlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	now := time.Now()
	planJSON := mustJSON(plan)

	// Create activity in pending_approval state
	name := "AI 决策待确认"
	if len(plan.Activities) > 0 {
		name = fmt.Sprintf("AI 决策待确认：%s", decisionActivityName(plan.Activities[0]))
	}

	act := &activityModel{
		TicketID:     ticketID,
		Name:         name,
		ActivityType: plan.NextStepType,
		Status:       ActivityPendingApproval,
		AIDecision:   planJSON,
		AIReasoning:  plan.Reasoning,
		AIConfidence: plan.Confidence,
		StartedAt:    &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	if err := tx.Model(&ticketModel{}).Where("id = ?", ticketID).Updates(map[string]any{
		"current_activity_id": act.ID,
		"assignee_id":         nil,
	}).Error; err != nil {
		return err
	}

	if len(plan.Activities) > 0 {
		da := plan.Activities[0]
		switch {
		case da.ParticipantID != nil && *da.ParticipantID > 0:
			assignment := &assignmentModel{
				TicketID:        ticketID,
				ActivityID:      act.ID,
				ParticipantType: "user",
				UserID:          da.ParticipantID,
				AssigneeID:      da.ParticipantID,
				Status:          "pending",
				IsCurrent:       true,
			}
			if err := tx.Create(assignment).Error; err != nil {
				return err
			}
			if err := tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", *da.ParticipantID).Error; err != nil {
				return err
			}
		case da.ParticipantType == "position_department" && da.PositionCode != "" && da.DepartmentCode != "":
			e.createPositionAssignment(tx, ticketID, act.ID, da.PositionCode, da.DepartmentCode)
		default:
			e.tryFallbackAssignment(tx, ticketID, act.ID)
		}
	}

	e.recordTimeline(tx, ticketID, &act.ID, 0, "ai_decision_pending",
		fmt.Sprintf("AI 决策信心不足（%.0f%%），等待人工确认", plan.Confidence*100),
		plan.Reasoning)

	return nil
}

// --- Validation ---

func (e *SmartEngine) validateDecisionPlan(tx *gorm.DB, plan *DecisionPlan, svc *serviceModel) error {
	if plan == nil {
		return fmt.Errorf("decision plan is nil")
	}

	// Check next_step_type is allowed
	if !AllowedSmartStepTypes[plan.NextStepType] {
		return fmt.Errorf("next_step_type %q 不合法", plan.NextStepType)
	}

	// Validate activities
	for i, a := range plan.Activities {
		if !AllowedSmartStepTypes[a.Type] {
			return fmt.Errorf("activities[%d].type %q 不合法", i, a.Type)
		}

		// Check participant is an active user
		if a.ParticipantID != nil && *a.ParticipantID > 0 {
			var user struct {
				IsActive bool
			}
			if err := tx.Table("users").Where("id = ?", *a.ParticipantID).
				Select("is_active").First(&user).Error; err != nil {
				return fmt.Errorf("activities[%d].participant_id %d 用户不存在", i, *a.ParticipantID)
			}
			if !user.IsActive {
				return fmt.Errorf("activities[%d].participant_id %d 用户未激活", i, *a.ParticipantID)
			}
		}

		// Check action_id exists for this service
		if a.Type == "action" && a.ActionID != nil && *a.ActionID > 0 {
			var count int64
			tx.Table("itsm_service_actions").
				Where("id = ? AND service_id = ? AND is_active = ? AND deleted_at IS NULL",
					*a.ActionID, svc.ID, true).
				Count(&count)
			if count == 0 {
				return fmt.Errorf("activities[%d].action_id %d 不在服务可用动作列表中", i, *a.ActionID)
			}
		}
	}

	// Validate confidence is in range
	if plan.Confidence < 0 || plan.Confidence > 1 {
		return fmt.Errorf("confidence %.2f 不在 [0, 1] 范围内", plan.Confidence)
	}

	return nil
}

// --- Helpers ---

// loadServiceForTicket loads service definition info for a ticket.
func (e *SmartEngine) loadServiceForTicket(tx *gorm.DB, ticketID uint) (*serviceModel, error) {
	var svc serviceModel
	err := tx.Table("itsm_service_definitions").
		Joins("JOIN itsm_tickets ON itsm_tickets.service_id = itsm_service_definitions.id").
		Where("itsm_tickets.id = ?", ticketID).
		Select("itsm_service_definitions.*").
		First(&svc).Error
	return &svc, err
}

func (e *SmartEngine) recordTimeline(tx *gorm.DB, ticketID uint, activityID *uint, operatorID uint, eventType, message, reasoning string) error {
	tl := &timelineModel{
		TicketID:   ticketID,
		ActivityID: activityID,
		OperatorID: operatorID,
		EventType:  eventType,
		Message:    message,
		Reasoning:  reasoning,
	}
	return tx.Create(tl).Error
}

func parseDecisionPlan(content string) (*DecisionPlan, error) {
	// Try to extract JSON from the response (may be wrapped in markdown code blocks)
	jsonStr := llm.ExtractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("无法从 Agent 输出中提取 JSON")
	}

	var plan DecisionPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return &plan, nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func decisionActivityName(da DecisionActivity) string {
	switch da.Type {
	case "approve":
		return "审批"
	case "process":
		return "处理"
	case "action":
		return "自动动作"
	case "notify":
		return "通知"
	case "form":
		return "表单填写"
	case "complete":
		return "完结"
	case "escalate":
		return "升级"
	default:
		return da.Type
	}
}

// serviceModel is a lightweight struct for reading service definition data.
type serviceModel struct {
	ID                uint   `gorm:"primaryKey"`
	Name              string `gorm:"column:name"`
	Description       string `gorm:"column:description"`
	EngineType        string `gorm:"column:engine_type"`
	CollaborationSpec string `gorm:"column:collaboration_spec"`
	AgentID           *uint  `gorm:"column:agent_id"`
	AgentConfig       string `gorm:"column:agent_config"`
	KnowledgeBaseIDs  string `gorm:"column:knowledge_base_ids"`
	WorkflowJSON      string `gorm:"column:workflow_json"`
}

func (serviceModel) TableName() string { return "itsm_service_definitions" }

// ensureContinuation checks whether the smart engine should trigger the next
// decision cycle after an activity completes. It handles terminal states,
// circuit-breaker, and parallel convergence (with SELECT FOR UPDATE).
func (e *SmartEngine) ensureContinuation(tx *gorm.DB, ticket *ticketModel, completedActivityID uint) {
	// 1. Terminal state → nothing to do
	switch ticket.Status {
	case "completed", "cancelled", "failed":
		return
	}

	// 2. Circuit-breaker
	if ticket.AIFailureCount >= MaxAIFailureCount {
		slog.Warn("ensureContinuation: AI disabled, skipping", "ticketID", ticket.ID, "failures", ticket.AIFailureCount)
		return
	}

	// 3. Parallel convergence check
	if completedActivityID > 0 {
		var groupID string
		tx.Model(&activityModel{}).Where("id = ?", completedActivityID).Select("activity_group_id").Scan(&groupID)

		if groupID != "" {
			// Lock the group rows and check for incomplete siblings
			var ids []uint
			tx.Model(&activityModel{}).
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("activity_group_id = ? AND status NOT IN (?, ?)", groupID, ActivityCompleted, ActivityCancelled).
				Pluck("id", &ids)

			if len(ids) > 0 {
				slog.Info("ensureContinuation: parallel group not converged",
					"ticketID", ticket.ID, "groupID", groupID, "remaining", len(ids))
				return
			}
			slog.Info("ensureContinuation: parallel group converged",
				"ticketID", ticket.ID, "groupID", groupID)
		}
	}

	// 4. Submit async task for next decision cycle
	if e.scheduler != nil {
		payload, _ := json.Marshal(SmartProgressPayload{
			TicketID:            ticket.ID,
			CompletedActivityID: uintPtrIf(completedActivityID),
		})
		if err := e.scheduler.SubmitTask("itsm-smart-progress", payload); err != nil {
			slog.Error("ensureContinuation: failed to submit smart-progress task", "error", err, "ticketID", ticket.ID)
		}
	}
}

// uintPtrIf returns a *uint if v > 0, else nil.
func uintPtrIf(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

// ExecuteConfirmedPlan executes a confirmed decision plan (after human approval).
func (e *SmartEngine) ExecuteConfirmedPlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	if plan.NextStepType == "complete" {
		return e.handleComplete(tx, ticketID, plan)
	}
	return e.executeDecisionPlan(tx, ticketID, plan)
}

// SubmitProgressTask submits an async smart-progress task.
func (e *SmartEngine) SubmitProgressTask(payload json.RawMessage) error {
	if e.scheduler != nil {
		return e.scheduler.SubmitTask("itsm-smart-progress", payload)
	}
	return nil
}

// RunDecisionCycleForTicket runs the decision cycle for a ticket (used by scheduler task).
func (e *SmartEngine) RunDecisionCycleForTicket(ctx context.Context, tx *gorm.DB, ticketID uint, completedActivityID *uint) error {
	svcInfo, err := e.loadServiceForTicket(tx, ticketID)
	if err != nil {
		return fmt.Errorf("load service: %w", err)
	}
	return e.runDecisionCycle(ctx, tx, ticketID, completedActivityID, svcInfo)
}

// --- Agentic decision (delegates to DecisionExecutor) ---

// agenticDecision builds domain context and tools, then delegates the ReAct loop
// to the DecisionExecutor (implemented by the AI App).
func (e *SmartEngine) agenticDecision(ctx context.Context, tx *gorm.DB, ticketID uint, svc *serviceModel) (*DecisionPlan, error) {
	var agentID uint
	if e.configProvider != nil {
		agentID = e.configProvider.DecisionAgentID()
	}
	if agentID == 0 {
		return nil, fmt.Errorf("决策智能体未配置")
	}

	// Build seed messages (domain context)
	var decisionMode string
	if e.configProvider != nil {
		decisionMode = e.configProvider.DecisionMode()
	}
	systemMsg, userMsg, err := e.buildInitialSeed(tx, ticketID, svc, decisionMode)
	if err != nil {
		return nil, fmt.Errorf("build initial seed: %w", err)
	}

	// Prepare tool context
	toolCtx := &decisionToolContext{
		ctx:               ctx,
		data:              NewDecisionDataStore(tx),
		ticketID:          ticketID,
		serviceID:         svc.ID,
		knowledgeSearcher: e.knowledgeSearcher,
		resolver:          e.resolver,
		actionExecutor:    e.actionExecutor,
	}
	if svc.KnowledgeBaseIDs != "" {
		var kbIDs []uint
		if err := json.Unmarshal([]byte(svc.KnowledgeBaseIDs), &kbIDs); err == nil {
			toolCtx.knowledgeBaseIDs = kbIDs
		}
	}

	// Build tool definitions and handler map
	tools := allDecisionTools()
	handlerMap := make(map[string]func(*decisionToolContext, json.RawMessage) (json.RawMessage, error))
	toolDefs := make([]app.AIToolDef, len(tools))
	for i, t := range tools {
		toolDefs[i] = app.AIToolDef{
			Name:        t.Def.Name,
			Description: t.Def.Description,
			Parameters:  t.Def.Parameters,
		}
		handlerMap[t.Def.Name] = t.Handler
	}

	// Build tool handler closure
	toolHandler := func(name string, args json.RawMessage) (json.RawMessage, error) {
		handler, ok := handlerMap[name]
		if !ok {
			return toolError(fmt.Sprintf("未知工具: %s", name))
		}
		return handler(toolCtx, args)
	}

	resp, err := e.decisionExecutor.Execute(ctx, agentID, app.AIDecisionRequest{
		SystemPrompt: systemMsg,
		UserMessage:  userMsg,
		Tools:        toolDefs,
		ToolHandler:  toolHandler,
		MaxTurns:     DecisionToolMaxTurns,
	})
	if err != nil {
		return nil, err
	}

	slog.Info("agentic decision completed", "ticketID", ticketID, "turns", resp.Turns,
		"inputTokens", resp.InputTokens, "outputTokens", resp.OutputTokens)

	return parseDecisionPlan(resp.Content)
}

// buildInitialSeed constructs the system and user messages for the agentic decision.
// The seed is intentionally lightweight — the agent queries detailed context via tools.
func (e *SmartEngine) buildInitialSeed(tx *gorm.DB, ticketID uint, svc *serviceModel, decisionMode string) (string, string, error) {
	// System message (domain context only; agent's system prompt is prepended by DecisionExecutor)
	systemMsg := buildAgenticSystemPrompt(svc.CollaborationSpec, decisionMode, svc.WorkflowJSON)

	// User message: lightweight ticket snapshot + policy constraints
	var ticket struct {
		Code        string
		Title       string
		Description string
		Status      string
		Source      string
		PriorityID  uint
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).
		Select("code, title, description, status, source, priority_id").
		First(&ticket).Error; err != nil {
		return "", "", fmt.Errorf("ticket not found: %w", err)
	}

	var priorityName string
	if ticket.PriorityID > 0 {
		var p struct{ Name string }
		if err := tx.Table("itsm_priorities").Where("id = ?", ticket.PriorityID).
			Select("name").First(&p).Error; err == nil {
			priorityName = p.Name
		}
	}

	allowedSteps := []string{"approve", "process", "action", "notify", "form", "complete", "escalate"}
	switch ticket.Status {
	case "completed", "cancelled", "failed":
		allowedSteps = []string{}
	}

	seed := map[string]any{
		"ticket": map[string]any{
			"code":        ticket.Code,
			"title":       ticket.Title,
			"description": ticket.Description,
			"status":      ticket.Status,
			"source":      ticket.Source,
			"priority":    priorityName,
		},
		"service": map[string]any{
			"name":        svc.Name,
			"description": svc.Description,
		},
		"policy": map[string]any{
			"allowed_step_types": allowedSteps,
			"current_status":     ticket.Status,
		},
	}
	seedJSON, _ := json.MarshalIndent(seed, "", "  ")
	userMsg := fmt.Sprintf("## 工单信息与策略约束\n\n```json\n%s\n```\n\n请使用可用工具收集所需信息，然后输出最终决策。", seedJSON)

	return systemMsg, userMsg, nil
}

// buildAgenticSystemPrompt constructs the domain system prompt for the agentic decision.
// The agent's own system prompt is prepended by the DecisionExecutor.
func buildAgenticSystemPrompt(collaborationSpec, decisionMode, workflowJSON string) string {
	prompt := ""
	if collaborationSpec != "" {
		prompt += "## 服务处理规范\n\n" + collaborationSpec + "\n\n---\n\n"
	}
	// DecisionMode prompt injection
	switch decisionMode {
	case "ai_only":
		prompt += "## 决策策略\n\n始终使用 AI 推理决定下一步，不依赖预定义路径。\n\n---\n\n"
	default: // "direct_first" or empty
		hints := extractWorkflowHints(workflowJSON)
		if hints != "" {
			prompt += "## 决策策略\n\n优先参考以下工作流路径来决定下一步，无法确定时使用 AI 推理。\n\n"
			prompt += "## 工作流参考路径\n\n" + hints + "\n\n---\n\n"
		} else {
			slog.Warn("direct_first mode but no workflow hints available, degrading to ai_only")
			prompt += "## 决策策略\n\n始终使用 AI 推理决定下一步，不依赖预定义路径。\n\n---\n\n"
		}
	}
	prompt += agenticToolGuidance
	prompt += "\n\n---\n\n"
	prompt += agenticOutputFormat
	return prompt
}

const agenticToolGuidance = `## 工具使用指引

你可以通过以下工具按需查询信息来辅助决策：

- **decision.ticket_context** — 获取工单完整上下文（表单数据、SLA、活动历史、当前指派、已执行动作、并签组状态）
- **decision.knowledge_search** — 搜索服务关联知识库
- **decision.resolve_participant** — 按类型解析参与人（user/position/department/position_department/requester_manager）
- **decision.user_workload** — 查询用户当前工单负载
- **decision.similar_history** — 查询同服务历史工单的处理模式
- **decision.sla_status** — 查询 SLA 状态和紧急程度
- **decision.list_actions** — 查询服务可用的自动化动作
- **decision.execute_action** — 同步执行服务配置的自动化动作（webhook），返回执行结果。在决策推理过程中直接触发自动化操作并观察结果。

### 推荐推理步骤

1. 先用 decision.ticket_context 了解完整上下文（注意 activity_history、executed_actions 和 parallel_groups）
2. 用 decision.list_actions 查看是否有可用的自动化动作
3. 如需查阅处理规范，使用 decision.knowledge_search
4. 如果协作规范要求执行某个动作（如预检、放行等），使用 decision.execute_action 同步执行并获取结果，而非输出 type 为 "action" 的活动
5. 如果需要多角色并行审批（并签），设置 execution_mode 为 "parallel"，在 activities 中列出所有并行角色
6. 如果需要人工审批，用 decision.resolve_participant 解析指派人
7. 可选：用 decision.user_workload 做负载均衡，用 decision.similar_history 参考历史模式
8. 最终输出决策 JSON（不调用任何工具）

### 完成判断

当 decision.ticket_context 返回的 all_actions_completed 为 true，表示服务配置的所有自动化动作均已成功执行。此时请对照协作规范判断流程是否应当结束——如果规范说"动作完成后结束"，你必须输出 next_step_type 为 "complete"，不要再创建新活动。`

const agenticOutputFormat = "## 输出要求\n\n" +
	"当你完成信息收集和推理后，直接输出以下 JSON 格式的决策（不要再调用任何工具）：\n\n" +
	"```json\n" +
	"{\n" +
	"  \"next_step_type\": \"process|approve|action|notify|form|complete|escalate\",\n" +
	"  \"execution_mode\": \"single|parallel\",\n" +
	"  \"activities\": [\n" +
	"    {\n" +
	"      \"type\": \"process|approve|action|notify|form\",\n" +
	"      \"participant_type\": \"user|position_department\",\n" +
	"      \"participant_id\": 42,\n" +
	"      \"position_code\": \"db_admin\",\n" +
	"      \"department_code\": \"it\",\n" +
	"      \"action_id\": null,\n" +
	"      \"instructions\": \"操作指引\"\n" +
	"    }\n" +
	"  ],\n" +
	"  \"reasoning\": \"决策推理过程...\",\n" +
	"  \"confidence\": 0.85\n" +
	"}\n" +
	"```\n\n" +
	"字段说明：\n" +
	"- next_step_type: 下一步类型。\"complete\" 表示流程可以结束。\n" +
	"- execution_mode: 执行模式。\"single\"（默认）为串行，\"parallel\" 为并签模式——activities 中的多个活动将并行等待处理，全部完成后才推进到下一步。当协作规范要求多角色并行审批时使用 \"parallel\"。\n" +
	"- activities: 需要创建的活动列表。\n" +
	"- participant_type: \"user\" 需填 participant_id；\"position_department\" 需填 position_code + department_code。\n" +
	"- position_code / department_code: 当 participant_type 为 position_department 时，填写岗位编码和部门编码。\n" +
	"- action_id: 当 type 为 \"action\" 时，填写 decision.list_actions 返回的 action id。\n" +
	"- reasoning: 你的推理过程（会展示给管理员审核）。\n" +
	"- confidence: 决策信心（0.0-1.0）。越高表示越确信。"

// extractWorkflowHints extracts a structured step summary from WorkflowJSON
// for injection into the system prompt in direct_first mode.
func extractWorkflowHints(workflowJSON string) string {
	if workflowJSON == "" {
		return ""
	}

	var wf struct {
		Nodes []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Data struct {
				Label        string `json:"label"`
				Participants []struct {
					Type           string `json:"type"`
					Value          string `json:"value"`
					PositionCode   string `json:"position_code"`
					DepartmentCode string `json:"department_code"`
				} `json:"participants"`
				ApproveMode      string `json:"approve_mode"`
				GatewayDirection string `json:"gateway_direction"`
				ActionID         *uint  `json:"action_id"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
			Target string `json:"target"`
			Data   struct {
				Outcome   string `json:"outcome"`
				Default   bool   `json:"default"`
				Condition *struct {
					Field    string `json:"field"`
					Operator string `json:"operator"`
					Value    any    `json:"value"`
				} `json:"condition"`
			} `json:"data"`
		} `json:"edges"`
	}

	if err := json.Unmarshal([]byte(workflowJSON), &wf); err != nil {
		return ""
	}

	if len(wf.Nodes) == 0 {
		return ""
	}

	// Build node map and adjacency list
	nodeMap := make(map[string]int) // id → index
	for i, n := range wf.Nodes {
		nodeMap[n.ID] = i
	}

	outEdges := make(map[string][]int) // source → edge indices
	for i, e := range wf.Edges {
		outEdges[e.Source] = append(outEdges[e.Source], i)
	}

	// Walk from start node to build hints
	var startID string
	for _, n := range wf.Nodes {
		if n.Type == "start" {
			startID = n.ID
			break
		}
	}
	if startID == "" {
		return ""
	}

	var hints []string
	step := 1
	visited := make(map[string]bool)
	queue := []string{startID}

	for len(queue) > 0 && step <= 20 { // cap at 20 steps
		nodeID := queue[0]
		queue = queue[1:]

		if visited[nodeID] {
			continue
		}
		visited[nodeID] = true

		idx, ok := nodeMap[nodeID]
		if !ok {
			continue
		}
		node := wf.Nodes[idx]

		// Skip start/end nodes in the hints
		if node.Type == "start" {
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
			continue
		}
		if node.Type == "end" {
			hints = append(hints, fmt.Sprintf("%d. **结束流程**", step))
			step++
			continue
		}

		// Build step description
		label := node.Data.Label
		if label == "" {
			label = node.Type
		}

		var desc string
		switch node.Type {
		case "exclusive":
			desc = fmt.Sprintf("%d. **条件分支** [%s]", step, label)
			for _, ei := range outEdges[nodeID] {
				edge := wf.Edges[ei]
				if edge.Data.Condition != nil {
					desc += fmt.Sprintf("\n   - 当 %s %s %v → ", edge.Data.Condition.Field, edge.Data.Condition.Operator, edge.Data.Condition.Value)
					if ti, ok := nodeMap[edge.Target]; ok {
						tl := wf.Nodes[ti].Data.Label
						if tl == "" {
							tl = wf.Nodes[ti].Type
						}
						desc += tl
					}
				} else if edge.Data.Default {
					desc += "\n   - 默认 → "
					if ti, ok := nodeMap[edge.Target]; ok {
						tl := wf.Nodes[ti].Data.Label
						if tl == "" {
							tl = wf.Nodes[ti].Type
						}
						desc += tl
					}
				}
				queue = append(queue, edge.Target)
			}
		case "parallel", "inclusive":
			if node.Data.GatewayDirection == "fork" {
				desc = fmt.Sprintf("%d. **并行处理** [%s]（以下步骤同时进行）", step, label)
			} else {
				desc = fmt.Sprintf("%d. **汇聚等待** [%s]（等待所有并行分支完成）", step, label)
			}
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "approve":
			participant := describeParticipants(node.Data.Participants)
			mode := ""
			if node.Data.ApproveMode == "parallel" {
				mode = "（并签）"
			}
			desc = fmt.Sprintf("%d. **审批** [%s] — %s%s", step, label, participant, mode)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "process":
			participant := describeParticipants(node.Data.Participants)
			desc = fmt.Sprintf("%d. **处理** [%s] — %s", step, label, participant)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "action":
			desc = fmt.Sprintf("%d. **自动动作** [%s]", step, label)
			if node.Data.ActionID != nil {
				desc += fmt.Sprintf("（action_id: %d）", *node.Data.ActionID)
			}
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "form":
			participant := describeParticipants(node.Data.Participants)
			desc = fmt.Sprintf("%d. **表单采集** [%s] — %s", step, label, participant)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "notify":
			desc = fmt.Sprintf("%d. **通知** [%s]", step, label)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "wait":
			desc = fmt.Sprintf("%d. **等待** [%s]", step, label)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		default:
			desc = fmt.Sprintf("%d. **%s** [%s]", step, node.Type, label)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		}

		hints = append(hints, desc)
		step++
	}

	if len(hints) == 0 {
		return ""
	}

	return strings.Join(hints, "\n")
}

// describeParticipants returns a human-readable description of participant assignments.
func describeParticipants(participants []struct {
	Type           string `json:"type"`
	Value          string `json:"value"`
	PositionCode   string `json:"position_code"`
	DepartmentCode string `json:"department_code"`
}) string {
	if len(participants) == 0 {
		return "待指定"
	}

	var parts []string
	for _, p := range participants {
		switch p.Type {
		case "user":
			parts = append(parts, fmt.Sprintf("用户(%s)", p.Value))
		case "position":
			parts = append(parts, fmt.Sprintf("岗位(%s)", p.Value))
		case "department":
			parts = append(parts, fmt.Sprintf("部门(%s)", p.Value))
		case "position_department":
			parts = append(parts, fmt.Sprintf("岗位(%s)@部门(%s)", p.PositionCode, p.DepartmentCode))
		case "requester_manager":
			parts = append(parts, "申请人主管")
		default:
			parts = append(parts, p.Type)
		}
	}
	return strings.Join(parts, ", ")
}

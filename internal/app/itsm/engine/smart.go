package engine

import (
	"context"
	"encoding/json"
	"errors"
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
	// AuditLevel returns how much AI reasoning is written to the ticket timeline.
	AuditLevel() string
	// SLACriticalThresholdSeconds returns the SLA critical urgency threshold in seconds (default 1800).
	SLACriticalThresholdSeconds() int
	// SLAWarningThresholdSeconds returns the SLA warning urgency threshold in seconds (default 3600).
	SLAWarningThresholdSeconds() int
	// SimilarHistoryLimit returns the max number of similar history records (default 5).
	SimilarHistoryLimit() int
	// ParallelConvergenceTimeout returns the max duration for parallel group convergence (default 72h).
	ParallelConvergenceTimeout() time.Duration
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
	NodeID          string `json:"node_id,omitempty"`
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
	db                *gorm.DB
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

func (e *SmartEngine) SetDB(db *gorm.DB) {
	e.db = db
}

func (e *SmartEngine) DispatchDecisionAsync(ticketID uint, completedActivityID *uint, triggerReason string) {
	if e == nil || e.db == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("direct decision dispatch panic", "ticketID", ticketID, "panic", r)
				_ = e.db.Session(&gorm.Session{NewDB: true}).Create(&timelineModel{
					TicketID:   ticketID,
					OperatorID: 0,
					EventType:  "ai_decision_failed",
					Message:    fmt.Sprintf("AI 决策异常: %v", r),
				}).Error
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		db := e.db.Session(&gorm.Session{NewDB: true}).WithContext(ctx)
		slog.Info("direct decision dispatch: starting", "ticketID", ticketID, "completedActivityID", completedActivityID, "triggerReason", triggerReason)
		err := e.RunDecisionCycleForTicket(ctx, db, ticketID, completedActivityID, triggerReason)
		if err != nil {
			if err == ErrAIDecisionFailed || err == ErrAIDisabled {
				slog.Warn("direct decision dispatch: handled decision error", "ticketID", ticketID, "error", err)
				return
			}
			slog.Error("direct decision dispatch: failed", "ticketID", ticketID, "error", err)
			_ = db.Create(&timelineModel{
				TicketID:   ticketID,
				OperatorID: 0,
				EventType:  "ai_decision_failed",
				Message:    fmt.Sprintf("AI 决策调度失败: %v", err),
			}).Error
			return
		}
		slog.Info("direct decision dispatch: completed", "ticketID", ticketID, "completedActivityID", completedActivityID)
	}()
}

// Start initialises the workflow for a smart-engine ticket.
func (e *SmartEngine) Start(ctx context.Context, tx *gorm.DB, params StartParams) error {
	if !e.IsAvailable() {
		return ErrSmartEngineUnavailable
	}

	if _, err := e.loadServiceForTicket(tx, params.TicketID); err != nil {
		return fmt.Errorf("load service: %w", err)
	}

	// Update ticket status to decisioning; the first decision cycle is dispatched after commit.
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).
		Update("status", TicketStatusDecisioning).Error; err != nil {
		return err
	}

	// Record timeline: workflow started
	e.recordTimeline(tx, params.TicketID, nil, params.RequesterID, "workflow_started", "智能流程已启动", "")

	return nil
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
	if _, _, err := completePendingAssignment(tx, e.resolver, activity.ID, params.OperatorID, params.Outcome, now, params.OperatorPositionIDs, params.OperatorDepartmentIDs, params.OperatorOrgScopeReady); err != nil {
		if errors.Is(err, ErrNoActiveAssignment) && activityBecameInactive(tx, params.ActivityID) {
			return ErrActivityNotActive
		}
		return err
	}

	updates := map[string]any{
		"status":             humanOrCompletedActivityStatus(activity.ActivityType, params.Outcome),
		"transition_outcome": params.Outcome,
		"finished_at":        now,
	}
	if params.Opinion != "" {
		updates["decision_reasoning"] = params.Opinion
	}
	if len(params.Result) > 0 {
		updates["form_data"] = string(params.Result)
	}
	if err := tx.Model(&activityModel{}).Where("id = ?", params.ActivityID).Updates(updates).Error; err != nil {
		return err
	}

	e.recordTimeline(tx, params.TicketID, &params.ActivityID, params.OperatorID, "activity_completed",
		humanProgressMessage(activity.Name, params.Outcome, params.Opinion), params.Opinion)

	// Load ticket for ensureContinuation
	var ticket ticketModel
	if err := tx.First(&ticket, params.TicketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	queued, err := e.ensureContinuation(tx, &ticket, params.ActivityID)
	if err != nil {
		return err
	}
	if queued {
		if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).
			Update("current_activity_id", nil).Error; err != nil {
			return err
		}
	}
	return nil
}

// Cancel terminates all active activities and marks the ticket cancelled.
func (e *SmartEngine) Cancel(ctx context.Context, tx *gorm.DB, params CancelParams) error {
	// Cancel all active activities
	if err := tx.Model(&activityModel{}).
		Where("ticket_id = ? AND status IN ?", params.TicketID,
			[]string{ActivityPending, ActivityInProgress}).
		Updates(map[string]any{
			"status":      ActivityCancelled,
			"finished_at": time.Now(),
		}).Error; err != nil {
		return err
	}

	// Cancel all pending assignments
	if err := tx.Model(&assignmentModel{}).
		Where("ticket_id = ? AND status = ?", params.TicketID, "pending").
		Update("status", ActivityCancelled).Error; err != nil {
		return err
	}

	// Update ticket
	now := time.Now()
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).Updates(map[string]any{
		"status":      ticketCancelStatus(params.EventType),
		"outcome":     ticketCancelOutcome(params.EventType),
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
	if err := e.recordTimeline(tx, params.TicketID, nil, params.OperatorID, eventType, msg, ""); err != nil {
		return err
	}

	// Note: ensureContinuation gates on terminal status, so this is currently a no-op
	// for cancelled tickets. Included for completeness in case gate logic evolves.
	var ticket ticketModel
	if err := tx.First(&ticket, params.TicketID).Error; err == nil {
		e.ensureContinuation(tx, &ticket, 0)
	}

	return nil
}

// --- Decision cycle ---

// runDecisionCycle executes the full AI decision cycle for a ticket.
func (e *SmartEngine) runDecisionCycle(ctx context.Context, tx *gorm.DB, ticketID uint, completedActivityID *uint, svcInfo *serviceModel, triggerReason string) error {
	// Resolve agentID and decisionMode early for logging
	var agentID uint
	var decisionMode string
	if e.configProvider != nil {
		agentID = e.configProvider.DecisionAgentID()
		decisionMode = e.configProvider.DecisionMode()
	}
	slog.Info("decision-cycle: starting",
		"ticketID", ticketID, "triggerReason", triggerReason,
		"serviceID", svcInfo.ID, "agentID", agentID, "decisionMode", decisionMode)

	// Check if AI is disabled for this ticket
	var ticket ticketModel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	if ticket.AIFailureCount >= MaxAIFailureCount {
		slog.Warn("decision-cycle: skipped", "ticketID", ticketID, "reason", "ai_disabled", "failureCount", ticket.AIFailureCount)
		e.recordTimeline(tx, ticketID, nil, 0, "ai_disabled",
			fmt.Sprintf("AI 决策已停用（连续 %d 次失败），请手动处理", ticket.AIFailureCount), "")
		return ErrAIDisabled
	}

	cfg := ParseSmartServiceConfig(svcInfo.AgentConfig)

	// Check terminal state
	if IsTerminalTicketStatus(ticket.Status) {
		slog.Info("decision-cycle: skipped", "ticketID", ticketID, "reason", "terminal_state", "status", ticket.Status)
		return nil
	}

	var activeCount int64
	if err := tx.Model(&activityModel{}).
		Where("ticket_id = ? AND status IN ?", ticketID, []string{ActivityPending, ActivityInProgress}).
		Count(&activeCount).Error; err != nil {
		return err
	}
	if activeCount > 0 {
		slog.Info("decision-cycle: skipped",
			"ticketID", ticketID, "reason", "active_activities", "activeCount", activeCount)
		return nil
	}

	// Call agentic decision with timeout
	timeout := time.Duration(cfg.DecisionTimeoutSeconds) * time.Second
	decisionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	plan, err := e.agenticDecision(decisionCtx, tx, ticketID, completedActivityID, svcInfo, triggerReason)
	if err != nil {
		reason := fmt.Sprintf("AI 决策失败: %v", err)
		if decisionCtx.Err() == context.DeadlineExceeded {
			reason = fmt.Sprintf("AI 决策超时（%ds）", cfg.DecisionTimeoutSeconds)
		}
		return e.handleDecisionFailure(tx, ticketID, reason)
	}

	// Log decision plan summary
	slog.Info("decision-cycle: plan",
		"ticketID", ticketID, "nextStepType", plan.NextStepType,
		"confidence", plan.Confidence, "activityCount", len(plan.Activities),
		"executionMode", plan.ExecutionMode)

	if err := e.applyDeterministicServiceGuards(ctx, tx, ticketID, plan, svcInfo); err != nil {
		slog.Warn("decision-cycle: service-guard-failed", "ticketID", ticketID, "error", err.Error())
		return e.handleDecisionFailure(tx, ticketID, fmt.Sprintf("AI 决策服务护栏失败: %v", err))
	}

	// Validate decision plan
	if err := e.validateDecisionPlan(tx, ticketID, plan, svcInfo, completedActivityID); err != nil {
		slog.Warn("decision-cycle: validation-failed", "ticketID", ticketID, "error", err.Error())
		return e.handleDecisionFailure(tx, ticketID, fmt.Sprintf("AI 决策校验失败: %v", err))
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
	return e.pendManualHandlingPlan(tx, ticketID, plan)
}

// handleDecisionFailure increments failure count and records timeline.
func (e *SmartEngine) handleDecisionFailure(tx *gorm.DB, ticketID uint, reason string) error {
	tx.Model(&ticketModel{}).Where("id = ?", ticketID).
		UpdateColumn("ai_failure_count", gorm.Expr("ai_failure_count + 1"))

	details := decisionExplanationDetails(buildDecisionExplanationSnapshot(nil, "ai_decision_failed", reason, "等待人工介入或恢复动作", nil))
	e.recordTimelineWithDetails(tx, ticketID, nil, 0, "ai_decision_failed", reason, details, "")

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

func (e *SmartEngine) applyDeterministicServiceGuards(ctx context.Context, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) error {
	if plan == nil || svc == nil {
		return nil
	}
	for _, policy := range builtInSmartDecisionPolicies() {
		applied, err := policy.Apply(ctx, e, tx, ticketID, plan, svc)
		if err != nil {
			return err
		}
		if applied {
			return nil
		}
	}
	return nil
}

func looksLikeDBBackupWhitelistSpec(spec string) bool {
	if strings.Contains(spec, "数据库备份") &&
		strings.Contains(spec, "白名单") &&
		strings.Contains(spec, "放行") &&
		(strings.Contains(spec, "数据库管理员") || strings.Contains(spec, "db_admin")) {
		return true
	}
	return strings.Contains(spec, "db_admin") &&
		strings.Contains(spec, "precheck") &&
		strings.Contains(spec, "apply") &&
		strings.Contains(spec, "decision.execute_action")
}

func isDBBackupWhitelistActionCode(code string) bool {
	return code == "db_backup_whitelist_precheck" || code == "db_backup_whitelist_apply"
}

func looksLikeBossSerialChangeSpec(spec string) bool {
	if strings.Contains(spec, "高风险变更协同申请") &&
		strings.Contains(spec, "总部处理人") &&
		strings.Contains(spec, "运维管理员") {
		return true
	}
	return strings.Contains(spec, "高风险变更协同申请") &&
		strings.Contains(spec, "headquarters") &&
		strings.Contains(spec, "serial_reviewer") &&
		strings.Contains(spec, "ops_admin")
}

func (e *SmartEngine) applyBossSerialChangeGuard(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	headDone, err := ticketHasSatisfiedDepartmentPositionProcess(tx, ticketID, "headquarters", "serial_reviewer")
	if err != nil {
		return err
	}
	opsDone, err := ticketHasSatisfiedDepartmentPositionProcess(tx, ticketID, "it", "ops_admin")
	if err != nil {
		return err
	}

	switch {
	case !headDone:
		forceSingleDepartmentPositionProcessPlan(plan, "headquarters", "serial_reviewer", "协作规范要求先由总部处理人岗位处理，workflow_json 仅作辅助背景，不得跳过首级岗位或使用旧固定用户。")
	case !opsDone:
		forceSingleDepartmentPositionProcessPlan(plan, "it", "ops_admin", "总部处理人已完成，协作规范要求再由信息部运维管理员岗位处理。")
	default:
		forceCompletePlan(plan, "总部处理人和信息部运维管理员均已完成，按协作规范立即结束流程。")
	}
	return nil
}

func (e *SmartEngine) applyDBBackupWhitelistGuard(ctx context.Context, tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) error {
	if err := validateDBBackupWhitelistFormData(tx, ticketID); err != nil {
		return err
	}

	precheckDone, err := ticketActionSucceeded(tx, ticketID, "db_backup_whitelist_precheck")
	if err != nil {
		return err
	}
	applyDone, err := ticketActionSucceeded(tx, ticketID, "db_backup_whitelist_apply")
	if err != nil {
		return err
	}
	dbaDone, err := ticketHasSatisfiedPositionProcess(tx, ticketID, "db_admin")
	if err != nil {
		return err
	}

	switch {
	case !precheckDone:
		if err := e.executeServiceActionOnce(ctx, tx, ticketID, svc, "db_backup_whitelist_precheck"); err != nil {
			return err
		}
		forceSinglePositionProcessPlan(plan, "db_admin", "预检动作已执行成功，按协作规范交给数据库管理员处理。")
	case !dbaDone:
		forceSinglePositionProcessPlan(plan, "db_admin", "预检已完成，协作规范要求先由数据库管理员处理。")
	case !applyDone:
		if err := e.executeServiceActionOnce(ctx, tx, ticketID, svc, "db_backup_whitelist_apply"); err != nil {
			return err
		}
		forceCompletePlan(plan, "数据库管理员处理已完成且放行动作已执行成功，按协作规范结束流程。")
	default:
		forceCompletePlan(plan, "预检、数据库管理员处理和放行动作均已完成，按协作规范结束流程。")
	}
	return nil
}

func validateDBBackupWhitelistFormData(tx *gorm.DB, ticketID uint) error {
	var ticket ticketModel
	if err := tx.Select("id, form_data").First(&ticket, ticketID).Error; err != nil {
		return fmt.Errorf("读取数据库备份白名单申请失败: %w", err)
	}
	return validateDBBackupWhitelistFormJSON(ticket.FormData)
}

func validateDBBackupWhitelistFormJSON(rawFormData string) error {
	var formData map[string]any
	if err := json.Unmarshal([]byte(rawFormData), &formData); err != nil {
		return fmt.Errorf("数据库备份白名单申请表单不是有效 JSON: %w", err)
	}

	required := map[string]string{
		"database_name":    "目标数据库",
		"source_ip":        "来源 IP",
		"whitelist_window": "放行时间窗",
		"access_reason":    "申请原因",
	}
	for key, label := range required {
		value := strings.TrimSpace(fmt.Sprint(formData[key]))
		if value == "" || value == "<nil>" || strings.Contains(value, "{{ticket.form_data.") {
			return fmt.Errorf("数据库备份白名单申请缺少%s；不得触发预检或放行动作", label)
		}
	}

	window := strings.TrimSpace(fmt.Sprint(formData["whitelist_window"]))
	if !isConcreteWhitelistWindow(window) {
		return fmt.Errorf("数据库备份白名单放行时间窗不明确；必须包含明确的开始和结束时刻，不得用“明天晚上/今晚/维护窗口”等模糊时段触发预检或放行")
	}

	return nil
}

func isConcreteWhitelistWindow(window string) bool {
	window = strings.TrimSpace(window)
	if window == "" || strings.Contains(window, "{{ticket.form_data.") {
		return false
	}
	clockCount := 0
	for _, part := range strings.FieldsFunc(window, func(r rune) bool {
		return r == ' ' || r == '~' || r == '～' || r == '-' || r == '到' || r == '至'
	}) {
		if strings.Contains(part, ":") || strings.Contains(part, "点") || strings.Contains(part, "时") {
			clockCount++
		}
	}
	return clockCount >= 2
}

func (e *SmartEngine) applySingleHumanRouteGuard(tx *gorm.DB, ticketID uint, plan *DecisionPlan, expectedPosition string, reason string) error {
	completed, err := ticketHasCompletedPositionProcess(tx, ticketID, expectedPosition)
	if err != nil {
		return err
	}
	if completed {
		forceCompletePlan(plan, fmt.Sprintf("%s，且岗位 %s 的人工处理已完成，按协作规范结束流程。", reason, expectedPosition))
		return nil
	}
	forceSinglePositionProcessPlan(plan, expectedPosition, fmt.Sprintf("%s，按协作规范交给 %s 处理。", reason, expectedPosition))
	return nil
}

func forceSinglePositionProcessPlan(plan *DecisionPlan, positionCode string, reasoning string) {
	forceSingleDepartmentPositionProcessPlan(plan, "it", positionCode, reasoning)
}

func forceSingleDepartmentPositionProcessPlan(plan *DecisionPlan, departmentCode, positionCode string, reasoning string) {
	plan.NextStepType = NodeProcess
	plan.ExecutionMode = "single"
	plan.Activities = []DecisionActivity{{
		Type:            NodeProcess,
		ParticipantType: "position_department",
		DepartmentCode:  departmentCode,
		PositionCode:    positionCode,
		Instructions:    reasoning,
	}}
	plan.Reasoning = appendDecisionReasoning(plan.Reasoning, reasoning)
	if plan.Confidence < DefaultConfidenceThreshold {
		plan.Confidence = DefaultConfidenceThreshold
	}
}

func forceCompletePlan(plan *DecisionPlan, reasoning string) {
	plan.NextStepType = "complete"
	plan.ExecutionMode = "single"
	plan.Activities = nil
	plan.Reasoning = appendDecisionReasoning(plan.Reasoning, reasoning)
	if plan.Confidence < DefaultConfidenceThreshold {
		plan.Confidence = DefaultConfidenceThreshold
	}
}

func appendDecisionReasoning(existing string, addition string) string {
	existing = strings.TrimSpace(existing)
	addition = strings.TrimSpace(addition)
	if existing == "" {
		return addition
	}
	if addition == "" || strings.Contains(existing, addition) {
		return existing
	}
	return existing + "\n" + addition
}

func ticketActionSucceeded(tx *gorm.DB, ticketID uint, actionCode string) (bool, error) {
	var count int64
	err := tx.Table("itsm_ticket_action_executions").
		Joins("JOIN itsm_service_actions ON itsm_service_actions.id = itsm_ticket_action_executions.service_action_id").
		Where("itsm_ticket_action_executions.ticket_id = ? AND itsm_service_actions.code IN ? AND itsm_ticket_action_executions.status = ?",
			ticketID, actionCodeAliases(actionCode), "success").
		Count(&count).Error
	return count > 0, err
}

func actionCodeAliases(actionCode string) []string {
	switch actionCode {
	case "db_backup_whitelist_precheck", "backup_whitelist_precheck":
		return []string{"db_backup_whitelist_precheck", "backup_whitelist_precheck"}
	case "db_backup_whitelist_apply", "backup_whitelist_apply":
		return []string{"db_backup_whitelist_apply", "backup_whitelist_apply"}
	default:
		return []string{actionCode}
	}
}

func ticketHasCompletedPositionProcess(tx *gorm.DB, ticketID uint, positionCode string) (bool, error) {
	var count int64
	err := tx.Table("itsm_ticket_activities").
		Joins("JOIN itsm_ticket_assignments ON itsm_ticket_assignments.activity_id = itsm_ticket_activities.id").
		Joins("JOIN positions ON positions.id = itsm_ticket_assignments.position_id").
		Joins("JOIN departments ON departments.id = itsm_ticket_assignments.department_id").
		Where("itsm_ticket_activities.ticket_id = ? AND itsm_ticket_activities.activity_type = ? AND itsm_ticket_activities.status IN ? AND positions.code = ? AND departments.code = ?",
			ticketID, NodeProcess, CompletedActivityStatuses(), positionCode, "it").
		Count(&count).Error
	return count > 0, err
}

func ticketHasSatisfiedPositionProcess(tx *gorm.DB, ticketID uint, positionCode string) (bool, error) {
	return ticketHasSatisfiedDepartmentPositionProcess(tx, ticketID, "it", positionCode)
}

func ticketHasSatisfiedDepartmentPositionProcess(tx *gorm.DB, ticketID uint, departmentCode, positionCode string) (bool, error) {
	var rows []struct {
		TransitionOutcome string
	}
	err := tx.Table("itsm_ticket_activities").
		Joins("JOIN itsm_ticket_assignments ON itsm_ticket_assignments.activity_id = itsm_ticket_activities.id").
		Joins("JOIN positions ON positions.id = itsm_ticket_assignments.position_id").
		Joins("JOIN departments ON departments.id = itsm_ticket_assignments.department_id").
		Where("itsm_ticket_activities.ticket_id = ? AND itsm_ticket_activities.activity_type = ? AND itsm_ticket_activities.status IN ? AND positions.code = ? AND departments.code = ?",
			ticketID, NodeProcess, CompletedActivityStatuses(), positionCode, departmentCode).
		Select("itsm_ticket_activities.transition_outcome").
		Find(&rows).Error
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if isPositiveActivityOutcome(row.TransitionOutcome) {
			return true, nil
		}
	}
	return false, nil
}

func (e *SmartEngine) executeServiceActionOnce(ctx context.Context, tx *gorm.DB, ticketID uint, svc *serviceModel, actionCode string) error {
	done, err := ticketActionSucceeded(tx, ticketID, actionCode)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	if e.actionExecutor == nil {
		return fmt.Errorf("动作执行器不可用，无法执行 %s", actionCode)
	}

	var action serviceActionModel
	if svc != nil && svc.ActionsJSON != "" {
		found, err := findSnapshotServiceActionByCodeAliases(svc, actionCodeAliases(actionCode), &action)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("服务动作 %s 不存在或未启用", actionCode)
		}
	} else {
		if svc == nil {
			return fmt.Errorf("服务定义不可用，无法执行 %s", actionCode)
		}
		if err := findActiveServiceActionByCodeAliases(tx, svc.ID, actionCodeAliases(actionCode), &action); err != nil {
			return fmt.Errorf("服务动作 %s 不存在或未启用: %w", actionCode, err)
		}
	}

	execCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if err := e.actionExecutor.ExecuteWithConfig(execCtx, ticketID, 0, action.ID, action.ActionType, action.ConfigJSON); err != nil {
		return fmt.Errorf("执行服务动作 %s 失败: %w", actionCode, err)
	}
	e.recordTimeline(tx, ticketID, nil, 0, "ai_decision_action_executed",
		fmt.Sprintf("AI 决策服务护栏已执行动作：%s", action.Name), "")
	return nil
}

func findActiveServiceActionByCodeAliases(tx *gorm.DB, serviceID uint, codes []string, action *serviceActionModel) error {
	var lastErr error
	for _, code := range codes {
		err := tx.Table("itsm_service_actions").
			Where("service_id = ? AND code = ? AND is_active = ? AND deleted_at IS NULL", serviceID, code, true).
			Select("id, name, code, service_id, is_active, config_json").
			First(action).Error
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func findSnapshotServiceActionByCodeAliases(svc *serviceModel, codes []string, action *serviceActionModel) (bool, error) {
	if svc == nil || svc.ActionsJSON == "" {
		return false, nil
	}
	rows, err := ParseServiceActionSnapshotRows(svc.ActionsJSON)
	if err != nil {
		return false, err
	}
	for _, code := range codes {
		for _, row := range rows {
			if row.Code == code && row.IsActive {
				*action = serviceActionModel{
					ID:         row.ID,
					Name:       row.Name,
					Code:       row.Code,
					ServiceID:  svc.ID,
					IsActive:   row.IsActive,
					ActionType: row.ActionType,
					ConfigJSON: row.ConfigJSON,
				}
				return true, nil
			}
		}
	}
	return false, nil
}

// handleComplete finishes the ticket when agent decides to complete.
func (e *SmartEngine) handleComplete(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	now := time.Now()

	// Find the end node ID from workflow_json
	var endNodeID string
	if svc, err := e.loadServiceForTicket(tx, ticketID); err == nil && svc.WorkflowJSON != "" {
		if def, err := ParseWorkflowDef(json.RawMessage(svc.WorkflowJSON)); err == nil {
			for _, n := range def.Nodes {
				if n.Type == NodeEnd {
					endNodeID = n.ID
					break
				}
			}
		}
	}

	// Create a completed end activity
	act := &activityModel{
		TicketID:          ticketID,
		Name:              "流程完结",
		ActivityType:      "complete",
		NodeID:            endNodeID,
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

	status, outcome := e.resolveCompletionStatus(tx, ticketID)
	if err := tx.Model(&ticketModel{}).Where("id = ?", ticketID).Updates(map[string]any{
		"status":              status,
		"outcome":             outcome,
		"finished_at":         now,
		"current_activity_id": act.ID,
	}).Error; err != nil {
		return err
	}

	slog.Info("decision-cycle: completed", "ticketID", ticketID)
	details := decisionExplanationDetails(buildDecisionExplanationSnapshot(plan, "workflow_completed", "智能流程已完结", completionDecisionLabel(status, outcome), &act.ID))
	return e.recordTimelineWithDetails(tx, ticketID, &act.ID, 0, "workflow_completed",
		"智能流程已完结", details, plan.Reasoning)
}

func (e *SmartEngine) resolveCompletionStatus(tx *gorm.DB, ticketID uint) (string, string) {
	var activity activityModel
	err := tx.Where("ticket_id = ? AND activity_type IN ? AND transition_outcome IN ?", ticketID,
		[]string{NodeApprove, NodeForm, NodeProcess}, []string{ActivityApproved, ActivityRejected}).
		Order("finished_at DESC, id DESC").
		First(&activity).Error
	if err == nil && activity.TransitionOutcome == ActivityRejected {
		return TicketStatusRejected, TicketOutcomeRejected
	}
	if err == nil && activity.TransitionOutcome == ActivityApproved {
		return TicketStatusCompleted, TicketOutcomeApproved
	}
	return TicketStatusCompleted, TicketOutcomeFulfilled
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
			NodeID:          da.NodeID,
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
		} else if da.ParticipantType == "requester" {
			e.createRequesterAssignment(tx, ticketID, act.ID)
		} else if da.ParticipantType == "position_department" && da.PositionCode != "" && da.DepartmentCode != "" {
			e.createPositionAssignment(tx, ticketID, act.ID, da.PositionCode, da.DepartmentCode)
		} else if da.Type == "process" || da.Type == "form" {
			e.tryFallbackAssignment(tx, ticketID, act.ID)
		}

		if i == 0 {
			tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("status", ticketStatusForDecisionActivity(da.Type))
		}

		// Submit action task if applicable
		if da.Type == "action" && da.ActionID != nil && e.scheduler != nil {
			payload, _ := json.Marshal(map[string]any{
				"ticket_id":   ticketID,
				"activity_id": act.ID,
				"action_id":   *da.ActionID,
			})
			if err := submitTaskInTx(e.scheduler, tx, "itsm-action-execute", payload); err != nil {
				slog.Error("failed to submit action task", "error", err, "ticketID", ticketID)
			}
		}

		msg := fmt.Sprintf("AI 并行处理活动：%s（组 %s，信心 %.0f%%）", decisionActivityName(da), groupID[:8], plan.Confidence*100)
		details := decisionExplanationDetails(buildDecisionExplanationSnapshot(plan, "ai_decision_executed", msg, decisionActivityName(da), &act.ID))
		e.recordTimelineWithDetails(tx, ticketID, &act.ID, 0, "ai_decision_executed", msg, details, plan.Reasoning)
	}

	_ = firstActID
	slog.Info("decision-cycle: executed",
		"ticketID", ticketID, "activityCount", len(plan.Activities),
		"executionMode", "parallel", "groupID", groupID)
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
	if da.Type == "form" || da.Type == "process" {
		status = ActivityPending
	}

	act := &activityModel{
		TicketID:     ticketID,
		Name:         decisionActivityName(da),
		ActivityType: da.Type,
		Status:       status,
		NodeID:       da.NodeID,
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
		"status":              ticketStatusForDecisionActivity(da.Type),
		"outcome":             "",
	}).Error; err != nil {
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
	} else if da.ParticipantType == "requester" {
		e.createRequesterAssignment(tx, ticketID, act.ID)
	} else if da.ParticipantType == "position_department" && da.PositionCode != "" && da.DepartmentCode != "" {
		e.createPositionAssignment(tx, ticketID, act.ID, da.PositionCode, da.DepartmentCode)
	} else if da.Type == NodeApprove || da.Type == NodeProcess || da.Type == NodeForm {
		e.tryFallbackAssignment(tx, ticketID, act.ID)
	}

	if da.Type == "action" && da.ActionID != nil && e.scheduler != nil {
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":   ticketID,
			"activity_id": act.ID,
			"action_id":   *da.ActionID,
		})
		if err := submitTaskInTx(e.scheduler, tx, "itsm-action-execute", payload); err != nil {
			slog.Error("failed to submit action task", "error", err, "ticketID", ticketID)
		}
	}

	msg := fmt.Sprintf("AI 自动执行：%s（信心 %.0f%%）", decisionActivityName(da), plan.Confidence*100)
	details := decisionExplanationDetails(buildDecisionExplanationSnapshot(plan, "ai_decision_executed", msg, decisionActivityName(da), &act.ID))
	e.recordTimelineWithDetails(tx, ticketID, &act.ID, 0, "ai_decision_executed", msg, details, plan.Reasoning)

	var assigneeID uint
	if da.ParticipantID != nil {
		assigneeID = *da.ParticipantID
	}
	slog.Info("decision-cycle: executed",
		"ticketID", ticketID, "activityType", da.Type, "activityID", act.ID,
		"assigneeID", assigneeID, "executionMode", "single")

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
			fmt.Sprintf("兜底处理人无效（ID=%d），请检查引擎设置", fallbackID), "")
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
	// Look up position and department IDs (best-effort, tables may not exist in tests)
	var positionID, departmentID uint
	tx.Table("positions").Where("code = ?", positionCode).Select("id").Scan(&positionID)
	tx.Table("departments").Where("code = ?", departmentCode).Select("id").Scan(&departmentID)
	userIDs, err := resolveUsersByPositionDepartmentInTx(tx, positionID, departmentID)
	if err != nil {
		slog.Warn("position assignment: failed to resolve users in workflow transaction",
			"positionCode", positionCode, "departmentCode", departmentCode, "error", err)
	}
	if len(userIDs) == 0 {
		slog.Warn("position assignment: no users found", "positionCode", positionCode, "departmentCode", departmentCode)
		e.recordTimeline(tx, ticketID, &activityID, 0, "participant_fallback_warning",
			fmt.Sprintf("岗位参与人 %s@%s 当前没有可用处理人，工单未 fallback 到其他岗位，等待 IT 管理员补充人员配置", positionCode, departmentCode), "")
	}

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

func resolveUsersByPositionDepartmentInTx(tx *gorm.DB, positionID, departmentID uint) ([]uint, error) {
	if positionID == 0 || departmentID == 0 {
		return nil, nil
	}
	var userIDs []uint
	err := tx.Table("user_positions").
		Joins("JOIN users ON users.id = user_positions.user_id").
		Where("user_positions.position_id = ? AND user_positions.department_id = ? AND user_positions.deleted_at IS NULL AND users.is_active = ?",
			positionID, departmentID, true).
		Pluck("DISTINCT users.id", &userIDs).Error
	return userIDs, err
}

func (e *SmartEngine) createRequesterAssignment(tx *gorm.DB, ticketID, activityID uint) {
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err != nil || ticket.RequesterID == 0 {
		e.tryFallbackAssignment(tx, ticketID, activityID)
		return
	}

	assignment := &assignmentModel{
		TicketID:        ticketID,
		ActivityID:      activityID,
		ParticipantType: "requester",
		UserID:          &ticket.RequesterID,
		AssigneeID:      &ticket.RequesterID,
		Status:          "pending",
		IsCurrent:       true,
	}
	if err := tx.Create(assignment).Error; err != nil {
		slog.Warn("requester assignment: failed to create assignment", "ticketID", ticketID, "activityID", activityID, "error", err)
		return
	}
	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", ticket.RequesterID)
}

// pendManualHandlingPlan creates a manual process activity for low-confidence decisions.
func (e *SmartEngine) pendManualHandlingPlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	slog.Info("decision-cycle: low-confidence", "ticketID", ticketID, "confidence", plan.Confidence)
	now := time.Now()
	planJSON := mustJSON(plan)

	name := "AI 低置信待处置"
	var nodeID string
	if len(plan.Activities) > 0 {
		name = fmt.Sprintf("AI 低置信待处置：%s", decisionActivityName(plan.Activities[0]))
		nodeID = plan.Activities[0].NodeID
	}

	act := &activityModel{
		TicketID:     ticketID,
		Name:         name,
		ActivityType: NodeProcess,
		NodeID:       nodeID,
		Status:       ActivityPending,
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
		"status":              TicketStatusWaitingHuman,
		"outcome":             "",
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
		case da.ParticipantType == "requester":
			e.createRequesterAssignment(tx, ticketID, act.ID)
		case da.ParticipantType == "position_department" && da.PositionCode != "" && da.DepartmentCode != "":
			e.createPositionAssignment(tx, ticketID, act.ID, da.PositionCode, da.DepartmentCode)
		default:
			e.tryFallbackAssignment(tx, ticketID, act.ID)
		}
	}

	msg := fmt.Sprintf("AI 决策信心不足（%.0f%%），等待人工处置", plan.Confidence*100)
	details := decisionExplanationDetails(buildDecisionExplanationSnapshot(plan, "ai_decision_pending", msg, "等待人工处置", &act.ID))
	e.recordTimelineWithDetails(tx, ticketID, &act.ID, 0, "ai_decision_pending", msg, details, plan.Reasoning)

	return nil
}

// --- Validation ---

func (e *SmartEngine) validateDecisionPlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel, completedActivityID *uint) error {
	if plan == nil {
		return fmt.Errorf("decision plan is nil")
	}

	// Check next_step_type is allowed
	if !AllowedSmartStepTypes[plan.NextStepType] {
		return fmt.Errorf("next_step_type %q 不合法", plan.NextStepType)
	}
	if plan.NextStepType == "complete" && len(plan.Activities) > 0 {
		return fmt.Errorf("complete 决策不能包含活动")
	}
	if plan.NextStepType != "complete" && len(plan.Activities) == 0 {
		return fmt.Errorf("next_step_type %q 必须包含至少一个活动", plan.NextStepType)
	}

	// Validate confidence is in range before any confidence-gated normalization.
	if plan.Confidence < 0 || plan.Confidence > 1 {
		return fmt.Errorf("confidence %.2f 不在 [0, 1] 范围内", plan.Confidence)
	}

	if err := e.validateAndNormalizeStructuredRoutingDecision(tx, ticketID, plan, svc); err != nil {
		return err
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
			if err := tx.Table("users").Where("id = ? AND deleted_at IS NULL", *a.ParticipantID).
				Select("is_active").First(&user).Error; err != nil {
				return fmt.Errorf("activities[%d].participant_id %d 用户不存在", i, *a.ParticipantID)
			}
			if !user.IsActive {
				return fmt.Errorf("activities[%d].participant_id %d 用户未激活", i, *a.ParticipantID)
			}
		}
		if isHumanActivityType(a.Type) {
			if !e.hasResolvableHumanParticipant(tx, a) {
				return fmt.Errorf("activities[%d] 缺少可解析的处理人", i)
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

		// Validate node_id against workflow_json if provided
		if a.NodeID != "" && svc.WorkflowJSON != "" {
			def, parseErr := ParseWorkflowDef(json.RawMessage(svc.WorkflowJSON))
			if parseErr == nil {
				nodeMap, _ := def.BuildMaps()
				if node, ok := nodeMap[a.NodeID]; ok {
					if node.Type != a.Type && !(a.Type == "approve" && node.Type == "process") {
						slog.Warn("decision plan node_id type mismatch, clearing",
							"activity_index", i, "node_id", a.NodeID,
							"node_type", node.Type, "activity_type", a.Type)
						plan.Activities[i].NodeID = ""
					}
				} else {
					slog.Warn("decision plan node_id not found in workflow_json, clearing",
						"activity_index", i, "node_id", a.NodeID)
					plan.Activities[i].NodeID = ""
				}
			}
		}
	}

	if err := e.validateRoutingConflictDecision(tx, ticketID, plan, svc); err != nil {
		return err
	}
	if err := e.validateRejectedRecoveryDecision(tx, ticketID, completedActivityID, plan, svc); err != nil {
		return err
	}
	if err := e.validateNoDuplicateCompletedHumanActivity(tx, ticketID, plan); err != nil {
		return err
	}

	return nil
}

func (e *SmartEngine) hasResolvableHumanParticipant(tx *gorm.DB, a DecisionActivity) bool {
	if a.ParticipantID != nil && *a.ParticipantID > 0 {
		return true
	}
	if a.ParticipantType == "position_department" && a.PositionCode != "" && a.DepartmentCode != "" {
		return true
	}
	if a.ParticipantType == "requester" {
		return true
	}
	return false
}

func (e *SmartEngine) validateRoutingConflictDecision(tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) error {
	if plan == nil || svc == nil || svc.WorkflowJSON == "" || plan.Confidence < DefaultConfidenceThreshold {
		return nil
	}
	if !planCreatesSingleRouteHumanWork(plan) {
		return nil
	}
	conflicts, err := detectTicketRoutingConflicts(tx, ticketID, svc.WorkflowJSON)
	if err != nil || len(conflicts) == 0 {
		return nil
	}
	return fmt.Errorf("表单路由字段存在跨分支冲突：%s；不得高置信选择单一路由，请先澄清或降级人工处置", strings.Join(conflicts, "；"))
}

func planCreatesSingleRouteHumanWork(plan *DecisionPlan) bool {
	for _, activity := range plan.Activities {
		if !isHumanActivityType(activity.Type) {
			continue
		}
		if activity.ParticipantType == "requester" {
			continue
		}
		return true
	}
	return false
}

func (e *SmartEngine) validateAndNormalizeStructuredRoutingDecision(tx *gorm.DB, ticketID uint, plan *DecisionPlan, svc *serviceModel) error {
	if plan == nil || svc == nil || plan.Confidence < DefaultConfidenceThreshold {
		return nil
	}
	if !planCreatesSingleRouteHumanWork(plan) {
		return nil
	}

	expectedPositions, ok, err := collaborationSpecRequestKindPositions(tx, ticketID, svc.CollaborationSpec)
	if err != nil || !ok {
		expectedPosition, ok, err := collaborationSpecAccessPurposePosition(tx, ticketID, svc.CollaborationSpec)
		if err != nil || !ok {
			return err
		}
		if expectedPosition == "" {
			return fmt.Errorf("form.access_reason/form.operation_purpose 缺失、为空或未命中协作规范定义的访问原因分支；不得高置信选择单一路由")
		}
		normalizePlanHumanParticipant(tx, plan, expectedPosition)
		return nil
	}
	if len(expectedPositions) == 0 {
		return fmt.Errorf("form.request_kind 缺失、为空或未命中协作规范定义的路由枚举；不得高置信选择网络或安全单一路由")
	}
	if len(expectedPositions) > 1 {
		return fmt.Errorf("form.request_kind 命中多个协作规范分支；不得高置信选择单一路由")
	}

	expectedPosition := ""
	for position := range expectedPositions {
		expectedPosition = position
	}
	normalizePlanHumanParticipant(tx, plan, expectedPosition)
	return nil
}

func normalizePlanHumanParticipant(tx *gorm.DB, plan *DecisionPlan, expectedPosition string) {
	for i := range plan.Activities {
		if !isHumanActivityType(plan.Activities[i].Type) || plan.Activities[i].ParticipantType == "requester" {
			continue
		}
		if decisionActivityTargetsPosition(tx, plan.Activities[i], expectedPosition) {
			continue
		}
		slog.Warn("decision plan route conflicts with collaboration spec, normalizing participant",
			"activity_index", i,
			"expected_position", expectedPosition,
			"actual_participant_type", plan.Activities[i].ParticipantType,
			"actual_position", plan.Activities[i].PositionCode)
		plan.Activities[i].ParticipantID = nil
		plan.Activities[i].ParticipantType = "position_department"
		plan.Activities[i].DepartmentCode = "it"
		plan.Activities[i].PositionCode = expectedPosition
		if !strings.Contains(plan.Reasoning, "协作规范") {
			plan.Reasoning = strings.TrimSpace(plan.Reasoning + "\n协作规范优先于 workflow_json：表单访问目的已命中协作规范岗位分支，已按协作规范校正处理岗位。")
		}
	}
}

func collaborationSpecRequestKindPositions(tx *gorm.DB, ticketID uint, spec string) (map[string]struct{}, bool, error) {
	if !looksLikeVPNRequestKindSpec(spec) {
		return nil, false, nil
	}

	var ticket struct {
		FormData string
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).Select("form_data").First(&ticket).Error; err != nil {
		return nil, true, err
	}

	var formData map[string]any
	if strings.TrimSpace(ticket.FormData) != "" {
		_ = json.Unmarshal([]byte(ticket.FormData), &formData)
	}
	values := conditionValues(formData["request_kind"])
	if len(values) == 0 {
		return map[string]struct{}{}, true, nil
	}

	positions := map[string]struct{}{}
	for _, value := range values {
		switch value {
		case "online_support", "troubleshooting", "production_emergency", "network_access_issue":
			positions["network_admin"] = struct{}{}
		case "external_collaboration", "long_term_remote_work", "cross_border_access", "security_compliance":
			positions["security_admin"] = struct{}{}
		default:
			return map[string]struct{}{}, true, nil
		}
	}
	return positions, true, nil
}

func looksLikeVPNRequestKindSpec(spec string) bool {
	return strings.Contains(spec, "form.request_kind") &&
		strings.Contains(spec, "network_admin") &&
		strings.Contains(spec, "security_admin") &&
		strings.Contains(spec, "online_support") &&
		strings.Contains(spec, "security_compliance")
}

func collaborationSpecAccessPurposePosition(tx *gorm.DB, ticketID uint, spec string) (string, bool, error) {
	if !looksLikeServerAccessPurposeSpec(spec) {
		return "", false, nil
	}

	var ticket struct {
		Title    string
		FormData string
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).
		Select("title, form_data").First(&ticket).Error; err != nil {
		return "", true, err
	}

	var formData map[string]any
	if strings.TrimSpace(ticket.FormData) != "" {
		_ = json.Unmarshal([]byte(ticket.FormData), &formData)
	}
	text := serverAccessRoutingText(formData)
	if text == "" {
		text = strings.TrimSpace(ticket.Title)
	}
	matches := serverAccessPurposeMatches(text)
	if len(matches) != 1 {
		return "", true, nil
	}
	return matches[0], true, nil
}

func looksLikeServerAccessPurposeSpec(spec string) bool {
	return strings.Contains(spec, "生产服务器临时访问") &&
		strings.Contains(spec, "访问") &&
		((strings.Contains(spec, "运维管理员") &&
			strings.Contains(spec, "网络管理员") &&
			strings.Contains(spec, "安全管理员")) ||
			(strings.Contains(spec, "运维管理员") &&
				strings.Contains(spec, "网络管理员") &&
				strings.Contains(spec, "信息安全管理员")) ||
			(strings.Contains(spec, "ops_admin") &&
				strings.Contains(spec, "network_admin") &&
				strings.Contains(spec, "security_admin")))
}

func serverAccessRoutingText(formData map[string]any) string {
	if len(formData) == 0 {
		return ""
	}
	var parts []string
	for _, key := range []string{"access_reason", "operation_purpose", "access_purpose"} {
		value := strings.TrimSpace(fmt.Sprint(formData[key]))
		if value == "" || value == "<nil>" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n")
}

func containsAnyText(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func serverAccessPurposeMatches(text string) []string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return nil
	}

	matches := make([]string, 0, 3)
	if containsAnyText(normalized,
		"安全审计", "取证", "证据", "保全", "漏洞", "入侵", "合规核查", "合规验证", "异常访问", "高敏访问", "安全取证", "安全核查",
	) {
		matches = append(matches, "security_admin")
	}
	if containsAnyText(normalized,
		"抓包", "链路", "acl", "负载均衡", "防火墙", "网络侧", "网络访问路径", "连通性",
	) {
		matches = append(matches, "network_admin")
	}
	if containsAnyText(normalized,
		"应用排障", "应用进程", "主机巡检", "日志查看", "查看应用日志", "进程处理", "磁盘清理", "运行状态", "一般生产运维",
	) {
		matches = append(matches, "ops_admin")
	}
	return matches
}

func decisionActivityTargetsPosition(tx *gorm.DB, da DecisionActivity, expectedPosition string) bool {
	if expectedPosition == "" {
		return false
	}
	if da.ParticipantType == "position_department" {
		return strings.EqualFold(strings.TrimSpace(da.PositionCode), expectedPosition)
	}
	if da.ParticipantID != nil && *da.ParticipantID > 0 {
		var count int64
		tx.Table("user_positions").
			Joins("JOIN positions ON positions.id = user_positions.position_id").
			Where("user_positions.user_id = ? AND positions.code = ? AND user_positions.deleted_at IS NULL", *da.ParticipantID, expectedPosition).
			Count(&count)
		return count > 0
	}
	return false
}

func detectTicketRoutingConflicts(tx *gorm.DB, ticketID uint, workflowJSON string) ([]string, error) {
	var ticket struct {
		FormData string
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).Select("form_data").First(&ticket).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(ticket.FormData) == "" {
		return nil, nil
	}

	var formData map[string]any
	if err := json.Unmarshal([]byte(ticket.FormData), &formData); err != nil {
		return nil, nil
	}

	def, err := ParseWorkflowDef(json.RawMessage(workflowJSON))
	if err != nil {
		return nil, nil
	}
	fieldRoutes := map[string]map[string]string{}
	for _, edge := range def.Edges {
		if edge.Data.Condition == nil {
			continue
		}
		collectConditionRoutes(*edge.Data.Condition, edge.Target, fieldRoutes)
	}

	var conflicts []string
	for field, valueRoutes := range fieldRoutes {
		formKey := strings.TrimPrefix(field, "form.")
		raw, ok := formData[formKey]
		if !ok {
			continue
		}
		targets := map[string]struct{}{}
		for _, value := range conditionValues(raw) {
			if target := valueRoutes[value]; target != "" {
				targets[target] = struct{}{}
			}
		}
		if len(targets) > 1 {
			conflicts = append(conflicts, fmt.Sprintf("%s 命中 %d 条分支", field, len(targets)))
		}
	}
	return conflicts, nil
}

func collectConditionRoutes(cond GatewayCondition, target string, routes map[string]map[string]string) {
	if cond.Field != "" && strings.HasPrefix(cond.Field, "form.") {
		values := conditionValues(cond.Value)
		if len(values) > 0 {
			if routes[cond.Field] == nil {
				routes[cond.Field] = map[string]string{}
			}
			for _, value := range values {
				routes[cond.Field][value] = target
			}
		}
	}
	for _, child := range cond.Conditions {
		collectConditionRoutes(child, target, routes)
	}
}

func conditionValues(raw any) []string {
	switch v := raw.(type) {
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				values = append(values, s)
			}
		}
		return values
	case []string:
		values := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				values = append(values, s)
			}
		}
		return values
	case string:
		if strings.Contains(v, ",") {
			parts := strings.Split(v, ",")
			values := make([]string, 0, len(parts))
			for _, part := range parts {
				if s := strings.TrimSpace(part); s != "" {
					values = append(values, s)
				}
			}
			return values
		}
		if s := strings.TrimSpace(v); s != "" {
			return []string{s}
		}
	}
	return nil
}

func (e *SmartEngine) validateNoDuplicateCompletedHumanActivity(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
	for i, da := range plan.Activities {
		if !isHumanActivityType(da.Type) {
			continue
		}

		var completed []activityModel
		if err := tx.Where("ticket_id = ? AND status IN ? AND activity_type = ?", ticketID, CompletedActivityStatuses(), da.Type).
			Order("id ASC").
			Find(&completed).Error; err != nil {
			return err
		}

		plannedName := decisionActivityName(da)
		plannedInstructions := strings.TrimSpace(da.Instructions)
		plannedSignatures := participantSignaturesForDecisionActivity(tx, da)
		data := NewDecisionDataStore(tx)

		for _, existing := range completed {
			if !sameActivityMeaning(existing, plannedName, plannedInstructions) {
				continue
			}
			assignments, err := data.GetActivityAssignments(existing.ID)
			if err != nil {
				return err
			}
			existingSignatures := participantSignaturesForAssignments(assignments)
			if len(plannedSignatures) == 0 || participantSignaturesOverlap(plannedSignatures, existingSignatures) {
				return fmt.Errorf("activities[%d] 重复创建已完成的人工活动：%s / %s", i, da.Type, plannedName)
			}
		}
	}
	return nil
}

func (e *SmartEngine) validateRejectedRecoveryDecision(tx *gorm.DB, ticketID uint, completedActivityID *uint, plan *DecisionPlan, svc *serviceModel) error {
	if completedActivityID == nil || *completedActivityID == 0 || plan == nil || plan.NextStepType == "complete" {
		return nil
	}

	var completed activityModel
	if err := tx.Where("ticket_id = ? AND id = ?", ticketID, *completedActivityID).First(&completed).Error; err != nil {
		return err
	}
	if !isHumanActivityType(completed.ActivityType) || isPositiveActivityOutcome(completed.TransitionOutcome) {
		return nil
	}
	if (rejectedRecoveryCreatesForm(plan) || rejectedRecoveryCreatesRequesterHumanWork(plan)) && !collaborationSpecAllowsRejectedFormRecovery(svc) {
		return fmt.Errorf("rejected 后试图创建申请人补充/返工活动，但协作规范未显式定义补充信息或返工路径；不得把驳回默认解释为退回申请人补充")
	}

	assignments, err := NewDecisionDataStore(tx).GetActivityAssignments(completed.ID)
	if err != nil {
		return err
	}
	existingSignatures := participantSignaturesForAssignments(assignments)

	for i, da := range plan.Activities {
		if !isHumanActivityType(da.Type) || da.Type != completed.ActivityType {
			continue
		}
		plannedSignatures := participantSignaturesForDecisionActivity(tx, da)
		if len(plannedSignatures) == 0 || !participantSignaturesOverlap(plannedSignatures, existingSignatures) {
			continue
		}
		if hasExplicitRecoveryIntent(da.Instructions) {
			continue
		}
		return fmt.Errorf("activities[%d] 试图重复创建刚被驳回的人工活动：%s；请基于驳回原因、协作规范和 workflow_json 选择规范明确允许的恢复路径、升级/转交、结束失败或明确不同的下一步", i, completed.Name)
	}

	return nil
}

func rejectedRecoveryCreatesForm(plan *DecisionPlan) bool {
	if plan == nil {
		return false
	}
	if plan.NextStepType == NodeForm {
		return true
	}
	for _, da := range plan.Activities {
		if da.Type == NodeForm {
			return true
		}
	}
	return false
}

func rejectedRecoveryCreatesRequesterHumanWork(plan *DecisionPlan) bool {
	if plan == nil {
		return false
	}
	for _, da := range plan.Activities {
		if !isHumanActivityType(da.Type) {
			continue
		}
		if da.ParticipantType == "requester" {
			return true
		}
	}
	return false
}

func collaborationSpecAllowsRejectedFormRecovery(svc *serviceModel) bool {
	if svc == nil {
		return false
	}
	spec := strings.TrimSpace(svc.CollaborationSpec)
	if spec == "" {
		return false
	}
	hasRejectedCue := strings.Contains(spec, "驳回") ||
		strings.Contains(spec, "拒绝") ||
		strings.Contains(strings.ToLower(spec), "reject")
	hasFormRecoveryCue := strings.Contains(spec, "补充") ||
		strings.Contains(spec, "返工") ||
		strings.Contains(spec, "退回申请人") ||
		strings.Contains(spec, "重新填写") ||
		strings.Contains(spec, "重填") ||
		strings.Contains(spec, "修改后提交")
	return hasRejectedCue && hasFormRecoveryCue
}

// --- Helpers ---

// loadServiceForTicket loads service definition info for a ticket.
func (e *SmartEngine) loadServiceForTicket(tx *gorm.DB, ticketID uint) (*serviceModel, error) {
	var ticket struct {
		ServiceID        uint
		ServiceVersionID *uint
	}
	selectColumns := "service_id"
	if tx.Migrator().HasColumn(&ticketModel{}, "service_version_id") {
		selectColumns = "service_id, service_version_id"
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).Select(selectColumns).First(&ticket).Error; err != nil {
		return nil, err
	}
	if ticket.ServiceVersionID != nil {
		var snapshot serviceModel
		err := tx.Table("itsm_service_definition_versions").
			Where("id = ? AND service_id = ?", *ticket.ServiceVersionID, ticket.ServiceID).
			Select("service_id AS id, id AS runtime_version_id, engine_type, collaboration_spec, agent_id, agent_config, knowledge_base_ids, workflow_json, actions_json").
			First(&snapshot).Error
		if err == nil {
			return &snapshot, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		slog.Warn("smart engine falling back to live service definition because service version snapshot is missing", "ticketID", ticketID, "serviceVersionID", *ticket.ServiceVersionID)
	} else {
		slog.Warn("smart engine falling back to live service definition because ticket has no service_version_id", "ticketID", ticketID)
	}

	var svc serviceModel
	err := tx.Table("itsm_service_definitions").
		Where("id = ?", ticket.ServiceID).
		Select("itsm_service_definitions.*").
		First(&svc).Error
	return &svc, err
}

func (e *SmartEngine) recordTimeline(tx *gorm.DB, ticketID uint, activityID *uint, operatorID uint, eventType, message, reasoning string) error {
	return e.recordTimelineWithDetails(tx, ticketID, activityID, operatorID, eventType, message, nil, reasoning)
}

func (e *SmartEngine) recordTimelineWithDetails(tx *gorm.DB, ticketID uint, activityID *uint, operatorID uint, eventType, message string, details map[string]any, reasoning string) error {
	var detailsJSON string
	if len(details) > 0 {
		detailsJSON = mustJSON(details)
	}
	tl := &timelineModel{
		TicketID:   ticketID,
		ActivityID: activityID,
		OperatorID: operatorID,
		EventType:  eventType,
		Message:    message,
		Details:    detailsJSON,
		Reasoning:  e.auditReasoning(reasoning),
	}
	return tx.Create(tl).Error
}

func decisionExplanationDetails(snapshot map[string]any) map[string]any {
	if len(snapshot) == 0 {
		return nil
	}
	return map[string]any{
		"decision_explanation": snapshot,
	}
}

func buildDecisionExplanationSnapshot(plan *DecisionPlan, trigger, decision, nextStep string, activityID *uint) map[string]any {
	basis := "协作规范、workflow_json 与 workflow_context"
	humanOverride := "可执行重试、转人工或撤回"
	if plan != nil {
		if reasoning := strings.TrimSpace(plan.Reasoning); reasoning != "" {
			basis = reasoning
		}
		if humanActivity := firstHumanDecisionActivity(plan.Activities); humanActivity != nil {
			humanOverride = fmt.Sprintf("可通过转人工处理到 %s 节点", decisionActivityName(*humanActivity))
		}
	}
	snapshot := map[string]any{
		"basis":         basis,
		"trigger":       strings.TrimSpace(trigger),
		"decision":      strings.TrimSpace(decision),
		"nextStep":      strings.TrimSpace(nextStep),
		"humanOverride": humanOverride,
	}
	if activityID != nil && *activityID > 0 {
		snapshot["activityId"] = *activityID
	}
	return snapshot
}

func firstHumanDecisionActivity(activities []DecisionActivity) *DecisionActivity {
	for i := range activities {
		if isHumanActivityType(activities[i].Type) {
			return &activities[i]
		}
	}
	return nil
}

func completionDecisionLabel(status, outcome string) string {
	switch status {
	case TicketStatusRejected:
		return "已驳回"
	case TicketStatusCompleted:
		if outcome == TicketOutcomeApproved {
			return "已通过"
		}
		if outcome == TicketOutcomeFulfilled {
			return "已履约"
		}
		return "已完成"
	default:
		return status
	}
}

func (e *SmartEngine) auditReasoning(reasoning string) string {
	if reasoning == "" || e.configProvider == nil {
		return reasoning
	}
	switch e.configProvider.AuditLevel() {
	case "off":
		return ""
	case "summary":
		return truncateReasoning(reasoning, 240)
	default:
		return reasoning
	}
}

func truncateReasoning(reasoning string, limit int) string {
	reasoning = strings.TrimSpace(reasoning)
	if len(reasoning) <= limit {
		return reasoning
	}
	return reasoning[:limit] + "..."
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

	// Validate execution_mode
	switch plan.ExecutionMode {
	case "", "single", "parallel":
		// valid
	default:
		slog.Warn("invalid execution_mode, defaulting to single", "mode", plan.ExecutionMode)
		plan.ExecutionMode = ""
	}

	return &plan, nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func decisionActivityName(da DecisionActivity) string {
	switch da.Type {
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

func humanProgressMessage(activityName, outcome, opinion string) string {
	msg := fmt.Sprintf("活动 [%s] 完成，结果: %s", activityName, outcome)
	if opinion != "" {
		msg = fmt.Sprintf("%s，处理意见: %s", msg, opinion)
	}
	return msg
}

func sameActivityMeaning(existing activityModel, plannedName string, plannedInstructions string) bool {
	existingName := normalizeActivityText(existing.Name)
	plannedName = normalizeActivityText(plannedName)
	if existingName != "" && plannedName != "" && (existingName == plannedName || strings.Contains(existingName, plannedName) || strings.Contains(plannedName, existingName)) {
		return true
	}

	if plannedInstructions == "" || existing.AIDecision == "" {
		return false
	}
	var source DecisionPlan
	if err := json.Unmarshal([]byte(existing.AIDecision), &source); err != nil {
		return false
	}
	for _, a := range source.Activities {
		if normalizeActivityText(a.Instructions) == normalizeActivityText(plannedInstructions) {
			return true
		}
	}
	return false
}

func normalizeActivityText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func hasExplicitRecoveryIntent(instructions string) bool {
	text := strings.ToLower(strings.TrimSpace(instructions))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"退回", "补充", "修正", "更正", "复核", "升级", "转交", "转派", "重新分配", "其他角色", "失败", "取消",
		"return", "revise", "revision", "rework", "review", "escalate", "handoff", "reassign", "cancel", "fail",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func participantSignaturesForDecisionActivity(tx *gorm.DB, da DecisionActivity) map[string]struct{} {
	signatures := map[string]struct{}{}
	if da.ParticipantID != nil && *da.ParticipantID > 0 {
		signatures[fmt.Sprintf("user:%d", *da.ParticipantID)] = struct{}{}
	}
	if da.ParticipantType == "requester" {
		signatures["requester"] = struct{}{}
	}
	if da.ParticipantType == "position_department" && da.PositionCode != "" && da.DepartmentCode != "" {
		positionID, departmentID := resolvePositionDepartmentIDs(tx, da.PositionCode, da.DepartmentCode)
		if positionID > 0 && departmentID > 0 {
			signatures[fmt.Sprintf("position_department:%d:%d", positionID, departmentID)] = struct{}{}
		}
		signatures["position_department_code:"+normalizeActivityText(da.PositionCode)+":"+normalizeActivityText(da.DepartmentCode)] = struct{}{}
	}
	return signatures
}

func participantSignaturesForAssignments(assignments []ActivityAssignmentInfo) map[string]struct{} {
	signatures := map[string]struct{}{}
	for _, a := range assignments {
		if a.ParticipantType == "requester" {
			signatures["requester"] = struct{}{}
		}
		if a.AssigneeID != nil && *a.AssigneeID > 0 {
			signatures[fmt.Sprintf("user:%d", *a.AssigneeID)] = struct{}{}
		}
		if a.UserID != nil && *a.UserID > 0 {
			signatures[fmt.Sprintf("user:%d", *a.UserID)] = struct{}{}
		}
		if a.PositionID != nil && *a.PositionID > 0 && a.DepartmentID != nil && *a.DepartmentID > 0 {
			signatures[fmt.Sprintf("position_department:%d:%d", *a.PositionID, *a.DepartmentID)] = struct{}{}
		}
		if a.PositionID != nil && *a.PositionID > 0 {
			signatures[fmt.Sprintf("position:%d", *a.PositionID)] = struct{}{}
		}
		if a.DepartmentID != nil && *a.DepartmentID > 0 {
			signatures[fmt.Sprintf("department:%d", *a.DepartmentID)] = struct{}{}
		}
	}
	return signatures
}

func participantSignaturesOverlap(left, right map[string]struct{}) bool {
	for sig := range left {
		if _, ok := right[sig]; ok {
			return true
		}
	}
	return false
}

func resolvePositionDepartmentIDs(tx *gorm.DB, positionCode, departmentCode string) (uint, uint) {
	var positionID uint
	var departmentID uint
	tx.Table("positions").Where("code = ?", positionCode).Select("id").Scan(&positionID)
	tx.Table("departments").Where("code = ?", departmentCode).Select("id").Scan(&departmentID)
	return positionID, departmentID
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
	ActionsJSON       string `gorm:"column:actions_json"`
	RuntimeVersionID  *uint  `gorm:"column:runtime_version_id"`
}

func (serviceModel) TableName() string { return "itsm_service_definitions" }

// ensureContinuation checks whether the smart engine should trigger the next
// decision cycle after an activity completes. It handles terminal states,
// circuit-breaker, and parallel convergence (with SELECT FOR UPDATE).
func (e *SmartEngine) ensureContinuation(tx *gorm.DB, ticket *ticketModel, completedActivityID uint) (bool, error) {
	// 1. Terminal state → nothing to do
	switch ticket.Status {
	case TicketStatusCompleted, TicketStatusRejected, TicketStatusWithdrawn, TicketStatusCancelled, TicketStatusFailed:
		return false, nil
	}

	// 2. Circuit-breaker
	if ticket.AIFailureCount >= MaxAIFailureCount {
		slog.Warn("ensureContinuation: AI disabled, skipping", "ticketID", ticket.ID, "failures", ticket.AIFailureCount)
		return false, nil
	}

	// 3. Parallel convergence check
	if completedActivityID > 0 {
		var groupID string
		if err := tx.Model(&activityModel{}).Where("id = ?", completedActivityID).Select("activity_group_id").Scan(&groupID).Error; err != nil {
			return false, err
		}

		if groupID != "" {
			// Lock the group rows and check for incomplete siblings
			var ids []uint
			if err := tx.Model(&activityModel{}).
				Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("activity_group_id = ? AND status NOT IN ?", groupID, CompletedActivityStatuses()).
				Pluck("id", &ids).Error; err != nil {
				return false, err
			}

			if len(ids) > 0 {
				// Check for convergence timeout.
				// Use ORDER BY + LIMIT 1 instead of MIN() aggregate to avoid
				// SQLite driver scan issues with bare time.Time.
				var earliest activityModel
				if err := tx.Where("activity_group_id = ?", groupID).
					Order("created_at ASC").
					First(&earliest).Error; err != nil || earliest.CreatedAt.IsZero() {
					// Can't determine age, keep waiting
					slog.Info("ensureContinuation: parallel group not converged",
						"ticketID", ticket.ID, "groupID", groupID, "remaining", len(ids))
					return false, nil
				}
				earliestCreated := earliest.CreatedAt

				timeout := e.resolveConvergenceTimeout(tx, ticket)
				if time.Since(earliestCreated) > timeout {
					// Timeout: cancel pending siblings
					slog.Warn("ensureContinuation: parallel group convergence timeout",
						"ticketID", ticket.ID, "groupID", groupID,
						"remaining", len(ids), "timeout", timeout)
					if err := tx.Model(&activityModel{}).
						Where("id IN ? AND status NOT IN ?", ids, CompletedActivityStatuses()).
						Updates(map[string]any{
							"status":      ActivityCancelled,
							"finished_at": time.Now(),
						}).Error; err != nil {
						return false, fmt.Errorf("cancel timed-out activities: %w", err)
					}
					e.recordTimeline(tx, ticket.ID, nil, 0, "parallel_convergence_timeout",
						fmt.Sprintf("并行审批组 %s 超时，%d 个未完成活动已取消", groupID, len(ids)), "")
					// Fall through to submit next decision cycle
				} else {
					slog.Info("ensureContinuation: parallel group not converged",
						"ticketID", ticket.ID, "groupID", groupID, "remaining", len(ids))
					return false, nil
				}
			}
			slog.Info("ensureContinuation: parallel group converged",
				"ticketID", ticket.ID, "groupID", groupID)
		}
	}

	decisioningStatus := TicketStatusDecisioning
	if completedActivityID > 0 {
		var outcome string
		if err := tx.Model(&activityModel{}).Where("id = ?", completedActivityID).Select("transition_outcome").Scan(&outcome).Error; err != nil {
			return false, err
		}
		decisioningStatus = TicketDecisioningStatusForOutcome(outcome)
	}
	if err := tx.Model(&ticketModel{}).Where("id = ?", ticket.ID).Updates(map[string]any{
		"status":              decisioningStatus,
		"outcome":             "",
		"current_activity_id": nil,
	}).Error; err != nil {
		return false, err
	}
	return true, nil
}

// uintPtrIf returns a *uint if v > 0, else nil.
func uintPtrIf(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

// resolveConvergenceTimeout determines the convergence timeout for a parallel group.
// Priority: SLA resolution_deadline > EngineConfigProvider > hardcoded 168h.
func (e *SmartEngine) resolveConvergenceTimeout(tx *gorm.DB, ticket *ticketModel) time.Duration {
	// 1. Try SLA resolution deadline
	if ticket.SLAResolutionDeadline != nil && !ticket.SLAResolutionDeadline.IsZero() {
		remaining := time.Until(*ticket.SLAResolutionDeadline)
		if remaining > 0 {
			return remaining
		}
	}

	// 2. Try config provider
	if e.configProvider != nil {
		if t := e.configProvider.ParallelConvergenceTimeout(); t > 0 {
			return t
		}
	}

	// 3. Hardcoded fallback
	return 168 * time.Hour
}

// ExecuteDecisionPlan executes an already validated decision plan.
func (e *SmartEngine) ExecuteDecisionPlan(tx *gorm.DB, ticketID uint, plan *DecisionPlan) error {
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

func (e *SmartEngine) SubmitProgressTaskTx(tx *gorm.DB, payload json.RawMessage) error {
	return submitTaskInTx(e.scheduler, tx, "itsm-smart-progress", payload)
}

// RunDecisionCycleForTicket runs the decision cycle for a ticket (used by scheduler task).
func (e *SmartEngine) RunDecisionCycleForTicket(ctx context.Context, tx *gorm.DB, ticketID uint, completedActivityID *uint, triggerReasons ...string) error {
	svcInfo, err := e.loadServiceForTicket(tx, ticketID)
	if err != nil {
		return fmt.Errorf("load service: %w", err)
	}
	return e.runDecisionCycle(ctx, tx, ticketID, completedActivityID, svcInfo, normalizedTriggerReason(completedActivityID, triggerReasons...))
}

// --- Agentic decision (delegates to DecisionExecutor) ---

// agenticDecision builds domain context and tools, then delegates the ReAct loop
// to the DecisionExecutor (implemented by the AI App).
func (e *SmartEngine) agenticDecision(ctx context.Context, tx *gorm.DB, ticketID uint, completedActivityID *uint, svc *serviceModel, triggerReason string) (*DecisionPlan, error) {
	var agentID uint
	if e.configProvider != nil {
		agentID = e.configProvider.DecisionAgentID()
	}
	if agentID == 0 {
		return nil, fmt.Errorf("流程决策岗未上岗")
	}

	// Build seed messages (domain context)
	var decisionMode string
	if e.configProvider != nil {
		decisionMode = e.configProvider.DecisionMode()
	}
	systemMsg, userMsg, err := e.buildInitialSeed(tx, ticketID, svc, decisionMode, completedActivityID, triggerReason)
	if err != nil {
		return nil, fmt.Errorf("build initial seed: %w", err)
	}

	// Prepare tool context
	toolCtx := &decisionToolContext{
		ctx:                 ctx,
		data:                NewDecisionDataStore(tx),
		ticketID:            ticketID,
		serviceID:           svc.ID,
		workflowJSON:        svc.WorkflowJSON,
		collaborationSpec:   svc.CollaborationSpec,
		knowledgeSearcher:   e.knowledgeSearcher,
		resolver:            e.resolver,
		actionExecutor:      e.actionExecutor,
		completedActivityID: completedActivityID,
		configProvider:      e.configProvider,
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

	// Build tool handler closure with logging wrapper
	toolHandler := func(name string, args json.RawMessage) (json.RawMessage, error) {
		handler, ok := handlerMap[name]
		if !ok {
			slog.Warn("decision-tool: unknown", "ticketID", ticketID, "tool", name)
			return toolError(fmt.Sprintf("未知工具: %s", name))
		}
		start := time.Now()
		result, err := handler(toolCtx, args)
		elapsed := time.Since(start)
		if err != nil {
			slog.Warn("decision-tool: error",
				"ticketID", ticketID, "tool", name, "durationMs", elapsed.Milliseconds(), "error", err.Error())
		} else {
			slog.Info("decision-tool: call",
				"ticketID", ticketID, "tool", name, "durationMs", elapsed.Milliseconds(), "ok", true)
		}
		return result, err
	}

	resp, err := e.decisionExecutor.Execute(ctx, agentID, app.AIDecisionRequest{
		SystemPrompt: systemMsg,
		UserMessage:  userMsg,
		Tools:        toolDefs,
		ToolHandler:  toolHandler,
		MaxTurns:     DecisionToolMaxTurns,
		Metadata:     map[string]any{"ticketID": ticketID, "serviceID": svc.ID},
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
func (e *SmartEngine) buildInitialSeed(tx *gorm.DB, ticketID uint, svc *serviceModel, decisionMode string, completedActivityID *uint, triggerReason string) (string, string, error) {
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
		FormData    string
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).
		Select("code, title, description, status, source, priority_id, form_data").
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

	allowedSteps := []string{"process", "action", "notify", "form", "complete", "escalate"}
	if IsTerminalTicketStatus(ticket.Status) {
		allowedSteps = []string{}
	}

	seed := map[string]any{
		"decision_cycle": map[string]any{
			"trigger_reason":        triggerReason,
			"completed_activity_id": completedActivityID,
			"decision_mode":         normalizedDecisionMode(decisionMode),
		},
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
	if svc.WorkflowJSON != "" {
		seed["workflow_context"] = map[string]any{
			"kind":    "ai_generated_workflow_blueprint",
			"summary": extractWorkflowHints(svc.WorkflowJSON),
			"policy": []string{
				"协作规范是核心事实源，workflow_json 是辅助理解协作规范的结构化背景。",
				"当协作规范与 workflow_json 冲突时，必须以协作规范为准。",
				"activity_completed 触发时，必须解释 completed_activity 与 workflow_json 中节点、边、条件的关系。",
				"不得在没有新证据的情况下重复创建刚被驳回的同一人工处理任务。",
				"协作规范未显式定义补充信息或返工路径时，不得把 rejected 解释为退回申请人补充，也不得创建申请人补充/返工类人工活动。",
			},
		}
	}
	formData := map[string]any{}
	if ticket.FormData != "" {
		_ = json.Unmarshal([]byte(ticket.FormData), &formData)
	}
	var completedActivity *activityModel
	if completedActivityID != nil && *completedActivityID > 0 {
		data := NewDecisionDataStore(tx)
		if completed, err := data.GetActivityByID(ticketID, *completedActivityID); err == nil {
			// Lightweight anchor — full facts available via ticket_context tool
			completedActivity = completed
			seed["completed_activity"] = map[string]any{
				"id":               completed.ID,
				"outcome":          completed.TransitionOutcome,
				"operator_opinion": completed.DecisionReasoning,
			}
			if isHumanActivityType(completed.ActivityType) && !isPositiveActivityOutcome(completed.TransitionOutcome) {
				policy := map[string]any{
					"must_explain_rejection": true,
					"operator_opinion":       completed.DecisionReasoning,
					"forbidden_path":         "没有新证据时重复创建与刚被驳回活动相同的处理任务",
				}
				// Derive recovery path from workflow graph
				if rejTarget := findRejectedEdgeTarget(svc.WorkflowJSON, completed.NodeID); rejTarget != "" {
					policy["workflow_rejected_target"] = rejTarget
					policy["instruction"] = fmt.Sprintf(
						"workflow_json 的 rejected 出边指向 %s，必须遵循此路径。如果目标是 end 节点，直接输出 complete 结束流程。",
						rejTarget,
					)
				} else {
					policy["instruction"] = "未找到 workflow_json 的 rejected 出边时，必须回到协作规范判断恢复路径；协作规范未显式定义补充信息或返工路径时，不得创建申请人补充表单或申请人补充/返工类人工活动。"
					policy["allowed_recovery_paths"] = []string{"按协作规范定义的恢复路径处理", "升级/转交其他角色", "结束为失败或取消"}
				}
				seed["rejected_activity_policy"] = policy
			} else if isPositiveActivityOutcome(completed.TransitionOutcome) && completed.NodeID != "" {
				// Symmetric approved path injection
				if targetID, targetLabel, targetType := findOutcomeEdgeTargetInfo(svc.WorkflowJSON, completed.NodeID, "approved"); targetID != "" {
					instruction := fmt.Sprintf("workflow_json 的 approved 出边指向 %s，应遵循此路径继续推进，不应偏离", targetLabel)
					if targetType == "end" {
						instruction += "。目标是 end 节点，流程可能即将结束"
					}
					seed["approved_next_step"] = map[string]any{
						"target_node_id":    targetID,
						"target_node_label": targetLabel,
						"target_node_type":  targetType,
						"instruction":       instruction,
					}
				}
			}
		}
	}
	if branchInsights := buildBranchInsights(svc.WorkflowJSON, svc.CollaborationSpec, formData, "", "", completedActivity); len(branchInsights) > 0 {
		for _, key := range []string{"selected_branch", "active_branch_contract", "current_branch_node_id", "allowed_next_branch_nodes", "completion_contract", "branch_reasoning_basis"} {
			if value, ok := branchInsights[key]; ok {
				seed[key] = value
			}
		}
		if workflowCtx, ok := seed["workflow_context"].(map[string]any); ok {
			for _, key := range []string{"selected_branch", "active_branch_contract", "current_branch_node_id", "allowed_next_branch_nodes", "completion_contract", "branch_reasoning_basis"} {
				if value, ok := branchInsights[key]; ok {
					workflowCtx[key] = value
				}
			}
		}
	}
	seedJSON, _ := json.MarshalIndent(seed, "", "  ")
	userMsg := fmt.Sprintf("## 工单信息与策略约束\n\n```json\n%s\n```\n\n请使用可用工具收集所需信息，然后输出最终决策。", seedJSON)

	return systemMsg, userMsg, nil
}

func decisionTriggerReason(completedActivityID *uint) string {
	if completedActivityID != nil && *completedActivityID > 0 {
		return TriggerReasonActivityDone
	}
	return TriggerReasonInitialDecision
}

func normalizedTriggerReason(completedActivityID *uint, triggerReasons ...string) string {
	for _, reason := range triggerReasons {
		if reason = strings.TrimSpace(reason); reason != "" {
			return reason
		}
	}
	return decisionTriggerReason(completedActivityID)
}

func normalizedDecisionMode(decisionMode string) string {
	if decisionMode == "" {
		return "direct_first"
	}
	return decisionMode
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
			prompt += "## 决策策略\n\n协作规范是核心事实源，workflow_json 是辅助理解协作规范的结构化背景。优先遵守协作规范；workflow_json 可用于理解节点、边和条件，但不能发明协作规范没有的业务动作。两者冲突时，以协作规范为准；无法确定时保守降级为人工处置。\n\n"
			prompt += "## 工作流参考路径\n\n" + hints + "\n\n---\n\n"
		} else {
			slog.Warn("direct_first mode but no workflow hints available, degrading to ai_only")
			prompt += "## 决策策略\n\n始终使用 AI 推理决定下一步，不依赖预定义路径。\n\n---\n\n"
		}
	}
	prompt += "## 分支闭环约束\n\n业务分支与候选处理人不是一回事。一旦工单已经命中某条业务分支，后续只能在该分支内推进或结束，不能因为其他岗位也相关就切换到别的业务分支。若协作规范写明“处理完成后直接结束流程”，则 approved/rejected 都应优先解释为当前分支的终态推进，workflow_json 的 approved/rejected 出边属于 continuation contract，而不是普通建议。\n\n---\n\n"
	if guidance := agenticStructuredRoutingGuidance(collaborationSpec); guidance != "" {
		prompt += guidance + "\n\n---\n\n"
	}
	prompt += agenticToolGuidance
	prompt += "\n\n---\n\n"
	prompt += agenticOutputFormat
	return prompt
}

func agenticStructuredRoutingGuidance(collaborationSpec string) string {
	switch {
	case looksLikeServerAccessPurposeSpec(collaborationSpec):
		return `## 结构化路由判定守卫

生产服务器临时访问申请必须先按协作规范和 form.access_reason、form.operation_purpose 判定业务分支，再解析参与人：
- 应用排障、应用进程、日志查看、运行状态、主机巡检、进程处理、磁盘清理、一般生产运维 => decision.resolve_participant 使用 {"type":"position_department","department_code":"it","position_code":"ops_admin"}。
- 抓包、链路诊断、ACL、负载均衡、防火墙策略、网络访问路径、连通性 => decision.resolve_participant 使用 {"type":"position_department","department_code":"it","position_code":"network_admin"}。
- 安全审计、取证、证据保全、漏洞、入侵排查、合规核查、异常访问、高敏访问 => decision.resolve_participant 使用 {"type":"position_department","department_code":"it","position_code":"security_admin"}。

“安全窗口”“生产安全窗口”“高敏发布安全窗口”只是访问时段或变更窗口修饰词，不是 security_admin 分支证据。若 access_reason/operation_purpose 同时命中多个业务分支，或缺失/未知，不得高置信选择单一路由，应降级为澄清或人工诊断。decision.resolve_participant 的 department_code/position_code 必须与最终输出活动的业务分支一致；如果发现解析了错误岗位，必须重新按协作规范解析正确岗位后再输出决策。`
	case looksLikeVPNRequestKindSpec(collaborationSpec):
		return `## 结构化路由判定守卫

VPN 申请必须以 form.request_kind 的枚举值为路由事实源，不能被 device_usage、reason 或 workflow_json 中的自由文本诱导。网络类枚举解析 it/network_admin，安全类枚举解析 it/security_admin；缺失、未知或同时命中多个分支时，不得高置信选择单一路由。decision.resolve_participant 的 department_code/position_code 必须与最终输出活动的业务分支一致。`
	default:
		return ""
	}
}

const agenticToolGuidance = `## 工具使用指引

你可以使用以下工具收集证据。工具结果是决策事实，不能用猜测替代。

- **decision.ticket_context** — 获取工单完整上下文（表单数据、SLA、当前处理任务、活动历史、动作进度、并行组状态）
- **decision.knowledge_search** — 搜索服务关联知识库
- **decision.resolve_participant** — 按类型解析参与人（requester/user/position/department/position_department/requester_manager）
- **decision.user_workload** — 查询用户当前工单负载
- **decision.similar_history** — 查询同服务历史工单的处理模式
- **decision.sla_status** — 查询 SLA 状态和紧急程度
- **decision.list_actions** — 查询服务可用的自动化动作
- **decision.execute_action** — 同步执行服务配置的自动化动作（webhook），返回执行结果。在决策推理过程中直接触发自动化操作并观察结果。

### 推荐推理步骤

1. 必须先用 decision.ticket_context 了解完整上下文，尤其是 current_activities、activity_history、action_progress、parallel_groups 和 is_terminal。
2. 如果 is_terminal=true，直接输出 complete 或保持终态判断，不要创建新活动。
3. 当 trigger_reason=activity_completed 时，必须先读取 completed_activity、completed_requirements 和 workflow_context；刚完成的人工活动如果已经满足当前服务规范，不得再次创建同一处理/表单，必须进入下一条件或 complete。
4. 当 completed_activity.outcome=rejected 或 completed_activity.satisfied=false 时，必须先解释驳回原因、协作规范定义的恢复路径，以及 workflow_json 与该路径的关系。协作规范未显式定义补充信息或返工路径时，不得把 rejected 解释为退回申请人补充，也不得创建申请人补充/返工类人工活动；没有新证据时不得重复创建刚被驳回的同一人工处理任务。
5. 用 decision.list_actions 查看是否有可用自动化动作；协作规范要求预检、放行等同步动作时，优先 decision.execute_action，而不是输出 action 活动。
6. 如需查阅处理规范或知识库，使用 decision.knowledge_search。知识不可用或无命中时可以降级，但要在 reasoning 说明。
7. 需要人工处理/表单时，必须先用 decision.resolve_participant 解析参与人；count=0 时不得高置信输出该人工活动。
8. 候选多人时，可用 decision.user_workload 选择负载较低者；SLA 风险明显时可用 decision.sla_status。
9. 如果需要多角色并行处理，设置 execution_mode 为 "parallel"，在 activities 中列出所有并行角色。
10. 最终输出决策 JSON（不调用任何工具）。

### 完成判断

只有同时满足以下条件，才输出 next_step_type 为 "complete"：
- 服务协作规范或工作流参考允许结束；
- decision.ticket_context.current_activities 为空；
- decision.ticket_context.parallel_groups 没有未完成项；
- 必要的处理、表单或自动动作前置条件已经完成。
- 本轮 completed_activity 已满足服务规范里最后一个人工处理条件。

action_progress.all_completed=true 只说明动作已全部成功执行，不自动代表流程应结束；必须对照服务规范判断。`

const agenticOutputFormat = "## 输出要求\n\n" +
	"当你完成信息收集和推理后，直接输出以下 JSON 格式的决策（不要再调用任何工具）：\n\n" +
	"```json\n" +
	"{\n" +
	"  \"next_step_type\": \"approve|process|action|notify|form|complete|escalate\",\n" +
	"  \"execution_mode\": \"single|parallel\",\n" +
	"  \"activities\": [\n" +
	"    {\n" +
	"      \"type\": \"process|action|notify|form\",\n" +
	"      \"participant_type\": \"requester|user|position_department\",\n" +
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
	"- execution_mode: 执行模式。\"single\"（默认）为串行，\"parallel\" 表示 activities 中的多个活动将并行等待处理，全部完成后才推进到下一步。\n" +
	"- activities: 需要创建的活动列表。\n" +
	"- complete 决策的 activities 必须为空数组。\n" +
	"- participant_type: \"requester\" 表示当前工单申请人，无需 participant_id；\"user\" 需填 participant_id；\"position_department\" 需填 position_code + department_code。\n" +
	"- position_code / department_code: 当 participant_type 为 position_department 时，填写岗位编码和部门编码。\n" +
	"- action_id: 当 type 为 \"action\" 时，填写 decision.list_actions 返回的 action id。\n" +
	"- reasoning: 你的推理过程（会展示给管理员查看）。\n" +
	"- confidence: 决策信心（0.0-1.0）。越高表示越确信。"

// extractWorkflowHints extracts a structured step summary from WorkflowJSON
// for injection into the system prompt in direct_first mode.
// findRejectedEdgeTarget looks up the rejected edge's target for a given workflow node.
// Returns a description like "end（结束）" or "node_3（填写补充信息）", or "" if not found.
func findRejectedEdgeTarget(workflowJSON string, nodeID string) string {
	return findOutcomeEdgeTarget(workflowJSON, nodeID, "rejected")
}

func findApprovedEdgeTarget(workflowJSON string, nodeID string) string {
	return findOutcomeEdgeTarget(workflowJSON, nodeID, "approved")
}

// findOutcomeEdgeTargetInfo returns (nodeID, label, nodeType) for the target of an outcome edge.
func findOutcomeEdgeTargetInfo(workflowJSON string, nodeID string, outcome string) (string, string, string) {
	if workflowJSON == "" || nodeID == "" {
		return "", "", ""
	}
	def, err := ParseWorkflowDef(json.RawMessage(workflowJSON))
	if err != nil {
		return "", "", ""
	}
	nodeMap := make(map[string]*WFNode, len(def.Nodes))
	for i := range def.Nodes {
		nodeMap[def.Nodes[i].ID] = &def.Nodes[i]
	}
	for _, e := range def.Edges {
		if e.Source == nodeID && e.Data.Outcome == outcome {
			if target, ok := nodeMap[e.Target]; ok {
				nd, _ := ParseNodeData(target.Data)
				label := nd.Label
				if label == "" {
					label = target.Type
				}
				return target.ID, label, target.Type
			}
			return e.Target, "", ""
		}
	}
	return "", "", ""
}

func findOutcomeEdgeTarget(workflowJSON string, nodeID string, outcome string) string {
	targetID, label, nodeType := findOutcomeEdgeTargetInfo(workflowJSON, nodeID, outcome)
	if targetID == "" {
		return ""
	}
	if label != "" {
		return fmt.Sprintf("%s（%s，类型: %s）", targetID, label, nodeType)
	}
	return targetID
}

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
		case "process":
			participant := describeParticipants(node.Data.Participants)
			desc = fmt.Sprintf("%d. **处理** [%s] — %s", step, label, participant)
			for _, ei := range outEdges[nodeID] {
				edge := wf.Edges[ei]
				if edge.Data.Outcome != "" {
					targetLabel := edge.Target
					if ti, ok := nodeMap[edge.Target]; ok {
						tl := wf.Nodes[ti].Data.Label
						if tl == "" {
							tl = wf.Nodes[ti].Type
						}
						targetLabel = tl
					}
					desc += fmt.Sprintf("\n   - %s → %s", edge.Data.Outcome, targetLabel)
				}
				queue = append(queue, edge.Target)
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
		case "requester":
			parts = append(parts, "申请人")
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

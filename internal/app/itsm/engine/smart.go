package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// --- Interfaces for AI App dependency injection ---

// AgentProvider provides agent configuration from the AI App.
type AgentProvider interface {
	// GetAgentConfig returns the agent's configuration (system prompt, model info, temperature).
	GetAgentConfig(agentID uint) (*SmartAgentConfig, error)
	// GetAgentConfigByCode returns agent configuration by code (e.g. "itsm.decision").
	GetAgentConfigByCode(code string) (*SmartAgentConfig, error)
}

// SmartAgentConfig holds the agent configuration needed for LLM calls.
type SmartAgentConfig struct {
	Name         string
	SystemPrompt string
	Temperature  float64
	MaxTokens    int
	Model        string // model identifier (e.g. "gpt-4o")
	Protocol     string // "openai" or "anthropic"
	BaseURL      string
	APIKey       string
}

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
	NextStepType string             `json:"next_step_type"` // approve|process|action|notify|form|complete|escalate
	Activities   []DecisionActivity `json:"activities"`
	Reasoning    string             `json:"reasoning"`
	Confidence   float64            `json:"confidence"`
}

// DecisionActivity is a single activity within a decision plan.
type DecisionActivity struct {
	Type            string `json:"type"`
	ParticipantType string `json:"participant_type,omitempty"`
	ParticipantID   *uint  `json:"participant_id,omitempty"`
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
	agentProvider     AgentProvider
	knowledgeSearcher KnowledgeSearcher
	userProvider      UserProvider
	resolver          *ParticipantResolver
	scheduler         TaskSubmitter
	configProvider    EngineConfigProvider
}

// NewSmartEngine creates a SmartEngine with optional AI dependencies.
func NewSmartEngine(
	agentProvider AgentProvider,
	knowledgeSearcher KnowledgeSearcher,
	userProvider UserProvider,
	resolver *ParticipantResolver,
	scheduler TaskSubmitter,
	configProvider EngineConfigProvider,
) *SmartEngine {
	return &SmartEngine{
		agentProvider:     agentProvider,
		knowledgeSearcher: knowledgeSearcher,
		userProvider:      userProvider,
		resolver:          resolver,
		scheduler:         scheduler,
		configProvider:    configProvider,
	}
}

// IsAvailable returns true if the smart engine has AI dependencies.
func (e *SmartEngine) IsAvailable() bool {
	return e.agentProvider != nil
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

	// Submit async task for next decision cycle
	svcInfo, err := e.loadServiceForTicket(tx, params.TicketID)
	if err != nil {
		return fmt.Errorf("load service: %w", err)
	}

	if e.scheduler != nil {
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":             params.TicketID,
			"completed_activity_id": params.ActivityID,
		})
		if err := e.scheduler.SubmitTask("itsm-smart-progress", payload); err != nil {
			slog.Error("failed to submit smart-progress task", "error", err, "ticketID", params.TicketID)
			// Fallback: run decision cycle synchronously
			return e.runDecisionCycle(ctx, tx, params.TicketID, &params.ActivityID, svcInfo)
		}
		return nil
	}

	// No scheduler available, run synchronously
	return e.runDecisionCycle(ctx, tx, params.TicketID, &params.ActivityID, svcInfo)
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
	if err := tx.First(&ticket, ticketID).Error; err != nil {
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
	now := time.Now()
	planJSON := mustJSON(plan)

	for _, da := range plan.Activities {
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

		tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)

		// Create assignment if participant specified
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
			tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", *da.ParticipantID)
		} else if da.Type == "approve" || da.Type == "process" || da.Type == "form" {
			// Activity needs a participant but AI didn't specify one — try fallback
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
			fmt.Sprintf("AI 自动执行：%s（信心 %.0f%%）", decisionActivityName(da), plan.Confidence*100),
			plan.Reasoning)
	}

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

	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)

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
	jsonStr := extractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("无法从 Agent 输出中提取 JSON")
	}

	var plan DecisionPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return &plan, nil
}

// extractJSON extracts a JSON object from text that may contain markdown code blocks.
func extractJSON(s string) string {
	// Try to find JSON in code blocks first
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start = i
			break
		}
	}
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
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
}

func (serviceModel) TableName() string { return "itsm_service_definitions" }

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

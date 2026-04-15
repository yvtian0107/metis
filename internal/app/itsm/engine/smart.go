package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"metis/internal/llm"
)

// --- Interfaces for AI App dependency injection ---

// AgentProvider provides agent configuration from the AI App.
type AgentProvider interface {
	// GetAgentConfig returns the agent's configuration (system prompt, model info, temperature).
	GetAgentConfig(agentID uint) (*SmartAgentConfig, error)
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

// --- Ticket case snapshot types ---

// TicketCase is the complete snapshot sent to the agent for decision-making.
type TicketCase struct {
	Ticket            TicketInfo        `json:"ticket"`
	Service           ServiceInfo       `json:"service"`
	CollaborationSpec string            `json:"collaboration_spec,omitempty"`
	SLAStatus         *SLAStatusInfo    `json:"sla_status,omitempty"`
	ActivityHistory   []ActivitySummary `json:"activity_history,omitempty"`
	FormData          json.RawMessage   `json:"form_data,omitempty"`
}

type TicketInfo struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
	Source      string `json:"source"`
}

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	EngineType  string `json:"engine_type"`
}

type SLAStatusInfo struct {
	ResponseRemainingSeconds   int64 `json:"response_remaining_seconds"`
	ResolutionRemainingSeconds int64 `json:"resolution_remaining_seconds"`
}

type ActivitySummary struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Outcome     string  `json:"outcome"`
	CompletedAt string  `json:"completed_at,omitempty"`
	AIReasoning string  `json:"ai_reasoning,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}

// --- Policy snapshot types ---

// TicketPolicySnapshot defines what the agent is allowed to do.
type TicketPolicySnapshot struct {
	AllowedStepTypes         []string               `json:"allowed_step_types"`
	ParticipantCandidates    []ParticipantCandidate  `json:"participant_candidates"`
	AvailableActions         []ActionInfo            `json:"available_actions"`
	AllowedStatusTransitions []string                `json:"allowed_status_transitions"`
	CurrentStatus            string                  `json:"current_status"`
}

type ActionInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// --- SmartEngine ---

// SmartEngine implements WorkflowEngine via AI Agent-driven decisions.
type SmartEngine struct {
	agentProvider     AgentProvider
	knowledgeSearcher KnowledgeSearcher
	userProvider      UserProvider
	scheduler         TaskSubmitter
}

// NewSmartEngine creates a SmartEngine with optional AI dependencies.
func NewSmartEngine(
	agentProvider AgentProvider,
	knowledgeSearcher KnowledgeSearcher,
	userProvider UserProvider,
	scheduler TaskSubmitter,
) *SmartEngine {
	return &SmartEngine{
		agentProvider:     agentProvider,
		knowledgeSearcher: knowledgeSearcher,
		userProvider:      userProvider,
		scheduler:         scheduler,
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

	msg := "工单已取消"
	if params.Reason != "" {
		msg = "工单已取消: " + params.Reason
	}
	return e.recordTimeline(tx, params.TicketID, nil, params.OperatorID, "ticket_cancelled", msg, "")
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

	// Build TicketCase snapshot
	ticketCase, err := e.buildTicketCase(tx, ticketID, svcInfo)
	if err != nil {
		return e.handleDecisionFailure(tx, ticketID, fmt.Sprintf("构建工单快照失败: %v", err))
	}

	// Compile policy snapshot
	policy, err := e.compilePolicy(tx, ticketID, svcInfo)
	if err != nil {
		return e.handleDecisionFailure(tx, ticketID, fmt.Sprintf("编译策略失败: %v", err))
	}

	// Check terminal state
	if len(policy.AllowedStepTypes) == 0 {
		return nil // ticket is in terminal state, nothing to do
	}

	// Call agent with timeout
	timeout := time.Duration(cfg.DecisionTimeoutSeconds) * time.Second
	decisionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	plan, err := e.callAgent(decisionCtx, tx, svcInfo, ticketCase, policy)
	if err != nil {
		reason := fmt.Sprintf("AI 决策失败: %v", err)
		if decisionCtx.Err() == context.DeadlineExceeded {
			reason = fmt.Sprintf("AI 决策超时（%ds）", cfg.DecisionTimeoutSeconds)
		}
		return e.handleDecisionFailure(tx, ticketID, reason)
	}

	// Validate decision plan
	if err := e.validateDecisionPlan(plan, policy, svcInfo); err != nil {
		// Retry once with format correction
		slog.Warn("decision plan validation failed, retrying", "error", err, "ticketID", ticketID)
		plan, err = e.callAgentWithCorrection(decisionCtx, tx, svcInfo, ticketCase, policy, err.Error())
		if err != nil {
			return e.handleDecisionFailure(tx, ticketID, fmt.Sprintf("AI 决策校验不通过（重试后仍失败）: %v", err))
		}
		if err := e.validateDecisionPlan(plan, policy, svcInfo); err != nil {
			return e.handleDecisionFailure(tx, ticketID, fmt.Sprintf("AI 决策校验不通过: %v", err))
		}
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

// --- Agent call ---

func (e *SmartEngine) callAgent(ctx context.Context, tx *gorm.DB, svc *serviceModel, ticketCase *TicketCase, policy *TicketPolicySnapshot) (*DecisionPlan, error) {
	if svc.AgentID == nil {
		return nil, fmt.Errorf("service has no bound agent")
	}

	agentCfg, err := e.agentProvider.GetAgentConfig(*svc.AgentID)
	if err != nil {
		return nil, fmt.Errorf("get agent config: %w", err)
	}

	// Build messages
	systemPrompt := buildSystemPrompt(svc.CollaborationSpec, agentCfg.SystemPrompt)

	// Knowledge context will be injected from ServiceKnowledgeDocuments (TODO: itsm-service-knowledge-doc)

	caseJSON, _ := json.MarshalIndent(ticketCase, "", "  ")
	policyJSON, _ := json.MarshalIndent(policy, "", "  ")

	userMessage := fmt.Sprintf("## 工单上下文\n\n```json\n%s\n```\n\n## 策略约束\n\n```json\n%s\n```\n\n请根据以上工单上下文和策略约束，输出下一步决策。", caseJSON, policyJSON)

	// Create LLM client
	client, err := llm.NewClient(agentCfg.Protocol, agentCfg.BaseURL, agentCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	temp := float32(agentCfg.Temperature)
	maxTokens := agentCfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	resp, err := client.Chat(ctx, llm.ChatRequest{
		Model: agentCfg.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userMessage},
		},
		MaxTokens:   maxTokens,
		Temperature: &temp,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	return parseDecisionPlan(resp.Content)
}

func (e *SmartEngine) callAgentWithCorrection(ctx context.Context, tx *gorm.DB, svc *serviceModel, ticketCase *TicketCase, policy *TicketPolicySnapshot, validationError string) (*DecisionPlan, error) {
	if svc.AgentID == nil {
		return nil, fmt.Errorf("service has no bound agent")
	}

	agentCfg, err := e.agentProvider.GetAgentConfig(*svc.AgentID)
	if err != nil {
		return nil, fmt.Errorf("get agent config: %w", err)
	}

	systemPrompt := buildSystemPrompt(svc.CollaborationSpec, agentCfg.SystemPrompt)

	caseJSON, _ := json.MarshalIndent(ticketCase, "", "  ")
	policyJSON, _ := json.MarshalIndent(policy, "", "  ")

	userMessage := fmt.Sprintf(`## 工单上下文

%s

## 策略约束

%s

## 格式纠正

上一次输出校验失败，错误：%s

请严格按照以下 JSON 格式输出：
{"next_step_type": "...", "activities": [...], "reasoning": "...", "confidence": 0.0}

next_step_type 必须是以下之一：%v`,
		string(caseJSON), string(policyJSON), validationError, policy.AllowedStepTypes)

	client, err := llm.NewClient(agentCfg.Protocol, agentCfg.BaseURL, agentCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	temp := float32(agentCfg.Temperature)
	resp, err := client.Chat(ctx, llm.ChatRequest{
		Model: agentCfg.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userMessage},
		},
		MaxTokens:   2048,
		Temperature: &temp,
	})
	if err != nil {
		return nil, fmt.Errorf("llm chat retry: %w", err)
	}

	return parseDecisionPlan(resp.Content)
}

// --- Snapshot builders ---

func (e *SmartEngine) buildTicketCase(tx *gorm.DB, ticketID uint, svc *serviceModel) (*TicketCase, error) {
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err != nil {
		return nil, err
	}

	// Load priority name
	var priorityName string
	var priority struct {
		Name string
	}
	if err := tx.Table("itsm_priorities").Where("id = ?", ticket.PriorityID).Select("name").First(&priority).Error; err == nil {
		priorityName = priority.Name
	}

	tc := &TicketCase{
		Ticket: TicketInfo{
			Code:        ticket.Status, // will fix below
			Title:       "",
			Description: "",
			Priority:    priorityName,
			Status:      ticket.Status,
			Source:      "",
		},
		Service: ServiceInfo{
			Name:        svc.Name,
			Description: svc.Description,
			EngineType:  svc.EngineType,
		},
		CollaborationSpec: svc.CollaborationSpec,
	}

	// Load full ticket info from the real table
	var fullTicket struct {
		Code        string
		Title       string
		Description string
		Status      string
		Source      string
		FormData    string
		SLAResponseDeadline   *time.Time
		SLAResolutionDeadline *time.Time
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).
		Select("code, title, description, status, source, form_data, sla_response_deadline, sla_resolution_deadline").
		First(&fullTicket).Error; err == nil {
		tc.Ticket.Code = fullTicket.Code
		tc.Ticket.Title = fullTicket.Title
		tc.Ticket.Description = fullTicket.Description
		tc.Ticket.Status = fullTicket.Status
		tc.Ticket.Source = fullTicket.Source

		if fullTicket.FormData != "" {
			tc.FormData = json.RawMessage(fullTicket.FormData)
		}

		// SLA status
		now := time.Now()
		if fullTicket.SLAResponseDeadline != nil || fullTicket.SLAResolutionDeadline != nil {
			sla := &SLAStatusInfo{}
			if fullTicket.SLAResponseDeadline != nil {
				sla.ResponseRemainingSeconds = int64(fullTicket.SLAResponseDeadline.Sub(now).Seconds())
			}
			if fullTicket.SLAResolutionDeadline != nil {
				sla.ResolutionRemainingSeconds = int64(fullTicket.SLAResolutionDeadline.Sub(now).Seconds())
			}
			tc.SLAStatus = sla
		}
	}

	// Load activity history
	var activities []activityModel
	tx.Where("ticket_id = ? AND status = ?", ticketID, ActivityCompleted).
		Order("id ASC").Find(&activities)

	for _, a := range activities {
		summary := ActivitySummary{
			Type:        a.ActivityType,
			Name:        a.Name,
			Outcome:     a.TransitionOutcome,
			AIReasoning: a.AIReasoning,
			Confidence:  a.AIConfidence,
		}
		if a.FinishedAt != nil {
			summary.CompletedAt = a.FinishedAt.Format(time.RFC3339)
		}
		tc.ActivityHistory = append(tc.ActivityHistory, summary)
	}

	return tc, nil
}

func (e *SmartEngine) compilePolicy(tx *gorm.DB, ticketID uint, svc *serviceModel) (*TicketPolicySnapshot, error) {
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err != nil {
		return nil, err
	}

	policy := &TicketPolicySnapshot{
		CurrentStatus: ticket.Status,
	}

	// Determine allowed step types based on ticket status
	switch ticket.Status {
	case "completed", "cancelled", "failed":
		policy.AllowedStepTypes = []string{}
	default:
		policy.AllowedStepTypes = []string{"approve", "process", "action", "notify", "form", "complete", "escalate"}
	}

	// Determine allowed status transitions
	switch ticket.Status {
	case "pending":
		policy.AllowedStatusTransitions = []string{"in_progress", "cancelled"}
	case "in_progress":
		policy.AllowedStatusTransitions = []string{"completed", "cancelled", "waiting_approval"}
	case "waiting_approval":
		policy.AllowedStatusTransitions = []string{"in_progress", "completed", "cancelled"}
	default:
		policy.AllowedStatusTransitions = []string{}
	}

	// Load participant candidates
	if e.userProvider != nil {
		candidates, err := e.userProvider.ListActiveUsers()
		if err == nil {
			policy.ParticipantCandidates = candidates
		}
	}

	// Load available actions for this service
	var actions []struct {
		ID          uint
		Name        string
		Description string
	}
	tx.Table("itsm_service_actions").
		Where("service_id = ? AND is_active = ?", svc.ID, true).
		Select("id, name, description").
		Find(&actions)

	for _, a := range actions {
		policy.AvailableActions = append(policy.AvailableActions, ActionInfo{
			ID:          a.ID,
			Name:        a.Name,
			Description: a.Description,
		})
	}

	return policy, nil
}

// --- Validation ---

func (e *SmartEngine) validateDecisionPlan(plan *DecisionPlan, policy *TicketPolicySnapshot, svc *serviceModel) error {
	if plan == nil {
		return fmt.Errorf("decision plan is nil")
	}

	// Check next_step_type is allowed
	allowed := false
	for _, t := range policy.AllowedStepTypes {
		if t == plan.NextStepType {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("next_step_type %q 不在允许列表 %v 中", plan.NextStepType, policy.AllowedStepTypes)
	}

	// Validate activities
	for i, a := range plan.Activities {
		if !AllowedSmartStepTypes[a.Type] {
			return fmt.Errorf("activities[%d].type %q 不合法", i, a.Type)
		}

		// Check participant is in candidates if specified
		if a.ParticipantID != nil && *a.ParticipantID > 0 && len(policy.ParticipantCandidates) > 0 {
			found := false
			for _, c := range policy.ParticipantCandidates {
				if c.UserID == *a.ParticipantID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("activities[%d].participant_id %d 不在候选列表中", i, *a.ParticipantID)
			}
		}

		// Check action_id exists if action type
		if a.Type == "action" && a.ActionID != nil && *a.ActionID > 0 {
			found := false
			for _, act := range policy.AvailableActions {
				if act.ID == *a.ActionID {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("activities[%d].action_id %d 不在可用动作列表中", i, *a.ActionID)
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

func buildSystemPrompt(collaborationSpec, agentSystemPrompt string) string {
	prompt := ""
	if collaborationSpec != "" {
		prompt += "## 服务处理规范\n\n" + collaborationSpec + "\n\n---\n\n"
	}
	if agentSystemPrompt != "" {
		prompt += "## 角色定义\n\n" + agentSystemPrompt + "\n\n---\n\n"
	}
	prompt += decisionOutputFormat
	return prompt
}

const decisionOutputFormat = `## 输出要求

你必须严格按照以下 JSON 格式输出决策，不要包含其他文本：

{
  "next_step_type": "process|approve|action|notify|form|complete|escalate",
  "activities": [
    {
      "type": "process|approve|action|notify|form",
      "participant_type": "user",
      "participant_id": 42,
      "action_id": null,
      "instructions": "操作指引"
    }
  ],
  "reasoning": "决策推理过程...",
  "confidence": 0.85
}

字段说明：
- next_step_type: 下一步类型。"complete" 表示流程可以结束。
- activities: 需要创建的活动列表。
- reasoning: 你的推理过程（会展示给管理员审核）。
- confidence: 决策信心（0.0-1.0）。越高表示越确信。`

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

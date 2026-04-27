package bdd

// steps_vpn_smart_deterministic_test.go — deterministic step definitions for smart engine
// BDD scenarios. Uses crafted DecisionPlan to test engine execution paths without LLM.

import (
	"context"
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"time"

	"github.com/cucumber/godog"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/engine"
	"metis/internal/model"
)

// registerDeterministicSteps registers all deterministic smart engine step definitions.
func registerDeterministicSteps(sc *godog.ScenarioContext, bc *bddContext) {
	// Given
	sc.Given(`^已基于静态工作流发布 VPN 开通服务（智能引擎）$`, bc.givenStaticSmartServicePublished)
	sc.Given(`^引擎已配置兜底处理人为已停用用户$`, bc.givenFallbackAssigneeInactive)

	// When
	sc.When(`^执行确定性决策 type="([^"]*)" 参与者为 "([^"]*)"$`, bc.whenDeterministicPlanWithParticipant)
	sc.When(`^执行无参与者的确定性决策 type="([^"]*)"$`, bc.whenDeterministicPlanWithoutParticipant)
	sc.When(`^执行确定性 complete 决策$`, bc.whenDeterministicCompletePlan)
	sc.When(`^工单 AI 失败次数设为上限并尝试决策$`, bc.whenAICircuitBreaker)
	sc.When(`^取消智能工单，原因为 "([^"]*)"$`, bc.whenCancelSmartTicket)
	sc.When(`^创建确定性人工处置决策 type="([^"]*)"$`, bc.whenCreatePendingProcessDecision)
	sc.When(`^管理员取消当前人工处置决策$`, bc.whenAdminRejectsDecision)

	// Then
	sc.Then(`^最新活动类型为 "([^"]*)" 且状态为 "([^"]*)"$`, bc.thenLatestActivityTypeAndStatus)
	sc.Then(`^最新活动存在指派记录$`, bc.thenLatestActivityHasAssignment)
	sc.Then(`^最新活动无指派记录$`, bc.thenLatestActivityNoAssignment)
	sc.Then(`^时间线包含 "([^"]*)" 类型事件$`, bc.thenTimelineContainsEventType)
	sc.Then(`^工单分配人为 "([^"]*)"$`, bc.thenTicketAssigneeIs)
	sc.Then(`^所有活动和指派均已取消$`, bc.thenAllActivitiesAndAssignmentsCancelled)
}

// --- Given steps ---

// givenStaticSmartServicePublished creates a smart service with a static workflow JSON,
// bypassing LLM generation. Deterministic tests only use ExecuteDecisionPlan, so the
// workflow content doesn't matter — we just need valid service + ticket records.
func (bc *bddContext) givenStaticSmartServicePublished() error {
	// Static workflow — minimal but valid structure.
	staticWorkflow := json.RawMessage(`{
		"nodes": [
			{"id": "start", "type": "start", "label": "开始"},
				{"id": "process", "type": "process", "label": "处理", "config": {"participant_type": "position_department", "position_code": "network_admin", "department_code": "it"}},
			{"id": "end", "type": "end", "label": "结束"}
		],
		"edges": [
				{"source": "start", "target": "process"},
				{"source": "process", "target": "end", "condition": "completed"}
		]
	}`)

	catalog := &ServiceCatalog{
		Name:     "VPN服务(智能-确定性)",
		Code:     "vpn-smart-deterministic",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create service catalog: %w", err)
	}

	priority := &Priority{
		Name:     "普通",
		Code:     "normal-smart-det",
		Value:    3,
		Color:    "#52c41a",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	agent := &ai.Agent{
		Name:         "流程决策智能体(确定性)",
		Type:         "assistant",
		IsActive:     true,
		Visibility:   "private",
		Strategy:     "react",
		SystemPrompt: "确定性测试用智能体",
		Temperature:  0.2,
		MaxTokens:    2048,
		MaxTurns:     1,
		CreatedBy:    1,
	}
	if err := bc.db.Create(agent).Error; err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	svc := &ServiceDefinition{
		Name:              "VPN开通申请(智能-确定性)",
		Code:              "vpn-activation-smart-det",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(staticWorkflow),
		CollaborationSpec: vpnCollaborationSpec,
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc
	return nil
}

func (bc *bddContext) givenFallbackAssigneeInactive() error {
	// Create a user then deactivate it (GORM default:true on IsActive skips false on Create).
	user := &model.User{Username: "inactive-fallback", IsActive: true}
	if err := bc.db.Create(user).Error; err != nil {
		return fmt.Errorf("create inactive user: %w", err)
	}
	if err := bc.db.Model(user).Update("is_active", false).Error; err != nil {
		return fmt.Errorf("deactivate user: %w", err)
	}

	bc.fallbackUserID = user.ID
	configProvider := &testConfigProvider{fallbackAssigneeID: user.ID}

	executor := &testDecisionExecutor{db: bc.db, llmCfg: bc.llmCfg, recordToolCall: bc.recordToolCall}
	userProvider := &testUserProvider{db: bc.db}
	orgSvc := &testOrgService{db: bc.db}
	resolver := engine.NewParticipantResolver(orgSvc)

	bc.smartEngine = engine.NewSmartEngine(executor, nil, userProvider, resolver, &noopSubmitter{}, configProvider)
	return nil
}

// --- When steps ---

func (bc *bddContext) whenDeterministicPlanWithParticipant(actType, username string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	// Ensure ticket is in_progress.
	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", "in_progress").Error; err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}

	plan := &engine.DecisionPlan{
		NextStepType: actType,
		Activities: []engine.DecisionActivity{
			{
				Type:          actType,
				ParticipantID: &user.ID,
				Instructions:  fmt.Sprintf("确定性测试：%s", actType),
			},
		},
		Reasoning:  fmt.Sprintf("确定性测试：%s 类型决策", actType),
		Confidence: 0.9,
	}

	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, plan); err != nil {
		return fmt.Errorf("execute confirmed plan: %w", err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenDeterministicPlanWithoutParticipant(actType string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", "in_progress").Error; err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}

	plan := &engine.DecisionPlan{
		NextStepType: actType,
		Activities: []engine.DecisionActivity{
			{
				Type:         actType,
				Instructions: fmt.Sprintf("确定性测试：%s", actType),
			},
		},
		Reasoning:  fmt.Sprintf("确定性测试：%s 类型决策", actType),
		Confidence: 0.9,
	}

	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, plan); err != nil {
		return fmt.Errorf("execute confirmed plan: %w", err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenDeterministicCompletePlan() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", "in_progress").Error; err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}

	plan := &engine.DecisionPlan{
		NextStepType: "complete",
		Activities:   []engine.DecisionActivity{},
		Reasoning:    "确定性测试：流程完结",
		Confidence:   0.95,
	}

	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, plan); err != nil {
		return fmt.Errorf("execute confirmed plan: %w", err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenAICircuitBreaker() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	// Set ticket to in_progress with failure count at max.
	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Updates(map[string]any{
			"status":           "in_progress",
			"ai_failure_count": engine.MaxAIFailureCount,
		}).Error; err != nil {
		return fmt.Errorf("set failure count: %w", err)
	}

	ctx := context.Background()
	err := bc.smartEngine.RunDecisionCycleForTicket(ctx, bc.db, bc.ticket.ID, nil)
	if err != nil {
		bc.lastErr = err
		return nil // Let Then steps check the error.
	}
	return nil
}

func (bc *bddContext) whenCancelSmartTicket(reason string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	ctx := context.Background()
	if err := bc.smartEngine.Cancel(ctx, bc.db, engine.CancelParams{
		TicketID: bc.ticket.ID,
		Reason:   reason,
	}); err != nil {
		return fmt.Errorf("cancel smart ticket: %w", err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenCreatePendingProcessDecision(actType string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", "in_progress").Error; err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}

	now := time.Now()
	activity := &TicketActivity{
		TicketID:     bc.ticket.ID,
		Name:         "AI 决策待人工处置",
		ActivityType: actType,
		Status:       engine.ActivityPending,
		AIReasoning:  "确定性测试：低置信度决策",
		AIConfidence: 0.5,
		StartedAt:    &now,
	}
	if err := bc.db.Create(activity).Error; err != nil {
		return fmt.Errorf("create manual handling activity: %w", err)
	}

	bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).Update("current_activity_id", activity.ID)
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenAdminRejectsDecision() error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.Status != engine.ActivityPending {
		return fmt.Errorf("expected pending, got %q", activity.Status)
	}

	now := time.Now()
	if err := bc.db.Model(&TicketActivity{}).Where("id = ?", activity.ID).
		Updates(map[string]any{
			"status":      engine.ActivityCancelled,
			"finished_at": now,
		}).Error; err != nil {
		return fmt.Errorf("reject activity: %w", err)
	}

	return bc.db.Create(&TicketTimeline{
		TicketID:   bc.ticket.ID,
		ActivityID: &activity.ID,
		EventType:  "ai_decision_cancelled",
		Message:    "管理员取消了 AI 人工处置任务",
	}).Error
}

// --- Then steps ---

func (bc *bddContext) thenLatestActivityTypeAndStatus(actType, status string) error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.ActivityType != actType {
		return fmt.Errorf("expected activity type %q, got %q", actType, activity.ActivityType)
	}
	if activity.Status != status {
		return fmt.Errorf("expected activity status %q, got %q", status, activity.Status)
	}
	return nil
}

func (bc *bddContext) thenLatestActivityHasAssignment() error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	var count int64
	if err := bc.db.Model(&TicketAssignment{}).Where("activity_id = ?", activity.ID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no assignment found for activity %d", activity.ID)
	}
	return nil
}

func (bc *bddContext) thenLatestActivityNoAssignment() error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	var count int64
	if err := bc.db.Model(&TicketAssignment{}).Where("activity_id = ?", activity.ID).
		Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("expected no assignments for activity %d, got %d", activity.ID, count)
	}
	return nil
}

func (bc *bddContext) thenTimelineContainsEventType(eventType string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var count int64
	if err := bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type = ?", bc.ticket.ID, eventType).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no %q event found in timeline for ticket %d", eventType, bc.ticket.ID)
	}
	return nil
}

func (bc *bddContext) thenTicketAssigneeIs(username string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}
	if bc.ticket.AssigneeID == nil || *bc.ticket.AssigneeID != user.ID {
		actual := uint(0)
		if bc.ticket.AssigneeID != nil {
			actual = *bc.ticket.AssigneeID
		}
		return fmt.Errorf("expected ticket assignee_id=%d (%s), got %d", user.ID, username, actual)
	}
	return nil
}

func (bc *bddContext) thenAllActivitiesAndAssignmentsCancelled() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var activeActivities int64
	bc.db.Model(&TicketActivity{}).
		Where("ticket_id = ? AND status NOT IN ?", bc.ticket.ID,
			[]string{engine.ActivityCancelled, engine.ActivityCompleted}).
		Count(&activeActivities)
	if activeActivities > 0 {
		return fmt.Errorf("expected all activities cancelled, but %d still active", activeActivities)
	}

	var activeAssignments int64
	bc.db.Model(&TicketAssignment{}).
		Where("ticket_id = ? AND status NOT IN ?", bc.ticket.ID,
			[]string{"cancelled", "completed"}).
		Count(&activeAssignments)
	if activeAssignments > 0 {
		return fmt.Errorf("expected all assignments cancelled, but %d still active", activeAssignments)
	}
	return nil
}

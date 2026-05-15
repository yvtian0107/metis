package bdd

// steps_parallel_approval_new_test.go — step definitions for new multi-role parallel approval
// coverage scenarios (BDD-NEW-1 through BDD-NEW-8).
// All scenarios are @deterministic — no LLM required.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cucumber/godog"

	. "metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/engine"

	ai "metis/internal/app/ai/runtime"
	org "metis/internal/app/org/domain"
	"metis/internal/model"
)

// registerParallelApprovalNewSteps registers step definitions for BDD-NEW-1..BDD-NEW-8.
func registerParallelApprovalNewSteps(sc *godog.ScenarioContext, bc *bddContext) {
	// Given
	sc.Given(`^已基于静态并签工作流发布多角色申请服务（智能引擎）$`, bc.givenStaticParallelApprovalServicePublished)
	sc.Given(`^岗位 "([^"]*)" 当前没有活跃成员$`, bc.givenPositionHasNoActiveMembers)

	// When
	sc.When(`^向岗位 "([^"]*)" 添加活跃成员 "([^"]*)"$`, bc.whenAddActiveMemberToPosition)
	sc.When(`^执行 SmartRecovery 周期任务$`, bc.whenRunSmartRecovery)
	sc.When(`^执行确定性并签审批决策$`, bc.whenDeterministicParallelApproveDecision)
	sc.When(`^执行确定性单签审批决策，岗位为 "([^"]*)"$`, bc.whenDeterministicSingleApproveDecision)
	sc.When(`^并签审批组中岗位 "([^"]*)" 的审批人认领并审批驳回$`, bc.whenParallelApprovalRoleRejects)
	sc.When(`^当前活动的被分配人认领并审批驳回$`, bc.whenCurrentActivityRejected)
	sc.When(`^并签审批组两岗位审批人模拟并发通过$`, bc.whenBothParallelApprovesConcurrent)
	sc.When(`^执行确定性完成决策$`, bc.whenDeterministicCompleteWithoutReset)

	// Then
	sc.Then(`^有且只有一个分配给岗位 "([^"]*)" 的待处理审批活动$`, bc.thenExactlyOnePendingApprovalForPosition)
}

// ---------------------------------------------------------------------------
// Given steps
// ---------------------------------------------------------------------------

// givenStaticParallelApprovalServicePublished publishes the multi-role parallel approval service
// using the embedded static workflow JSON instead of calling the LLM. Enables @deterministic tests.
func (bc *bddContext) givenStaticParallelApprovalServicePublished() error {
	workflowJSON := json.RawMessage(parallelApprovalStaticWorkflowJSON)

	catalog := &ServiceCatalog{
		Name:     "安全与合规服务（确定性）",
		Code:     "security-compliance-pa-det",
		IsActive: true,
	}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create catalog: %w", err)
	}

	priority := &Priority{
		Name:     "普通",
		Code:     "normal-pa-det",
		Value:    3,
		Color:    "#52c41a",
		IsActive: true,
	}
	if err := bc.db.Create(priority).Error; err != nil {
		return fmt.Errorf("create priority: %w", err)
	}
	bc.priority = priority

	// A placeholder agent is required so that bddConfigProvider.DecisionAgentID() returns non-zero
	// if ever needed. For @deterministic tests we call ExecuteDecisionPlan directly.
	agent := &ai.Agent{
		Name:         "并签占位智能体（确定性）",
		Type:         "assistant",
		IsActive:     true,
		Visibility:   "private",
		Strategy:     "react",
		SystemPrompt: "确定性并签测试占位",
		MaxTokens:    2048,
		MaxTurns:     1,
		CreatedBy:    1,
	}
	if err := bc.db.Create(agent).Error; err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	svc := &ServiceDefinition{
		Name:              "多角色并签申请（确定性）",
		Code:              "multi-role-parallel-approval-det",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		WorkflowJSON:      JSONField(workflowJSON),
		CollaborationSpec: parallelApprovalCollaborationSpec,
		AgentID:           &agent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	bc.service = svc
	return nil
}

// givenPositionHasNoActiveMembers deactivates all users assigned to the given position.
// The deactivation is reversible via whenAddActiveMemberToPosition.
func (bc *bddContext) givenPositionHasNoActiveMembers(positionCode string) error {
	pos, ok := bc.positions[positionCode]
	if !ok {
		return fmt.Errorf("position %q not found in test context", positionCode)
	}

	// Find all UserPosition records for this position.
	var ups []org.UserPosition
	if err := bc.db.Where("position_id = ?", pos.ID).Find(&ups).Error; err != nil {
		return fmt.Errorf("query user_positions for position %q: %w", positionCode, err)
	}
	if len(ups) == 0 {
		return nil // no members to deactivate
	}

	userIDs := make([]uint, 0, len(ups))
	for _, up := range ups {
		userIDs = append(userIDs, up.UserID)
	}

	if err := bc.db.Model(&model.User{}).
		Where("id IN ?", userIDs).
		Update("is_active", false).Error; err != nil {
		return fmt.Errorf("deactivate users for position %q: %w", positionCode, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// When steps
// ---------------------------------------------------------------------------

// whenAddActiveMemberToPosition reactivates the specified user (restoring their membership
// in the position they were deactivated from in givenPositionHasNoActiveMembers).
func (bc *bddContext) whenAddActiveMemberToPosition(positionCode, username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in test context", username)
	}
	if err := bc.db.Model(&model.User{}).Where("id = ?", user.ID).
		Update("is_active", true).Error; err != nil {
		return fmt.Errorf("reactivate user %q: %w", username, err)
	}
	return nil
}

// whenRunSmartRecovery calls HandleSmartRecovery synchronously. This triggers both
// the orphaned-decisioning recovery and the suspended-ticket recovery logic.
func (bc *bddContext) whenRunSmartRecovery() error {
	handler := engine.HandleSmartRecovery(bc.db, bc.smartEngine)
	return handler(context.Background(), nil)
}

// whenDeterministicParallelApproveDecision executes a hardcoded parallel approve plan
// for the two parallel approval positions (network_admin + security_admin in dept "it").
// Bypasses LLM for @deterministic tests.
func (bc *bddContext) whenDeterministicParallelApproveDecision() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", TicketStatusDecisioning).Error; err != nil {
		return fmt.Errorf("set ticket to decisioning: %w", err)
	}

	plan := &engine.DecisionPlan{
		NextStepType:  engine.NodeApprove,
		ExecutionMode: "parallel",
		Activities: []engine.DecisionActivity{
			{
				Type:            engine.NodeApprove,
				ParticipantType: "position_department",
				DepartmentCode:  "it",
				PositionCode:    "network_admin",
				Instructions:    "网络管理员并签审批",
			},
			{
				Type:            engine.NodeApprove,
				ParticipantType: "position_department",
				DepartmentCode:  "it",
				PositionCode:    "security_admin",
				Instructions:    "安全管理员并签审批",
			},
		},
		Reasoning:  "deterministic: 并签审批阶段",
		Confidence: 1.0,
	}
	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, plan); err != nil {
		return fmt.Errorf("execute parallel approve plan: %w", err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// whenDeterministicSingleApproveDecision executes a hardcoded single-sign approve plan
// for the specified position in department "it". Bypasses LLM for @deterministic tests.
func (bc *bddContext) whenDeterministicSingleApproveDecision(positionCode string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	// Reset ticket to decisioning so ExecuteDecisionPlan can run.
	if err := bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).
		Update("status", TicketStatusDecisioning).Error; err != nil {
		return fmt.Errorf("set ticket to decisioning: %w", err)
	}

	plan := &engine.DecisionPlan{
		NextStepType:  engine.NodeApprove,
		ExecutionMode: "single",
		Activities: []engine.DecisionActivity{
			{
				Type:            engine.NodeApprove,
				ParticipantType: "position_department",
				DepartmentCode:  "it",
				PositionCode:    positionCode,
				Instructions:    fmt.Sprintf("%s 单签审批", positionCode),
			},
		},
		Reasoning:  fmt.Sprintf("deterministic: 单签审批 %s", positionCode),
		Confidence: 1.0,
	}
	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, plan); err != nil {
		return fmt.Errorf("execute single approve plan for %q: %w", positionCode, err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// whenParallelApprovalRoleRejects is analogous to whenParallelApprovalRoleApproves but
// sets outcome="rejected".
func (bc *bddContext) whenParallelApprovalRoleRejects(positionCode string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	// Find pending parallel activities.
	var activities []TicketActivity
	bc.db.Where("ticket_id = ? AND activity_group_id != '' AND status IN ?",
		bc.ticket.ID, []string{"pending", "in_progress"}).
		Find(&activities)

	if len(activities) == 0 {
		return fmt.Errorf("no pending parallel activities found for ticket %d", bc.ticket.ID)
	}

	// Find activity assigned to positionCode.
	var targetActivity *TicketActivity
	var targetAssignment TicketAssignment
	orgSvc := &testOrgService{db: bc.db}

	for i := range activities {
		var assignment TicketAssignment
		if err := bc.db.Where("activity_id = ?", activities[i].ID).First(&assignment).Error; err != nil {
			continue
		}

		// Match by direct PositionID.
		if assignment.PositionID != nil {
			for code, pos := range bc.positions {
				if pos.ID == *assignment.PositionID && code == positionCode {
					targetActivity = &activities[i]
					targetAssignment = assignment
					break
				}
			}
		}

		// Match by assignee belonging to the position.
		if targetActivity == nil {
			var userID uint
			if assignment.AssigneeID != nil {
				userID = *assignment.AssigneeID
			} else if assignment.UserID != nil {
				userID = *assignment.UserID
			}
			if userID > 0 {
				for _, dept := range bc.departments {
					userIDs, _ := orgSvc.FindUsersByPositionAndDepartment(positionCode, dept.Code)
					for _, uid := range userIDs {
						if uid == userID {
							targetActivity = &activities[i]
							targetAssignment = assignment
							break
						}
					}
					if targetActivity != nil {
						break
					}
				}
			}
		}

		if targetActivity != nil {
			break
		}
	}

	if targetActivity == nil {
		return fmt.Errorf("no pending parallel activity found for position %q in ticket %d", positionCode, bc.ticket.ID)
	}

	// Resolve operator ID.
	var operatorID uint
	if targetAssignment.AssigneeID != nil {
		operatorID = *targetAssignment.AssigneeID
	} else if targetAssignment.UserID != nil {
		operatorID = *targetAssignment.UserID
	}
	if operatorID == 0 {
		operatorID = bc.findFallbackOperator()
	}

	// Claim the activity.
	bc.db.Model(&TicketAssignment{}).
		Where("activity_id = ?", targetActivity.ID).
		Updates(map[string]any{"assignee_id": operatorID, "status": "claimed"})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: targetActivity.ID,
		Outcome:    "rejected",
		OperatorID: operatorID,
	})
	if err != nil {
		bc.lastErr = err
	}

	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// whenCurrentActivityRejected rejects the current single-sign activity.
func (bc *bddContext) whenCurrentActivityRejected() error {
	return bc.progressCurrentActivity("rejected", "驳回意见：测试驳回")
}

// whenBothParallelApprovesConcurrent simulates both parallel approvers completing almost
// simultaneously by running them concurrently in goroutines.
func (bc *bddContext) whenBothParallelApprovesConcurrent() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	positionCodes := []string{"network_admin", "security_admin"}
	errs := make([]error, len(positionCodes))

	var wg sync.WaitGroup
	for i, code := range positionCodes {
		i, code := i, code
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = bc.whenParallelApprovalRoleApproves(code)
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// whenDeterministicCompleteWithoutReset executes a "complete" DecisionPlan WITHOUT first
// resetting the ticket status to in_progress. This preserves rejected_decisioning / approved_decisioning
// context so that ticketStatusForCompletePlan can determine the correct final status.
func (bc *bddContext) whenDeterministicCompleteWithoutReset() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	plan := &engine.DecisionPlan{
		NextStepType: "complete",
		Activities:   []engine.DecisionActivity{},
		Reasoning:    "确定性测试：完结（保留 decisioning 状态）",
		Confidence:   0.95,
	}
	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, plan); err != nil {
		return fmt.Errorf("execute complete plan: %w", err)
	}
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// ---------------------------------------------------------------------------
// Then steps
// ---------------------------------------------------------------------------

// thenExactlyOnePendingApprovalForPosition asserts that exactly one pending (non-parallel)
// approve activity is assigned to the given position.
func (bc *bddContext) thenExactlyOnePendingApprovalForPosition(positionCode string) error {
	pos, ok := bc.positions[positionCode]
	if !ok {
		return fmt.Errorf("position %q not found in test context", positionCode)
	}

	var count int64
	if err := bc.db.Model(&TicketAssignment{}).
		Joins("JOIN itsm_ticket_activities ON itsm_ticket_activities.id = itsm_ticket_assignments.activity_id").
		Where("itsm_ticket_assignments.ticket_id = ? AND itsm_ticket_assignments.position_id = ? AND itsm_ticket_activities.status IN ?",
			bc.ticket.ID, pos.ID, []string{"pending", "in_progress"}).
		Count(&count).Error; err != nil {
		return fmt.Errorf("count pending activities for position %q: %w", positionCode, err)
	}

	if count != 1 {
		return fmt.Errorf("expected exactly 1 pending activity for position %q, got %d", positionCode, count)
	}
	return nil
}

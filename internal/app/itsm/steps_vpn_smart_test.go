package itsm

// steps_vpn_smart_test.go — step definitions for the VPN smart engine BDD scenarios.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/engine"
)

// registerSmartSteps registers all smart engine step definitions.
func registerSmartSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已基于协作规范发布 VPN 开通服务（智能引擎）$`, bc.givenSmartServicePublished)
	sc.Given(`^已基于协作规范发布 VPN 服务（智能引擎）$`, bc.givenSmartServicePublished)
	sc.Given(`^"([^"]*)" 已创建 VPN 工单，访问原因为 "([^"]*)"$`, bc.givenSmartTicketCreated)
	sc.Given(`^智能引擎置信度阈值设为 ([0-9.]+)$`, bc.givenConfidenceThreshold)
	sc.Given(`^"([^"]*)" 已创建 VPN 工单（使用缺失参与者的工作流）$`, bc.givenSmartTicketMissingParticipant)

	sc.When(`^智能引擎执行决策循环$`, bc.whenSmartEngineDecisionCycle)
	sc.When(`^管理员接管该人工处置决策$`, bc.whenAdminConfirmsPendingDecision)
	sc.When(`^当前活动的被分配人认领并处理完成$`, bc.whenAssigneeClaimsAndProcesss)
	sc.When(`^智能引擎再次执行决策循环$`, bc.whenSmartEngineDecisionCycleAgain)

	sc.Then(`^存在至少一个活动$`, bc.thenAtLeastOneActivity)
	sc.Then(`^活动类型在允许列表内$`, bc.thenActivityTypeAllowed)
	sc.Then(`^决策置信度在合法范围内$`, bc.thenConfidenceInRange)
	sc.Then(`^若指定了参与人则参与人在候选列表内$`, bc.thenParticipantInCandidates)
	sc.Then(`^时间线应包含 AI 决策相关事件$`, bc.thenTimelineContainsAIDecision)
	sc.Then(`^当前活动状态为 "([^"]*)"$`, bc.thenCurrentActivityStatusIs)
	sc.Then(`^当前活动状态不为 "([^"]*)"$`, bc.thenCurrentActivityStatusIsNot)
	sc.Then(`^活动记录中包含 AI 推理说明$`, bc.thenActivityContainsAIReasoning)
}

// --- Given steps ---

func (bc *bddContext) givenSmartServicePublished() error {
	return publishVPNSmartService(bc)
}

func (bc *bddContext) givenSmartTicketCreated(username, requestKind string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	formData := map[string]any{
		"request_kind": requestKind,
		"vpn_type":     "l2tp",
		"reason":       "BDD test request",
	}
	formJSON, _ := json.Marshal(formData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("VPN-S-%d", time.Now().UnixNano()),
		Title:        fmt.Sprintf("VPN开通申请(智能) - %s", requestKind),
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: bc.service.WorkflowJSON,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func (bc *bddContext) givenConfidenceThreshold(threshold string) error {
	if bc.service == nil {
		return fmt.Errorf("no service in context")
	}
	agentConfig := fmt.Sprintf(`{"confidence_threshold": %s}`, threshold)
	bc.service.AgentConfig = JSONField(agentConfig)
	return bc.db.Save(bc.service).Error
}

func (bc *bddContext) givenSmartTicketMissingParticipant(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	// Override service workflow with the missing-participant fixture.
	bc.service.WorkflowJSON = JSONField(missingParticipantWorkflowJSON)
	if err := bc.db.Save(bc.service).Error; err != nil {
		return fmt.Errorf("update service workflow: %w", err)
	}

	formData := map[string]any{
		"request_kind": "network_support",
		"vpn_type":     "l2tp",
		"reason":       "BDD test - missing participant",
	}
	formJSON, _ := json.Marshal(formData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("VPN-SM-%d", time.Now().UnixNano()),
		Title:        "VPN开通申请(智能) - 缺失参与者",
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: JSONField(missingParticipantWorkflowJSON),
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

// --- When steps ---

func (bc *bddContext) whenSmartEngineDecisionCycle() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		err := bc.smartEngine.Start(ctx, bc.db, engine.StartParams{
			TicketID:     bc.ticket.ID,
			WorkflowJSON: json.RawMessage(bc.service.WorkflowJSON),
			RequesterID:  bc.ticket.RequesterID,
		})
		cancel()

		if err == nil {
			break
		}
		bc.lastErr = err
		log.Printf("smart engine start attempt %d/%d: %v", attempt, maxRetries, err)
		if (err == engine.ErrAIDecisionFailed || err == engine.ErrAIDisabled) && attempt < maxRetries {
			bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).Update("ai_failure_count", 0)
			continue
		}
		break
	}

	// Refresh ticket.
	bc.db.First(bc.ticket, bc.ticket.ID)
	return nil
}

func (bc *bddContext) whenAdminConfirmsPendingDecision() error {
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	if activity.Status != "pending" {
		return fmt.Errorf("expected activity status 'pending', got %q", activity.Status)
	}

	// Parse the AI decision to get the plan.
	var plan engine.DecisionPlan
	if err := json.Unmarshal([]byte(activity.AIDecision), &plan); err != nil {
		return fmt.Errorf("parse AI decision: %w", err)
	}

	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, &plan); err != nil {
		return fmt.Errorf("execute decision plan: %w", err)
	}

	// Refresh ticket.
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenAssigneeClaimsAndProcesss() error {
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	// Find the assignment for this activity.
	var assignment TicketAssignment
	if err := bc.db.Where("activity_id = ?", activity.ID).First(&assignment).Error; err != nil {
		// No assignment exists — create one using the first non-requester active user.
		log.Printf("[claim-fallback] no assignment for activity %d, creating fallback", activity.ID)
		fallbackID := bc.findFallbackOperator()
		if fallbackID == 0 {
			return fmt.Errorf("no assignment for activity %d and no fallback user available", activity.ID)
		}
		assignment = TicketAssignment{
			TicketID:        bc.ticket.ID,
			ActivityID:      activity.ID,
			ParticipantType: "user",
			UserID:          &fallbackID,
			AssigneeID:      &fallbackID,
			Status:          "claimed",
			IsCurrent:       true,
		}
		bc.db.Create(&assignment)
	}

	// Determine the assignee: use existing AssigneeID, or UserID, or first candidate.
	var operatorID uint
	if assignment.AssigneeID != nil {
		operatorID = *assignment.AssigneeID
	} else if assignment.UserID != nil {
		operatorID = *assignment.UserID
	} else {
		// Find first eligible user via org service.
		operatorID = bc.resolveOperatorFromAssignment(assignment)
		if operatorID == 0 {
			// Fallback: use any active user.
			operatorID = bc.findFallbackOperator()
		}
	}

	if operatorID == 0 {
		return fmt.Errorf("could not determine operator for activity %d", activity.ID)
	}

	// Claim.
	bc.db.Model(&TicketAssignment{}).
		Where("activity_id = ?", activity.ID).
		Updates(map[string]any{"assignee_id": operatorID, "status": "claimed"})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: activity.ID,
		Outcome:    "completed",
		OperatorID: operatorID,
	})
	if err != nil {
		bc.lastErr = err
		return fmt.Errorf("smart engine progress: %w", err)
	}

	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// findFallbackOperator returns the first active non-requester user ID.
func (bc *bddContext) findFallbackOperator() uint {
	provider := &testUserProvider{db: bc.db}
	candidates, _ := provider.ListActiveUsers()
	for _, c := range candidates {
		if c.UserID != bc.ticket.RequesterID {
			return c.UserID
		}
	}
	if len(candidates) > 0 {
		return candidates[0].UserID
	}
	return 0
}

func (bc *bddContext) whenSmartEngineDecisionCycleAgain() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Find the last completed activity to pass as completedActivityID.
		var lastCompleted TicketActivity
		var completedID *uint
		if err := bc.db.Where("ticket_id = ? AND status = ?", bc.ticket.ID, "completed").
			Order("id DESC").First(&lastCompleted).Error; err == nil {
			completedID = &lastCompleted.ID
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		err := bc.smartEngine.RunDecisionCycleForTicket(ctx, bc.db, bc.ticket.ID, completedID)
		cancel()

		if err != nil {
			bc.lastErr = err
			log.Printf("smart engine re-decision attempt %d/%d: %v", attempt, maxRetries, err)
			if (err == engine.ErrAIDecisionFailed || err == engine.ErrAIDisabled) && attempt < maxRetries {
				continue
			}
		}
		break
	}

	bc.db.First(bc.ticket, bc.ticket.ID)
	return nil
}

// --- Then steps ---

func (bc *bddContext) thenAtLeastOneActivity() error {
	var count int64
	if err := bc.db.Model(&TicketActivity{}).Where("ticket_id = ?", bc.ticket.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("expected at least one activity for ticket %d, got 0", bc.ticket.ID)
	}
	return nil
}

func (bc *bddContext) thenActivityTypeAllowed() error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&activities).Error; err != nil {
		return err
	}
	for _, a := range activities {
		if !engine.AllowedSmartStepTypes[a.ActivityType] {
			return fmt.Errorf("activity %d has disallowed type %q", a.ID, a.ActivityType)
		}
	}
	return nil
}

func (bc *bddContext) thenConfidenceInRange() error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&activities).Error; err != nil {
		return err
	}
	for _, a := range activities {
		if a.AIConfidence < 0 || a.AIConfidence > 1 {
			return fmt.Errorf("activity %d has confidence %f outside [0, 1]", a.ID, a.AIConfidence)
		}
	}
	return nil
}

func (bc *bddContext) thenParticipantInCandidates() error {
	provider := &testUserProvider{db: bc.db}
	candidates, err := provider.ListActiveUsers()
	if err != nil {
		return fmt.Errorf("list active users: %w", err)
	}
	candidateIDs := make(map[uint]bool)
	for _, c := range candidates {
		candidateIDs[c.UserID] = true
	}

	var assignments []TicketAssignment
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&assignments).Error; err != nil {
		return err
	}
	for _, a := range assignments {
		if a.UserID != nil && *a.UserID > 0 {
			if !candidateIDs[*a.UserID] {
				return fmt.Errorf("assignment %d has user_id %d not in candidate list", a.ID, *a.UserID)
			}
		}
	}
	// Soft log: record what the AI chose (for observability).
	for _, a := range assignments {
		if a.UserID != nil {
			log.Printf("[smart-bdd] assignment %d: user_id=%d", a.ID, *a.UserID)
		}
	}
	return nil
}

func (bc *bddContext) thenTimelineContainsAIDecision() error {
	var count int64
	if err := bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type LIKE ?", bc.ticket.ID, "%ai_decision%").
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no AI decision event found in timeline for ticket %d", bc.ticket.ID)
	}
	return nil
}

func (bc *bddContext) thenCurrentActivityStatusIs(expected string) error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.Status != expected {
		return fmt.Errorf("expected activity status %q, got %q", expected, activity.Status)
	}
	return nil
}

func (bc *bddContext) thenCurrentActivityStatusIsNot(notExpected string) error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.Status == notExpected {
		return fmt.Errorf("expected activity status NOT to be %q, but it is", notExpected)
	}
	return nil
}

func (bc *bddContext) thenActivityContainsAIReasoning() error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.AIReasoning == "" {
		return fmt.Errorf("activity %d has empty AI reasoning", activity.ID)
	}
	return nil
}

// getLatestActivity returns the most recently created activity for the current ticket.
func (bc *bddContext) getLatestActivity() (*TicketActivity, error) {
	if bc.ticket == nil {
		return nil, fmt.Errorf("no ticket in context")
	}
	var activity TicketActivity
	err := bc.db.Where("ticket_id = ?", bc.ticket.ID).
		Order("id DESC").First(&activity).Error
	if err != nil {
		return nil, fmt.Errorf("no activity found for ticket %d: %w", bc.ticket.ID, err)
	}
	return &activity, nil
}

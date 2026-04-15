package itsm

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app/itsm/engine"
)

var (
	ErrTicketNotFound   = errors.New("ticket not found")
	ErrTicketTerminal   = errors.New("ticket is in a terminal state and cannot be modified")
	ErrServiceNotActive = errors.New("service is not active")
	ErrActivityNotOwner = errors.New("only the assignee or admin can progress this activity")
	ErrActivityNotWait  = errors.New("signal is only allowed on wait nodes")
	ErrNotApprover      = errors.New("you are not an assigned approver for this activity")
	ErrActivityAlready  = errors.New("activity already completed")
)

type TicketService struct {
	ticketRepo    *TicketRepo
	timelineRepo  *TimelineRepo
	serviceRepo   *ServiceDefRepo
	slaRepo       *SLATemplateRepo
	priorityRepo  *PriorityRepo
	classicEngine *engine.ClassicEngine
	smartEngine   *engine.SmartEngine
}

func NewTicketService(i do.Injector) (*TicketService, error) {
	return &TicketService{
		ticketRepo:    do.MustInvoke[*TicketRepo](i),
		timelineRepo:  do.MustInvoke[*TimelineRepo](i),
		serviceRepo:   do.MustInvoke[*ServiceDefRepo](i),
		slaRepo:       do.MustInvoke[*SLATemplateRepo](i),
		priorityRepo:  do.MustInvoke[*PriorityRepo](i),
		classicEngine: do.MustInvoke[*engine.ClassicEngine](i),
		smartEngine:   do.MustInvoke[*engine.SmartEngine](i),
	}, nil
}

type CreateTicketInput struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ServiceID   uint      `json:"serviceId"`
	PriorityID  uint      `json:"priorityId"`
	FormData    JSONField `json:"formData"`
}

func (s *TicketService) Create(input CreateTicketInput, requesterID uint) (*Ticket, error) {
	// Validate service
	svc, err := s.serviceRepo.FindByID(input.ServiceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceDefNotFound
		}
		return nil, err
	}
	if !svc.IsActive {
		return nil, ErrServiceNotActive
	}

	// Validate priority
	if _, err := s.priorityRepo.FindByID(input.PriorityID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPriorityNotFound
		}
		return nil, err
	}

	// For classic engine, validate workflow_json before creating ticket
	if svc.EngineType == "classic" {
		if len(svc.WorkflowJSON) == 0 {
			return nil, errors.New("服务未配置工作流")
		}
		if errs := engine.ValidateWorkflow(json.RawMessage(svc.WorkflowJSON)); len(errs) > 0 {
			return nil, errors.New("工作流校验失败: " + errs[0].Message)
		}
	}

	// Generate ticket code
	code, err := s.ticketRepo.NextCode()
	if err != nil {
		return nil, err
	}

	ticket := &Ticket{
		Code:        code,
		Title:       input.Title,
		Description: input.Description,
		ServiceID:   input.ServiceID,
		EngineType:  svc.EngineType,
		Status:      TicketStatusPending,
		PriorityID:  input.PriorityID,
		RequesterID: requesterID,
		Source:      TicketSourceCatalog,
		FormData:    input.FormData,
		SLAStatus:   SLAStatusOnTrack,
	}

	// Snapshot workflow_json for classic engine
	if svc.EngineType == "classic" {
		ticket.WorkflowJSON = svc.WorkflowJSON
	}

	// Calculate SLA deadlines
	if svc.SLAID != nil {
		sla, err := s.slaRepo.FindByID(*svc.SLAID)
		if err == nil {
			now := time.Now()
			responseDeadline := now.Add(time.Duration(sla.ResponseMinutes) * time.Minute)
			resolutionDeadline := now.Add(time.Duration(sla.ResolutionMinutes) * time.Minute)
			ticket.SLAResponseDeadline = &responseDeadline
			ticket.SLAResolutionDeadline = &resolutionDeadline
		}
	}

	// Create in transaction
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		// Record timeline
		tl := &TicketTimeline{
			TicketID:   ticket.ID,
			OperatorID: requesterID,
			EventType:  "ticket_created",
			Message:    "工单已创建",
		}
		if err := tx.Create(tl).Error; err != nil {
			return err
		}

		// Start engine workflow
		switch svc.EngineType {
		case "classic":
			return s.classicEngine.Start(context.Background(), tx, engine.StartParams{
				TicketID:     ticket.ID,
				WorkflowJSON: json.RawMessage(ticket.WorkflowJSON),
				RequesterID:  requesterID,
			})
		case "smart":
			return s.smartEngine.Start(context.Background(), tx, engine.StartParams{
				TicketID:    ticket.ID,
				RequesterID: requesterID,
			})
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticket.ID)
}

// Progress advances a workflow ticket. The operator must be the assignee or have admin privileges.
func (s *TicketService) Progress(ticketID uint, activityID uint, outcome string, result json.RawMessage, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	eng := s.engineFor(t.EngineType)
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, engine.ProgressParams{
			TicketID:   ticketID,
			ActivityID: activityID,
			Outcome:    outcome,
			Result:     result,
			OperatorID: operatorID,
		})
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// Signal triggers a wait node's continuation from an external source.
func (s *TicketService) Signal(ticketID uint, activityID uint, outcome string, data json.RawMessage, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	// Verify the activity is a wait node and is pending
	var activity TicketActivity
	if err := s.ticketRepo.DB().First(&activity, activityID).Error; err != nil {
		return nil, engine.ErrActivityNotFound
	}
	if activity.ActivityType != engine.NodeWait {
		return nil, ErrActivityNotWait
	}
	if activity.Status != engine.ActivityPending && activity.Status != engine.ActivityInProgress {
		return nil, engine.ErrActivityNotActive
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		return s.classicEngine.Progress(context.Background(), tx, engine.ProgressParams{
			TicketID:   ticketID,
			ActivityID: activityID,
			Outcome:    outcome,
			Result:     data,
			OperatorID: operatorID,
		})
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// GetActivities returns all activities for a ticket.
func (s *TicketService) GetActivities(ticketID uint) ([]TicketActivity, error) {
	var activities []TicketActivity
	if err := s.ticketRepo.DB().Where("ticket_id = ?", ticketID).Order("id ASC").Find(&activities).Error; err != nil {
		return nil, err
	}
	return activities, nil
}

func (s *TicketService) Get(id uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	return t, nil
}

func (s *TicketService) List(params TicketListParams) ([]Ticket, int64, error) {
	return s.ticketRepo.List(params)
}

func (s *TicketService) Mine(requesterID uint, status string, page, pageSize int) ([]Ticket, int64, error) {
	params := TicketListParams{
		RequesterID: &requesterID,
		Status:      status,
		Page:        page,
		PageSize:    pageSize,
	}
	return s.ticketRepo.List(params)
}

func (s *TicketService) Todo(assigneeID uint, page, pageSize int) ([]Ticket, int64, error) {
	return s.ticketRepo.ListTodo(assigneeID, page, pageSize)
}

func (s *TicketService) History(params HistoryListParams) ([]Ticket, int64, error) {
	return s.ticketRepo.ListHistory(params)
}

func (s *TicketService) Assign(id uint, assigneeID uint, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"assignee_id": assigneeID,
			"status":      TicketStatusInProgress,
		}
		if err := s.ticketRepo.UpdateInTx(tx, id, updates); err != nil {
			return err
		}
		tl := &TicketTimeline{
			TicketID:   id,
			OperatorID: operatorID,
			EventType:  "ticket_assigned",
			Message:    "工单已指派",
		}
		return s.timelineRepo.CreateInTx(tx, tl)
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(id)
}

func (s *TicketService) Complete(id uint, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	now := time.Now()
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":      TicketStatusCompleted,
			"finished_at": now,
		}
		if err := s.ticketRepo.UpdateInTx(tx, id, updates); err != nil {
			return err
		}
		tl := &TicketTimeline{
			TicketID:   id,
			OperatorID: operatorID,
			EventType:  "ticket_completed",
			Message:    "工单已完成",
		}
		return s.timelineRepo.CreateInTx(tx, tl)
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(id)
}

type CancelTicketInput struct {
	Reason string `json:"reason"`
}

func (s *TicketService) Cancel(id uint, reason string, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	// For engine-managed tickets, use engine's Cancel to properly clean up activities
	if t.EngineType == "classic" || t.EngineType == "smart" {
		eng := s.engineFor(t.EngineType)
		if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
			return eng.Cancel(context.Background(), tx, engine.CancelParams{
				TicketID:   id,
				Reason:     reason,
				OperatorID: operatorID,
			})
		}); err != nil {
			return nil, err
		}
		return s.ticketRepo.FindByID(id)
	}

	// Manual mode: original cancel logic
	now := time.Now()
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"status":      TicketStatusCancelled,
			"finished_at": now,
		}
		if err := s.ticketRepo.UpdateInTx(tx, id, updates); err != nil {
			return err
		}
		tl := &TicketTimeline{
			TicketID:   id,
			OperatorID: operatorID,
			EventType:  "ticket_cancelled",
			Message:    "工单已取消: " + reason,
		}
		return s.timelineRepo.CreateInTx(tx, tl)
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(id)
}

// engineFor returns the WorkflowEngine for the given engine type.
func (s *TicketService) engineFor(engineType string) engine.WorkflowEngine {
	if engineType == "smart" {
		return s.smartEngine
	}
	return s.classicEngine
}

// --- Smart engine override operations ---

// ConfirmActivity confirms a pending_approval activity and executes the AI decision plan.
func (s *TicketService) ConfirmActivity(ticketID uint, activityID uint, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		var activity TicketActivity
		if err := tx.First(&activity, activityID).Error; err != nil {
			return engine.ErrActivityNotFound
		}
		if activity.Status != engine.ActivityPendingApproval {
			return errors.New("activity is not pending approval")
		}

		// Parse the stored decision plan
		var plan engine.DecisionPlan
		if err := json.Unmarshal([]byte(activity.AIDecision), &plan); err != nil {
			return errors.New("stored decision plan is invalid")
		}

		// Mark activity as confirmed and completed
		now := time.Now()
		if err := tx.Model(&TicketActivity{}).Where("id = ?", activityID).Updates(map[string]any{
			"status":      engine.ActivityCompleted,
			"finished_at": now,
		}).Error; err != nil {
			return err
		}

		// Record timeline
		tl := &TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "ai_decision_confirmed",
			Message:    "人工确认 AI 决策",
		}
		if err := tx.Create(tl).Error; err != nil {
			return err
		}

		// Execute the decision plan
		return s.smartEngine.ExecuteConfirmedPlan(tx, ticketID, &plan)
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// RejectActivity rejects a pending_approval activity and discards the AI decision plan.
func (s *TicketService) RejectActivity(ticketID uint, activityID uint, reason string, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		var activity TicketActivity
		if err := tx.First(&activity, activityID).Error; err != nil {
			return engine.ErrActivityNotFound
		}
		if activity.Status != engine.ActivityPendingApproval {
			return errors.New("activity is not pending approval")
		}

		now := time.Now()
		if err := tx.Model(&TicketActivity{}).Where("id = ?", activityID).Updates(map[string]any{
			"status":        engine.ActivityRejected,
			"finished_at":   now,
			"overridden_by": operatorID,
		}).Error; err != nil {
			return err
		}

		msg := "人工拒绝 AI 决策"
		if reason != "" {
			msg += ": " + reason
		}
		tl := &TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "ai_decision_rejected",
			Message:    msg,
		}
		return tx.Create(tl).Error
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// OverrideJump cancels the current activity and creates a new one of the specified type.
func (s *TicketService) OverrideJump(ticketID uint, activityType string, assigneeID *uint, reason string, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		// Cancel current active activities
		now := time.Now()
		tx.Model(&TicketActivity{}).
			Where("ticket_id = ? AND status IN ?", ticketID,
				[]string{engine.ActivityPending, engine.ActivityPendingApproval, engine.ActivityInProgress}).
			Updates(map[string]any{
				"status":        engine.ActivityCancelled,
				"finished_at":   now,
				"overridden_by": operatorID,
			})

		// Create new activity
		act := &TicketActivity{
			TicketID:     ticketID,
			Name:         "人工跳转: " + activityType,
			ActivityType: activityType,
			Status:       engine.ActivityPending,
			OverriddenBy: &operatorID,
		}
		act.StartedAt = &now
		if err := tx.Create(act).Error; err != nil {
			return err
		}

		updates := map[string]any{"current_activity_id": act.ID}
		if assigneeID != nil && *assigneeID > 0 {
			updates["assignee_id"] = *assigneeID
			// Create assignment
			assignment := &TicketAssignment{
				TicketID:        ticketID,
				ActivityID:      act.ID,
				ParticipantType: "user",
				UserID:          assigneeID,
				AssigneeID:      assigneeID,
				Status:          "pending",
				IsCurrent:       true,
			}
			if err := tx.Create(assignment).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
			return err
		}

		tl := &TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &act.ID,
			OperatorID: operatorID,
			EventType:  "override_jump",
			Message:    "人工强制跳转至 " + activityType + ": " + reason,
		}
		return tx.Create(tl).Error
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// OverrideReassign changes the assignee of the current activity.
func (s *TicketService) OverrideReassign(ticketID uint, activityID uint, newAssigneeID uint, reason string, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		// Update assignment
		tx.Model(&TicketAssignment{}).
			Where("ticket_id = ? AND activity_id = ? AND is_current = ?", ticketID, activityID, true).
			Updates(map[string]any{
				"assignee_id": newAssigneeID,
			})

		// Update ticket assignee
		if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).
			Update("assignee_id", newAssigneeID).Error; err != nil {
			return err
		}

		tl := &TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "override_reassign",
			Message:    "改派处理人: " + reason,
		}
		return tx.Create(tl).Error
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// RetryAI resets ai_failure_count and triggers a new decision cycle.
func (s *TicketService) RetryAI(ticketID uint, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}
	if t.EngineType != "smart" {
		return nil, errors.New("retry-ai is only available for smart engine tickets")
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		// Reset failure count
		if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).
			Update("ai_failure_count", 0).Error; err != nil {
			return err
		}

		tl := &TicketTimeline{
			TicketID:   ticketID,
			OperatorID: operatorID,
			EventType:  "ai_retry",
			Message:    "重新启用 AI 决策",
		}
		if err := tx.Create(tl).Error; err != nil {
			return err
		}

		// Trigger new decision cycle via async task
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":             ticketID,
			"completed_activity_id": nil,
		})
		return s.smartEngine.SubmitProgressTask(payload)
	}); err != nil {
		return nil, err
	}
	return s.ticketRepo.FindByID(ticketID)
}

// --- Approval operations ---

// Approvals returns pending approval activities assigned to the given user.
func (s *TicketService) Approvals(userID uint, page, pageSize int) ([]ApprovalItem, int64, error) {
	// For now, only match by direct userID. Org App integration (positionIDs, deptIDs) can be added later.
	return s.ticketRepo.ListApprovals(userID, nil, nil, page, pageSize)
}

// ApprovalCount returns the count of pending approval activities for the given user.
func (s *TicketService) ApprovalCount(userID uint) (int64, error) {
	return s.ticketRepo.CountApprovals(userID, nil, nil)
}

// ApproveActivity approves a pending approval activity and advances the workflow.
func (s *TicketService) ApproveActivity(ticketID uint, activityID uint, operatorID uint) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	// Verify the activity is an approve type and is active
	var activity TicketActivity
	if err := s.ticketRepo.DB().First(&activity, activityID).Error; err != nil {
		return nil, engine.ErrActivityNotFound
	}
	if activity.ActivityType != engine.NodeApprove {
		return nil, errors.New("activity is not an approval step")
	}
	if activity.Status != engine.ActivityPending && activity.Status != engine.ActivityInProgress {
		return nil, ErrActivityAlready
	}

	// Verify the operator is an assigned approver
	if err := s.verifyApprover(activityID, operatorID); err != nil {
		return nil, err
	}

	// Use engine Progress to advance the workflow
	eng := s.engineFor(t.EngineType)
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		// Record approval timeline
		tl := &TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "activity_approved",
			Message:    "审批通过",
		}
		if err := tx.Create(tl).Error; err != nil {
			return err
		}

		return eng.Progress(context.Background(), tx, engine.ProgressParams{
			TicketID:   ticketID,
			ActivityID: activityID,
			Outcome:    "approve",
			OperatorID: operatorID,
		})
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// DenyActivity denies/rejects a pending approval activity and advances the workflow.
func (s *TicketService) DenyActivity(ticketID uint, activityID uint, operatorID uint, reason string) (*Ticket, error) {
	t, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if t.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	var activity TicketActivity
	if err := s.ticketRepo.DB().First(&activity, activityID).Error; err != nil {
		return nil, engine.ErrActivityNotFound
	}
	if activity.ActivityType != engine.NodeApprove {
		return nil, errors.New("activity is not an approval step")
	}
	if activity.Status != engine.ActivityPending && activity.Status != engine.ActivityInProgress {
		return nil, ErrActivityAlready
	}

	if err := s.verifyApprover(activityID, operatorID); err != nil {
		return nil, err
	}

	eng := s.engineFor(t.EngineType)
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		msg := "审批驳回"
		if reason != "" {
			msg += ": " + reason
		}
		tl := &TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "activity_denied",
			Message:    msg,
			Reasoning:  reason,
		}
		if err := tx.Create(tl).Error; err != nil {
			return err
		}

		return eng.Progress(context.Background(), tx, engine.ProgressParams{
			TicketID:   ticketID,
			ActivityID: activityID,
			Outcome:    "reject",
			OperatorID: operatorID,
		})
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

// verifyApprover checks that the operator is assigned to the given activity.
func (s *TicketService) verifyApprover(activityID uint, operatorID uint) error {
	var count int64
	s.ticketRepo.DB().Model(&TicketAssignment{}).
		Where("activity_id = ? AND (user_id = ? OR assignee_id = ?)", activityID, operatorID, operatorID).
		Count(&count)
	if count == 0 {
		return ErrNotApprover
	}
	return nil
}

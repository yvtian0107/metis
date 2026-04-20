package itsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"metis/internal/app"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/tools"
)

var (
	ErrTicketNotFound       = errors.New("ticket not found")
	ErrTicketTerminal       = errors.New("ticket is in a terminal state and cannot be modified")
	ErrServiceNotActive     = errors.New("service is not active")
	ErrActivityNotOwner     = errors.New("only the assignee or admin can progress this activity")
	ErrActivityNotWait      = errors.New("signal is only allowed on wait nodes")
	ErrNotApprover          = errors.New("you are not an assigned approver for this activity")
	ErrActivityAlready      = errors.New("activity already completed")
	ErrSLAAlreadyPaused     = errors.New("SLA is already paused")
	ErrSLANotPaused         = errors.New("SLA is not paused")
	ErrAssignmentNotFound   = errors.New("assignment not found")
	ErrAssignmentNotPending = errors.New("assignment is not in pending status")
	ErrNoActiveAssignment   = errors.New("no active pending assignment for this activity")
	ErrNotRequester         = errors.New("only the ticket requester can withdraw")
	ErrTicketClaimed        = errors.New("ticket has been claimed and cannot be withdrawn")
)

type TicketService struct {
	ticketRepo    *TicketRepo
	timelineRepo  *TimelineRepo
	serviceRepo   *ServiceDefRepo
	slaRepo       *SLATemplateRepo
	priorityRepo  *PriorityRepo
	classicEngine *engine.ClassicEngine
	smartEngine   *engine.SmartEngine
	orgResolver   app.OrgResolver // nil when Org App not installed
}

func NewTicketService(i do.Injector) (*TicketService, error) {
	svc := &TicketService{
		ticketRepo:    do.MustInvoke[*TicketRepo](i),
		timelineRepo:  do.MustInvoke[*TimelineRepo](i),
		serviceRepo:   do.MustInvoke[*ServiceDefRepo](i),
		slaRepo:       do.MustInvoke[*SLATemplateRepo](i),
		priorityRepo:  do.MustInvoke[*PriorityRepo](i),
		classicEngine: do.MustInvoke[*engine.ClassicEngine](i),
		smartEngine:   do.MustInvoke[*engine.SmartEngine](i),
	}
	// Optional: resolve OrgResolver if Org App is installed
	resolver, err := do.InvokeAs[app.OrgResolver](i)
	if err == nil && resolver != nil {
		svc.orgResolver = resolver
		slog.Info("ITSM TicketService: OrgResolver available for multi-dimensional participant matching")
	}
	return svc, nil
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
			startParams := engine.StartParams{
				TicketID:     ticket.ID,
				WorkflowJSON: json.RawMessage(ticket.WorkflowJSON),
				RequesterID:  requesterID,
			}
			// Load start form schema from inline IntakeFormSchema for variable binding
			if len(svc.IntakeFormSchema) > 0 {
				startParams.StartFormSchema = string(svc.IntakeFormSchema)
				startParams.StartFormData = string(ticket.FormData)
			}
			return s.classicEngine.Start(context.Background(), tx, startParams)
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

// CreateFromAgent creates a ticket from an AI agent session, using full TicketService processing
// (validation, SLA, engine start, timeline). This ensures agent-created tickets are identical to
// UI-created tickets in terms of lifecycle processing.
func (s *TicketService) CreateFromAgent(ctx context.Context, req tools.AgentTicketRequest) (*tools.AgentTicketResult, error) {
	// Resolve default priority (lowest value = highest priority)
	defaultPriority, err := s.priorityRepo.FindDefaultActive()
	if err != nil {
		return nil, fmt.Errorf("no active priority found: %w", err)
	}

	formJSON, _ := json.Marshal(req.FormData)
	input := CreateTicketInput{
		Title:       req.Summary,
		Description: req.Summary,
		ServiceID:   req.ServiceID,
		PriorityID:  defaultPriority.ID,
		FormData:    JSONField(formJSON),
	}

	ticket, err := s.Create(input, req.UserID)
	if err != nil {
		return nil, err
	}

	// Update source and agent session binding
	updates := map[string]any{
		"source":           TicketSourceAgent,
		"agent_session_id": req.SessionID,
	}
	if err := s.ticketRepo.DB().Model(&Ticket{}).Where("id = ?", ticket.ID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update ticket source: %w", err)
	}

	return &tools.AgentTicketResult{
		TicketID:   ticket.ID,
		TicketCode: ticket.Code,
		Status:     ticket.Status,
	}, nil
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
		return s.engineFor(t.EngineType).Progress(context.Background(), tx, engine.ProgressParams{
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
func (s *TicketService) GetActivities(ticketID uint, operatorID uint) ([]TicketActivity, error) {
	var activities []TicketActivity
	if err := s.ticketRepo.DB().Where("ticket_id = ?", ticketID).Order("id ASC").Find(&activities).Error; err != nil {
		return nil, err
	}
	for i := range activities {
		activities[i].CanAct = false
		if activities[i].Status == engine.ActivityPendingApproval {
			canAct, err := s.canActOnPendingApproval(activities[i].ID, operatorID)
			if err != nil {
				return nil, err
			}
			activities[i].CanAct = canAct
		}
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

func (s *TicketService) Todo(userID uint, keyword, status string, page, pageSize int) ([]Ticket, int64, error) {
	params := TodoListParams{
		UserID:   userID,
		Keyword:  keyword,
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	}
	if s.orgResolver != nil {
		if posIDs, err := s.orgResolver.GetUserPositionIDs(userID); err == nil {
			params.PositionIDs = posIDs
		}
		if deptIDs, err := s.orgResolver.GetUserDepartmentIDs(userID); err == nil {
			params.DeptIDs = deptIDs
		}
	}
	return s.ticketRepo.ListTodo(params)
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

// TransferInput is the request body for task transfer.
type TransferInput struct {
	ActivityID   uint `json:"activityId" binding:"required"`
	TargetUserID uint `json:"targetUserId" binding:"required"`
}

// DelegateInput is the request body for task delegation.
type DelegateInput struct {
	ActivityID   uint `json:"activityId" binding:"required"`
	TargetUserID uint `json:"targetUserId" binding:"required"`
}

// ClaimInput is the request body for task claim.
type ClaimInput struct {
	ActivityID uint `json:"activityId" binding:"required"`
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

// Withdraw allows the ticket requester to withdraw their ticket before it has been claimed.
func (s *TicketService) Withdraw(id uint, reason string, operatorID uint) (*Ticket, error) {
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
	if t.RequesterID != operatorID {
		return nil, ErrNotRequester
	}

	// Check if any assignment has been claimed.
	var claimedCount int64
	s.ticketRepo.DB().Model(&TicketAssignment{}).
		Where("ticket_id = ? AND claimed_at IS NOT NULL", id).
		Count(&claimedCount)
	if claimedCount > 0 {
		return nil, ErrTicketClaimed
	}

	// Delegate to engine for proper cleanup.
	msg := "工单已撤回"
	if reason != "" {
		msg = "工单已撤回: " + reason
	}
	eng := s.engineFor(t.EngineType)
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		return eng.Cancel(context.Background(), tx, engine.CancelParams{
			TicketID:   id,
			Reason:     reason,
			OperatorID: operatorID,
			EventType:  "withdrawn",
			Message:    msg,
		})
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

// ConfirmActivity confirms a pending_approval activity and triggers the next decision cycle.
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
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&activity, activityID).Error; err != nil {
			return engine.ErrActivityNotFound
		}
		if activity.TicketID != ticketID {
			return engine.ErrActivityNotFound
		}
		if activity.Status != engine.ActivityPendingApproval {
			return ErrActivityAlready
		}
		canAct, err := s.canActOnPendingApprovalTx(tx, activity, operatorID)
		if err != nil {
			return err
		}
		if !canAct {
			return ErrNotApprover
		}

		// Mark activity as confirmed and completed
		now := time.Now()
		if err := tx.Model(&TicketActivity{}).Where("id = ?", activityID).Updates(map[string]any{
			"status":             engine.ActivityCompleted,
			"transition_outcome": "confirm",
			"finished_at":        now,
		}).Error; err != nil {
			return err
		}
		tx.Model(&TicketAssignment{}).Where("activity_id = ?", activityID).Updates(map[string]any{
			"status":      AssignmentCompleted,
			"finished_at": now,
		})

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

		payload, _ := json.Marshal(engine.SmartProgressPayload{
			TicketID:            ticketID,
			CompletedActivityID: &activityID,
		})
		return s.smartEngine.SubmitProgressTask(payload)
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
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&activity, activityID).Error; err != nil {
			return engine.ErrActivityNotFound
		}
		if activity.TicketID != ticketID {
			return engine.ErrActivityNotFound
		}
		if activity.Status != engine.ActivityPendingApproval {
			return ErrActivityAlready
		}
		canAct, err := s.canActOnPendingApprovalTx(tx, activity, operatorID)
		if err != nil {
			return err
		}
		if !canAct {
			return ErrNotApprover
		}

		now := time.Now()
		if err := tx.Model(&TicketActivity{}).Where("id = ?", activityID).Updates(map[string]any{
			"status":             engine.ActivityRejected,
			"transition_outcome": "reject",
			"decision_reasoning": reason,
			"finished_at":        now,
			"overridden_by":      operatorID,
		}).Error; err != nil {
			return err
		}
		tx.Model(&TicketAssignment{}).Where("activity_id = ?", activityID).Updates(map[string]any{
			"status":      AssignmentCompleted,
			"finished_at": now,
		})

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
			Reasoning:  reason,
		}
		if err := tx.Create(tl).Error; err != nil {
			return err
		}

		payload, _ := json.Marshal(engine.SmartProgressPayload{
			TicketID:            ticketID,
			CompletedActivityID: &activityID,
		})
		return s.smartEngine.SubmitProgressTask(payload)
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
	posIDs, deptIDs := s.resolveUserOrg(userID)
	return s.ticketRepo.ListApprovals(userID, posIDs, deptIDs, page, pageSize)
}

// ApprovalCount returns the count of pending approval activities for the given user.
func (s *TicketService) ApprovalCount(userID uint) (int64, error) {
	posIDs, deptIDs := s.resolveUserOrg(userID)
	return s.ticketRepo.CountApprovals(userID, posIDs, deptIDs)
}

// resolveUserOrg returns the user's position and department IDs if Org App is available.
func (s *TicketService) resolveUserOrg(userID uint) (positionIDs []uint, deptIDs []uint) {
	if s.orgResolver != nil {
		if ids, err := s.orgResolver.GetUserPositionIDs(userID); err == nil {
			positionIDs = ids
		}
		if ids, err := s.orgResolver.GetUserDepartmentIDs(userID); err == nil {
			deptIDs = ids
		}
	}
	return
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

func (s *TicketService) canActOnPendingApproval(activityID uint, operatorID uint) (bool, error) {
	var activity TicketActivity
	if err := s.ticketRepo.DB().First(&activity, activityID).Error; err != nil {
		return false, err
	}
	return s.canActOnPendingApprovalTx(s.ticketRepo.DB(), activity, operatorID)
}

func (s *TicketService) canActOnPendingApprovalTx(tx *gorm.DB, activity TicketActivity, operatorID uint) (bool, error) {
	if activity.Status != engine.ActivityPendingApproval {
		return false, nil
	}

	var count int64
	query := tx.Model(&TicketAssignment{}).
		Where("activity_id = ? AND status = ?", activity.ID, AssignmentPending).
		Where("user_id = ? OR assignee_id = ?", operatorID, operatorID)

	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// SLAPause pauses the SLA clock for a ticket.
func (s *TicketService) SLAPause(ticketID uint, operatorID uint) (*Ticket, error) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.IsTerminal() {
		return nil, ErrTicketTerminal
	}
	if ticket.SLAPausedAt != nil {
		return nil, ErrSLAAlreadyPaused
	}

	now := time.Now()
	ticket.SLAPausedAt = &now
	if err := s.ticketRepo.DB().Save(ticket).Error; err != nil {
		return nil, err
	}

	s.timelineRepo.Create(&TicketTimeline{
		TicketID:   ticketID,
		OperatorID: operatorID,
		EventType:  "sla_paused",
		Message:    "SLA 计时已暂停",
	})

	return ticket, nil
}

// SLAResume resumes the SLA clock for a ticket, extending deadlines by the paused duration.
func (s *TicketService) SLAResume(ticketID uint, operatorID uint) (*Ticket, error) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.IsTerminal() {
		return nil, ErrTicketTerminal
	}
	if ticket.SLAPausedAt == nil {
		return nil, ErrSLANotPaused
	}

	pausedDuration := time.Since(*ticket.SLAPausedAt)

	// Extend deadlines by the paused duration
	if ticket.SLAResponseDeadline != nil {
		extended := ticket.SLAResponseDeadline.Add(pausedDuration)
		ticket.SLAResponseDeadline = &extended
	}
	if ticket.SLAResolutionDeadline != nil {
		extended := ticket.SLAResolutionDeadline.Add(pausedDuration)
		ticket.SLAResolutionDeadline = &extended
	}
	ticket.SLAPausedAt = nil

	if err := s.ticketRepo.DB().Save(ticket).Error; err != nil {
		return nil, err
	}

	s.timelineRepo.Create(&TicketTimeline{
		TicketID:   ticketID,
		OperatorID: operatorID,
		EventType:  "sla_resumed",
		Message:    fmt.Sprintf("SLA 计时已恢复，暂停时长 %s，截止时间已顺延", pausedDuration.Round(time.Second)),
	})

	return ticket, nil
}

// Transfer transfers an assignment from the current assignee to a new user.
func (s *TicketService) Transfer(ticketID, activityID, targetUserID, operatorID uint) (*Ticket, error) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	db := s.ticketRepo.DB()

	// Find the operator's pending assignment for this activity
	var original TicketAssignment
	if err := db.Where("activity_id = ? AND (user_id = ? OR assignee_id = ?) AND status = ?",
		activityID, operatorID, operatorID, AssignmentPending).First(&original).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoActiveAssignment
		}
		return nil, err
	}

	// Mark original as transferred
	db.Model(&original).Update("status", AssignmentTransferred)

	// Create new assignment for target user
	db.Create(&TicketAssignment{
		TicketID:        ticketID,
		ActivityID:      activityID,
		ParticipantType: "user",
		UserID:          &targetUserID,
		AssigneeID:      &targetUserID,
		Status:          AssignmentPending,
		IsCurrent:       original.IsCurrent,
		TransferFrom:    &original.ID,
	})

	// Update ticket assignee
	db.Model(&Ticket{}).Where("id = ?", ticketID).Update("assignee_id", targetUserID)

	s.timelineRepo.Create(&TicketTimeline{
		TicketID:   ticketID,
		ActivityID: &activityID,
		OperatorID: operatorID,
		EventType:  "transfer",
		Message:    fmt.Sprintf("工单已转办给用户 %d", targetUserID),
	})

	return s.ticketRepo.FindByID(ticketID)
}

// Delegate delegates an assignment to another user. After the delegate completes,
// the assignment returns to the original assignee.
func (s *TicketService) Delegate(ticketID, activityID, targetUserID, operatorID uint) (*Ticket, error) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	db := s.ticketRepo.DB()

	// Find the operator's pending assignment for this activity
	var original TicketAssignment
	if err := db.Where("activity_id = ? AND (user_id = ? OR assignee_id = ?) AND status = ?",
		activityID, operatorID, operatorID, AssignmentPending).First(&original).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoActiveAssignment
		}
		return nil, err
	}

	// Mark original as delegated (not completed — it will be restored after delegate finishes)
	db.Model(&original).Update("status", AssignmentDelegated)

	// Create delegated assignment
	db.Create(&TicketAssignment{
		TicketID:        ticketID,
		ActivityID:      activityID,
		ParticipantType: "user",
		UserID:          &targetUserID,
		AssigneeID:      &targetUserID,
		Status:          AssignmentPending,
		IsCurrent:       true,
		DelegatedFrom:   &original.ID,
	})

	s.timelineRepo.Create(&TicketTimeline{
		TicketID:   ticketID,
		ActivityID: &activityID,
		OperatorID: operatorID,
		EventType:  "delegate",
		Message:    fmt.Sprintf("工单已委派给用户 %d", targetUserID),
	})

	return s.ticketRepo.FindByID(ticketID)
}

// Claim allows a user to claim a ticket assignment, marking other pending assignments
// for the same activity as claimed_by_other.
func (s *TicketService) Claim(ticketID, activityID, operatorID uint) (*Ticket, error) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	if ticket.IsTerminal() {
		return nil, ErrTicketTerminal
	}

	db := s.ticketRepo.DB()

	// Find the operator's pending assignment for this activity
	var assignment TicketAssignment
	if err := db.Where("activity_id = ? AND (user_id = ? OR assignee_id = ?) AND status = ?",
		activityID, operatorID, operatorID, AssignmentPending).First(&assignment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoActiveAssignment
		}
		return nil, err
	}

	now := time.Now()

	// Mark the claimer's assignment as claimed
	db.Model(&assignment).Updates(map[string]any{
		"assignee_id": operatorID,
		"claimed_at":  now,
	})

	// Mark other pending assignments for the same activity as claimed_by_other
	db.Model(&TicketAssignment{}).
		Where("activity_id = ? AND status = ? AND id != ?", activityID, AssignmentPending, assignment.ID).
		Update("status", AssignmentClaimedByOther)

	// Update ticket assignee
	db.Model(&Ticket{}).Where("id = ?", ticketID).Update("assignee_id", operatorID)

	s.timelineRepo.Create(&TicketTimeline{
		TicketID:   ticketID,
		ActivityID: &activityID,
		OperatorID: operatorID,
		EventType:  "claim",
		Message:    "用户已抢单",
	})

	return s.ticketRepo.FindByID(ticketID)
}

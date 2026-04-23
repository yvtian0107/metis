package itsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/tools"
)

var (
	ErrTicketNotFound         = errors.New("ticket not found")
	ErrTicketTerminal         = errors.New("ticket is in a terminal state and cannot be modified")
	ErrServiceNotActive       = errors.New("service is not active")
	ErrActivityNotOwner       = errors.New("only the assignee or admin can progress this activity")
	ErrActivityNotWait        = errors.New("signal is only allowed on wait nodes")
	ErrActivityAlready        = errors.New("activity already completed")
	ErrSLAAlreadyPaused       = errors.New("SLA is already paused")
	ErrSLANotPaused           = errors.New("SLA is not paused")
	ErrAssignmentNotFound     = errors.New("assignment not found")
	ErrAssignmentNotPending   = errors.New("assignment is not in pending status")
	ErrNoActiveAssignment     = errors.New("no active pending assignment for this activity")
	ErrNotRequester           = errors.New("only the ticket requester can withdraw")
	ErrTicketClaimed          = errors.New("ticket has been claimed and cannot be withdrawn")
	ErrInvalidProgressOutcome = errors.New("人工节点只能提交 approved 或 rejected")
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
func (s *TicketService) Progress(ticketID uint, activityID uint, outcome string, opinion string, result json.RawMessage, operatorID uint) (*Ticket, error) {
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
	opinion = strings.TrimSpace(opinion)
	if err := s.validateHumanProgress(ticketID, activityID, outcome, opinion, operatorID); err != nil {
		return nil, err
	}

	eng := s.engineFor(t.EngineType)
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, engine.ProgressParams{
			TicketID:   ticketID,
			ActivityID: activityID,
			Outcome:    outcome,
			Result:     result,
			Opinion:    opinion,
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
		if activities[i].Status == engine.ActivityPending || activities[i].Status == engine.ActivityInProgress {
			posIDs, deptIDs := s.resolveUserOrg(operatorID)
			activities[i].CanAct = s.assignmentCanAct(activities[i].ID, operatorID, posIDs, deptIDs)
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

// BuildResponse returns a UI-ready ticket DTO with display names and smart-state
// summary. The base Ticket model intentionally stores IDs only; this method is
// the contract boundary used by handlers.
func (s *TicketService) BuildResponse(t *Ticket, operatorID uint) (TicketResponse, error) {
	responses, err := s.BuildResponses([]Ticket{*t}, operatorID)
	if err != nil {
		return t.ToResponse(), err
	}
	if len(responses) == 0 {
		return t.ToResponse(), nil
	}
	return responses[0], nil
}

func (s *TicketService) BuildResponses(items []Ticket, operatorID uint) ([]TicketResponse, error) {
	responses := make([]TicketResponse, len(items))
	if len(items) == 0 {
		return responses, nil
	}

	serviceIDs := make(map[uint]struct{})
	priorityIDs := make(map[uint]struct{})
	userIDs := make(map[uint]struct{})
	activityIDs := make(map[uint]struct{})
	for i := range items {
		t := &items[i]
		responses[i] = t.ToResponse()
		serviceIDs[t.ServiceID] = struct{}{}
		priorityIDs[t.PriorityID] = struct{}{}
		userIDs[t.RequesterID] = struct{}{}
		if t.AssigneeID != nil {
			userIDs[*t.AssigneeID] = struct{}{}
		}
		if t.CurrentActivityID != nil {
			activityIDs[*t.CurrentActivityID] = struct{}{}
		}
	}

	db := s.ticketRepo.DB()
	serviceNames := map[uint]string{}
	if ids := keysOf(serviceIDs); len(ids) > 0 {
		var rows []struct {
			ID   uint
			Name string
		}
		if err := db.Table("itsm_service_definitions").Where("id IN ?", ids).Select("id, name").Scan(&rows).Error; err != nil {
			return responses, err
		}
		for _, r := range rows {
			serviceNames[r.ID] = r.Name
		}
	}

	type priorityDisplay struct {
		Name  string
		Color string
	}
	priorities := map[uint]priorityDisplay{}
	if ids := keysOf(priorityIDs); len(ids) > 0 {
		var rows []struct {
			ID    uint
			Name  string
			Color string
		}
		if err := db.Table("itsm_priorities").Where("id IN ?", ids).Select("id, name, color").Scan(&rows).Error; err != nil {
			return responses, err
		}
		for _, r := range rows {
			priorities[r.ID] = priorityDisplay{Name: r.Name, Color: r.Color}
		}
	}

	userNames := map[uint]string{}
	if ids := keysOf(userIDs); len(ids) > 0 {
		var rows []struct {
			ID       uint
			Username string
		}
		if err := db.Table("users").Where("id IN ?", ids).Select("id, username").Scan(&rows).Error; err != nil {
			return responses, err
		}
		for _, r := range rows {
			userNames[r.ID] = r.Username
		}
	}

	activities := map[uint]TicketActivity{}
	if ids := keysOf(activityIDs); len(ids) > 0 {
		var rows []TicketActivity
		if err := db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return responses, err
		}
		for _, a := range rows {
			activities[a.ID] = a
		}
	}

	assignments := map[uint]ticketAssignmentDisplay{}
	if ids := keysOf(activityIDs); len(ids) > 0 {
		var rows []ticketAssignmentDisplay
		query := db.Table("itsm_ticket_assignments AS a").
			Joins("LEFT JOIN users AS au ON au.id = a.assignee_id").
			Joins("LEFT JOIN users AS uu ON uu.id = a.user_id").
			Joins("LEFT JOIN positions AS p ON p.id = a.position_id").
			Joins("LEFT JOIN departments AS d ON d.id = a.department_id").
			Where("a.activity_id IN ? AND a.status = ?", ids, AssignmentPending).
			Select(`a.activity_id, a.participant_type,
				COALESCE(au.username, uu.username, '') AS owner_name,
				COALESCE(p.name, '') AS position_name,
				COALESCE(d.name, '') AS department_name`)
		if err := query.Scan(&rows).Error; err != nil {
			return responses, err
		}
		posIDs, deptIDs := s.resolveUserOrg(operatorID)
		for _, r := range rows {
			if r.OwnerName == "" {
				switch {
				case r.PositionName != "" && r.DepartmentName != "":
					r.OwnerName = r.DepartmentName + " / " + r.PositionName
				case r.PositionName != "":
					r.OwnerName = r.PositionName
				case r.DepartmentName != "":
					r.OwnerName = r.DepartmentName
				}
			}
			r.CanAct = s.assignmentCanAct(r.ActivityID, operatorID, posIDs, deptIDs)
			assignments[r.ActivityID] = r
		}
	}

	for i := range responses {
		resp := &responses[i]
		resp.ServiceName = serviceNames[resp.ServiceID]
		if p, ok := priorities[resp.PriorityID]; ok {
			resp.PriorityName = p.Name
			resp.PriorityColor = p.Color
		}
		resp.RequesterName = userNames[resp.RequesterID]
		if resp.AssigneeID != nil {
			resp.AssigneeName = userNames[*resp.AssigneeID]
		}
		if resp.EngineType == "smart" {
			resp.CanOverride = operatorID > 0 && resp.Status != TicketStatusCompleted && resp.Status != TicketStatusCancelled && resp.Status != TicketStatusFailed
			s.populateSmartSummary(resp, activities, assignments)
		}
	}

	return responses, nil
}

type ticketAssignmentDisplay struct {
	ActivityID      uint
	ParticipantType string
	OwnerName       string
	PositionName    string
	DepartmentName  string
	CanAct          bool
}

func (s *TicketService) populateSmartSummary(resp *TicketResponse, activities map[uint]TicketActivity, assignments map[uint]ticketAssignmentDisplay) {
	if resp.Status == TicketStatusCompleted || resp.Status == TicketStatusCancelled || resp.Status == TicketStatusFailed {
		resp.SmartState = "terminal"
		resp.NextStepSummary = "流程已结束"
		return
	}
	if resp.AIFailureCount >= engine.MaxAIFailureCount {
		resp.SmartState = "ai_disabled"
		resp.NextStepSummary = "AI 连续失败，等待人工接管"
		return
	}
	if resp.CurrentActivityID == nil {
		resp.SmartState = "ai_reasoning"
		resp.CurrentOwnerType = "ai"
		resp.CurrentOwnerName = "AI 智能引擎"
		resp.NextStepSummary = "决策中"
		return
	}
	activity, ok := activities[*resp.CurrentActivityID]
	if !ok {
		resp.SmartState = "ai_reasoning"
		resp.CurrentOwnerType = "ai"
		resp.CurrentOwnerName = "AI 智能引擎"
		resp.NextStepSummary = "决策中"
		return
	}
	resp.NextStepSummary = activity.Name
	if resp.NextStepSummary == "" {
		resp.NextStepSummary = activity.ActivityType
	}
	if assignment, ok := assignments[activity.ID]; ok {
		resp.CurrentOwnerType = assignment.ParticipantType
		resp.CurrentOwnerName = assignment.OwnerName
		resp.CanAct = assignment.CanAct
	}
	switch {
	case activity.ActivityType == engine.NodeAction && (activity.Status == engine.ActivityPending || activity.Status == engine.ActivityInProgress):
		resp.SmartState = "action_running"
		resp.CurrentOwnerType = "system"
		resp.CurrentOwnerName = "自动化动作"
	case engine.IsHumanNode(activity.ActivityType) && (activity.Status == engine.ActivityPending || activity.Status == engine.ActivityInProgress):
		resp.SmartState = "waiting_human"
		if resp.CurrentOwnerName == "" {
			resp.CurrentOwnerName = "待分配"
		}
	default:
		resp.SmartState = "ai_decided"
		resp.CurrentOwnerType = "ai"
		resp.CurrentOwnerName = "AI 智能引擎"
	}
}

func (s *TicketService) assignmentCanAct(activityID uint, operatorID uint, positionIDs []uint, deptIDs []uint) bool {
	if operatorID == 0 {
		return false
	}
	var count int64
	s.ticketRepo.DB().Model(&TicketAssignment{}).
		Where("activity_id = ? AND status = ?", activityID, AssignmentPending).
		Where(s.ticketRepo.assignmentOperatorCondition("itsm_ticket_assignments", operatorID, positionIDs, deptIDs)).
		Count(&count)
	return count > 0
}

func keysOf(set map[uint]struct{}) []uint {
	keys := make([]uint, 0, len(set))
	for id := range set {
		if id > 0 {
			keys = append(keys, id)
		}
	}
	return keys
}

func (s *TicketService) List(params TicketListParams) ([]Ticket, int64, error) {
	return s.ticketRepo.List(params)
}

func (s *TicketService) Mine(requesterID uint, keyword, status string, startDate, endDate *time.Time, page, pageSize int) ([]Ticket, int64, error) {
	params := TicketListParams{
		RequesterID: &requesterID,
		Keyword:     keyword,
		Status:      status,
		StartDate:   startDate,
		EndDate:     endDate,
		Page:        page,
		PageSize:    pageSize,
	}
	return s.ticketRepo.List(params)
}

func (s *TicketService) PendingApprovals(operatorID uint, keyword string, page, pageSize int) ([]Ticket, int64, error) {
	positionIDs, departmentIDs := s.resolveUserOrg(operatorID)
	return s.ticketRepo.ListPendingApprovals(TicketApprovalListParams{
		Keyword:  keyword,
		Page:     page,
		PageSize: pageSize,
	}, operatorID, positionIDs, departmentIDs)
}

func (s *TicketService) ApprovalHistory(operatorID uint, keyword string, page, pageSize int) ([]Ticket, int64, error) {
	return s.ticketRepo.ListApprovalHistory(TicketApprovalListParams{
		Keyword:  keyword,
		Page:     page,
		PageSize: pageSize,
	}, operatorID)
}

func (s *TicketService) validateHumanProgress(ticketID uint, activityID uint, outcome string, opinion string, operatorID uint) error {
	if operatorID == 0 {
		return nil
	}

	var activity TicketActivity
	if err := s.ticketRepo.DB().Where("ticket_id = ? AND id = ?", ticketID, activityID).First(&activity).Error; err != nil {
		return engine.ErrActivityNotFound
	}
	if activity.ActivityType != engine.NodeApprove && activity.ActivityType != engine.NodeProcess && activity.ActivityType != engine.NodeForm {
		return nil
	}
	switch strings.TrimSpace(outcome) {
	case "approved", "rejected":
	default:
		return ErrInvalidProgressOutcome
	}
	return nil
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
				[]string{engine.ActivityPending, engine.ActivityInProgress}).
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
func (s *TicketService) RetryAI(ticketID uint, reason string, operatorID uint) (*Ticket, error) {
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

		details, _ := json.Marshal(map[string]string{"reason": reason})
		tl := &TicketTimeline{
			TicketID:   ticketID,
			OperatorID: operatorID,
			EventType:  "ai_retry",
			Message:    "重新启用 AI 决策",
			Details:    JSONField(details),
		}
		if reason != "" {
			tl.Message = "重新启用 AI 决策：" + reason
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
	posIDs, deptIDs := s.resolveUserOrg(operatorID)
	if err := db.Where("activity_id = ? AND status = ?", activityID, AssignmentPending).
		Where(s.ticketRepo.assignmentOperatorCondition("itsm_ticket_assignments", operatorID, posIDs, deptIDs)).
		First(&assignment).Error; err != nil {
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

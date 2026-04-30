package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	. "metis/internal/app/itsm/definition"
	. "metis/internal/app/itsm/domain"
	. "metis/internal/app/itsm/sla"
	"sort"
	"strings"
	"time"

	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"metis/internal/app"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/tools"
	"metis/internal/model"
)

var (
	ErrTicketNotFound            = errors.New("ticket not found")
	ErrTicketTerminal            = errors.New("ticket is in a terminal state and cannot be modified")
	ErrServiceNotActive          = errors.New("service is not active")
	ErrActivityNotOwner          = errors.New("only the assignee or admin can progress this activity")
	ErrActivityNotWait           = errors.New("signal is only allowed on wait nodes")
	ErrActivityAlready           = errors.New("activity already completed")
	ErrSLAAlreadyPaused          = errors.New("SLA is already paused")
	ErrSLANotPaused              = errors.New("SLA is not paused")
	ErrAssignmentNotFound        = errors.New("assignment not found")
	ErrAssignmentNotPending      = errors.New("assignment is not in pending status")
	ErrNoActiveAssignment        = errors.New("no active pending assignment for this activity")
	ErrNotRequester              = errors.New("only the ticket requester can withdraw")
	ErrTicketClaimed             = errors.New("ticket has been claimed and cannot be withdrawn")
	ErrTicketForbidden           = errors.New("ticket access forbidden")
	ErrInvalidProgressOutcome    = errors.New("人工节点只能提交 approved 或 rejected")
	ErrCatalogSubmissionClassic  = errors.New("服务目录提单仅支持经典服务，请通过服务台提交智能服务")
	ErrInvalidRecoveryAction     = errors.New("invalid recovery action")
	ErrRecoveryActionTooFrequent = errors.New("recovery action is too frequent")
	errSubmissionAlreadyExists   = errors.New("service desk submission already exists")
)

const (
	defaultAgentPriorityCode = "P3"
	recoveryDedupWindow      = 15 * time.Second
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
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	ServiceID        uint      `json:"serviceId"`
	ServiceVersionID *uint     `json:"serviceVersionId"`
	PriorityID       uint      `json:"priorityId"`
	FormData         JSONField `json:"formData"`
	Source           string    `json:"source"`
	AgentSessionID   *uint     `json:"agentSessionId"`
}

func (s *TicketService) Create(input CreateTicketInput, requesterID uint) (*Ticket, error) {
	ticket, svc, err := s.prepareTicket(input, requesterID)
	if err != nil {
		return nil, err
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		return s.createTicketLifecycleInTx(context.Background(), tx, ticket, svc, requesterID)
	}); err != nil {
		return nil, err
	}
	if ticket.EngineType == "smart" {
		s.smartEngine.DispatchDecisionAsync(ticket.ID, nil, engine.TriggerReasonTicketCreated)
	}

	return s.ticketRepo.FindByID(ticket.ID)
}

func (s *TicketService) CreateCatalog(input CreateTicketInput, requesterID uint) (*Ticket, error) {
	svc, err := s.serviceRepo.FindByID(input.ServiceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceDefNotFound
		}
		return nil, err
	}
	if svc.EngineType != "classic" {
		return nil, ErrCatalogSubmissionClassic
	}
	input.Source = TicketSourceCatalog
	return s.Create(input, requesterID)
}

func (s *TicketService) prepareTicket(input CreateTicketInput, requesterID uint) (*Ticket, *ServiceDefinition, error) {
	// Validate service
	svc, err := s.serviceRepo.FindByID(input.ServiceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrServiceDefNotFound
		}
		return nil, nil, err
	}
	if !svc.IsActive {
		return nil, nil, ErrServiceNotActive
	}
	var version *ServiceDefinitionVersion
	if input.ServiceVersionID != nil && *input.ServiceVersionID > 0 {
		var snapshot ServiceDefinitionVersion
		if err := s.ticketRepo.DB().
			Where("id = ? AND service_id = ?", *input.ServiceVersionID, input.ServiceID).
			First(&snapshot).Error; err != nil {
			return nil, nil, err
		}
		version = &snapshot
	} else {
		version, err = s.serviceRepo.GetOrCreateRuntimeVersion(input.ServiceID)
		if err != nil {
			return nil, nil, err
		}
	}
	runtimeSvc := *svc
	runtimeSvc.EngineType = version.EngineType
	runtimeSvc.SLAID = version.SLAID
	runtimeSvc.IntakeFormSchema = version.IntakeFormSchema
	runtimeSvc.WorkflowJSON = version.WorkflowJSON
	runtimeSvc.CollaborationSpec = version.CollaborationSpec
	runtimeSvc.AgentID = version.AgentID
	runtimeSvc.AgentConfig = version.AgentConfig
	runtimeSvc.KnowledgeBaseIDs = version.KnowledgeBaseIDs

	// Validate priority
	if _, err := s.priorityRepo.FindByID(input.PriorityID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrPriorityNotFound
		}
		return nil, nil, err
	}

	// For classic engine, validate workflow_json before creating ticket
	if runtimeSvc.EngineType == "classic" {
		if len(runtimeSvc.WorkflowJSON) == 0 {
			return nil, nil, errors.New("服务未配置工作流")
		}
		if errs := engine.ValidateWorkflow(json.RawMessage(runtimeSvc.WorkflowJSON)); len(errs) > 0 {
			return nil, nil, errors.New("工作流校验失败: " + errs[0].Message)
		}
	}

	// Generate ticket code
	code, err := s.ticketRepo.NextCode()
	if err != nil {
		return nil, nil, err
	}
	source := input.Source
	if source == "" {
		source = TicketSourceCatalog
	}

	ticket := &Ticket{
		Code:             code,
		Title:            input.Title,
		Description:      input.Description,
		ServiceID:        input.ServiceID,
		ServiceVersionID: &version.ID,
		EngineType:       runtimeSvc.EngineType,
		Status:           TicketStatusSubmitted,
		PriorityID:       input.PriorityID,
		RequesterID:      requesterID,
		Source:           source,
		AgentSessionID:   input.AgentSessionID,
		FormData:         input.FormData,
		SLAStatus:        SLAStatusOnTrack,
	}

	// Snapshot workflow_json for classic engine
	if runtimeSvc.EngineType == "classic" {
		ticket.WorkflowJSON = runtimeSvc.WorkflowJSON
	}

	// Calculate SLA deadlines from the bound service definition snapshot.
	if runtimeSvc.SLAID != nil {
		var sla SLATemplateResponse
		if len(version.SLATemplateJSON) == 0 {
			return nil, nil, ErrSLATemplateNotFound
		}
		if err := json.Unmarshal(version.SLATemplateJSON, &sla); err != nil {
			return nil, nil, err
		}
		if !sla.IsActive {
			return nil, nil, ErrSLATemplateNotFound
		}
		now := time.Now()
		responseDeadline := now.Add(time.Duration(sla.ResponseMinutes) * time.Minute)
		resolutionDeadline := now.Add(time.Duration(sla.ResolutionMinutes) * time.Minute)
		ticket.SLAResponseDeadline = &responseDeadline
		ticket.SLAResolutionDeadline = &resolutionDeadline
	}
	return ticket, &runtimeSvc, nil
}

func (s *TicketService) createTicketLifecycleInTx(ctx context.Context, tx *gorm.DB, ticket *Ticket, svc *ServiceDefinition, requesterID uint) error {
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
		return s.classicEngine.Start(ctx, tx, startParams)
	case "smart":
		return s.smartEngine.Start(ctx, tx, engine.StartParams{
			TicketID:    ticket.ID,
			RequesterID: requesterID,
		})
	}
	return nil
}

// CreateFromAgent creates a ticket from an AI agent session, using full TicketService processing
// (validation, SLA, engine start, timeline). This ensures agent-created tickets are identical to
// UI-created tickets in terms of lifecycle processing.
func (s *TicketService) CreateFromAgent(ctx context.Context, req tools.AgentTicketRequest) (*tools.AgentTicketResult, error) {
	defaultPriority, err := s.priorityRepo.FindActiveByCode(defaultAgentPriorityCode)
	if err != nil {
		return nil, fmt.Errorf("default agent priority %s is not active: %w", defaultAgentPriorityCode, err)
	}

	formJSON, _ := json.Marshal(req.FormData)
	input := CreateTicketInput{
		Title:            req.Summary,
		Description:      req.Summary,
		ServiceID:        req.ServiceID,
		ServiceVersionID: serviceVersionIDPointer(req.ServiceVersionID),
		PriorityID:       defaultPriority.ID,
		FormData:         JSONField(formJSON),
		Source:           TicketSourceAgent,
		AgentSessionID:   &req.SessionID,
	}

	ticket, err := s.createAgentTicket(ctx, input, req)
	if err != nil {
		return nil, err
	}

	return &tools.AgentTicketResult{
		TicketID:   ticket.ID,
		TicketCode: ticket.Code,
		Status:     ticket.Status,
	}, nil
}

func (s *TicketService) createAgentTicket(ctx context.Context, input CreateTicketInput, req tools.AgentTicketRequest) (*Ticket, error) {
	if req.DraftVersion <= 0 || strings.TrimSpace(req.FieldsHash) == "" {
		ticket, svc, err := s.prepareTicket(input, req.UserID)
		if err != nil {
			return nil, err
		}
		if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
			return s.createTicketLifecycleInTx(ctx, tx, ticket, svc, req.UserID)
		}); err != nil {
			return nil, err
		}
		if ticket.EngineType == "smart" {
			s.smartEngine.DispatchDecisionAsync(ticket.ID, nil, engine.TriggerReasonTicketCreated)
		}
		return s.ticketRepo.FindByID(ticket.ID)
	}

	if ticket, ok, err := s.findSubmittedDraftTicket(req.SessionID, req.DraftVersion, req.FieldsHash); err != nil || ok {
		return ticket, err
	}

	ticket, svc, err := s.prepareTicket(input, req.UserID)
	if err != nil {
		return nil, err
	}

	var created *Ticket
	err = s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		var existing ServiceDeskSubmission
		result := tx.Where("session_id = ? AND draft_version = ? AND fields_hash = ?", req.SessionID, req.DraftVersion, req.FieldsHash).
			Limit(1).
			Find(&existing)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return errSubmissionAlreadyExists
		}

		submission := &ServiceDeskSubmission{
			SessionID:    req.SessionID,
			DraftVersion: req.DraftVersion,
			FieldsHash:   req.FieldsHash,
			RequestHash:  req.RequestHash,
			Status:       "submitting",
			SubmittedBy:  req.UserID,
			SubmittedAt:  time.Now(),
		}
		if err := tx.Create(submission).Error; err != nil {
			if isUniqueConstraintError(err) {
				return errSubmissionAlreadyExists
			}
			return err
		}

		if err := s.createTicketLifecycleInTx(ctx, tx, ticket, svc, req.UserID); err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{
			"session_id":    req.SessionID,
			"draft_version": req.DraftVersion,
			"fields_hash":   req.FieldsHash,
			"request_hash":  req.RequestHash,
			"submitted_by":  req.UserID,
		})
		if err := tx.Create(&TicketTimeline{
			TicketID:   ticket.ID,
			OperatorID: req.UserID,
			EventType:  "draft_submitted",
			Message:    "服务台草稿已确认提交",
			Details:    JSONField(details),
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(submission).Updates(map[string]any{
			"ticket_id": ticket.ID,
			"status":    "submitted",
		}).Error; err != nil {
			return err
		}
		created = ticket
		return nil
	})
	if errors.Is(err, errSubmissionAlreadyExists) {
		ticket, ok, findErr := s.findSubmittedDraftTicket(req.SessionID, req.DraftVersion, req.FieldsHash)
		if findErr != nil {
			return nil, findErr
		}
		if ok {
			return ticket, nil
		}
		return nil, fmt.Errorf("draft submission is already being created")
	}
	if err != nil {
		return nil, err
	}
	if created.EngineType == "smart" {
		s.smartEngine.DispatchDecisionAsync(created.ID, nil, engine.TriggerReasonTicketCreated)
	}
	return s.ticketRepo.FindByID(created.ID)
}

func serviceVersionIDPointer(id uint) *uint {
	if id == 0 {
		return nil
	}
	return &id
}

func (s *TicketService) findSubmittedDraftTicket(sessionID uint, draftVersion int, fieldsHash string) (*Ticket, bool, error) {
	var submission ServiceDeskSubmission
	result := s.ticketRepo.DB().
		Where("session_id = ? AND draft_version = ? AND fields_hash = ?", sessionID, draftVersion, fieldsHash).
		Limit(1).
		Find(&submission)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	if submission.TicketID == 0 || submission.Status != "submitted" {
		return nil, false, nil
	}
	ticket, err := s.ticketRepo.FindByID(submission.TicketID)
	if err != nil {
		return nil, false, err
	}
	return ticket, true, nil
}

func isUniqueConstraintError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}

func isDecisioningStatus(status string) bool {
	switch status {
	case TicketStatusApprovedDecisioning, TicketStatusRejectedDecisioning, TicketStatusDecisioning:
		return true
	default:
		return false
	}
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
	positionIDs, departmentIDs := s.operatorOrgScope(operatorID)
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, engine.ProgressParams{
			TicketID:              ticketID,
			ActivityID:            activityID,
			Outcome:               outcome,
			Result:                result,
			Opinion:               opinion,
			OperatorID:            operatorID,
			OperatorPositionIDs:   positionIDs,
			OperatorDepartmentIDs: departmentIDs,
			OperatorOrgScopeReady: true,
		})
	}); err != nil {
		if errors.Is(err, engine.ErrNoActiveAssignment) {
			return nil, ErrNoActiveAssignment
		}
		return nil, err
	}

	if t.EngineType == "smart" {
		updated, findErr := s.ticketRepo.FindByID(ticketID)
		if findErr == nil && isDecisioningStatus(updated.Status) {
			event := engine.NewActivityDecidedEvent(ticketID, activityID, outcome, operatorID)
			s.smartEngine.DispatchDecisionAsync(event.TicketID, event.CompletedActivityID, event.TriggerReason)
			return updated, nil
		}
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

	positionIDs, departmentIDs := s.operatorOrgScope(operatorID)
	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		return s.engineFor(t.EngineType).Progress(context.Background(), tx, engine.ProgressParams{
			TicketID:              ticketID,
			ActivityID:            activityID,
			Outcome:               outcome,
			Result:                data,
			OperatorID:            operatorID,
			OperatorPositionIDs:   positionIDs,
			OperatorDepartmentIDs: departmentIDs,
			OperatorOrgScopeReady: true,
		})
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

func (s *TicketService) operatorOrgScope(operatorID uint) ([]uint, []uint) {
	if s.orgResolver == nil || operatorID == 0 {
		return nil, nil
	}
	positionIDs, _ := s.orgResolver.GetUserPositionIDs(operatorID)
	departmentIDs, _ := s.orgResolver.GetUserDepartmentIDs(operatorID)
	return positionIDs, departmentIDs
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

func (s *TicketService) GetVisible(id uint, operatorID uint, roleCode string) (*Ticket, error) {
	if err := s.EnsureCanViewTicket(id, operatorID, roleCode); err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *TicketService) EnsureCanViewTicket(ticketID uint, operatorID uint, roleCode string) error {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTicketNotFound
		}
		return err
	}
	if roleCode == model.RoleAdmin {
		return nil
	}
	if operatorID == 0 {
		return ErrTicketForbidden
	}
	if ticket.RequesterID == operatorID {
		return nil
	}

	positionIDs, departmentIDs := s.resolveUserOrg(operatorID)
	ok, err := s.hasActiveAssignmentAccess(ticketID, operatorID, positionIDs, departmentIDs)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	ok, err = s.hasHistoryAssignmentAccess(ticketID, operatorID)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return ErrTicketForbidden
}

func (s *TicketService) hasActiveAssignmentAccess(ticketID uint, operatorID uint, positionIDs []uint, departmentIDs []uint) (bool, error) {
	var count int64
	err := s.ticketRepo.DB().Model(&TicketAssignment{}).
		Where("ticket_id = ? AND status IN ?", ticketID, []string{AssignmentPending, AssignmentInProgress}).
		Where(s.ticketRepo.assignmentOperatorCondition("itsm_ticket_assignments", operatorID, positionIDs, departmentIDs)).
		Count(&count).Error
	return count > 0, err
}

func (s *TicketService) hasHistoryAssignmentAccess(ticketID uint, operatorID uint) (bool, error) {
	var count int64
	err := s.ticketRepo.DB().Model(&TicketAssignment{}).
		Where("ticket_id = ? AND assignee_id = ? AND status IN ?", ticketID, operatorID, []string{AssignmentApproved, AssignmentRejected}).
		Count(&count).Error
	return count > 0, err
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
	ticketIDs := make(map[uint]struct{})
	for i := range items {
		t := &items[i]
		responses[i] = t.ToResponse()
		ticketIDs[t.ID] = struct{}{}
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
	type serviceDisplay struct {
		Name             string
		IntakeFormSchema JSONField
	}
	services := map[uint]serviceDisplay{}
	if ids := keysOf(serviceIDs); len(ids) > 0 {
		var rows []struct {
			ID               uint
			Name             string
			IntakeFormSchema JSONField
		}
		if err := db.Table("itsm_service_definitions").Where("id IN ?", ids).Select("id, name, intake_form_schema").Scan(&rows).Error; err != nil {
			return responses, err
		}
		for _, r := range rows {
			services[r.ID] = serviceDisplay{Name: r.Name, IntakeFormSchema: r.IntakeFormSchema}
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

	assignments, err := s.loadAssignmentDisplays(activityIDs, operatorID)
	if err != nil {
		return responses, err
	}
	lastHumanOutcomes, err := s.loadLastHumanOutcomes(ticketIDs)
	if err != nil {
		return responses, err
	}
	explanationSnapshots, err := s.loadLatestDecisionExplanations(ticketIDs)
	if err != nil {
		return responses, err
	}

	for i := range responses {
		resp := &responses[i]
		resp.StatusLabel = TicketStatusLabel(resp.Status, resp.Outcome)
		resp.StatusTone = TicketStatusTone(resp.Status, resp.Outcome)
		resp.LastHumanOutcome = lastHumanOutcomes[resp.ID]
		resp.DecisioningReason = decisioningReason(resp.Status)
		if service, ok := services[resp.ServiceID]; ok {
			resp.ServiceName = service.Name
			resp.IntakeFormSchema = service.IntakeFormSchema
		}
		if p, ok := priorities[resp.PriorityID]; ok {
			resp.PriorityName = p.Name
			resp.PriorityColor = p.Color
		}
		resp.RequesterName = userNames[resp.RequesterID]
		if resp.AssigneeID != nil {
			resp.AssigneeName = userNames[*resp.AssigneeID]
		}
		if resp.EngineType == "smart" {
			resp.CanOverride = operatorID > 0 && !IsTerminalTicketStatus(resp.Status)
			s.populateSmartSummary(resp, activities, assignments)
			var currentActivity *TicketActivity
			if resp.CurrentActivityID != nil {
				if activity, ok := activities[*resp.CurrentActivityID]; ok {
					currentActivity = &activity
				}
			}
			resp.DecisionExplanation = buildDecisionExplanation(resp, currentActivity, explanationSnapshots[resp.ID])
			resp.RecoveryActions = buildRecoveryActions(resp)
		}
	}

	return responses, nil
}

func buildDecisionExplanation(resp *TicketResponse, activity *TicketActivity, snapshot *DecisionExplanation) *DecisionExplanation {
	explanation := &DecisionExplanation{
		Basis:         "协作规范、workflow_json 与 workflow_context",
		Trigger:       strings.TrimSpace(resp.DecisioningReason),
		Decision:      strings.TrimSpace(resp.StatusLabel),
		NextStep:      strings.TrimSpace(resp.NextStepSummary),
		HumanOverride: "可执行重试、转人工或撤回",
	}
	if snapshot != nil {
		if snapshot.ActivityID != nil && *snapshot.ActivityID > 0 {
			explanation.ActivityID = snapshot.ActivityID
		}
		if value := strings.TrimSpace(snapshot.Basis); value != "" {
			explanation.Basis = value
		}
		if value := strings.TrimSpace(snapshot.Trigger); value != "" {
			explanation.Trigger = value
		}
		if value := strings.TrimSpace(snapshot.Decision); value != "" {
			explanation.Decision = value
		}
		if value := strings.TrimSpace(snapshot.NextStep); value != "" {
			explanation.NextStep = value
		}
		if value := strings.TrimSpace(snapshot.HumanOverride); value != "" {
			explanation.HumanOverride = value
		}
	}
	if activity != nil {
		if explanation.ActivityID == nil {
			explanation.ActivityID = &activity.ID
		}
		if reasoning := strings.TrimSpace(activity.AIReasoning); reasoning != "" {
			if snapshot == nil || strings.TrimSpace(snapshot.Basis) == "" {
				explanation.Basis = reasoning
			}
		}
		if decisionReasoning := strings.TrimSpace(activity.DecisionReasoning); decisionReasoning != "" {
			if snapshot == nil || strings.TrimSpace(snapshot.Decision) == "" {
				explanation.Decision = decisionReasoning
			}
		}
	}
	if explanation.Trigger == "" {
		explanation.Trigger = engine.TriggerReasonAIDecision
	}
	if explanation.Decision == "" {
		explanation.Decision = "等待决策引擎输出"
	}
	if explanation.NextStep == "" {
		explanation.NextStep = "等待下一活动推进"
	}
	return explanation
}

func (s *TicketService) loadLatestDecisionExplanations(ticketIDs map[uint]struct{}) (map[uint]*DecisionExplanation, error) {
	result := map[uint]*DecisionExplanation{}
	ids := keysOf(ticketIDs)
	if len(ids) == 0 {
		return result, nil
	}

	var rows []TicketTimeline
	if err := s.ticketRepo.DB().Model(&TicketTimeline{}).
		Where("ticket_id IN ? AND event_type IN ?", ids, []string{"ai_decision_executed", "ai_decision_pending", "ai_decision_failed", "workflow_completed"}).
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return result, err
	}
	for _, row := range rows {
		if _, exists := result[row.TicketID]; exists {
			continue
		}
		snapshot := parseDecisionExplanationDetail(row.Details)
		if snapshot != nil {
			result[row.TicketID] = snapshot
		}
	}
	return result, nil
}

func parseDecisionExplanationDetail(details JSONField) *DecisionExplanation {
	if len(details) == 0 {
		return nil
	}
	var payload struct {
		DecisionExplanation *DecisionExplanation `json:"decision_explanation"`
	}
	if err := json.Unmarshal(details, &payload); err != nil {
		return nil
	}
	return payload.DecisionExplanation
}

func buildRecoveryActions(resp *TicketResponse) []RecoveryAction {
	if resp == nil || resp.EngineType != "smart" {
		return nil
	}
	actions := []RecoveryAction{}
	if resp.Status == TicketStatusFailed || resp.AIFailureCount > 0 {
		actions = append(actions, RecoveryAction{Code: "retry", Label: "重试决策"})
		actions = append(actions, RecoveryAction{Code: "handoff_human", Label: "转人工处理"})
	}
	if !IsTerminalTicketStatus(resp.Status) {
		actions = append(actions, RecoveryAction{Code: "withdraw", Label: "撤回工单"})
	}
	return actions
}

func decisioningReason(status string) string {
	switch status {
	case TicketStatusApprovedDecisioning:
		return engine.TriggerReasonActivityApprove
	case TicketStatusRejectedDecisioning:
		return engine.TriggerReasonActivityReject
	case TicketStatusDecisioning:
		return engine.TriggerReasonAIDecision
	default:
		return ""
	}
}

func (s *TicketService) loadLastHumanOutcomes(ticketIDs map[uint]struct{}) (map[uint]string, error) {
	result := map[uint]string{}
	ids := keysOf(ticketIDs)
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		TicketID uint
		Outcome  string
	}
	if err := s.ticketRepo.DB().Table("itsm_ticket_activities").
		Where("ticket_id IN ? AND activity_type IN ? AND transition_outcome IN ?", ids,
			[]string{engine.NodeApprove, engine.NodeForm, engine.NodeProcess},
			[]string{TicketOutcomeApproved, TicketOutcomeRejected}).
		Order("finished_at DESC, id DESC").
		Select("ticket_id, transition_outcome AS outcome").
		Scan(&rows).Error; err != nil {
		return result, err
	}
	for _, row := range rows {
		if _, exists := result[row.TicketID]; !exists {
			result[row.TicketID] = row.Outcome
		}
	}
	return result, nil
}

type ticketAssignmentDisplay struct {
	ActivityID      uint
	ParticipantType string
	OwnerName       string
	PositionName    string
	DepartmentName  string
	UserID          *uint
	AssigneeID      *uint
	PositionID      *uint
	DepartmentID    *uint
	CanAct          bool
}

func (s *TicketService) loadAssignmentDisplays(activityIDs map[uint]struct{}, operatorID uint) (map[uint]ticketAssignmentDisplay, error) {
	result := map[uint]ticketAssignmentDisplay{}
	ids := keysOf(activityIDs)
	if len(ids) == 0 {
		return result, nil
	}

	db := s.ticketRepo.DB()
	selects := []string{
		"a.activity_id",
		"a.participant_type",
		"a.user_id",
		"a.assignee_id",
		"a.position_id",
		"a.department_id",
		"COALESCE(au.username, uu.username, '') AS owner_name",
	}
	query := db.Table("itsm_ticket_assignments AS a").
		Joins("LEFT JOIN users AS au ON au.id = a.assignee_id").
		Joins("LEFT JOIN users AS uu ON uu.id = a.user_id").
		Where("a.activity_id IN ? AND a.status = ?", ids, AssignmentPending)

	if db.Migrator().HasTable("positions") {
		query = query.Joins("LEFT JOIN positions AS p ON p.id = a.position_id")
		selects = append(selects, "COALESCE(p.name, '') AS position_name")
	} else {
		selects = append(selects, "'' AS position_name")
	}
	if db.Migrator().HasTable("departments") {
		query = query.Joins("LEFT JOIN departments AS d ON d.id = a.department_id")
		selects = append(selects, "COALESCE(d.name, '') AS department_name")
	} else {
		selects = append(selects, "'' AS department_name")
	}

	var rows []ticketAssignmentDisplay
	if err := query.Select(strings.Join(selects, ", ")).Scan(&rows).Error; err != nil {
		return result, err
	}

	posIDs, deptIDs := s.resolveUserOrg(operatorID)
	for _, r := range rows {
		if r.OwnerName == "" {
			r.OwnerName = assignmentOwnerFallback(r)
		}
		r.CanAct = s.assignmentCanAct(r.ActivityID, operatorID, posIDs, deptIDs)
		result[r.ActivityID] = r
	}
	return result, nil
}

func assignmentOwnerFallback(a ticketAssignmentDisplay) string {
	switch {
	case a.PositionName != "" && a.DepartmentName != "":
		return a.DepartmentName + " / " + a.PositionName
	case a.PositionName != "":
		return a.PositionName
	case a.DepartmentName != "":
		return a.DepartmentName
	case a.PositionID != nil && a.DepartmentID != nil:
		return fmt.Sprintf("部门 #%d / 岗位 #%d", *a.DepartmentID, *a.PositionID)
	case a.PositionID != nil:
		return fmt.Sprintf("岗位 #%d", *a.PositionID)
	case a.DepartmentID != nil:
		return fmt.Sprintf("部门 #%d", *a.DepartmentID)
	default:
		return ""
	}
}

func (s *TicketService) populateSmartSummary(resp *TicketResponse, activities map[uint]TicketActivity, assignments map[uint]ticketAssignmentDisplay) {
	if IsTerminalTicketStatus(resp.Status) {
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
		resp.NextStepSummary = TicketStatusLabel(resp.Status, resp.Outcome)
		return
	}
	activity, ok := activities[*resp.CurrentActivityID]
	if !ok {
		resp.SmartState = "ai_reasoning"
		resp.CurrentOwnerType = "ai"
		resp.CurrentOwnerName = "AI 智能引擎"
		resp.NextStepSummary = TicketStatusLabel(resp.Status, resp.Outcome)
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

const (
	monitorNoActivityBlockAfter = 5 * time.Minute
	monitorHumanWaitRiskAfter   = 60 * time.Minute
	monitorActionRiskAfter      = 15 * time.Minute
	monitorSLADueRiskBefore     = 30 * time.Minute
)

func (s *TicketService) Monitor(params TicketMonitorParams, operatorID uint) (*TicketMonitorResponse, error) {
	if params.OperatorID == 0 {
		params.OperatorID = operatorID
	}
	tickets, err := s.ticketRepo.ListMonitorBase(params)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	completedToday, err := s.ticketRepo.CountMonitorCompletedToday(params, now)
	if err != nil {
		return nil, err
	}

	facts, err := s.loadMonitorFacts(tickets)
	if err != nil {
		return nil, err
	}

	items := make([]TicketMonitorItem, 0, len(tickets))
	summary := TicketMonitorSummary{}
	for i := range tickets {
		ticket := &tickets[i]
		item := TicketMonitorItem{TicketResponse: ticket.ToResponse(), RiskLevel: "normal"}
		s.populateMonitorItem(&item, ticket, facts[ticket.ID], now)
		s.accumulateMonitorSummary(&summary, ticket, &item, now)
		if monitorRiskMatches(params.RiskLevel, item.RiskLevel) && monitorMetricMatches(params.MetricCode, ticket, &item, now) {
			items = append(items, item)
		}
	}
	summary.CompletedTodayTotal = int(completedToday)

	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := monitorRiskRank(items[i].RiskLevel), monitorRiskRank(items[j].RiskLevel)
		if ri != rj {
			return ri < rj
		}
		if items[i].WaitingMinutes != items[j].WaitingMinutes {
			return items[i].WaitingMinutes > items[j].WaitingMinutes
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})

	total := int64(len(items))
	page, pageSize := normalizePage(params.Page, params.PageSize)
	start := (page - 1) * pageSize
	if start >= len(items) {
		items = []TicketMonitorItem{}
	} else {
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		items = items[start:end]
	}
	if len(items) > 0 {
		ticketByID := make(map[uint]Ticket, len(tickets))
		for _, ticket := range tickets {
			ticketByID[ticket.ID] = ticket
		}
		pageTickets := make([]Ticket, 0, len(items))
		itemByID := make(map[uint]*TicketMonitorItem, len(items))
		for i := range items {
			pageTickets = append(pageTickets, ticketByID[items[i].ID])
			itemByID[items[i].ID] = &items[i]
		}
		responses, err := s.BuildResponses(pageTickets, operatorID)
		if err != nil {
			return nil, err
		}
		for _, response := range responses {
			if item := itemByID[response.ID]; item != nil {
				ticket := ticketByID[response.ID]
				item.TicketResponse = response
				s.populateMonitorItem(item, &ticket, facts[response.ID], now)
			}
		}
	}

	return &TicketMonitorResponse{Summary: summary, Items: items, Total: total}, nil
}

type decisionQualityTicketDim struct {
	TicketID      uint
	Status        string
	DimensionID   uint
	DimensionName string
}

type decisionQualityAccumulator struct {
	DimensionType      string
	DimensionID        uint
	DimensionName      string
	ApprovedCount      int64
	RejectedCount      int64
	RetryCount         int64
	LatencyTotalSecond float64
	LatencyCount       int64
	RecoveryAttempt    int64
	RecoverySuccess    int64
}

func (s *TicketService) DecisionQuality(windowDays int, dimension string, serviceID *uint, departmentID *uint) (*DecisionQualityResponse, error) {
	if windowDays <= 0 || windowDays > 180 {
		windowDays = 30
	}
	dimension = strings.TrimSpace(strings.ToLower(dimension))
	if dimension != "department" {
		dimension = "service"
	}

	windowStart := time.Now().Add(-time.Duration(windowDays) * 24 * time.Hour)
	tickets, err := s.loadDecisionQualityTicketDimensions(windowStart, dimension, serviceID, departmentID)
	if err != nil {
		return nil, err
	}
	if len(tickets) == 0 {
		return &DecisionQualityResponse{
			Version:     DecisionQualityMetricVersion,
			WindowDays:  windowDays,
			GeneratedAt: time.Now(),
			Items:       []DecisionQualityItem{},
		}, nil
	}

	ticketIDs := make([]uint, 0, len(tickets))
	ticketToDim := make(map[uint]decisionQualityTicketDim, len(tickets))
	for _, ticket := range tickets {
		ticketIDs = append(ticketIDs, ticket.TicketID)
		ticketToDim[ticket.TicketID] = ticket
	}

	accByDim := make(map[string]*decisionQualityAccumulator)
	getAcc := func(dim decisionQualityTicketDim) *decisionQualityAccumulator {
		key := fmt.Sprintf("%s:%d", dimension, dim.DimensionID)
		if existing, ok := accByDim[key]; ok {
			return existing
		}
		created := &decisionQualityAccumulator{
			DimensionType: dimension,
			DimensionID:   dim.DimensionID,
			DimensionName: dim.DimensionName,
		}
		accByDim[key] = created
		return created
	}

	var activityRows []struct {
		TicketID          uint
		TransitionOutcome string
	}
	if err := s.ticketRepo.DB().Model(&TicketActivity{}).
		Where("ticket_id IN ?", ticketIDs).
		Where("activity_type IN ?", []string{engine.NodeApprove, engine.NodeForm, engine.NodeProcess}).
		Where("transition_outcome IN ?", []string{TicketOutcomeApproved, TicketOutcomeRejected}).
		Where("finished_at >= ?", windowStart).
		Select("ticket_id, transition_outcome").
		Find(&activityRows).Error; err != nil {
		return nil, err
	}
	for _, row := range activityRows {
		dim, ok := ticketToDim[row.TicketID]
		if !ok {
			continue
		}
		acc := getAcc(dim)
		switch row.TransitionOutcome {
		case TicketOutcomeApproved:
			acc.ApprovedCount++
		case TicketOutcomeRejected:
			acc.RejectedCount++
		}
	}

	var timelineRows []TicketTimeline
	if err := s.ticketRepo.DB().Model(&TicketTimeline{}).
		Where("ticket_id IN ? AND created_at >= ?", ticketIDs, windowStart).
		Order("ticket_id ASC, created_at ASC, id ASC").
		Find(&timelineRows).Error; err != nil {
		return nil, err
	}

	type ticketRuntimeStat struct {
		LastTriggerAt *time.Time
		HasRecovery   bool
	}
	runtimeByTicket := make(map[uint]*ticketRuntimeStat, len(ticketIDs))
	for _, row := range timelineRows {
		dim, ok := ticketToDim[row.TicketID]
		if !ok {
			continue
		}
		acc := getAcc(dim)
		rt := runtimeByTicket[row.TicketID]
		if rt == nil {
			rt = &ticketRuntimeStat{}
			runtimeByTicket[row.TicketID] = rt
		}

		switch row.EventType {
		case "activity_completed", "ai_retry", "recovery_retry", "recovery_handoff_human":
			t := row.CreatedAt
			rt.LastTriggerAt = &t
		}

		switch row.EventType {
		case "ai_retry", "recovery_retry":
			acc.RetryCount++
		}
		switch row.EventType {
		case "recovery_retry", "recovery_handoff_human":
			rt.HasRecovery = true
		}

		switch row.EventType {
		case "ai_decision_executed", "ai_decision_pending", "workflow_completed", "ai_decision_failed":
			if rt.LastTriggerAt != nil && row.CreatedAt.After(*rt.LastTriggerAt) {
				acc.LatencyTotalSecond += row.CreatedAt.Sub(*rt.LastTriggerAt).Seconds()
				acc.LatencyCount++
			}
		}
	}

	for ticketID, rt := range runtimeByTicket {
		if !rt.HasRecovery {
			continue
		}
		dim := ticketToDim[ticketID]
		acc := getAcc(dim)
		acc.RecoveryAttempt++
		switch dim.Status {
		case TicketStatusCompleted, TicketStatusRejected, TicketStatusWithdrawn, TicketStatusCancelled:
			acc.RecoverySuccess++
		}
	}

	items := make([]DecisionQualityItem, 0, len(accByDim))
	for _, acc := range accByDim {
		decisionTotal := acc.ApprovedCount + acc.RejectedCount
		approvalRate := 0.0
		rejectionRate := 0.0
		retryRate := 0.0
		if decisionTotal > 0 {
			approvalRate = float64(acc.ApprovedCount) / float64(decisionTotal)
			rejectionRate = float64(acc.RejectedCount) / float64(decisionTotal)
			retryRate = float64(acc.RetryCount) / float64(decisionTotal)
		}
		avgLatency := 0.0
		if acc.LatencyCount > 0 {
			avgLatency = acc.LatencyTotalSecond / float64(acc.LatencyCount)
		}
		recoveryRate := 0.0
		if acc.RecoveryAttempt > 0 {
			recoveryRate = float64(acc.RecoverySuccess) / float64(acc.RecoveryAttempt)
		}
		items = append(items, DecisionQualityItem{
			DimensionType:             acc.DimensionType,
			DimensionID:               acc.DimensionID,
			DimensionName:             acc.DimensionName,
			ApprovalRate:              approvalRate,
			RejectionRate:             rejectionRate,
			RetryRate:                 retryRate,
			AvgDecisionLatencySeconds: avgLatency,
			RecoverySuccessRate:       recoveryRate,
			DecisionCount:             decisionTotal,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].DecisionCount != items[j].DecisionCount {
			return items[i].DecisionCount > items[j].DecisionCount
		}
		return items[i].DimensionID < items[j].DimensionID
	})

	return &DecisionQualityResponse{
		Version:     DecisionQualityMetricVersion,
		WindowDays:  windowDays,
		GeneratedAt: time.Now(),
		Items:       items,
	}, nil
}

func (s *TicketService) loadDecisionQualityTicketDimensions(windowStart time.Time, dimension string, serviceID *uint, departmentID *uint) ([]decisionQualityTicketDim, error) {
	switch dimension {
	case "department":
		return s.loadDecisionQualityByDepartment(windowStart, departmentID)
	default:
		return s.loadDecisionQualityByService(windowStart, serviceID)
	}
}

func (s *TicketService) loadDecisionQualityByService(windowStart time.Time, serviceID *uint) ([]decisionQualityTicketDim, error) {
	query := s.ticketRepo.DB().Table("itsm_tickets AS t").
		Joins("LEFT JOIN itsm_service_definitions AS svc ON svc.id = t.service_id").
		Where("t.deleted_at IS NULL").
		Where("(t.created_at >= ? OR t.updated_at >= ?)", windowStart, windowStart)
	if serviceID != nil && *serviceID > 0 {
		query = query.Where("t.service_id = ?", *serviceID)
	}
	rows := make([]decisionQualityTicketDim, 0)
	if err := query.Select(`
		t.id AS ticket_id,
		t.status AS status,
		t.service_id AS dimension_id,
		COALESCE(svc.name, '服务#' || t.service_id) AS dimension_name
	`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *TicketService) loadDecisionQualityByDepartment(windowStart time.Time, departmentID *uint) ([]decisionQualityTicketDim, error) {
	db := s.ticketRepo.DB()
	query := db.Table("itsm_tickets AS t").
		Where("t.deleted_at IS NULL").
		Where("(t.created_at >= ? OR t.updated_at >= ?)", windowStart, windowStart)
	selectSQL := `
		t.id AS ticket_id,
		t.status AS status,
		0 AS dimension_id,
		'未分配部门' AS dimension_name
	`

	if db.Migrator().HasTable("user_positions") && db.Migrator().HasTable("departments") {
		query = query.Joins(`
			LEFT JOIN (
				SELECT user_id, MIN(department_id) AS department_id
				FROM user_positions
				WHERE department_id IS NOT NULL
				GROUP BY user_id
			) AS ud ON ud.user_id = t.requester_id
		`).
			Joins("LEFT JOIN departments AS dept ON dept.id = ud.department_id")
		selectSQL = `
			t.id AS ticket_id,
			t.status AS status,
			COALESCE(ud.department_id, 0) AS dimension_id,
			COALESCE(dept.name, '未分配部门') AS dimension_name
		`
		if departmentID != nil && *departmentID > 0 {
			query = query.Where("ud.department_id = ?", *departmentID)
		}
	}

	rows := make([]decisionQualityTicketDim, 0)
	if err := query.Select(selectSQL).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
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
			"status":      TicketStatusWaitingHuman,
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
			"outcome":     TicketOutcomeCancelled,
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

		updates := map[string]any{
			"current_activity_id": act.ID,
			"status":              ticketStatusForManualActivity(activityType),
			"outcome":             "",
		}
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
		if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).Updates(map[string]any{
			"assignee_id": newAssigneeID,
			"status":      TicketStatusWaitingHuman,
			"outcome":     "",
		}).Error; err != nil {
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
	return s.retryAI(ticketID, reason, operatorID, true)
}

func (s *TicketService) Recover(ticketID uint, action string, reason string, operatorID uint) (*Ticket, error) {
	action = strings.TrimSpace(strings.ToLower(action))
	switch action {
	case "retry":
		return s.retryAI(ticketID, reason, operatorID, false)
	case "handoff_human":
		return s.handoffHuman(ticketID, reason, operatorID)
	case "withdraw":
		return s.Withdraw(ticketID, reason, operatorID)
	default:
		return nil, ErrInvalidRecoveryAction
	}
}

func (s *TicketService) retryAI(ticketID uint, reason string, operatorID uint, legacyTimeline bool) (*Ticket, error) {
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
	if operatorID == 0 {
		return nil, errors.New("operator is required")
	}

	if err := s.ticketRepo.DB().Transaction(func(tx *gorm.DB) error {
		if blocked, blockErr := s.recentRecoveryActionExists(tx, ticketID, "recovery_retry", recoveryDedupWindow); blockErr != nil {
			return blockErr
		} else if blocked {
			return ErrRecoveryActionTooFrequent
		}
		// Reset failure count
		if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).Updates(map[string]any{
			"ai_failure_count":    0,
			"status":              TicketStatusDecisioning,
			"outcome":             "",
			"current_activity_id": nil,
		}).Error; err != nil {
			return err
		}

		if legacyTimeline {
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
		}
		actionDetails, _ := json.Marshal(map[string]any{
			"action": "retry",
			"reason": reason,
		})
		if err := tx.Create(&TicketTimeline{
			TicketID:   ticketID,
			OperatorID: operatorID,
			EventType:  "recovery_retry",
			Message:    "恢复动作：重试决策",
			Details:    JSONField(actionDetails),
		}).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}
	s.smartEngine.DispatchDecisionAsync(ticketID, nil, engine.TriggerReasonManualRetry)
	return s.ticketRepo.FindByID(ticketID)
}

func (s *TicketService) handoffHuman(ticketID uint, reason string, operatorID uint) (*Ticket, error) {
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
		return nil, errors.New("handoff-human is only available for smart engine tickets")
	}
	if operatorID == 0 {
		return nil, errors.New("operator is required")
	}

	if blocked, err := s.recentRecoveryActionExists(s.ticketRepo.DB(), ticketID, "recovery_handoff_human", recoveryDedupWindow); err != nil {
		return nil, err
	} else if blocked {
		return nil, ErrRecoveryActionTooFrequent
	}

	ticket, err := s.OverrideJump(ticketID, engine.NodeProcess, nil, reason, operatorID)
	if err != nil {
		return nil, err
	}

	actionDetails, _ := json.Marshal(map[string]any{
		"action": "handoff_human",
		"reason": reason,
	})
	if err := s.ticketRepo.DB().Create(&TicketTimeline{
		TicketID:   ticketID,
		OperatorID: operatorID,
		EventType:  "recovery_handoff_human",
		Message:    "恢复动作：转人工处理",
		Details:    JSONField(actionDetails),
	}).Error; err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *TicketService) recentRecoveryActionExists(db *gorm.DB, ticketID uint, eventType string, window time.Duration) (bool, error) {
	var count int64
	if err := db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type = ? AND created_at >= ?", ticketID, eventType, time.Now().Add(-window)).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func ticketStatusForManualActivity(activityType string) string {
	switch activityType {
	case engine.NodeAction:
		return TicketStatusExecutingAction
	case engine.NodeApprove, engine.NodeForm, engine.NodeProcess:
		return TicketStatusWaitingHuman
	default:
		return TicketStatusDecisioning
	}
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

	if err := db.Transaction(func(tx *gorm.DB) error {
		// Find the operator's pending assignment for this ticket activity.
		var original TicketAssignment
		if err := tx.Where("ticket_id = ? AND activity_id = ? AND (user_id = ? OR assignee_id = ?) AND status = ?",
			ticketID, activityID, operatorID, operatorID, AssignmentPending).First(&original).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoActiveAssignment
			}
			return err
		}

		if err := tx.Model(&original).Update("status", AssignmentTransferred).Error; err != nil {
			return err
		}

		if err := tx.Create(&TicketAssignment{
			TicketID:        ticketID,
			ActivityID:      activityID,
			ParticipantType: "user",
			UserID:          &targetUserID,
			AssigneeID:      &targetUserID,
			Status:          AssignmentPending,
			IsCurrent:       original.IsCurrent,
			TransferFrom:    &original.ID,
		}).Error; err != nil {
			return err
		}

		if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).Update("assignee_id", targetUserID).Error; err != nil {
			return err
		}

		return tx.Create(&TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "transfer",
			Message:    fmt.Sprintf("工单已转办给用户 %d", targetUserID),
		}).Error
	}); err != nil {
		return nil, err
	}

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

	if err := db.Transaction(func(tx *gorm.DB) error {
		// Find the operator's pending assignment for this ticket activity.
		var original TicketAssignment
		if err := tx.Where("ticket_id = ? AND activity_id = ? AND (user_id = ? OR assignee_id = ?) AND status = ?",
			ticketID, activityID, operatorID, operatorID, AssignmentPending).First(&original).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoActiveAssignment
			}
			return err
		}

		if err := tx.Model(&original).Update("status", AssignmentDelegated).Error; err != nil {
			return err
		}

		if err := tx.Create(&TicketAssignment{
			TicketID:        ticketID,
			ActivityID:      activityID,
			ParticipantType: "user",
			UserID:          &targetUserID,
			AssigneeID:      &targetUserID,
			Status:          AssignmentPending,
			IsCurrent:       true,
			DelegatedFrom:   &original.ID,
		}).Error; err != nil {
			return err
		}

		return tx.Create(&TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "delegate",
			Message:    fmt.Sprintf("工单已委派给用户 %d", targetUserID),
		}).Error
	}); err != nil {
		return nil, err
	}

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

	if err := db.Transaction(func(tx *gorm.DB) error {
		// Find the operator's pending assignment for this ticket activity.
		var assignment TicketAssignment
		posIDs, deptIDs := s.resolveUserOrg(operatorID)
		if err := tx.Where("ticket_id = ? AND activity_id = ? AND status = ?", ticketID, activityID, AssignmentPending).
			Where(s.ticketRepo.assignmentOperatorCondition("itsm_ticket_assignments", operatorID, posIDs, deptIDs)).
			First(&assignment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoActiveAssignment
			}
			return err
		}

		now := time.Now()

		if err := tx.Model(&assignment).Updates(map[string]any{
			"assignee_id": operatorID,
			"status":      AssignmentInProgress,
			"claimed_at":  now,
		}).Error; err != nil {
			return err
		}

		if err := tx.Model(&TicketAssignment{}).
			Where("ticket_id = ? AND activity_id = ? AND status = ? AND id != ?", ticketID, activityID, AssignmentPending, assignment.ID).
			Update("status", AssignmentClaimedByOther).Error; err != nil {
			return err
		}

		if err := tx.Model(&Ticket{}).Where("id = ?", ticketID).Updates(map[string]any{
			"assignee_id": operatorID,
			"status":      TicketStatusWaitingHuman,
		}).Error; err != nil {
			return err
		}

		return tx.Create(&TicketTimeline{
			TicketID:   ticketID,
			ActivityID: &activityID,
			OperatorID: operatorID,
			EventType:  "claim",
			Message:    "用户已抢单",
		}).Error
	}); err != nil {
		return nil, err
	}

	return s.ticketRepo.FindByID(ticketID)
}

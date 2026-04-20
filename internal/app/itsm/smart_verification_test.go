package itsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	appcore "metis/internal/app"
	aiapp "metis/internal/app/ai"
	"metis/internal/app/itsm/engine"
	org "metis/internal/app/org"
	"metis/internal/database"
	"metis/internal/model"
)

type recordingSubmitter struct {
	payloads []engine.SmartProgressPayload
	count    int
}

func (r *recordingSubmitter) SubmitTask(name string, payload json.RawMessage) error {
	if name != "itsm-smart-progress" {
		return nil
	}
	r.count++
	var p engine.SmartProgressPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	r.payloads = append(r.payloads, p)
	return nil
}

type staticDecisionExecutor struct {
	content string
	err     error
	calls   int
}

func (e *staticDecisionExecutor) Execute(_ context.Context, _ uint, _ appcore.AIDecisionRequest) (*appcore.AIDecisionResponse, error) {
	e.calls++
	if e.err != nil {
		return nil, e.err
	}
	return &appcore.AIDecisionResponse{Content: e.content, Turns: 1}, nil
}

type staticConfigProvider struct {
	agentID    uint
	mode       string
	fallbackID uint
}

func (p *staticConfigProvider) FallbackAssigneeID() uint { return p.fallbackID }
func (p *staticConfigProvider) DecisionAgentID() uint    { return p.agentID }
func (p *staticConfigProvider) DecisionMode() string {
	if p.mode == "" {
		return "ai_only"
	}
	return p.mode
}

type smartTestOrgResolver struct {
	positionIDsByUser map[uint][]uint
	deptIDsByUser     map[uint][]uint
	usersByPosDept    map[string][]uint
}

func (r *smartTestOrgResolver) GetUserDeptScope(_ uint, _ bool) ([]uint, error) { return nil, nil }
func (r *smartTestOrgResolver) GetUserPositions(_ uint) ([]appcore.OrgPosition, error) {
	return nil, nil
}
func (r *smartTestOrgResolver) GetUserDepartment(_ uint) (*appcore.OrgDepartment, error) {
	return nil, nil
}
func (r *smartTestOrgResolver) QueryContext(_, _, _ string, _ bool) (*appcore.OrgContextResult, error) {
	return nil, nil
}
func (r *smartTestOrgResolver) FindUsersByPositionCode(string) ([]uint, error)   { return nil, nil }
func (r *smartTestOrgResolver) FindUsersByDepartmentCode(string) ([]uint, error) { return nil, nil }
func (r *smartTestOrgResolver) FindUsersByPositionAndDepartment(posCode, deptCode string) ([]uint, error) {
	return append([]uint(nil), r.usersByPosDept[posCode+"@"+deptCode]...), nil
}
func (r *smartTestOrgResolver) FindUsersByPositionID(uint) ([]uint, error)   { return nil, nil }
func (r *smartTestOrgResolver) FindUsersByDepartmentID(uint) ([]uint, error) { return nil, nil }
func (r *smartTestOrgResolver) FindManagerByUserID(uint) (uint, error)       { return 0, nil }
func (r *smartTestOrgResolver) GetUserPositionIDs(userID uint) ([]uint, error) {
	return append([]uint(nil), r.positionIDsByUser[userID]...), nil
}
func (r *smartTestOrgResolver) GetUserDepartmentIDs(userID uint) ([]uint, error) {
	return append([]uint(nil), r.deptIDsByUser[userID]...), nil
}

type smartServiceFixture struct {
	db         *gorm.DB
	service    *TicketService
	smart      *engine.SmartEngine
	submitter  *recordingSubmitter
	executor   *staticDecisionExecutor
	requester  model.User
	approver   model.User
	outsider   model.User
	ticket     Ticket
	activity   TicketActivity
	assignment TicketAssignment
}

func newSmartServiceFixture(t *testing.T, plan string) *smartServiceFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:smart_service_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := db.AutoMigrate(
		&Ticket{}, &TicketActivity{}, &TicketAssignment{}, &TicketTimeline{},
		&ServiceCatalog{}, &ServiceDefinition{}, &ServiceAction{}, &SLATemplate{}, &Priority{},
		&model.User{}, &model.Role{}, &model.SystemConfig{}, &aiapp.Agent{},
		&org.Department{}, &org.Position{}, &org.UserPosition{}, &org.DepartmentPosition{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	requester := model.User{Username: "requester", IsActive: true}
	approver := model.User{Username: "approver", IsActive: true}
	outsider := model.User{Username: "outsider", IsActive: true}
	for _, u := range []*model.User{&requester, &approver, &outsider} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user %s: %v", u.Username, err)
		}
	}

	dept := org.Department{Name: "信息部", Code: "it", IsActive: true}
	pos := org.Position{Name: "IT管理员", Code: "it_admin", IsActive: true}
	if err := db.Create(&dept).Error; err != nil {
		t.Fatalf("create dept: %v", err)
	}
	if err := db.Create(&pos).Error; err != nil {
		t.Fatalf("create pos: %v", err)
	}
	if err := db.Create(&org.UserPosition{UserID: approver.ID, DepartmentID: dept.ID, PositionID: pos.ID, IsPrimary: true}).Error; err != nil {
		t.Fatalf("create user position: %v", err)
	}

	decisionCode := "itsm.decision"
	agent := aiapp.Agent{Name: "decision", Code: &decisionCode, Type: "internal", IsActive: true}
	if err := db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	catalog := ServiceCatalog{Name: "Root", Code: "root"}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	priority := Priority{Name: "P1", Code: "p1", Value: 1, Color: "#f00", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	serviceDef := ServiceDefinition{
		Name:              "Smart Service",
		Code:              "smart-service",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		AgentID:           &agent.ID,
		IsActive:          true,
		CollaborationSpec: "test",
	}
	if err := db.Create(&serviceDef).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	ticket := Ticket{
		Code:        "TK-SMART-001",
		Title:       "Smart Ticket",
		ServiceID:   serviceDef.ID,
		EngineType:  "smart",
		Status:      TicketStatusInProgress,
		PriorityID:  priority.ID,
		RequesterID: requester.ID,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	activity := TicketActivity{
		TicketID:     ticket.ID,
		Name:         "AI 决策待确认",
		ActivityType: "approve",
		Status:       engine.ActivityPendingApproval,
		AIDecision:   JSONField(plan),
		AIReasoning:  "need human review",
		AIConfidence: 0.4,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	assignment := TicketAssignment{
		TicketID:        ticket.ID,
		ActivityID:      activity.ID,
		ParticipantType: "user",
		UserID:          &approver.ID,
		AssigneeID:      &approver.ID,
		Status:          AssignmentPending,
		IsCurrent:       true,
	}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	if err := db.Model(&Ticket{}).Where("id = ?", ticket.ID).Updates(map[string]any{"current_activity_id": activity.ID, "assignee_id": approver.ID}).Error; err != nil {
		t.Fatalf("update ticket current activity: %v", err)
	}

	submitter := &recordingSubmitter{}
	executor := &staticDecisionExecutor{content: `{"next_step_type":"complete","execution_mode":"single","activities":[],"reasoning":"done","confidence":0.95}`}
	orgResolver := &smartTestOrgResolver{
		positionIDsByUser: map[uint][]uint{approver.ID: {pos.ID}},
		deptIDsByUser:     map[uint][]uint{approver.ID: {dept.ID}},
		usersByPosDept:    map[string][]uint{"it_admin@it": {approver.ID}},
	}
	resolver := engine.NewParticipantResolver(orgResolver)
	smartEngine := engine.NewSmartEngine(executor, nil, &testUserProvider{db: db}, resolver, submitter, &staticConfigProvider{agentID: agent.ID, mode: "ai_only"})

	ticketRepo := &TicketRepo{db: &database.DB{DB: db}}
	serviceRepo := &ServiceDefRepo{db: &database.DB{DB: db}}
	service := &TicketService{
		ticketRepo:    ticketRepo,
		timelineRepo:  &TimelineRepo{db: &database.DB{DB: db}},
		serviceRepo:   serviceRepo,
		classicEngine: engine.NewClassicEngine(resolver, &noopSubmitter{}, nil),
		smartEngine:   smartEngine,
		orgResolver:   orgResolver,
	}

	return &smartServiceFixture{
		db:         db,
		service:    service,
		smart:      smartEngine,
		submitter:  submitter,
		executor:   executor,
		requester:  requester,
		approver:   approver,
		outsider:   outsider,
		ticket:     ticket,
		activity:   activity,
		assignment: assignment,
	}
}

func TestConfirmActivity_SubmitsImmediateSmartContinuation(t *testing.T) {
	plan := `{"next_step_type":"process","execution_mode":"single","activities":[{"type":"process","participant_type":"user","participant_id":2,"instructions":"continue"}],"reasoning":"continue flow","confidence":0.4}`
	f := newSmartServiceFixture(t, plan)

	if _, err := f.service.ConfirmActivity(f.ticket.ID, f.activity.ID, f.approver.ID); err != nil {
		t.Fatalf("confirm activity: %v", err)
	}
	if f.submitter.count != 1 {
		t.Fatalf("expected one smart-progress submission, got %d", f.submitter.count)
	}
	if len(f.submitter.payloads) != 1 || f.submitter.payloads[0].CompletedActivityID == nil || *f.submitter.payloads[0].CompletedActivityID != f.activity.ID {
		t.Fatalf("expected payload to include completed activity id %d, got %+v", f.activity.ID, f.submitter.payloads)
	}
}

func TestRejectActivity_SubmitsImmediateSmartContinuationAndStoresReason(t *testing.T) {
	plan := `{"next_step_type":"process","execution_mode":"single","activities":[{"type":"process","participant_type":"user","participant_id":2,"instructions":"continue"}],"reasoning":"continue flow","confidence":0.4}`
	f := newSmartServiceFixture(t, plan)

	if _, err := f.service.RejectActivity(f.ticket.ID, f.activity.ID, "need more detail", f.approver.ID); err != nil {
		t.Fatalf("reject activity: %v", err)
	}
	if f.submitter.count != 1 {
		t.Fatalf("expected one smart-progress submission, got %d", f.submitter.count)
	}
	var activity TicketActivity
	if err := f.db.First(&activity, f.activity.ID).Error; err != nil {
		t.Fatalf("reload activity: %v", err)
	}
	if activity.DecisionReasoning != "need more detail" {
		t.Fatalf("expected rejection reason to be persisted, got %q", activity.DecisionReasoning)
	}
}

func TestApprovalsAndApprovalCount_OnlyIncludeActionablePendingApproval(t *testing.T) {
	plan := `{"next_step_type":"process","execution_mode":"single","activities":[{"type":"process","participant_type":"user","participant_id":2,"instructions":"continue"}],"reasoning":"continue flow","confidence":0.4}`
	f := newSmartServiceFixture(t, plan)

	items, total, err := f.service.Approvals(f.approver.ID, 1, 20)
	if err != nil {
		t.Fatalf("list approvals: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one actionable approval, got total=%d len=%d", total, len(items))
	}
	if items[0].ApprovalKind != "ai_confirm" || !items[0].CanAct {
		t.Fatalf("expected actionable ai_confirm item, got %+v", items[0])
	}

	count, err := f.service.ApprovalCount(f.approver.ID)
	if err != nil {
		t.Fatalf("approval count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected approval count 1, got %d", count)
	}

	items, total, err = f.service.Approvals(f.outsider.ID, 1, 20)
	if err != nil {
		t.Fatalf("list outsider approvals: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected outsider to see no approvals, got total=%d len=%d", total, len(items))
	}
	count, err = f.service.ApprovalCount(f.outsider.ID)
	if err != nil {
		t.Fatalf("outsider approval count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected outsider approval count 0, got %d", count)
	}
}

func TestConfirmRejectPendingApproval_RejectUnauthorizedOperator(t *testing.T) {
	plan := `{"next_step_type":"process","execution_mode":"single","activities":[{"type":"process","participant_type":"user","participant_id":2,"instructions":"continue"}],"reasoning":"continue flow","confidence":0.4}`

	t.Run("confirm", func(t *testing.T) {
		f := newSmartServiceFixture(t, plan)
		_, err := f.service.ConfirmActivity(f.ticket.ID, f.activity.ID, f.outsider.ID)
		if !errors.Is(err, ErrNotApprover) {
			t.Fatalf("expected ErrNotApprover, got %v", err)
		}
		if f.submitter.count != 0 {
			t.Fatalf("expected no progress submission on unauthorized confirm, got %d", f.submitter.count)
		}
	})

	t.Run("reject", func(t *testing.T) {
		f := newSmartServiceFixture(t, plan)
		_, err := f.service.RejectActivity(f.ticket.ID, f.activity.ID, "nope", f.outsider.ID)
		if !errors.Is(err, ErrNotApprover) {
			t.Fatalf("expected ErrNotApprover, got %v", err)
		}
		if f.submitter.count != 0 {
			t.Fatalf("expected no progress submission on unauthorized reject, got %d", f.submitter.count)
		}
	})
}

func TestRunDecisionCycleForTicket_SkipsDuplicateContinuationWhenActiveWorkExists(t *testing.T) {
	plan := `{"next_step_type":"process","execution_mode":"single","activities":[{"type":"process","participant_type":"user","participant_id":2,"instructions":"continue"}],"reasoning":"continue flow","confidence":0.95}`
	f := newSmartServiceFixture(t, plan)

	if err := f.smart.RunDecisionCycleForTicket(context.Background(), f.db, f.ticket.ID, nil); err != nil {
		t.Fatalf("run decision cycle: %v", err)
	}
	if f.executor.calls != 0 {
		t.Fatalf("expected duplicate continuation guard to skip executor, got %d calls", f.executor.calls)
	}

	var activities []TicketActivity
	if err := f.db.Where("ticket_id = ?", f.ticket.ID).Find(&activities).Error; err != nil {
		t.Fatalf("list activities: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected no new activities, got %d", len(activities))
	}
}

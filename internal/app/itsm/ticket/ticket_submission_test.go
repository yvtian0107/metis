package ticket

import (
	"context"
	"encoding/json"
	. "metis/internal/app/itsm/definition"
	. "metis/internal/app/itsm/domain"
	. "metis/internal/app/itsm/sla"
	"testing"
	"time"

	appcore "metis/internal/app"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/testutil"
	"metis/internal/app/itsm/tools"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/scheduler"

	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

type submissionTestDecisionExecutor struct{}

func (submissionTestDecisionExecutor) Execute(context.Context, uint, appcore.AIDecisionRequest) (*appcore.AIDecisionResponse, error) {
	return nil, nil
}

func TestAgentDraftSubmission_IdempotentConfirmedDraftStartsSmartProgressTask(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	req := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "VPN 开通申请",
		FormData:     map[string]any{"vpn_account": "admin@dev.com", "request_kind": "线上支持"},
		SessionID:    99,
		DraftVersion: 3,
		FieldsHash:   "fields-v1",
		RequestHash:  "request-v1",
	}

	first, err := ticketSvc.CreateFromAgent(context.Background(), req)
	if err != nil {
		t.Fatalf("first create from agent: %v", err)
	}
	second, err := ticketSvc.CreateFromAgent(context.Background(), req)
	if err != nil {
		t.Fatalf("second create from agent: %v", err)
	}
	if second.TicketID != first.TicketID || second.TicketCode != first.TicketCode {
		t.Fatalf("expected idempotent ticket result, first=%+v second=%+v", first, second)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 1 {
		t.Fatalf("expected one ticket after duplicate submit, got %d", ticketCount)
	}

	var ticket Ticket
	if err := db.First(&ticket, first.TicketID).Error; err != nil {
		t.Fatalf("load ticket: %v", err)
	}
	if ticket.Source != TicketSourceAgent {
		t.Fatalf("expected source=agent, got %q", ticket.Source)
	}
	if ticket.AgentSessionID == nil || *ticket.AgentSessionID != req.SessionID {
		t.Fatalf("expected agent_session_id=%d, got %v", req.SessionID, ticket.AgentSessionID)
	}

	var submissions []ServiceDeskSubmission
	if err := db.Find(&submissions).Error; err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(submissions) != 1 || submissions[0].TicketID != first.TicketID || submissions[0].Status != "submitted" {
		t.Fatalf("unexpected submissions: %+v", submissions)
	}

	var draftTimeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", first.TicketID, "draft_submitted").First(&draftTimeline).Error; err != nil {
		t.Fatalf("load draft_submitted timeline: %v", err)
	}

	var created Ticket
	if err := db.First(&created, first.TicketID).Error; err != nil {
		t.Fatalf("load created ticket: %v", err)
	}
	if created.Status != TicketStatusDecisioning {
		t.Fatalf("expected created smart ticket status %q, got %q", TicketStatusDecisioning, created.Status)
	}
}

func TestAgentDraftSubmission_SingleSQLiteConnectionDoesNotBlock(t *testing.T) {
	db := newTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	req := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "VPN 开通申请",
		FormData:     map[string]any{"vpn_account": "admin@dev.com", "request_kind": "线上支持"},
		SessionID:    100,
		DraftVersion: 1,
		FieldsHash:   "fields-v1",
		RequestHash:  "request-v1",
	}

	type createResult struct {
		ticket *tools.AgentTicketResult
		err    error
	}
	done := make(chan createResult, 1)
	go func() {
		ticket, err := ticketSvc.CreateFromAgent(context.Background(), req)
		done <- createResult{ticket: ticket, err: err}
	}()

	var result createResult
	select {
	case result = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("confirmed draft submission blocked with a single SQLite connection")
	}
	if result.err != nil {
		t.Fatalf("create from agent: %v", result.err)
	}
	if result.ticket == nil || result.ticket.TicketID == 0 || result.ticket.TicketCode == "" {
		t.Fatalf("expected created ticket result, got %+v", result.ticket)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Where("agent_session_id = ?", req.SessionID).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count session tickets: %v", err)
	}
	if ticketCount != 1 {
		t.Fatalf("expected one session ticket, got %d", ticketCount)
	}

	var submission ServiceDeskSubmission
	if err := db.Where("session_id = ? AND draft_version = ? AND fields_hash = ?", req.SessionID, req.DraftVersion, req.FieldsHash).
		First(&submission).Error; err != nil {
		t.Fatalf("load submission: %v", err)
	}
	if submission.TicketID != result.ticket.TicketID || submission.Status != "submitted" {
		t.Fatalf("unexpected submission: %+v", submission)
	}

	var created Ticket
	if err := db.First(&created, result.ticket.TicketID).Error; err != nil {
		t.Fatalf("load created ticket: %v", err)
	}
	if created.Status != TicketStatusDecisioning {
		t.Fatalf("expected created smart ticket status %q, got %q", TicketStatusDecisioning, created.Status)
	}
}

func TestTicketProgress_SingleSQLiteConnectionWithOrgScopeDoesNotBlock(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outcome string
		opinion string
	}{
		{name: "approve", outcome: "approved", opinion: "同意开通"},
		{name: "reject", outcome: "rejected", opinion: "不符合申请要求"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatalf("get sql db: %v", err)
			}
			sqlDB.SetMaxOpenConns(1)
			sqlDB.SetMaxIdleConns(1)
			if err := db.Exec("CREATE TABLE operator_positions (user_id INTEGER NOT NULL, position_id INTEGER NOT NULL)").Error; err != nil {
				t.Fatalf("create operator_positions: %v", err)
			}
			if err := db.Exec("CREATE TABLE operator_departments (user_id INTEGER NOT NULL, department_id INTEGER NOT NULL)").Error; err != nil {
				t.Fatalf("create operator_departments: %v", err)
			}
			if err := db.Exec("INSERT INTO operator_positions (user_id, position_id) VALUES (?, ?)", 7, 77).Error; err != nil {
				t.Fatalf("seed operator position: %v", err)
			}

			ticketSvc := newSubmissionTicketServiceWithOrgResolver(t, db, &rootDBOrgResolver{db: db})
			service := testutil.SeedSmartSubmissionService(t, db)
			ticket := Ticket{
				Code:        "TICK-ORG-SCOPE-" + tc.name,
				Title:       "VPN 开通申请",
				ServiceID:   service.ID,
				EngineType:  "smart",
				Status:      TicketStatusWaitingHuman,
				PriorityID:  1,
				RequesterID: 1,
			}
			if err := db.Create(&ticket).Error; err != nil {
				t.Fatalf("create ticket: %v", err)
			}
			activity := TicketActivity{
				TicketID:     ticket.ID,
				Name:         "审批",
				ActivityType: engine.NodeApprove,
				Status:       engine.ActivityPending,
				NodeID:       "approve",
			}
			if err := db.Create(&activity).Error; err != nil {
				t.Fatalf("create activity: %v", err)
			}
			positionID := uint(77)
			assignment := TicketAssignment{
				TicketID:        ticket.ID,
				ActivityID:      activity.ID,
				ParticipantType: "position",
				PositionID:      &positionID,
				Status:          "pending",
				IsCurrent:       true,
			}
			if err := db.Create(&assignment).Error; err != nil {
				t.Fatalf("create assignment: %v", err)
			}

			type progressResult struct {
				ticket *Ticket
				err    error
			}
			done := make(chan progressResult, 1)
			go func() {
				ticket, err := ticketSvc.Progress(ticket.ID, activity.ID, tc.outcome, tc.opinion, nil, 7)
				done <- progressResult{ticket: ticket, err: err}
			}()

			var result progressResult
			select {
			case result = <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("ticket progress outcome %s blocked with a single SQLite connection and org scope resolver", tc.outcome)
			}
			if result.err != nil {
				t.Fatalf("progress ticket activity with outcome %s: %v", tc.outcome, result.err)
			}
			if result.ticket == nil || result.ticket.ID != ticket.ID {
				t.Fatalf("unexpected progress result: %+v", result.ticket)
			}

			var updatedAssignment TicketAssignment
			if err := db.First(&updatedAssignment, assignment.ID).Error; err != nil {
				t.Fatalf("load assignment: %v", err)
			}
			if updatedAssignment.Status != tc.outcome || updatedAssignment.AssigneeID == nil || *updatedAssignment.AssigneeID != 7 {
				t.Fatalf("expected position assignment %s by operator, got %+v", tc.outcome, updatedAssignment)
			}

			var updatedActivity TicketActivity
			if err := db.First(&updatedActivity, activity.ID).Error; err != nil {
				t.Fatalf("load activity: %v", err)
			}
			if updatedActivity.Status != tc.outcome || updatedActivity.TransitionOutcome != tc.outcome {
				t.Fatalf("unexpected activity state: %+v", updatedActivity)
			}
		})
	}
}

func TestRetryAI_SingleSQLiteConnectionSubmitsSmartProgressInTransaction(t *testing.T) {
	db := newTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	ticket := Ticket{
		Code:           "TICK-RETRY-AI-SQLITE",
		Title:          "智能审批重试",
		ServiceID:      service.ID,
		EngineType:     "smart",
		Status:         TicketStatusWaitingHuman,
		PriorityID:     1,
		RequesterID:    1,
		AIFailureCount: 3,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	type retryResult struct {
		ticket *Ticket
		err    error
	}
	done := make(chan retryResult, 1)
	go func() {
		retried, err := ticketSvc.RetryAI(ticket.ID, "重新跑智能引擎", 7)
		done <- retryResult{ticket: retried, err: err}
	}()

	var result retryResult
	select {
	case result = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("retry AI blocked while submitting smart-progress with a single SQLite connection")
	}
	if result.err != nil {
		t.Fatalf("retry ai: %v", result.err)
	}
	if result.ticket == nil || result.ticket.AIFailureCount != 0 {
		t.Fatalf("expected retry result with ai_failure_count reset, got %+v", result.ticket)
	}

	var reloaded Ticket
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.AIFailureCount != 0 {
		t.Fatalf("expected ai_failure_count reset to 0, got %d", reloaded.AIFailureCount)
	}

	var timeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "ai_retry").First(&timeline).Error; err != nil {
		t.Fatalf("load retry timeline: %v", err)
	}

	if reloaded.Status != TicketStatusDecisioning {
		t.Fatalf("expected retry to put ticket into %q, got %q", TicketStatusDecisioning, reloaded.Status)
	}
}

func newSubmissionTicketService(t *testing.T, db *gorm.DB) *TicketService {
	return newSubmissionTicketServiceWithOrgResolver(t, db, nil)
}

func newSubmissionTicketServiceWithOrgResolver(t *testing.T, db *gorm.DB, orgResolver appcore.OrgResolver) *TicketService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	resolver := engine.NewParticipantResolver(orgResolver)
	injector := do.New()
	submitter := &submissionTestSubmitter{db: db}
	do.ProvideValue(injector, wrapped)
	if orgResolver != nil {
		do.ProvideValue[appcore.OrgResolver](injector, orgResolver)
	}
	do.Provide(injector, NewTicketRepo)
	do.Provide(injector, NewTimelineRepo)
	do.Provide(injector, NewServiceDefRepo)
	do.Provide(injector, NewSLATemplateRepo)
	do.Provide(injector, NewPriorityRepo)
	do.ProvideValue(injector, engine.NewClassicEngine(resolver, nil, nil))
	do.ProvideValue(injector, engine.NewSmartEngine(submissionTestDecisionExecutor{}, nil, nil, resolver, submitter, nil))
	do.Provide(injector, NewTicketService)
	return do.MustInvoke[*TicketService](injector)
}

type rootDBOrgResolver struct {
	db *gorm.DB
}

func (r *rootDBOrgResolver) GetUserDeptScope(uint, bool) ([]uint, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) GetUserPositionIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Table("operator_positions").Where("user_id = ?", userID).Pluck("position_id", &ids).Error
	return ids, err
}

func (r *rootDBOrgResolver) GetUserDepartmentIDs(userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Table("operator_departments").Where("user_id = ?", userID).Pluck("department_id", &ids).Error
	return ids, err
}

func (r *rootDBOrgResolver) GetUserPositions(uint) ([]appcore.OrgPosition, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) GetUserDepartment(uint) (*appcore.OrgDepartment, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) QueryContext(string, string, string, bool) (*appcore.OrgContextResult, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) FindUsersByPositionCode(string) ([]uint, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) FindUsersByDepartmentCode(string) ([]uint, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) FindUsersByPositionAndDepartment(string, string) ([]uint, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) FindUsersByPositionID(uint) ([]uint, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) FindUsersByDepartmentID(uint) ([]uint, error) {
	return nil, nil
}

func (r *rootDBOrgResolver) FindManagerByUserID(uint) (uint, error) {
	return 0, nil
}

type submissionTestSubmitter struct {
	db *gorm.DB
}

func (s *submissionTestSubmitter) SubmitTask(name string, payload json.RawMessage) error {
	return s.db.Create(&model.TaskExecution{
		TaskName: name,
		Trigger:  scheduler.TriggerAPI,
		Status:   scheduler.ExecPending,
		Payload:  string(payload),
	}).Error
}

func (s *submissionTestSubmitter) SubmitTaskTx(tx *gorm.DB, name string, payload json.RawMessage) error {
	return tx.Create(&model.TaskExecution{
		TaskName: name,
		Trigger:  scheduler.TriggerAPI,
		Status:   scheduler.ExecPending,
		Payload:  string(payload),
	}).Error
}

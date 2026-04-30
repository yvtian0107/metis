package ticket

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	. "metis/internal/app/itsm/definition"
	. "metis/internal/app/itsm/domain"
	. "metis/internal/app/itsm/sla"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

func TestCreateSmartTicket_BindsImmutableServiceRuntimeVersion(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	action := ServiceAction{
		Name:       "Original action",
		Code:       "notify",
		ActionType: "http",
		ConfigJSON: JSONField(`{"url":"https://example.com/original","method":"POST","timeout":30,"retries":3}`),
		ServiceID:  service.ID,
		IsActive:   true,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create action: %v", err)
	}

	created, err := ticketSvc.Create(CreateTicketInput{
		Title:      "VPN 开通申请",
		ServiceID:  service.ID,
		PriorityID: 1,
		FormData:   JSONField(`{"vpn_account":"admin@example.com"}`),
	}, 7)
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if created.ServiceVersionID == nil || *created.ServiceVersionID == 0 {
		t.Fatalf("expected smart ticket to bind service_version_id, got %+v", created.ServiceVersionID)
	}

	if err := db.Model(&ServiceDefinition{}).Where("id = ?", service.ID).Updates(map[string]any{
		"collaboration_spec": "mutated runtime spec",
		"agent_config":       JSONField(`{"temperature":0.9}`),
	}).Error; err != nil {
		t.Fatalf("mutate service: %v", err)
	}
	if err := db.Model(&ServiceAction{}).Where("id = ?", action.ID).Updates(map[string]any{
		"name":        "Mutated action",
		"config_json": JSONField(`{"url":"https://example.com/mutated","method":"DELETE","timeout":30,"retries":3}`),
	}).Error; err != nil {
		t.Fatalf("mutate action: %v", err)
	}

	var version ServiceDefinitionVersion
	if err := db.First(&version, *created.ServiceVersionID).Error; err != nil {
		t.Fatalf("load service version: %v", err)
	}
	if version.CollaborationSpec != service.CollaborationSpec {
		t.Fatalf("expected snapshot collaboration spec %q, got %q", service.CollaborationSpec, version.CollaborationSpec)
	}
	if !strings.Contains(string(version.ActionsJSON), "https://example.com/original") {
		t.Fatalf("expected version actions snapshot to retain original action config, got %s", version.ActionsJSON)
	}
	if strings.Contains(string(version.ActionsJSON), "https://example.com/mutated") {
		t.Fatalf("version actions snapshot drifted after live action mutation: %s", version.ActionsJSON)
	}
}

func TestCreateTicketUsesRuntimeVersionSLASnapshotForDeadlines(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	sla := SLATemplate{Name: "Snapshot SLA", Code: "snapshot-sla", ResponseMinutes: 7, ResolutionMinutes: 70, IsActive: true}
	if err := db.Create(&sla).Error; err != nil {
		t.Fatalf("create sla: %v", err)
	}
	if err := db.Model(&ServiceDefinition{}).Where("id = ?", service.ID).Update("sla_id", sla.ID).Error; err != nil {
		t.Fatalf("bind sla: %v", err)
	}
	var priority Priority
	if err := db.Where("code = ?", "P3").First(&priority).Error; err != nil {
		t.Fatalf("load priority: %v", err)
	}

	before := time.Now()
	created, err := ticketSvc.Create(CreateTicketInput{
		Title:      "VPN 开通申请",
		ServiceID:  service.ID,
		PriorityID: priority.ID,
		FormData:   JSONField(`{"vpn_account":"admin@example.com"}`),
	}, 7)
	after := time.Now()
	if err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if created.ServiceVersionID == nil || *created.ServiceVersionID == 0 {
		t.Fatalf("expected service_version_id, got %v", created.ServiceVersionID)
	}
	var version ServiceDefinitionVersion
	if err := db.First(&version, *created.ServiceVersionID).Error; err != nil {
		t.Fatalf("load service version: %v", err)
	}
	if !strings.Contains(string(version.SLATemplateJSON), `"responseMinutes":7`) {
		t.Fatalf("expected version SLA snapshot to contain responseMinutes=7, got %s", version.SLATemplateJSON)
	}
	if created.SLAResponseDeadline == nil || created.SLAResolutionDeadline == nil {
		t.Fatalf("expected SLA deadlines, got response=%v resolution=%v", created.SLAResponseDeadline, created.SLAResolutionDeadline)
	}
	minResponse := before.Add(7 * time.Minute)
	maxResponse := after.Add(7 * time.Minute)
	if created.SLAResponseDeadline.Before(minResponse) || created.SLAResponseDeadline.After(maxResponse) {
		t.Fatalf("response deadline %s not derived from 7 minute snapshot window [%s,%s]", created.SLAResponseDeadline, minResponse, maxResponse)
	}
	minResolution := before.Add(70 * time.Minute)
	maxResolution := after.Add(70 * time.Minute)
	if created.SLAResolutionDeadline.Before(minResolution) || created.SLAResolutionDeadline.After(maxResolution) {
		t.Fatalf("resolution deadline %s not derived from 70 minute snapshot window [%s,%s]", created.SLAResolutionDeadline, minResolution, maxResolution)
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

func TestAgentTicketUsesP3AsDefaultPriority(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	p0 := Priority{Name: "P0", Code: "P0", Value: 0, Color: "#dc2626", IsActive: true}
	if err := db.Create(&p0).Error; err != nil {
		t.Fatalf("create P0 priority: %v", err)
	}

	result, err := ticketSvc.CreateFromAgent(context.Background(), tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "VPN 开通申请",
		FormData:     map[string]any{"vpn_account": "admin@dev.com", "request_kind": "线上支持"},
		SessionID:    101,
		DraftVersion: 1,
		FieldsHash:   "fields-v1",
		RequestHash:  "request-v1",
	})
	if err != nil {
		t.Fatalf("create from agent: %v", err)
	}

	var priority Priority
	if err := db.Table("itsm_priorities").
		Joins("JOIN itsm_tickets ON itsm_tickets.priority_id = itsm_priorities.id").
		Where("itsm_tickets.id = ?", result.TicketID).
		First(&priority).Error; err != nil {
		t.Fatalf("load ticket priority: %v", err)
	}
	if priority.Code != "P3" {
		t.Fatalf("expected agent ticket default priority P3, got %s", priority.Code)
	}
}

func TestCreateTicketFailsWhenBoundSLAIsMissingOrInactive(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setupSLA func(t *testing.T, db *gorm.DB) uint
	}{
		{
			name: "missing",
			setupSLA: func(t *testing.T, db *gorm.DB) uint {
				return 999
			},
		},
		{
			name: "inactive",
			setupSLA: func(t *testing.T, db *gorm.DB) uint {
				sla := SLATemplate{Name: "停用 SLA", Code: "inactive-sla", ResponseMinutes: 1, ResolutionMinutes: 5, IsActive: false}
				if err := db.Create(&sla).Error; err != nil {
					t.Fatalf("create inactive SLA: %v", err)
				}
				if err := db.Model(&sla).Update("is_active", false).Error; err != nil {
					t.Fatalf("deactivate SLA: %v", err)
				}
				return sla.ID
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			ticketSvc := newSubmissionTicketService(t, db)
			service := testutil.SeedSmartSubmissionService(t, db)
			slaID := tc.setupSLA(t, db)
			if err := db.Model(&ServiceDefinition{}).Where("id = ?", service.ID).Update("sla_id", slaID).Error; err != nil {
				t.Fatalf("bind SLA: %v", err)
			}
			var priority Priority
			if err := db.Where("code = ?", "P3").First(&priority).Error; err != nil {
				t.Fatalf("load P3 priority: %v", err)
			}

			_, err := ticketSvc.Create(CreateTicketInput{
				Title:      "VPN 开通申请",
				ServiceID:  service.ID,
				PriorityID: priority.ID,
				FormData:   JSONField(`{}`),
			}, 7)
			if err == nil {
				t.Fatal("expected ticket creation to fail for invalid bound SLA")
			}
		})
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

func TestTicketProgressRejectsOperatorWithoutPendingAssignment(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	ticket := Ticket{
		Code:              "TICK-UNAUTHORIZED-PROGRESS",
		Title:             "VPN 开通申请",
		ServiceID:         service.ID,
		EngineType:        "smart",
		Status:            TicketStatusWaitingHuman,
		PriorityID:        1,
		RequesterID:       1,
		CurrentActivityID: nil,
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
	assigneeID := uint(7)
	assignment := TicketAssignment{
		TicketID:   ticket.ID,
		ActivityID: activity.ID,
		UserID:     &assigneeID,
		AssigneeID: &assigneeID,
		Status:     AssignmentPending,
		IsCurrent:  true,
	}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	_, err := ticketSvc.Progress(ticket.ID, activity.ID, "approved", "越权同意", nil, 8)
	if !errors.Is(err, ErrNoActiveAssignment) {
		t.Fatalf("expected ErrNoActiveAssignment, got %v", err)
	}

	var reloadedActivity TicketActivity
	if err := db.First(&reloadedActivity, activity.ID).Error; err != nil {
		t.Fatalf("reload activity: %v", err)
	}
	if reloadedActivity.Status != engine.ActivityPending || reloadedActivity.TransitionOutcome != "" {
		t.Fatalf("unauthorized progress mutated activity: %+v", reloadedActivity)
	}
	var reloadedAssignment TicketAssignment
	if err := db.First(&reloadedAssignment, assignment.ID).Error; err != nil {
		t.Fatalf("reload assignment: %v", err)
	}
	if reloadedAssignment.Status != AssignmentPending || reloadedAssignment.AssigneeID == nil || *reloadedAssignment.AssigneeID != assigneeID {
		t.Fatalf("unauthorized progress mutated assignment: %+v", reloadedAssignment)
	}
	var completedTimelineCount int64
	if err := db.Model(&TicketTimeline{}).Where("ticket_id = ? AND event_type = ?", ticket.ID, "activity_completed").Count(&completedTimelineCount).Error; err != nil {
		t.Fatalf("count completion timeline: %v", err)
	}
	if completedTimelineCount != 0 {
		t.Fatalf("expected no completion timeline, got %d", completedTimelineCount)
	}
}

func TestTicketHandlerCreateClassicTicketStartsLifecycle(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service, priority := seedClassicSubmissionService(t, db)
	handler := &TicketHandler{svc: ticketSvc}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userId", uint(7))
		c.Set("userRole", model.RoleUser)
		c.Next()
	})
	router.POST("/api/v1/itsm/tickets", handler.Create)

	body := bytes.NewBufferString(`{"title":"经典 VPN 申请","description":"需要网络支持","serviceId":` +
		jsonNumber(service.ID) + `,"priorityId":` + jsonNumber(priority.ID) + `,"formData":{"request_kind":"network_support"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var ticket Ticket
	if err := db.Where("title = ?", "经典 VPN 申请").First(&ticket).Error; err != nil {
		t.Fatalf("load created ticket: %v", err)
	}
	if ticket.Source != TicketSourceCatalog || ticket.RequesterID != 7 || ticket.EngineType != "classic" || ticket.Status != TicketStatusWaitingHuman {
		t.Fatalf("unexpected created ticket: %+v", ticket)
	}
	var activity TicketActivity
	if err := db.Where("ticket_id = ? AND activity_type = ?", ticket.ID, engine.NodeApprove).First(&activity).Error; err != nil {
		t.Fatalf("load first activity: %v", err)
	}
	var assignment TicketAssignment
	if err := db.Where("ticket_id = ? AND activity_id = ?", ticket.ID, activity.ID).First(&assignment).Error; err != nil {
		t.Fatalf("load assignment: %v", err)
	}
	if assignment.AssigneeID == nil || *assignment.AssigneeID != 8 || assignment.Status != AssignmentPending {
		t.Fatalf("unexpected assignment: %+v", assignment)
	}
}

func TestTicketHandlerCreateRejectsSmartService(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	var priority Priority
	if err := db.Where("code = ?", "P3").First(&priority).Error; err != nil {
		t.Fatalf("load priority: %v", err)
	}
	handler := &TicketHandler{svc: ticketSvc}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userId", uint(7))
		c.Set("userRole", model.RoleUser)
		c.Next()
	})
	router.POST("/api/v1/itsm/tickets", handler.Create)

	body := bytes.NewBufferString(`{"title":"智能 VPN 申请","serviceId":` +
		jsonNumber(service.ID) + `,"priorityId":` + jsonNumber(priority.ID) + `,"formData":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for smart service, got %d body=%s", rec.Code, rec.Body.String())
	}
	var ticketCount int64
	if err := db.Model(&Ticket{}).Where("title = ?", "智能 VPN 申请").Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected smart service API submission to create no tickets, got %d", ticketCount)
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

func TestRecoverRetry_DedupWindow(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	ticket := Ticket{
		Code:           "TICK-RECOVER-RETRY",
		Title:          "恢复重试",
		ServiceID:      service.ID,
		EngineType:     "smart",
		Status:         TicketStatusDecisioning,
		PriorityID:     1,
		RequesterID:    1,
		AIFailureCount: 1,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	if _, err := ticketSvc.Recover(ticket.ID, "retry", "第一次重试", 7); err != nil {
		t.Fatalf("first recover retry: %v", err)
	}
	if _, err := ticketSvc.Recover(ticket.ID, "retry", "重复重试", 7); !errors.Is(err, ErrRecoveryActionTooFrequent) {
		t.Fatalf("expected ErrRecoveryActionTooFrequent, got %v", err)
	}
}

func TestRecoverHandoffHuman_WritesAuditTimeline(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	ticket := Ticket{
		Code:        "TICK-RECOVER-HANDOFF",
		Title:       "恢复转人工",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusDecisioning,
		PriorityID:  1,
		RequesterID: 1,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	recovered, err := ticketSvc.Recover(ticket.ID, "handoff_human", "需要人工接手", 7)
	if err != nil {
		t.Fatalf("recover handoff_human: %v", err)
	}
	if recovered.Status != TicketStatusWaitingHuman {
		t.Fatalf("expected waiting_human after handoff, got %q", recovered.Status)
	}

	var timeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "recovery_handoff_human").First(&timeline).Error; err != nil {
		t.Fatalf("expected recovery_handoff_human timeline, got %v", err)
	}
}

func newSubmissionTicketService(t *testing.T, db *gorm.DB) *TicketService {
	return newSubmissionTicketServiceWithOrgResolver(t, db, nil)
}

func seedClassicSubmissionService(t *testing.T, db *gorm.DB) (ServiceDefinition, Priority) {
	t.Helper()
	if err := db.Create(&model.User{
		BaseModel: model.BaseModel{ID: 7},
		Username:  "classic-requester",
		IsActive:  true,
	}).Error; err != nil {
		t.Fatalf("create requester: %v", err)
	}
	if err := db.Create(&model.User{
		BaseModel: model.BaseModel{ID: 8},
		Username:  "classic-approver",
		IsActive:  true,
	}).Error; err != nil {
		t.Fatalf("create approver: %v", err)
	}
	priority := Priority{Name: "P3", Code: "P3", Value: 3, Color: "#64748b", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	catalog := ServiceCatalog{Name: "账号与权限", Code: "classic-account", IsActive: true}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	workflow := JSONField(`{
		"nodes": [
			{"id": "start", "type": "start", "data": {"label": "开始"}},
			{"id": "approve", "type": "approve", "data": {"label": "审批", "participants": [{"type": "user", "value": "8"}]}},
			{"id": "end", "type": "end", "data": {"label": "结束"}}
		],
		"edges": [
			{"id": "e1", "source": "start", "target": "approve", "data": {}},
			{"id": "e2", "source": "approve", "target": "end", "data": {"outcome": "approved"}}
		]
	}`)
	service := ServiceDefinition{
		Name:         "经典 VPN 开通",
		Code:         "classic-vpn-access",
		CatalogID:    catalog.ID,
		EngineType:   "classic",
		WorkflowJSON: workflow,
		IsActive:     true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service, priority
}

func jsonNumber(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
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

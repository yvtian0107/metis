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

	appcore "metis/internal/app"
	"metis/internal/app/itsm/engine"
	"metis/internal/app/itsm/testutil"
	"metis/internal/app/itsm/tools"
	"metis/internal/database"
	"metis/internal/model"
	"metis/internal/scheduler"

	"github.com/gin-gonic/gin"

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

	var tasks []model.TaskExecution
	if err := db.Where("task_name = ?", "itsm-smart-progress").Find(&tasks).Error; err != nil {
		t.Fatalf("load smart progress tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one smart-progress task for idempotent submission, got %d", len(tasks))
	}
	var payload engine.SmartProgressPayload
	if err := json.Unmarshal([]byte(tasks[0].Payload), &payload); err != nil {
		t.Fatalf("decode smart progress payload: %v", err)
	}
	if payload.TicketID != first.TicketID || payload.CompletedActivityID != nil || payload.TriggerReason != engine.TriggerReasonTicketCreated {
		t.Fatalf("unexpected smart progress payload: %+v", payload)
	}
}

func TestAgentDraftSubmission_SameSessionDifferentRequestHashCreatesNewTicket(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	firstReq := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "VPN 开通申请",
		FormData:     map[string]any{"vpn_account": "admin@dev.com", "request_kind": "线上支持"},
		SessionID:    99,
		DraftVersion: 1,
		FieldsHash:   "fields-v1",
		RequestHash:  "request-v1",
	}
	secondReq := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "VPN 权限变更申请",
		FormData:     map[string]any{"vpn_account": "ops@dev.com", "request_kind": "权限变更"},
		SessionID:    99,
		DraftVersion: 1,
		FieldsHash:   "fields-v1",
		RequestHash:  "request-v2",
	}

	first, err := ticketSvc.CreateFromAgent(context.Background(), firstReq)
	if err != nil {
		t.Fatalf("first create from agent: %v", err)
	}
	second, err := ticketSvc.CreateFromAgent(context.Background(), secondReq)
	if err != nil {
		t.Fatalf("second create from agent: %v", err)
	}
	if second.TicketID == first.TicketID || second.TicketCode == first.TicketCode {
		t.Fatalf("expected distinct ticket result for different request hash, first=%+v second=%+v", first, second)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 2 {
		t.Fatalf("expected two tickets after distinct requests, got %d", ticketCount)
	}

	var submissions []ServiceDeskSubmission
	if err := db.Where("session_id = ? AND draft_version = ? AND fields_hash = ?", firstReq.SessionID, firstReq.DraftVersion, firstReq.FieldsHash).
		Order("request_hash asc").
		Find(&submissions).Error; err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(submissions) != 2 {
		t.Fatalf("expected two submissions for distinct request hashes, got %+v", submissions)
	}
	if submissions[0].RequestHash == submissions[1].RequestHash {
		t.Fatalf("expected unique request hashes, got %+v", submissions)
	}

	var tasks []model.TaskExecution
	if err := db.Where("task_name = ?", "itsm-smart-progress").Find(&tasks).Error; err != nil {
		t.Fatalf("load smart progress tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected two smart-progress tasks for distinct submissions, got %d", len(tasks))
	}
	var payloads []engine.SmartProgressPayload
	for _, task := range tasks {
		var payload engine.SmartProgressPayload
		if err := json.Unmarshal([]byte(task.Payload), &payload); err != nil {
			t.Fatalf("decode smart progress payload: %v", err)
		}
		payloads = append(payloads, payload)
	}
	if payloads[0].TicketID == payloads[1].TicketID {
		t.Fatalf("expected distinct smart-progress payloads, got %+v", payloads)
	}
}

func TestAgentDraftSubmission_TrimmedFingerprintRemainsIdempotent(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	firstReq := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "VPN 开通申请",
		FormData:     map[string]any{"vpn_account": "admin@dev.com", "request_kind": "线上支持"},
		SessionID:    199,
		DraftVersion: 2,
		FieldsHash:   "fields-trimmed",
		RequestHash:  "request-trimmed",
	}
	secondReq := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "VPN 开通申请",
		FormData:     map[string]any{"vpn_account": "admin@dev.com", "request_kind": "线上支持"},
		SessionID:    199,
		DraftVersion: 2,
		FieldsHash:   "  fields-trimmed  ",
		RequestHash:  "  request-trimmed  ",
	}

	first, err := ticketSvc.CreateFromAgent(context.Background(), firstReq)
	if err != nil {
		t.Fatalf("first create from agent: %v", err)
	}
	second, err := ticketSvc.CreateFromAgent(context.Background(), secondReq)
	if err != nil {
		t.Fatalf("second create from agent: %v", err)
	}
	if second.TicketID != first.TicketID || second.TicketCode != first.TicketCode {
		t.Fatalf("expected trimmed fingerprints to stay idempotent, first=%+v second=%+v", first, second)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 1 {
		t.Fatalf("expected one ticket after trimmed duplicate submit, got %d", ticketCount)
	}

	var submissions []ServiceDeskSubmission
	if err := db.Find(&submissions).Error; err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("expected one submission after trimmed duplicate submit, got %+v", submissions)
	}
	if submissions[0].FieldsHash != "fields-trimmed" || submissions[0].RequestHash != "request-trimmed" {
		t.Fatalf("expected normalized stored fingerprint, got %+v", submissions[0])
	}
}

func TestAgentTicketWithoutDraftFingerprintBypassesSubmissionRecords(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	result, err := ticketSvc.CreateFromAgent(context.Background(), tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "无草稿指纹提单",
		FormData:     map[string]any{"vpn_account": "agent@example.com"},
		SessionID:    301,
		DraftVersion: 0,
		FieldsHash:   "",
		RequestHash:  "request-no-draft",
	})
	if err != nil {
		t.Fatalf("create from agent without draft fingerprint: %v", err)
	}

	var created Ticket
	if err := db.First(&created, result.TicketID).Error; err != nil {
		t.Fatalf("load created ticket: %v", err)
	}
	if created.Source != TicketSourceAgent || created.AgentSessionID == nil || *created.AgentSessionID != 301 {
		t.Fatalf("unexpected created agent ticket: %+v", created)
	}

	var submissionCount int64
	if err := db.Model(&ServiceDeskSubmission{}).Count(&submissionCount).Error; err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	if submissionCount != 0 {
		t.Fatalf("expected no service desk submission rows, got %d", submissionCount)
	}

	var draftTimelineCount int64
	if err := db.Model(&TicketTimeline{}).Where("ticket_id = ? AND event_type = ?", result.TicketID, "draft_submitted").Count(&draftTimelineCount).Error; err != nil {
		t.Fatalf("count draft_submitted timelines: %v", err)
	}
	if draftTimelineCount != 0 {
		t.Fatalf("expected no draft_submitted timeline, got %d", draftTimelineCount)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("expected one smart-progress task, got %d", taskCount)
	}
}

func TestAgentDraftSubmission_InProgressSubmissionRejectsDuplicateCreate(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	req := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "重复草稿提交",
		FormData:     map[string]any{"vpn_account": "agent@example.com"},
		SessionID:    302,
		DraftVersion: 2,
		FieldsHash:   "fields-v2",
		RequestHash:  "request-v2",
	}
	if err := db.Create(&ServiceDeskSubmission{
		SessionID:    req.SessionID,
		DraftVersion: req.DraftVersion,
		FieldsHash:   req.FieldsHash,
		RequestHash:  req.RequestHash,
		Status:       "submitting",
		SubmittedBy:  req.UserID,
		SubmittedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("create in-progress submission: %v", err)
	}

	if _, err := ticketSvc.CreateFromAgent(context.Background(), req); err == nil || !strings.Contains(err.Error(), "draft submission is already being created") {
		t.Fatalf("expected in-progress draft error, got %v", err)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected no tickets for duplicate in-progress draft, got %d", ticketCount)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected no smart-progress task for duplicate in-progress draft, got %d", taskCount)
	}
}

func TestAgentDraftSubmission_StaleSubmittedRecordSurfacesLookupError(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	req := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "陈旧 submission 记录",
		FormData:     map[string]any{"vpn_account": "agent@example.com"},
		SessionID:    303,
		DraftVersion: 1,
		FieldsHash:   "fields-stale",
		RequestHash:  "request-stale",
	}
	if err := db.Create(&ServiceDeskSubmission{
		SessionID:    req.SessionID,
		DraftVersion: req.DraftVersion,
		FieldsHash:   req.FieldsHash,
		RequestHash:  req.RequestHash,
		TicketID:     999999,
		Status:       "submitted",
		SubmittedBy:  req.UserID,
		SubmittedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("create stale submission: %v", err)
	}

	if _, err := ticketSvc.CreateFromAgent(context.Background(), req); err == nil {
		t.Fatal("expected stale submitted record to surface ticket lookup error")
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected no recreated ticket for stale submission, got %d", ticketCount)
	}
}

func TestAgentDraftSubmission_FailedRecordCanBeRetried(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)
	req := tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "失败草稿重试",
		FormData:     map[string]any{"vpn_account": "agent@example.com"},
		SessionID:    304,
		DraftVersion: 1,
		FieldsHash:   "fields-failed",
		RequestHash:  "request-failed",
	}
	if err := db.Create(&ServiceDeskSubmission{
		SessionID:    req.SessionID,
		DraftVersion: req.DraftVersion,
		FieldsHash:   req.FieldsHash,
		RequestHash:  req.RequestHash,
		TicketID:     0,
		Status:       "failed",
		SubmittedBy:  req.UserID,
		SubmittedAt:  time.Now().Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("create failed submission: %v", err)
	}

	result, err := ticketSvc.CreateFromAgent(context.Background(), req)
	if err != nil {
		t.Fatalf("retry create from failed submission: %v", err)
	}

	var created Ticket
	if err := db.First(&created, result.TicketID).Error; err != nil {
		t.Fatalf("load retried ticket: %v", err)
	}

	var submissions []ServiceDeskSubmission
	if err := db.Where("session_id = ? AND draft_version = ? AND fields_hash = ? AND request_hash = ?", req.SessionID, req.DraftVersion, req.FieldsHash, req.RequestHash).
		Find(&submissions).Error; err != nil {
		t.Fatalf("list retried submissions: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("expected failed submission row to be reused, got %+v", submissions)
	}
	if submissions[0].Status != "submitted" || submissions[0].TicketID != created.ID {
		t.Fatalf("expected submission to be promoted to submitted with new ticket, got %+v", submissions[0])
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

func TestCreateSmartTicket_RollsBackWhenSmartEngineUnavailable(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketServiceWithoutDecisionExecutor(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	_, err := ticketSvc.Create(CreateTicketInput{
		Title:      "智能引擎缺失回滚",
		ServiceID:  service.ID,
		PriorityID: 1,
		FormData:   JSONField(`{"vpn_account":"rollback@example.com"}`),
	}, 7)
	if !errors.Is(err, engine.ErrSmartEngineUnavailable) {
		t.Fatalf("expected ErrSmartEngineUnavailable, got %v", err)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected smart create rollback to leave no tickets, got %d", ticketCount)
	}

	var timelineCount int64
	if err := db.Model(&TicketTimeline{}).Count(&timelineCount).Error; err != nil {
		t.Fatalf("count timelines: %v", err)
	}
	if timelineCount != 0 {
		t.Fatalf("expected no ticket timelines after rollback, got %d", timelineCount)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected no smart-progress task after rollback, got %d", taskCount)
	}
}

func TestCreateFromAgent_RollsBackWhenSmartEngineUnavailable(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketServiceWithoutDecisionExecutor(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	_, err := ticketSvc.CreateFromAgent(context.Background(), tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "智能草稿提交缺少决策执行器",
		FormData:     map[string]any{"vpn_account": "rollback-agent-ai@example.com"},
		SessionID:    409,
		DraftVersion: 1,
		FieldsHash:   "fields-no-ai",
		RequestHash:  "request-no-ai",
	})
	if !errors.Is(err, engine.ErrSmartEngineUnavailable) {
		t.Fatalf("expected ErrSmartEngineUnavailable, got %v", err)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected no tickets after agent smart-engine rollback, got %d", ticketCount)
	}

	var submissionCount int64
	if err := db.Model(&ServiceDeskSubmission{}).Count(&submissionCount).Error; err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	if submissionCount != 0 {
		t.Fatalf("expected no draft submissions after agent smart-engine rollback, got %d", submissionCount)
	}

	var timelineCount int64
	if err := db.Model(&TicketTimeline{}).Count(&timelineCount).Error; err != nil {
		t.Fatalf("count timelines: %v", err)
	}
	if timelineCount != 0 {
		t.Fatalf("expected no timelines after agent smart-engine rollback, got %d", timelineCount)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected no smart-progress task after agent smart-engine rollback, got %d", taskCount)
	}
}

func TestCreateFromAgent_RollsBackWhenSmartProgressDispatchUnavailable(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketServiceWithoutScheduler(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	_, err := ticketSvc.CreateFromAgent(context.Background(), tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "智能草稿提交缺少调度器",
		FormData:     map[string]any{"vpn_account": "rollback-agent@example.com"},
		SessionID:    410,
		DraftVersion: 1,
		FieldsHash:   "fields-no-scheduler",
		RequestHash:  "request-no-scheduler",
	})
	if err == nil || !strings.Contains(err.Error(), "smart task scheduler is not configured") {
		t.Fatalf("expected missing scheduler error, got %v", err)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected no tickets after agent rollback, got %d", ticketCount)
	}

	var submissionCount int64
	if err := db.Model(&ServiceDeskSubmission{}).Count(&submissionCount).Error; err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	if submissionCount != 0 {
		t.Fatalf("expected no draft submissions after agent rollback, got %d", submissionCount)
	}

	var timelineCount int64
	if err := db.Model(&TicketTimeline{}).Count(&timelineCount).Error; err != nil {
		t.Fatalf("count timelines: %v", err)
	}
	if timelineCount != 0 {
		t.Fatalf("expected no timelines after agent rollback, got %d", timelineCount)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected no smart-progress task after agent rollback, got %d", taskCount)
	}
}

func TestCreateSmartTicket_RollsBackWhenSmartProgressDispatchUnavailable(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketServiceWithoutScheduler(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	_, err := ticketSvc.Create(CreateTicketInput{
		Title:      "智能调度器缺失回滚",
		ServiceID:  service.ID,
		PriorityID: 1,
		FormData:   JSONField(`{"vpn_account":"rollback-create@example.com"}`),
	}, 7)
	if err == nil || !strings.Contains(err.Error(), "smart task scheduler is not configured") {
		t.Fatalf("expected missing scheduler error, got %v", err)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected no tickets after create rollback, got %d", ticketCount)
	}

	var timelineCount int64
	if err := db.Model(&TicketTimeline{}).Count(&timelineCount).Error; err != nil {
		t.Fatalf("count timelines: %v", err)
	}
	if timelineCount != 0 {
		t.Fatalf("expected no timelines after create rollback, got %d", timelineCount)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected no smart-progress task after create rollback, got %d", taskCount)
	}
}

func TestCreateSmartTicket_StartsDecisioningLifecycleAndQueuesProgressTask(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	created, err := ticketSvc.Create(CreateTicketInput{
		Title:       "VPN 智能提单",
		Description: "普通入口智能提单",
		ServiceID:   service.ID,
		PriorityID:  1,
		FormData:    JSONField(`{"vpn_account":"smart@example.com","request_kind":"线上支持"}`),
	}, 7)
	if err != nil {
		t.Fatalf("create smart ticket: %v", err)
	}
	if created.EngineType != "smart" || created.Status != TicketStatusDecisioning {
		t.Fatalf("unexpected created smart ticket: %+v", created)
	}
	if created.CurrentActivityID != nil {
		t.Fatalf("expected smart ticket to remain decisioning without current activity, got %v", created.CurrentActivityID)
	}

	var ticketCreated TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", created.ID, "ticket_created").First(&ticketCreated).Error; err != nil {
		t.Fatalf("load ticket_created timeline: %v", err)
	}
	if ticketCreated.OperatorID != 7 || ticketCreated.Message != "工单已创建" {
		t.Fatalf("unexpected ticket_created timeline: %+v", ticketCreated)
	}

	var workflowStarted TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", created.ID, "workflow_started").First(&workflowStarted).Error; err != nil {
		t.Fatalf("load workflow_started timeline: %v", err)
	}
	if workflowStarted.OperatorID != 7 || workflowStarted.Message != "智能流程已启动" {
		t.Fatalf("unexpected workflow_started timeline: %+v", workflowStarted)
	}

	var activityCount int64
	if err := db.Model(&TicketActivity{}).Where("ticket_id = ?", created.ID).Count(&activityCount).Error; err != nil {
		t.Fatalf("count smart ticket activities: %v", err)
	}
	if activityCount != 0 {
		t.Fatalf("expected no human activity on smart create before decision cycle, got %d", activityCount)
	}

	var tasks []model.TaskExecution
	if err := db.Where("task_name = ?", "itsm-smart-progress").Order("id asc").Find(&tasks).Error; err != nil {
		t.Fatalf("load smart progress tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one smart-progress task, got %d", len(tasks))
	}
	var payload engine.SmartProgressPayload
	if err := json.Unmarshal([]byte(tasks[0].Payload), &payload); err != nil {
		t.Fatalf("decode smart progress payload: %v", err)
	}
	if payload.TicketID != created.ID || payload.CompletedActivityID != nil || payload.TriggerReason != engine.TriggerReasonTicketCreated {
		t.Fatalf("unexpected smart progress payload: %+v", payload)
	}
}

func TestCreateClassicTicket_StartFormValidationFailureStillStartsWorkflow(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service, priority := seedClassicSubmissionService(t, db)
	schema := JSONField(`{
		"version": 1,
		"fields": [
			{"key":"vpn_account","type":"text","label":"账号","required":true,"binding":"vpn_account"},
			{"key":"reason","type":"text","label":"原因","required":true,"binding":"request_reason"}
		]
	}`)
	if err := db.Model(&ServiceDefinition{}).Where("id = ?", service.ID).Update("intake_form_schema", schema).Error; err != nil {
		t.Fatalf("update intake form schema: %v", err)
	}

	created, err := ticketSvc.Create(CreateTicketInput{
		Title:      "经典开始表单校验失败",
		ServiceID:  service.ID,
		PriorityID: priority.ID,
		FormData:   JSONField(`{"vpn_account":"classic@example.com"}`),
	}, 7)
	if err != nil {
		t.Fatalf("create classic ticket with invalid start form: %v", err)
	}
	if created.EngineType != "classic" || created.Status != TicketStatusWaitingHuman {
		t.Fatalf("unexpected created ticket: %+v", created)
	}

	var activity TicketActivity
	if err := db.Where("ticket_id = ? AND activity_type = ?", created.ID, engine.NodeApprove).First(&activity).Error; err != nil {
		t.Fatalf("load created approval activity: %v", err)
	}
	if activity.Status != engine.ActivityPending {
		t.Fatalf("expected pending approval activity after validation warning, got %+v", activity)
	}

	var timeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", created.ID, "form_validation_failed").First(&timeline).Error; err != nil {
		t.Fatalf("load form validation timeline: %v", err)
	}
	if !strings.Contains(timeline.Message, "开始表单验证失败") {
		t.Fatalf("expected validation timeline message, got %q", timeline.Message)
	}

	var workflowTimeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", created.ID, "workflow_started").First(&workflowTimeline).Error; err != nil {
		t.Fatalf("load workflow_started timeline: %v", err)
	}
}

func TestCreateCatalogClassicCreatesWorkflowWithoutSmartTask(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service, priority := seedClassicSubmissionService(t, db)

	created, err := ticketSvc.CreateCatalog(CreateTicketInput{
		Title:       "经典目录提单",
		Description: "目录入口",
		ServiceID:   service.ID,
		PriorityID:  priority.ID,
		FormData:    JSONField(`{"request_kind":"network_support"}`),
	}, 7)
	if err != nil {
		t.Fatalf("create catalog ticket: %v", err)
	}
	if created.Source != TicketSourceCatalog || created.EngineType != "classic" || created.Status != TicketStatusWaitingHuman {
		t.Fatalf("unexpected created catalog ticket: %+v", created)
	}

	var activity TicketActivity
	if err := db.Where("ticket_id = ? AND activity_type = ?", created.ID, engine.NodeApprove).First(&activity).Error; err != nil {
		t.Fatalf("load catalog approval activity: %v", err)
	}

	var assignment TicketAssignment
	if err := db.Where("ticket_id = ? AND activity_id = ?", created.ID, activity.ID).First(&assignment).Error; err != nil {
		t.Fatalf("load catalog assignment: %v", err)
	}
	if assignment.AssigneeID == nil || *assignment.AssigneeID != 8 || assignment.Status != AssignmentPending {
		t.Fatalf("unexpected catalog assignment: %+v", assignment)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected classic catalog submit to enqueue no smart-progress task, got %d", taskCount)
	}
}

func TestCreateManualTicket_PersistsWithoutWorkflowOrSmartTask(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)

	if err := db.Create(&model.User{
		BaseModel: model.BaseModel{ID: 7},
		Username:  "manual-requester",
		IsActive:  true,
	}).Error; err != nil {
		t.Fatalf("create requester: %v", err)
	}
	priority := Priority{Name: "P3", Code: "P3", Value: 3, Color: "#64748b", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	catalog := ServiceCatalog{Name: "通用咨询", Code: "manual-general", IsActive: true}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	service := ServiceDefinition{
		Name:       "人工受理服务",
		Code:       "manual-intake",
		CatalogID:  catalog.ID,
		EngineType: "manual",
		IsActive:   true,
	}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create manual service: %v", err)
	}

	created, err := ticketSvc.Create(CreateTicketInput{
		Title:       "人工服务提单",
		Description: "只落 ticket_created，不启动流程",
		ServiceID:   service.ID,
		PriorityID:  priority.ID,
		FormData:    JSONField(`{"summary":"manual only"}`),
	}, 7)
	if err != nil {
		t.Fatalf("create manual ticket: %v", err)
	}
	if created.EngineType != "manual" || created.Status != TicketStatusSubmitted || created.Source != TicketSourceCatalog {
		t.Fatalf("unexpected created manual ticket: %+v", created)
	}
	if created.CurrentActivityID != nil {
		t.Fatalf("expected no current activity for manual create, got %v", created.CurrentActivityID)
	}

	var ticketCreated TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", created.ID, "ticket_created").First(&ticketCreated).Error; err != nil {
		t.Fatalf("load manual ticket_created timeline: %v", err)
	}
	if ticketCreated.Message != "工单已创建" || ticketCreated.OperatorID != 7 {
		t.Fatalf("unexpected manual ticket_created timeline: %+v", ticketCreated)
	}

	var workflowTimelineCount int64
	if err := db.Model(&TicketTimeline{}).Where("ticket_id = ? AND event_type = ?", created.ID, "workflow_started").Count(&workflowTimelineCount).Error; err != nil {
		t.Fatalf("count manual workflow_started timeline: %v", err)
	}
	if workflowTimelineCount != 0 {
		t.Fatalf("expected manual create to skip workflow_started timeline, got %d", workflowTimelineCount)
	}

	var activityCount int64
	if err := db.Model(&TicketActivity{}).Where("ticket_id = ?", created.ID).Count(&activityCount).Error; err != nil {
		t.Fatalf("count manual activities: %v", err)
	}
	if activityCount != 0 {
		t.Fatalf("expected no activities for manual create, got %d", activityCount)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count manual smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected manual create to enqueue no smart-progress task, got %d", taskCount)
	}
}

func TestAgentDraftSubmission_ClassicServiceCreatesWorkflowWithoutSmartTask(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service, _ := seedClassicSubmissionService(t, db)

	result, err := ticketSvc.CreateFromAgent(context.Background(), tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "经典草稿确认提单",
		FormData:     map[string]any{"request_kind": "network_support"},
		SessionID:    233,
		DraftVersion: 2,
		FieldsHash:   "classic-fields-v1",
		RequestHash:  "classic-request-v1",
	})
	if err != nil {
		t.Fatalf("create classic ticket from agent: %v", err)
	}

	var created Ticket
	if err := db.First(&created, result.TicketID).Error; err != nil {
		t.Fatalf("load created classic ticket: %v", err)
	}
	if created.Source != TicketSourceAgent || created.EngineType != "classic" || created.Status != TicketStatusWaitingHuman {
		t.Fatalf("unexpected created classic agent ticket: %+v", created)
	}
	if created.AgentSessionID == nil || *created.AgentSessionID != 233 {
		t.Fatalf("expected agent session to persist, got %+v", created.AgentSessionID)
	}

	var submission ServiceDeskSubmission
	if err := db.Where("session_id = ? AND draft_version = ? AND fields_hash = ?", 233, 2, "classic-fields-v1").First(&submission).Error; err != nil {
		t.Fatalf("load classic draft submission: %v", err)
	}
	if submission.Status != "submitted" || submission.TicketID != created.ID {
		t.Fatalf("unexpected classic submission: %+v", submission)
	}

	var draftTimeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", created.ID, "draft_submitted").First(&draftTimeline).Error; err != nil {
		t.Fatalf("load classic draft timeline: %v", err)
	}
	var workflowTimeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", created.ID, "workflow_started").First(&workflowTimeline).Error; err != nil {
		t.Fatalf("load classic workflow timeline: %v", err)
	}

	var activity TicketActivity
	if err := db.Where("ticket_id = ? AND activity_type = ?", created.ID, engine.NodeApprove).First(&activity).Error; err != nil {
		t.Fatalf("load classic approval activity: %v", err)
	}
	var assignment TicketAssignment
	if err := db.Where("ticket_id = ? AND activity_id = ?", created.ID, activity.ID).First(&assignment).Error; err != nil {
		t.Fatalf("load classic approval assignment: %v", err)
	}
	if assignment.AssigneeID == nil || *assignment.AssigneeID != 8 || assignment.Status != AssignmentPending {
		t.Fatalf("unexpected classic approval assignment: %+v", assignment)
	}

	var taskCount int64
	if err := db.Model(&model.TaskExecution{}).Where("task_name = ?", "itsm-smart-progress").Count(&taskCount).Error; err != nil {
		t.Fatalf("count smart progress tasks: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("expected classic agent submit to enqueue no smart-progress task, got %d", taskCount)
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

func TestCreateFromAgentRejectsMissingDefaultPriority(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	if err := db.Where("code = ?", "P3").Delete(&Priority{}).Error; err != nil {
		t.Fatalf("delete default P3 priority: %v", err)
	}

	_, err := ticketSvc.CreateFromAgent(context.Background(), tools.AgentTicketRequest{
		UserID:       7,
		ServiceID:    service.ID,
		Summary:      "缺失默认优先级",
		FormData:     map[string]any{"vpn_account": "agent@example.com"},
		SessionID:    401,
		DraftVersion: 1,
		FieldsHash:   "fields-missing-p3",
		RequestHash:  "request-missing-p3",
	})
	if err == nil || !strings.Contains(err.Error(), "default agent priority P3 is not active") {
		t.Fatalf("expected missing default priority error, got %v", err)
	}

	var ticketCount int64
	if err := db.Model(&Ticket{}).Count(&ticketCount).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if ticketCount != 0 {
		t.Fatalf("expected no agent ticket when default priority missing, got %d", ticketCount)
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

func TestTicketProgressTaskDispatchContracts(t *testing.T) {
	db := newTestDB(t)
	ticketSvc := newSubmissionTicketService(t, db)
	service := testutil.SeedSmartSubmissionService(t, db)

	ticket := Ticket{
		Code:        "TICK-PROGRESS-SMART-TASK",
		Title:       "智能人工推进后继续决策",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusWaitingHuman,
		PriorityID:  1,
		RequesterID: 7,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create smart ticket: %v", err)
	}
	activity := TicketActivity{
		TicketID:     ticket.ID,
		Name:         "人工审批",
		ActivityType: engine.NodeApprove,
		Status:       engine.ActivityPending,
		NodeID:       "approve",
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create smart activity: %v", err)
	}
	if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
		t.Fatalf("set current activity: %v", err)
	}
	assigneeID := uint(7)
	if err := db.Create(&TicketAssignment{
		TicketID:   ticket.ID,
		ActivityID: activity.ID,
		UserID:     &assigneeID,
		AssigneeID: &assigneeID,
		Status:     AssignmentPending,
		IsCurrent:  true,
	}).Error; err != nil {
		t.Fatalf("create smart assignment: %v", err)
	}

	progressed, err := ticketSvc.Progress(ticket.ID, activity.ID, "approved", "同意，继续交给 AI", nil, assigneeID)
	if err != nil {
		t.Fatalf("progress smart ticket: %v", err)
	}
	if progressed == nil || !isDecisioningStatus(progressed.Status) {
		t.Fatalf("expected smart ticket to re-enter decisioning, got %+v", progressed)
	}

	var tasks []model.TaskExecution
	if err := db.Where("task_name = ?", "itsm-smart-progress").Order("id asc").Find(&tasks).Error; err != nil {
		t.Fatalf("load smart progress tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected exactly one smart progress task, got %d", len(tasks))
	}
	var payload engine.SmartProgressPayload
	if err := json.Unmarshal([]byte(tasks[0].Payload), &payload); err != nil {
		t.Fatalf("decode smart progress payload: %v", err)
	}
	if payload.TicketID != ticket.ID || payload.CompletedActivityID == nil || *payload.CompletedActivityID != activity.ID || payload.TriggerReason != engine.TriggerReasonActivityApprove {
		t.Fatalf("unexpected smart progress payload: %+v", payload)
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

func TestTicketHandlerCreateMapsValidationAndServiceGuards(t *testing.T) {
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

	t.Run("bad json is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", bytes.NewBufferString(`{"title":`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("bad json status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing service maps to 404", func(t *testing.T) {
		body := bytes.NewBufferString(`{"title":"缺失服务","serviceId":999999,"priorityId":` + jsonNumber(priority.ID) + `,"formData":{}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("missing service status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("inactive service and missing priority map to 400", func(t *testing.T) {
		if err := db.Model(&ServiceDefinition{}).Where("id = ?", service.ID).Update("is_active", false).Error; err != nil {
			t.Fatalf("deactivate service: %v", err)
		}
		body := bytes.NewBufferString(`{"title":"停用服务","serviceId":` + jsonNumber(service.ID) + `,"priorityId":` + jsonNumber(priority.ID) + `,"formData":{}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("inactive service status=%d body=%s", rec.Code, rec.Body.String())
		}

		if err := db.Model(&ServiceDefinition{}).Where("id = ?", service.ID).Update("is_active", true).Error; err != nil {
			t.Fatalf("reactivate service: %v", err)
		}
		body = bytes.NewBufferString(`{"title":"缺失优先级","serviceId":` + jsonNumber(service.ID) + `,"priorityId":999999,"formData":{}}`)
		req = httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", body)
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("missing priority status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
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
	assigneeID := uint(12)
	ticket := Ticket{
		Code:           "TICK-RETRY-AI-SQLITE",
		Title:          "智能审批重试",
		ServiceID:      service.ID,
		EngineType:     "smart",
		Status:         TicketStatusWaitingHuman,
		PriorityID:     1,
		RequesterID:    1,
		AIFailureCount: 3,
		AssigneeID:     &assigneeID,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	activity := TicketActivity{
		TicketID:     ticket.ID,
		Name:         "人工审批",
		ActivityType: engine.NodeApprove,
		Status:       engine.ActivityInProgress,
		StartedAt:    ptrTime(time.Now().Add(-10 * time.Minute)),
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
		t.Fatalf("set current activity: %v", err)
	}
	if err := db.Create(&TicketAssignment{
		TicketID:   ticket.ID,
		ActivityID: activity.ID,
		UserID:     &assigneeID,
		AssigneeID: &assigneeID,
		Status:     AssignmentInProgress,
		IsCurrent:  true,
		ClaimedAt:  ptrTime(time.Now().Add(-5 * time.Minute)),
	}).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
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
	if reloaded.Status != TicketStatusDecisioning || reloaded.CurrentActivityID != nil || reloaded.AssigneeID != nil {
		t.Fatalf("expected retry to clear current manual ownership, got %+v", reloaded)
	}

	var timeline TicketTimeline
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "ai_retry").First(&timeline).Error; err != nil {
		t.Fatalf("load retry timeline: %v", err)
	}

	var reloadedActivity TicketActivity
	if err := db.First(&reloadedActivity, activity.ID).Error; err != nil {
		t.Fatalf("reload activity: %v", err)
	}
	if reloadedActivity.Status != engine.ActivityCancelled || reloadedActivity.FinishedAt == nil {
		t.Fatalf("expected retry to cancel old activity, got %+v", reloadedActivity)
	}

	var reloadedAssignment TicketAssignment
	if err := db.Where("ticket_id = ? AND activity_id = ?", ticket.ID, activity.ID).First(&reloadedAssignment).Error; err != nil {
		t.Fatalf("reload assignment: %v", err)
	}
	if reloadedAssignment.Status != AssignmentCancelled || reloadedAssignment.IsCurrent {
		t.Fatalf("expected retry to cancel old assignment, got %+v", reloadedAssignment)
	}

	var tasks []model.TaskExecution
	if err := db.Where("task_name = ?", "itsm-smart-progress").Find(&tasks).Error; err != nil {
		t.Fatalf("load smart progress tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected retry to submit one smart-progress task, got %d", len(tasks))
	}
	var payload engine.SmartProgressPayload
	if err := json.Unmarshal([]byte(tasks[0].Payload), &payload); err != nil {
		t.Fatalf("decode smart progress payload: %v", err)
	}
	if payload.TicketID != ticket.ID || payload.TriggerReason != engine.TriggerReasonManualRetry {
		t.Fatalf("unexpected retry payload: %+v", payload)
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

func TestTicketService_CancelContracts(t *testing.T) {
	t.Run("manual ticket cancel writes terminal outcome and timeline", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)
		assigneeID := uint(21)

		ticket := Ticket{
			Code:        "TICK-MANUAL-CANCEL",
			Title:       "手工取消",
			ServiceID:   service.ID,
			EngineType:  "manual",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &assigneeID,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create manual ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "手工处理",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityInProgress,
			StartedAt:    ptrTime(time.Now().Add(-5 * time.Minute)),
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create manual activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &assigneeID,
			AssigneeID: &assigneeID,
			Status:     AssignmentInProgress,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create manual assignment: %v", err)
		}

		cancelled, err := ticketSvc.Cancel(ticket.ID, "无需继续处理", 99)
		if err != nil {
			t.Fatalf("cancel manual ticket: %v", err)
		}
		if cancelled.Status != TicketStatusCancelled || cancelled.Outcome != TicketOutcomeCancelled || cancelled.FinishedAt == nil {
			t.Fatalf("unexpected cancelled ticket: %+v", cancelled)
		}
		if cancelled.AssigneeID != nil || cancelled.CurrentActivityID != nil {
			t.Fatalf("expected manual cancel to clear current ownership, got %+v", cancelled)
		}

		var cancelledActivity TicketActivity
		if err := db.First(&cancelledActivity, activity.ID).Error; err != nil {
			t.Fatalf("reload manual activity: %v", err)
		}
		if cancelledActivity.Status != engine.ActivityCancelled || cancelledActivity.FinishedAt == nil {
			t.Fatalf("expected manual cancel to cancel activity, got %+v", cancelledActivity)
		}

		var assignment TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ?", ticket.ID, activity.ID).First(&assignment).Error; err != nil {
			t.Fatalf("reload manual assignment: %v", err)
		}
		if assignment.Status != AssignmentCancelled || assignment.IsCurrent {
			t.Fatalf("expected manual cancel to close assignment, got %+v", assignment)
		}

		var timeline TicketTimeline
		if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "ticket_cancelled").First(&timeline).Error; err != nil {
			t.Fatalf("load manual cancel timeline: %v", err)
		}
		if !strings.Contains(timeline.Message, "无需继续处理") {
			t.Fatalf("expected cancel reason in timeline, got %q", timeline.Message)
		}
	})

	t.Run("smart ticket cancel delegates to engine cleanup", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		assigneeID := uint(11)
		ticket := Ticket{
			Code:        "TICK-SMART-CANCEL",
			Title:       "智能取消",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &assigneeID,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create smart ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "人工审批",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityPending,
			StartedAt:    ptrTime(time.Now().Add(-10 * time.Minute)),
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create smart activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &assigneeID,
			AssigneeID: &assigneeID,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create pending assignment: %v", err)
		}

		cancelled, err := ticketSvc.Cancel(ticket.ID, "智能流程终止", 99)
		if err != nil {
			t.Fatalf("cancel smart ticket: %v", err)
		}
		if cancelled.Status != TicketStatusCancelled || cancelled.Outcome != TicketOutcomeCancelled || cancelled.FinishedAt == nil {
			t.Fatalf("unexpected smart cancelled ticket: %+v", cancelled)
		}

		var reloadedActivity TicketActivity
		if err := db.First(&reloadedActivity, activity.ID).Error; err != nil {
			t.Fatalf("reload activity: %v", err)
		}
		if reloadedActivity.Status != engine.ActivityCancelled || reloadedActivity.FinishedAt == nil {
			t.Fatalf("expected engine cancel to cancel activity, got %+v", reloadedActivity)
		}

		var assignment TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ?", ticket.ID, activity.ID).First(&assignment).Error; err != nil {
			t.Fatalf("reload assignment: %v", err)
		}
		if assignment.Status != engine.ActivityCancelled {
			t.Fatalf("expected engine cancel to cancel assignment, got %+v", assignment)
		}

		var timeline TicketTimeline
		if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "ticket_cancelled").First(&timeline).Error; err != nil {
			t.Fatalf("load smart cancel timeline: %v", err)
		}
		if !strings.Contains(timeline.Message, "智能流程终止") {
			t.Fatalf("expected smart cancel reason in timeline, got %q", timeline.Message)
		}
	})

	t.Run("terminal tickets reject cancel", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)
		ticket := Ticket{
			Code:        "TICK-CANCEL-TERMINAL",
			Title:       "终态取消",
			ServiceID:   service.ID,
			EngineType:  "manual",
			Status:      TicketStatusCompleted,
			Outcome:     TicketOutcomeApproved,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create terminal ticket: %v", err)
		}

		if _, err := ticketSvc.Cancel(ticket.ID, "终态不应再取消", 99); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("expected ErrTicketTerminal, got %v", err)
		}
	})
}

func TestTicketService_HandoffHumanAndManualActivityContracts(t *testing.T) {
	t.Run("handoffHuman rejects non-smart tickets and missing operator", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		manualTicket := Ticket{
			Code:        "TICK-HANDOFF-MANUAL",
			Title:       "非智能转人工",
			ServiceID:   service.ID,
			EngineType:  "manual",
			Status:      TicketStatusDecisioning,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&manualTicket).Error; err != nil {
			t.Fatalf("create manual ticket: %v", err)
		}

		if _, err := ticketSvc.handoffHuman(manualTicket.ID, "不支持", 7); err == nil || !strings.Contains(err.Error(), "only available for smart") {
			t.Fatalf("expected non-smart rejection, got %v", err)
		}

		smartTicket := Ticket{
			Code:        "TICK-HANDOFF-NO-OP",
			Title:       "缺少操作人",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusDecisioning,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&smartTicket).Error; err != nil {
			t.Fatalf("create smart ticket: %v", err)
		}
		if _, err := ticketSvc.handoffHuman(smartTicket.ID, "需要人工", 0); err == nil || !strings.Contains(err.Error(), "operator is required") {
			t.Fatalf("expected missing operator rejection, got %v", err)
		}
	})

	t.Run("handoffHuman deduplicates recent recovery action", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		operatorID := uint(19)
		ticket := Ticket{
			Code:        "TICK-HANDOFF-DEDUP",
			Title:       "转人工去重",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &operatorID,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create smart ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "原人工处理",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityPending,
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &operatorID,
			AssigneeID: &operatorID,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create assignment: %v", err)
		}
		timeline := &TicketTimeline{
			TicketID:   ticket.ID,
			OperatorID: operatorID,
			EventType:  "recovery_handoff_human",
			Message:    "刚刚转人工",
		}
		if err := db.Create(timeline).Error; err != nil {
			t.Fatalf("seed recovery timeline: %v", err)
		}
		if err := db.Model(timeline).Update("created_at", time.Now()).Error; err != nil {
			t.Fatalf("backdate recovery timeline: %v", err)
		}

		if _, err := ticketSvc.handoffHuman(ticket.ID, "重复转人工", operatorID); !errors.Is(err, ErrRecoveryActionTooFrequent) {
			t.Fatalf("expected ErrRecoveryActionTooFrequent, got %v", err)
		}
	})

	t.Run("ticketStatusForManualActivity maps human and action nodes distinctly", func(t *testing.T) {
		if got := ticketStatusForManualActivity(engine.NodeAction); got != TicketStatusExecutingAction {
			t.Fatalf("action status = %q, want %q", got, TicketStatusExecutingAction)
		}
		for _, activityType := range []string{engine.NodeApprove, engine.NodeForm, engine.NodeProcess} {
			if got := ticketStatusForManualActivity(activityType); got != TicketStatusWaitingHuman {
				t.Fatalf("%s status = %q, want %q", activityType, got, TicketStatusWaitingHuman)
			}
		}
		if got := ticketStatusForManualActivity("unknown"); got != TicketStatusDecisioning {
			t.Fatalf("unknown status = %q, want %q", got, TicketStatusDecisioning)
		}
	})
}

func TestTicketService_SignalContracts(t *testing.T) {
	t.Run("terminal ticket rejects signal", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)
		ticket := Ticket{
			Code:        "TICK-SIGNAL-TERMINAL",
			Title:       "终态 signal",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusCompleted,
			Outcome:     TicketOutcomeFulfilled,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create terminal ticket: %v", err)
		}
		if _, err := ticketSvc.Signal(ticket.ID, 1, "done", nil, 11); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("expected ErrTicketTerminal, got %v", err)
		}
	})

	t.Run("non-wait activity is rejected", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)
		ticket := Ticket{
			Code:        "TICK-SIGNAL-NONWAIT",
			Title:       "非 wait signal",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "人工处理", ActivityType: engine.NodeProcess, Status: engine.ActivityPending}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create non-wait activity: %v", err)
		}

		if _, err := ticketSvc.Signal(ticket.ID, activity.ID, "done", nil, 11); !errors.Is(err, ErrActivityNotWait) {
			t.Fatalf("expected ErrActivityNotWait, got %v", err)
		}
	})

	t.Run("wait activity signal completes activity and moves ticket into decisioning", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		operatorID := uint(11)
		ticket := Ticket{
			Code:        "TICK-SIGNAL-WAIT",
			Title:       "wait signal",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "等待回调", ActivityType: engine.NodeWait, Status: engine.ActivityPending}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create wait activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &operatorID,
			AssigneeID: &operatorID,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create wait assignment: %v", err)
		}

		payload := JSONField(`{"callback":"done","result":"ok"}`)
		signaled, err := ticketSvc.Signal(ticket.ID, activity.ID, "done", json.RawMessage(payload), operatorID)
		if err != nil {
			t.Fatalf("signal wait activity: %v", err)
		}
		if signaled.Status != TicketStatusDecisioning || signaled.CurrentActivityID != nil {
			t.Fatalf("expected decisioning ticket with cleared current activity, got %+v", signaled)
		}

		var reloadedActivity TicketActivity
		if err := db.First(&reloadedActivity, activity.ID).Error; err != nil {
			t.Fatalf("reload activity: %v", err)
		}
		if reloadedActivity.Status != engine.ActivityApproved || reloadedActivity.TransitionOutcome != "done" || string(reloadedActivity.FormData) != string(payload) {
			t.Fatalf("unexpected signaled activity: %+v", reloadedActivity)
		}

		var assignment TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ?", ticket.ID, activity.ID).First(&assignment).Error; err != nil {
			t.Fatalf("reload assignment: %v", err)
		}
		if assignment.Status != engine.ActivityApproved || assignment.AssigneeID == nil || *assignment.AssigneeID != operatorID {
			t.Fatalf("unexpected signaled assignment: %+v", assignment)
		}

		var timeline TicketTimeline
		if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "activity_completed").First(&timeline).Error; err != nil {
			t.Fatalf("load activity_completed timeline: %v", err)
		}
		if !strings.Contains(timeline.Message, "结果: done") {
			t.Fatalf("expected timeline to mention signal outcome, got %q", timeline.Message)
		}
	})
}

func TestTicketService_WithdrawContracts(t *testing.T) {
	t.Run("terminal ticket rejects withdraw", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)
		ticket := Ticket{
			Code:        "TICK-WITHDRAW-TERMINAL",
			Title:       "terminal withdraw",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusCompleted,
			Outcome:     TicketOutcomeFulfilled,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create terminal ticket: %v", err)
		}

		if _, err := ticketSvc.Withdraw(ticket.ID, "too late", 7); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("expected ErrTicketTerminal, got %v", err)
		}
	})

	t.Run("non requester rejects withdraw", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)
		ticket := Ticket{
			Code:        "TICK-WITHDRAW-FORBIDDEN",
			Title:       "forbidden withdraw",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}

		if _, err := ticketSvc.Withdraw(ticket.ID, "not mine", 8); !errors.Is(err, ErrNotRequester) {
			t.Fatalf("expected ErrNotRequester, got %v", err)
		}
	})

	t.Run("claimed assignment rejects withdraw without mutation", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-WITHDRAW-CLAIMED",
			Title:       "claimed withdraw",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "已认领活动", ActivityType: engine.NodeApprove, Status: engine.ActivityInProgress}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		assigneeID := uint(9)
		claimedAt := time.Now().Add(-time.Minute)
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &assigneeID,
			AssigneeID: &assigneeID,
			Status:     AssignmentInProgress,
			IsCurrent:  true,
			ClaimedAt:  &claimedAt,
		}).Error; err != nil {
			t.Fatalf("create claimed assignment: %v", err)
		}

		if _, err := ticketSvc.Withdraw(ticket.ID, "too late", 7); !errors.Is(err, ErrTicketClaimed) {
			t.Fatalf("expected ErrTicketClaimed, got %v", err)
		}

		var reloaded Ticket
		if err := db.First(&reloaded, ticket.ID).Error; err != nil {
			t.Fatalf("reload ticket: %v", err)
		}
		if reloaded.Status != TicketStatusWaitingHuman || reloaded.Outcome != "" || reloaded.FinishedAt != nil {
			t.Fatalf("claimed withdraw mutated ticket: %+v", reloaded)
		}

		var timelineCount int64
		if err := db.Model(&TicketTimeline{}).Where("ticket_id = ? AND event_type = ?", ticket.ID, "withdrawn").Count(&timelineCount).Error; err != nil {
			t.Fatalf("count withdrawn timeline: %v", err)
		}
		if timelineCount != 0 {
			t.Fatalf("expected no withdrawn timeline for claimed ticket, got %d", timelineCount)
		}
	})

	t.Run("requester withdraw cancels workflow state and records timeline", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-WITHDRAW-SMART",
			Title:       "smart withdraw",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "待处理活动", ActivityType: engine.NodeProcess, Status: engine.ActivityPending}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		assigneeID := uint(12)
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &assigneeID,
			AssigneeID: &assigneeID,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create assignment: %v", err)
		}

		withdrawn, err := ticketSvc.Withdraw(ticket.ID, "申请有误", 7)
		if err != nil {
			t.Fatalf("withdraw ticket: %v", err)
		}
		if withdrawn.Status != TicketStatusWithdrawn || withdrawn.Outcome != TicketOutcomeWithdrawn || withdrawn.FinishedAt == nil {
			t.Fatalf("unexpected withdrawn ticket: %+v", withdrawn)
		}

		var reloadedActivity TicketActivity
		if err := db.First(&reloadedActivity, activity.ID).Error; err != nil {
			t.Fatalf("reload activity: %v", err)
		}
		if reloadedActivity.Status != engine.ActivityCancelled || reloadedActivity.FinishedAt == nil {
			t.Fatalf("expected activity cancelled by withdraw, got %+v", reloadedActivity)
		}

		var assignment TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ?", ticket.ID, activity.ID).First(&assignment).Error; err != nil {
			t.Fatalf("reload assignment: %v", err)
		}
		if assignment.Status != AssignmentCancelled {
			t.Fatalf("expected assignment cancelled by withdraw, got %+v", assignment)
		}

		var activeAssignmentCount int64
		if err := db.Model(&TicketAssignment{}).
			Where("ticket_id = ? AND status IN ?", ticket.ID, []string{AssignmentPending, AssignmentInProgress}).
			Count(&activeAssignmentCount).Error; err != nil {
			t.Fatalf("count active assignments: %v", err)
		}
		if activeAssignmentCount != 0 {
			t.Fatalf("expected no actionable assignments after withdraw, got %d", activeAssignmentCount)
		}

		var timeline TicketTimeline
		if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "withdrawn").First(&timeline).Error; err != nil {
			t.Fatalf("load withdrawn timeline: %v", err)
		}
		if !strings.Contains(timeline.Message, "申请有误") {
			t.Fatalf("expected withdrawn timeline to include reason, got %q", timeline.Message)
		}
	})

	t.Run("manual ticket withdraw writes terminal outcome and timeline", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-WITHDRAW-MANUAL",
			Title:       "manual withdraw",
			ServiceID:   service.ID,
			EngineType:  "manual",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create manual ticket: %v", err)
		}

		withdrawn, err := ticketSvc.Withdraw(ticket.ID, "无需继续处理", 7)
		if err != nil {
			t.Fatalf("withdraw manual ticket: %v", err)
		}
		if withdrawn.Status != TicketStatusWithdrawn || withdrawn.Outcome != TicketOutcomeWithdrawn || withdrawn.FinishedAt == nil {
			t.Fatalf("unexpected manual withdrawn ticket: %+v", withdrawn)
		}

		var timeline TicketTimeline
		if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "withdrawn").First(&timeline).Error; err != nil {
			t.Fatalf("load manual withdrawn timeline: %v", err)
		}
		if !strings.Contains(timeline.Message, "无需继续处理") {
			t.Fatalf("expected manual withdrawn timeline to include reason, got %q", timeline.Message)
		}
	})
}

func TestTicketService_AssignmentRoutingContracts(t *testing.T) {
	t.Run("assign updates assignee and writes timeline", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-ASSIGN-SVC",
			Title:       "assign service",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusDecisioning,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:      ticket.ID,
			Name:          "待指派活动",
			ActivityType:  engine.NodeProcess,
			Status:        engine.ActivityPending,
			ExecutionMode: "single",
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("seed current activity: %v", err)
		}
		oldAssigneeID := uint(20)
		claimedAt := time.Now().Add(-5 * time.Minute)
		positionID := uint(7)
		departmentID := uint(9)
		assignment := TicketAssignment{
			TicketID:        ticket.ID,
			ActivityID:      activity.ID,
			ParticipantType: "position_department",
			UserID:          &oldAssigneeID,
			PositionID:      &positionID,
			DepartmentID:    &departmentID,
			AssigneeID:      &oldAssigneeID,
			Status:          AssignmentInProgress,
			IsCurrent:       true,
			ClaimedAt:       &claimedAt,
		}
		if err := db.Create(&assignment).Error; err != nil {
			t.Fatalf("create assignment: %v", err)
		}

		assigned, err := ticketSvc.Assign(ticket.ID, 33, 99)
		if err != nil {
			t.Fatalf("Assign: %v", err)
		}
		if assigned.AssigneeID == nil || *assigned.AssigneeID != 33 || assigned.Status != TicketStatusWaitingHuman {
			t.Fatalf("unexpected assigned ticket: %+v", assigned)
		}
		var reassigned TicketAssignment
		if err := db.First(&reassigned, assignment.ID).Error; err != nil {
			t.Fatalf("reload assignment: %v", err)
		}
		if reassigned.ParticipantType != "user" || reassigned.UserID == nil || *reassigned.UserID != 33 || reassigned.AssigneeID == nil || *reassigned.AssigneeID != 33 {
			t.Fatalf("assignment should retarget current owner, got %+v", reassigned)
		}
		// position_id and department_id are intentionally preserved after Assign() so
		// that deterministic service guards can still identify the completed step by role
		// (ticketHasSatisfiedPositionProcess uses INNER JOIN on position_id).
		if reassigned.PositionID == nil || *reassigned.PositionID != positionID {
			t.Fatalf("assignment should preserve position_id for guard queries, got %+v", reassigned)
		}
		if reassigned.DepartmentID == nil || *reassigned.DepartmentID != departmentID {
			t.Fatalf("assignment should preserve department_id for guard queries, got %+v", reassigned)
		}
		if reassigned.Status != AssignmentPending || reassigned.ClaimedAt != nil {
			t.Fatalf("assignment should return to pending unclaimed state, got %+v", reassigned)
		}

		var timeline TicketTimeline
		if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "ticket_assigned").First(&timeline).Error; err != nil {
			t.Fatalf("load assign timeline: %v", err)
		}
	})

	t.Run("assign rejects workflow ticket without active current assignment", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-ASSIGN-NO-ACTIVE",
			Title:       "assign no active assignment",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:      ticket.ID,
			Name:          "无人接手活动",
			ActivityType:  engine.NodeProcess,
			Status:        engine.ActivityPending,
			ExecutionMode: "single",
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("seed current activity: %v", err)
		}

		if _, err := ticketSvc.Assign(ticket.ID, 33, 99); !errors.Is(err, ErrNoActiveAssignment) {
			t.Fatalf("Assign missing active assignment error = %v, want %v", err, ErrNoActiveAssignment)
		}

		var reloaded Ticket
		if err := db.First(&reloaded, ticket.ID).Error; err != nil {
			t.Fatalf("reload ticket: %v", err)
		}
		if reloaded.AssigneeID != nil || reloaded.Status != TicketStatusWaitingHuman {
			t.Fatalf("assign should not mutate ticket without active assignment, got %+v", reloaded)
		}
	})

	t.Run("assign coalesces competing current assignments to one assignee", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-ASSIGN-COALESCE",
			Title:       "assign coalesces current assignments",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:      ticket.ID,
			Name:          "多人待办活动",
			ActivityType:  engine.NodeProcess,
			Status:        engine.ActivityPending,
			ExecutionMode: "single",
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("seed current activity: %v", err)
		}

		firstUserID := uint(20)
		secondUserID := uint(21)
		first := TicketAssignment{
			TicketID:        ticket.ID,
			ActivityID:      activity.ID,
			ParticipantType: "user",
			UserID:          &firstUserID,
			AssigneeID:      &firstUserID,
			Status:          AssignmentPending,
			IsCurrent:       true,
		}
		second := TicketAssignment{
			TicketID:        ticket.ID,
			ActivityID:      activity.ID,
			ParticipantType: "user",
			UserID:          &secondUserID,
			AssigneeID:      &secondUserID,
			Status:          AssignmentClaimedByOther,
			IsCurrent:       true,
			ClaimedAt:       ptrTime(time.Now().Add(-2 * time.Minute)),
		}
		if err := db.Create(&first).Error; err != nil {
			t.Fatalf("create first assignment: %v", err)
		}
		if err := db.Create(&second).Error; err != nil {
			t.Fatalf("create second assignment: %v", err)
		}

		assigned, err := ticketSvc.Assign(ticket.ID, 33, 99)
		if err != nil {
			t.Fatalf("Assign with competing current assignments: %v", err)
		}
		if assigned.AssigneeID == nil || *assigned.AssigneeID != 33 {
			t.Fatalf("ticket assignee = %v, want 33", assigned.AssigneeID)
		}

		var reloadedFirst TicketAssignment
		if err := db.First(&reloadedFirst, first.ID).Error; err != nil {
			t.Fatalf("reload first assignment: %v", err)
		}
		if reloadedFirst.Status != AssignmentPending || !reloadedFirst.IsCurrent || reloadedFirst.UserID == nil || *reloadedFirst.UserID != 33 || reloadedFirst.AssigneeID == nil || *reloadedFirst.AssigneeID != 33 || reloadedFirst.ClaimedAt != nil {
			t.Fatalf("first assignment should become sole current assignee, got %+v", reloadedFirst)
		}

		var reloadedSecond TicketAssignment
		if err := db.First(&reloadedSecond, second.ID).Error; err != nil {
			t.Fatalf("reload second assignment: %v", err)
		}
		if reloadedSecond.Status != AssignmentCancelled || reloadedSecond.IsCurrent || reloadedSecond.FinishedAt == nil {
			t.Fatalf("second assignment should be cancelled, got %+v", reloadedSecond)
		}

		var currentCount int64
		if err := db.Model(&TicketAssignment{}).Where("ticket_id = ? AND activity_id = ? AND is_current = ?", ticket.ID, activity.ID, true).Count(&currentCount).Error; err != nil {
			t.Fatalf("count current assignments: %v", err)
		}
		if currentCount != 1 {
			t.Fatalf("expected one current assignment after assign, got %d", currentCount)
		}
	})

	t.Run("claim marks operator in progress and suppresses competing assignments", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-CLAIM-SVC",
			Title:       "claim service",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "claim", ActivityType: engine.NodeProcess, Status: engine.ActivityPending}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		operatorID := uint(11)
		otherID := uint(12)
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &operatorID,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create operator assignment: %v", err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &otherID,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create competing assignment: %v", err)
		}

		claimed, err := ticketSvc.Claim(ticket.ID, activity.ID, operatorID)
		if err != nil {
			t.Fatalf("Claim: %v", err)
		}
		if claimed.AssigneeID == nil || *claimed.AssigneeID != operatorID || claimed.Status != TicketStatusWaitingHuman {
			t.Fatalf("unexpected claimed ticket: %+v", claimed)
		}

		var mine TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND user_id = ?", ticket.ID, activity.ID, operatorID).First(&mine).Error; err != nil {
			t.Fatalf("reload claimed assignment: %v", err)
		}
		if mine.Status != AssignmentInProgress || mine.AssigneeID == nil || *mine.AssigneeID != operatorID || mine.ClaimedAt == nil {
			t.Fatalf("unexpected claimed assignment: %+v", mine)
		}

		var other TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND user_id = ?", ticket.ID, activity.ID, otherID).First(&other).Error; err != nil {
			t.Fatalf("reload competing assignment: %v", err)
		}
		if other.Status != AssignmentClaimedByOther {
			t.Fatalf("expected competing assignment claimed_by_other, got %+v", other)
		}
	})

	t.Run("transfer and delegate preserve lineage", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-ROUTE-SVC",
			Title:       "route service",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "route", ActivityType: engine.NodeProcess, Status: engine.ActivityPending}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		operatorID := uint(21)
		transferTarget := uint(22)
		delegateTarget := uint(23)
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &operatorID,
			AssigneeID: &operatorID,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create original assignment: %v", err)
		}

		transferred, err := ticketSvc.Transfer(ticket.ID, activity.ID, transferTarget, operatorID)
		if err != nil {
			t.Fatalf("Transfer: %v", err)
		}
		if transferred.AssigneeID == nil || *transferred.AssigneeID != transferTarget {
			t.Fatalf("unexpected transferred ticket: %+v", transferred)
		}

		var transferredOriginal TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, operatorID).First(&transferredOriginal).Error; err != nil {
			t.Fatalf("reload transferred original: %v", err)
		}
		if transferredOriginal.Status != AssignmentTransferred || transferredOriginal.IsCurrent {
			t.Fatalf("expected original assignment transferred, got %+v", transferredOriginal)
		}

		var transferredNew TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, transferTarget).First(&transferredNew).Error; err != nil {
			t.Fatalf("reload transferred target: %v", err)
		}
		if transferredNew.TransferFrom == nil || *transferredNew.TransferFrom != transferredOriginal.ID || transferredNew.Status != AssignmentPending || !transferredNew.IsCurrent {
			t.Fatalf("unexpected transferred target assignment: %+v", transferredNew)
		}

		delegated, err := ticketSvc.Delegate(ticket.ID, activity.ID, delegateTarget, transferTarget)
		if err != nil {
			t.Fatalf("Delegate: %v", err)
		}
		if delegated.AssigneeID == nil || *delegated.AssigneeID != transferTarget {
			t.Fatalf("delegate should not rewrite ticket assignee eagerly, got %+v", delegated)
		}

		var delegatedOriginal TicketAssignment
		if err := db.Where("id = ?", transferredNew.ID).First(&delegatedOriginal).Error; err != nil {
			t.Fatalf("reload delegated original: %v", err)
		}
		if delegatedOriginal.Status != AssignmentDelegated || delegatedOriginal.IsCurrent {
			t.Fatalf("expected transferred target assignment delegated, got %+v", delegatedOriginal)
		}

		var delegatedNew TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, delegateTarget).First(&delegatedNew).Error; err != nil {
			t.Fatalf("reload delegated target: %v", err)
		}
		if delegatedNew.DelegatedFrom == nil || *delegatedNew.DelegatedFrom != delegatedOriginal.ID || delegatedNew.Status != AssignmentPending || !delegatedNew.IsCurrent {
			t.Fatalf("unexpected delegated target assignment: %+v", delegatedNew)
		}
	})

	t.Run("transfer and delegate work for claimed in-progress assignment", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-ROUTE-INPROGRESS",
			Title:       "route in progress",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "route", ActivityType: engine.NodeProcess, Status: engine.ActivityInProgress}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		operatorID := uint(31)
		transferTarget := uint(32)
		delegateTarget := uint(33)
		claimedAt := time.Now().Add(-time.Minute)
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &operatorID,
			AssigneeID: &operatorID,
			Status:     AssignmentInProgress,
			IsCurrent:  true,
			ClaimedAt:  &claimedAt,
		}).Error; err != nil {
			t.Fatalf("create in-progress assignment: %v", err)
		}

		transferred, err := ticketSvc.Transfer(ticket.ID, activity.ID, transferTarget, operatorID)
		if err != nil {
			t.Fatalf("Transfer in-progress: %v", err)
		}
		if transferred.AssigneeID == nil || *transferred.AssigneeID != transferTarget {
			t.Fatalf("unexpected transferred ticket: %+v", transferred)
		}

		var transferredOriginal TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, operatorID).First(&transferredOriginal).Error; err != nil {
			t.Fatalf("reload transferred original: %v", err)
		}
		if transferredOriginal.Status != AssignmentTransferred || transferredOriginal.IsCurrent {
			t.Fatalf("expected original in-progress assignment transferred, got %+v", transferredOriginal)
		}

		var transferredNew TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, transferTarget).First(&transferredNew).Error; err != nil {
			t.Fatalf("reload transferred target: %v", err)
		}
		if transferredNew.TransferFrom == nil || *transferredNew.TransferFrom != transferredOriginal.ID || transferredNew.Status != AssignmentPending || !transferredNew.IsCurrent {
			t.Fatalf("unexpected transferred target assignment: %+v", transferredNew)
		}

		delegated, err := ticketSvc.Delegate(ticket.ID, activity.ID, delegateTarget, transferTarget)
		if err != nil {
			t.Fatalf("Delegate in-progress lineage: %v", err)
		}
		if delegated.AssigneeID == nil || *delegated.AssigneeID != transferTarget {
			t.Fatalf("delegate should keep current ticket assignee until delegate acts, got %+v", delegated)
		}

		var delegatedOriginal TicketAssignment
		if err := db.Where("id = ?", transferredNew.ID).First(&delegatedOriginal).Error; err != nil {
			t.Fatalf("reload delegated original: %v", err)
		}
		if delegatedOriginal.Status != AssignmentDelegated || delegatedOriginal.IsCurrent {
			t.Fatalf("expected transferred target assignment delegated, got %+v", delegatedOriginal)
		}

		var delegatedNew TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, delegateTarget).First(&delegatedNew).Error; err != nil {
			t.Fatalf("reload delegated target: %v", err)
		}
		if delegatedNew.DelegatedFrom == nil || *delegatedNew.DelegatedFrom != delegatedOriginal.ID || delegatedNew.Status != AssignmentPending || !delegatedNew.IsCurrent {
			t.Fatalf("unexpected delegated target assignment: %+v", delegatedNew)
		}
	})

	t.Run("delegate directly from claimed in-progress assignment", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-DELEGATE-INPROGRESS",
			Title:       "delegate in progress",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "delegate", ActivityType: engine.NodeProcess, Status: engine.ActivityInProgress}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		operatorID := uint(41)
		delegateTarget := uint(42)
		claimedAt := time.Now().Add(-time.Minute)
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &operatorID,
			AssigneeID: &operatorID,
			Status:     AssignmentInProgress,
			IsCurrent:  true,
			ClaimedAt:  &claimedAt,
		}).Error; err != nil {
			t.Fatalf("create in-progress assignment: %v", err)
		}

		delegated, err := ticketSvc.Delegate(ticket.ID, activity.ID, delegateTarget, operatorID)
		if err != nil {
			t.Fatalf("Delegate in-progress direct: %v", err)
		}
		if delegated.AssigneeID != nil && *delegated.AssigneeID != operatorID {
			t.Fatalf("delegate should not rewrite ticket assignee eagerly, got %+v", delegated)
		}

		var delegatedOriginal TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, operatorID).First(&delegatedOriginal).Error; err != nil {
			t.Fatalf("reload delegated original: %v", err)
		}
		if delegatedOriginal.Status != AssignmentDelegated || delegatedOriginal.IsCurrent {
			t.Fatalf("expected original in-progress assignment delegated, got %+v", delegatedOriginal)
		}

		var delegatedNew TicketAssignment
		if err := db.Where("ticket_id = ? AND activity_id = ? AND assignee_id = ?", ticket.ID, activity.ID, delegateTarget).First(&delegatedNew).Error; err != nil {
			t.Fatalf("reload delegated target: %v", err)
		}
		if delegatedNew.DelegatedFrom == nil || *delegatedNew.DelegatedFrom != delegatedOriginal.ID || delegatedNew.Status != AssignmentPending || !delegatedNew.IsCurrent {
			t.Fatalf("unexpected delegated target assignment: %+v", delegatedNew)
		}
	})

	t.Run("route operations reject missing active assignment", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		ticket := Ticket{
			Code:        "TICK-ROUTE-GUARD",
			Title:       "route guard",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "guard", ActivityType: engine.NodeProcess, Status: engine.ActivityPending}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}

		if _, err := ticketSvc.Transfer(ticket.ID, activity.ID, 30, 29); !errors.Is(err, ErrNoActiveAssignment) {
			t.Fatalf("Transfer missing assignment error = %v, want %v", err, ErrNoActiveAssignment)
		}
		if _, err := ticketSvc.Delegate(ticket.ID, activity.ID, 30, 29); !errors.Is(err, ErrNoActiveAssignment) {
			t.Fatalf("Delegate missing assignment error = %v, want %v", err, ErrNoActiveAssignment)
		}
		if _, err := ticketSvc.Claim(ticket.ID, activity.ID, 29); !errors.Is(err, ErrNoActiveAssignment) {
			t.Fatalf("Claim missing assignment error = %v, want %v", err, ErrNoActiveAssignment)
		}
	})

	t.Run("route operations reject missing and terminal tickets", func(t *testing.T) {
		db := newTestDB(t)
		ticketSvc := newSubmissionTicketService(t, db)
		service := testutil.SeedSmartSubmissionService(t, db)

		if _, err := ticketSvc.Assign(999999, 31, 1); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("Assign missing ticket error = %v, want %v", err, ErrTicketNotFound)
		}
		if _, err := ticketSvc.Transfer(999999, 1, 31, 1); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("Transfer missing ticket error = %v, want %v", err, ErrTicketNotFound)
		}
		if _, err := ticketSvc.Delegate(999999, 1, 31, 1); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("Delegate missing ticket error = %v, want %v", err, ErrTicketNotFound)
		}
		if _, err := ticketSvc.Claim(999999, 1, 1); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("Claim missing ticket error = %v, want %v", err, ErrTicketNotFound)
		}

		terminal := Ticket{
			Code:        "TICK-ROUTE-TERMINAL",
			Title:       "terminal routing",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusCompleted,
			Outcome:     TicketOutcomeFulfilled,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&terminal).Error; err != nil {
			t.Fatalf("create terminal ticket: %v", err)
		}

		if _, err := ticketSvc.Assign(terminal.ID, 31, 1); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("Assign terminal ticket error = %v, want %v", err, ErrTicketTerminal)
		}
		if _, err := ticketSvc.Transfer(terminal.ID, 1, 31, 1); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("Transfer terminal ticket error = %v, want %v", err, ErrTicketTerminal)
		}
		if _, err := ticketSvc.Delegate(terminal.ID, 1, 31, 1); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("Delegate terminal ticket error = %v, want %v", err, ErrTicketTerminal)
		}
		if _, err := ticketSvc.Claim(terminal.ID, 1, 1); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("Claim terminal ticket error = %v, want %v", err, ErrTicketTerminal)
		}
	})
}

func newSubmissionTicketService(t *testing.T, db *gorm.DB) *TicketService {
	return newSubmissionTicketServiceWithOrgResolver(t, db, nil)
}

func newSubmissionTicketServiceWithoutDecisionExecutor(t *testing.T, db *gorm.DB) *TicketService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	resolver := engine.NewParticipantResolver(nil)
	injector := do.New()
	submitter := &submissionTestSubmitter{db: db}
	do.ProvideValue(injector, wrapped)
	do.Provide(injector, NewTicketRepo)
	do.Provide(injector, NewTimelineRepo)
	do.Provide(injector, NewServiceDefRepo)
	do.Provide(injector, NewSLATemplateRepo)
	do.Provide(injector, NewPriorityRepo)
	do.ProvideValue(injector, engine.NewClassicEngine(resolver, nil, nil))
	do.ProvideValue(injector, engine.NewSmartEngine(nil, nil, nil, resolver, submitter, nil))
	do.Provide(injector, NewTicketService)
	return do.MustInvoke[*TicketService](injector)
}

func newSubmissionTicketServiceWithoutScheduler(t *testing.T, db *gorm.DB) *TicketService {
	t.Helper()
	wrapped := &database.DB{DB: db}
	resolver := engine.NewParticipantResolver(nil)
	injector := do.New()
	do.ProvideValue(injector, wrapped)
	do.Provide(injector, NewTicketRepo)
	do.Provide(injector, NewTimelineRepo)
	do.Provide(injector, NewServiceDefRepo)
	do.Provide(injector, NewSLATemplateRepo)
	do.Provide(injector, NewPriorityRepo)
	do.ProvideValue(injector, engine.NewClassicEngine(resolver, nil, nil))
	do.ProvideValue(injector, engine.NewSmartEngine(submissionTestDecisionExecutor{}, nil, nil, resolver, nil, nil))
	do.Provide(injector, NewTicketService)
	return do.MustInvoke[*TicketService](injector)
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

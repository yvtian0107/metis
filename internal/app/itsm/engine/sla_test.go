package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"metis/internal/app"
)

type fakeSLAAssuranceExecutor struct {
	trigger bool
}

type fakeEscalationNotifier struct {
	err   error
	calls []fakeEscalationNotifyCall
}

type fakeEscalationNotifyCall struct {
	channelID    uint
	subject      string
	body         string
	recipientIDs []uint
}

func (f *fakeEscalationNotifier) Send(ctx context.Context, channelID uint, subject, body string, recipientIDs []uint) error {
	f.calls = append(f.calls, fakeEscalationNotifyCall{
		channelID:    channelID,
		subject:      subject,
		body:         body,
		recipientIDs: append([]uint(nil), recipientIDs...),
	})
	return f.err
}

func (f fakeSLAAssuranceExecutor) Execute(ctx context.Context, agentID uint, req app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	if _, err := req.ToolHandler("sla.ticket_context", json.RawMessage(`{"ticket_id":1}`)); err != nil {
		return nil, err
	}
	if _, err := req.ToolHandler("sla.escalation_rules", json.RawMessage(`{"ticket_id":1}`)); err != nil {
		return nil, err
	}
	if f.trigger {
		if _, err := req.ToolHandler("sla.trigger_escalation", json.RawMessage(`{"ticket_id":1,"rule_id":7,"reasoning":"命中 0 分钟响应超时升级规则，触发通知动作。"}`)); err != nil {
			return nil, err
		}
	}
	return &app.AIDecisionResponse{Content: "done", Turns: 1}, nil
}

type slaPriorityTestModel struct {
	ID       uint `gorm:"primaryKey"`
	Name     string
	Code     string
	IsActive bool `gorm:"column:is_active"`
}

func (slaPriorityTestModel) TableName() string { return "itsm_priorities" }

func setupSLAAssuranceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ticketModel{}, &timelineModel{}, &slaPriorityTestModel{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Exec("ALTER TABLE itsm_tickets ADD COLUMN assignee_id INTEGER").Error; err != nil {
		t.Fatalf("add assignee column: %v", err)
	}
	if err := db.Exec(`CREATE TABLE users (id integer primary key, username text, is_active boolean, deleted_at datetime, manager_id integer)`).Error; err != nil {
		t.Fatalf("create users table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE itsm_service_definitions (id integer primary key, name text, sla_id integer)`).Error; err != nil {
		t.Fatalf("create service definitions table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE itsm_service_definition_versions (
		id integer primary key,
		service_id integer,
		version integer,
		content_hash text,
		engine_type text,
		sla_id integer,
		sla_template_json text,
		escalation_rules_json text
	)`).Error; err != nil {
		t.Fatalf("create service definition versions table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE itsm_escalation_rules (
		id integer primary key,
		sla_id integer,
		trigger_type text,
		level integer,
		wait_minutes integer,
		action_type text,
		target_config text,
		is_active boolean,
		deleted_at datetime
	)`).Error; err != nil {
		t.Fatalf("create escalation rules table: %v", err)
	}
	if err := db.Exec(`INSERT INTO users (id, username, is_active) VALUES (10, 'notify-a', true), (11, 'notify-b', true), (20, 'ops-a', true), (21, 'ops-b', true)`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := db.Exec(`INSERT INTO itsm_service_definitions (id, name, sla_id) VALUES (1, 'VPN', 3)`).Error; err != nil {
		t.Fatalf("seed service definition: %v", err)
	}
	return db
}

func seedSLARuntimeVersion(t *testing.T, db *gorm.DB, serviceID uint, slaID uint, rulesJSON string) uint {
	t.Helper()
	versionID := uint(slaID + 100)
	if err := db.Exec(`INSERT INTO itsm_service_definition_versions
		(id, service_id, version, content_hash, engine_type, sla_id, sla_template_json, escalation_rules_json)
		VALUES (?, ?, 1, ?, 'smart', ?, ?, ?)`,
		versionID, serviceID, "hash", slaID,
		`{"id":3,"name":"Snapshot SLA","code":"snapshot","responseMinutes":1,"resolutionMinutes":60,"isActive":true}`,
		rulesJSON,
	).Error; err != nil {
		t.Fatalf("seed sla runtime version: %v", err)
	}
	return versionID
}

func TestSLACheckTriggersDelayedRuleOnLaterScan(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	deadline := time.Now().Add(-5 * time.Minute)
	ticket := &ticketModel{
		ID:                  1,
		Code:                "T-1",
		Title:               "VPN 申请",
		ServiceID:           1,
		EngineType:          "classic",
		Status:              TicketStatusWaitingHuman,
		SLAResponseDeadline: &deadline,
		SLAStatus:           slaOnTrack,
	}
	versionID := seedSLARuntimeVersion(t, db, ticket.ServiceID, 3,
		`[{"id":7,"slaId":3,"triggerType":"response_timeout","level":1,"waitMinutes":10,"actionType":"notify","targetConfig":{"recipients":[{"type":"user","value":"10"}],"channelId":5},"isActive":true}]`)
	ticket.ServiceVersionID = &versionID
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Exec(`INSERT INTO itsm_escalation_rules
		(id, sla_id, trigger_type, level, wait_minutes, action_type, target_config, is_active)
		VALUES (7, 3, 'response_timeout', 1, 10, 'notify',
		'{"recipients":[{"type":"user","value":"10"}],"channelId":5}', true)`).Error; err != nil {
		t.Fatalf("create escalation rule: %v", err)
	}
	notifier := &fakeEscalationNotifier{}

	if err := checkTicketSLA(context.Background(), db, ticket, deadline.Add(5*time.Minute), nil, nil, NewParticipantResolver(nil), notifier); err != nil {
		t.Fatalf("early sla check: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("delayed escalation fired too early: %+v", notifier.calls)
	}
	var earlyCount int64
	if err := db.Table("itsm_ticket_timelines").Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_escalation").Count(&earlyCount).Error; err != nil {
		t.Fatalf("count early timelines: %v", err)
	}
	if earlyCount != 0 {
		t.Fatalf("expected no early escalation timeline, got %d", earlyCount)
	}

	if err := checkTicketSLA(context.Background(), db, ticket, deadline.Add(11*time.Minute), nil, nil, NewParticipantResolver(nil), notifier); err != nil {
		t.Fatalf("late sla check: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected delayed escalation on later scan, got calls=%+v", notifier.calls)
	}
}

func TestSLACheckSkipsSoftDeletedEscalationRules(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	deadline := time.Now().Add(-15 * time.Minute)
	ticket := &ticketModel{
		ID:                  1,
		Code:                "T-1",
		Title:               "VPN 申请",
		ServiceID:           1,
		EngineType:          "classic",
		Status:              TicketStatusWaitingHuman,
		SLAResponseDeadline: &deadline,
		SLAStatus:           slaOnTrack,
	}
	versionID := seedSLARuntimeVersion(t, db, ticket.ServiceID, 3, `[]`)
	ticket.ServiceVersionID = &versionID
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Exec(`INSERT INTO itsm_escalation_rules
		(id, sla_id, trigger_type, level, wait_minutes, action_type, target_config, is_active, deleted_at)
		VALUES (7, 3, 'response_timeout', 1, 0, 'notify',
		'{"recipients":[{"type":"user","value":"10"}],"channelId":5}', true, CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("create soft deleted escalation rule: %v", err)
	}
	notifier := &fakeEscalationNotifier{}

	if err := checkTicketSLA(context.Background(), db, ticket, deadline.Add(15*time.Minute), nil, nil, NewParticipantResolver(nil), notifier); err != nil {
		t.Fatalf("sla check: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("soft-deleted escalation rule fired: %+v", notifier.calls)
	}
	var count int64
	if err := db.Table("itsm_ticket_timelines").Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_escalation").Count(&count).Error; err != nil {
		t.Fatalf("count timelines: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no escalation timeline for soft-deleted rule, got %d", count)
	}
}

func TestSLACheckUsesRuntimeVersionEscalationSnapshot(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	deadline := time.Now().Add(-5 * time.Minute)
	versionID := seedSLARuntimeVersion(t, db, 1, 3,
		`[{"id":7,"slaId":3,"triggerType":"response_timeout","level":1,"waitMinutes":0,"actionType":"notify","targetConfig":{"recipients":[{"type":"user","value":"10"}],"channelId":5},"isActive":true}]`)
	if err := db.Exec(`UPDATE itsm_service_definitions SET sla_id = 4 WHERE id = 1`).Error; err != nil {
		t.Fatalf("mutate live service sla: %v", err)
	}
	if err := db.Exec(`INSERT INTO itsm_escalation_rules
		(id, sla_id, trigger_type, level, wait_minutes, action_type, target_config, is_active)
		VALUES (8, 4, 'response_timeout', 1, 0, 'notify',
		'{"recipients":[{"type":"user","value":"11"}],"channelId":6}', true)`).Error; err != nil {
		t.Fatalf("create live escalation rule: %v", err)
	}
	ticket := &ticketModel{
		ID:                  1,
		Code:                "T-1",
		Title:               "VPN 申请",
		ServiceID:           1,
		ServiceVersionID:    &versionID,
		EngineType:          "classic",
		Status:              TicketStatusWaitingHuman,
		SLAResponseDeadline: &deadline,
		SLAStatus:           slaOnTrack,
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	notifier := &fakeEscalationNotifier{}

	if err := checkTicketSLA(context.Background(), db, ticket, deadline.Add(5*time.Minute), nil, nil, NewParticipantResolver(nil), notifier); err != nil {
		t.Fatalf("sla check: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected one snapshot notification, got %+v", notifier.calls)
	}
	call := notifier.calls[0]
	if call.channelID != 5 || len(call.recipientIDs) != 1 || call.recipientIDs[0] != 10 {
		t.Fatalf("expected snapshot rule channel=5 recipient=10, got %+v", call)
	}
}

func TestSLACheckWritesBreachTimelineBeforeEscalation(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	responseDeadline := time.Now().Add(-5 * time.Minute)
	resolutionDeadline := time.Now().Add(30 * time.Minute)
	versionID := seedSLARuntimeVersion(t, db, 1, 3, `[]`)
	ticket := &ticketModel{
		ID:                    1,
		Code:                  "T-1",
		Title:                 "VPN 申请",
		ServiceID:             1,
		ServiceVersionID:      &versionID,
		EngineType:            "classic",
		Status:                TicketStatusWaitingHuman,
		SLAResponseDeadline:   &responseDeadline,
		SLAResolutionDeadline: &resolutionDeadline,
		SLAStatus:             slaOnTrack,
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	if err := checkTicketSLA(context.Background(), db, ticket, time.Now(), nil, nil, NewParticipantResolver(nil), nil); err != nil {
		t.Fatalf("sla check: %v", err)
	}
	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.SLAStatus != slaBreachedResponse {
		t.Fatalf("sla_status = %q, want %q", reloaded.SLAStatus, slaBreachedResponse)
	}
	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_response_breached").First(&timeline).Error; err != nil {
		t.Fatalf("expected response breach timeline: %v", err)
	}
	if timeline.Message == "" || timeline.Details == "" {
		t.Fatalf("expected breach timeline message and details, got %+v", timeline)
	}
}

func TestSLACheckWritesResolutionBreachTimeline(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	responseDeadline := time.Now().Add(-30 * time.Minute)
	resolutionDeadline := time.Now().Add(-5 * time.Minute)
	versionID := seedSLARuntimeVersion(t, db, 1, 3, `[]`)
	ticket := &ticketModel{
		ID:                    1,
		Code:                  "T-1",
		Title:                 "VPN 申请",
		ServiceID:             1,
		ServiceVersionID:      &versionID,
		EngineType:            "classic",
		Status:                TicketStatusWaitingHuman,
		SLAResponseDeadline:   &responseDeadline,
		SLAResolutionDeadline: &resolutionDeadline,
		SLAStatus:             slaOnTrack,
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	if err := checkTicketSLA(context.Background(), db, ticket, time.Now(), nil, nil, NewParticipantResolver(nil), nil); err != nil {
		t.Fatalf("sla check: %v", err)
	}
	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.SLAStatus != slaBreachedResolve {
		t.Fatalf("sla_status = %q, want %q", reloaded.SLAStatus, slaBreachedResolve)
	}
	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_resolution_breached").First(&timeline).Error; err != nil {
		t.Fatalf("expected resolution breach timeline: %v", err)
	}
	if timeline.Message == "" || timeline.Details == "" {
		t.Fatalf("expected resolution breach timeline message and details, got %+v", timeline)
	}
}

func TestHandleSLACheckAggregatesTicketErrorsAndContinues(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	deadline := time.Now().Add(-5 * time.Minute)
	badVersionID := seedSLARuntimeVersion(t, db, 1, 31, `{malformed`)
	goodVersionID := seedSLARuntimeVersion(t, db, 1, 32,
		`[{"id":8,"slaId":32,"triggerType":"response_timeout","level":1,"waitMinutes":0,"actionType":"notify","targetConfig":{"recipients":[{"type":"user","value":"10"}],"channelId":6},"isActive":true}]`)
	tickets := []ticketModel{
		{
			ID:                  1,
			Code:                "T-BAD",
			Title:               "坏快照工单",
			ServiceID:           1,
			ServiceVersionID:    &badVersionID,
			EngineType:          "classic",
			Status:              TicketStatusWaitingHuman,
			SLAResponseDeadline: &deadline,
			SLAStatus:           slaOnTrack,
		},
		{
			ID:                  2,
			Code:                "T-GOOD",
			Title:               "有效快照工单",
			ServiceID:           1,
			ServiceVersionID:    &goodVersionID,
			EngineType:          "classic",
			Status:              TicketStatusWaitingHuman,
			SLAResponseDeadline: &deadline,
			SLAStatus:           slaOnTrack,
		},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}
	notifier := &fakeEscalationNotifier{}

	err := HandleSLACheck(db, nil, nil, NewParticipantResolver(nil), notifier)(context.Background(), nil)
	if err == nil {
		t.Fatal("expected aggregated SLA check error")
	}
	if !strings.Contains(err.Error(), "ticket 1(T-BAD)") {
		t.Fatalf("expected aggregated error to identify bad ticket, got %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected valid ticket escalation to continue, got calls=%+v", notifier.calls)
	}
	if notifier.calls[0].channelID != 6 || len(notifier.calls[0].recipientIDs) != 1 || notifier.calls[0].recipientIDs[0] != 10 {
		t.Fatalf("unexpected valid ticket notification: %+v", notifier.calls[0])
	}
	var goodTimelineCount int64
	if err := db.Table("itsm_ticket_timelines").
		Where("ticket_id = ? AND event_type = ?", 2, "sla_escalation").
		Count(&goodTimelineCount).Error; err != nil {
		t.Fatalf("count good ticket escalation timeline: %v", err)
	}
	if goodTimelineCount != 1 {
		t.Fatalf("expected good ticket escalation timeline, got %d", goodTimelineCount)
	}
}

func TestSLACheckReturnsErrorAndSkipsEscalationWhenBreachAuditFails(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	deadline := time.Now().Add(-5 * time.Minute)
	versionID := seedSLARuntimeVersion(t, db, 1, 3,
		`[{"id":7,"slaId":3,"triggerType":"response_timeout","level":1,"waitMinutes":0,"actionType":"notify","targetConfig":{"recipients":[{"type":"user","value":"10"}],"channelId":5},"isActive":true}]`)
	ticket := &ticketModel{
		ID:                  1,
		Code:                "T-1",
		Title:               "VPN 申请",
		ServiceID:           1,
		ServiceVersionID:    &versionID,
		EngineType:          "classic",
		Status:              TicketStatusWaitingHuman,
		SLAResponseDeadline: &deadline,
		SLAStatus:           slaOnTrack,
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Exec("DROP TABLE itsm_ticket_timelines").Error; err != nil {
		t.Fatalf("drop timeline table: %v", err)
	}
	notifier := &fakeEscalationNotifier{}

	err := checkTicketSLA(context.Background(), db, ticket, time.Now(), nil, nil, NewParticipantResolver(nil), notifier)
	if err == nil {
		t.Fatal("expected sla check to fail when breach timeline cannot be written")
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("escalation should not run after breach audit failure, got %+v", notifier.calls)
	}
	var reloaded ticketModel
	if err := db.First(&reloaded, ticket.ID).Error; err != nil {
		t.Fatalf("reload ticket: %v", err)
	}
	if reloaded.SLAStatus != slaOnTrack {
		t.Fatalf("expected sla_status rollback to %q, got %q", slaOnTrack, reloaded.SLAStatus)
	}
}

func TestSLAAssuranceAgentTriggersEscalationTool(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	ticket := &ticketModel{ID: 1, Code: "T-1", Title: "VPN 申请", Status: "in_progress", SLAStatus: slaBreachedResponse}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	rule := &escalationRuleModel{
		ID:          7,
		SLAID:       3,
		TriggerType: "response_timeout",
		Level:       1,
		ActionType:  "notify",
		IsActive:    true,
	}
	err := runSLAAssuranceAgent(context.Background(), db, ticket, rule, "response_timeout", 9, "SLA 保障智能体", fakeSLAAssuranceExecutor{trigger: true}, nil, nil)
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_escalation").First(&timeline).Error; err != nil {
		t.Fatalf("timeline not written: %v", err)
	}
	if timeline.Reasoning == "" {
		t.Fatal("expected agent reasoning in timeline")
	}
}

func TestSLAAssuranceAgentMustTriggerEscalation(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	ticket := &ticketModel{ID: 1, Code: "T-1", Title: "VPN 申请", Status: "in_progress", SLAStatus: slaBreachedResponse}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	rule := &escalationRuleModel{ID: 7, SLAID: 3, TriggerType: "response_timeout", Level: 1, ActionType: "notify", IsActive: true}
	if err := runSLAAssuranceAgent(context.Background(), db, ticket, rule, "response_timeout", 9, "SLA 保障智能体", fakeSLAAssuranceExecutor{}, nil, nil); err == nil {
		t.Fatal("expected error when agent does not trigger escalation")
	}
}

func TestEscalationNotifySendsResolvedUsers(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	ticket := &ticketModel{ID: 1, Code: "T-1", Title: "VPN 申请", Status: "in_progress", PriorityID: 2, SLAStatus: slaBreachedResponse}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	rule := &escalationRuleModel{
		ID:           7,
		SLAID:        3,
		TriggerType:  "response_timeout",
		Level:        1,
		ActionType:   "notify",
		TargetConfig: `{"recipients":[{"type":"user","value":"10"},{"type":"user","value":"10"},{"type":"user","value":"11"}],"channelId":5,"subjectTemplate":"告警 {{ticket.code}}","bodyTemplate":"请处理 {{ticket.title}}"}`,
		IsActive:     true,
	}
	notifier := &fakeEscalationNotifier{}

	err := executeEscalationAction(context.Background(), db, ticket, rule, "response_timeout", 0, "系统计时器", "命中规则", NewParticipantResolver(nil), notifier)
	if err != nil {
		t.Fatalf("execute escalation: %v", err)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("notify calls = %d, want 1", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.channelID != 5 || call.subject != "告警 T-1" || call.body != "请处理 VPN 申请" {
		t.Fatalf("unexpected notification call: %+v", call)
	}
	if got, want := call.recipientIDs, []uint{10, 11}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("recipient IDs = %v, want %v", got, want)
	}

	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_escalation").First(&timeline).Error; err != nil {
		t.Fatalf("timeline not written: %v", err)
	}
	if timeline.Message != "SLA 升级通知已发送" {
		t.Fatalf("timeline message = %q", timeline.Message)
	}
}

func TestEscalationReassignTakesFirstResolvedUser(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	if err := db.AutoMigrate(&activityModel{}, &assignmentModel{}); err != nil {
		t.Fatalf("migrate activity assignment models: %v", err)
	}
	activityID := uint(88)
	currentUser := uint(19)
	ticket := &ticketModel{ID: 1, Code: "T-1", Title: "VPN 申请", Status: "in_progress", CurrentActivityID: &activityID, SLAStatus: slaBreachedResponse}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	activity := activityModel{ID: activityID, TicketID: ticket.ID, ActivityType: NodeProcess, Status: ActivityPending}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	assignment := assignmentModel{TicketID: ticket.ID, ActivityID: activity.ID, ParticipantType: "user", UserID: &currentUser, AssigneeID: &currentUser, Status: ActivityPending, IsCurrent: true}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	rule := &escalationRuleModel{
		ID:           8,
		SLAID:        3,
		TriggerType:  "resolution_timeout",
		Level:        2,
		ActionType:   "reassign",
		TargetConfig: `{"assigneeCandidates":[{"type":"user","value":"20"},{"type":"user","value":"21"}]}`,
		IsActive:     true,
	}

	err := executeEscalationAction(context.Background(), db, ticket, rule, "resolution_timeout", 0, "系统计时器", "命中规则", NewParticipantResolver(nil), nil)
	if err != nil {
		t.Fatalf("execute escalation: %v", err)
	}
	var selected uint
	if err := db.Table("itsm_tickets").Select("assignee_id").Where("id = ?", ticket.ID).Scan(&selected).Error; err != nil {
		t.Fatalf("query assignee: %v", err)
	}
	if selected != 20 {
		t.Fatalf("assignee_id = %d, want 20", selected)
	}

	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_escalation").First(&timeline).Error; err != nil {
		t.Fatalf("timeline not written: %v", err)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(timeline.Details), &details); err != nil {
		t.Fatalf("decode details: %v", err)
	}
	if got := uint(details["selected_user_id"].(float64)); got != 20 {
		t.Fatalf("selected_user_id = %d, want 20", got)
	}
}

func TestEscalationReassignUpdatesCurrentAssignment(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	if err := db.AutoMigrate(&activityModel{}, &assignmentModel{}); err != nil {
		t.Fatalf("migrate activity assignment models: %v", err)
	}
	activityID := uint(99)
	currentUser := uint(20)
	ticket := &ticketModel{ID: 1, Code: "T-1", Title: "VPN 申请", Status: "waiting_human", CurrentActivityID: &activityID, SLAStatus: slaBreachedResponse}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	activity := activityModel{ID: activityID, TicketID: ticket.ID, ActivityType: NodeProcess, Status: ActivityPending}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	assignment := assignmentModel{TicketID: ticket.ID, ActivityID: activity.ID, ParticipantType: "user", UserID: &currentUser, AssigneeID: &currentUser, Status: ActivityPending, IsCurrent: true}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	rule := &escalationRuleModel{
		ID:           8,
		SLAID:        3,
		TriggerType:  "resolution_timeout",
		Level:        2,
		ActionType:   "reassign",
		TargetConfig: `{"assigneeCandidates":[{"type":"user","value":"21"}]}`,
		IsActive:     true,
	}

	err := executeEscalationAction(context.Background(), db, ticket, rule, "resolution_timeout", 0, "系统计时器", "命中规则", NewParticipantResolver(nil), nil)
	if err != nil {
		t.Fatalf("execute escalation: %v", err)
	}
	var updated assignmentModel
	if err := db.First(&updated, assignment.ID).Error; err != nil {
		t.Fatalf("load assignment: %v", err)
	}
	if updated.UserID == nil || *updated.UserID != 21 || updated.AssigneeID == nil || *updated.AssigneeID != 21 {
		t.Fatalf("expected current assignment to move to user 21, got %+v", updated)
	}
}

func TestEscalationPriorityMissingTargetLeavesTicketUnchanged(t *testing.T) {
	db := setupSLAAssuranceTestDB(t)
	ticket := &ticketModel{ID: 1, Code: "T-1", Title: "VPN 申请", Status: "in_progress", PriorityID: 2, SLAStatus: slaBreachedResponse}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	rule := &escalationRuleModel{
		ID:           9,
		SLAID:        3,
		TriggerType:  "response_timeout",
		Level:        1,
		ActionType:   "escalate_priority",
		TargetConfig: `{"priorityId":99}`,
		IsActive:     true,
	}

	err := executeEscalationAction(context.Background(), db, ticket, rule, "response_timeout", 0, "系统计时器", "命中规则", NewParticipantResolver(nil), nil)
	if err != nil {
		t.Fatalf("execute escalation: %v", err)
	}
	var priorityID uint
	if err := db.Table("itsm_tickets").Select("priority_id").Where("id = ?", ticket.ID).Scan(&priorityID).Error; err != nil {
		t.Fatalf("query priority: %v", err)
	}
	if priorityID != 2 {
		t.Fatalf("priority_id = %d, want unchanged 2", priorityID)
	}
	var timeline timelineModel
	if err := db.Where("ticket_id = ? AND event_type = ?", ticket.ID, "sla_escalation").First(&timeline).Error; err != nil {
		t.Fatalf("timeline not written: %v", err)
	}
	if timeline.Message != "SLA 升级：目标优先级不存在或已停用" {
		t.Fatalf("timeline message = %q", timeline.Message)
	}
}

func TestCheckTicketSLA_ResponseBreach(t *testing.T) {
	// This is a logic-level test verifying breach detection.
	// In production, checkTicketSLA writes to DB — here we test the condition logic directly.
	now := time.Now()
	pastDeadline := now.Add(-10 * time.Minute)
	futureDeadline := now.Add(10 * time.Minute)

	tests := []struct {
		name             string
		responseDeadline *time.Time
		resolveDeadline  *time.Time
		currentSLA       string
		wantBreach       bool
		breachType       string
	}{
		{
			name:             "response breached",
			responseDeadline: &pastDeadline,
			currentSLA:       slaOnTrack,
			wantBreach:       true,
			breachType:       "response",
		},
		{
			name:            "resolution breached",
			resolveDeadline: &pastDeadline,
			currentSLA:      slaOnTrack,
			wantBreach:      true,
			breachType:      "resolution",
		},
		{
			name:             "no breach - future deadline",
			responseDeadline: &futureDeadline,
			resolveDeadline:  &futureDeadline,
			currentSLA:       slaOnTrack,
			wantBreach:       false,
		},
		{
			name:             "already breached - no re-trigger",
			responseDeadline: &pastDeadline,
			currentSLA:       slaBreachedResponse,
			wantBreach:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &ticketModel{
				ID:                    1,
				SLAResponseDeadline:   tt.responseDeadline,
				SLAResolutionDeadline: tt.resolveDeadline,
				SLAStatus:             tt.currentSLA,
			}

			// Verify breach detection logic
			responseBreach := ticket.SLAResponseDeadline != nil &&
				now.After(*ticket.SLAResponseDeadline) &&
				ticket.SLAStatus != slaBreachedResponse &&
				ticket.SLAStatus != slaBreachedResolve

			resolveBreach := !responseBreach &&
				ticket.SLAResolutionDeadline != nil &&
				now.After(*ticket.SLAResolutionDeadline) &&
				ticket.SLAStatus != slaBreachedResolve

			gotBreach := responseBreach || resolveBreach
			if gotBreach != tt.wantBreach {
				t.Errorf("breach detection: got %v, want %v", gotBreach, tt.wantBreach)
			}
			if tt.wantBreach && tt.breachType == "response" && !responseBreach {
				t.Error("expected response breach")
			}
			if tt.wantBreach && tt.breachType == "resolution" && !resolveBreach {
				t.Error("expected resolution breach")
			}
		})
	}
}

func TestEscalationTriggerTiming(t *testing.T) {
	now := time.Now()
	deadline := now.Add(-30 * time.Minute) // breached 30 minutes ago

	tests := []struct {
		name        string
		waitMinutes int
		shouldFire  bool
	}{
		{"fires immediately (0 min wait)", 0, true},
		{"fires after 15 min wait", 15, true},
		{"fires after 30 min wait", 30, true},
		{"does not fire after 45 min wait", 45, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triggerTime := deadline.Add(time.Duration(tt.waitMinutes) * time.Minute)
			fired := !now.Before(triggerTime)
			if fired != tt.shouldFire {
				t.Errorf("got fired=%v, want %v", fired, tt.shouldFire)
			}
		})
	}
}

func TestSLAPauseResumeDeadlineExtension(t *testing.T) {
	// Simulate pause/resume cycle
	originalDeadline := time.Now().Add(2 * time.Hour)
	pausedAt := time.Now().Add(-30 * time.Minute) // paused 30 minutes ago
	pausedDuration := time.Since(pausedAt)

	extendedDeadline := originalDeadline.Add(pausedDuration)

	// The extended deadline should be approximately 30 minutes later than original
	diff := extendedDeadline.Sub(originalDeadline)
	if diff < 29*time.Minute || diff > 31*time.Minute {
		t.Errorf("deadline extension should be ~30 minutes, got %v", diff)
	}
}

func TestSLAConstants(t *testing.T) {
	// Verify SLA status constants match expected values
	if slaOnTrack != "on_track" {
		t.Error("slaOnTrack mismatch")
	}
	if slaBreachedResponse != "breached_response" {
		t.Error("slaBreachedResponse mismatch")
	}
	if slaBreachedResolve != "breached_resolution" {
		t.Error("slaBreachedResolve mismatch")
	}
}

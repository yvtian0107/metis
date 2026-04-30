package ticket

import (
	. "metis/internal/app/itsm/domain"
	"slices"
	"strings"
	"testing"
	"time"

	"metis/internal/app/itsm/engine"
	"metis/internal/database"
	"metis/internal/model"

	"gorm.io/gorm"
)

func newTicketMonitorServiceForTest(db *gorm.DB) *TicketService {
	wrapped := &database.DB{DB: db}
	return &TicketService{ticketRepo: &TicketRepo{db: wrapped}}
}

func seedTicketMonitorBase(t *testing.T, db *gorm.DB) (ServiceDefinition, Priority, model.User) {
	t.Helper()
	user := model.User{Username: "monitor-admin", IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	catalog := ServiceCatalog{Name: "IT", Code: "it"}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	service := ServiceDefinition{Name: "VPN", Code: "vpn", CatalogID: catalog.ID, EngineType: "smart", IsActive: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	priority := Priority{Name: "P1", Code: "p1", Value: 1, Color: "#d34", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	return service, priority, user
}

func createMonitorTicket(t *testing.T, db *gorm.DB, service ServiceDefinition, priority Priority, requester model.User, patch func(*Ticket)) Ticket {
	t.Helper()
	ticket := Ticket{
		Code:        "TICK-" + time.Now().Format("150405.000000"),
		Title:       "监控工单",
		ServiceID:   service.ID,
		EngineType:  service.EngineType,
		Status:      TicketStatusWaitingHuman,
		PriorityID:  priority.ID,
		RequesterID: requester.ID,
		Source:      TicketSourceAgent,
		SLAStatus:   SLAStatusOnTrack,
	}
	if patch != nil {
		patch(&ticket)
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	return ticket
}

func createCurrentActivity(t *testing.T, db *gorm.DB, ticket Ticket, activityType string, startedAt time.Time) TicketActivity {
	t.Helper()
	activity := TicketActivity{
		TicketID:     ticket.ID,
		Name:         "当前处理",
		ActivityType: activityType,
		Status:       engine.ActivityPending,
		StartedAt:    &startedAt,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if err := db.Model(&Ticket{}).Where("id = ?", ticket.ID).Update("current_activity_id", activity.ID).Error; err != nil {
		t.Fatalf("set current activity: %v", err)
	}
	return activity
}

func createPendingAssignment(t *testing.T, db *gorm.DB, ticket Ticket, activity TicketActivity, userID uint) {
	t.Helper()
	assignment := TicketAssignment{
		TicketID:        ticket.ID,
		ActivityID:      activity.ID,
		ParticipantType: "user",
		UserID:          &userID,
		Status:          AssignmentPending,
		IsCurrent:       true,
	}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create pending assignment: %v", err)
	}
}

func monitorReasonsContain(item TicketMonitorItem, needle string) bool {
	return slices.ContainsFunc(item.StuckReasons, func(reason string) bool {
		return strings.Contains(reason, needle)
	})
}

func monitorReasonByRule(item TicketMonitorItem, ruleCode string) (TicketMonitorReason, bool) {
	for _, reason := range item.MonitorReasons {
		if reason.RuleCode == ruleCode {
			return reason, true
		}
	}
	return TicketMonitorReason{}, false
}

func TestTicketMonitorDetectsAIAndHumanBlockedTickets(t *testing.T) {
	db := newTestDB(t)
	service, priority, requester := seedTicketMonitorBase(t, db)
	svc := newTicketMonitorServiceForTest(db)
	now := time.Now()

	aiTicket := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-AI"
		ticket.AIFailureCount = engine.MaxAIFailureCount
	})
	humanTicket := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-HUMAN"
		ticket.EngineType = "classic"
		ticket.Source = TicketSourceCatalog
	})
	createCurrentActivity(t, db, humanTicket, engine.NodeProcess, now.Add(-10*time.Minute))

	resp, err := svc.Monitor(TicketMonitorParams{Page: 1, PageSize: 20}, requester.ID)
	if err != nil {
		t.Fatalf("monitor: %v", err)
	}
	if resp.Summary.ActiveTotal != 2 || resp.Summary.StuckTotal != 2 || resp.Summary.AIIncidentTotal != 1 {
		t.Fatalf("unexpected summary: %+v", resp.Summary)
	}

	var aiItem, humanItem *TicketMonitorItem
	for i := range resp.Items {
		switch resp.Items[i].ID {
		case aiTicket.ID:
			aiItem = &resp.Items[i]
		case humanTicket.ID:
			humanItem = &resp.Items[i]
		}
	}
	if aiItem == nil || aiItem.RiskLevel != "blocked" || !monitorReasonsContain(*aiItem, "AI 连续失败") {
		t.Fatalf("expected AI blocked item, got %+v", aiItem)
	}
	if reason, ok := monitorReasonByRule(*aiItem, "ai_circuit_breaker"); !ok ||
		reason.MetricCode != "ai_incident_total" ||
		reason.Severity != "blocked" ||
		reason.Evidence["ai_failure_count"] != float64(engine.MaxAIFailureCount) {
		t.Fatalf("expected auditable AI circuit-breaker reason, got ok=%v reason=%+v", ok, reason)
	}
	if humanItem == nil || humanItem.RiskLevel != "blocked" || !monitorReasonsContain(*humanItem, "没有处理人") {
		t.Fatalf("expected human blocked item, got %+v", humanItem)
	}
	if reason, ok := monitorReasonByRule(*humanItem, "human_assignment_missing"); !ok ||
		reason.MetricCode != "blocked_total" ||
		reason.Severity != "blocked" ||
		reason.Evidence["assignment_count"] != float64(0) {
		t.Fatalf("expected auditable human assignment reason, got ok=%v reason=%+v", ok, reason)
	}
}

func TestTicketMonitorDetectsSLAAndActionRisk(t *testing.T) {
	db := newTestDB(t)
	service, priority, requester := seedTicketMonitorBase(t, db)
	svc := newTicketMonitorServiceForTest(db)
	now := time.Now()

	breached := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-SLA-BLOCKED"
		deadline := now.Add(-10 * time.Minute)
		ticket.SLAResolutionDeadline = &deadline
		ticket.SLAStatus = SLAStatusOnTrack
	})
	responseDueSoon := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-SLA-RESPONSE-RISK"
		deadline := now.Add(20 * time.Minute)
		ticket.SLAResponseDeadline = &deadline
	})
	resolutionDueSoon := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-SLA-RESOLUTION-RISK"
		deadline := now.Add(20 * time.Minute)
		ticket.SLAResolutionDeadline = &deadline
	})
	actionFailed := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-ACTION-FAILED"
	})
	failedActivity := createCurrentActivity(t, db, actionFailed, engine.NodeAction, now.Add(-2*time.Minute))
	if err := db.Create(&TicketActionExecution{TicketID: actionFailed.ID, ActivityID: failedActivity.ID, Status: "failed"}).Error; err != nil {
		t.Fatalf("create failed action: %v", err)
	}
	actionRunning := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-ACTION-RISK"
	})
	createCurrentActivity(t, db, actionRunning, engine.NodeAction, now.Add(-20*time.Minute))
	humanWaiting := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-HUMAN-RISK"
		ticket.EngineType = "classic"
		ticket.Source = TicketSourceCatalog
	})
	humanActivity := createCurrentActivity(t, db, humanWaiting, engine.NodeProcess, now.Add(-70*time.Minute))
	createPendingAssignment(t, db, humanWaiting, humanActivity, requester.ID)

	resp, err := svc.Monitor(TicketMonitorParams{Page: 1, PageSize: 20}, requester.ID)
	if err != nil {
		t.Fatalf("monitor: %v", err)
	}
	if resp.Summary.SLARiskTotal != 3 || resp.Summary.StuckTotal != 2 || resp.Summary.RiskTotal != 4 {
		t.Fatalf("expected separated SLA/stuck/risk totals, got %+v", resp.Summary)
	}
	byID := map[uint]TicketMonitorItem{}
	for _, item := range resp.Items {
		byID[item.ID] = item
	}
	if byID[breached.ID].RiskLevel != "blocked" || !monitorReasonsContain(byID[breached.ID], "解决 SLA 已超时") {
		t.Fatalf("expected SLA blocked, got %+v", byID[breached.ID])
	}
	if reason, ok := monitorReasonByRule(byID[breached.ID], "sla_resolution_breached"); !ok ||
		reason.MetricCode != "sla_risk_total" ||
		reason.Severity != "blocked" ||
		reason.Evidence["deadline_field"] != "sla_resolution_deadline" {
		t.Fatalf("expected auditable SLA resolution breach, got ok=%v reason=%+v", ok, reason)
	}
	if byID[responseDueSoon.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[responseDueSoon.ID], "响应 SLA 距离截止") {
		t.Fatalf("expected response SLA due risk, got %+v", byID[responseDueSoon.ID])
	}
	if reason, ok := monitorReasonByRule(byID[responseDueSoon.ID], "sla_response_due_soon"); !ok ||
		reason.MetricCode != "sla_risk_total" ||
		reason.Severity != "risk" ||
		reason.Evidence["threshold_minutes"] != float64(30) {
		t.Fatalf("expected auditable SLA response due-soon reason, got ok=%v reason=%+v", ok, reason)
	}
	if byID[resolutionDueSoon.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[resolutionDueSoon.ID], "解决 SLA 距离截止") {
		t.Fatalf("expected resolution SLA due risk, got %+v", byID[resolutionDueSoon.ID])
	}
	if byID[actionFailed.ID].RiskLevel != "blocked" || !monitorReasonsContain(byID[actionFailed.ID], "动作执行失败") {
		t.Fatalf("expected action failure blocked, got %+v", byID[actionFailed.ID])
	}
	if reason, ok := monitorReasonByRule(byID[actionFailed.ID], "action_execution_failed"); !ok ||
		reason.MetricCode != "blocked_total" ||
		reason.Severity != "blocked" ||
		reason.Evidence["action_failed"] != true {
		t.Fatalf("expected auditable action failure reason, got ok=%v reason=%+v", ok, reason)
	}
	if byID[actionRunning.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[actionRunning.ID], "动作运行超过") {
		t.Fatalf("expected action running risk, got %+v", byID[actionRunning.ID])
	}
	if reason, ok := monitorReasonByRule(byID[actionRunning.ID], "action_running_too_long"); !ok ||
		reason.MetricCode != "risk_total" ||
		reason.Severity != "risk" ||
		reason.Evidence["threshold_minutes"] != float64(15) {
		t.Fatalf("expected auditable action runtime reason, got ok=%v reason=%+v", ok, reason)
	}
	if byID[humanWaiting.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[humanWaiting.ID], "人工节点等待超过") {
		t.Fatalf("expected human wait risk, got %+v", byID[humanWaiting.ID])
	}
	if reason, ok := monitorReasonByRule(byID[humanWaiting.ID], "human_waiting_too_long"); !ok ||
		reason.MetricCode != "risk_total" ||
		reason.Severity != "risk" ||
		reason.Evidence["threshold_minutes"] != float64(60) {
		t.Fatalf("expected auditable human wait reason, got ok=%v reason=%+v", ok, reason)
	}

	slaFiltered, err := svc.Monitor(TicketMonitorParams{MetricCode: "sla_risk_total", Page: 1, PageSize: 20}, requester.ID)
	if err != nil {
		t.Fatalf("monitor SLA metric filter: %v", err)
	}
	if slaFiltered.Total != int64(resp.Summary.SLARiskTotal) || len(slaFiltered.Items) != resp.Summary.SLARiskTotal {
		t.Fatalf("SLA metric filter must be reproducible from summary, total=%d len=%d summary=%+v", slaFiltered.Total, len(slaFiltered.Items), resp.Summary)
	}
}

func TestTicketMonitorSummaryAndRiskPagination(t *testing.T) {
	db := newTestDB(t)
	service, priority, requester := seedTicketMonitorBase(t, db)
	svc := newTicketMonitorServiceForTest(db)
	now := time.Now()

	_ = createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-ACTIVE"
	})
	blocked := createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-BLOCKED"
		ticket.AIFailureCount = engine.MaxAIFailureCount
	})
	finishedAt := now
	_ = createMonitorTicket(t, db, service, priority, requester, func(ticket *Ticket) {
		ticket.Code = "TICK-DONE"
		ticket.Status = TicketStatusCompleted
		ticket.FinishedAt = &finishedAt
	})

	resp, err := svc.Monitor(TicketMonitorParams{RiskLevel: "blocked", Page: 1, PageSize: 1}, requester.ID)
	if err != nil {
		t.Fatalf("monitor: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 || resp.Items[0].ID != blocked.ID {
		t.Fatalf("expected only blocked item, total=%d items=%+v", resp.Total, resp.Items)
	}
	if resp.Summary.ActiveTotal != 2 || resp.Summary.CompletedTodayTotal != 1 || resp.Summary.StuckTotal != 1 {
		t.Fatalf("summary should ignore risk filter and exclude terminal from active, got %+v", resp.Summary)
	}

	completedResp, err := svc.Monitor(TicketMonitorParams{MetricCode: "completed_today_total", Page: 1, PageSize: 20}, requester.ID)
	if err != nil {
		t.Fatalf("monitor completed metric filter: %v", err)
	}
	if completedResp.Total != int64(resp.Summary.CompletedTodayTotal) || completedResp.Items[0].ID == blocked.ID {
		t.Fatalf("completed metric filter must include only today's completed tickets, total=%d items=%+v summary=%+v", completedResp.Total, completedResp.Items, resp.Summary)
	}
}

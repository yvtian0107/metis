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
	if humanItem == nil || humanItem.RiskLevel != "blocked" || !monitorReasonsContain(*humanItem, "没有处理人") {
		t.Fatalf("expected human blocked item, got %+v", humanItem)
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
	if byID[responseDueSoon.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[responseDueSoon.ID], "响应 SLA 距离截止") {
		t.Fatalf("expected response SLA due risk, got %+v", byID[responseDueSoon.ID])
	}
	if byID[resolutionDueSoon.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[resolutionDueSoon.ID], "解决 SLA 距离截止") {
		t.Fatalf("expected resolution SLA due risk, got %+v", byID[resolutionDueSoon.ID])
	}
	if byID[actionFailed.ID].RiskLevel != "blocked" || !monitorReasonsContain(byID[actionFailed.ID], "动作执行失败") {
		t.Fatalf("expected action failure blocked, got %+v", byID[actionFailed.ID])
	}
	if byID[actionRunning.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[actionRunning.ID], "动作运行超过") {
		t.Fatalf("expected action running risk, got %+v", byID[actionRunning.ID])
	}
	if byID[humanWaiting.ID].RiskLevel != "risk" || !monitorReasonsContain(byID[humanWaiting.ID], "人工节点等待超过") {
		t.Fatalf("expected human wait risk, got %+v", byID[humanWaiting.ID])
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
}

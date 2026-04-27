package ticket

import (
	"testing"
	"time"

	. "metis/internal/app/itsm/domain"
	"metis/internal/database"
	"metis/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDecisionQualityAggregatesByServiceAndDepartment(t *testing.T) {
	db := newDecisionQualityDB(t)
	now := time.Now()

	service := ServiceDefinition{Name: "VPN 开通", EngineType: "smart", IsActive: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS departments (id INTEGER PRIMARY KEY, name TEXT)`).Error; err != nil {
		t.Fatalf("create departments table: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS user_positions (id INTEGER PRIMARY KEY, user_id INTEGER, department_id INTEGER)`).Error; err != nil {
		t.Fatalf("create user_positions table: %v", err)
	}
	if err := db.Exec(`INSERT INTO departments (id, name) VALUES (10, '网络部'), (20, '安全部')`).Error; err != nil {
		t.Fatalf("seed departments: %v", err)
	}
	if err := db.Exec(`INSERT INTO user_positions (user_id, department_id) VALUES (1, 10), (2, 20)`).Error; err != nil {
		t.Fatalf("seed user_positions: %v", err)
	}

	ticket1 := Ticket{
		Code:        "TICK-001",
		Title:       "VPN 申请",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusCompleted,
		Outcome:     TicketOutcomeApproved,
		PriorityID:  1,
		RequesterID: 1,
	}
	ticket2 := Ticket{
		Code:        "TICK-002",
		Title:       "账号放行",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusFailed,
		Outcome:     TicketOutcomeFailed,
		PriorityID:  1,
		RequesterID: 2,
	}
	if err := db.Create(&ticket1).Error; err != nil {
		t.Fatalf("create ticket1: %v", err)
	}
	if err := db.Create(&ticket2).Error; err != nil {
		t.Fatalf("create ticket2: %v", err)
	}

	activity1 := TicketActivity{TicketID: ticket1.ID, ActivityType: "process", TransitionOutcome: TicketOutcomeApproved, Status: "approved", FinishedAt: &now}
	activity2 := TicketActivity{TicketID: ticket2.ID, ActivityType: "process", TransitionOutcome: TicketOutcomeRejected, Status: "rejected", FinishedAt: &now}
	if err := db.Create(&activity1).Error; err != nil {
		t.Fatalf("create activity1: %v", err)
	}
	if err := db.Create(&activity2).Error; err != nil {
		t.Fatalf("create activity2: %v", err)
	}

	timelineRows := []TicketTimeline{
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-40 * time.Minute)}, TicketID: ticket1.ID, EventType: "activity_completed"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-35 * time.Minute)}, TicketID: ticket1.ID, EventType: "ai_decision_executed"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-20 * time.Minute)}, TicketID: ticket1.ID, EventType: "recovery_retry"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-10 * time.Minute)}, TicketID: ticket1.ID, EventType: "workflow_completed"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-30 * time.Minute)}, TicketID: ticket2.ID, EventType: "activity_completed"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-29 * time.Minute)}, TicketID: ticket2.ID, EventType: "ai_decision_failed"},
		{BaseModel: model.BaseModel{CreatedAt: now.Add(-15 * time.Minute)}, TicketID: ticket2.ID, EventType: "recovery_handoff_human"},
	}
	for i := range timelineRows {
		if err := db.Create(&timelineRows[i]).Error; err != nil {
			t.Fatalf("create timeline row %d: %v", i, err)
		}
	}

	svc := &TicketService{
		ticketRepo: &TicketRepo{db: &database.DB{DB: db}},
	}

	serviceResp, err := svc.DecisionQuality(30, "service", nil, nil)
	if err != nil {
		t.Fatalf("decision quality by service: %v", err)
	}
	if serviceResp.Version != DecisionQualityMetricVersion {
		t.Fatalf("expected metric version %s, got %s", DecisionQualityMetricVersion, serviceResp.Version)
	}
	if len(serviceResp.Items) != 1 {
		t.Fatalf("expected 1 service metric row, got %d", len(serviceResp.Items))
	}
	serviceItem := serviceResp.Items[0]
	if serviceItem.DecisionCount != 2 {
		t.Fatalf("expected decision count 2, got %d", serviceItem.DecisionCount)
	}
	if serviceItem.ApprovalRate != 0.5 || serviceItem.RejectionRate != 0.5 {
		t.Fatalf("expected approval/rejection rate 0.5, got %+v", serviceItem)
	}
	if serviceItem.RetryRate != 0.5 {
		t.Fatalf("expected retry rate 0.5, got %+v", serviceItem.RetryRate)
	}
	if serviceItem.RecoverySuccessRate != 0.5 {
		t.Fatalf("expected recovery success rate 0.5, got %+v", serviceItem.RecoverySuccessRate)
	}
	if serviceItem.AvgDecisionLatencySeconds <= 0 {
		t.Fatalf("expected positive latency, got %+v", serviceItem.AvgDecisionLatencySeconds)
	}

	departmentResp, err := svc.DecisionQuality(30, "department", nil, nil)
	if err != nil {
		t.Fatalf("decision quality by department: %v", err)
	}
	if len(departmentResp.Items) != 2 {
		t.Fatalf("expected 2 department metric rows, got %d", len(departmentResp.Items))
	}
}

func newDecisionQualityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ticket_quality?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ServiceDefinition{}, &Ticket{}, &TicketActivity{}, &TicketTimeline{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

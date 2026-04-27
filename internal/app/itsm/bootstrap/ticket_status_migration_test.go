package bootstrap

import (
	"testing"
	"time"

	. "metis/internal/app/itsm/domain"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateTicketStatusModelMapsLegacyStatusToNewStatusAndOutcome(t *testing.T) {
	db := newTicketStatusMigrationDB(t)
	now := time.Now()

	tickets := []Ticket{
		{Code: "T-1", Title: "legacy pending", ServiceID: 1, EngineType: "smart", Status: "pending", PriorityID: 1, RequesterID: 1},
		{Code: "T-2", Title: "legacy in_progress human", ServiceID: 1, EngineType: "smart", Status: "in_progress", PriorityID: 1, RequesterID: 1},
		{Code: "T-3", Title: "legacy waiting_action", ServiceID: 1, EngineType: "smart", Status: "waiting_action", PriorityID: 1, RequesterID: 1},
		{Code: "T-4", Title: "legacy completed approved", ServiceID: 1, EngineType: "smart", Status: "completed", PriorityID: 1, RequesterID: 1},
		{Code: "T-5", Title: "legacy completed rejected", ServiceID: 1, EngineType: "smart", Status: "completed", PriorityID: 1, RequesterID: 1},
		{Code: "T-6", Title: "legacy completed fulfilled", ServiceID: 1, EngineType: "smart", Status: "completed", PriorityID: 1, RequesterID: 1},
		{Code: "T-7", Title: "legacy cancelled withdrawn", ServiceID: 1, EngineType: "smart", Status: "cancelled", PriorityID: 1, RequesterID: 1},
		{Code: "T-8", Title: "legacy cancelled", ServiceID: 1, EngineType: "smart", Status: "cancelled", PriorityID: 1, RequesterID: 1},
		{Code: "T-9", Title: "legacy failed", ServiceID: 1, EngineType: "smart", Status: "failed", PriorityID: 1, RequesterID: 1},
		{Code: "T-10", Title: "already new status", ServiceID: 1, EngineType: "smart", Status: TicketStatusDecisioning, PriorityID: 1, RequesterID: 1},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create tickets: %v", err)
	}

	activityRows := []TicketActivity{
		{TicketID: tickets[1].ID, ActivityType: "approve", Status: "pending", TransitionOutcome: ""},
		{TicketID: tickets[2].ID, ActivityType: "action", Status: "pending", TransitionOutcome: ""},
		{TicketID: tickets[3].ID, ActivityType: "approve", Status: "completed", TransitionOutcome: TicketOutcomeApproved, FinishedAt: ptrTime(now.Add(-20 * time.Minute))},
		{TicketID: tickets[4].ID, ActivityType: "process", Status: "completed", TransitionOutcome: TicketOutcomeRejected, FinishedAt: ptrTime(now.Add(-15 * time.Minute))},
		{TicketID: tickets[5].ID, ActivityType: "action", Status: "completed", TransitionOutcome: "success", FinishedAt: ptrTime(now.Add(-10 * time.Minute))},
	}
	if err := db.Create(&activityRows).Error; err != nil {
		t.Fatalf("create activities: %v", err)
	}

	assignments := []TicketAssignment{
		{TicketID: tickets[3].ID, ActivityID: activityRows[2].ID, ParticipantType: "user", Status: "completed"},
		{TicketID: tickets[4].ID, ActivityID: activityRows[3].ID, ParticipantType: "user", Status: "completed"},
		{TicketID: tickets[5].ID, ActivityID: activityRows[4].ID, ParticipantType: "user", Status: "completed"},
	}
	if err := db.Create(&assignments).Error; err != nil {
		t.Fatalf("create assignments: %v", err)
	}

	if err := db.Create(&TicketTimeline{
		TicketID:   tickets[6].ID,
		OperatorID: 1,
		EventType:  "withdrawn",
		Message:    "用户撤回",
	}).Error; err != nil {
		t.Fatalf("create withdrawn timeline: %v", err)
	}

	if err := MigrateTicketStatusModel(db); err != nil {
		t.Fatalf("run migration: %v", err)
	}
	// 再跑一次，确认一次性迁移逻辑幂等。
	if err := MigrateTicketStatusModel(db); err != nil {
		t.Fatalf("run migration second time: %v", err)
	}

	assertTicket := func(ticketID uint, wantStatus, wantOutcome string, expectFinished bool) {
		t.Helper()
		var got Ticket
		if err := db.First(&got, ticketID).Error; err != nil {
			t.Fatalf("load ticket %d: %v", ticketID, err)
		}
		if got.Status != wantStatus || got.Outcome != wantOutcome {
			t.Fatalf("ticket %d status/outcome mismatch: got (%s,%s), want (%s,%s)", ticketID, got.Status, got.Outcome, wantStatus, wantOutcome)
		}
		if expectFinished && got.FinishedAt == nil {
			t.Fatalf("ticket %d expected finished_at to be set", ticketID)
		}
		if !expectFinished && got.FinishedAt != nil {
			t.Fatalf("ticket %d expected finished_at to be nil", ticketID)
		}
	}

	assertTicket(tickets[0].ID, TicketStatusSubmitted, "", false)
	assertTicket(tickets[1].ID, TicketStatusWaitingHuman, "", false)
	assertTicket(tickets[2].ID, TicketStatusExecutingAction, "", false)
	assertTicket(tickets[3].ID, TicketStatusCompleted, TicketOutcomeApproved, true)
	assertTicket(tickets[4].ID, TicketStatusRejected, TicketOutcomeRejected, true)
	assertTicket(tickets[5].ID, TicketStatusCompleted, TicketOutcomeFulfilled, true)
	assertTicket(tickets[6].ID, TicketStatusWithdrawn, TicketOutcomeWithdrawn, true)
	assertTicket(tickets[7].ID, TicketStatusCancelled, TicketOutcomeCancelled, true)
	assertTicket(tickets[8].ID, TicketStatusFailed, TicketOutcomeFailed, true)
	assertTicket(tickets[9].ID, TicketStatusDecisioning, "", false)

	var approvedActivity TicketActivity
	if err := db.First(&approvedActivity, activityRows[2].ID).Error; err != nil {
		t.Fatalf("load approved activity: %v", err)
	}
	if approvedActivity.Status != TicketOutcomeApproved {
		t.Fatalf("expected activity status approved, got %s", approvedActivity.Status)
	}

	var rejectedActivity TicketActivity
	if err := db.First(&rejectedActivity, activityRows[3].ID).Error; err != nil {
		t.Fatalf("load rejected activity: %v", err)
	}
	if rejectedActivity.Status != TicketOutcomeRejected {
		t.Fatalf("expected activity status rejected, got %s", rejectedActivity.Status)
	}

	var untouchedActivity TicketActivity
	if err := db.First(&untouchedActivity, activityRows[4].ID).Error; err != nil {
		t.Fatalf("load untouched activity: %v", err)
	}
	if untouchedActivity.Status != "completed" {
		t.Fatalf("expected action activity status unchanged, got %s", untouchedActivity.Status)
	}

	var approvedAssignment TicketAssignment
	if err := db.First(&approvedAssignment, assignments[0].ID).Error; err != nil {
		t.Fatalf("load approved assignment: %v", err)
	}
	if approvedAssignment.Status != AssignmentApproved {
		t.Fatalf("expected assignment status approved, got %s", approvedAssignment.Status)
	}

	var rejectedAssignment TicketAssignment
	if err := db.First(&rejectedAssignment, assignments[1].ID).Error; err != nil {
		t.Fatalf("load rejected assignment: %v", err)
	}
	if rejectedAssignment.Status != AssignmentRejected {
		t.Fatalf("expected assignment status rejected, got %s", rejectedAssignment.Status)
	}

	var untouchedAssignment TicketAssignment
	if err := db.First(&untouchedAssignment, assignments[2].ID).Error; err != nil {
		t.Fatalf("load untouched assignment: %v", err)
	}
	if untouchedAssignment.Status != "completed" {
		t.Fatalf("expected assignment status unchanged, got %s", untouchedAssignment.Status)
	}
}

func newTicketStatusMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:itsm_ticket_status_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Ticket{}, &TicketActivity{}, &TicketAssignment{}, &TicketTimeline{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

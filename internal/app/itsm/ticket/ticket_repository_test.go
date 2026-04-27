package ticket

import (
	. "metis/internal/app/itsm/bootstrap"
	. "metis/internal/app/itsm/domain"
	"testing"
	"time"

	"metis/internal/database"
)

func migrateTicketHistoryTestDB(t *testing.T) *database.DB {
	t.Helper()
	db := newTestDB(t)
	if err := db.AutoMigrate(
		&Priority{},
		&Ticket{},
		&TicketActivity{},
		&TicketAssignment{},
		&TicketTimeline{},
	); err != nil {
		t.Fatalf("migrate ticket tables: %v", err)
	}
	return &database.DB{DB: db}
}

func TestRepairCompletedHumanAssignmentsMakesApproveHistoryVisible(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	now := time.Now()
	operatorID := uint(1)
	priority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	ticket := Ticket{
		Code:        "TICK-1",
		Title:       "审批历史",
		ServiceID:   1,
		EngineType:  "smart",
		Status:      TicketStatusCompleted,
		PriorityID:  priority.ID,
		RequesterID: operatorID,
		FinishedAt:  &now,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	activity := TicketActivity{
		TicketID:     ticket.ID,
		Name:         "审批",
		ActivityType: "approve",
		Status:       "completed",
		FinishedAt:   &now,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	assignment := TicketAssignment{
		TicketID:        ticket.ID,
		ActivityID:      activity.ID,
		ParticipantType: "user",
		UserID:          &operatorID,
		AssigneeID:      &operatorID,
		Status:          AssignmentPending,
		IsCurrent:       true,
	}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}
	if err := db.Create(&TicketTimeline{
		TicketID:   ticket.ID,
		ActivityID: &activity.ID,
		OperatorID: operatorID,
		EventType:  "activity_completed",
		Message:    "活动 [审批] 完成，结果: approved",
	}).Error; err != nil {
		t.Fatalf("create timeline: %v", err)
	}

	repo := &TicketRepo{db: db}
	items, total, err := repo.ListApprovalHistory(TicketApprovalListParams{Page: 1, PageSize: 20}, operatorID)
	if err != nil {
		t.Fatalf("list history before repair: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected dirty pending assignment to be hidden before repair, got total=%d len=%d", total, len(items))
	}

	if err := RepairCompletedHumanAssignments(db.DB); err != nil {
		t.Fatalf("repair assignments: %v", err)
	}

	items, total, err = repo.ListApprovalHistory(TicketApprovalListParams{Page: 1, PageSize: 20}, operatorID)
	if err != nil {
		t.Fatalf("list history after repair: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != ticket.ID {
		t.Fatalf("expected repaired approve ticket in history, got total=%d items=%v", total, items)
	}
}

func TestListApprovalHistoryDeduplicatesMultipleCompletedActivities(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	operatorID := uint(1)
	priority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	ticket := Ticket{
		Code:        "TICK-2",
		Title:       "多次审批",
		ServiceID:   1,
		EngineType:  "smart",
		Status:      TicketStatusCompleted,
		PriorityID:  priority.ID,
		RequesterID: operatorID,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	for i := 0; i < 2; i++ {
		finishedAt := time.Now().Add(time.Duration(i) * time.Minute)
		activity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "审批",
			ActivityType: "approve",
			Status:       "completed",
			FinishedAt:   &finishedAt,
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity %d: %v", i, err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:        ticket.ID,
			ActivityID:      activity.ID,
			ParticipantType: "user",
			UserID:          &operatorID,
			AssigneeID:      &operatorID,
			Status:          AssignmentApproved,
			FinishedAt:      &finishedAt,
		}).Error; err != nil {
			t.Fatalf("create assignment %d: %v", i, err)
		}
	}

	repo := &TicketRepo{db: db}
	items, total, err := repo.ListApprovalHistory(TicketApprovalListParams{Page: 1, PageSize: 20}, operatorID)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != ticket.ID {
		t.Fatalf("expected one deduplicated ticket, got total=%d items=%v", total, items)
	}
}

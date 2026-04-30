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

func TestListApprovalHistoryOrdersByLatestFinishedAtDesc(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	operatorID := uint(1)
	priority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}

	olderTicket := Ticket{
		Code:        "TICK-HIST-OLDER",
		Title:       "历史较旧",
		ServiceID:   1,
		EngineType:  "smart",
		Status:      TicketStatusCompleted,
		PriorityID:  priority.ID,
		RequesterID: operatorID,
	}
	if err := db.Create(&olderTicket).Error; err != nil {
		t.Fatalf("create older ticket: %v", err)
	}
	newerTicket := Ticket{
		Code:        "TICK-HIST-NEWER",
		Title:       "历史较新",
		ServiceID:   1,
		EngineType:  "smart",
		Status:      TicketStatusCompleted,
		PriorityID:  priority.ID,
		RequesterID: operatorID,
	}
	if err := db.Create(&newerTicket).Error; err != nil {
		t.Fatalf("create newer ticket: %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)
	for i, ticket := range []Ticket{olderTicket, newerTicket} {
		finishedAt := oldTime
		if i == 1 {
			finishedAt = newTime
		}
		activity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "审批",
			ActivityType: "approve",
			Status:       "completed",
			FinishedAt:   &finishedAt,
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create history activity %d: %v", i, err)
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
			t.Fatalf("create history assignment %d: %v", i, err)
		}
	}

	repo := &TicketRepo{db: db}
	items, total, err := repo.ListApprovalHistory(TicketApprovalListParams{Page: 1, PageSize: 20}, operatorID)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 history tickets, got total=%d len=%d", total, len(items))
	}
	if items[0].ID != newerTicket.ID || items[1].ID != olderTicket.ID {
		t.Fatalf("expected newer ticket first, got items=%v", items)
	}
}

func TestListPendingApprovalsDeduplicatesAndOrdersByPriorityThenCreatedAt(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	operatorID := uint(1)
	highPriority := Priority{Name: "高", Code: "high", Value: 1, Color: "#f00"}
	normalPriority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&highPriority).Error; err != nil {
		t.Fatalf("create high priority: %v", err)
	}
	if err := db.Create(&normalPriority).Error; err != nil {
		t.Fatalf("create normal priority: %v", err)
	}

	early := time.Now().Add(-2 * time.Hour)
	late := time.Now().Add(-1 * time.Hour)
	ticketA := Ticket{
		Code:        "TICK-PEND-A",
		Title:       "高优先级",
		ServiceID:   1,
		EngineType:  "smart",
		Status:      TicketStatusSubmitted,
		PriorityID:  highPriority.ID,
		RequesterID: operatorID,
	}
	ticketB := Ticket{
		Code:        "TICK-PEND-B",
		Title:       "普通优先级-更早",
		ServiceID:   1,
		EngineType:  "smart",
		Status:      TicketStatusSubmitted,
		PriorityID:  normalPriority.ID,
		RequesterID: operatorID,
	}
	if err := db.Create(&ticketA).Error; err != nil {
		t.Fatalf("create ticket A: %v", err)
	}
	if err := db.Create(&ticketB).Error; err != nil {
		t.Fatalf("create ticket B: %v", err)
	}
	if err := db.Model(&Ticket{}).Where("id = ?", ticketA.ID).Update("created_at", late).Error; err != nil {
		t.Fatalf("update ticket A created_at: %v", err)
	}
	if err := db.Model(&Ticket{}).Where("id = ?", ticketB.ID).Update("created_at", early).Error; err != nil {
		t.Fatalf("update ticket B created_at: %v", err)
	}

	addPending := func(ticketID uint, suffix string) {
		activity := TicketActivity{
			TicketID:     ticketID,
			Name:         "审批-" + suffix,
			ActivityType: "approve",
			Status:       "pending",
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create pending activity %s: %v", suffix, err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:        ticketID,
			ActivityID:      activity.ID,
			ParticipantType: "user",
			UserID:          &operatorID,
			AssigneeID:      &operatorID,
			Status:          AssignmentPending,
		}).Error; err != nil {
			t.Fatalf("create pending assignment %s: %v", suffix, err)
		}
	}

	addPending(ticketA.ID, "A1")
	addPending(ticketA.ID, "A2")
	addPending(ticketB.ID, "B1")

	repo := &TicketRepo{db: db}
	items, total, err := repo.ListPendingApprovals(TicketApprovalListParams{Page: 1, PageSize: 20}, operatorID, nil, nil)
	if err != nil {
		t.Fatalf("list pending approvals: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 deduplicated pending tickets, got total=%d len=%d", total, len(items))
	}
	if items[0].ID != ticketA.ID || items[1].ID != ticketB.ID {
		t.Fatalf("expected high priority ticket first, got items=%v", items)
	}
}

func TestListSupportsGroupedStatusFilters(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	priority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	requesterID := uint(7)
	statuses := []string{
		TicketStatusSubmitted,
		TicketStatusWaitingHuman,
		TicketStatusDecisioning,
		TicketStatusCompleted,
		TicketStatusRejected,
	}
	for i, status := range statuses {
		ticket := Ticket{
			Code:        "TICK-GROUP-" + string(rune('A'+i)),
			Title:       "group filter",
			ServiceID:   1,
			EngineType:  "smart",
			Status:      status,
			PriorityID:  priority.ID,
			RequesterID: requesterID,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket %s: %v", status, err)
		}
	}

	repo := &TicketRepo{db: db}
	activeItems, activeTotal, err := repo.List(TicketListParams{
		RequesterID: &requesterID,
		Status:      "active",
		Page:        1,
		PageSize:    20,
	})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if activeTotal != 3 || len(activeItems) != 3 {
		t.Fatalf("expected 3 active tickets, got total=%d len=%d", activeTotal, len(activeItems))
	}
	for _, item := range activeItems {
		if IsTerminalTicketStatus(item.Status) {
			t.Fatalf("active filter should not contain terminal status, got %s", item.Status)
		}
	}

	terminalItems, terminalTotal, err := repo.List(TicketListParams{
		RequesterID: &requesterID,
		Status:      "terminal",
		Page:        1,
		PageSize:    20,
	})
	if err != nil {
		t.Fatalf("list terminal: %v", err)
	}
	if terminalTotal != 2 || len(terminalItems) != 2 {
		t.Fatalf("expected 2 terminal tickets, got total=%d len=%d", terminalTotal, len(terminalItems))
	}
	for _, item := range terminalItems {
		if !IsTerminalTicketStatus(item.Status) {
			t.Fatalf("terminal filter should only contain terminal status, got %s", item.Status)
		}
	}
}

func TestListDecisioningFilterIncludesDecisioningVariants(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	priority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	requesterID := uint(9)
	statuses := []string{
		TicketStatusDecisioning,
		TicketStatusApprovedDecisioning,
		TicketStatusRejectedDecisioning,
		TicketStatusWaitingHuman,
	}
	for i, status := range statuses {
		ticket := Ticket{
			Code:        "TICK-DEC-" + string(rune('A'+i)),
			Title:       "decisioning filter",
			ServiceID:   1,
			EngineType:  "smart",
			Status:      status,
			PriorityID:  priority.ID,
			RequesterID: requesterID,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket %s: %v", status, err)
		}
	}

	repo := &TicketRepo{db: db}
	items, total, err := repo.List(TicketListParams{
		RequesterID: &requesterID,
		Status:      TicketStatusDecisioning,
		Page:        1,
		PageSize:    20,
	})
	if err != nil {
		t.Fatalf("list decisioning: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("expected 3 decisioning tickets, got total=%d len=%d", total, len(items))
	}
	for _, item := range items {
		if item.Status != TicketStatusDecisioning && item.Status != TicketStatusApprovedDecisioning && item.Status != TicketStatusRejectedDecisioning {
			t.Fatalf("unexpected status in decisioning filter: %s", item.Status)
		}
	}
}

func TestListDateRangeFiltersByCreatedAt(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	priority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	requesterID := uint(11)
	createTicketAt := func(code string, ts time.Time) uint {
		ticket := Ticket{
			Code:        code,
			Title:       "date filter",
			ServiceID:   1,
			EngineType:  "smart",
			Status:      TicketStatusCompleted,
			PriorityID:  priority.ID,
			RequesterID: requesterID,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket %s: %v", code, err)
		}
		if err := db.Model(&Ticket{}).Where("id = ?", ticket.ID).
			Updates(map[string]any{"created_at": ts, "updated_at": ts}).Error; err != nil {
			t.Fatalf("set created_at for %s: %v", code, err)
		}
		return ticket.ID
	}

	createTicketAt("TICK-DATE-1", time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC))
	createTicketAt("TICK-DATE-2", time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC))
	createTicketAt("TICK-DATE-3", time.Date(2026, 4, 20, 18, 0, 0, 0, time.UTC))

	start := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 15, 23, 59, 59, 0, time.UTC)

	repo := &TicketRepo{db: db}
	items, total, err := repo.List(TicketListParams{
		RequesterID: &requesterID,
		StartDate:   &start,
		EndDate:     &end,
		Page:        1,
		PageSize:    20,
	})
	if err != nil {
		t.Fatalf("list date range: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 ticket in created_at range, got total=%d len=%d", total, len(items))
	}
	if items[0].Code != "TICK-DATE-2" {
		t.Fatalf("expected TICK-DATE-2, got %s", items[0].Code)
	}
}

func TestWorkspaceListsClampPageSizeToOneHundred(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	operatorID := uint(77)
	priority := Priority{Name: "普通", Code: "normal-page-size", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	for i := 0; i < 120; i++ {
		ticket := Ticket{
			Code:        "TICK-CLAMP-" + string(rune('A'+(i%26))) + string(rune('A'+(i/26))),
			Title:       "分页上限",
			ServiceID:   1,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  priority.ID,
			RequesterID: operatorID,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket %d: %v", i, err)
		}
		activity := TicketActivity{TicketID: ticket.ID, Name: "处理", ActivityType: "process", Status: "pending"}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity %d: %v", i, err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &operatorID,
			AssigneeID: &operatorID,
			Status:     AssignmentPending,
		}).Error; err != nil {
			t.Fatalf("create assignment %d: %v", i, err)
		}
	}

	repo := &TicketRepo{db: db}
	mine, total, err := repo.List(TicketListParams{RequesterID: &operatorID, Page: 1, PageSize: 100000})
	if err != nil {
		t.Fatalf("list mine: %v", err)
	}
	if total != 120 || len(mine) != 100 {
		t.Fatalf("expected mine page size clamp to 100, total=%d len=%d", total, len(mine))
	}
	pending, total, err := repo.ListPendingApprovals(TicketApprovalListParams{Page: 1, PageSize: 100000}, operatorID, nil, nil)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if total != 120 || len(pending) != 100 {
		t.Fatalf("expected pending page size clamp to 100, total=%d len=%d", total, len(pending))
	}
}

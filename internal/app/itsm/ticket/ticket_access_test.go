package ticket

import (
	"errors"
	. "metis/internal/app/itsm/domain"
	"metis/internal/database"
	"metis/internal/model"
	"testing"
	"time"
)

func testTicketServiceForAccess(db *database.DB, orgResolver *rootDBOrgResolver) *TicketService {
	svc := &TicketService{
		ticketRepo:   &TicketRepo{db: db},
		timelineRepo: &TimelineRepo{db: db},
	}
	if orgResolver != nil {
		svc.orgResolver = orgResolver
	}
	return svc
}

func TestEnsureCanViewTicketAllowsWorkspaceActorsAndRejectsUnrelatedUser(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	if err := db.Exec("CREATE TABLE operator_positions (user_id INTEGER NOT NULL, position_id INTEGER NOT NULL)").Error; err != nil {
		t.Fatalf("create operator_positions: %v", err)
	}
	if err := db.Exec("CREATE TABLE operator_departments (user_id INTEGER NOT NULL, department_id INTEGER NOT NULL)").Error; err != nil {
		t.Fatalf("create operator_departments: %v", err)
	}
	if err := db.Exec("INSERT INTO operator_positions (user_id, position_id) VALUES (?, ?)", 30, 300).Error; err != nil {
		t.Fatalf("seed operator position: %v", err)
	}
	if err := db.Exec("INSERT INTO operator_departments (user_id, department_id) VALUES (?, ?)", 30, 400).Error; err != nil {
		t.Fatalf("seed operator department: %v", err)
	}
	svc := testTicketServiceForAccess(db, &rootDBOrgResolver{db: db.DB})
	priority := Priority{Name: "普通", Code: "normal", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	ticket := Ticket{
		Code:        "TICK-ACCESS",
		Title:       "对象级可见性",
		ServiceID:   1,
		EngineType:  "smart",
		Status:      TicketStatusWaitingHuman,
		PriorityID:  priority.ID,
		RequesterID: 10,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	pendingActivity := TicketActivity{TicketID: ticket.ID, Name: "待处理", ActivityType: "process", Status: "pending"}
	if err := db.Create(&pendingActivity).Error; err != nil {
		t.Fatalf("create pending activity: %v", err)
	}
	directUser := uint(20)
	if err := db.Create(&TicketAssignment{
		TicketID:   ticket.ID,
		ActivityID: pendingActivity.ID,
		UserID:     &directUser,
		AssigneeID: &directUser,
		Status:     AssignmentPending,
	}).Error; err != nil {
		t.Fatalf("create direct assignment: %v", err)
	}
	positionID, departmentID := uint(300), uint(400)
	if err := db.Create(&TicketAssignment{
		TicketID:        ticket.ID,
		ActivityID:      pendingActivity.ID,
		ParticipantType: "position_department",
		PositionID:      &positionID,
		DepartmentID:    &departmentID,
		Status:          AssignmentPending,
	}).Error; err != nil {
		t.Fatalf("create org assignment: %v", err)
	}
	finishedAt := time.Now()
	historyActivity := TicketActivity{TicketID: ticket.ID, Name: "已处理", ActivityType: "approve", Status: "completed", FinishedAt: &finishedAt}
	if err := db.Create(&historyActivity).Error; err != nil {
		t.Fatalf("create history activity: %v", err)
	}
	historyUser := uint(40)
	if err := db.Create(&TicketAssignment{
		TicketID:   ticket.ID,
		ActivityID: historyActivity.ID,
		AssigneeID: &historyUser,
		Status:     AssignmentApproved,
		FinishedAt: &finishedAt,
	}).Error; err != nil {
		t.Fatalf("create history assignment: %v", err)
	}

	for _, tc := range []struct {
		name string
		user uint
		role string
	}{
		{name: "requester", user: 10, role: model.RoleUser},
		{name: "direct pending assignee", user: 20, role: model.RoleUser},
		{name: "org pending participant", user: 30, role: model.RoleUser},
		{name: "history assignee", user: 40, role: model.RoleUser},
		{name: "admin", user: 99, role: model.RoleAdmin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.EnsureCanViewTicket(ticket.ID, tc.user, tc.role); err != nil {
				t.Fatalf("expected access, got %v", err)
			}
		})
	}

	if err := svc.EnsureCanViewTicket(ticket.ID, 50, model.RoleUser); !errors.Is(err, ErrTicketForbidden) {
		t.Fatalf("expected unrelated user forbidden, got %v", err)
	}
	if err := svc.EnsureCanViewTicket(9999, 10, model.RoleUser); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected missing ticket not found, got %v", err)
	}
}

func TestTaskDispatchRejectsActivityFromDifferentTicketWithoutMutation(t *testing.T) {
	db := migrateTicketHistoryTestDB(t)
	svc := testTicketServiceForAccess(db, nil)
	priority := Priority{Name: "普通", Code: "normal-dispatch", Value: 10, Color: "#666"}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	ticketA := Ticket{Code: "TICK-DISPATCH-A", Title: "A", ServiceID: 1, EngineType: "smart", Status: TicketStatusWaitingHuman, PriorityID: priority.ID, RequesterID: 1}
	ticketB := Ticket{Code: "TICK-DISPATCH-B", Title: "B", ServiceID: 1, EngineType: "smart", Status: TicketStatusWaitingHuman, PriorityID: priority.ID, RequesterID: 2}
	if err := db.Create(&ticketA).Error; err != nil {
		t.Fatalf("create ticket A: %v", err)
	}
	if err := db.Create(&ticketB).Error; err != nil {
		t.Fatalf("create ticket B: %v", err)
	}
	activityA := TicketActivity{TicketID: ticketA.ID, Name: "A activity", ActivityType: "process", Status: "pending"}
	if err := db.Create(&activityA).Error; err != nil {
		t.Fatalf("create activity A: %v", err)
	}
	operatorID := uint(7)
	assignmentA := TicketAssignment{TicketID: ticketA.ID, ActivityID: activityA.ID, UserID: &operatorID, AssigneeID: &operatorID, Status: AssignmentPending, IsCurrent: true}
	if err := db.Create(&assignmentA).Error; err != nil {
		t.Fatalf("create assignment A: %v", err)
	}

	for _, tc := range []struct {
		name string
		call func() (*Ticket, error)
	}{
		{name: "claim", call: func() (*Ticket, error) { return svc.Claim(ticketB.ID, activityA.ID, operatorID) }},
		{name: "transfer", call: func() (*Ticket, error) { return svc.Transfer(ticketB.ID, activityA.ID, 99, operatorID) }},
		{name: "delegate", call: func() (*Ticket, error) { return svc.Delegate(ticketB.ID, activityA.ID, 99, operatorID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.call(); !errors.Is(err, ErrNoActiveAssignment) {
				t.Fatalf("expected ErrNoActiveAssignment, got %v", err)
			}
			var reloadedAssignment TicketAssignment
			if err := db.First(&reloadedAssignment, assignmentA.ID).Error; err != nil {
				t.Fatalf("reload assignment: %v", err)
			}
			if reloadedAssignment.Status != AssignmentPending {
				t.Fatalf("assignment status mutated to %s", reloadedAssignment.Status)
			}
			var reloadedB Ticket
			if err := db.First(&reloadedB, ticketB.ID).Error; err != nil {
				t.Fatalf("reload ticket B: %v", err)
			}
			if reloadedB.AssigneeID != nil || reloadedB.Status != TicketStatusWaitingHuman {
				t.Fatalf("ticket B mutated: assignee=%v status=%s", reloadedB.AssigneeID, reloadedB.Status)
			}
			var timelineCount int64
			if err := db.Model(&TicketTimeline{}).Where("ticket_id = ?", ticketB.ID).Count(&timelineCount).Error; err != nil {
				t.Fatalf("count timelines: %v", err)
			}
			if timelineCount != 0 {
				t.Fatalf("expected no ticket B timeline, got %d", timelineCount)
			}
		})
	}
}

package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// approvalTestFixture holds all objects created by setupApprovalTest.
type approvalTestFixture struct {
	db          *gorm.DB
	engine      *ClassicEngine
	ticket      ticketModel
	activity    activityModel
	assigneeIDs []uint
}

// setupApprovalTest creates an in-memory SQLite database with all required tables,
// a ClassicEngine, a ticket, a token, an approval activity, and N assignments.
func setupApprovalTest(t *testing.T, mode string, numAssignees int) *approvalTestFixture {
	t.Helper()

	// Open a unique in-memory SQLite database per test
	dsn := fmt.Sprintf("file:approval_%p?mode=memory&cache=shared", t)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	// AutoMigrate all required models
	if err := db.AutoMigrate(
		&ticketModel{},
		&activityModel{},
		&assignmentModel{},
		&timelineModel{},
		&executionTokenModel{},
		&formDefModel{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Add columns that exist in the full ITSM model but not in the lightweight engine structs.
	// The engine code references these via string-based GORM updates.
	db.Exec("ALTER TABLE itsm_tickets ADD COLUMN assignee_id INTEGER")
	db.Exec("ALTER TABLE itsm_ticket_assignments ADD COLUMN finished_at DATETIME")

	engine := NewClassicEngine(NewParticipantResolver(nil), &noopSubmitter{}, nil)

	// Build workflow JSON: start -> approve -> end
	// The approve node's approve_mode must match the mode parameter.
	workflowJSON := fmt.Sprintf(`{
		"nodes": [
			{"id": "start1", "type": "start", "data": {}},
			{"id": "approve1", "type": "approve", "data": {"label": "审批", "approve_mode": "%s", "participants": [{"type": "user", "value": "1"}]}},
			{"id": "end1", "type": "end", "data": {}}
		],
		"edges": [
			{"id": "e1", "source": "start1", "target": "approve1", "data": {}},
			{"id": "e2", "source": "approve1", "target": "end1", "data": {"outcome": "approve"}},
			{"id": "e3", "source": "approve1", "target": "end1", "data": {"outcome": "reject"}}
		]
	}`, mode)

	// Create ticket
	ticket := ticketModel{
		Status:       "in_progress",
		WorkflowJSON: workflowJSON,
		RequesterID:  100,
		EngineType:   "classic",
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	// Create execution token
	token := executionTokenModel{
		TicketID:  ticket.ID,
		NodeID:    "approve1",
		Status:    TokenActive,
		TokenType: TokenMain,
		ScopeID:   "root",
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Create approval activity
	activity := activityModel{
		TicketID:      ticket.ID,
		TokenID:       &token.ID,
		Name:          "审批",
		ActivityType:  NodeApprove,
		Status:        ActivityPending,
		NodeID:        "approve1",
		ExecutionMode: mode,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("failed to create activity: %v", err)
	}

	// Update ticket's current_activity_id
	db.Model(&ticketModel{}).Where("id = ?", ticket.ID).Update("current_activity_id", activity.ID)

	// Create N assignments with unique assignee IDs
	assigneeIDs := make([]uint, numAssignees)
	for i := 0; i < numAssignees; i++ {
		uid := uint(i + 1)
		assigneeIDs[i] = uid
		assignment := assignmentModel{
			TicketID:        ticket.ID,
			ActivityID:      activity.ID,
			ParticipantType: "user",
			UserID:          &uid,
			AssigneeID:      &uid,
			Status:          "pending",
			Sequence:        i,
			IsCurrent:       i == 0,
		}
		if err := db.Create(&assignment).Error; err != nil {
			t.Fatalf("failed to create assignment %d: %v", i, err)
		}
	}

	return &approvalTestFixture{
		db:          db,
		engine:      engine,
		ticket:      ticket,
		activity:    activity,
		assigneeIDs: assigneeIDs,
	}
}

// loadActivity reloads an activity from the database.
func loadActivity(t *testing.T, db *gorm.DB, id uint) activityModel {
	t.Helper()
	var a activityModel
	if err := db.First(&a, id).Error; err != nil {
		t.Fatalf("failed to load activity %d: %v", id, err)
	}
	return a
}

// loadAssignments returns all assignments for an activity ordered by sequence.
func loadAssignments(t *testing.T, db *gorm.DB, activityID uint) []assignmentModel {
	t.Helper()
	var assignments []assignmentModel
	if err := db.Where("activity_id = ?", activityID).Order("sequence ASC").Find(&assignments).Error; err != nil {
		t.Fatalf("failed to load assignments: %v", err)
	}
	return assignments
}

// loadTicket reloads a ticket from the database.
func loadTicket(t *testing.T, db *gorm.DB, id uint) ticketModel {
	t.Helper()
	var tk ticketModel
	if err := db.First(&tk, id).Error; err != nil {
		t.Fatalf("failed to load ticket %d: %v", id, err)
	}
	return tk
}

// --- Test 1: Parallel Approval — All Approve ---

func TestParallelApproval_AllApprove(t *testing.T) {
	f := setupApprovalTest(t, "parallel", 3)
	ctx := context.Background()

	// First two users approve; activity should remain pending
	for i := 0; i < 2; i++ {
		err := f.engine.Progress(ctx, f.db, ProgressParams{
			TicketID:   f.ticket.ID,
			ActivityID: f.activity.ID,
			Outcome:    "approve",
			OperatorID: f.assigneeIDs[i],
		})
		if err != nil {
			t.Fatalf("progress user %d: %v", f.assigneeIDs[i], err)
		}
	}

	// After 2 of 3, activity should still be pending
	act := loadActivity(t, f.db, f.activity.ID)
	if act.Status == ActivityCompleted {
		t.Fatal("activity should not be completed after 2 of 3 approvals")
	}

	// Third user approves — activity should complete
	err := f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "approve",
		OperatorID: f.assigneeIDs[2],
	})
	if err != nil {
		t.Fatalf("progress user %d: %v", f.assigneeIDs[2], err)
	}

	// Verify activity completed with "approve" outcome
	act = loadActivity(t, f.db, f.activity.ID)
	if act.Status != ActivityCompleted {
		t.Errorf("activity status: got %q, want %q", act.Status, ActivityCompleted)
	}
	if act.TransitionOutcome != "approve" {
		t.Errorf("transition_outcome: got %q, want %q", act.TransitionOutcome, "approve")
	}

	// All 3 assignments should be completed
	assignments := loadAssignments(t, f.db, f.activity.ID)
	for _, a := range assignments {
		if a.Status != "completed" {
			t.Errorf("assignment %d status: got %q, want %q", a.ID, a.Status, "completed")
		}
	}

	// Ticket should advance to end and be completed
	tk := loadTicket(t, f.db, f.ticket.ID)
	if tk.Status != "completed" {
		t.Errorf("ticket status: got %q, want %q", tk.Status, "completed")
	}
}

// --- Test 2: Parallel Approval — One Reject ---

func TestParallelApproval_OneReject(t *testing.T) {
	f := setupApprovalTest(t, "parallel", 3)
	ctx := context.Background()

	// First user approves
	err := f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "approve",
		OperatorID: f.assigneeIDs[0],
	})
	if err != nil {
		t.Fatalf("progress user %d (approve): %v", f.assigneeIDs[0], err)
	}

	// Second user rejects — should immediately complete
	err = f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "reject",
		OperatorID: f.assigneeIDs[1],
	})
	if err != nil {
		t.Fatalf("progress user %d (reject): %v", f.assigneeIDs[1], err)
	}

	// Activity should be completed with "reject" outcome
	act := loadActivity(t, f.db, f.activity.ID)
	if act.Status != ActivityCompleted {
		t.Errorf("activity status: got %q, want %q", act.Status, ActivityCompleted)
	}
	if act.TransitionOutcome != "reject" {
		t.Errorf("transition_outcome: got %q, want %q", act.TransitionOutcome, "reject")
	}

	// Check assignment statuses
	assignments := loadAssignments(t, f.db, f.activity.ID)
	completedCount := 0
	cancelledCount := 0
	for _, a := range assignments {
		switch a.Status {
		case "completed":
			completedCount++
		case "cancelled":
			cancelledCount++
		}
	}
	// Two assignments completed (user 0 approved, user 1 rejected), one cancelled (user 2)
	if completedCount != 2 {
		t.Errorf("completed assignments: got %d, want 2", completedCount)
	}
	if cancelledCount != 1 {
		t.Errorf("cancelled assignments: got %d, want 1", cancelledCount)
	}

	// Ticket should advance to end and be completed (reject edge also goes to end)
	tk := loadTicket(t, f.db, f.ticket.ID)
	if tk.Status != "completed" {
		t.Errorf("ticket status: got %q, want %q", tk.Status, "completed")
	}
}

// --- Test 3: Sequential Approval — Chain Advance ---

func TestSequentialApproval_ChainAdvance(t *testing.T) {
	f := setupApprovalTest(t, "sequential", 3)
	ctx := context.Background()

	// Step 1: User 0 approves
	err := f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "approve",
		OperatorID: f.assigneeIDs[0],
	})
	if err != nil {
		t.Fatalf("progress user 0: %v", err)
	}

	t.Run("after_user0_approve", func(t *testing.T) {
		assignments := loadAssignments(t, f.db, f.activity.ID)
		// Assignment 0 should be completed
		if assignments[0].Status != "completed" {
			t.Errorf("assignment 0 status: got %q, want %q", assignments[0].Status, "completed")
		}
		// Assignment 1 should be current
		if !assignments[1].IsCurrent {
			t.Error("assignment 1 should be is_current=true")
		}
		if assignments[1].Status != "pending" {
			t.Errorf("assignment 1 status: got %q, want %q", assignments[1].Status, "pending")
		}
		// Activity should still be pending
		act := loadActivity(t, f.db, f.activity.ID)
		if act.Status == ActivityCompleted {
			t.Error("activity should not yet be completed")
		}
	})

	// Step 2: User 1 approves
	err = f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "approve",
		OperatorID: f.assigneeIDs[1],
	})
	if err != nil {
		t.Fatalf("progress user 1: %v", err)
	}

	t.Run("after_user1_approve", func(t *testing.T) {
		assignments := loadAssignments(t, f.db, f.activity.ID)
		if assignments[1].Status != "completed" {
			t.Errorf("assignment 1 status: got %q, want %q", assignments[1].Status, "completed")
		}
		if !assignments[2].IsCurrent {
			t.Error("assignment 2 should be is_current=true")
		}
		// Activity should still not be completed
		act := loadActivity(t, f.db, f.activity.ID)
		if act.Status == ActivityCompleted {
			t.Error("activity should not yet be completed")
		}
	})

	// Step 3: User 2 approves — final
	err = f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "approve",
		OperatorID: f.assigneeIDs[2],
	})
	if err != nil {
		t.Fatalf("progress user 2: %v", err)
	}

	t.Run("after_user2_approve", func(t *testing.T) {
		// Activity should be completed with "approve"
		act := loadActivity(t, f.db, f.activity.ID)
		if act.Status != ActivityCompleted {
			t.Errorf("activity status: got %q, want %q", act.Status, ActivityCompleted)
		}
		if act.TransitionOutcome != "approve" {
			t.Errorf("transition_outcome: got %q, want %q", act.TransitionOutcome, "approve")
		}

		// All assignments should be completed
		assignments := loadAssignments(t, f.db, f.activity.ID)
		for i, a := range assignments {
			if a.Status != "completed" {
				t.Errorf("assignment %d status: got %q, want %q", i, a.Status, "completed")
			}
		}

		// Ticket should be completed
		tk := loadTicket(t, f.db, f.ticket.ID)
		if tk.Status != "completed" {
			t.Errorf("ticket status: got %q, want %q", tk.Status, "completed")
		}
	})
}

// --- Test 4: Sequential Approval — Reject Midchain ---

func TestSequentialApproval_RejectMidchain(t *testing.T) {
	f := setupApprovalTest(t, "sequential", 3)
	ctx := context.Background()

	// User 0 approves
	err := f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "approve",
		OperatorID: f.assigneeIDs[0],
	})
	if err != nil {
		t.Fatalf("progress user 0 (approve): %v", err)
	}

	// User 1 rejects — activity should complete immediately with "reject"
	err = f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "reject",
		OperatorID: f.assigneeIDs[1],
	})
	if err != nil {
		t.Fatalf("progress user 1 (reject): %v", err)
	}

	// Activity should be completed with "reject"
	act := loadActivity(t, f.db, f.activity.ID)
	if act.Status != ActivityCompleted {
		t.Errorf("activity status: got %q, want %q", act.Status, ActivityCompleted)
	}
	if act.TransitionOutcome != "reject" {
		t.Errorf("transition_outcome: got %q, want %q", act.TransitionOutcome, "reject")
	}

	// Check assignments
	assignments := loadAssignments(t, f.db, f.activity.ID)
	// Assignment 0: completed (approved)
	if assignments[0].Status != "completed" {
		t.Errorf("assignment 0 status: got %q, want %q", assignments[0].Status, "completed")
	}
	// Assignment 1: completed (rejected)
	if assignments[1].Status != "completed" {
		t.Errorf("assignment 1 status: got %q, want %q", assignments[1].Status, "completed")
	}
	// Assignment 2: cancelled (remaining)
	if assignments[2].Status != "cancelled" {
		t.Errorf("assignment 2 status: got %q, want %q", assignments[2].Status, "cancelled")
	}

	// Ticket should advance to end via the reject edge and be completed
	tk := loadTicket(t, f.db, f.ticket.ID)
	if tk.Status != "completed" {
		t.Errorf("ticket status: got %q, want %q", tk.Status, "completed")
	}
}

// --- Test 5: Single Approval — Does Not Use progressApproval Path ---

func TestSingleApproval_Unchanged(t *testing.T) {
	f := setupApprovalTest(t, "single", 1)
	ctx := context.Background()

	// Progress with single mode — should complete directly, not through progressApproval
	err := f.engine.Progress(ctx, f.db, ProgressParams{
		TicketID:   f.ticket.ID,
		ActivityID: f.activity.ID,
		Outcome:    "approve",
		OperatorID: f.assigneeIDs[0],
	})
	if err != nil {
		t.Fatalf("progress: %v", err)
	}

	// Activity should be completed
	act := loadActivity(t, f.db, f.activity.ID)
	if act.Status != ActivityCompleted {
		t.Errorf("activity status: got %q, want %q", act.Status, ActivityCompleted)
	}
	if act.TransitionOutcome != "approve" {
		t.Errorf("transition_outcome: got %q, want %q", act.TransitionOutcome, "approve")
	}

	// Assignment should remain pending (single mode does not touch assignments via progressApproval)
	assignments := loadAssignments(t, f.db, f.activity.ID)
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
	// In single mode, Progress does not call progressApproval, so assignments are untouched
	if assignments[0].Status != "pending" {
		t.Errorf("assignment status: got %q, want %q (single mode bypasses progressApproval)", assignments[0].Status, "pending")
	}

	// Ticket should be completed (workflow advanced to end)
	tk := loadTicket(t, f.db, f.ticket.ID)
	if tk.Status != "completed" {
		t.Errorf("ticket status: got %q, want %q", tk.Status, "completed")
	}
}

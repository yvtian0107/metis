package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// noopSubmitter implements TaskSubmitter as a no-op for testing.
type noopSubmitter struct{}

func (*noopSubmitter) SubmitTask(string, json.RawMessage) error { return nil }

// dispatchFixture holds the objects returned by setupDispatchTest.
type dispatchFixture struct {
	db         *gorm.DB
	engine     *ClassicEngine
	ticket     ticketModel
	token      executionTokenModel
	activity   activityModel
	assignment assignmentModel
}

// setupDispatchTest creates an in-memory SQLite database with all required tables,
// a ClassicEngine with a noop submitter, and a base scenario: a ticket (in_progress),
// an active main token, a pending parallel-approve activity, and one pending assignment
// for user 100.
func setupDispatchTest(t *testing.T) *dispatchFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:dispatch_%p?mode=memory&cache=shared", t)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&ticketModel{},
		&executionTokenModel{},
		&activityModel{},
		&assignmentModel{},
		&timelineModel{},
		&processVariableModel{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Add columns that exist in the full ITSM model but not in the lightweight engine structs.
	// The engine code references these via string-based GORM updates.
	db.Exec("ALTER TABLE itsm_tickets ADD COLUMN assignee_id INTEGER")
	db.Exec("ALTER TABLE itsm_ticket_assignments ADD COLUMN finished_at DATETIME")

	workflowJSON := `{"nodes":[{"id":"s","type":"start","data":{}},{"id":"a1","type":"approve","data":{"label":"审批","approve_mode":"parallel","participants":[{"type":"user","value":"100"}]}},{"id":"e","type":"end","data":{}}],"edges":[{"id":"e1","source":"s","target":"a1","data":{}},{"id":"e2","source":"a1","target":"e","data":{"condition":{"field":"outcome","operator":"eq","value":"approve"}}}]}`

	ticket := ticketModel{
		ID:           1,
		Status:       "in_progress",
		WorkflowJSON: workflowJSON,
		RequesterID:  50,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}

	token := executionTokenModel{
		TicketID:  1,
		NodeID:    "a1",
		Status:    TokenActive,
		TokenType: TokenMain,
		ScopeID:   "root",
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	activity := activityModel{
		TicketID:      1,
		TokenID:       &token.ID,
		Name:          "审批",
		ActivityType:  NodeApprove,
		Status:        ActivityPending,
		NodeID:        "a1",
		ExecutionMode: "parallel",
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("failed to create activity: %v", err)
	}

	// Point ticket.current_activity_id at the activity
	db.Model(&ticketModel{}).Where("id = ?", ticket.ID).Update("current_activity_id", activity.ID)

	assigneeID := uint(100)
	assignment := assignmentModel{
		TicketID:        1,
		ActivityID:      activity.ID,
		ParticipantType: "user",
		UserID:          &assigneeID,
		AssigneeID:      &assigneeID,
		Status:          "pending",
		Sequence:        0,
		IsCurrent:       true,
	}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("failed to create assignment: %v", err)
	}

	eng := NewClassicEngine(NewParticipantResolver(nil), &noopSubmitter{}, nil)

	return &dispatchFixture{
		db:         db,
		engine:     eng,
		ticket:     ticket,
		token:      token,
		activity:   activity,
		assignment: assignment,
	}
}

func TestDispatch(t *testing.T) {
	t.Run("Transfer_CreatesNewAssignment", func(t *testing.T) {
		f := setupDispatchTest(t)
		db := f.db
		original := f.assignment

		// Simulate transfer: mark original as transferred
		if err := db.Model(&assignmentModel{}).Where("id = ?", original.ID).
			Updates(map[string]any{"status": "transferred", "is_current": false}).Error; err != nil {
			t.Fatalf("failed to mark original as transferred: %v", err)
		}

		// Create new assignment for user 200 with transfer_from
		newAssigneeID := uint(200)
		newAssignment := assignmentModel{
			TicketID:        original.TicketID,
			ActivityID:      original.ActivityID,
			ParticipantType: "user",
			UserID:          &newAssigneeID,
			AssigneeID:      &newAssigneeID,
			Status:          "pending",
			Sequence:        0,
			IsCurrent:       true,
			TransferFrom:    &original.ID,
		}
		if err := db.Create(&newAssignment).Error; err != nil {
			t.Fatalf("failed to create transferred assignment: %v", err)
		}

		// Update ticket assignee
		if err := db.Model(&ticketModel{}).Where("id = ?", f.ticket.ID).
			Update("assignee_id", 200).Error; err != nil {
			t.Fatalf("failed to update ticket assignee: %v", err)
		}

		// Assert: original assignment status is "transferred"
		var reloadedOriginal assignmentModel
		if err := db.First(&reloadedOriginal, original.ID).Error; err != nil {
			t.Fatalf("failed to reload original: %v", err)
		}
		if reloadedOriginal.Status != "transferred" {
			t.Errorf("original assignment status: got %q, want %q", reloadedOriginal.Status, "transferred")
		}
		if reloadedOriginal.IsCurrent {
			t.Error("original assignment should not be current after transfer")
		}

		// Assert: new assignment has correct transfer_from
		var reloadedNew assignmentModel
		if err := db.First(&reloadedNew, newAssignment.ID).Error; err != nil {
			t.Fatalf("failed to reload new assignment: %v", err)
		}
		if reloadedNew.TransferFrom == nil || *reloadedNew.TransferFrom != original.ID {
			t.Errorf("new assignment transfer_from: got %v, want %d", reloadedNew.TransferFrom, original.ID)
		}
		if reloadedNew.Status != "pending" {
			t.Errorf("new assignment status: got %q, want %q", reloadedNew.Status, "pending")
		}
		if !reloadedNew.IsCurrent {
			t.Error("new assignment should be current")
		}
		if reloadedNew.AssigneeID == nil || *reloadedNew.AssigneeID != 200 {
			t.Errorf("new assignment assignee_id: got %v, want 200", reloadedNew.AssigneeID)
		}

		// Assert: ticket assignee updated to 200
		var assigneeID int
		db.Raw("SELECT assignee_id FROM itsm_tickets WHERE id = ?", f.ticket.ID).Scan(&assigneeID)
		if assigneeID != 200 {
			t.Errorf("ticket assignee_id: got %d, want 200", assigneeID)
		}
	})

	t.Run("Delegate_CreatesNewAssignment", func(t *testing.T) {
		f := setupDispatchTest(t)
		db := f.db
		original := f.assignment

		// Simulate delegate: mark original as delegated
		if err := db.Model(&assignmentModel{}).Where("id = ?", original.ID).
			Updates(map[string]any{"status": "delegated", "is_current": false}).Error; err != nil {
			t.Fatalf("failed to mark original as delegated: %v", err)
		}

		// Create new assignment for user 200 with delegated_from
		delegateAssigneeID := uint(200)
		delegated := assignmentModel{
			TicketID:        original.TicketID,
			ActivityID:      original.ActivityID,
			ParticipantType: "user",
			UserID:          &delegateAssigneeID,
			AssigneeID:      &delegateAssigneeID,
			Status:          "pending",
			Sequence:        0,
			IsCurrent:       true,
			DelegatedFrom:   &original.ID,
		}
		if err := db.Create(&delegated).Error; err != nil {
			t.Fatalf("failed to create delegated assignment: %v", err)
		}

		// Assert: original assignment status is "delegated"
		var reloadedOriginal assignmentModel
		if err := db.First(&reloadedOriginal, original.ID).Error; err != nil {
			t.Fatalf("failed to reload original: %v", err)
		}
		if reloadedOriginal.Status != "delegated" {
			t.Errorf("original assignment status: got %q, want %q", reloadedOriginal.Status, "delegated")
		}
		if reloadedOriginal.IsCurrent {
			t.Error("original assignment should not be current after delegation")
		}

		// Assert: delegated assignment has delegated_from pointing to original
		var reloadedDelegated assignmentModel
		if err := db.First(&reloadedDelegated, delegated.ID).Error; err != nil {
			t.Fatalf("failed to reload delegated assignment: %v", err)
		}
		if reloadedDelegated.DelegatedFrom == nil || *reloadedDelegated.DelegatedFrom != original.ID {
			t.Errorf("delegated assignment delegated_from: got %v, want %d", reloadedDelegated.DelegatedFrom, original.ID)
		}
		if reloadedDelegated.Status != "pending" {
			t.Errorf("delegated assignment status: got %q, want %q", reloadedDelegated.Status, "pending")
		}
		if !reloadedDelegated.IsCurrent {
			t.Error("delegated assignment should be current")
		}
	})

	t.Run("Delegate_AutoReturn", func(t *testing.T) {
		f := setupDispatchTest(t)
		db := f.db

		// Setup: mark user 100's original assignment as "delegated"
		if err := db.Model(&assignmentModel{}).Where("id = ?", f.assignment.ID).
			Updates(map[string]any{"status": "delegated", "is_current": false}).Error; err != nil {
			t.Fatalf("failed to mark original as delegated: %v", err)
		}

		// Create delegated assignment for user 200 with delegated_from pointing to original
		delegateAssigneeID := uint(200)
		delegated := assignmentModel{
			TicketID:        f.assignment.TicketID,
			ActivityID:      f.assignment.ActivityID,
			ParticipantType: "user",
			UserID:          &delegateAssigneeID,
			AssigneeID:      &delegateAssigneeID,
			Status:          "pending",
			Sequence:        0,
			IsCurrent:       true,
			DelegatedFrom:   &f.assignment.ID,
		}
		if err := db.Create(&delegated).Error; err != nil {
			t.Fatalf("failed to create delegated assignment: %v", err)
		}

		// Call engine.Progress with OperatorID=200 and Outcome="approve"
		err := f.engine.Progress(context.Background(), db, ProgressParams{
			TicketID:   f.ticket.ID,
			ActivityID: f.activity.ID,
			Outcome:    "approve",
			OperatorID: 200,
		})
		if err != nil {
			t.Fatalf("engine.Progress failed: %v", err)
		}

		// Assert: user 200's assignment is "completed"
		var reloadedDelegated assignmentModel
		if err := db.First(&reloadedDelegated, delegated.ID).Error; err != nil {
			t.Fatalf("failed to reload delegated assignment: %v", err)
		}
		if reloadedDelegated.Status != "completed" {
			t.Errorf("delegated assignment status: got %q, want %q", reloadedDelegated.Status, "completed")
		}

		// Assert: original assignment (user 100) is restored to "pending" with is_current=true
		var reloadedOriginal assignmentModel
		if err := db.First(&reloadedOriginal, f.assignment.ID).Error; err != nil {
			t.Fatalf("failed to reload original assignment: %v", err)
		}
		if reloadedOriginal.Status != "pending" {
			t.Errorf("original assignment status after auto-return: got %q, want %q", reloadedOriginal.Status, "pending")
		}
		if !reloadedOriginal.IsCurrent {
			t.Error("original assignment should be current after auto-return")
		}

		// Assert: timeline has "delegate_return" event
		var timeline timelineModel
		if err := db.Where("ticket_id = ? AND event_type = ?", f.ticket.ID, "delegate_return").
			First(&timeline).Error; err != nil {
			t.Fatalf("expected delegate_return timeline event, got error: %v", err)
		}
		if timeline.EventType != "delegate_return" {
			t.Errorf("timeline event_type: got %q, want %q", timeline.EventType, "delegate_return")
		}

		// Assert: workflow does NOT advance (activity should still be pending)
		var reloadedActivity activityModel
		if err := db.First(&reloadedActivity, f.activity.ID).Error; err != nil {
			t.Fatalf("failed to reload activity: %v", err)
		}
		if reloadedActivity.Status != ActivityPending {
			t.Errorf("activity status should remain pending after delegate auto-return, got %q", reloadedActivity.Status)
		}
	})

	t.Run("Claim_MarksOthersClaimedByOther", func(t *testing.T) {
		f := setupDispatchTest(t)
		db := f.db

		// Add two more assignments for users 200 and 300
		user200 := uint(200)
		user300 := uint(300)
		assign200 := assignmentModel{
			TicketID:        f.ticket.ID,
			ActivityID:      f.activity.ID,
			ParticipantType: "user",
			UserID:          &user200,
			AssigneeID:      &user200,
			Status:          "pending",
			Sequence:        1,
			IsCurrent:       true,
		}
		assign300 := assignmentModel{
			TicketID:        f.ticket.ID,
			ActivityID:      f.activity.ID,
			ParticipantType: "user",
			UserID:          &user300,
			AssigneeID:      &user300,
			Status:          "pending",
			Sequence:        2,
			IsCurrent:       true,
		}
		if err := db.Create(&assign200).Error; err != nil {
			t.Fatalf("failed to create assign200: %v", err)
		}
		if err := db.Create(&assign300).Error; err != nil {
			t.Fatalf("failed to create assign300: %v", err)
		}

		// Simulate claim by user 200: mark 200 as completed/current, others as claimed_by_other
		if err := db.Model(&assignmentModel{}).Where("id = ?", assign200.ID).
			Updates(map[string]any{"status": "completed", "is_current": true}).Error; err != nil {
			t.Fatalf("failed to update assign200: %v", err)
		}
		// Mark users 100 and 300 as claimed_by_other
		if err := db.Model(&assignmentModel{}).
			Where("activity_id = ? AND id != ?", f.activity.ID, assign200.ID).
			Updates(map[string]any{"status": "claimed_by_other", "is_current": false}).Error; err != nil {
			t.Fatalf("failed to update others: %v", err)
		}
		// Update ticket assignee to 200
		if err := db.Model(&ticketModel{}).Where("id = ?", f.ticket.ID).
			Update("assignee_id", 200).Error; err != nil {
			t.Fatalf("failed to update ticket assignee: %v", err)
		}

		// Assert: user 200 has completed status
		var reloaded200 assignmentModel
		if err := db.First(&reloaded200, assign200.ID).Error; err != nil {
			t.Fatalf("failed to reload assign200: %v", err)
		}
		if reloaded200.Status != "completed" {
			t.Errorf("user 200 assignment status: got %q, want %q", reloaded200.Status, "completed")
		}

		// Assert: users 100 and 300 have "claimed_by_other"
		var reloaded100 assignmentModel
		if err := db.First(&reloaded100, f.assignment.ID).Error; err != nil {
			t.Fatalf("failed to reload assign100: %v", err)
		}
		if reloaded100.Status != "claimed_by_other" {
			t.Errorf("user 100 assignment status: got %q, want %q", reloaded100.Status, "claimed_by_other")
		}
		if reloaded100.IsCurrent {
			t.Error("user 100 should not be current after claim")
		}

		var reloaded300 assignmentModel
		if err := db.First(&reloaded300, assign300.ID).Error; err != nil {
			t.Fatalf("failed to reload assign300: %v", err)
		}
		if reloaded300.Status != "claimed_by_other" {
			t.Errorf("user 300 assignment status: got %q, want %q", reloaded300.Status, "claimed_by_other")
		}
		if reloaded300.IsCurrent {
			t.Error("user 300 should not be current after claim")
		}

		// Assert: ticket assignee is 200
		var assigneeID int
		db.Raw("SELECT assignee_id FROM itsm_tickets WHERE id = ?", f.ticket.ID).Scan(&assigneeID)
		if assigneeID != 200 {
			t.Errorf("ticket assignee_id: got %d, want 200", assigneeID)
		}
	})

	t.Run("Transfer_OriginalAssignmentNotPending_Fails", func(t *testing.T) {
		f := setupDispatchTest(t)
		db := f.db

		// Setup: mark user 100's assignment as already completed
		if err := db.Model(&assignmentModel{}).Where("id = ?", f.assignment.ID).
			Update("status", "completed").Error; err != nil {
			t.Fatalf("failed to mark assignment as completed: %v", err)
		}

		// Try to find a pending assignment for transfer -- should find nothing
		result := db.Model(&assignmentModel{}).
			Where("activity_id = ? AND assignee_id = ? AND status = ?",
				f.activity.ID, 100, "pending").
			Update("status", "transferred")

		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}
		if result.RowsAffected != 0 {
			t.Errorf("expected 0 rows affected for non-pending assignment, got %d", result.RowsAffected)
		}
	})
}

package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestClassicProgressCompletesApproveAssignment(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&ticketModel{},
		&activityModel{},
		&assignmentModel{},
		&timelineModel{},
		&executionTokenModel{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Exec("ALTER TABLE itsm_tickets ADD COLUMN assignee_id INTEGER").Error; err != nil {
		t.Fatalf("add assignee_id: %v", err)
	}

	workflow := json.RawMessage(`{
		"nodes": [
			{"id": "start", "type": "start", "data": {"label": "开始"}},
			{"id": "approve", "type": "approve", "data": {"label": "审批", "participants": [{"type": "user", "value": "1"}]}},
			{"id": "end", "type": "end", "data": {"label": "结束"}}
		],
		"edges": [
			{"id": "e1", "source": "start", "target": "approve", "data": {}},
			{"id": "e2", "source": "approve", "target": "end", "data": {"outcome": "approved"}}
		]
	}`)
	ticket := ticketModel{Status: "pending", WorkflowJSON: string(workflow), RequesterID: 1}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	eng := NewClassicEngine(NewParticipantResolver(nil), nil, nil)
	if err := eng.Start(context.Background(), db, StartParams{
		TicketID:     ticket.ID,
		WorkflowJSON: workflow,
		RequesterID:  1,
	}); err != nil {
		t.Fatalf("start workflow: %v", err)
	}

	var activity activityModel
	if err := db.Where("ticket_id = ? AND activity_type = ?", ticket.ID, NodeApprove).First(&activity).Error; err != nil {
		t.Fatalf("find approve activity: %v", err)
	}

	if err := eng.Progress(context.Background(), db, ProgressParams{
		TicketID:   ticket.ID,
		ActivityID: activity.ID,
		Outcome:    "approved",
		Opinion:    "同意",
		OperatorID: 1,
	}); err != nil {
		t.Fatalf("progress approve activity: %v", err)
	}

	var assignment assignmentModel
	if err := db.Where("activity_id = ?", activity.ID).First(&assignment).Error; err != nil {
		t.Fatalf("find assignment: %v", err)
	}
	if assignment.Status != ActivityApproved {
		t.Fatalf("expected assignment approved, got %q", assignment.Status)
	}
	if assignment.AssigneeID == nil || *assignment.AssigneeID != 1 {
		t.Fatalf("expected assignee_id=1, got %v", assignment.AssigneeID)
	}
	if assignment.FinishedAt == nil {
		t.Fatalf("expected assignment finished_at to be set")
	}
	if assignment.IsCurrent {
		t.Fatalf("expected completed assignment to no longer be current")
	}
}

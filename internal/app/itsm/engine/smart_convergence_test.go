package engine

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

// mockConfigProvider implements EngineConfigProvider for convergence timeout tests.
type mockConfigProvider struct {
	convergenceTimeout time.Duration
}

func (m *mockConfigProvider) FallbackAssigneeID() uint                  { return 0 }
func (m *mockConfigProvider) DecisionMode() string                      { return "ai_only" }
func (m *mockConfigProvider) DecisionAgentID() uint                     { return 0 }
func (m *mockConfigProvider) AuditLevel() string                        { return "full" }
func (m *mockConfigProvider) SLACriticalThresholdSeconds() int          { return 1800 }
func (m *mockConfigProvider) SLAWarningThresholdSeconds() int           { return 3600 }
func (m *mockConfigProvider) SimilarHistoryLimit() int                  { return 5 }
func (m *mockConfigProvider) ParallelConvergenceTimeout() time.Duration { return m.convergenceTimeout }

func TestSmartCompletionStatusPreservesOutcomeWithSharedEnd(t *testing.T) {
	tests := []struct {
		name        string
		outcome     string
		wantStatus  string
		wantOutcome string
	}{
		{name: "approved", outcome: ActivityApproved, wantStatus: TicketStatusCompleted, wantOutcome: TicketOutcomeApproved},
		{name: "rejected", outcome: ActivityRejected, wantStatus: TicketStatusRejected, wantOutcome: TicketOutcomeRejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newSmartContinuationDB(t)
			ticket, activity := createSmartContinuationTicket(t, db, "shared-end", tt.outcome)
			sharedEndWorkflow := `{
				"nodes":[
					{"id":"start","type":"start","data":{"label":"开始"}},
					{"id":"process","type":"process","data":{"label":"处理","participants":[{"type":"requester"}]}},
					{"id":"end","type":"end","data":{"label":"完成"}}
				],
				"edges":[
					{"id":"e1","source":"start","target":"process","data":{}},
					{"id":"e2","source":"process","target":"end","data":{"outcome":"approved"}},
					{"id":"e3","source":"process","target":"end","data":{"outcome":"rejected"}}
				]
			}`
			if err := db.Model(&ticketModel{}).Where("id = ?", ticket.ID).Update("workflow_json", sharedEndWorkflow).Error; err != nil {
				t.Fatalf("set workflow json: %v", err)
			}
			now := time.Now()
			if err := db.Model(&activityModel{}).Where("id = ?", activity.ID).Updates(map[string]any{
				"status":             HumanActivityResultStatus(tt.outcome),
				"transition_outcome": tt.outcome,
				"finished_at":        now,
			}).Error; err != nil {
				t.Fatalf("complete activity: %v", err)
			}

			eng := &SmartEngine{}
			gotStatus, gotOutcome := eng.resolveCompletionStatus(db, ticket.ID)
			if gotStatus != tt.wantStatus || gotOutcome != tt.wantOutcome {
				t.Fatalf("ticket status/outcome = %s/%s, want %s/%s", gotStatus, gotOutcome, tt.wantStatus, tt.wantOutcome)
			}
		})
	}
}

func TestConvergenceTimeoutCancelsPendingActivities(t *testing.T) {
	db := newSmartContinuationDB(t)

	// Create a ticket with two activities in the same parallel group.
	ticket, first := createSmartContinuationTicket(t, db, "conv-group-1", ActivityPending)
	second := activityModel{
		TicketID:        ticket.ID,
		Name:            "并行处理 B",
		ActivityType:    NodeProcess,
		Status:          ActivityPending,
		ActivityGroupID: "conv-group-1",
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second activity: %v", err)
	}

	// Push the group's created_at back 73 hours so it exceeds the 72h config timeout.
	pastTime := time.Now().Add(-73 * time.Hour)
	if err := db.Model(&activityModel{}).Where("id = ?", first.ID).Update("created_at", pastTime).Error; err != nil {
		t.Fatalf("backdate first activity: %v", err)
	}
	if err := db.Model(&activityModel{}).Where("id = ?", second.ID).Update("created_at", pastTime).Error; err != nil {
		t.Fatalf("backdate second activity: %v", err)
	}

	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(
		availableDecisionExecutor{}, nil, nil, nil, submitter,
		&mockConfigProvider{convergenceTimeout: 72 * time.Hour},
	)

	// Progress the first activity (marks it completed, triggers ensureContinuation).
	if err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: first.ID,
			Outcome:    "completed",
			OperatorID: 1,
		})
	}); err != nil {
		t.Fatalf("progress first activity: %v", err)
	}

	// Assert: the second activity should be cancelled.
	var reloadedSecond activityModel
	if err := db.First(&reloadedSecond, second.ID).Error; err != nil {
		t.Fatalf("reload second activity: %v", err)
	}
	if reloadedSecond.Status != ActivityCancelled {
		t.Fatalf("expected second activity status %q, got %q", ActivityCancelled, reloadedSecond.Status)
	}

	// Assert: a parallel_convergence_timeout timeline event is recorded.
	var timelineCount int64
	if err := db.Model(&timelineModel{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, "parallel_convergence_timeout").
		Count(&timelineCount).Error; err != nil {
		t.Fatalf("count timeline: %v", err)
	}
	if timelineCount != 1 {
		t.Fatalf("expected 1 parallel_convergence_timeout timeline event, got %d", timelineCount)
	}

	if submitter.txCalls != 0 {
		t.Fatalf("expected no scheduler submit call after convergence timeout, got %d", submitter.txCalls)
	}
}

func TestConvergenceTimeoutPreservesCompletedResults(t *testing.T) {
	db := newSmartContinuationDB(t)

	// Create a ticket with three activities in the same group.
	ticket, first := createSmartContinuationTicket(t, db, "conv-group-2", ActivityPending)

	second := activityModel{
		TicketID:        ticket.ID,
		Name:            "并行处理 B",
		ActivityType:    NodeProcess,
		Status:          ActivityPending,
		ActivityGroupID: "conv-group-2",
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second activity: %v", err)
	}
	assignSmartActivityToOperator(t, db, ticket.ID, second.ID, 1)

	third := activityModel{
		TicketID:        ticket.ID,
		Name:            "并行处理 C",
		ActivityType:    NodeProcess,
		Status:          ActivityPending,
		ActivityGroupID: "conv-group-2",
	}
	if err := db.Create(&third).Error; err != nil {
		t.Fatalf("create third activity: %v", err)
	}

	// Complete activities 1 and 2 directly in the database (simulate prior completions).
	now := time.Now()
	if err := db.Model(&activityModel{}).Where("id = ?", first.ID).Updates(map[string]any{
		"status":      ActivityCompleted,
		"finished_at": now,
	}).Error; err != nil {
		t.Fatalf("complete first activity: %v", err)
	}

	// Push the group's created_at back 73 hours.
	pastTime := time.Now().Add(-73 * time.Hour)
	for _, id := range []uint{first.ID, second.ID, third.ID} {
		if err := db.Model(&activityModel{}).Where("id = ?", id).Update("created_at", pastTime).Error; err != nil {
			t.Fatalf("backdate activity %d: %v", id, err)
		}
	}

	// Point current_activity_id at the second activity (the one we will progress).
	if err := db.Model(&ticketModel{}).Where("id = ?", ticket.ID).Update("current_activity_id", second.ID).Error; err != nil {
		t.Fatalf("set current activity to second: %v", err)
	}

	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(
		availableDecisionExecutor{}, nil, nil, nil, submitter,
		&mockConfigProvider{convergenceTimeout: 72 * time.Hour},
	)

	// Progress the second activity (as if it just completed).
	if err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: second.ID,
			Outcome:    "completed",
			OperatorID: 1,
		})
	}); err != nil {
		t.Fatalf("progress second activity: %v", err)
	}

	// Assert: activity 3 (pending) is cancelled.
	var reloadedThird activityModel
	if err := db.First(&reloadedThird, third.ID).Error; err != nil {
		t.Fatalf("reload third activity: %v", err)
	}
	if reloadedThird.Status != ActivityCancelled {
		t.Fatalf("expected third activity status %q, got %q", ActivityCancelled, reloadedThird.Status)
	}

	// Assert: activities 1 and 2 remain completed (not touched).
	var reloadedFirst activityModel
	if err := db.First(&reloadedFirst, first.ID).Error; err != nil {
		t.Fatalf("reload first activity: %v", err)
	}
	if reloadedFirst.Status != ActivityCompleted {
		t.Fatalf("expected first activity status %q, got %q", ActivityCompleted, reloadedFirst.Status)
	}

	var reloadedSecond activityModel
	if err := db.First(&reloadedSecond, second.ID).Error; err != nil {
		t.Fatalf("reload second activity: %v", err)
	}
	if reloadedSecond.Status != ActivityApproved {
		t.Fatalf("expected second activity status %q, got %q", ActivityApproved, reloadedSecond.Status)
	}
}

func TestConvergenceNoTimeoutWaitsNormally(t *testing.T) {
	db := newSmartContinuationDB(t)

	// Create two activities in a group with recent created_at (now, the default).
	ticket, first := createSmartContinuationTicket(t, db, "conv-group-3", ActivityPending)
	second := activityModel{
		TicketID:        ticket.ID,
		Name:            "并行处理 B",
		ActivityType:    NodeProcess,
		Status:          ActivityPending,
		ActivityGroupID: "conv-group-3",
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second activity: %v", err)
	}

	submitter := &txRecordingSubmitter{}
	eng := NewSmartEngine(
		availableDecisionExecutor{}, nil, nil, nil, submitter,
		&mockConfigProvider{convergenceTimeout: 72 * time.Hour},
	)

	// Progress the first activity.
	if err := db.Transaction(func(tx *gorm.DB) error {
		return eng.Progress(context.Background(), tx, ProgressParams{
			TicketID:   ticket.ID,
			ActivityID: first.ID,
			Outcome:    "completed",
			OperatorID: 1,
		})
	}); err != nil {
		t.Fatalf("progress first activity: %v", err)
	}

	// Assert: the second activity is still pending (NOT cancelled).
	var reloadedSecond activityModel
	if err := db.First(&reloadedSecond, second.ID).Error; err != nil {
		t.Fatalf("reload second activity: %v", err)
	}
	if reloadedSecond.Status != ActivityPending {
		t.Fatalf("expected second activity status %q, got %q", ActivityPending, reloadedSecond.Status)
	}

	// Assert: no decision task submitted — still waiting for sibling convergence.
	if submitter.txCalls != 0 {
		t.Fatalf("expected 0 tx submit calls (still waiting for convergence), got %d", submitter.txCalls)
	}
}

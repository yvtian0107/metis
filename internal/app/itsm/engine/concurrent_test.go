package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// noopSubmitterC implements TaskSubmitter as a no-op for concurrency tests.
// Uses a unique name to avoid collision with other test files in the same package.
type noopSubmitterC struct{}

func (*noopSubmitterC) SubmitTask(string, json.RawMessage) error { return nil }

// workflowJSON is a minimal start → form → end workflow for testing Progress.
const testWorkflowJSON = `{"nodes":[{"id":"s","type":"start","data":{}},{"id":"f1","type":"form","data":{"label":"填写表单","participants":[{"type":"user","value":"1"}]}},{"id":"e","type":"end","data":{}}],"edges":[{"id":"e1","source":"s","target":"f1","data":{}},{"id":"e2","source":"f1","target":"e","data":{"default":true}}]}`

// setupConcurrencyTest creates an in-memory SQLite database, auto-migrates all engine models,
// and returns the DB, a ClassicEngine, and the IDs of the created ticket, token, and activity.
func setupConcurrencyTest(t *testing.T) (db *gorm.DB, eng *ClassicEngine, ticketID, tokenID, activityID uint) {
	t.Helper()

	dsn := fmt.Sprintf("file:conc_%p?mode=memory&cache=shared", t)
	var err error
	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := db.AutoMigrate(
		&ticketModel{},
		&activityModel{},
		&assignmentModel{},
		&timelineModel{},
		&executionTokenModel{},
		&processVariableModel{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	resolver := NewParticipantResolver(nil)
	eng = NewClassicEngine(resolver, &noopSubmitterC{}, nil)

	// Create ticket
	ticket := &ticketModel{
		Status:       "in_progress",
		EngineType:   "classic",
		WorkflowJSON: testWorkflowJSON,
		RequesterID:  1,
	}
	if err := db.Create(ticket).Error; err != nil {
		t.Fatalf("failed to create ticket: %v", err)
	}
	ticketID = ticket.ID

	// Create root execution token (active, main, root)
	token := &executionTokenModel{
		TicketID:  ticketID,
		NodeID:    "f1",
		Status:    TokenActive,
		TokenType: TokenMain,
		ScopeID:   "root",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	tokenID = token.ID

	// Create form activity (pending, single mode)
	now := time.Now()
	activity := &activityModel{
		TicketID:      ticketID,
		TokenID:       &token.ID,
		Name:          "填写表单",
		ActivityType:  NodeForm,
		Status:        ActivityPending,
		NodeID:        "f1",
		ExecutionMode: "single",
		StartedAt:     &now,
	}
	if err := db.Create(activity).Error; err != nil {
		t.Fatalf("failed to create activity: %v", err)
	}
	activityID = activity.ID

	// Create assignment so Progress can complete properly
	uid := uint(1)
	assignment := &assignmentModel{
		TicketID:        ticketID,
		ActivityID:      activityID,
		ParticipantType: "user",
		UserID:          &uid,
		AssigneeID:      &uid,
		Status:          "pending",
		Sequence:        0,
		IsCurrent:       true,
	}
	if err := db.Create(assignment).Error; err != nil {
		t.Fatalf("failed to create assignment: %v", err)
	}

	// Point ticket's current_activity_id at this activity
	db.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", activityID)

	return db, eng, ticketID, tokenID, activityID
}

// TestProgress_ActivityAlreadyCompleted_Fails verifies that calling Progress on an
// already-completed activity returns ErrActivityNotActive.
func TestProgress_ActivityAlreadyCompleted_Fails(t *testing.T) {
	db, eng, ticketID, _, activityID := setupConcurrencyTest(t)

	// Manually mark the activity as completed before calling Progress
	db.Model(&activityModel{}).Where("id = ?", activityID).Update("status", ActivityCompleted)

	err := eng.Progress(context.Background(), db, ProgressParams{
		TicketID:   ticketID,
		ActivityID: activityID,
		Outcome:    "completed",
		OperatorID: 1,
	})
	if !errors.Is(err, ErrActivityNotActive) {
		t.Errorf("expected ErrActivityNotActive, got: %v", err)
	}
}

// TestProgress_TokenNotActive_Fails verifies that calling Progress when the execution
// token is not active returns ErrTokenNotActive.
func TestProgress_TokenNotActive_Fails(t *testing.T) {
	db, eng, ticketID, tokenID, activityID := setupConcurrencyTest(t)

	// Mark the token as completed
	db.Model(&executionTokenModel{}).Where("id = ?", tokenID).Update("status", TokenCompleted)

	err := eng.Progress(context.Background(), db, ProgressParams{
		TicketID:   ticketID,
		ActivityID: activityID,
		Outcome:    "completed",
		OperatorID: 1,
	})
	if !errors.Is(err, ErrTokenNotActive) {
		t.Errorf("expected ErrTokenNotActive, got: %v", err)
	}
}

// TestProgress_SequentialCallsSameActivity verifies idempotency: the first Progress
// call succeeds and the second returns ErrActivityNotActive because the activity
// was already completed.
func TestProgress_SequentialCallsSameActivity(t *testing.T) {
	db, eng, ticketID, _, activityID := setupConcurrencyTest(t)

	// First call should succeed: advances through form → end, completes ticket
	err := eng.Progress(context.Background(), db, ProgressParams{
		TicketID:   ticketID,
		ActivityID: activityID,
		Outcome:    "completed",
		OperatorID: 1,
	})
	if err != nil {
		t.Fatalf("first Progress call should succeed, got: %v", err)
	}

	// Verify the human activity records the submitted outcome directly.
	var act activityModel
	db.First(&act, activityID)
	if act.Status != ActivityApproved {
		t.Errorf("activity status after first Progress: got %q, want %q", act.Status, ActivityApproved)
	}

	// Second call on the same activity should fail
	err = eng.Progress(context.Background(), db, ProgressParams{
		TicketID:   ticketID,
		ActivityID: activityID,
		Outcome:    "completed",
		OperatorID: 1,
	})
	if !errors.Is(err, ErrActivityNotActive) {
		t.Errorf("second Progress call: expected ErrActivityNotActive, got: %v", err)
	}
}

// TestProgress_ActivityNotFound verifies that calling Progress with a non-existent
// activity ID returns ErrActivityNotFound.
func TestProgress_ActivityNotFound(t *testing.T) {
	db, eng, ticketID, _, _ := setupConcurrencyTest(t)

	err := eng.Progress(context.Background(), db, ProgressParams{
		TicketID:   ticketID,
		ActivityID: 99999, // does not exist
		Outcome:    "completed",
		OperatorID: 1,
	})
	if !errors.Is(err, ErrActivityNotFound) {
		t.Errorf("expected ErrActivityNotFound, got: %v", err)
	}
}

// TestProgress_ConcurrentGoroutines launches 10 goroutines all calling Progress
// simultaneously on the same pending activity.
//
// IMPORTANT: SQLite does NOT support SELECT ... FOR UPDATE. The clause.Locking
// in Progress is silently ignored, so multiple goroutines may read the activity
// as "pending" before any of them writes "completed". This means more than one
// goroutine can succeed with SQLite. With a real PostgreSQL database, the FOR
// UPDATE lock would serialize access and exactly 1 goroutine would succeed.
//
// This test validates:
//  1. At least 1 goroutine succeeds (the workflow advances).
//  2. Every goroutine either succeeds or receives an expected error
//     (ErrActivityNotActive or ErrTokenNotActive — the latter occurs when the
//     token was already completed by another goroutine reaching the end node).
//  3. No panics or unexpected errors occur under concurrent access.
func TestProgress_ConcurrentGoroutines(t *testing.T) {
	db, eng, ticketID, _, activityID := setupConcurrencyTest(t)

	const goroutines = 10
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes int
		notActive int
		tokenDone int
		otherErrs []error
	)

	wg.Add(goroutines)
	// Use a channel as a barrier so all goroutines start at roughly the same time
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			<-start

			err := eng.Progress(context.Background(), db, ProgressParams{
				TicketID:   ticketID,
				ActivityID: activityID,
				Outcome:    "completed",
				OperatorID: 1,
			})

			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrActivityNotActive), errors.Is(err, ErrNoActiveAssignment):
				notActive++
			case errors.Is(err, ErrTokenNotActive):
				tokenDone++
			default:
				otherErrs = append(otherErrs, err)
			}
		}()
	}

	// Release all goroutines simultaneously
	close(start)
	wg.Wait()

	t.Logf("results: successes=%d, notActive=%d, tokenDone=%d, otherErrors=%d",
		successes, notActive, tokenDone, len(otherErrs))
	for _, e := range otherErrs {
		t.Logf("  unexpected error: %v", e)
	}

	// At least 1 goroutine must succeed
	if successes < 1 {
		t.Errorf("expected at least 1 success, got %d", successes)
	}

	// All goroutines should resolve to a known outcome (no unexpected errors)
	if len(otherErrs) > 0 {
		t.Errorf("got %d unexpected errors (see log above)", len(otherErrs))
	}

	// Total should account for all goroutines
	total := successes + notActive + tokenDone + len(otherErrs)
	if total != goroutines {
		t.Errorf("total outcomes (%d) != goroutines (%d)", total, goroutines)
	}
}

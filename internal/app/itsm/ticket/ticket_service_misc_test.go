package ticket

import (
	"errors"
	"testing"
	"time"

	. "metis/internal/app/itsm/domain"
	"metis/internal/app/itsm/engine"
	"metis/internal/database"
	"metis/internal/model"

	"gorm.io/gorm"
)

func TestTicketService_ValidateHumanProgressContracts(t *testing.T) {
	db := newTestDB(t)
	svc := newSubmissionTicketService(t, db)
	service := testutilSeedSmartTicketService(t, db)

	ticket := Ticket{
		Code:        "TICK-VALIDATE-PROGRESS",
		Title:       "validate progress",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusWaitingHuman,
		PriorityID:  1,
		RequesterID: 7,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	approve := TicketActivity{TicketID: ticket.ID, Name: "approve", ActivityType: engine.NodeApprove, Status: engine.ActivityPending}
	if err := db.Create(&approve).Error; err != nil {
		t.Fatalf("create approve activity: %v", err)
	}
	action := TicketActivity{TicketID: ticket.ID, Name: "action", ActivityType: engine.NodeAction, Status: engine.ActivityPending}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("create action activity: %v", err)
	}

	if err := svc.validateHumanProgress(ticket.ID, approve.ID, "approved", "ok", 9); err != nil {
		t.Fatalf("expected approved human progress to pass, got %v", err)
	}
	if err := svc.validateHumanProgress(ticket.ID, approve.ID, " rejected ", "ok", 9); err != nil {
		t.Fatalf("expected trimmed rejected human progress to pass, got %v", err)
	}
	if err := svc.validateHumanProgress(ticket.ID, approve.ID, "done", "ok", 9); !errors.Is(err, ErrInvalidProgressOutcome) {
		t.Fatalf("expected ErrInvalidProgressOutcome, got %v", err)
	}
	if err := svc.validateHumanProgress(ticket.ID, action.ID, "done", "", 9); err != nil {
		t.Fatalf("non-human activity should bypass outcome validation, got %v", err)
	}
	if err := svc.validateHumanProgress(ticket.ID, 999999, "approved", "", 9); !errors.Is(err, engine.ErrActivityNotFound) {
		t.Fatalf("missing activity error = %v, want %v", err, engine.ErrActivityNotFound)
	}
	if err := svc.validateHumanProgress(ticket.ID, approve.ID, "done", "", 0); err != nil {
		t.Fatalf("operatorID=0 should bypass validation, got %v", err)
	}
}

func TestTicketService_GetAndBuildResponseContracts(t *testing.T) {
	db := newTestDB(t)
	svc := &TicketService{ticketRepo: &TicketRepo{db: &database.DB{DB: db}}}

	requester := model.User{Username: "requester", IsActive: true}
	if err := db.Create(&requester).Error; err != nil {
		t.Fatalf("create requester: %v", err)
	}
	catalog := ServiceCatalog{Name: "IT", Code: "it"}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	service := ServiceDefinition{Name: "Decisioning Service", Code: "decisioning-service", CatalogID: catalog.ID, EngineType: "smart", IsActive: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	priority := Priority{Name: "P2", Code: "p2", Value: 2, Color: "#0ea5e9", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	ticket := Ticket{
		Code:        "TICK-GET-BUILD",
		Title:       "get build",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusApprovedDecisioning,
		PriorityID:  priority.ID,
		RequesterID: requester.ID,
		Source:      TicketSourceCatalog,
		SLAStatus:   SLAStatusOnTrack,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	got, err := svc.Get(ticket.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != ticket.ID || got.Code != ticket.Code {
		t.Fatalf("unexpected Get result: %+v", got)
	}
	if _, err := svc.Get(999999); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("Get missing ticket error = %v, want %v", err, ErrTicketNotFound)
	}

	resp, err := svc.BuildResponse(got, requester.ID)
	if err != nil {
		t.Fatalf("BuildResponse: %v", err)
	}
	if resp.ServiceName != service.Name || resp.PriorityName != priority.Name || resp.RequesterName != requester.Username {
		t.Fatalf("unexpected BuildResponse projection: %+v", resp)
	}
	if resp.DecisioningReason != engine.TriggerReasonActivityApprove {
		t.Fatalf("decisioning reason = %q, want %q", resp.DecisioningReason, engine.TriggerReasonActivityApprove)
	}
	if resp.DecisionExplanation == nil || resp.DecisionExplanation.Trigger != engine.TriggerReasonActivityApprove {
		t.Fatalf("expected decision explanation derived from response state, got %+v", resp.DecisionExplanation)
	}
}

func TestDecisioningReasonContracts(t *testing.T) {
	if got := decisioningReason(TicketStatusApprovedDecisioning); got != engine.TriggerReasonActivityApprove {
		t.Fatalf("approved decisioning reason = %q", got)
	}
	if got := decisioningReason(TicketStatusRejectedDecisioning); got != engine.TriggerReasonActivityReject {
		t.Fatalf("rejected decisioning reason = %q", got)
	}
	if got := decisioningReason(TicketStatusDecisioning); got != engine.TriggerReasonAIDecision {
		t.Fatalf("ai decisioning reason = %q", got)
	}
	if got := decisioningReason(TicketStatusWaitingHuman); got != "" {
		t.Fatalf("waiting human should not expose decisioning reason, got %q", got)
	}
}

func TestTicketService_SLAContractsAndHelpers(t *testing.T) {
	db := newTestDB(t)
	svc := newSubmissionTicketService(t, db)
	service := testutilSeedSmartTicketService(t, db)

	activeTicket := Ticket{
		Code:                  "TICK-SLA-CONTRACT",
		Title:                 "sla contract",
		ServiceID:             service.ID,
		EngineType:            "smart",
		Status:                TicketStatusWaitingHuman,
		PriorityID:            1,
		RequesterID:           7,
		SLAResponseDeadline:   ptrTime(time.Now().Add(30 * time.Minute)),
		SLAResolutionDeadline: ptrTime(time.Now().Add(2 * time.Hour)),
	}
	if err := db.Create(&activeTicket).Error; err != nil {
		t.Fatalf("create active ticket: %v", err)
	}

	t.Run("pause and resume adjust deadlines and timelines", func(t *testing.T) {
		paused, err := svc.SLAPause(activeTicket.ID, 9)
		if err != nil {
			t.Fatalf("SLAPause: %v", err)
		}
		if paused.SLAPausedAt == nil {
			t.Fatal("expected paused_at to be set")
		}

		backdatedPausedAt := time.Now().Add(-5 * time.Minute)
		if err := db.Model(&Ticket{}).Where("id = ?", activeTicket.ID).Update("sla_paused_at", backdatedPausedAt).Error; err != nil {
			t.Fatalf("backdate paused_at: %v", err)
		}

		originalResponse := *activeTicket.SLAResponseDeadline
		originalResolution := *activeTicket.SLAResolutionDeadline
		resumed, err := svc.SLAResume(activeTicket.ID, 9)
		if err != nil {
			t.Fatalf("SLAResume: %v", err)
		}
		if resumed.SLAPausedAt != nil {
			t.Fatalf("expected paused_at to clear, got %v", resumed.SLAPausedAt)
		}
		if resumed.SLAResponseDeadline == nil || resumed.SLAResolutionDeadline == nil {
			t.Fatalf("expected deadlines to remain populated, got %+v", resumed)
		}
		if resumed.SLAResponseDeadline.Sub(originalResponse) < 4*time.Minute {
			t.Fatalf("response deadline not extended enough: got %v want > %v", resumed.SLAResponseDeadline, originalResponse)
		}
		if resumed.SLAResolutionDeadline.Sub(originalResolution) < 4*time.Minute {
			t.Fatalf("resolution deadline not extended enough: got %v want > %v", resumed.SLAResolutionDeadline, originalResolution)
		}

		var timelineCount int64
		if err := db.Model(&TicketTimeline{}).Where("ticket_id = ? AND event_type IN ?", activeTicket.ID, []string{"sla_paused", "sla_resumed"}).Count(&timelineCount).Error; err != nil {
			t.Fatalf("count sla timelines: %v", err)
		}
		if timelineCount != 2 {
			t.Fatalf("expected pause/resume timelines, got %d", timelineCount)
		}
	})

	t.Run("resume without deadlines still clears paused marker", func(t *testing.T) {
		ticket := Ticket{
			Code:        "TICK-SLA-NIL-DEADLINE",
			Title:       "nil deadline resume",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create nil-deadline ticket: %v", err)
		}
		pausedAt := time.Now().Add(-2 * time.Minute)
		if err := db.Model(&ticket).Update("sla_paused_at", pausedAt).Error; err != nil {
			t.Fatalf("seed paused_at: %v", err)
		}

		resumed, err := svc.SLAResume(ticket.ID, 11)
		if err != nil {
			t.Fatalf("SLAResume nil deadlines: %v", err)
		}
		if resumed.SLAPausedAt != nil || resumed.SLAResponseDeadline != nil || resumed.SLAResolutionDeadline != nil {
			t.Fatalf("unexpected resumed ticket with nil deadlines: %+v", resumed)
		}
	})

	t.Run("pause and resume guards map missing terminal and invalid states", func(t *testing.T) {
		if _, err := svc.SLAPause(999999, 1); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("SLAPause missing ticket error = %v, want %v", err, ErrTicketNotFound)
		}
		if _, err := svc.SLAResume(999999, 1); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("SLAResume missing ticket error = %v, want %v", err, ErrTicketNotFound)
		}

		terminal := Ticket{
			Code:        "TICK-SLA-TERMINAL",
			Title:       "terminal",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusCancelled,
			Outcome:     TicketOutcomeCancelled,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&terminal).Error; err != nil {
			t.Fatalf("create terminal ticket: %v", err)
		}
		if _, err := svc.SLAPause(terminal.ID, 1); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("SLAPause terminal error = %v, want %v", err, ErrTicketTerminal)
		}
		if err := db.Model(&terminal).Update("sla_paused_at", time.Now()).Error; err != nil {
			t.Fatalf("seed terminal paused_at: %v", err)
		}
		if _, err := svc.SLAResume(terminal.ID, 1); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("SLAResume terminal error = %v, want %v", err, ErrTicketTerminal)
		}

		alreadyPaused := Ticket{
			Code:        "TICK-SLA-PAUSED",
			Title:       "already paused",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&alreadyPaused).Error; err != nil {
			t.Fatalf("create already paused ticket: %v", err)
		}
		if err := db.Model(&alreadyPaused).Update("sla_paused_at", time.Now()).Error; err != nil {
			t.Fatalf("seed already paused_at: %v", err)
		}
		if _, err := svc.SLAPause(alreadyPaused.ID, 1); !errors.Is(err, ErrSLAAlreadyPaused) {
			t.Fatalf("SLAPause already paused error = %v, want %v", err, ErrSLAAlreadyPaused)
		}

		notPaused := Ticket{
			Code:        "TICK-SLA-NOT-PAUSED",
			Title:       "not paused",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&notPaused).Error; err != nil {
			t.Fatalf("create not paused ticket: %v", err)
		}
		if _, err := svc.SLAResume(notPaused.ID, 1); !errors.Is(err, ErrSLANotPaused) {
			t.Fatalf("SLAResume not paused error = %v, want %v", err, ErrSLANotPaused)
		}
	})
}

func TestTicketService_HelperContracts(t *testing.T) {
	if got := serviceVersionIDPointer(0); got != nil {
		t.Fatalf("serviceVersionIDPointer(0) = %v, want nil", got)
	}
	if got := serviceVersionIDPointer(42); got == nil || *got != 42 {
		t.Fatalf("serviceVersionIDPointer(42) = %v", got)
	}

	if !isUniqueConstraintError(errors.New("UNIQUE constraint failed: itsm_service_desk_submissions.session_id")) {
		t.Fatal("expected sqlite unique error to be detected")
	}
	if !isUniqueConstraintError(errors.New("duplicate key value violates unique constraint")) {
		t.Fatal("expected postgres duplicate error to be detected")
	}
	if isUniqueConstraintError(errors.New("record not found")) {
		t.Fatal("non-unique error should not be classified as duplicate")
	}

	if !isDecisioningStatus(TicketStatusApprovedDecisioning) || !isDecisioningStatus(TicketStatusRejectedDecisioning) || !isDecisioningStatus(TicketStatusDecisioning) {
		t.Fatal("expected decisioning statuses to be recognized")
	}
	if isDecisioningStatus(TicketStatusWaitingHuman) {
		t.Fatal("waiting_human should not be treated as decisioning")
	}
}

func TestTicketService_FindSubmittedDraftTicketContracts(t *testing.T) {
	db := newTestDB(t)
	svc := newSubmissionTicketService(t, db)
	service := testutilSeedSmartTicketService(t, db)

	if ticket, ok, err := svc.findSubmittedDraftTicket(77, 1, "fields-a", "request-a"); err != nil || ok || ticket != nil {
		t.Fatalf("expected empty lookup, got ticket=%+v ok=%v err=%v", ticket, ok, err)
	}

	if err := db.Create(&ServiceDeskSubmission{
		SessionID:    77,
		DraftVersion: 1,
		FieldsHash:   "fields-a",
		RequestHash:  "request-a",
		Status:       "submitting",
		SubmittedBy:  7,
		SubmittedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("create submitting draft: %v", err)
	}
	if ticket, ok, err := svc.findSubmittedDraftTicket(77, 1, "fields-a", "request-a"); err != nil || ok || ticket != nil {
		t.Fatalf("expected submitting draft to return empty, got ticket=%+v ok=%v err=%v", ticket, ok, err)
	}

	created := Ticket{
		Code:        "TICK-DRAFT-FIND",
		Title:       "draft find",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusDecisioning,
		PriorityID:  1,
		RequesterID: 7,
	}
	if err := db.Create(&created).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Model(&ServiceDeskSubmission{}).
		Where("session_id = ? AND draft_version = ? AND fields_hash = ? AND request_hash = ?", 77, 1, "fields-a", "request-a").
		Updates(map[string]any{"ticket_id": created.ID, "status": "submitted"}).Error; err != nil {
		t.Fatalf("update submission to submitted: %v", err)
	}

	ticket, ok, err := svc.findSubmittedDraftTicket(77, 1, "fields-a", "request-a")
	if err != nil {
		t.Fatalf("findSubmittedDraftTicket submitted: %v", err)
	}
	if !ok || ticket == nil || ticket.ID != created.ID {
		t.Fatalf("expected submitted draft ticket %+v, got ticket=%+v ok=%v", created, ticket, ok)
	}
}

func TestTicketService_RecoveryAndOverrideContracts(t *testing.T) {
	db := newTestDB(t)
	svc := newSubmissionTicketService(t, db)
	service := testutilSeedSmartTicketService(t, db)

	t.Run("submit smart decision task requires configured smart engine", func(t *testing.T) {
		bare := &TicketService{}
		if err := bare.submitSmartDecisionTask(1, nil, engine.TriggerReasonTicketCreated); err == nil || err.Error() != "smart engine is not configured" {
			t.Fatalf("expected smart engine configuration error, got %v", err)
		}
	})

	t.Run("handoff human rejects non-smart ticket and missing operator", func(t *testing.T) {
		manual := Ticket{
			Code:        "TICK-HANDOFF-MANUAL",
			Title:       "manual",
			ServiceID:   service.ID,
			EngineType:  "manual",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&manual).Error; err != nil {
			t.Fatalf("create manual ticket: %v", err)
		}
		if _, err := svc.handoffHuman(manual.ID, "need help", 9); err == nil || err.Error() != "handoff-human is only available for smart engine tickets" {
			t.Fatalf("expected non-smart handoff error, got %v", err)
		}

		smart := Ticket{
			Code:        "TICK-HANDOFF-NO-OP",
			Title:       "smart",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusDecisioning,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&smart).Error; err != nil {
			t.Fatalf("create smart ticket: %v", err)
		}
		if _, err := svc.handoffHuman(smart.ID, "need human", 0); err == nil || err.Error() != "operator is required" {
			t.Fatalf("expected operator required error, got %v", err)
		}
	})

	t.Run("handoff human rejects missing and terminal tickets", func(t *testing.T) {
		if _, err := svc.handoffHuman(999999, "missing", 9); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("missing handoff ticket error = %v, want %v", err, ErrTicketNotFound)
		}

		terminal := Ticket{
			Code:        "TICK-HANDOFF-TERMINAL",
			Title:       "terminal handoff",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusCancelled,
			Outcome:     TicketOutcomeCancelled,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&terminal).Error; err != nil {
			t.Fatalf("create terminal handoff ticket: %v", err)
		}
		if _, err := svc.handoffHuman(terminal.ID, "terminal", 9); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("terminal handoff error = %v, want %v", err, ErrTicketTerminal)
		}
	})

	t.Run("handoff human dedups repeated recovery actions", func(t *testing.T) {
		smart := Ticket{
			Code:        "TICK-HANDOFF-DEDUP",
			Title:       "smart dedup",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusDecisioning,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&smart).Error; err != nil {
			t.Fatalf("create smart dedup ticket: %v", err)
		}
		if _, err := svc.handoffHuman(smart.ID, "first handoff", 9); err != nil {
			t.Fatalf("first handoffHuman: %v", err)
		}
		if _, err := svc.handoffHuman(smart.ID, "second handoff", 9); !errors.Is(err, ErrRecoveryActionTooFrequent) {
			t.Fatalf("expected ErrRecoveryActionTooFrequent, got %v", err)
		}
	})

	t.Run("override reassign updates current assignment ticket and timeline", func(t *testing.T) {
		currentAssignee := uint(11)
		newAssignee := uint(22)
		ticket := Ticket{
			Code:        "TICK-OVERRIDE-REASSIGN",
			Title:       "override reassign",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusDecisioning,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &currentAssignee,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create override ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "人工处理",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityPending,
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create override activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		assignment := TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: activity.ID,
			UserID:     &currentAssignee,
			AssigneeID: &currentAssignee,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}
		if err := db.Create(&assignment).Error; err != nil {
			t.Fatalf("create current assignment: %v", err)
		}

		updated, err := svc.OverrideReassign(ticket.ID, activity.ID, newAssignee, "重新改派", 99)
		if err != nil {
			t.Fatalf("OverrideReassign: %v", err)
		}
		if updated.AssigneeID == nil || *updated.AssigneeID != newAssignee || updated.Status != TicketStatusWaitingHuman || updated.Outcome != "" {
			t.Fatalf("unexpected updated ticket: %+v", updated)
		}

		var refreshed TicketAssignment
		if err := db.First(&refreshed, assignment.ID).Error; err != nil {
			t.Fatalf("reload assignment: %v", err)
		}
		if refreshed.AssigneeID == nil || *refreshed.AssigneeID != newAssignee {
			t.Fatalf("assignment not reassigned: %+v", refreshed)
		}

		var timeline TicketTimeline
		if err := db.Where("ticket_id = ? AND activity_id = ? AND event_type = ?", ticket.ID, activity.ID, "override_reassign").First(&timeline).Error; err != nil {
			t.Fatalf("load override_reassign timeline: %v", err)
		}
		if timeline.OperatorID != 99 {
			t.Fatalf("unexpected override timeline: %+v", timeline)
		}
	})

	t.Run("override reassign guards missing and terminal ticket", func(t *testing.T) {
		if _, err := svc.OverrideReassign(999999, 1, 2, "missing", 9); !errors.Is(err, ErrTicketNotFound) {
			t.Fatalf("missing ticket error = %v, want %v", err, ErrTicketNotFound)
		}

		terminal := Ticket{
			Code:        "TICK-OVERRIDE-TERMINAL",
			Title:       "terminal",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusCancelled,
			Outcome:     TicketOutcomeCancelled,
			PriorityID:  1,
			RequesterID: 7,
		}
		if err := db.Create(&terminal).Error; err != nil {
			t.Fatalf("create terminal override ticket: %v", err)
		}
		if _, err := svc.OverrideReassign(terminal.ID, 1, 2, "terminal", 9); !errors.Is(err, ErrTicketTerminal) {
			t.Fatalf("terminal ticket error = %v, want %v", err, ErrTicketTerminal)
		}
	})

	t.Run("override reassign rejects activity without current assignment and leaves ticket untouched", func(t *testing.T) {
		currentAssignee := uint(41)
		ticket := Ticket{
			Code:        "TICK-OVERRIDE-NO-ASSIGNMENT",
			Title:       "override no assignment",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &currentAssignee,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		activity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "待人工认领",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityPending,
		}
		if err := db.Create(&activity).Error; err != nil {
			t.Fatalf("create activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}

		if _, err := svc.OverrideReassign(ticket.ID, activity.ID, 88, "no current assignment", 99); !errors.Is(err, ErrNoActiveAssignment) {
			t.Fatalf("override reassign error = %v, want %v", err, ErrNoActiveAssignment)
		}

		var refreshed Ticket
		if err := db.First(&refreshed, ticket.ID).Error; err != nil {
			t.Fatalf("reload ticket: %v", err)
		}
		if refreshed.AssigneeID == nil || *refreshed.AssigneeID != currentAssignee {
			t.Fatalf("ticket assignee changed unexpectedly: %+v", refreshed)
		}

		var timelineCount int64
		if err := db.Model(&TicketTimeline{}).Where("ticket_id = ? AND event_type = ?", ticket.ID, "override_reassign").Count(&timelineCount).Error; err != nil {
			t.Fatalf("count override_reassign timelines: %v", err)
		}
		if timelineCount != 0 {
			t.Fatalf("expected no override_reassign timeline, got %d", timelineCount)
		}
	})

	t.Run("override jump without assignee cancels previous work and clears stale assignee", func(t *testing.T) {
		currentAssignee := uint(51)
		ticket := Ticket{
			Code:        "TICK-OVERRIDE-JUMP-CLEAR",
			Title:       "override jump clear assignee",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &currentAssignee,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		oldActivity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "旧人工处理",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityPending,
		}
		if err := db.Create(&oldActivity).Error; err != nil {
			t.Fatalf("create old activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", oldActivity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		oldAssignment := TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: oldActivity.ID,
			UserID:     &currentAssignee,
			AssigneeID: &currentAssignee,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}
		if err := db.Create(&oldAssignment).Error; err != nil {
			t.Fatalf("create old assignment: %v", err)
		}

		updated, err := svc.OverrideJump(ticket.ID, engine.NodeProcess, nil, "转人工重新分配", 99)
		if err != nil {
			t.Fatalf("OverrideJump: %v", err)
		}
		if updated.AssigneeID != nil {
			t.Fatalf("expected stale assignee to be cleared, got %+v", updated)
		}
		if updated.CurrentActivityID == nil || *updated.CurrentActivityID == oldActivity.ID {
			t.Fatalf("expected new current activity, got %+v", updated)
		}
		if updated.Status != TicketStatusWaitingHuman || updated.Outcome != "" {
			t.Fatalf("unexpected updated ticket: %+v", updated)
		}

		var cancelledActivity TicketActivity
		if err := db.First(&cancelledActivity, oldActivity.ID).Error; err != nil {
			t.Fatalf("reload cancelled activity: %v", err)
		}
		if cancelledActivity.Status != engine.ActivityCancelled || cancelledActivity.FinishedAt == nil {
			t.Fatalf("expected old activity cancelled, got %+v", cancelledActivity)
		}

		var cancelledAssignment TicketAssignment
		if err := db.First(&cancelledAssignment, oldAssignment.ID).Error; err != nil {
			t.Fatalf("reload cancelled assignment: %v", err)
		}
		if cancelledAssignment.Status != AssignmentCancelled || cancelledAssignment.IsCurrent {
			t.Fatalf("expected old assignment cancelled and not current, got %+v", cancelledAssignment)
		}

		var newAssignmentCount int64
		if err := db.Model(&TicketAssignment{}).Where("ticket_id = ? AND activity_id = ?", ticket.ID, *updated.CurrentActivityID).Count(&newAssignmentCount).Error; err != nil {
			t.Fatalf("count new assignments: %v", err)
		}
		if newAssignmentCount != 0 {
			t.Fatalf("expected no new assignment for unassigned override jump, got %d", newAssignmentCount)
		}
	})

	t.Run("override jump also cancels claimed_by_other companions", func(t *testing.T) {
		currentAssignee := uint(61)
		competingAssignee := uint(62)
		ticket := Ticket{
			Code:        "TICK-OVERRIDE-JUMP-CLAIMED-OTHER",
			Title:       "override jump claimed by other",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &currentAssignee,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		oldActivity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "旧人工处理",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityInProgress,
		}
		if err := db.Create(&oldActivity).Error; err != nil {
			t.Fatalf("create old activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", oldActivity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		claimedAt := time.Now().Add(-time.Minute)
		mine := TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: oldActivity.ID,
			UserID:     &currentAssignee,
			AssigneeID: &currentAssignee,
			Status:     AssignmentInProgress,
			IsCurrent:  true,
			ClaimedAt:  &claimedAt,
		}
		if err := db.Create(&mine).Error; err != nil {
			t.Fatalf("create claimed assignment: %v", err)
		}
		competing := TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: oldActivity.ID,
			UserID:     &competingAssignee,
			AssigneeID: &competingAssignee,
			Status:     AssignmentClaimedByOther,
			IsCurrent:  true,
		}
		if err := db.Create(&competing).Error; err != nil {
			t.Fatalf("create claimed_by_other assignment: %v", err)
		}

		updated, err := svc.OverrideJump(ticket.ID, engine.NodeProcess, nil, "改派流程", 99)
		if err != nil {
			t.Fatalf("OverrideJump: %v", err)
		}
		if updated.CurrentActivityID == nil || *updated.CurrentActivityID == oldActivity.ID {
			t.Fatalf("expected new current activity, got %+v", updated)
		}

		var refreshedCompeting TicketAssignment
		if err := db.First(&refreshedCompeting, competing.ID).Error; err != nil {
			t.Fatalf("reload competing assignment: %v", err)
		}
		if refreshedCompeting.Status != AssignmentCancelled || refreshedCompeting.IsCurrent {
			t.Fatalf("expected claimed_by_other assignment cancelled and not current, got %+v", refreshedCompeting)
		}
	})

	t.Run("override jump rejects invalid activity type without mutating ticket state", func(t *testing.T) {
		currentAssignee := uint(68)
		ticket := Ticket{
			Code:        "TICK-OVERRIDE-JUMP-INVALID-TYPE",
			Title:       "override jump invalid type",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &currentAssignee,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		oldActivity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "旧人工处理",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityPending,
		}
		if err := db.Create(&oldActivity).Error; err != nil {
			t.Fatalf("create old activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", oldActivity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: oldActivity.ID,
			UserID:     &currentAssignee,
			AssigneeID: &currentAssignee,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create old assignment: %v", err)
		}

		if _, err := svc.OverrideJump(ticket.ID, "bogus_node", nil, "非法跳转", 99); !errors.Is(err, ErrInvalidActivityType) {
			t.Fatalf("OverrideJump invalid activity type error = %v, want %v", err, ErrInvalidActivityType)
		}

		var reloaded Ticket
		if err := db.First(&reloaded, ticket.ID).Error; err != nil {
			t.Fatalf("reload ticket: %v", err)
		}
		if reloaded.CurrentActivityID == nil || *reloaded.CurrentActivityID != oldActivity.ID || reloaded.Status != TicketStatusWaitingHuman {
			t.Fatalf("expected ticket state unchanged, got %+v", reloaded)
		}

		var activityCount int64
		if err := db.Model(&TicketActivity{}).Where("ticket_id = ?", ticket.ID).Count(&activityCount).Error; err != nil {
			t.Fatalf("count activities: %v", err)
		}
		if activityCount != 1 {
			t.Fatalf("expected no new activity for invalid override jump, got %d", activityCount)
		}
	})

	t.Run("override jump with explicit zero assignee treats ticket as unassigned and action node enters executing state", func(t *testing.T) {
		currentAssignee := uint(71)
		ticket := Ticket{
			Code:        "TICK-OVERRIDE-JUMP-ZERO",
			Title:       "override jump zero assignee",
			ServiceID:   service.ID,
			EngineType:  "smart",
			Status:      TicketStatusWaitingHuman,
			PriorityID:  1,
			RequesterID: 7,
			AssigneeID:  &currentAssignee,
		}
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
		oldActivity := TicketActivity{
			TicketID:     ticket.ID,
			Name:         "旧人工处理",
			ActivityType: engine.NodeProcess,
			Status:       engine.ActivityPending,
		}
		if err := db.Create(&oldActivity).Error; err != nil {
			t.Fatalf("create old activity: %v", err)
		}
		if err := db.Model(&ticket).Update("current_activity_id", oldActivity.ID).Error; err != nil {
			t.Fatalf("set current activity: %v", err)
		}
		if err := db.Create(&TicketAssignment{
			TicketID:   ticket.ID,
			ActivityID: oldActivity.ID,
			UserID:     &currentAssignee,
			AssigneeID: &currentAssignee,
			Status:     AssignmentPending,
			IsCurrent:  true,
		}).Error; err != nil {
			t.Fatalf("create old assignment: %v", err)
		}

		explicitZero := uint(0)
		updated, err := svc.OverrideJump(ticket.ID, engine.NodeAction, &explicitZero, "切到自动动作", 99)
		if err != nil {
			t.Fatalf("OverrideJump explicit zero assignee: %v", err)
		}
		if updated.AssigneeID != nil {
			t.Fatalf("expected explicit zero assignee to clear ticket owner, got %+v", updated)
		}
		if updated.Status != TicketStatusExecutingAction {
			t.Fatalf("expected action override to enter executing_action, got %q", updated.Status)
		}
		if updated.CurrentActivityID == nil || *updated.CurrentActivityID == oldActivity.ID {
			t.Fatalf("expected new current activity, got %+v", updated)
		}

		var newActivity TicketActivity
		if err := db.First(&newActivity, *updated.CurrentActivityID).Error; err != nil {
			t.Fatalf("load new activity: %v", err)
		}
		if newActivity.ActivityType != engine.NodeAction || newActivity.Status != engine.ActivityPending {
			t.Fatalf("unexpected new action activity: %+v", newActivity)
		}

		var assignmentCount int64
		if err := db.Model(&TicketAssignment{}).Where("ticket_id = ? AND activity_id = ?", ticket.ID, newActivity.ID).Count(&assignmentCount).Error; err != nil {
			t.Fatalf("count new assignments: %v", err)
		}
		if assignmentCount != 0 {
			t.Fatalf("expected no assignment for explicit zero assignee override, got %d", assignmentCount)
		}
	})
}

// TestAssignPreservesPositionContext is a regression test for the bug in TICK-00109.
// Assign() must NOT clear position_id / department_id so that deterministic service
// guards (e.g. applyDBBackupWhitelistGuard) can still recognise the completed step
// via their INNER JOIN on position_id.
func TestAssignPreservesPositionContext(t *testing.T) {
	db := newTestDB(t)
	svc := newSubmissionTicketService(t, db)
	service := testutilSeedSmartTicketService(t, db)

	positionID := uint(1)
	departmentID := uint(1)
	originalAssignee := uint(10)
	newAssignee := uint(20)

	ticket := Ticket{
		Code:        "TICK-ASSIGN-PRESERVE-POS",
		Title:       "assign preserves position",
		ServiceID:   service.ID,
		EngineType:  "smart",
		Status:      TicketStatusWaitingHuman,
		PriorityID:  1,
		RequesterID: 7,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	activity := TicketActivity{
		TicketID:     ticket.ID,
		Name:         "db_admin 处理",
		ActivityType: engine.NodeProcess,
		Status:       engine.ActivityPending,
	}
	if err := db.Create(&activity).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if err := db.Model(&ticket).Update("current_activity_id", activity.ID).Error; err != nil {
		t.Fatalf("set current_activity_id: %v", err)
	}

	// Simulate initial AI-created assignment: position_department type, db_admin role.
	assignment := TicketAssignment{
		TicketID:        ticket.ID,
		ActivityID:      activity.ID,
		ParticipantType: "position_department",
		PositionID:      &positionID,
		DepartmentID:    &departmentID,
		AssigneeID:      &originalAssignee,
		Status:          AssignmentPending,
		IsCurrent:       true,
	}
	if err := db.Create(&assignment).Error; err != nil {
		t.Fatalf("create assignment: %v", err)
	}

	// Admin invokes Assign() to take over the step.
	if _, err := svc.Assign(ticket.ID, newAssignee, 99); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	var refreshed TicketAssignment
	if err := db.First(&refreshed, assignment.ID).Error; err != nil {
		t.Fatalf("reload assignment: %v", err)
	}

	// participant_type must be "user" and assignee must be the new user.
	if refreshed.ParticipantType != "user" {
		t.Errorf("participant_type = %q, want \"user\"", refreshed.ParticipantType)
	}
	if refreshed.AssigneeID == nil || *refreshed.AssigneeID != newAssignee {
		t.Errorf("assignee_id = %v, want %d", refreshed.AssigneeID, newAssignee)
	}
	if refreshed.UserID == nil || *refreshed.UserID != newAssignee {
		t.Errorf("user_id = %v, want %d", refreshed.UserID, newAssignee)
	}

	// position_id and department_id MUST be preserved so guards can still
	// identify the completed step by role.
	if refreshed.PositionID == nil || *refreshed.PositionID != positionID {
		t.Errorf("position_id = %v, want %d — Assign must not clear position_id", refreshed.PositionID, positionID)
	}
	if refreshed.DepartmentID == nil || *refreshed.DepartmentID != departmentID {
		t.Errorf("department_id = %v, want %d — Assign must not clear department_id", refreshed.DepartmentID, departmentID)
	}
}

func TestTicketService_SignalRejectsActivityFromAnotherTicket(t *testing.T) {
	db := newTestDB(t)
	svc := newSubmissionTicketService(t, db)
	service := testutilSeedSmartTicketService(t, db)

	ticketA := Ticket{
		Code:        "TICK-SIGNAL-A",
		Title:       "signal a",
		ServiceID:   service.ID,
		EngineType:  "classic",
		Status:      TicketStatusWaitingHuman,
		PriorityID:  1,
		RequesterID: 7,
	}
	if err := db.Create(&ticketA).Error; err != nil {
		t.Fatalf("create ticket A: %v", err)
	}

	ticketB := Ticket{
		Code:        "TICK-SIGNAL-B",
		Title:       "signal b",
		ServiceID:   service.ID,
		EngineType:  "classic",
		Status:      TicketStatusWaitingHuman,
		PriorityID:  1,
		RequesterID: 7,
	}
	if err := db.Create(&ticketB).Error; err != nil {
		t.Fatalf("create ticket B: %v", err)
	}

	waitActivity := TicketActivity{
		TicketID:     ticketB.ID,
		Name:         "wait-external",
		ActivityType: engine.NodeWait,
		Status:       engine.ActivityPending,
	}
	if err := db.Create(&waitActivity).Error; err != nil {
		t.Fatalf("create wait activity: %v", err)
	}

	if _, err := svc.Signal(ticketA.ID, waitActivity.ID, "done", nil, 11); !errors.Is(err, engine.ErrActivityNotFound) {
		t.Fatalf("cross-ticket signal error = %v, want %v", err, engine.ErrActivityNotFound)
	}
}

func TestTicketService_DecisionExplanationParsingAndOwnerFallbackContracts(t *testing.T) {
	if got := parseDecisionExplanationDetail(nil); got != nil {
		t.Fatalf("expected nil snapshot for empty details, got %+v", got)
	}
	if got := parseDecisionExplanationDetail(JSONField(`{"decision_explanation":`)); got != nil {
		t.Fatalf("expected nil snapshot for invalid json, got %+v", got)
	}

	positionID := uint(7)
	departmentID := uint(9)
	for _, tc := range []struct {
		name string
		in   ticketAssignmentDisplay
		want string
	}{
		{name: "position only", in: ticketAssignmentDisplay{PositionName: "值班岗"}, want: "值班岗"},
		{name: "department only", in: ticketAssignmentDisplay{DepartmentName: "网络部"}, want: "网络部"},
		{name: "position and department ids", in: ticketAssignmentDisplay{PositionID: &positionID, DepartmentID: &departmentID}, want: "部门 #9 / 岗位 #7"},
		{name: "position id only", in: ticketAssignmentDisplay{PositionID: &positionID}, want: "岗位 #7"},
		{name: "department id only", in: ticketAssignmentDisplay{DepartmentID: &departmentID}, want: "部门 #9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := assignmentOwnerFallback(tc.in); got != tc.want {
				t.Fatalf("assignmentOwnerFallback(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func testutilSeedSmartTicketService(t *testing.T, db *gorm.DB) ServiceDefinition {
	t.Helper()
	catalog := ServiceCatalog{Name: "IT", Code: "it-validate", IsActive: true}
	if err := db.Create(&catalog).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}
	priority := Priority{Name: "P3", Code: "p3", Value: 3, Color: "#64748b", IsActive: true}
	if err := db.Create(&priority).Error; err != nil {
		t.Fatalf("create priority: %v", err)
	}
	_ = priority
	service := ServiceDefinition{Name: "Validate Service", Code: "validate-service", CatalogID: catalog.ID, EngineType: "smart", IsActive: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service
}

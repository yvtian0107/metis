package engine

import "testing"

func TestNewActivityDecidedEvent_MapsTriggerReasonByOutcome(t *testing.T) {
	approved := NewActivityDecidedEvent(11, 22, ActivityApproved, 7)
	if approved.EventType != DecisionEventActivityDecided || approved.TriggerReason != TriggerReasonActivityApprove {
		t.Fatalf("expected approved trigger reason, got %+v", approved)
	}
	if approved.CompletedActivityID == nil || *approved.CompletedActivityID != 22 {
		t.Fatalf("expected completed activity id, got %+v", approved.CompletedActivityID)
	}

	rejected := NewActivityDecidedEvent(11, 23, ActivityRejected, 8)
	if rejected.TriggerReason != TriggerReasonActivityReject {
		t.Fatalf("expected rejected trigger reason, got %+v", rejected)
	}
}

func TestNewActionFinishedEvent_UsesActionTrigger(t *testing.T) {
	event := NewActionFinishedEvent(11, 24, "success")
	if event.EventType != DecisionEventActionFinished || event.TriggerReason != TriggerReasonActionCompleted {
		t.Fatalf("unexpected action event: %+v", event)
	}
	if event.CompletedActivityID == nil || *event.CompletedActivityID != 24 {
		t.Fatalf("expected completed activity id set, got %+v", event.CompletedActivityID)
	}
}

func TestNewDecisionFailedEvent_UsesFailureTrigger(t *testing.T) {
	event := NewDecisionFailedEvent(11, "timeout")
	if event.EventType != DecisionEventDecisionFailed || event.TriggerReason != TriggerReasonDecisionFailed {
		t.Fatalf("unexpected decision failed event: %+v", event)
	}
	if event.Reason != "timeout" {
		t.Fatalf("expected reason timeout, got %q", event.Reason)
	}
}

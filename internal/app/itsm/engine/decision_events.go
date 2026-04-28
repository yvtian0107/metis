package engine

import "time"

const (
	DecisionEventActivityDecided = "ticket.activity.decided"
	DecisionEventActionFinished  = "ticket.action.finished"
	DecisionEventDecisionFailed  = "ticket.decision.failed"
)

const (
	TriggerReasonInitialDecision = "initial_decision"
	TriggerReasonTicketCreated   = "ticket_created"
	TriggerReasonActivityDone    = "activity_completed"
	TriggerReasonActivityApprove = "activity_approved"
	TriggerReasonActivityReject  = "activity_rejected"
	TriggerReasonActionCompleted = "action_completed"
	TriggerReasonManualRetry     = "manual_retry"
	TriggerReasonRecovery        = "recovery"
	TriggerReasonDecisionFailed  = "decision_failed"
	TriggerReasonAIDecision      = "ai_decision"
)

// DecisionDomainEvent is the shared payload contract for decision-related events.
// It is intentionally lightweight and stable so dispatcher/recovery paths can
// evolve independently without changing event semantics.
type DecisionDomainEvent struct {
	EventType           string    `json:"event_type"`
	TicketID            uint      `json:"ticket_id"`
	CompletedActivityID *uint     `json:"completed_activity_id,omitempty"`
	Outcome             string    `json:"outcome,omitempty"`
	Reason              string    `json:"reason,omitempty"`
	OperatorID          uint      `json:"operator_id,omitempty"`
	TriggerReason       string    `json:"trigger_reason"`
	OccurredAt          time.Time `json:"occurred_at"`
}

func NewActivityDecidedEvent(ticketID uint, activityID uint, outcome string, operatorID uint) DecisionDomainEvent {
	trigger := TriggerReasonActivityDone
	switch outcome {
	case ActivityApproved:
		trigger = TriggerReasonActivityApprove
	case ActivityRejected:
		trigger = TriggerReasonActivityReject
	}
	return DecisionDomainEvent{
		EventType:           DecisionEventActivityDecided,
		TicketID:            ticketID,
		CompletedActivityID: uintPtrIf(activityID),
		Outcome:             outcome,
		OperatorID:          operatorID,
		TriggerReason:       trigger,
		OccurredAt:          time.Now(),
	}
}

func NewActionFinishedEvent(ticketID uint, activityID uint, outcome string) DecisionDomainEvent {
	return DecisionDomainEvent{
		EventType:           DecisionEventActionFinished,
		TicketID:            ticketID,
		CompletedActivityID: uintPtrIf(activityID),
		Outcome:             outcome,
		TriggerReason:       TriggerReasonActionCompleted,
		OccurredAt:          time.Now(),
	}
}

func NewDecisionFailedEvent(ticketID uint, reason string) DecisionDomainEvent {
	return DecisionDomainEvent{
		EventType:     DecisionEventDecisionFailed,
		TicketID:      ticketID,
		Reason:        reason,
		TriggerReason: TriggerReasonDecisionFailed,
		OccurredAt:    time.Now(),
	}
}

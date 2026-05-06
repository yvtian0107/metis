package contract

type TicketStatus string
type TicketOutcome string
type ActivityStatus string
type EngineType string
type NodeType string
type ServiceDeskStage string
type SurfaceType string

const (
	TicketStatusSubmitted           TicketStatus = "submitted"
	TicketStatusWaitingHuman        TicketStatus = "waiting_human"
	TicketStatusApprovedDecisioning TicketStatus = "approved_decisioning"
	TicketStatusRejectedDecisioning TicketStatus = "rejected_decisioning"
	TicketStatusDecisioning         TicketStatus = "decisioning"
	TicketStatusExecutingAction     TicketStatus = "executing_action"
	TicketStatusCompleted           TicketStatus = "completed"
	TicketStatusRejected            TicketStatus = "rejected"
	TicketStatusWithdrawn           TicketStatus = "withdrawn"
	TicketStatusCancelled           TicketStatus = "cancelled"
	TicketStatusFailed              TicketStatus = "failed"
)

const (
	TicketOutcomeApproved  TicketOutcome = "approved"
	TicketOutcomeRejected  TicketOutcome = "rejected"
	TicketOutcomeFulfilled TicketOutcome = "fulfilled"
	TicketOutcomeWithdrawn TicketOutcome = "withdrawn"
	TicketOutcomeCancelled TicketOutcome = "cancelled"
	TicketOutcomeFailed    TicketOutcome = "failed"
)

const (
	ActivityStatusPending        ActivityStatus = "pending"
	ActivityStatusInProgress     ActivityStatus = "in_progress"
	ActivityStatusApproved       ActivityStatus = "approved"
	ActivityStatusRejected       ActivityStatus = "rejected"
	ActivityStatusTransferred    ActivityStatus = "transferred"
	ActivityStatusDelegated      ActivityStatus = "delegated"
	ActivityStatusClaimedByOther ActivityStatus = "claimed_by_other"
	ActivityStatusCompleted      ActivityStatus = "completed"
	ActivityStatusCancelled      ActivityStatus = "cancelled"
	ActivityStatusFailed         ActivityStatus = "failed"
)

const (
	EngineTypeClassic EngineType = "classic"
	EngineTypeSmart   EngineType = "smart"
)

const (
	NodeTypeStart      NodeType = "start"
	NodeTypeEnd        NodeType = "end"
	NodeTypeForm       NodeType = "form"
	NodeTypeApprove    NodeType = "approve"
	NodeTypeProcess    NodeType = "process"
	NodeTypeAction     NodeType = "action"
	NodeTypeNotify     NodeType = "notify"
	NodeTypeWait       NodeType = "wait"
	NodeTypeExclusive  NodeType = "exclusive"
	NodeTypeParallel   NodeType = "parallel"
	NodeTypeInclusive  NodeType = "inclusive"
	NodeTypeScript     NodeType = "script"
	NodeTypeSubprocess NodeType = "subprocess"
	NodeTypeTimer      NodeType = "timer"
	NodeTypeSignal     NodeType = "signal"
	NodeTypeBTimer     NodeType = "b_timer"
	NodeTypeBError     NodeType = "b_error"
)

func (n NodeType) IsExecutable() bool {
	switch n {
	case NodeTypeTimer, NodeTypeSignal:
		return false
	default:
		return true
	}
}

const (
	ServiceDeskStageIdle                 ServiceDeskStage = "idle"
	ServiceDeskStageCandidatesReady      ServiceDeskStage = "candidates_ready"
	ServiceDeskStageServiceSelected      ServiceDeskStage = "service_selected"
	ServiceDeskStageServiceLoaded        ServiceDeskStage = "service_loaded"
	ServiceDeskStageAwaitingConfirmation ServiceDeskStage = "awaiting_confirmation"
	ServiceDeskStageConfirmed            ServiceDeskStage = "confirmed"
	ServiceDeskStageSubmitted            ServiceDeskStage = "submitted"
)

const (
	SurfaceTypeITSMDraftForm SurfaceType = "itsm.draft_form"
)

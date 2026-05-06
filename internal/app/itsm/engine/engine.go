package engine

import (
	"context"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"metis/internal/app/itsm/contract"
)

// WorkflowEngine defines the contract for workflow execution engines.
// ClassicEngine (BPMN graph traversal) implements this in Phase 2.
// SmartEngine (Agent-driven) will implement this in Phase 3.
type WorkflowEngine interface {
	// Start initialises the workflow for a ticket. It parses the workflow definition,
	// finds the start node, and creates the first Activity on the target node.
	Start(ctx context.Context, tx *gorm.DB, params StartParams) error

	// Progress advances the workflow. It completes the current Activity with the
	// given outcome and creates the next Activity based on outgoing edges.
	Progress(ctx context.Context, tx *gorm.DB, params ProgressParams) error

	// Cancel terminates all active Activities and marks the ticket as cancelled.
	Cancel(ctx context.Context, tx *gorm.DB, params CancelParams) error
}

type StartParams struct {
	TicketID        uint
	WorkflowJSON    json.RawMessage
	RequesterID     uint
	StartFormSchema string // form schema JSON for variable binding (optional)
	StartFormData   string // form data JSON for variable binding (optional)
}

type ProgressParams struct {
	TicketID              uint
	ActivityID            uint
	Outcome               string
	Result                json.RawMessage // form data or processing result
	Opinion               string          // human approval / rejection opinion
	OperatorID            uint
	OperatorPositionIDs   []uint
	OperatorDepartmentIDs []uint
	OperatorOrgScopeReady bool
}

type CancelParams struct {
	TicketID   uint
	Reason     string
	OperatorID uint
	EventType  string // override timeline event_type (default "ticket_cancelled")
	Message    string // override timeline message (default "工单已取消[: reason]")
}

// Errors
var (
	ErrNoStartNode             = errors.New("workflow: no start node found")
	ErrMultipleStartNodes      = errors.New("workflow: multiple start nodes found")
	ErrNoEndNode               = errors.New("workflow: no end node found")
	ErrNoOutgoingEdge          = errors.New("workflow: no matching outgoing edge for outcome")
	ErrMaxDepthExceeded        = errors.New("workflow: automatic step depth exceeded maximum (50)")
	ErrInvalidNodeType         = errors.New("workflow: invalid node type")
	ErrActivityNotFound        = errors.New("workflow: activity not found")
	ErrActivityNotActive       = errors.New("workflow: activity is not in an active state")
	ErrNoActiveAssignment      = errors.New("workflow: no active pending assignment for this activity")
	ErrNodeNotFound            = errors.New("workflow: referenced node not found in workflow")
	ErrTokenNotFound           = errors.New("workflow: execution token not found")
	ErrTokenNotActive          = errors.New("workflow: execution token is not in active state")
	ErrNodeNotImplemented      = errors.New("workflow: node type registered but execution logic not yet implemented")
	ErrGatewayNoOutEdge        = errors.New("workflow: gateway node has no outgoing edges")
	ErrGatewayJoinIncomplete   = errors.New("workflow: not all sibling tokens have completed at join")
	ErrGatewayMissingDirection = errors.New("workflow: parallel/inclusive node missing gateway_direction (fork or join)")
)

// Node types
const (
	NodeStart   = string(contract.NodeTypeStart)
	NodeEnd     = string(contract.NodeTypeEnd)
	NodeForm    = string(contract.NodeTypeForm)
	NodeApprove = string(contract.NodeTypeApprove)
	NodeProcess = string(contract.NodeTypeProcess)
	NodeAction  = string(contract.NodeTypeAction)
	NodeNotify  = string(contract.NodeTypeNotify)
	NodeWait    = string(contract.NodeTypeWait)

	// Gateway types (③ itsm-execution-tokens: exclusive implemented; ④: parallel/inclusive)
	NodeExclusive = string(contract.NodeTypeExclusive)
	NodeParallel  = string(contract.NodeTypeParallel)  // registered only — execution logic in ④ itsm-gateway-parallel
	NodeInclusive = string(contract.NodeTypeInclusive) // registered only — execution logic in ④ itsm-gateway-parallel

	// Advanced node types — registered only, execution logic in ⑤ itsm-advanced-nodes
	NodeScript     = string(contract.NodeTypeScript)
	NodeSubprocess = string(contract.NodeTypeSubprocess)
	NodeTimer      = string(contract.NodeTypeTimer)  // intermediate timer event
	NodeSignal     = string(contract.NodeTypeSignal) // intermediate signal event
	NodeBTimer     = string(contract.NodeTypeBTimer) // boundary timer event
	NodeBError     = string(contract.NodeTypeBError) // boundary error event
)

var ValidNodeTypes = map[string]bool{
	NodeStart: true, NodeEnd: true, NodeForm: true,
	NodeApprove: true, NodeProcess: true, NodeAction: true,
	NodeExclusive: true, NodeParallel: true, NodeInclusive: true,
	NodeNotify: true, NodeWait: true,
	NodeScript: true, NodeSubprocess: true,
	NodeTimer: true, NodeSignal: true,
	NodeBTimer: true, NodeBError: true,
}

// UnimplementedNodeTypes lists node types that are registered but not yet executable.
var UnimplementedNodeTypes = map[string]bool{
	NodeTimer: true, NodeSignal: true,
}

// IsAutoNode returns true for node types that execute automatically without human intervention.
func IsAutoNode(nodeType string) bool {
	return nodeType == NodeExclusive || nodeType == NodeAction || nodeType == NodeNotify || nodeType == NodeScript
}

// IsHumanNode returns true for node types that require human interaction.
func IsHumanNode(nodeType string) bool {
	return nodeType == NodeForm || nodeType == NodeApprove || nodeType == NodeProcess || nodeType == NodeWait
}

// Token status constants
const (
	TokenActive    = "active"
	TokenWaiting   = "waiting" // fork: parent waits for children — ④ itsm-gateway-parallel
	TokenCompleted = "completed"
	TokenCancelled = "cancelled"
	TokenSuspended = "suspended" // reserved for ⑤ itsm-advanced-nodes (boundary event suspend/resume)
)

// Token type constants
const (
	TokenMain          = "main"           // root token, one per ticket
	TokenParallel      = "parallel"       // parallel gateway fork — ④ itsm-gateway-parallel
	TokenSubprocess    = "subprocess"     // subprocess token — ⑤ itsm-advanced-nodes
	TokenMultiInstance = "multi_instance" // multi-instance token — ⑤ itsm-advanced-nodes
	TokenBoundary      = "boundary"       // boundary event token — ⑤ itsm-advanced-nodes
)

// MaxAutoDepth limits recursive automatic node processing to prevent infinite loops.
const MaxAutoDepth = 50

// Gateway direction constants (④ itsm-gateway-parallel)
const (
	GatewayFork = "fork"
	GatewayJoin = "join"
)

// Activity status constants
const (
	ActivityPending    = string(contract.ActivityStatusPending)
	ActivityInProgress = string(contract.ActivityStatusInProgress)
	ActivityApproved   = string(contract.ActivityStatusApproved)
	ActivityRejected   = string(contract.ActivityStatusRejected)
	ActivityCompleted  = string(contract.ActivityStatusCompleted)
	ActivityCancelled  = string(contract.ActivityStatusCancelled)
)

const (
	TicketStatusSubmitted           = string(contract.TicketStatusSubmitted)
	TicketStatusWaitingHuman        = string(contract.TicketStatusWaitingHuman)
	TicketStatusApprovedDecisioning = string(contract.TicketStatusApprovedDecisioning)
	TicketStatusRejectedDecisioning = string(contract.TicketStatusRejectedDecisioning)
	TicketStatusDecisioning         = string(contract.TicketStatusDecisioning)
	TicketStatusExecutingAction     = string(contract.TicketStatusExecutingAction)
	TicketStatusCompleted           = string(contract.TicketStatusCompleted)
	TicketStatusRejected            = string(contract.TicketStatusRejected)
	TicketStatusWithdrawn           = string(contract.TicketStatusWithdrawn)
	TicketStatusCancelled           = string(contract.TicketStatusCancelled)
	TicketStatusFailed              = string(contract.TicketStatusFailed)
)

const (
	TicketOutcomeApproved  = string(contract.TicketOutcomeApproved)
	TicketOutcomeRejected  = string(contract.TicketOutcomeRejected)
	TicketOutcomeFulfilled = string(contract.TicketOutcomeFulfilled)
	TicketOutcomeWithdrawn = string(contract.TicketOutcomeWithdrawn)
	TicketOutcomeCancelled = string(contract.TicketOutcomeCancelled)
	TicketOutcomeFailed    = string(contract.TicketOutcomeFailed)
)

func IsTerminalTicketStatus(status string) bool {
	switch status {
	case TicketStatusCompleted, TicketStatusRejected, TicketStatusWithdrawn, TicketStatusCancelled, TicketStatusFailed:
		return true
	default:
		return false
	}
}

func HumanActivityResultStatus(outcome string) string {
	switch outcome {
	case ActivityRejected:
		return ActivityRejected
	default:
		return ActivityApproved
	}
}

func TicketDecisioningStatusForOutcome(outcome string) string {
	if outcome == ActivityRejected {
		return TicketStatusRejectedDecisioning
	}
	if outcome == ActivityApproved {
		return TicketStatusApprovedDecisioning
	}
	return TicketStatusDecisioning
}

func humanOrCompletedActivityStatus(activityType string, outcome string) string {
	if IsHumanNode(activityType) {
		return HumanActivityResultStatus(outcome)
	}
	return ActivityCompleted
}

func ticketCancelStatus(eventType string) string {
	if eventType == "withdrawn" {
		return TicketStatusWithdrawn
	}
	return TicketStatusCancelled
}

func ticketCancelOutcome(eventType string) string {
	if eventType == "withdrawn" {
		return TicketOutcomeWithdrawn
	}
	return TicketOutcomeCancelled
}

func ticketStatusForDecisionActivity(activityType string) string {
	switch activityType {
	case NodeAction, NodeNotify, NodeScript:
		return TicketStatusExecutingAction
	case NodeApprove, NodeForm, NodeProcess, NodeWait:
		return TicketStatusWaitingHuman
	default:
		return TicketStatusDecisioning
	}
}

func CompletedActivityStatuses() []string {
	return []string{ActivityCompleted, ActivityApproved, ActivityRejected, ActivityCancelled}
}

func IsCompletedActivityStatus(status string) bool {
	switch status {
	case ActivityCompleted, ActivityApproved, ActivityRejected, ActivityCancelled:
		return true
	default:
		return false
	}
}

// Smart engine errors
var (
	ErrSmartEngineUnavailable = errors.New("智能引擎不可用：AI 模块未安装")
	ErrAIDecisionFailed       = errors.New("AI 决策失败")
	ErrAIDecisionTimeout      = errors.New("AI 决策超时")
	ErrAIDisabled             = errors.New("AI 决策已停用（连续失败次数过多）")
	ErrInvalidDecisionPlan    = errors.New("AI 决策计划校验失败")
)

// Smart engine defaults
const (
	DefaultConfidenceThreshold    = 0.8
	DefaultDecisionTimeoutSeconds = 60
	MaxAIFailureCount             = 3
	DecisionToolMaxTurns          = 8
)

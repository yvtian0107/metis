package domain

import (
	"time"

	"metis/internal/app/itsm/contract"
	"metis/internal/model"
)

// Ticket status constants
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

// Ticket outcome constants
const (
	TicketOutcomeApproved  = string(contract.TicketOutcomeApproved)
	TicketOutcomeRejected  = string(contract.TicketOutcomeRejected)
	TicketOutcomeFulfilled = string(contract.TicketOutcomeFulfilled)
	TicketOutcomeWithdrawn = string(contract.TicketOutcomeWithdrawn)
	TicketOutcomeCancelled = string(contract.TicketOutcomeCancelled)
	TicketOutcomeFailed    = string(contract.TicketOutcomeFailed)
)

// Ticket source constants
const (
	TicketSourceCatalog = "catalog"
	TicketSourceAgent   = "agent"
)

// SLA status constants
const (
	SLAStatusOnTrack          = "on_track"
	SLAStatusBreachedResponse = "breached_response"
	SLAStatusBreachedResolve  = "breached_resolution"
)

// Assignment status constants
const (
	AssignmentPending        = string(contract.ActivityStatusPending)
	AssignmentInProgress     = string(contract.ActivityStatusInProgress)
	AssignmentApproved       = string(contract.ActivityStatusApproved)
	AssignmentRejected       = string(contract.ActivityStatusRejected)
	AssignmentTransferred    = string(contract.ActivityStatusTransferred)
	AssignmentDelegated      = string(contract.ActivityStatusDelegated)
	AssignmentClaimedByOther = string(contract.ActivityStatusClaimedByOther)
	AssignmentCancelled      = string(contract.ActivityStatusCancelled)
	AssignmentFailed         = string(contract.ActivityStatusFailed)
)

// Ticket 工单
type Ticket struct {
	model.BaseModel
	Code                  string     `json:"code" gorm:"size:32;uniqueIndex;not null"`
	Title                 string     `json:"title" gorm:"size:256;not null"`
	Description           string     `json:"description" gorm:"type:text"`
	ServiceID             uint       `json:"serviceId" gorm:"not null;index"`
	ServiceVersionID      *uint      `json:"serviceVersionId" gorm:"index"`
	EngineType            string     `json:"engineType" gorm:"size:16;not null"`
	Status                string     `json:"status" gorm:"size:32;not null;default:submitted;index"`
	Outcome               string     `json:"outcome" gorm:"size:32;index"`
	PriorityID            uint       `json:"priorityId" gorm:"not null;index"`
	RequesterID           uint       `json:"requesterId" gorm:"not null;index"`
	AssigneeID            *uint      `json:"assigneeId" gorm:"index"`
	CurrentActivityID     *uint      `json:"currentActivityId" gorm:"index"`
	Source                string     `json:"source" gorm:"size:16;not null;default:catalog"` // catalog | agent
	AgentSessionID        *uint      `json:"agentSessionId" gorm:"index"`
	AIFailureCount        int        `json:"aiFailureCount" gorm:"default:0"` // smart engine: consecutive AI decision failure count
	FormData              JSONField  `json:"formData" gorm:"type:text"`
	WorkflowJSON          JSONField  `json:"workflowJson" gorm:"type:text"` // snapshot of workflow at creation
	SLAResponseDeadline   *time.Time `json:"slaResponseDeadline"`
	SLAResolutionDeadline *time.Time `json:"slaResolutionDeadline"`
	SLAStatus             string     `json:"slaStatus" gorm:"size:32;default:on_track"`
	SLAPausedAt           *time.Time `json:"slaPausedAt"`
	FinishedAt            *time.Time `json:"finishedAt"`
}

func (Ticket) TableName() string { return "itsm_tickets" }

type TicketResponse struct {
	ID                    uint                 `json:"id"`
	Code                  string               `json:"code"`
	Title                 string               `json:"title"`
	Description           string               `json:"description"`
	ServiceID             uint                 `json:"serviceId"`
	ServiceVersionID      *uint                `json:"serviceVersionId"`
	ServiceName           string               `json:"serviceName"`
	IntakeFormSchema      JSONField            `json:"intakeFormSchema,omitempty"`
	EngineType            string               `json:"engineType"`
	Status                string               `json:"status"`
	Outcome               string               `json:"outcome"`
	StatusLabel           string               `json:"statusLabel"`
	StatusTone            string               `json:"statusTone"`
	LastHumanOutcome      string               `json:"lastHumanOutcome"`
	DecisioningReason     string               `json:"decisioningReason"`
	PriorityID            uint                 `json:"priorityId"`
	PriorityName          string               `json:"priorityName"`
	PriorityColor         string               `json:"priorityColor"`
	RequesterID           uint                 `json:"requesterId"`
	RequesterName         string               `json:"requesterName"`
	AssigneeID            *uint                `json:"assigneeId"`
	AssigneeName          string               `json:"assigneeName"`
	CurrentActivityID     *uint                `json:"currentActivityId"`
	Source                string               `json:"source"`
	AgentSessionID        *uint                `json:"agentSessionId"`
	AIFailureCount        int                  `json:"aiFailureCount"`
	FormData              JSONField            `json:"formData"`
	WorkflowJSON          JSONField            `json:"workflowJson"`
	SLAResponseDeadline   *time.Time           `json:"slaResponseDeadline"`
	SLAResolutionDeadline *time.Time           `json:"slaResolutionDeadline"`
	SLAStatus             string               `json:"slaStatus"`
	SLAPausedAt           *time.Time           `json:"slaPausedAt"`
	FinishedAt            *time.Time           `json:"finishedAt"`
	SmartState            string               `json:"smartState,omitempty"`
	CurrentOwnerType      string               `json:"currentOwnerType,omitempty"`
	CurrentOwnerName      string               `json:"currentOwnerName,omitempty"`
	NextStepSummary       string               `json:"nextStepSummary,omitempty"`
	CanAct                bool                 `json:"canAct"`
	CanOverride           bool                 `json:"canOverride"`
	DecisionExplanation   *DecisionExplanation `json:"decisionExplanation,omitempty"`
	RecoveryActions       []RecoveryAction     `json:"recoveryActions,omitempty"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

type DecisionExplanation struct {
	ActivityID    *uint  `json:"activityId,omitempty"`
	Basis         string `json:"basis"`
	Trigger       string `json:"trigger"`
	Decision      string `json:"decision"`
	NextStep      string `json:"nextStep"`
	HumanOverride string `json:"humanOverride"`
}

type RecoveryAction struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type TicketMonitorSummary struct {
	ActiveTotal         int `json:"activeTotal"`
	StuckTotal          int `json:"stuckTotal"`
	RiskTotal           int `json:"riskTotal"`
	SLARiskTotal        int `json:"slaRiskTotal"`
	AIIncidentTotal     int `json:"aiIncidentTotal"`
	CompletedTodayTotal int `json:"completedTodayTotal"`
	SmartActiveTotal    int `json:"smartActiveTotal"`
	ClassicActiveTotal  int `json:"classicActiveTotal"`
}

type TicketMonitorReason struct {
	MetricCode string         `json:"metricCode"`
	RuleCode   string         `json:"ruleCode"`
	Severity   string         `json:"severity"`
	Message    string         `json:"message"`
	Evidence   map[string]any `json:"evidence"`
}

type TicketMonitorItem struct {
	TicketResponse
	RiskLevel                string                `json:"riskLevel"`
	Stuck                    bool                  `json:"stuck"`
	StuckReasons             []string              `json:"stuckReasons"`
	MonitorReasons           []TicketMonitorReason `json:"monitorReasons"`
	WaitingMinutes           int                   `json:"waitingMinutes"`
	CurrentActivityName      string                `json:"currentActivityName"`
	CurrentActivityType      string                `json:"currentActivityType"`
	CurrentActivityStartedAt *time.Time            `json:"currentActivityStartedAt"`
}

type TicketMonitorResponse struct {
	Summary TicketMonitorSummary `json:"summary"`
	Items   []TicketMonitorItem  `json:"items"`
	Total   int64                `json:"total"`
}

const DecisionQualityMetricVersion = "2026-04-27.v1"

type DecisionQualityItem struct {
	DimensionType             string  `json:"dimensionType"`
	DimensionID               uint    `json:"dimensionId"`
	DimensionName             string  `json:"dimensionName"`
	ApprovalRate              float64 `json:"approvalRate"`
	RejectionRate             float64 `json:"rejectionRate"`
	RetryRate                 float64 `json:"retryRate"`
	AvgDecisionLatencySeconds float64 `json:"avgDecisionLatencySeconds"`
	RecoverySuccessRate       float64 `json:"recoverySuccessRate"`
	DecisionCount             int64   `json:"decisionCount"`
}

type DecisionQualityResponse struct {
	Version     string                `json:"version"`
	WindowDays  int                   `json:"windowDays"`
	GeneratedAt time.Time             `json:"generatedAt"`
	Items       []DecisionQualityItem `json:"items"`
}

func (t *Ticket) ToResponse() TicketResponse {
	return TicketResponse{
		ID:                    t.ID,
		Code:                  t.Code,
		Title:                 t.Title,
		Description:           t.Description,
		ServiceID:             t.ServiceID,
		ServiceVersionID:      t.ServiceVersionID,
		EngineType:            t.EngineType,
		Status:                t.Status,
		Outcome:               t.Outcome,
		StatusLabel:           TicketStatusLabel(t.Status, t.Outcome),
		StatusTone:            TicketStatusTone(t.Status, t.Outcome),
		PriorityID:            t.PriorityID,
		RequesterID:           t.RequesterID,
		AssigneeID:            t.AssigneeID,
		CurrentActivityID:     t.CurrentActivityID,
		Source:                t.Source,
		AgentSessionID:        t.AgentSessionID,
		AIFailureCount:        t.AIFailureCount,
		FormData:              t.FormData,
		WorkflowJSON:          t.WorkflowJSON,
		SLAResponseDeadline:   t.SLAResponseDeadline,
		SLAResolutionDeadline: t.SLAResolutionDeadline,
		SLAStatus:             t.SLAStatus,
		SLAPausedAt:           t.SLAPausedAt,
		FinishedAt:            t.FinishedAt,
		CreatedAt:             t.CreatedAt,
		UpdatedAt:             t.UpdatedAt,
	}
}

// IsTerminal returns true if the ticket is in a terminal state.
func (t *Ticket) IsTerminal() bool {
	return IsTerminalTicketStatus(t.Status)
}

func IsTerminalTicketStatus(status string) bool {
	switch status {
	case TicketStatusCompleted, TicketStatusRejected, TicketStatusWithdrawn, TicketStatusCancelled, TicketStatusFailed:
		return true
	default:
		return false
	}
}

func TerminalTicketStatuses() []string {
	return []string{
		TicketStatusCompleted,
		TicketStatusRejected,
		TicketStatusWithdrawn,
		TicketStatusCancelled,
		TicketStatusFailed,
	}
}

func IsActiveTicketStatus(status string) bool {
	return !IsTerminalTicketStatus(status)
}

func TicketStatusLabel(status string, outcome string) string {
	switch status {
	case TicketStatusSubmitted:
		return "已提交"
	case TicketStatusWaitingHuman:
		return "待人工处理"
	case TicketStatusApprovedDecisioning:
		return "已同意，决策中"
	case TicketStatusRejectedDecisioning:
		return "已驳回，决策中"
	case TicketStatusDecisioning:
		return "AI 决策中"
	case TicketStatusExecutingAction:
		return "自动执行中"
	case TicketStatusCompleted:
		if outcome == TicketOutcomeFulfilled {
			return "已履约"
		}
		return "已通过"
	case TicketStatusRejected:
		return "已驳回"
	case TicketStatusWithdrawn:
		return "已撤回"
	case TicketStatusCancelled:
		return "已取消"
	case TicketStatusFailed:
		return "失败"
	default:
		return status
	}
}

func TicketStatusTone(status string, outcome string) string {
	switch status {
	case TicketStatusCompleted:
		return "success"
	case TicketStatusRejected, TicketStatusCancelled, TicketStatusFailed:
		return "destructive"
	case TicketStatusWithdrawn:
		return "secondary"
	case TicketStatusApprovedDecisioning, TicketStatusRejectedDecisioning, TicketStatusDecisioning, TicketStatusExecutingAction:
		return "progress"
	case TicketStatusWaitingHuman:
		return "warning"
	default:
		return "secondary"
	}
}

// TicketActivity 工单活动（工作流步骤）
type TicketActivity struct {
	model.BaseModel
	TicketID          uint       `json:"ticketId" gorm:"not null;index"`
	TokenID           *uint      `json:"tokenId" gorm:"column:token_id;index"`
	Name              string     `json:"name" gorm:"size:128"`
	ActivityType      string     `json:"activityType" gorm:"column:activity_type;size:16"`
	Status            string     `json:"status" gorm:"size:16;default:pending"`
	NodeID            string     `json:"nodeId" gorm:"column:node_id;size:64"`
	ExecutionMode     string     `json:"executionMode" gorm:"column:execution_mode;size:16"`
	ActivityGroupID   string     `json:"activityGroupId" gorm:"column:activity_group_id;size:36;index"`
	FormSchema        JSONField  `json:"formSchema" gorm:"column:form_schema;type:text"`
	FormData          JSONField  `json:"formData" gorm:"column:form_data;type:text"`
	TransitionOutcome string     `json:"transitionOutcome" gorm:"column:transition_outcome;size:16"`
	AIDecision        JSONField  `json:"aiDecision" gorm:"column:ai_decision;type:text"`
	AIReasoning       string     `json:"aiReasoning" gorm:"column:ai_reasoning;type:text"`
	AIConfidence      float64    `json:"aiConfidence" gorm:"column:ai_confidence;default:0"`
	OverriddenBy      *uint      `json:"overriddenBy" gorm:"column:overridden_by"`
	DecisionReasoning string     `json:"decisionReasoning" gorm:"column:decision_reasoning;type:text"`
	CanAct            bool       `json:"canAct" gorm:"-"`
	StartedAt         *time.Time `json:"startedAt" gorm:"column:started_at"`
	FinishedAt        *time.Time `json:"finishedAt" gorm:"column:finished_at"`
}

func (TicketActivity) TableName() string { return "itsm_ticket_activities" }

// TicketAssignment 工单参与人分配
type TicketAssignment struct {
	model.BaseModel
	TicketID        uint       `json:"ticketId" gorm:"not null;index"`
	ActivityID      uint       `json:"activityId" gorm:"not null;index"`
	ParticipantType string     `json:"participantType" gorm:"size:32;not null"` // requester | user | requester_manager | position | department | position_department
	UserID          *uint      `json:"userId" gorm:"index"`
	PositionID      *uint      `json:"positionId" gorm:"index"`
	DepartmentID    *uint      `json:"departmentId" gorm:"index"`
	AssigneeID      *uint      `json:"assigneeId" gorm:"index"` // actual claimed person
	Status          string     `json:"status" gorm:"size:16;default:pending"`
	Sequence        int        `json:"sequence" gorm:"default:0"`
	IsCurrent       bool       `json:"isCurrent" gorm:"default:false"`
	DelegatedFrom   *uint      `json:"delegatedFrom" gorm:"index"` // original assignment ID when delegated
	TransferFrom    *uint      `json:"transferFrom" gorm:"index"`  // original assignment ID when transferred
	ClaimedAt       *time.Time `json:"claimedAt"`
	FinishedAt      *time.Time `json:"finishedAt"`
}

func (TicketAssignment) TableName() string { return "itsm_ticket_assignments" }

// TicketTimeline 工单时间线
type TicketTimeline struct {
	model.BaseModel
	TicketID   uint      `json:"ticketId" gorm:"not null;index"`
	ActivityID *uint     `json:"activityId" gorm:"index"`
	OperatorID uint      `json:"operatorId" gorm:"not null"`
	EventType  string    `json:"eventType" gorm:"size:32;not null"`
	Message    string    `json:"message" gorm:"size:512"`
	Details    JSONField `json:"details" gorm:"type:text"`
	Reasoning  string    `json:"reasoning" gorm:"type:text"`
}

func (TicketTimeline) TableName() string { return "itsm_ticket_timelines" }

// ServiceDeskSubmission records the one-time confirmation boundary between a
// service desk draft and the ticket created from it.
type ServiceDeskSubmission struct {
	model.BaseModel
	SessionID    uint      `json:"sessionId" gorm:"not null;index;uniqueIndex:idx_itsm_submission_draft"`
	DraftVersion int       `json:"draftVersion" gorm:"not null;uniqueIndex:idx_itsm_submission_draft"`
	FieldsHash   string    `json:"fieldsHash" gorm:"size:128;not null;uniqueIndex:idx_itsm_submission_draft"`
	RequestHash  string    `json:"requestHash" gorm:"size:128;not null"`
	TicketID     uint      `json:"ticketId" gorm:"not null;index"`
	Status       string    `json:"status" gorm:"size:32;not null"`
	SubmittedBy  uint      `json:"submittedBy" gorm:"not null;index"`
	SubmittedAt  time.Time `json:"submittedAt" gorm:"not null"`
}

func (ServiceDeskSubmission) TableName() string { return "itsm_service_desk_submissions" }

type TicketTimelineResponse struct {
	ID           uint      `json:"id"`
	TicketID     uint      `json:"ticketId"`
	ActivityID   *uint     `json:"activityId"`
	OperatorID   uint      `json:"operatorId"`
	OperatorName string    `json:"operatorName"`
	EventType    string    `json:"eventType"`
	Message      string    `json:"message"`
	Content      string    `json:"content"`
	Details      JSONField `json:"details"`
	Metadata     JSONField `json:"metadata"`
	Reasoning    string    `json:"reasoning"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (t *TicketTimeline) ToResponse() TicketTimelineResponse {
	return TicketTimelineResponse{
		ID:         t.ID,
		TicketID:   t.TicketID,
		ActivityID: t.ActivityID,
		OperatorID: t.OperatorID,
		EventType:  t.EventType,
		Message:    t.Message,
		Content:    t.Message,
		Details:    t.Details,
		Metadata:   t.Details,
		Reasoning:  t.Reasoning,
		CreatedAt:  t.CreatedAt,
	}
}

// TicketActionExecution 动作执行记录
type TicketActionExecution struct {
	model.BaseModel
	TicketID        uint      `json:"ticketId" gorm:"not null;index"`
	ActivityID      uint      `json:"activityId" gorm:"not null;index"`
	ServiceActionID uint      `json:"serviceActionId" gorm:"not null"`
	Status          string    `json:"status" gorm:"size:16;default:pending"` // pending | success | failed
	RequestPayload  JSONField `json:"requestPayload" gorm:"type:text"`
	ResponsePayload JSONField `json:"responsePayload" gorm:"type:text"`
	FailureReason   string    `json:"failureReason" gorm:"type:text"`
	RetryCount      int       `json:"retryCount" gorm:"default:0"`
}

func (TicketActionExecution) TableName() string { return "itsm_ticket_action_executions" }

// TicketLink 工单关联
type TicketLink struct {
	model.BaseModel
	ParentTicketID uint   `json:"parentTicketId" gorm:"not null;index"`
	ChildTicketID  uint   `json:"childTicketId" gorm:"not null;index"`
	LinkType       string `json:"linkType" gorm:"size:16;not null"` // related | caused_by | blocked_by
}

func (TicketLink) TableName() string { return "itsm_ticket_links" }

// PostMortem 故障复盘
type PostMortem struct {
	model.BaseModel
	TicketID       uint      `json:"ticketId" gorm:"uniqueIndex;not null"`
	RootCause      string    `json:"rootCause" gorm:"type:text"`
	ImpactSummary  string    `json:"impactSummary" gorm:"type:text"`
	ActionItems    JSONField `json:"actionItems" gorm:"type:text"`
	LessonsLearned string    `json:"lessonsLearned" gorm:"type:text"`
	CreatedBy      uint      `json:"createdBy" gorm:"not null"`
}

func (PostMortem) TableName() string { return "itsm_post_mortems" }

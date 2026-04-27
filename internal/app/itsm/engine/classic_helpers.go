package engine

import "time"

// --- Lightweight model structs for direct DB operations ---
// These avoid importing the parent itsm package (which would cause a cycle).

type ticketModel struct {
	ID                    uint       `gorm:"primaryKey"`
	Code                  string     `gorm:"column:code"`
	Title                 string     `gorm:"column:title"`
	Status                string     `gorm:"column:status"`
	Outcome               string     `gorm:"column:outcome"`
	ServiceID             uint       `gorm:"column:service_id"`
	EngineType            string     `gorm:"column:engine_type"`
	WorkflowJSON          string     `gorm:"column:workflow_json;type:text"`
	CurrentActivityID     *uint      `gorm:"column:current_activity_id"`
	FinishedAt            *time.Time `gorm:"column:finished_at"`
	RequesterID           uint       `gorm:"column:requester_id"`
	PriorityID            uint       `gorm:"column:priority_id"`
	FormData              string     `gorm:"column:form_data;type:text"`
	SLAResponseDeadline   *time.Time `gorm:"column:sla_response_deadline"`
	SLAResolutionDeadline *time.Time `gorm:"column:sla_resolution_deadline"`
	SLAStatus             string     `gorm:"column:sla_status"`
	SLAPausedAt           *time.Time `gorm:"column:sla_paused_at"`
	AIFailureCount        int        `gorm:"column:ai_failure_count;default:0"`
	CollaborationSpec     string     `gorm:"column:collaboration_spec;type:text"` // via service join
	AgentID               *uint      `gorm:"column:agent_id"`                     // via service join
	AgentConfig           string     `gorm:"column:agent_config;type:text"`       // via service join
}

func (ticketModel) TableName() string { return "itsm_tickets" }

type activityModel struct {
	ID                uint       `gorm:"primaryKey;autoIncrement"`
	TicketID          uint       `gorm:"column:ticket_id;not null"`
	TokenID           *uint      `gorm:"column:token_id;index"`
	Name              string     `gorm:"column:name;size:128"`
	ActivityType      string     `gorm:"column:activity_type;size:16"`
	Status            string     `gorm:"column:status;size:16;default:pending"`
	NodeID            string     `gorm:"column:node_id;size:64"`
	ExecutionMode     string     `gorm:"column:execution_mode;size:16"`
	ActivityGroupID   string     `gorm:"column:activity_group_id;size:36"`
	FormSchema        string     `gorm:"column:form_schema;type:text"`
	FormData          string     `gorm:"column:form_data;type:text"`
	TransitionOutcome string     `gorm:"column:transition_outcome;size:16"`
	AIDecision        string     `gorm:"column:ai_decision;type:text"`
	AIReasoning       string     `gorm:"column:ai_reasoning;type:text"`
	AIConfidence      float64    `gorm:"column:ai_confidence;default:0"`
	OverriddenBy      *uint      `gorm:"column:overridden_by"`
	DecisionReasoning string     `gorm:"column:decision_reasoning;type:text"`
	StartedAt         *time.Time `gorm:"column:started_at"`
	FinishedAt        *time.Time `gorm:"column:finished_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (activityModel) TableName() string { return "itsm_ticket_activities" }

type assignmentModel struct {
	ID              uint       `gorm:"primaryKey;autoIncrement"`
	TicketID        uint       `gorm:"column:ticket_id;not null"`
	ActivityID      uint       `gorm:"column:activity_id;not null"`
	ParticipantType string     `gorm:"column:participant_type;size:32;not null"`
	UserID          *uint      `gorm:"column:user_id"`
	PositionID      *uint      `gorm:"column:position_id"`
	DepartmentID    *uint      `gorm:"column:department_id"`
	AssigneeID      *uint      `gorm:"column:assignee_id"`
	Status          string     `gorm:"column:status;size:16;default:pending"`
	Sequence        int        `gorm:"column:sequence;default:0"`
	IsCurrent       bool       `gorm:"column:is_current;default:false"`
	DelegatedFrom   *uint      `gorm:"column:delegated_from"`
	TransferFrom    *uint      `gorm:"column:transfer_from"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`
	FinishedAt      *time.Time `gorm:"column:finished_at"`
}

func (assignmentModel) TableName() string { return "itsm_ticket_assignments" }

type timelineModel struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	TicketID   uint      `gorm:"column:ticket_id;not null"`
	ActivityID *uint     `gorm:"column:activity_id"`
	OperatorID uint      `gorm:"column:operator_id;not null"`
	EventType  string    `gorm:"column:event_type;size:32;not null"`
	Message    string    `gorm:"column:message;size:512"`
	Details    string    `gorm:"column:details;type:text"`
	Reasoning  string    `gorm:"column:reasoning;type:text"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (timelineModel) TableName() string { return "itsm_ticket_timelines" }

// executionTokenModel is a lightweight model for direct DB operations on tokens.
type executionTokenModel struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"`
	TicketID      uint      `gorm:"column:ticket_id;not null;index:idx_token_ticket_status"`
	ParentTokenID *uint     `gorm:"column:parent_token_id"`
	NodeID        string    `gorm:"column:node_id;size:64"`
	Status        string    `gorm:"column:status;size:16;not null;index:idx_token_ticket_status"`
	TokenType     string    `gorm:"column:token_type;size:16;not null"`
	ScopeID       string    `gorm:"column:scope_id;size:64;not null;default:root"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (executionTokenModel) TableName() string { return "itsm_execution_tokens" }

// --- Small utility functions ---

func labelOrDefault(data *NodeData, fallback string) string {
	if data.Label != "" {
		return data.Label
	}
	return fallback
}

func nodeLabel(node *WFNode) string {
	data, _ := ParseNodeData(node.Data)
	if data != nil && data.Label != "" {
		return data.Label
	}
	return node.ID
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Minute // default 30 minutes
	}
	return d
}

package contract

import "encoding/json"

// WorkflowDef is the single backend DTO for executable ReactFlow workflow JSON.
type WorkflowDef struct {
	Nodes []WFNode `json:"nodes"`
	Edges []WFEdge `json:"edges"`
}

type WFNode struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type WFEdge struct {
	ID     string     `json:"id"`
	Source string     `json:"source"`
	Target string     `json:"target"`
	Data   WFEdgeData `json:"data"`
}

type WFEdgeData struct {
	Outcome   string            `json:"outcome"`
	Default   bool              `json:"default"`
	Condition *GatewayCondition `json:"condition,omitempty"`
}

type NodeData struct {
	Label            string             `json:"label"`
	FormSchema       json.RawMessage    `json:"formSchema,omitempty"`
	Participants     []Participant      `json:"participants,omitempty"`
	ActionID         uint               `json:"action_id,omitempty"`
	Conditions       []GatewayCondition `json:"conditions,omitempty"`
	ChannelID        uint               `json:"channel_id,omitempty"`
	Template         string             `json:"template,omitempty"`
	Recipients       []Participant      `json:"recipients,omitempty"`
	WaitMode         string             `json:"wait_mode,omitempty"`
	Duration         string             `json:"duration,omitempty"`
	GatewayDirection string             `json:"gateway_direction,omitempty"`
	Assignments      []Assignment       `json:"assignments,omitempty"`
	AttachedTo       string             `json:"attached_to,omitempty"`
	Interrupting     bool               `json:"interrupting,omitempty"`
	SubProcessDef    json.RawMessage    `json:"subprocess_def,omitempty"`
}

type Participant struct {
	Type           string `json:"type"`
	Value          string `json:"value,omitempty"`
	PositionCode   string `json:"position_code,omitempty"`
	DepartmentCode string `json:"department_code,omitempty"`
}

type Assignment struct {
	Variable   string `json:"variable"`
	Expression string `json:"expression"`
}

type GatewayCondition struct {
	Field      string             `json:"field"`
	Operator   string             `json:"operator"`
	Value      any                `json:"value"`
	EdgeID     string             `json:"edge_id"`
	Logic      string             `json:"logic,omitempty"`
	Conditions []GatewayCondition `json:"conditions,omitempty"`
}

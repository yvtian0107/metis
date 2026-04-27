package engine

import "encoding/json"

// WorkflowDef represents the parsed ReactFlow JSON structure.
type WorkflowDef struct {
	Nodes []WFNode `json:"nodes"`
	Edges []WFEdge `json:"edges"`
}

// WFNode represents a ReactFlow node. Backend only reads id, type, data.
type WFNode struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	// ReactFlow layout fields (position, style, etc.) are ignored by the engine.
}

// WFEdge represents a ReactFlow edge. Backend reads id, source, target, data.
type WFEdge struct {
	ID     string     `json:"id"`
	Source string     `json:"source"`
	Target string     `json:"target"`
	Data   WFEdgeData `json:"data"`
}

// WFEdgeData holds edge configuration relevant to the engine.
type WFEdgeData struct {
	Outcome   string            `json:"outcome"`
	Default   bool              `json:"default"`
	Condition *GatewayCondition `json:"condition,omitempty"`
}

// NodeData holds parsed node configuration (varies by type).
type NodeData struct {
	Label            string             `json:"label"`
	FormSchema       json.RawMessage    `json:"formSchema,omitempty"`
	Participants     []Participant      `json:"participants,omitempty"`
	ActionID         uint               `json:"action_id,omitempty"`
	Conditions       []GatewayCondition `json:"conditions,omitempty"`
	ChannelID        uint               `json:"channel_id,omitempty"`
	Template         string             `json:"template,omitempty"`
	Recipients       []Participant      `json:"recipients,omitempty"`
	WaitMode         string             `json:"wait_mode,omitempty"`         // signal | timer
	Duration         string             `json:"duration,omitempty"`          // e.g. "2h", "30m"
	GatewayDirection string             `json:"gateway_direction,omitempty"` // fork | join (parallel/inclusive only)
	Assignments      []Assignment       `json:"assignments,omitempty"`       // script node variable assignments
	AttachedTo       string             `json:"attached_to,omitempty"`       // boundary event: host node ID
	Interrupting     bool               `json:"interrupting,omitempty"`      // boundary timer: interrupting mode
	SubProcessDef    json.RawMessage    `json:"subprocess_def,omitempty"`    // embedded subprocess workflow definition
}

// Participant defines who should handle a node.
type Participant struct {
	Type           string `json:"type"`                      // requester | user | position | department | position_department | requester_manager
	Value          string `json:"value,omitempty"`           // user ID, position ID, or department ID (string for flexibility)
	PositionCode   string `json:"position_code,omitempty"`   // position_department: position code
	DepartmentCode string `json:"department_code,omitempty"` // position_department: department code
}

// Assignment defines a variable assignment for script nodes.
type Assignment struct {
	Variable   string `json:"variable"`   // target variable name
	Expression string `json:"expression"` // expr-lang/expr expression
}

// GatewayCondition defines a single or compound condition for gateway evaluation.
type GatewayCondition struct {
	Field      string             `json:"field"`                // e.g. "ticket.priority", "form.urgency"
	Operator   string             `json:"operator"`             // equals | not_equals | contains_any | gt | lt | gte | lte | in | not_in | is_empty | is_not_empty | between | matches
	Value      any                `json:"value"`                // comparison value
	EdgeID     string             `json:"edge_id"`              // the edge this condition maps to
	Logic      string             `json:"logic,omitempty"`      // "and" | "or" for compound conditions
	Conditions []GatewayCondition `json:"conditions,omitempty"` // sub-conditions for compound evaluation
}

// ParseWorkflowDef parses workflow JSON into a WorkflowDef.
func ParseWorkflowDef(workflowJSON json.RawMessage) (*WorkflowDef, error) {
	var def WorkflowDef
	if err := json.Unmarshal(workflowJSON, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

// ParseNodeData parses the raw node data into a structured NodeData.
func ParseNodeData(raw json.RawMessage) (*NodeData, error) {
	var data NodeData
	if len(raw) == 0 {
		return &data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// BuildMaps builds lookup maps for nodes and edges from a WorkflowDef.
func (def *WorkflowDef) BuildMaps() (nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge) {
	nodeMap = make(map[string]*WFNode, len(def.Nodes))
	outEdges = make(map[string][]*WFEdge)
	for i := range def.Nodes {
		nodeMap[def.Nodes[i].ID] = &def.Nodes[i]
	}
	for i := range def.Edges {
		outEdges[def.Edges[i].Source] = append(outEdges[def.Edges[i].Source], &def.Edges[i])
	}
	return
}

// FindStartNode returns the single start node.
func (def *WorkflowDef) FindStartNode() (*WFNode, error) {
	var start *WFNode
	for i := range def.Nodes {
		if def.Nodes[i].Type == NodeStart {
			if start != nil {
				return nil, ErrMultipleStartNodes
			}
			start = &def.Nodes[i]
		}
	}
	if start == nil {
		return nil, ErrNoStartNode
	}
	return start, nil
}

// BuildBoundaryMap scans all b_timer/b_error nodes and groups them by attached_to host node ID.
func (def *WorkflowDef) BuildBoundaryMap() map[string][]*WFNode {
	m := make(map[string][]*WFNode)
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeBTimer && n.Type != NodeBError {
			continue
		}
		nd, err := ParseNodeData(n.Data)
		if err != nil || nd.AttachedTo == "" {
			continue
		}
		m[nd.AttachedTo] = append(m[nd.AttachedTo], n)
	}
	return m
}

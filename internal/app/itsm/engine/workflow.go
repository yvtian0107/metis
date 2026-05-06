package engine

import (
	"encoding/json"

	"metis/internal/app/itsm/contract"
)

type WorkflowDef contract.WorkflowDef
type WFNode = contract.WFNode
type WFEdge = contract.WFEdge
type WFEdgeData = contract.WFEdgeData
type NodeData = contract.NodeData
type Participant = contract.Participant
type Assignment = contract.Assignment
type GatewayCondition = contract.GatewayCondition

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

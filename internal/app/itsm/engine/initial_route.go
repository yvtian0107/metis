package engine

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrInitialRouteNoMatch means the workflow has an initial routing gateway but
// the submitted form data does not satisfy any route and no default edge exists.
var ErrInitialRouteNoMatch = errors.New("initial route has no matching condition")

// ReachableParticipantNode is a human-facing workflow node selected by the
// initial request form data before any later human outcome is known.
type ReachableParticipantNode struct {
	ID           string
	Type         string
	Label        string
	Participants []Participant
}

// InitialReachableParticipantNodes returns participant-bearing form/process
// nodes reachable from the start node using only initial form data.
func InitialReachableParticipantNodes(workflowJSON json.RawMessage, formData map[string]any) ([]ReachableParticipantNode, error) {
	def, err := ParseWorkflowDef(workflowJSON)
	if err != nil {
		return nil, err
	}
	start, err := def.FindStartNode()
	if err != nil {
		return nil, err
	}
	nodeMap, outEdges := def.BuildMaps()
	evalCtx := evalContext{}
	for key, val := range formData {
		evalCtx["form."+key] = val
	}

	visited := map[string]bool{}
	var nodes []ReachableParticipantNode
	var walk func(nodeID string) error
	walk = func(nodeID string) error {
		if visited[nodeID] {
			return nil
		}
		visited[nodeID] = true

		node, ok := nodeMap[nodeID]
		if !ok {
			return fmt.Errorf("workflow node %q not found", nodeID)
		}
		data, err := ParseNodeData(node.Data)
		if err != nil {
			return err
		}

		switch node.Type {
		case NodeEnd:
			return nil
		case NodeExclusive:
			edge, err := selectInitialExclusiveEdge(node.ID, data, outEdges[node.ID], evalCtx)
			if err != nil {
				return err
			}
			return walk(edge.Target)
		case NodeParallel:
			for _, edge := range outEdges[node.ID] {
				if err := walk(edge.Target); err != nil {
					return err
				}
			}
			return nil
		case NodeInclusive:
			edges, err := selectedInitialInclusiveEdges(node.ID, outEdges[node.ID], evalCtx)
			if err != nil {
				return err
			}
			for _, edge := range edges {
				if err := walk(edge.Target); err != nil {
					return err
				}
			}
			return nil
		case NodeForm, NodeProcess:
			if len(data.Participants) > 0 {
				nodes = append(nodes, ReachableParticipantNode{
					ID:           node.ID,
					Type:         node.Type,
					Label:        nodeLabelFromData(node.ID, data),
					Participants: data.Participants,
				})
			}
			if node.Type == NodeForm && formNodeSatisfiedByInitialRequest(data.Participants) {
				for _, edge := range outEdges[node.ID] {
					if err := walk(edge.Target); err != nil {
						return err
					}
				}
			}
			return nil
		default:
			for _, edge := range outEdges[node.ID] {
				if err := walk(edge.Target); err != nil {
					return err
				}
			}
			return nil
		}
	}

	if err := walk(start.ID); err != nil {
		return nil, err
	}
	return nodes, nil
}

func selectInitialExclusiveEdge(nodeID string, data *NodeData, edges []*WFEdge, evalCtx evalContext) (*WFEdge, error) {
	var defaultEdge *WFEdge
	for _, edge := range edges {
		if edge.Data.Default {
			defaultEdge = edge
			continue
		}
		if edge.Data.Condition != nil && evaluateCondition(*edge.Data.Condition, evalCtx) {
			return edge, nil
		}
	}
	for _, cond := range data.Conditions {
		if !evaluateCondition(cond, evalCtx) {
			continue
		}
		for _, edge := range edges {
			if edge.ID == cond.EdgeID {
				return edge, nil
			}
		}
	}
	if defaultEdge != nil {
		return defaultEdge, nil
	}
	return nil, fmt.Errorf("%w: exclusive gateway %s has no default edge", ErrInitialRouteNoMatch, nodeID)
}

func selectedInitialInclusiveEdges(nodeID string, edges []*WFEdge, evalCtx evalContext) ([]*WFEdge, error) {
	var selected []*WFEdge
	var defaultEdge *WFEdge
	for _, edge := range edges {
		if edge.Data.Default {
			defaultEdge = edge
			continue
		}
		if edge.Data.Condition == nil || evaluateCondition(*edge.Data.Condition, evalCtx) {
			selected = append(selected, edge)
		}
	}
	if len(selected) == 0 && defaultEdge != nil {
		selected = append(selected, defaultEdge)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("%w: inclusive gateway %s has no default edge", ErrInitialRouteNoMatch, nodeID)
	}
	return selected, nil
}

func formNodeSatisfiedByInitialRequest(participants []Participant) bool {
	if len(participants) == 0 {
		return true
	}
	for _, p := range participants {
		if p.Type != "requester" {
			return false
		}
	}
	return true
}

func nodeLabelFromData(nodeID string, data *NodeData) string {
	if data != nil && data.Label != "" {
		return data.Label
	}
	return nodeID
}

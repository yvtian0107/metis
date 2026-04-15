package engine

import (
	"encoding/json"
	"fmt"
)

// ValidationError represents a single validation issue.
type ValidationError struct {
	NodeID  string `json:"nodeId,omitempty"`
	EdgeID  string `json:"edgeId,omitempty"`
	Level   string `json:"level"` // "error" or "warning"; defaults to "error"
	Message string `json:"message"`
}

func (e ValidationError) Error() string { return e.Message }

// IsWarning returns true if this is a warning-level validation result, not a blocking error.
func (e ValidationError) IsWarning() bool { return e.Level == "warning" }

// ValidateWorkflow checks a workflow JSON for structural integrity.
// Returns a list of validation errors and warnings. Only errors block saving.
func ValidateWorkflow(workflowJSON json.RawMessage) []ValidationError {
	var errs []ValidationError

	def, err := ParseWorkflowDef(workflowJSON)
	if err != nil {
		return []ValidationError{{Level: "error", Message: fmt.Sprintf("JSON 解析失败: %v", err)}}
	}

	nodeMap := make(map[string]*WFNode, len(def.Nodes))
	for i := range def.Nodes {
		nodeMap[def.Nodes[i].ID] = &def.Nodes[i]
	}

	// Build edge maps
	outEdges := make(map[string][]*WFEdge)
	inEdges := make(map[string][]*WFEdge)
	for i := range def.Edges {
		e := &def.Edges[i]
		outEdges[e.Source] = append(outEdges[e.Source], e)
		inEdges[e.Target] = append(inEdges[e.Target], e)
	}

	// 1. Validate node types + warn for unimplemented types
	var startNodes []*WFNode
	var endNodes []*WFNode
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if !ValidNodeTypes[n.Type] {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("节点 %s 的类型 %q 不合法", n.ID, n.Type),
			})
		} else if UnimplementedNodeTypes[n.Type] {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "warning",
				Message: fmt.Sprintf("节点 %s 类型 %s 已注册但执行逻辑尚未实现，当前版本不支持运行", n.ID, n.Type),
			})
		}
		if n.Type == NodeStart {
			startNodes = append(startNodes, n)
		}
		if n.Type == NodeEnd {
			endNodes = append(endNodes, n)
		}
	}

	// 2. Exactly one start node
	if len(startNodes) == 0 {
		errs = append(errs, ValidationError{Level: "error", Message: "工作流必须包含一个开始节点"})
	} else if len(startNodes) > 1 {
		errs = append(errs, ValidationError{Level: "error", Message: "工作流只能包含一个开始节点"})
	} else {
		// Start node must have exactly one outgoing edge
		start := startNodes[0]
		if len(outEdges[start.ID]) != 1 {
			errs = append(errs, ValidationError{
				NodeID:  start.ID,
				Level:   "error",
				Message: "开始节点必须有且仅有一条出边",
			})
		}
		// Start node should have no incoming edges
		if len(inEdges[start.ID]) > 0 {
			errs = append(errs, ValidationError{
				NodeID:  start.ID,
				Level:   "error",
				Message: "开始节点不应有入边",
			})
		}
	}

	// 3. At least one end node
	if len(endNodes) == 0 {
		errs = append(errs, ValidationError{Level: "error", Message: "工作流必须包含至少一个结束节点"})
	} else {
		// End nodes must have no outgoing edges
		for _, n := range endNodes {
			if len(outEdges[n.ID]) > 0 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("结束节点 %s 不应有出边", n.ID),
				})
			}
		}
	}

	// 4. Edge references valid nodes
	for i := range def.Edges {
		e := &def.Edges[i]
		if _, ok := nodeMap[e.Source]; !ok {
			errs = append(errs, ValidationError{
				EdgeID:  e.ID,
				Level:   "error",
				Message: fmt.Sprintf("边 %s 引用了不存在的源节点 %s", e.ID, e.Source),
			})
		}
		if _, ok := nodeMap[e.Target]; !ok {
			errs = append(errs, ValidationError{
				EdgeID:  e.ID,
				Level:   "error",
				Message: fmt.Sprintf("边 %s 引用了不存在的目标节点 %s", e.ID, e.Target),
			})
		}
	}

	// 5. No isolated nodes (every non-start node must have at least one incoming edge)
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type == NodeStart {
			continue
		}
		if len(inEdges[n.ID]) == 0 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("节点 %s 没有入边，无法到达", n.ID),
			})
		}
	}

	// 6. Exclusive gateway constraints
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeExclusive {
			continue
		}
		edges := outEdges[n.ID]
		if len(edges) < 2 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("排他网关节点 %s 至少需要两条出边", n.ID),
			})
			continue
		}
		for _, e := range edges {
			if !e.Data.Default && e.Data.Condition == nil {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					EdgeID:  e.ID,
					Level:   "error",
					Message: fmt.Sprintf("排他网关节点 %s 的出边 %s 缺少条件配置", n.ID, e.ID),
				})
			}
		}
	}

	// 7. Parallel / Inclusive gateway constraints (④ itsm-gateway-parallel)
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeParallel && n.Type != NodeInclusive {
			continue
		}

		// Parse node data to get gateway_direction
		nd, err := ParseNodeData(n.Data)
		if err != nil {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("节点 %s 数据解析失败: %v", n.ID, err),
			})
			continue
		}

		typeName := "并行"
		if n.Type == NodeInclusive {
			typeName = "包含"
		}

		// gateway_direction is required
		if nd.GatewayDirection != GatewayFork && nd.GatewayDirection != GatewayJoin {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("节点 %s 类型 %s 必须配置 gateway_direction（fork 或 join）", n.ID, n.Type),
			})
			continue
		}

		if nd.GatewayDirection == GatewayFork {
			// Fork: at least 2 outgoing edges
			nodeOutEdges := outEdges[n.ID]
			if len(nodeOutEdges) < 2 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("%s网关 fork 节点 %s 至少需要两条出边", typeName, n.ID),
				})
			}

			// Inclusive fork: non-default edges must have conditions
			if n.Type == NodeInclusive {
				for _, e := range nodeOutEdges {
					if !e.Data.Default && e.Data.Condition == nil {
						errs = append(errs, ValidationError{
							NodeID:  n.ID,
							EdgeID:  e.ID,
							Level:   "error",
							Message: fmt.Sprintf("包含网关 fork 节点 %s 的出边 %s 缺少条件配置", n.ID, e.ID),
						})
					}
				}
			}
		} else {
			// Join: at least 2 incoming edges, exactly 1 outgoing edge
			nodeInEdges := inEdges[n.ID]
			if len(nodeInEdges) < 2 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("%s网关 join 节点 %s 至少需要两条入边", typeName, n.ID),
				})
			}
			nodeOutEdges := outEdges[n.ID]
			if len(nodeOutEdges) != 1 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("%s网关 join 节点 %s 必须有且仅有一条出边", typeName, n.ID),
				})
			}
		}
	}

	// 8. Script node constraints (⑤a itsm-script-task)
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeScript {
			continue
		}
		nodeOutEdges := outEdges[n.ID]
		if len(nodeOutEdges) != 1 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("脚本节点 %s 必须有且仅有一条出边", n.ID),
			})
		}
	}

	return errs
}

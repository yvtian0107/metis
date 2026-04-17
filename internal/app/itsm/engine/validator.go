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

	// 6b. Validate compound conditions on edges
	for i := range def.Edges {
		e := &def.Edges[i]
		if e.Data.Condition != nil {
			if condErrs := validateGatewayCondition(*e.Data.Condition, e.ID); len(condErrs) > 0 {
				errs = append(errs, condErrs...)
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

	// 9. Boundary event constraints (⑤b itsm-boundary-events)
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeBTimer && n.Type != NodeBError {
			continue
		}

		nd, err := ParseNodeData(n.Data)
		if err != nil {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("节点 %s 数据解析失败: %v", n.ID, err),
			})
			continue
		}

		typeName := "边界定时器"
		if n.Type == NodeBError {
			typeName = "边界错误事件"
		}

		// attached_to required
		if nd.AttachedTo == "" {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("%s %s 必须配置 attached_to", typeName, n.ID),
			})
			continue
		}

		// attached_to must reference an existing node
		hostNode, ok := nodeMap[nd.AttachedTo]
		if !ok {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("%s %s 的 attached_to 引用了不存在的节点 %s", typeName, n.ID, nd.AttachedTo),
			})
			continue
		}

		// b_timer must attach to human nodes (form/approve/process)
		if n.Type == NodeBTimer {
			if hostNode.Type != NodeForm && hostNode.Type != NodeApprove && hostNode.Type != NodeProcess {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("边界定时器 %s 只能附着在人工节点（form/approve/process）上，当前附着在 %s", n.ID, hostNode.Type),
				})
			}
			// b_timer must have duration
			if nd.Duration == "" {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("边界定时器 %s 必须配置 duration", n.ID),
				})
			}
		}

		// b_error must attach to action nodes
		if n.Type == NodeBError {
			if hostNode.Type != NodeAction {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("边界错误事件 %s 只能附着在 action 节点上，当前附着在 %s", n.ID, hostNode.Type),
				})
			}
		}

		// boundary nodes must have exactly one outgoing edge
		nodeOutEdges := outEdges[n.ID]
		if len(nodeOutEdges) != 1 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("%s %s 必须有且仅有一条出边", typeName, n.ID),
			})
		}

		// boundary nodes must have no incoming edges
		if len(inEdges[n.ID]) > 0 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("%s %s 不应有入边", typeName, n.ID),
			})
		}
	}

	// 10. Subprocess node constraints (⑤c itsm-subprocess)
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeSubprocess {
			continue
		}

		// Must have exactly one outgoing edge
		nodeOutEdges := outEdges[n.ID]
		if len(nodeOutEdges) != 1 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("子流程节点 %s 必须有且仅有一条出边", n.ID),
			})
		}

		// SubProcessDef must be present and parseable
		nd, err := ParseNodeData(n.Data)
		if err != nil {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("子流程节点 %s 数据解析失败: %v", n.ID, err),
			})
			continue
		}

		if len(nd.SubProcessDef) == 0 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("子流程节点 %s 必须配置 subprocess_def", n.ID),
			})
			continue
		}

		subDef, err := ParseWorkflowDef(nd.SubProcessDef)
		if err != nil {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("子流程节点 %s 的 subprocess_def 解析失败: %v", n.ID, err),
			})
			continue
		}

		// Reject nested subprocess (v1 limitation)
		for _, subNode := range subDef.Nodes {
			if subNode.Type == NodeSubprocess {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("子流程节点 %s 内包含嵌套子流程 %s，当前版本不支持嵌套子流程", n.ID, subNode.ID),
				})
			}
		}

		// Recursively validate the subprocess definition
		subErrs := ValidateWorkflow(nd.SubProcessDef)
		for _, se := range subErrs {
			prefix := fmt.Sprintf("子流程 %s → ", n.ID)
			errs = append(errs, ValidationError{
				NodeID:  se.NodeID,
				EdgeID:  se.EdgeID,
				Level:   se.Level,
				Message: prefix + se.Message,
			})
		}
	}

	return errs
}

// validateGatewayCondition recursively validates a gateway condition.
func validateGatewayCondition(cond GatewayCondition, edgeID string) []ValidationError {
	var errs []ValidationError

	if cond.Logic != "" {
		// Compound condition
		if cond.Logic != "and" && cond.Logic != "or" {
			errs = append(errs, ValidationError{
				EdgeID:  edgeID,
				Level:   "error",
				Message: fmt.Sprintf("边 %s 的条件 logic 值 %q 不合法，仅支持 and/or", edgeID, cond.Logic),
			})
		}
		if len(cond.Conditions) == 0 {
			errs = append(errs, ValidationError{
				EdgeID:  edgeID,
				Level:   "error",
				Message: fmt.Sprintf("边 %s 的复合条件（logic=%s）缺少子条件", edgeID, cond.Logic),
			})
		}
		for _, sub := range cond.Conditions {
			errs = append(errs, validateGatewayCondition(sub, edgeID)...)
		}
	} else {
		// Leaf condition: must have field and operator
		if cond.Field == "" {
			errs = append(errs, ValidationError{
				EdgeID:  edgeID,
				Level:   "error",
				Message: fmt.Sprintf("边 %s 的条件缺少 field", edgeID),
			})
		}
		if cond.Operator == "" {
			errs = append(errs, ValidationError{
				EdgeID:  edgeID,
				Level:   "error",
				Message: fmt.Sprintf("边 %s 的条件缺少 operator", edgeID),
			})
		}
	}

	return errs
}

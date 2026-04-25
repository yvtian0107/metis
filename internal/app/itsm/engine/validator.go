package engine

import (
	"encoding/json"
	"fmt"
	"strings"
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
				Level:   "error",
				Message: fmt.Sprintf("节点 %s 类型 %s 已注册但执行逻辑尚未实现，当前版本不支持运行", n.ID, n.Type),
			})
		}
		if n.Type == NodeStart {
			startNodes = append(startNodes, n)
		}
		if n.Type == NodeEnd {
			endNodes = append(endNodes, n)
		}
		if IsHumanNode(n.Type) {
			if participantErrs := validateHumanNodeParticipants(n); len(participantErrs) > 0 {
				errs = append(errs, participantErrs...)
			}
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

	// 6. Process node outcome edges — every process node must have both
	//    an approved and a rejected outgoing edge so the smart engine
	//    knows where to route each human decision outcome.
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeProcess {
			continue
		}
		edges := outEdges[n.ID]
		if len(edges) == 0 {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("process 节点 %s 至少需要一条出边", n.ID),
			})
			continue
		}
		hasApproved, hasRejected := false, false
		var approvedTarget, rejectedTarget string
		for _, e := range edges {
			switch e.Data.Outcome {
			case "approved":
				hasApproved = true
				approvedTarget = e.Target
			case "rejected":
				hasRejected = true
				rejectedTarget = e.Target
			}
		}
		if !hasApproved {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("process 节点 %s 缺少 outcome=\"approved\" 的出边；每个 process 节点必须有 approved 和 rejected 两条出边", n.ID),
			})
		}
		if !hasRejected {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("process 节点 %s 缺少 outcome=\"rejected\" 的出边；协作规范未定义驳回路径时 rejected 应指向一个独立的 end 节点", n.ID),
			})
		}
		if hasApproved && hasRejected && approvedTarget != "" && approvedTarget == rejectedTarget {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("process 节点 %s 的 approved 和 rejected 出边指向同一个目标节点 %s；两条出边必须指向不同的目标节点以形成清晰的审批分支", n.ID, approvedTarget),
			})
		}
	}

	// 7. Exclusive gateway constraints
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

	// 7b. Validate compound conditions on edges
	for i := range def.Edges {
		e := &def.Edges[i]
		if e.Data.Condition != nil {
			if condErrs := validateGatewayCondition(*e.Data.Condition, e.ID); len(condErrs) > 0 {
				errs = append(errs, condErrs...)
			}
		}
	}

	// 8. Parallel / Inclusive gateway constraints (④ itsm-gateway-parallel)
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
		switch n.Type {
		case NodeAction:
			nd, err := ParseNodeData(n.Data)
			if err != nil {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("动作节点 %s 数据解析失败: %v", n.ID, err),
				})
				continue
			}
			if nd.ActionID == 0 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("动作节点 %s 必须配置 action_id", n.ID),
				})
			}
			if len(outEdges[n.ID]) == 0 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("动作节点 %s 至少需要一条出边", n.ID),
				})
			}
		case NodeNotify:
			if len(outEdges[n.ID]) != 1 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("通知节点 %s 必须有且仅有一条出边", n.ID),
				})
			}
			nd, err := ParseNodeData(n.Data)
			if err != nil {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("通知节点 %s 数据解析失败: %v", n.ID, err),
				})
				continue
			}
			if nd.ChannelID == 0 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "warning",
					Message: fmt.Sprintf("通知节点 %s 未配置 channel_id，将只记录流程时间线", n.ID),
				})
			}
		case NodeWait:
			nd, err := ParseNodeData(n.Data)
			if err != nil {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("等待节点 %s 数据解析失败: %v", n.ID, err),
				})
				continue
			}
			if nd.WaitMode != "signal" && nd.WaitMode != "timer" {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("等待节点 %s 必须配置 wait_mode（signal 或 timer）", n.ID),
				})
			}
			if nd.WaitMode == "timer" && nd.Duration == "" {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("等待节点 %s 使用 timer 模式时必须配置 duration", n.ID),
				})
			}
			if len(outEdges[n.ID]) == 0 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("等待节点 %s 至少需要一条出边", n.ID),
				})
			}
		case NodeScript:
			nodeOutEdges := outEdges[n.ID]
			if len(nodeOutEdges) != 1 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("脚本节点 %s 必须有且仅有一条出边", n.ID),
				})
			}
			nd, err := ParseNodeData(n.Data)
			if err != nil {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("脚本节点 %s 数据解析失败: %v", n.ID, err),
				})
				continue
			}
			if len(nd.Assignments) == 0 {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("脚本节点 %s 必须配置 assignments", n.ID),
				})
			}
			for j, assignment := range nd.Assignments {
				if assignment.Variable == "" || assignment.Expression == "" {
					errs = append(errs, ValidationError{
						NodeID:  n.ID,
						Level:   "error",
						Message: fmt.Sprintf("脚本节点 %s 的第 %d 个 assignment 必须同时配置 variable 和 expression", n.ID, j+1),
					})
				}
			}
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

		// b_timer must attach to human nodes.
		if n.Type == NodeBTimer {
			if !IsHumanNode(hostNode.Type) {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("边界定时器 %s 只能附着在人工节点上，当前附着在 %s", n.ID, hostNode.Type),
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

	// 10b. Validate formSchema references in exclusive gateway conditions
	errs = append(errs, validateFormSchemaReferences(def, nodeMap, inEdges)...)

	return errs
}

// validateFormSchemaReferences checks that form.xxx fields referenced in exclusive gateway
// conditions actually exist in upstream form nodes' formSchema.
func validateFormSchemaReferences(def *WorkflowDef, nodeMap map[string]*WFNode, inEdges map[string][]*WFEdge) []ValidationError {
	var errs []ValidationError

	// Collect formSchema keys from all form nodes: nodeID -> set of field keys
	formFieldsByNode := make(map[string]map[string]bool)
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeForm {
			continue
		}
		nd, err := ParseNodeData(n.Data)
		if err != nil || len(nd.FormSchema) == 0 {
			continue
		}
		var schema struct {
			Fields []struct {
				Key string `json:"key"`
			} `json:"fields"`
		}
		if err := json.Unmarshal(nd.FormSchema, &schema); err != nil {
			continue
		}
		keys := make(map[string]bool, len(schema.Fields))
		for _, f := range schema.Fields {
			if f.Key != "" {
				keys[f.Key] = true
			}
		}
		if len(keys) > 0 {
			formFieldsByNode[n.ID] = keys
		}
	}

	if len(formFieldsByNode) == 0 {
		return nil // no form nodes with formSchema, nothing to check
	}

	// For each exclusive gateway, check condition field references
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.Type != NodeExclusive {
			continue
		}

		// BFS backwards to find upstream form nodes reachable from this gateway
		upstreamKeys := collectUpstreamFormKeys(n.ID, nodeMap, inEdges, formFieldsByNode)
		if len(upstreamKeys) == 0 {
			continue // no upstream form nodes — can't validate
		}

		// Check each outgoing edge's condition
		for _, e := range def.Edges {
			if e.Source != n.ID || e.Data.Condition == nil {
				continue
			}
			errs = append(errs, checkConditionFormRefs(*e.Data.Condition, n.ID, e.ID, upstreamKeys)...)
		}
	}

	return errs
}

// collectUpstreamFormKeys does a BFS backwards from gatewayID and returns the union of
// formSchema field keys from all reachable form nodes.
func collectUpstreamFormKeys(gatewayID string, nodeMap map[string]*WFNode, inEdges map[string][]*WFEdge, formFieldsByNode map[string]map[string]bool) map[string]bool {
	visited := map[string]bool{gatewayID: true}
	queue := []string{gatewayID}
	result := make(map[string]bool)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range inEdges[current] {
			src := edge.Source
			if visited[src] {
				continue
			}
			visited[src] = true
			if keys, ok := formFieldsByNode[src]; ok {
				for k := range keys {
					result[k] = true
				}
			}
			queue = append(queue, src)
		}
	}
	return result
}

// checkConditionFormRefs recursively checks if form.xxx references in a condition
// exist in the provided upstream keys set.
func checkConditionFormRefs(cond GatewayCondition, nodeID, edgeID string, upstreamKeys map[string]bool) []ValidationError {
	var errs []ValidationError
	if cond.Logic != "" {
		for _, sub := range cond.Conditions {
			errs = append(errs, checkConditionFormRefs(sub, nodeID, edgeID, upstreamKeys)...)
		}
		return errs
	}
	if strings.HasPrefix(cond.Field, "form.") {
		key := strings.TrimPrefix(cond.Field, "form.")
		if !upstreamKeys[key] {
			errs = append(errs, ValidationError{
				NodeID:  nodeID,
				EdgeID:  edgeID,
				Level:   "warning",
				Message: fmt.Sprintf("排他网关 %s 的条件引用了 %s，但上游 form 节点的 formSchema 中未找到字段 %s", nodeID, cond.Field, key),
			})
		}
	}
	return errs
}

func validateHumanNodeParticipants(n *WFNode) []ValidationError {
	nd, err := ParseNodeData(n.Data)
	if err != nil {
		return []ValidationError{{
			NodeID:  n.ID,
			Level:   "error",
			Message: fmt.Sprintf("人工节点 %s 数据解析失败: %v", n.ID, err),
		}}
	}
	if len(nd.Participants) == 0 {
		participantHint := `position_department 必须使用 {"type":"position_department","department_code":"部门编码","position_code":"岗位编码"}`
		if n.Type == NodeForm {
			participantHint = `表单填写节点通常由申请人处理，可使用 {"type":"requester"}`
		}
		return []ValidationError{{
			NodeID:  n.ID,
			Level:   "error",
			Message: fmt.Sprintf("人工节点 %s 必须配置处理人：在 data.participants 中按协作规范配置非空数组；%s", humanNodeRef(n.ID, nd.Label), participantHint),
		}}
	}

	var errs []ValidationError
	for i, p := range nd.Participants {
		switch p.Type {
		case "user", "position", "department":
			if p.Value == "" {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("人工节点 %s 的第 %d 个处理人缺少 value：user/position/department 类型必须在 participants 元素中配置 value", humanNodeRef(n.ID, nd.Label), i+1),
				})
			}
		case "position_department":
			if p.PositionCode == "" || p.DepartmentCode == "" {
				errs = append(errs, ValidationError{
					NodeID:  n.ID,
					Level:   "error",
					Message: fmt.Sprintf("人工节点 %s 的第 %d 个处理人必须同时配置 position_code 和 department_code：例如 {\"type\":\"position_department\",\"department_code\":\"it\",\"position_code\":\"network_admin\"}", humanNodeRef(n.ID, nd.Label), i+1),
				})
			}
		case "requester", "requester_manager":
		default:
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Level:   "error",
				Message: fmt.Sprintf("人工节点 %s 的第 %d 个处理人类型 %q 不支持", n.ID, i+1, p.Type),
			})
		}
	}
	return errs
}

func humanNodeRef(id string, label string) string {
	if label == "" {
		return id
	}
	return fmt.Sprintf("%s（%s）", id, label)
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

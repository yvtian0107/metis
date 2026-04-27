package engine

import (
	"encoding/json"
	"fmt"
)

func buildWorkflowContext(workflowJSON string, completed *activityModel) map[string]any {
	if workflowJSON == "" {
		return nil
	}

	def, err := ParseWorkflowDef(json.RawMessage(workflowJSON))
	if err != nil || len(def.Nodes) == 0 {
		return nil
	}

	nodeMap, outEdges := def.BuildMaps()
	ctx := map[string]any{
		"kind":    "ai_generated_workflow_blueprint",
		"summary": extractWorkflowHints(workflowJSON),
		"policy": []string{
			"协作规范是核心事实源，workflow_json 是辅助理解协作规范的结构化背景。",
			"当协作规范与 workflow_json 冲突时，必须以协作规范为准。",
			"本轮决策必须解释刚完成活动与 workflow_json 中节点、边、条件的关系。",
			"人工节点被 rejected 后，必须按协作规范定义的恢复路径处理；协作规范未显式定义补充信息或返工路径时，不得退回申请人补充；没有新证据时不得重复创建刚被驳回的同一处理任务。",
		},
	}

	humanSteps := make([]map[string]any, 0)
	for i := range def.Nodes {
		node := &def.Nodes[i]
		if !isHumanActivityType(node.Type) {
			continue
		}
		humanSteps = append(humanSteps, workflowNodeFact(node, nodeMap, outEdges[node.ID]))
	}
	if len(humanSteps) > 0 {
		ctx["human_steps"] = humanSteps
	}

	if completed == nil {
		return ctx
	}

	completedFact := map[string]any{
		"id":        completed.ID,
		"type":      completed.ActivityType,
		"name":      completed.Name,
		"node_id":   completed.NodeID,
		"outcome":   completed.TransitionOutcome,
		"satisfied": !isHumanActivityType(completed.ActivityType) || isPositiveActivityOutcome(completed.TransitionOutcome),
	}
	if completed.DecisionReasoning != "" {
		completedFact["operator_opinion"] = completed.DecisionReasoning
	}
	if isHumanActivityType(completed.ActivityType) && !isPositiveActivityOutcome(completed.TransitionOutcome) {
		completedFact["requires_recovery_decision"] = true
	}
	if completed.AIDecision != "" {
		var source any
		if err := json.Unmarshal([]byte(completed.AIDecision), &source); err == nil {
			completedFact["source_decision"] = source
		}
	}
	ctx["completed_activity"] = completedFact

	if completed.NodeID != "" {
		if node, ok := nodeMap[completed.NodeID]; ok {
			relatedStep := workflowNodeFact(node, nodeMap, outEdges[node.ID])
			// Attach outcome-specific edge target for precise path guidance
			if isPositiveActivityOutcome(completed.TransitionOutcome) {
				for _, edge := range outEdges[node.ID] {
					if edge.Data.Outcome == "approved" {
						if target, ok := nodeMap[edge.Target]; ok {
							relatedStep["approved_edge_target"] = map[string]any{
								"node_id": target.ID,
								"label":   workflowNodeLabel(target),
								"type":    target.Type,
							}
						}
						break
					}
				}
			} else if isHumanActivityType(completed.ActivityType) {
				for _, edge := range outEdges[node.ID] {
					if edge.Data.Outcome == "rejected" {
						if target, ok := nodeMap[edge.Target]; ok {
							relatedStep["rejected_edge_target"] = map[string]any{
								"node_id": target.ID,
								"label":   workflowNodeLabel(target),
								"type":    target.Type,
							}
						}
						break
					}
				}
			}
			ctx["related_step"] = relatedStep
		}
	} else {
		ctx["related_step_note"] = "completed_activity 缺少 node_id；请结合 source_decision、activity_history 和 human_steps 判断它对应的 workflow_json 节点。"
	}

	return ctx
}

func workflowNodeFact(node *WFNode, nodeMap map[string]*WFNode, edges []*WFEdge) map[string]any {
	data, _ := ParseNodeData(node.Data)
	label := node.Type
	if data != nil && data.Label != "" {
		label = data.Label
	}

	fact := map[string]any{
		"id":    node.ID,
		"type":  node.Type,
		"label": label,
	}
	if data != nil && len(data.Participants) > 0 {
		participants := make([]map[string]any, 0, len(data.Participants))
		for _, p := range data.Participants {
			item := map[string]any{"type": p.Type}
			if p.Value != "" {
				item["value"] = p.Value
			}
			if p.PositionCode != "" {
				item["position_code"] = p.PositionCode
			}
			if p.DepartmentCode != "" {
				item["department_code"] = p.DepartmentCode
			}
			participants = append(participants, item)
		}
		fact["participants"] = participants
	}

	outgoing := make([]map[string]any, 0, len(edges))
	for _, edge := range edges {
		item := map[string]any{
			"id":     edge.ID,
			"target": edge.Target,
		}
		if target, ok := nodeMap[edge.Target]; ok {
			item["target_label"] = workflowNodeLabel(target)
			item["target_type"] = target.Type
		}
		if edge.Data.Outcome != "" {
			item["outcome"] = edge.Data.Outcome
		}
		if edge.Data.Default {
			item["default"] = true
		}
		if edge.Data.Condition != nil {
			item["condition"] = workflowConditionFact(*edge.Data.Condition)
		}
		outgoing = append(outgoing, item)
	}
	if len(outgoing) > 0 {
		fact["outgoing_edges"] = outgoing
	}
	return fact
}

func workflowNodeLabel(node *WFNode) string {
	data, _ := ParseNodeData(node.Data)
	if data != nil && data.Label != "" {
		return data.Label
	}
	return node.Type
}

func workflowConditionFact(cond GatewayCondition) map[string]any {
	fact := map[string]any{
		"field":    cond.Field,
		"operator": cond.Operator,
		"value":    cond.Value,
	}
	if cond.EdgeID != "" {
		fact["edge_id"] = cond.EdgeID
	}
	if cond.Logic != "" {
		fact["logic"] = cond.Logic
	}
	if len(cond.Conditions) > 0 {
		children := make([]map[string]any, 0, len(cond.Conditions))
		for _, child := range cond.Conditions {
			children = append(children, workflowConditionFact(child))
		}
		fact["conditions"] = children
	}
	fact["description"] = fmt.Sprintf("%s %s %v", cond.Field, cond.Operator, cond.Value)
	return fact
}

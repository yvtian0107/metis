package engine

import (
	"encoding/json"
	"strings"
)

type branchTarget struct {
	NodeID string
	Label  string
	Type   string
}

type branchContract struct {
	GatewayNodeID            string
	GatewayLabel             string
	BranchNodeID             string
	BranchLabel              string
	BranchNodeType           string
	EntryEdgeID              string
	EntryCondition           *GatewayCondition
	DefaultEdge              bool
	ApprovedTarget           *branchTarget
	RejectedTarget           *branchTarget
	BranchTerminalCompletion bool
	BranchRejectedTerminal   bool
}

func buildBranchInsights(workflowJSON, collaborationSpec string, formData map[string]any, currentNodeID string, currentActivityName string, completed *activityModel) map[string]any {
	def, err := ParseWorkflowDef(json.RawMessage(workflowJSON))
	if err != nil || len(def.Nodes) == 0 {
		return nil
	}

	nodeMap, outEdges := def.BuildMaps()
	contracts := deriveBranchContracts(def, nodeMap, outEdges, collaborationSpec)
	if len(contracts) == 0 {
		return nil
	}

	selected := selectBranchContract(contracts, formData, currentNodeID, currentActivityName, completed)

	branchFacts := make([]map[string]any, 0, len(contracts))
	for _, contract := range contracts {
		branchFacts = append(branchFacts, branchContractFact(contract))
	}

	result := map[string]any{
		"branch_contracts": branchFacts,
	}
	if selected == nil {
		return result
	}

	selectedFact := branchContractFact(*selected)
	result["selected_branch"] = selectedFact
	result["active_branch_contract"] = selectedFact
	result["branch_reasoning_basis"] = buildBranchReasoningBasis(*selected, formData, currentNodeID, currentActivityName, completed)
	result["current_branch_node_id"] = selected.BranchNodeID
	result["allowed_next_branch_nodes"] = allowedNextBranchNodes(*selected, completed)
	result["completion_contract"] = buildCompletionContract(*selected, completed)
	return result
}

func deriveBranchContracts(def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge, collaborationSpec string) []branchContract {
	contracts := make([]branchContract, 0)
	terminalPhrase := collaborationSpecImpliesDirectCompletion(collaborationSpec)
	for i := range def.Nodes {
		gateway := &def.Nodes[i]
		if gateway.Type != NodeExclusive {
			continue
		}
		gatewayLabel := workflowNodeLabel(gateway)
		for _, edge := range outEdges[gateway.ID] {
			targetNode, ok := nodeMap[edge.Target]
			if !ok {
				continue
			}
			approved := outcomeTarget(nodeMap, outEdges[targetNode.ID], ActivityApproved)
			rejected := outcomeTarget(nodeMap, outEdges[targetNode.ID], ActivityRejected)
			contracts = append(contracts, branchContract{
				GatewayNodeID:            gateway.ID,
				GatewayLabel:             gatewayLabel,
				BranchNodeID:             targetNode.ID,
				BranchLabel:              workflowNodeLabel(targetNode),
				BranchNodeType:           targetNode.Type,
				EntryEdgeID:              edge.ID,
				EntryCondition:           edge.Data.Condition,
				DefaultEdge:              edge.Data.Default,
				ApprovedTarget:           approved,
				RejectedTarget:           rejected,
				BranchTerminalCompletion: terminalPhrase || isEndTarget(approved),
				BranchRejectedTerminal:   terminalPhrase || isEndTarget(rejected),
			})
		}
	}
	return contracts
}

func selectBranchContract(contracts []branchContract, formData map[string]any, currentNodeID string, currentActivityName string, completed *activityModel) *branchContract {
	if currentNodeID != "" {
		for i := range contracts {
			if contracts[i].BranchNodeID == currentNodeID {
				return &contracts[i]
			}
		}
	}
	currentName := normalizeActivityText(currentActivityName)
	if currentName != "" {
		for i := range contracts {
			branchName := normalizeActivityText(contracts[i].BranchLabel)
			if branchName != "" && (branchName == currentName || strings.Contains(branchName, currentName) || strings.Contains(currentName, branchName)) {
				return &contracts[i]
			}
		}
	}
	if completed != nil {
		if completed.NodeID != "" {
			for i := range contracts {
				if contracts[i].BranchNodeID == completed.NodeID {
					return &contracts[i]
				}
			}
		}
		completedName := normalizeActivityText(completed.Name)
		if completedName != "" {
			for i := range contracts {
				branchName := normalizeActivityText(contracts[i].BranchLabel)
				if branchName != "" && (branchName == completedName || strings.Contains(branchName, completedName) || strings.Contains(completedName, branchName)) {
					return &contracts[i]
				}
			}
		}
	}
	if len(formData) == 0 {
		return nil
	}

	ctx := evalContext{}
	for key, value := range formData {
		ctx["form."+key] = value
	}
	for i := range contracts {
		contract := &contracts[i]
		if contract.EntryCondition != nil && evaluateCondition(*contract.EntryCondition, ctx) {
			return contract
		}
	}
	for i := range contracts {
		if contracts[i].DefaultEdge {
			return &contracts[i]
		}
	}
	return nil
}

func branchContractFact(contract branchContract) map[string]any {
	fact := map[string]any{
		"gateway_node_id":               contract.GatewayNodeID,
		"gateway_label":                 contract.GatewayLabel,
		"branch_node_id":                contract.BranchNodeID,
		"branch_label":                  contract.BranchLabel,
		"branch_node_type":              contract.BranchNodeType,
		"entry_edge_id":                 contract.EntryEdgeID,
		"branch_terminal_on_completion": contract.BranchTerminalCompletion,
		"branch_rejected_terminal":      contract.BranchRejectedTerminal,
	}
	if contract.DefaultEdge {
		fact["default_edge"] = true
	}
	if contract.EntryCondition != nil {
		fact["entry_condition"] = workflowConditionFact(*contract.EntryCondition)
	}
	if contract.ApprovedTarget != nil {
		fact["approved_target"] = map[string]any{
			"node_id": contract.ApprovedTarget.NodeID,
			"label":   contract.ApprovedTarget.Label,
			"type":    contract.ApprovedTarget.Type,
		}
	}
	if contract.RejectedTarget != nil {
		fact["rejected_target"] = map[string]any{
			"node_id": contract.RejectedTarget.NodeID,
			"label":   contract.RejectedTarget.Label,
			"type":    contract.RejectedTarget.Type,
		}
	}
	return fact
}

func buildBranchReasoningBasis(contract branchContract, formData map[string]any, currentNodeID string, currentActivityName string, completed *activityModel) []string {
	basis := []string{
		"业务分支必须以协作规范和 workflow_json 的排他分支定义为准。",
		"一旦命中当前业务分支，后续不能因为别的岗位也相关就横跳到其他业务分支。",
	}
	if contract.EntryCondition != nil {
		basis = append(basis, "当前分支入口条件为 "+conditionDescription(*contract.EntryCondition)+"。")
	}
	if currentNodeID == contract.BranchNodeID {
		basis = append(basis, "当前活动节点已经位于该业务分支的处理节点。")
	} else if normalizeActivityText(currentActivityName) != "" && strings.Contains(normalizeActivityText(contract.BranchLabel), normalizeActivityText(currentActivityName)) {
		basis = append(basis, "当前活动名称与该业务分支处理节点语义匹配。")
	} else if completed != nil && completed.NodeID == contract.BranchNodeID {
		basis = append(basis, "刚完成的人工活动属于该业务分支。")
	} else if len(formData) > 0 {
		basis = append(basis, "表单数据命中了该业务分支的入口条件。")
	}
	if contract.BranchTerminalCompletion {
		basis = append(basis, "协作规范/流程定义表明该分支处理完成后直接结束流程。")
	}
	if contract.BranchRejectedTerminal {
		basis = append(basis, "该分支被驳回后也应在当前分支内收敛为终态，而不是切换到其他业务分支。")
	}
	return basis
}

func allowedNextBranchNodes(contract branchContract, completed *activityModel) []string {
	if completed == nil || completed.NodeID == "" || completed.NodeID != contract.BranchNodeID {
		return []string{contract.BranchNodeID}
	}
	targets := make([]string, 0, 1)
	if isPositiveActivityOutcome(completed.TransitionOutcome) {
		if contract.ApprovedTarget != nil && contract.ApprovedTarget.NodeID != "" {
			targets = append(targets, contract.ApprovedTarget.NodeID)
		}
	} else if contract.RejectedTarget != nil && contract.RejectedTarget.NodeID != "" {
		targets = append(targets, contract.RejectedTarget.NodeID)
	}
	return targets
}

func buildCompletionContract(contract branchContract, completed *activityModel) map[string]any {
	fact := map[string]any{
		"branch_node_id":                contract.BranchNodeID,
		"branch_label":                  contract.BranchLabel,
		"branch_terminal_on_completion": contract.BranchTerminalCompletion,
		"branch_rejected_terminal":      contract.BranchRejectedTerminal,
	}
	if contract.ApprovedTarget != nil {
		fact["approved_target_node_id"] = contract.ApprovedTarget.NodeID
		fact["approved_target_type"] = contract.ApprovedTarget.Type
		fact["approved_target_label"] = contract.ApprovedTarget.Label
		fact["can_complete_after_current_activity"] = contract.ApprovedTarget.Type == NodeEnd || contract.BranchTerminalCompletion
	}
	if contract.RejectedTarget != nil {
		fact["rejected_target_node_id"] = contract.RejectedTarget.NodeID
		fact["rejected_target_type"] = contract.RejectedTarget.Type
		fact["rejected_target_label"] = contract.RejectedTarget.Label
		fact["can_complete_after_rejection"] = contract.RejectedTarget.Type == NodeEnd || contract.BranchRejectedTerminal
	}
	if completed != nil {
		fact["completed_activity_node_id"] = completed.NodeID
		fact["completed_activity_outcome"] = completed.TransitionOutcome
	}
	return fact
}

func outcomeTarget(nodeMap map[string]*WFNode, edges []*WFEdge, outcome string) *branchTarget {
	for _, edge := range edges {
		if edge.Data.Outcome != outcome {
			continue
		}
		if target, ok := nodeMap[edge.Target]; ok {
			return &branchTarget{
				NodeID: target.ID,
				Label:  workflowNodeLabel(target),
				Type:   target.Type,
			}
		}
		return &branchTarget{NodeID: edge.Target}
	}
	return nil
}

func isEndTarget(target *branchTarget) bool {
	return target != nil && target.Type == NodeEnd
}

func collaborationSpecImpliesDirectCompletion(spec string) bool {
	text := strings.TrimSpace(spec)
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"处理完成后直接结束流程",
		"处理任务完成后直接结束流程",
		"完成后直接结束流程",
		"处理完成后直接结束",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func conditionDescription(cond GatewayCondition) string {
	fact := workflowConditionFact(cond)
	if desc, ok := fact["description"].(string); ok && strings.TrimSpace(desc) != "" {
		return desc
	}
	return ""
}

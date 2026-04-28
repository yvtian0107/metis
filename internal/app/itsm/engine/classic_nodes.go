package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- Node handlers ---

func (e *ClassicEngine) handleEnd(tx *gorm.DB, token *executionTokenModel, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	// Create completed activity for the end node
	act := &activityModel{
		TicketID:          token.TicketID,
		TokenID:           &token.ID,
		Name:              labelOrDefault(data, "结束"),
		ActivityType:      NodeEnd,
		Status:            ActivityCompleted,
		NodeID:            node.ID,
		TransitionOutcome: "completed",
		StartedAt:         &now,
		FinishedAt:        &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	status, outcome, err := resolveClassicCompletionStatus(tx, token.TicketID)
	if err != nil {
		return err
	}

	// Complete the token
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenCompleted)

	// Child token (parallel branch) — only complete this branch, check join
	if token.ParentTokenID != nil {
		// Check if all siblings are done → reactivate parent
		var remaining int64
		tx.Model(&executionTokenModel{}).
			Where("parent_token_id = ? AND status IN ? AND token_type != ?",
				*token.ParentTokenID, []string{TokenActive, TokenWaiting}, TokenBoundary).
			Count(&remaining)

		if remaining == 0 {
			// All siblings done — reactivate parent token
			var parentToken executionTokenModel
			if err := tx.First(&parentToken, *token.ParentTokenID).Error; err != nil {
				return fmt.Errorf("parent token %d not found: %w", *token.ParentTokenID, err)
			}
			tx.Model(&executionTokenModel{}).Where("id = ?", parentToken.ID).Update("status", TokenCompleted)
			// If the parent is also a child, recurse upwards
			if parentToken.ParentTokenID != nil {
				return e.handleEnd(tx, &parentToken, operatorID, node, data)
			}
			// Parent is root — complete ticket
			if err := tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Updates(map[string]any{
				"status":              status,
				"outcome":             outcome,
				"finished_at":         now,
				"current_activity_id": act.ID,
			}).Error; err != nil {
				return err
			}
			return e.recordTimeline(tx, token.TicketID, &act.ID, operatorID, "workflow_completed", "流程已完结")
		}

		// Other branches still running
		return nil
	}

	// Root token — complete the ticket
	if err := tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Updates(map[string]any{
		"status":              status,
		"outcome":             outcome,
		"finished_at":         now,
		"current_activity_id": act.ID,
	}).Error; err != nil {
		return err
	}

	return e.recordTimeline(tx, token.TicketID, &act.ID, operatorID, "workflow_completed", "流程已完结")
}

func resolveClassicCompletionStatus(tx *gorm.DB, ticketID uint) (string, string, error) {
	var activity activityModel
	err := tx.Where("ticket_id = ? AND activity_type IN ? AND status IN ?", ticketID,
		[]string{NodeApprove, NodeForm, NodeProcess}, CompletedActivityStatuses()).
		Order("finished_at DESC, id DESC").
		First(&activity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return TicketStatusCompleted, TicketOutcomeFulfilled, nil
	}
	if err != nil {
		return "", "", err
	}
	switch activity.TransitionOutcome {
	case ActivityRejected:
		return TicketStatusRejected, TicketOutcomeRejected, nil
	case ActivityApproved:
		return TicketStatusCompleted, TicketOutcomeApproved, nil
	default:
		return TicketStatusCompleted, TicketOutcomeFulfilled, nil
	}
}

func (e *ClassicEngine) handleForm(tx *gorm.DB, token *executionTokenModel, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()

	// Read inline formSchema from node data
	formSchema := string(data.FormSchema)

	act := &activityModel{
		TicketID:      token.TicketID,
		TokenID:       &token.ID,
		Name:          labelOrDefault(data, "表单填写"),
		ActivityType:  NodeForm,
		Status:        ActivityPending,
		NodeID:        node.ID,
		ExecutionMode: "single",
		FormSchema:    formSchema,
		StartedAt:     &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("current_activity_id", act.ID)

	return e.assignParticipants(tx, token.TicketID, act.ID, operatorID, data.Participants)
}

func (e *ClassicEngine) handleApprove(tx *gorm.DB, token *executionTokenModel, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()

	act := &activityModel{
		TicketID:      token.TicketID,
		TokenID:       &token.ID,
		Name:          labelOrDefault(data, "审批"),
		ActivityType:  NodeApprove,
		Status:        ActivityPending,
		NodeID:        node.ID,
		ExecutionMode: "single",
		StartedAt:     &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("current_activity_id", act.ID)

	return e.assignParticipants(tx, token.TicketID, act.ID, operatorID, data.Participants)
}

func (e *ClassicEngine) handleProcess(tx *gorm.DB, token *executionTokenModel, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()

	// Read inline formSchema from node data
	formSchema := string(data.FormSchema)

	act := &activityModel{
		TicketID:      token.TicketID,
		TokenID:       &token.ID,
		Name:          labelOrDefault(data, "处理"),
		ActivityType:  NodeProcess,
		Status:        ActivityPending,
		NodeID:        node.ID,
		ExecutionMode: "single",
		FormSchema:    formSchema,
		StartedAt:     &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("current_activity_id", act.ID)

	return e.assignParticipants(tx, token.TicketID, act.ID, operatorID, data.Participants)
}

func (e *ClassicEngine) handleAction(tx *gorm.DB, token *executionTokenModel, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	act := &activityModel{
		TicketID:     token.TicketID,
		TokenID:      &token.ID,
		Name:         labelOrDefault(data, "动作执行"),
		ActivityType: NodeAction,
		Status:       ActivityInProgress,
		NodeID:       node.ID,
		StartedAt:    &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("current_activity_id", act.ID)

	// Submit async task
	if e.scheduler != nil && data.ActionID != 0 {
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":   token.TicketID,
			"activity_id": act.ID,
			"action_id":   data.ActionID,
		})
		if err := submitTaskInTx(e.scheduler, tx, "itsm-action-execute", payload); err != nil {
			slog.Error("failed to submit action task", "error", err, "ticketID", token.TicketID)
			// Record timeline but don't fail the workflow
			e.recordTimeline(tx, token.TicketID, &act.ID, 0, "warning", "动作任务提交失败: "+err.Error())
		}
	}

	e.recordTimeline(tx, token.TicketID, &act.ID, operatorID, "action_started", "动作节点开始执行")
	return nil
}

func (e *ClassicEngine) handleExclusive(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	// Load ticket for field evaluation
	var ticket ticketModel
	if err := tx.First(&ticket, token.TicketID).Error; err != nil {
		return err
	}

	// Load latest completed activity's form data for condition evaluation
	var latestActivity activityModel
	tx.Where("ticket_id = ? AND status IN ? AND form_data IS NOT NULL AND form_data != ''", token.TicketID, CompletedActivityStatuses()).
		Order("id DESC").First(&latestActivity)

	evalCtx := buildEvalContext(tx, &ticket, &latestActivity)

	edges := outEdges[node.ID]
	var matchedEdge *WFEdge
	var defaultEdge *WFEdge

	for _, edge := range edges {
		if edge.Data.Default {
			defaultEdge = edge
			continue
		}
		if edge.Data.Condition != nil && evaluateCondition(*edge.Data.Condition, evalCtx) {
			matchedEdge = edge
			break
		}
	}

	// Also check conditions defined in node data
	if matchedEdge == nil {
		for _, cond := range data.Conditions {
			if evaluateCondition(cond, evalCtx) {
				// Find the edge this condition maps to
				for _, edge := range edges {
					if edge.ID == cond.EdgeID {
						matchedEdge = edge
						break
					}
				}
				if matchedEdge != nil {
					break
				}
			}
		}
	}

	if matchedEdge == nil {
		matchedEdge = defaultEdge
	}

	if matchedEdge == nil {
		// No matching edge — mark ticket as failed
		tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("status", "failed")
		e.recordTimeline(tx, token.TicketID, nil, 0, "error", fmt.Sprintf("排他网关节点 %s 无匹配条件且无默认边", node.ID))
		return fmt.Errorf("exclusive gateway %s: no matching condition and no default edge", node.ID)
	}

	targetNode, ok := nodeMap[matchedEdge.Target]
	if !ok {
		return fmt.Errorf("exclusive gateway target %q not found", matchedEdge.Target)
	}

	return e.processNode(ctx, tx, def, nodeMap, outEdges, token, operatorID, targetNode, depth+1)
}

func (e *ClassicEngine) handleWait(tx *gorm.DB, token *executionTokenModel, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	status := ActivityPending // signal mode
	if data.WaitMode == "timer" {
		status = ActivityInProgress
	}

	act := &activityModel{
		TicketID:     token.TicketID,
		TokenID:      &token.ID,
		Name:         labelOrDefault(data, "等待"),
		ActivityType: NodeWait,
		Status:       status,
		NodeID:       node.ID,
		StartedAt:    &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("current_activity_id", act.ID)

	if data.WaitMode == "timer" && e.scheduler != nil {
		dur := parseDuration(data.Duration)
		executeAfter := now.Add(dur)
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":     token.TicketID,
			"activity_id":   act.ID,
			"execute_after": executeAfter.Format(time.RFC3339),
		})
		if err := submitTaskInTx(e.scheduler, tx, "itsm-wait-timer", payload); err != nil {
			slog.Error("failed to submit wait timer task", "error", err, "ticketID", token.TicketID)
		}
		e.recordTimeline(tx, token.TicketID, &act.ID, operatorID, "wait_timer_started", fmt.Sprintf("等待定时器已设置: %s", data.Duration))
	} else {
		e.recordTimeline(tx, token.TicketID, &act.ID, operatorID, "wait_signal", "等待外部信号")
	}

	return nil
}

// --- Parallel / Inclusive Gateway Handlers (④ itsm-gateway-parallel) ---

func (e *ClassicEngine) handleParallelFork(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	edges := outEdges[node.ID]
	if len(edges) < 2 {
		return fmt.Errorf("%w: parallel fork %s 至少需要两条出边", ErrGatewayNoOutEdge, node.ID)
	}

	// Parent token enters waiting state
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenWaiting)
	token.Status = TokenWaiting

	e.recordTimeline(tx, token.TicketID, nil, operatorID, "parallel_fork",
		fmt.Sprintf("并行网关分裂: %d 条分支", len(edges)))

	// Create child tokens and process each branch
	for _, edge := range edges {
		targetNode, ok := nodeMap[edge.Target]
		if !ok {
			return fmt.Errorf("parallel fork target %q not found", edge.Target)
		}

		childToken := &executionTokenModel{
			TicketID:      token.TicketID,
			ParentTokenID: &token.ID,
			NodeID:        node.ID,
			Status:        TokenActive,
			TokenType:     TokenParallel,
			ScopeID:       token.ScopeID,
		}
		if err := tx.Create(childToken).Error; err != nil {
			return fmt.Errorf("failed to create child token for parallel fork: %w", err)
		}

		if err := e.processNode(ctx, tx, def, nodeMap, outEdges, childToken, operatorID, targetNode, depth+1); err != nil {
			return err
		}
	}

	return nil
}

func (e *ClassicEngine) handleParallelJoin(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	return e.tryCompleteJoin(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, depth)
}

func (e *ClassicEngine) handleInclusiveFork(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	edges := outEdges[node.ID]
	if len(edges) < 2 {
		return fmt.Errorf("%w: inclusive fork %s 至少需要两条出边", ErrGatewayNoOutEdge, node.ID)
	}

	// Load context for condition evaluation
	var ticket ticketModel
	if err := tx.First(&ticket, token.TicketID).Error; err != nil {
		return err
	}
	var latestActivity activityModel
	tx.Where("ticket_id = ? AND status IN ? AND form_data IS NOT NULL AND form_data != ''", token.TicketID, CompletedActivityStatuses()).
		Order("id DESC").First(&latestActivity)
	evalCtx := buildEvalContext(tx, &ticket, &latestActivity)

	// Evaluate conditions — collect matching edges
	var matchedEdges []*WFEdge
	var defaultEdge *WFEdge
	for _, edge := range edges {
		if edge.Data.Default {
			defaultEdge = edge
			continue
		}
		if edge.Data.Condition != nil && evaluateCondition(*edge.Data.Condition, evalCtx) {
			matchedEdges = append(matchedEdges, edge)
		}
	}

	// If no conditions matched, fall back to default edge
	if len(matchedEdges) == 0 {
		if defaultEdge == nil {
			tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("status", "failed")
			e.recordTimeline(tx, token.TicketID, nil, 0, "error",
				fmt.Sprintf("包含网关 fork 节点 %s 无匹配条件且无默认边", node.ID))
			return fmt.Errorf("inclusive fork %s: no matching condition and no default edge", node.ID)
		}
		matchedEdges = append(matchedEdges, defaultEdge)
	} else if defaultEdge != nil {
		// Default edge is also activated when other conditions match (inclusive semantics)
		matchedEdges = append(matchedEdges, defaultEdge)
	}

	// Parent token enters waiting state
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenWaiting)
	token.Status = TokenWaiting

	e.recordTimeline(tx, token.TicketID, nil, operatorID, "inclusive_fork",
		fmt.Sprintf("包含网关分裂: %d/%d 条分支激活", len(matchedEdges), len(edges)))

	// Create child tokens for matched edges
	for _, edge := range matchedEdges {
		targetNode, ok := nodeMap[edge.Target]
		if !ok {
			return fmt.Errorf("inclusive fork target %q not found", edge.Target)
		}

		childToken := &executionTokenModel{
			TicketID:      token.TicketID,
			ParentTokenID: &token.ID,
			NodeID:        node.ID,
			Status:        TokenActive,
			TokenType:     TokenParallel,
			ScopeID:       token.ScopeID,
		}
		if err := tx.Create(childToken).Error; err != nil {
			return fmt.Errorf("failed to create child token for inclusive fork: %w", err)
		}

		if err := e.processNode(ctx, tx, def, nodeMap, outEdges, childToken, operatorID, targetNode, depth+1); err != nil {
			return err
		}
	}

	return nil
}

func (e *ClassicEngine) handleInclusiveJoin(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	return e.tryCompleteJoin(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, depth)
}

// tryCompleteJoin completes the current child token and checks if all siblings are done.
// If all siblings completed, it reactivates the parent token and continues past the join node.
func (e *ClassicEngine) tryCompleteJoin(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	joinNode *WFNode, depth int,
) error {
	// Complete this child token
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenCompleted)
	token.Status = TokenCompleted

	if token.ParentTokenID == nil {
		// Should not happen — join node reached by root token
		return fmt.Errorf("join node %s reached by root token (no parent)", joinNode.ID)
	}

	// Lock the parent token first, then count remaining siblings (serializes concurrent join attempts)
	var parentToken executionTokenModel
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&parentToken, *token.ParentTokenID).Error; err != nil {
		return fmt.Errorf("parent token %d not found: %w", *token.ParentTokenID, err)
	}

	// Count remaining active/waiting siblings
	var remaining int64
	tx.Model(&executionTokenModel{}).
		Where("parent_token_id = ? AND status IN ? AND token_type != ?",
			*token.ParentTokenID, []string{TokenActive, TokenWaiting}, TokenBoundary).
		Count(&remaining)

	if remaining > 0 {
		// Other branches still running — stop here
		return nil
	}

	// All siblings completed — reactivate parent token

	tx.Model(&executionTokenModel{}).Where("id = ?", parentToken.ID).Update("status", TokenActive)
	parentToken.Status = TokenActive

	e.recordTimeline(tx, token.TicketID, nil, operatorID, "gateway_join",
		fmt.Sprintf("网关合并完成，所有分支已汇聚于节点 %s", joinNode.ID))

	// Continue past the join node
	edges := outEdges[joinNode.ID]
	if len(edges) == 0 {
		return fmt.Errorf("join node %s has no outgoing edge", joinNode.ID)
	}

	targetNode, ok := nodeMap[edges[0].Target]
	if !ok {
		return fmt.Errorf("join node target %q not found", edges[0].Target)
	}

	return e.processNode(ctx, tx, def, nodeMap, outEdges, &parentToken, operatorID, targetNode, depth+1)
}

// --- Boundary Event Handlers (⑤b itsm-boundary-events) ---

// attachBoundaryEvents scans for b_timer boundary nodes attached to the given host node,
// creates suspended boundary tokens, and submits itsm-boundary-timer scheduler tasks.
func (e *ClassicEngine) attachBoundaryEvents(
	tx *gorm.DB, def *WorkflowDef, token *executionTokenModel, node *WFNode,
) error {
	boundaryMap := def.BuildBoundaryMap()
	boundaries := boundaryMap[node.ID]
	if len(boundaries) == 0 {
		return nil
	}

	for _, bNode := range boundaries {
		if bNode.Type != NodeBTimer {
			continue // b_error tokens are created on-demand at failure time
		}

		bData, err := ParseNodeData(bNode.Data)
		if err != nil {
			continue
		}

		// Create suspended boundary token
		bToken := &executionTokenModel{
			TicketID:      token.TicketID,
			ParentTokenID: &token.ID,
			NodeID:        bNode.ID,
			Status:        TokenSuspended,
			TokenType:     TokenBoundary,
			ScopeID:       token.ScopeID,
		}
		if err := tx.Create(bToken).Error; err != nil {
			return fmt.Errorf("failed to create boundary token for %s: %w", bNode.ID, err)
		}

		// Submit boundary timer scheduler task
		if e.scheduler != nil {
			dur := parseDuration(bData.Duration)
			executeAfter := time.Now().Add(dur)
			payload, _ := json.Marshal(map[string]any{
				"ticket_id":         token.TicketID,
				"boundary_token_id": bToken.ID,
				"boundary_node_id":  bNode.ID,
				"host_token_id":     token.ID,
				"execute_after":     executeAfter.Format(time.RFC3339),
			})
			if err := submitTaskInTx(e.scheduler, tx, "itsm-boundary-timer", payload); err != nil {
				slog.Error("failed to submit boundary timer task", "error", err, "ticketID", token.TicketID)
			}
		}

		e.recordTimeline(tx, token.TicketID, nil, 0, "boundary_timer_set",
			fmt.Sprintf("边界定时器已设置: %s (节点 %s)", bData.Duration, labelOrDefault(bData, bNode.ID)))
	}

	return nil
}

// triggerBoundaryError handles an action failure when a b_error boundary node exists.
// It cancels the host activity/token, creates an active boundary token, and continues
// from the b_error node's outgoing edge.
func (e *ClassicEngine) triggerBoundaryError(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	ticketID, activityID uint, hostToken *executionTokenModel, bErrorNode *WFNode,
) error {
	now := time.Now()

	// Cancel host activity
	tx.Model(&activityModel{}).
		Where("id = ?", activityID).
		Updates(map[string]any{
			"status":      ActivityCancelled,
			"finished_at": now,
		})

	// Cancel host token
	tx.Model(&executionTokenModel{}).
		Where("id = ?", hostToken.ID).
		Update("status", TokenCancelled)

	// Create active boundary token
	bToken := &executionTokenModel{
		TicketID:      ticketID,
		ParentTokenID: &hostToken.ID,
		NodeID:        bErrorNode.ID,
		Status:        TokenActive,
		TokenType:     TokenBoundary,
		ScopeID:       hostToken.ScopeID,
	}
	if err := tx.Create(bToken).Error; err != nil {
		return fmt.Errorf("failed to create b_error boundary token: %w", err)
	}

	e.recordTimeline(tx, ticketID, &activityID, 0, "boundary_error_fired",
		"动作执行失败，已触发错误边界事件")

	// Continue from b_error node's outgoing edge
	edges := outEdges[bErrorNode.ID]
	if len(edges) == 0 {
		return fmt.Errorf("b_error node %s has no outgoing edge", bErrorNode.ID)
	}

	targetNode, ok := nodeMap[edges[0].Target]
	if !ok {
		return fmt.Errorf("b_error target %q not found", edges[0].Target)
	}

	return e.processNode(ctx, tx, def, nodeMap, outEdges, bToken, 0, targetNode, 0)
}

// --- Script Node Handler (⑤a itsm-script-task) ---

func (e *ClassicEngine) handleScript(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	// Build expression environment from process variables + ticket fields
	env := buildScriptEnv(tx, token.TicketID, token.ScopeID)

	// Execute each assignment in order
	for _, assign := range data.Assignments {
		if assign.Variable == "" || assign.Expression == "" {
			continue
		}

		result, err := evaluateExpression(assign.Expression, env)
		if err != nil {
			// Non-fatal: log warning and skip this assignment
			slog.Warn("script assignment eval failed",
				"ticketID", token.TicketID, "node", node.ID,
				"variable", assign.Variable, "expression", assign.Expression, "error", err)
			e.recordTimeline(tx, token.TicketID, nil, 0, "warning",
				fmt.Sprintf("脚本节点 %s 变量 %s 表达式求值失败: %v", node.ID, assign.Variable, err))
			continue
		}

		// Write result to process variable
		valueType := inferValueType(result)
		serialized := serializeVarValue(result)

		v := processVariableModel{
			TicketID:  token.TicketID,
			ScopeID:   token.ScopeID,
			Key:       assign.Variable,
			Value:     serialized,
			ValueType: valueType,
			Source:    fmt.Sprintf("script:%s", node.ID),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ticket_id"}, {Name: "scope_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "value_type", "source", "updated_at"}),
		}).Create(&v).Error; err != nil {
			return fmt.Errorf("script write variable %s: %w", assign.Variable, err)
		}

		// Update env so subsequent assignments can reference this variable
		env[assign.Variable] = result
	}

	e.recordTimeline(tx, token.TicketID, nil, operatorID, "script_executed",
		fmt.Sprintf("脚本节点 [%s] 执行完成，%d 个变量赋值", labelOrDefault(data, "脚本"), len(data.Assignments)))

	// Continue to next node
	edges := outEdges[node.ID]
	if len(edges) == 0 {
		return fmt.Errorf("script node %s has no outgoing edge", node.ID)
	}

	targetNode, ok := nodeMap[edges[0].Target]
	if !ok {
		return fmt.Errorf("script node target %q not found", edges[0].Target)
	}

	return e.processNode(ctx, tx, def, nodeMap, outEdges, token, operatorID, targetNode, depth+1)
}

// handleSubprocess executes an embedded subprocess by creating a subprocess token
// and recursively processing the subprocess's internal workflow.
func (e *ClassicEngine) handleSubprocess(
	ctx context.Context, tx *gorm.DB,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	if len(data.SubProcessDef) == 0 {
		return fmt.Errorf("subprocess node %s has no subprocess_def", node.ID)
	}

	subDef, err := ParseWorkflowDef(data.SubProcessDef)
	if err != nil {
		return fmt.Errorf("subprocess node %s: subprocess_def parse error: %w", node.ID, err)
	}

	subNodeMap, subOutEdges := subDef.BuildMaps()

	// Set parent token to waiting
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenWaiting)
	token.Status = TokenWaiting

	// Create subprocess token with isolated scope
	subToken := &executionTokenModel{
		TicketID:      token.TicketID,
		ParentTokenID: &token.ID,
		NodeID:        node.ID,
		Status:        TokenActive,
		TokenType:     TokenSubprocess,
		ScopeID:       node.ID, // variable scope isolation
	}
	if err := tx.Create(subToken).Error; err != nil {
		return fmt.Errorf("failed to create subprocess token for %s: %w", node.ID, err)
	}

	e.recordTimeline(tx, token.TicketID, nil, operatorID, "subprocess_started",
		fmt.Sprintf("子流程 [%s] 已启动", labelOrDefault(data, node.ID)))

	// Find start node in subprocess
	startNode, err := subDef.FindStartNode()
	if err != nil {
		return fmt.Errorf("subprocess %s: %w", node.ID, err)
	}

	startEdges := subOutEdges[startNode.ID]
	if len(startEdges) == 0 {
		return fmt.Errorf("subprocess %s: start node has no outgoing edge", node.ID)
	}

	targetNode, ok := subNodeMap[startEdges[0].Target]
	if !ok {
		return fmt.Errorf("subprocess %s: start node target %q not found", node.ID, startEdges[0].Target)
	}

	return e.processNode(ctx, tx, subDef, subNodeMap, subOutEdges, subToken, operatorID, targetNode, depth+1)
}

// completeSubprocess handles the end of a subprocess: completes the subprocess token,
// reactivates the parent token, and continues the parent flow past the subprocess node.
func (e *ClassicEngine) completeSubprocess(
	ctx context.Context, tx *gorm.DB,
	token *executionTokenModel, operatorID uint,
	node *WFNode, nodeData *NodeData, depth int,
) error {
	now := time.Now()

	// Create completed activity for the subprocess end node
	act := &activityModel{
		TicketID:          token.TicketID,
		TokenID:           &token.ID,
		Name:              labelOrDefault(nodeData, "子流程结束"),
		ActivityType:      NodeEnd,
		Status:            ActivityCompleted,
		NodeID:            node.ID,
		TransitionOutcome: "completed",
		StartedAt:         &now,
		FinishedAt:        &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	// Complete subprocess token
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenCompleted)

	if token.ParentTokenID == nil {
		return fmt.Errorf("subprocess token %d has no parent token", token.ID)
	}

	// Reactivate parent token
	var parentToken executionTokenModel
	if err := tx.First(&parentToken, *token.ParentTokenID).Error; err != nil {
		return fmt.Errorf("parent token %d not found: %w", *token.ParentTokenID, err)
	}

	tx.Model(&executionTokenModel{}).Where("id = ?", parentToken.ID).Update("status", TokenActive)
	parentToken.Status = TokenActive

	// Load main workflow to continue parent flow
	var ticket ticketModel
	if err := tx.First(&ticket, token.TicketID).Error; err != nil {
		return fmt.Errorf("ticket %d not found: %w", token.TicketID, err)
	}

	mainDef, err := ParseWorkflowDef(json.RawMessage(ticket.WorkflowJSON))
	if err != nil {
		return fmt.Errorf("main workflow parse error: %w", err)
	}
	mainNodeMap, mainOutEdges := mainDef.BuildMaps()

	e.recordTimeline(tx, token.TicketID, &act.ID, operatorID, "subprocess_completed",
		fmt.Sprintf("子流程 [%s] 已完成", parentToken.NodeID))

	// Continue past the subprocess node
	edges := mainOutEdges[parentToken.NodeID]
	if len(edges) == 0 {
		return fmt.Errorf("subprocess node %s has no outgoing edge", parentToken.NodeID)
	}

	targetNode, ok := mainNodeMap[edges[0].Target]
	if !ok {
		return fmt.Errorf("subprocess node target %q not found", edges[0].Target)
	}

	return e.processNode(ctx, tx, mainDef, mainNodeMap, mainOutEdges, &parentToken, operatorID, targetNode, depth+1)
}

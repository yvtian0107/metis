package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ClassicEngine implements WorkflowEngine via BPMN-style graph traversal.
type ClassicEngine struct {
	resolver *ParticipantResolver
	scheduler TaskSubmitter
}

// TaskSubmitter allows the engine to submit async scheduler tasks.
type TaskSubmitter interface {
	SubmitTask(name string, payload json.RawMessage) error
}

func NewClassicEngine(resolver *ParticipantResolver, scheduler TaskSubmitter) *ClassicEngine {
	return &ClassicEngine{resolver: resolver, scheduler: scheduler}
}

// Start parses the workflow, finds the start node, creates the root token, and processes the first node.
func (e *ClassicEngine) Start(ctx context.Context, tx *gorm.DB, params StartParams) error {
	def, err := ParseWorkflowDef(params.WorkflowJSON)
	if err != nil {
		return fmt.Errorf("workflow parse error: %w", err)
	}

	startNode, err := def.FindStartNode()
	if err != nil {
		return err
	}

	nodeMap, outEdges := def.BuildMaps()

	// Start node must have exactly one outgoing edge
	edges := outEdges[startNode.ID]
	if len(edges) == 0 {
		return fmt.Errorf("start node has no outgoing edge")
	}

	targetNodeID := edges[0].Target
	targetNode, ok := nodeMap[targetNodeID]
	if !ok {
		return fmt.Errorf("start node's target %q not found", targetNodeID)
	}

	// Update ticket status to in_progress
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).
		Update("status", "in_progress").Error; err != nil {
		return err
	}

	// Create root execution token
	token := &executionTokenModel{
		TicketID:  params.TicketID,
		NodeID:    startNode.ID,
		Status:    TokenActive,
		TokenType: TokenMain,
		ScopeID:   "root",
	}
	if err := tx.Create(token).Error; err != nil {
		return fmt.Errorf("failed to create root token: %w", err)
	}

	// Write start form bindings as process variables
	if params.StartFormSchema != "" && params.StartFormData != "" {
		if err := writeFormBindings(tx, params.TicketID, token.ScopeID, params.StartFormSchema, params.StartFormData, "form:start"); err != nil {
			slog.Warn("failed to write start form bindings", "ticketID", params.TicketID, "error", err)
		}
	}

	// Record timeline: workflow started
	if err := e.recordTimeline(tx, params.TicketID, nil, params.RequesterID, "workflow_started", "流程已启动"); err != nil {
		return err
	}

	// Process the first real node
	return e.processNode(ctx, tx, def, nodeMap, outEdges, token, params.RequesterID, targetNode, 0)
}

// Progress completes the current activity and advances the workflow via its token.
func (e *ClassicEngine) Progress(ctx context.Context, tx *gorm.DB, params ProgressParams) error {
	// Load the activity
	var activity activityModel
	if err := tx.First(&activity, params.ActivityID).Error; err != nil {
		return ErrActivityNotFound
	}
	if activity.Status != ActivityPending && activity.Status != ActivityInProgress {
		return ErrActivityNotActive
	}

	// Load the execution token for this activity
	if activity.TokenID == nil {
		return fmt.Errorf("activity %d has no associated execution token", params.ActivityID)
	}
	var token executionTokenModel
	if err := tx.First(&token, *activity.TokenID).Error; err != nil {
		return ErrTokenNotFound
	}
	if token.Status != TokenActive {
		return ErrTokenNotActive
	}

	// Load the ticket to get workflow_json
	var ticket ticketModel
	if err := tx.First(&ticket, params.TicketID).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	def, err := ParseWorkflowDef(json.RawMessage(ticket.WorkflowJSON))
	if err != nil {
		return fmt.Errorf("workflow parse error: %w", err)
	}

	nodeMap, outEdges := def.BuildMaps()

	currentNode, ok := nodeMap[activity.NodeID]
	if !ok {
		return ErrNodeNotFound
	}

	// Complete the current activity
	now := time.Now()
	updates := map[string]any{
		"status":             ActivityCompleted,
		"transition_outcome": params.Outcome,
		"finished_at":        now,
	}
	if len(params.Result) > 0 {
		updates["form_data"] = string(params.Result)
	}
	if err := tx.Model(&activityModel{}).Where("id = ?", params.ActivityID).Updates(updates).Error; err != nil {
		return err
	}

	// Write form bindings as process variables (if activity had a form with bindings)
	if activity.FormSchema != "" && len(params.Result) > 0 {
		source := fmt.Sprintf("form:%d", params.ActivityID)
		if err := writeFormBindings(tx, params.TicketID, token.ScopeID, activity.FormSchema, string(params.Result), source); err != nil {
			slog.Warn("failed to write form bindings on progress", "ticketID", params.TicketID, "activityID", params.ActivityID, "error", err)
		}
	}

	// Record timeline
	msg := fmt.Sprintf("节点 [%s] 完成，结果: %s", nodeLabel(currentNode), params.Outcome)
	e.recordTimeline(tx, params.TicketID, &params.ActivityID, params.OperatorID, "activity_completed", msg)

	// Find matching outgoing edge
	edge, err := e.matchEdge(outEdges[currentNode.ID], params.Outcome)
	if err != nil {
		return fmt.Errorf("从节点 %s 出发无法找到 outcome=%s 的路径: %w", currentNode.ID, params.Outcome, err)
	}

	targetNode, ok := nodeMap[edge.Target]
	if !ok {
		return fmt.Errorf("edge target %q not found", edge.Target)
	}

	return e.processNode(ctx, tx, def, nodeMap, outEdges, &token, params.OperatorID, targetNode, 0)
}

// Cancel terminates all active tokens, activities, and marks the ticket cancelled.
func (e *ClassicEngine) Cancel(ctx context.Context, tx *gorm.DB, params CancelParams) error {
	// Cancel all active execution tokens
	if err := tx.Model(&executionTokenModel{}).
		Where("ticket_id = ? AND status IN ?", params.TicketID, []string{TokenActive, TokenWaiting}).
		Update("status", TokenCancelled).Error; err != nil {
		return err
	}

	// Cancel all active activities
	if err := tx.Model(&activityModel{}).
		Where("ticket_id = ? AND status IN ?", params.TicketID, []string{ActivityPending, ActivityInProgress}).
		Updates(map[string]any{
			"status":      ActivityCancelled,
			"finished_at": time.Now(),
		}).Error; err != nil {
		return err
	}

	// Cancel all pending assignments
	if err := tx.Model(&assignmentModel{}).
		Where("ticket_id = ? AND status = ?", params.TicketID, "pending").
		Update("status", "cancelled").Error; err != nil {
		return err
	}

	// Update ticket
	now := time.Now()
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).Updates(map[string]any{
		"status":      "cancelled",
		"finished_at": now,
	}).Error; err != nil {
		return err
	}

	msg := "工单已取消"
	if params.Reason != "" {
		msg = "工单已取消: " + params.Reason
	}
	return e.recordTimeline(tx, params.TicketID, nil, params.OperatorID, "ticket_cancelled", msg)
}

// processNode handles a node based on its type, driven by an execution token.
// Auto nodes recurse; human nodes create pending activities.
func (e *ClassicEngine) processNode(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, depth int,
) error {
	if depth > MaxAutoDepth {
		// Mark token as cancelled and ticket as failed
		tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenCancelled)
		tx.Model(&ticketModel{}).Where("id = ?", token.TicketID).Update("status", "failed")
		e.recordTimeline(tx, token.TicketID, nil, 0, "error", "流程自动步进超过最大深度(50)")
		return ErrMaxDepthExceeded
	}

	// Update token's current node
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("node_id", node.ID)
	token.NodeID = node.ID

	nodeData, err := ParseNodeData(node.Data)
	if err != nil {
		return fmt.Errorf("parse node %s data: %w", node.ID, err)
	}

	switch node.Type {
	case NodeEnd:
		return e.handleEnd(tx, token, operatorID, node, nodeData)

	case NodeForm:
		return e.handleForm(tx, token, operatorID, node, nodeData)

	case NodeApprove:
		return e.handleApprove(tx, token, operatorID, node, nodeData)

	case NodeProcess:
		return e.handleProcess(tx, token, operatorID, node, nodeData)

	case NodeAction:
		return e.handleAction(tx, token, operatorID, node, nodeData)

	case NodeExclusive:
		return e.handleExclusive(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, nodeData, depth)

	case NodeNotify:
		return e.handleNotify(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, nodeData, depth)

	case NodeWait:
		return e.handleWait(tx, token, operatorID, node, nodeData)

	case NodeParallel:
		switch nodeData.GatewayDirection {
		case GatewayFork:
			return e.handleParallelFork(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, nodeData, depth)
		case GatewayJoin:
			return e.handleParallelJoin(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, nodeData, depth)
		default:
			return fmt.Errorf("%w: parallel node %s has gateway_direction=%q", ErrGatewayMissingDirection, node.ID, nodeData.GatewayDirection)
		}

	case NodeInclusive:
		switch nodeData.GatewayDirection {
		case GatewayFork:
			return e.handleInclusiveFork(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, nodeData, depth)
		case GatewayJoin:
			return e.handleInclusiveJoin(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, nodeData, depth)
		default:
			return fmt.Errorf("%w: inclusive node %s has gateway_direction=%q", ErrGatewayMissingDirection, node.ID, nodeData.GatewayDirection)
		}

	case NodeScript:
		return e.handleScript(ctx, tx, def, nodeMap, outEdges, token, operatorID, node, nodeData, depth)

	default:
		if UnimplementedNodeTypes[node.Type] {
			return fmt.Errorf("%w: %s", ErrNodeNotImplemented, node.Type)
		}
		return fmt.Errorf("%w: %s", ErrInvalidNodeType, node.Type)
	}
}

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

	// Complete the token
	tx.Model(&executionTokenModel{}).Where("id = ?", token.ID).Update("status", TokenCompleted)

	// Child token (parallel branch) — only complete this branch, check join
	if token.ParentTokenID != nil {
		// Check if all siblings are done → reactivate parent
		var remaining int64
		tx.Model(&executionTokenModel{}).
			Where("parent_token_id = ? AND status IN ?", *token.ParentTokenID, []string{TokenActive, TokenWaiting}).
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
				"status":              "completed",
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
		"status":              "completed",
		"finished_at":         now,
		"current_activity_id": act.ID,
	}).Error; err != nil {
		return err
	}

	return e.recordTimeline(tx, token.TicketID, &act.ID, operatorID, "workflow_completed", "流程已完结")
}

func (e *ClassicEngine) handleForm(tx *gorm.DB, token *executionTokenModel, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()

	// Resolve formId to schema snapshot
	formSchema := resolveFormSchema(tx, data.FormID)

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
	mode := data.ApproveMode
	if mode == "" {
		mode = "single"
	}
	act := &activityModel{
		TicketID:      token.TicketID,
		TokenID:       &token.ID,
		Name:          labelOrDefault(data, "审批"),
		ActivityType:  NodeApprove,
		Status:        ActivityPending,
		NodeID:        node.ID,
		ExecutionMode: mode,
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

	// Resolve formId to schema snapshot
	formSchema := resolveFormSchema(tx, data.FormID)

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
		if err := e.scheduler.SubmitTask("itsm-action-execute", payload); err != nil {
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
	tx.Where("ticket_id = ? AND status = ? AND form_data IS NOT NULL AND form_data != ''", token.TicketID, ActivityCompleted).
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

func (e *ClassicEngine) handleNotify(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	token *executionTokenModel, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	// Non-blocking notification — record timeline and continue
	e.recordTimeline(tx, token.TicketID, nil, operatorID, "notification_sent", fmt.Sprintf("通知已发送: %s", data.Label))

	// TODO: integrate with Kernel Channel for actual notification delivery

	// Continue to next node
	edges := outEdges[node.ID]
	if len(edges) == 0 {
		return fmt.Errorf("notify node %s has no outgoing edge", node.ID)
	}

	targetNode, ok := nodeMap[edges[0].Target]
	if !ok {
		return fmt.Errorf("notify target %q not found", edges[0].Target)
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
		if err := e.scheduler.SubmitTask("itsm-wait-timer", payload); err != nil {
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
	tx.Where("ticket_id = ? AND status = ? AND form_data IS NOT NULL AND form_data != ''", token.TicketID, ActivityCompleted).
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

	// Count remaining active/waiting siblings
	var remaining int64
	tx.Model(&executionTokenModel{}).
		Where("parent_token_id = ? AND status IN ?", *token.ParentTokenID, []string{TokenActive, TokenWaiting}).
		Count(&remaining)

	if remaining > 0 {
		// Other branches still running — stop here
		return nil
	}

	// All siblings completed — reactivate parent token
	var parentToken executionTokenModel
	if err := tx.First(&parentToken, *token.ParentTokenID).Error; err != nil {
		return fmt.Errorf("parent token %d not found: %w", *token.ParentTokenID, err)
	}

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

// --- Helpers ---

func (e *ClassicEngine) matchEdge(edges []*WFEdge, outcome string) (*WFEdge, error) {
	var defaultEdge *WFEdge
	for _, edge := range edges {
		if edge.Data.Outcome == outcome {
			return edge, nil
		}
		if edge.Data.Default {
			defaultEdge = edge
		}
	}
	if defaultEdge != nil {
		return defaultEdge, nil
	}
	return nil, ErrNoOutgoingEdge
}

func (e *ClassicEngine) assignParticipants(tx *gorm.DB, ticketID, activityID, operatorID uint, participants []Participant) error {
	if len(participants) == 0 {
		// No participants configured — record warning
		e.recordTimeline(tx, ticketID, &activityID, 0, "warning", "节点未配置参与人，等待管理员手动指派")
		return nil
	}

	for i, p := range participants {
		userIDs, err := e.resolver.Resolve(tx, ticketID, p)
		if err != nil {
			e.recordTimeline(tx, ticketID, &activityID, 0, "warning", fmt.Sprintf("参与人解析失败: %v", err))
			continue
		}

		if len(userIDs) == 0 {
			e.recordTimeline(tx, ticketID, &activityID, 0, "warning", fmt.Sprintf("参与人解析结果为空: type=%s value=%s", p.Type, p.Value))
			continue
		}

		for _, uid := range userIDs {
			assignment := &assignmentModel{
				TicketID:        ticketID,
				ActivityID:      activityID,
				ParticipantType: p.Type,
				AssigneeID:      &uid,
				Status:          "pending",
				Sequence:        i,
				IsCurrent:       i == 0,
			}
			if p.Type == "user" {
				assignment.UserID = &uid
			}
			if err := tx.Create(assignment).Error; err != nil {
				return err
			}
		}

		// Update ticket assignee to the first resolved user
		if i == 0 && len(userIDs) > 0 {
			tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("assignee_id", userIDs[0])
		}
	}

	return nil
}

func (e *ClassicEngine) recordTimeline(tx *gorm.DB, ticketID uint, activityID *uint, operatorID uint, eventType, message string) error {
	tl := &timelineModel{
		TicketID:   ticketID,
		ActivityID: activityID,
		OperatorID: operatorID,
		EventType:  eventType,
		Message:    message,
	}
	return tx.Create(tl).Error
}

func labelOrDefault(data *NodeData, fallback string) string {
	if data.Label != "" {
		return data.Label
	}
	return fallback
}

func nodeLabel(node *WFNode) string {
	data, _ := ParseNodeData(node.Data)
	if data != nil && data.Label != "" {
		return data.Label
	}
	return node.ID
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Minute // default 30 minutes
	}
	return d
}

// --- Lightweight model structs for direct DB operations ---
// These avoid importing the parent itsm package (which would cause a cycle).

type ticketModel struct {
	ID                    uint       `gorm:"primaryKey"`
	Status                string     `gorm:"column:status"`
	EngineType            string     `gorm:"column:engine_type"`
	WorkflowJSON          string     `gorm:"column:workflow_json;type:text"`
	CurrentActivityID     *uint      `gorm:"column:current_activity_id"`
	FinishedAt            *time.Time `gorm:"column:finished_at"`
	RequesterID           uint       `gorm:"column:requester_id"`
	PriorityID            uint       `gorm:"column:priority_id"`
	FormData              string     `gorm:"column:form_data;type:text"`
	AIFailureCount        int        `gorm:"column:ai_failure_count;default:0"`
	CollaborationSpec     string     `gorm:"column:collaboration_spec;type:text"`  // via service join
	AgentID               *uint      `gorm:"column:agent_id"`                      // via service join
	AgentConfig           string     `gorm:"column:agent_config;type:text"`        // via service join
}

func (ticketModel) TableName() string { return "itsm_tickets" }

type activityModel struct {
	ID                uint       `gorm:"primaryKey;autoIncrement"`
	TicketID          uint       `gorm:"column:ticket_id;not null"`
	TokenID           *uint      `gorm:"column:token_id;index"`
	Name              string     `gorm:"column:name;size:128"`
	ActivityType      string     `gorm:"column:activity_type;size:16"`
	Status            string     `gorm:"column:status;size:16;default:pending"`
	NodeID            string     `gorm:"column:node_id;size:64"`
	ExecutionMode     string     `gorm:"column:execution_mode;size:16"`
	FormSchema        string     `gorm:"column:form_schema;type:text"`
	FormData          string     `gorm:"column:form_data;type:text"`
	TransitionOutcome string     `gorm:"column:transition_outcome;size:16"`
	AIDecision        string     `gorm:"column:ai_decision;type:text"`
	AIReasoning       string     `gorm:"column:ai_reasoning;type:text"`
	AIConfidence      float64    `gorm:"column:ai_confidence;default:0"`
	OverriddenBy      *uint      `gorm:"column:overridden_by"`
	DecisionReasoning string     `gorm:"column:decision_reasoning;type:text"`
	StartedAt         *time.Time `gorm:"column:started_at"`
	FinishedAt        *time.Time `gorm:"column:finished_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (activityModel) TableName() string { return "itsm_ticket_activities" }

type assignmentModel struct {
	ID              uint   `gorm:"primaryKey;autoIncrement"`
	TicketID        uint   `gorm:"column:ticket_id;not null"`
	ActivityID      uint   `gorm:"column:activity_id;not null"`
	ParticipantType string `gorm:"column:participant_type;size:32;not null"`
	UserID          *uint  `gorm:"column:user_id"`
	PositionID      *uint  `gorm:"column:position_id"`
	DepartmentID    *uint  `gorm:"column:department_id"`
	AssigneeID      *uint  `gorm:"column:assignee_id"`
	Status          string `gorm:"column:status;size:16;default:pending"`
	Sequence        int    `gorm:"column:sequence;default:0"`
	IsCurrent       bool   `gorm:"column:is_current;default:false"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (assignmentModel) TableName() string { return "itsm_ticket_assignments" }

type timelineModel struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	TicketID   uint   `gorm:"column:ticket_id;not null"`
	ActivityID *uint  `gorm:"column:activity_id"`
	OperatorID uint   `gorm:"column:operator_id;not null"`
	EventType  string `gorm:"column:event_type;size:32;not null"`
	Message    string `gorm:"column:message;size:512"`
	Reasoning  string `gorm:"column:reasoning;type:text"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (timelineModel) TableName() string { return "itsm_ticket_timelines" }

// executionTokenModel is a lightweight model for direct DB operations on tokens.
type executionTokenModel struct {
	ID            uint       `gorm:"primaryKey;autoIncrement"`
	TicketID      uint       `gorm:"column:ticket_id;not null;index:idx_token_ticket_status"`
	ParentTokenID *uint      `gorm:"column:parent_token_id"`
	NodeID        string     `gorm:"column:node_id;size:64"`
	Status        string     `gorm:"column:status;size:16;not null;index:idx_token_ticket_status"`
	TokenType     string     `gorm:"column:token_type;size:16;not null"`
	ScopeID       string     `gorm:"column:scope_id;size:64;not null;default:root"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (executionTokenModel) TableName() string { return "itsm_execution_tokens" }

// formDefModel is a lightweight read-only model for resolving form schema by code.
type formDefModel struct {
	ID     uint   `gorm:"primaryKey"`
	Code   string `gorm:"column:code"`
	Schema string `gorm:"column:schema;type:text"`
}

func (formDefModel) TableName() string { return "itsm_form_definitions" }

// resolveFormSchema looks up a FormDefinition by code and returns its schema JSON.
// Returns empty string if formID is empty or not found (with a log warning).
func resolveFormSchema(tx *gorm.DB, formCode string) string {
	if formCode == "" {
		return ""
	}
	var fd formDefModel
	if err := tx.Where("code = ?", formCode).First(&fd).Error; err != nil {
		slog.Warn("form definition not found, activity will have no form schema",
			"formCode", formCode, "error", err)
		return ""
	}
	return fd.Schema
}

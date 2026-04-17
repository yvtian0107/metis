package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationSender is an optional interface for sending notifications from workflow nodes.
type NotificationSender interface {
	Send(ctx context.Context, channelID uint, subject, body string, recipientIDs []uint) error
}

// ClassicEngine implements WorkflowEngine via BPMN-style graph traversal.
type ClassicEngine struct {
	resolver  *ParticipantResolver
	scheduler TaskSubmitter
	notifier  NotificationSender
}

// TaskSubmitter allows the engine to submit async scheduler tasks.
type TaskSubmitter interface {
	SubmitTask(name string, payload json.RawMessage) error
}

func NewClassicEngine(resolver *ParticipantResolver, scheduler TaskSubmitter, notifier NotificationSender) *ClassicEngine {
	return &ClassicEngine{resolver: resolver, scheduler: scheduler, notifier: notifier}
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
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&activity, params.ActivityID).Error; err != nil {
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
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&token, *activity.TokenID).Error; err != nil {
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

	def, nodeMap, outEdges, err := resolveWorkflowContext(tx, &token, ticket.WorkflowJSON)
	if err != nil {
		return fmt.Errorf("resolve workflow context: %w", err)
	}

	currentNode, ok := nodeMap[activity.NodeID]
	if !ok {
		return ErrNodeNotFound
	}

	// Multi-person approval: delegate to progressApproval for approve-type activities
	// with parallel or sequential mode before completing the activity.
	if activity.ActivityType == NodeApprove && (activity.ExecutionMode == "parallel" || activity.ExecutionMode == "sequential") {
		shouldContinue, err := e.progressApproval(tx, &activity, params)
		if err != nil {
			return err
		}
		if !shouldContinue {
			// Activity still in progress (not all assignments done)
			return nil
		}
		// All assignments done — activity already completed by progressApproval, continue workflow
	} else {
		// Single mode or non-approve: complete the activity immediately
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

	// Cancel any suspended boundary tokens (⑤b itsm-boundary-events)
	cancelBoundaryTokens(tx, &token)

	// Reload activity to get the final transition_outcome (may differ from params.Outcome in multi-approval)
	var finalActivity activityModel
	tx.First(&finalActivity, params.ActivityID)
	finalOutcome := finalActivity.TransitionOutcome
	if finalOutcome == "" {
		finalOutcome = params.Outcome
	}

	// Find matching outgoing edge
	edge, err := e.matchEdge(outEdges[currentNode.ID], finalOutcome)
	if err != nil {
		return fmt.Errorf("从节点 %s 出发无法找到 outcome=%s 的路径: %w", currentNode.ID, finalOutcome, err)
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

	eventType := params.EventType
	if eventType == "" {
		eventType = "ticket_cancelled"
	}
	msg := params.Message
	if msg == "" {
		msg = "工单已取消"
		if params.Reason != "" {
			msg = "工单已取消: " + params.Reason
		}
	}
	return e.recordTimeline(tx, params.TicketID, nil, params.OperatorID, eventType, msg)
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
		// Subprocess end: complete subprocess and resume parent flow
		if token.TokenType == TokenSubprocess {
			return e.completeSubprocess(ctx, tx, token, operatorID, node, nodeData, depth)
		}
		return e.handleEnd(tx, token, operatorID, node, nodeData)

	case NodeForm:
		if err := e.handleForm(tx, token, operatorID, node, nodeData); err != nil {
			return err
		}
		return e.attachBoundaryEvents(tx, def, token, node)

	case NodeApprove:
		if err := e.handleApprove(tx, token, operatorID, node, nodeData); err != nil {
			return err
		}
		return e.attachBoundaryEvents(tx, def, token, node)

	case NodeProcess:
		if err := e.handleProcess(tx, token, operatorID, node, nodeData); err != nil {
			return err
		}
		return e.attachBoundaryEvents(tx, def, token, node)

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

	case NodeSubprocess:
		return e.handleSubprocess(ctx, tx, token, operatorID, node, nodeData, depth)

	default:
		if UnimplementedNodeTypes[node.Type] {
			return fmt.Errorf("%w: %s", ErrNodeNotImplemented, node.Type)
		}
		return fmt.Errorf("%w: %s", ErrInvalidNodeType, node.Type)
	}
}

// --- Multi-person Approval ---

// progressApproval handles multi-person approval modes (parallel/sequential).
// Returns (shouldContinue=true) when the activity is fully completed and workflow should advance,
// or (shouldContinue=false) when the activity is still waiting for more approvals.
func (e *ClassicEngine) progressApproval(tx *gorm.DB, activity *activityModel, params ProgressParams) (bool, error) {
	now := time.Now()

	// Complete the caller's assignment
	result := tx.Model(&assignmentModel{}).
		Where("activity_id = ? AND assignee_id = ? AND status = ?", activity.ID, params.OperatorID, "pending").
		Updates(map[string]any{
			"status":      "completed",
			"finished_at": now,
		})
	if result.RowsAffected == 0 {
		return false, fmt.Errorf("no pending assignment found for user %d on activity %d", params.OperatorID, activity.ID)
	}

	// Delegation auto-return: if the completed assignment was delegated,
	// restore the original assignment back to pending
	var completedAssignment assignmentModel
	if err := tx.Where("activity_id = ? AND assignee_id = ? AND status = ?",
		activity.ID, params.OperatorID, "completed").
		Order("id DESC").First(&completedAssignment).Error; err == nil {
		if completedAssignment.DelegatedFrom != nil {
			tx.Model(&assignmentModel{}).
				Where("id = ? AND status = ?", *completedAssignment.DelegatedFrom, "delegated").
				Updates(map[string]any{"status": "pending", "is_current": true})
			e.recordTimeline(tx, params.TicketID, &params.ActivityID, 0, "delegate_return",
				"委派任务已完成，工单已回归原处理人")
			return false, nil // don't advance workflow — original assignee still needs to act
		}
	}

	switch activity.ExecutionMode {
	case "parallel":
		return e.progressParallelApproval(tx, activity, params, now)
	case "sequential":
		return e.progressSequentialApproval(tx, activity, params, now)
	default:
		// Should not reach here (caller checks mode), but handle gracefully
		return e.completeActivity(tx, activity, params, now)
	}
}

// progressParallelApproval implements 会签 (countersign) mode:
// - Any reject → immediately complete activity with "reject" outcome
// - All approve → complete activity with "approve" outcome
// - Otherwise → wait for remaining participants
func (e *ClassicEngine) progressParallelApproval(tx *gorm.DB, activity *activityModel, params ProgressParams, now time.Time) (bool, error) {
	// If this person rejected, short-circuit: complete activity immediately with "reject"
	if params.Outcome == "reject" {
		// Cancel all remaining pending assignments
		tx.Model(&assignmentModel{}).
			Where("activity_id = ? AND status = ?", activity.ID, "pending").
			Updates(map[string]any{"status": "cancelled", "finished_at": now})

		return e.completeActivity(tx, activity, ProgressParams{
			TicketID:   params.TicketID,
			ActivityID: params.ActivityID,
			Outcome:    "reject",
			Result:     params.Result,
			OperatorID: params.OperatorID,
		}, now)
	}

	// Count remaining pending assignments
	var remaining int64
	tx.Model(&assignmentModel{}).
		Where("activity_id = ? AND status = ?", activity.ID, "pending").
		Count(&remaining)

	if remaining > 0 {
		// Still waiting for other participants
		e.recordTimeline(tx, params.TicketID, &params.ActivityID, params.OperatorID,
			"approval_partial", fmt.Sprintf("会签：用户 %d 已通过，还有 %d 人待审批", params.OperatorID, remaining))
		return false, nil
	}

	// All assignments completed — complete the activity with "approve"
	return e.completeActivity(tx, activity, ProgressParams{
		TicketID:   params.TicketID,
		ActivityID: params.ActivityID,
		Outcome:    "approve",
		Result:     params.Result,
		OperatorID: params.OperatorID,
	}, now)
}

// progressSequentialApproval implements 依次审批 mode:
// - Complete current assignment, advance is_current to next
// - If no more assignments, complete activity
func (e *ClassicEngine) progressSequentialApproval(tx *gorm.DB, activity *activityModel, params ProgressParams, now time.Time) (bool, error) {
	// If rejected at any point, complete activity with "reject"
	if params.Outcome == "reject" {
		// Cancel all remaining pending assignments
		tx.Model(&assignmentModel{}).
			Where("activity_id = ? AND status = ?", activity.ID, "pending").
			Updates(map[string]any{"status": "cancelled", "finished_at": now})

		return e.completeActivity(tx, activity, ProgressParams{
			TicketID:   params.TicketID,
			ActivityID: params.ActivityID,
			Outcome:    "reject",
			Result:     params.Result,
			OperatorID: params.OperatorID,
		}, now)
	}

	// Find next pending assignment by sequence order
	var nextAssignment assignmentModel
	err := tx.Where("activity_id = ? AND status = ?", activity.ID, "pending").
		Order("sequence ASC").First(&nextAssignment).Error

	if err != nil {
		// No more pending assignments — all done, complete activity
		return e.completeActivity(tx, activity, ProgressParams{
			TicketID:   params.TicketID,
			ActivityID: params.ActivityID,
			Outcome:    "approve",
			Result:     params.Result,
			OperatorID: params.OperatorID,
		}, now)
	}

	// Advance is_current to the next assignment
	tx.Model(&assignmentModel{}).Where("activity_id = ?", activity.ID).Update("is_current", false)
	tx.Model(&assignmentModel{}).Where("id = ?", nextAssignment.ID).Update("is_current", true)

	// Update ticket assignee to the next person
	if nextAssignment.AssigneeID != nil {
		tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).Update("assignee_id", *nextAssignment.AssigneeID)
	}

	e.recordTimeline(tx, params.TicketID, &params.ActivityID, params.OperatorID,
		"approval_sequential", fmt.Sprintf("依次审批：用户 %d 已通过，流转至下一审批人", params.OperatorID))

	return false, nil
}

// completeActivity marks an activity as completed with the given outcome.
func (e *ClassicEngine) completeActivity(tx *gorm.DB, activity *activityModel, params ProgressParams, now time.Time) (bool, error) {
	updates := map[string]any{
		"status":             ActivityCompleted,
		"transition_outcome": params.Outcome,
		"finished_at":        now,
	}
	if len(params.Result) > 0 {
		updates["form_data"] = string(params.Result)
	}
	if err := tx.Model(&activityModel{}).Where("id = ?", params.ActivityID).Updates(updates).Error; err != nil {
		return false, err
	}
	return true, nil
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
	// Send notification via NotificationSender if configured
	if e.notifier != nil && data.ChannelID != 0 {
		// Resolve recipients
		var recipientIDs []uint
		for _, p := range data.Recipients {
			ids, err := e.resolver.Resolve(tx, token.TicketID, p)
			if err != nil {
				slog.Warn("notify: failed to resolve recipient", "ticketID", token.TicketID, "error", err)
				continue
			}
			recipientIDs = append(recipientIDs, ids...)
		}

		if len(recipientIDs) > 0 {
			// Build notification body with template variable replacement
			subject := data.Label
			body := data.Template
			if body != "" {
				body = e.renderTemplate(tx, token.TicketID, token.ScopeID, body)
			}

			if err := e.notifier.Send(ctx, data.ChannelID, subject, body, recipientIDs); err != nil {
				// Non-blocking: record warning but continue workflow
				slog.Warn("notify: send failed", "ticketID", token.TicketID, "channelID", data.ChannelID, "error", err)
				e.recordTimeline(tx, token.TicketID, nil, operatorID, "warning",
					fmt.Sprintf("通知发送失败: %v", err))
			}
		}
	}

	// Record timeline
	e.recordTimeline(tx, token.TicketID, nil, operatorID, "notification_sent", fmt.Sprintf("通知已发送: %s", data.Label))

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
			if err := e.scheduler.SubmitTask("itsm-boundary-timer", payload); err != nil {
				slog.Error("failed to submit boundary timer task", "error", err, "ticketID", token.TicketID)
			}
		}

		e.recordTimeline(tx, token.TicketID, nil, 0, "boundary_timer_set",
			fmt.Sprintf("边界定时器已设置: %s (节点 %s)", bData.Duration, labelOrDefault(bData, bNode.ID)))
	}

	return nil
}

// cancelBoundaryTokens cancels all suspended boundary tokens for a host token.
func cancelBoundaryTokens(tx *gorm.DB, hostToken *executionTokenModel) {
	tx.Model(&executionTokenModel{}).
		Where("parent_token_id = ? AND token_type = ? AND status = ?",
			hostToken.ID, TokenBoundary, TokenSuspended).
		Update("status", TokenCancelled)
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

// --- Helpers ---

// renderTemplate replaces template variables in notification templates.
// Supports: {{ticket.code}}, {{ticket.status}}, {{activity.name}}, {{var.xxx}}
func (e *ClassicEngine) renderTemplate(tx *gorm.DB, ticketID uint, scopeID, tmpl string) string {
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err != nil {
		return tmpl
	}

	result := strings.ReplaceAll(tmpl, "{{ticket.code}}", fmt.Sprintf("TICK-%06d", ticketID))
	result = strings.ReplaceAll(result, "{{ticket.status}}", ticket.Status)

	// Replace process variables: {{var.xxx}}
	var vars []processVariableModel
	tx.Where("ticket_id = ? AND scope_id = ?", ticketID, scopeID).Find(&vars)
	for _, v := range vars {
		result = strings.ReplaceAll(result, "{{var."+v.Key+"}}", v.Value)
	}

	return result
}

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
	Code                  string     `gorm:"column:code"`
	Status                string     `gorm:"column:status"`
	EngineType            string     `gorm:"column:engine_type"`
	WorkflowJSON          string     `gorm:"column:workflow_json;type:text"`
	CurrentActivityID     *uint      `gorm:"column:current_activity_id"`
	FinishedAt            *time.Time `gorm:"column:finished_at"`
	RequesterID           uint       `gorm:"column:requester_id"`
	PriorityID            uint       `gorm:"column:priority_id"`
	FormData              string     `gorm:"column:form_data;type:text"`
	SLAResponseDeadline   *time.Time `gorm:"column:sla_response_deadline"`
	SLAResolutionDeadline *time.Time `gorm:"column:sla_resolution_deadline"`
	SLAStatus             string     `gorm:"column:sla_status"`
	SLAPausedAt           *time.Time `gorm:"column:sla_paused_at"`
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
	DelegatedFrom   *uint  `gorm:"column:delegated_from"`
	TransferFrom    *uint  `gorm:"column:transfer_from"`
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

// resolveWorkflowContext returns the correct WorkflowDef and maps for a token.
// For subprocess tokens, it navigates to the parent subprocess node and parses
// the embedded SubProcessDef. For main/parallel tokens, it uses the ticket's workflow JSON directly.
func resolveWorkflowContext(tx *gorm.DB, token *executionTokenModel, ticketWorkflowJSON string) (*WorkflowDef, map[string]*WFNode, map[string][]*WFEdge, error) {
	def, err := ParseWorkflowDef(json.RawMessage(ticketWorkflowJSON))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("workflow parse error: %w", err)
	}
	nodeMap, outEdges := def.BuildMaps()

	if token.TokenType != TokenSubprocess || token.ParentTokenID == nil {
		return def, nodeMap, outEdges, nil
	}

	// Subprocess token — find parent token's node (the subprocess node) and parse its SubProcessDef
	var parentToken executionTokenModel
	if err := tx.First(&parentToken, *token.ParentTokenID).Error; err != nil {
		return nil, nil, nil, fmt.Errorf("parent token %d not found: %w", *token.ParentTokenID, err)
	}

	subNode, ok := nodeMap[parentToken.NodeID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("subprocess node %s not found in workflow", parentToken.NodeID)
	}

	subData, err := ParseNodeData(subNode.Data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("subprocess node %s data parse error: %w", subNode.ID, err)
	}

	if len(subData.SubProcessDef) == 0 {
		return nil, nil, nil, fmt.Errorf("subprocess node %s has no subprocess_def", subNode.ID)
	}

	subDef, err := ParseWorkflowDef(subData.SubProcessDef)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("subprocess_def parse error for node %s: %w", subNode.ID, err)
	}

	subNodeMap, subOutEdges := subDef.BuildMaps()
	return subDef, subNodeMap, subOutEdges, nil
}

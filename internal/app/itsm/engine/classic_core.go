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

type transactionalTaskSubmitter interface {
	SubmitTaskTx(tx *gorm.DB, name string, payload json.RawMessage) error
}

func submitTaskInTx(submitter TaskSubmitter, tx *gorm.DB, name string, payload json.RawMessage) error {
	if submitter == nil {
		return nil
	}
	if txSubmitter, ok := submitter.(transactionalTaskSubmitter); ok && tx != nil {
		return txSubmitter.SubmitTaskTx(tx, name, payload)
	}
	return submitter.SubmitTask(name, payload)
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

	// Update ticket status to waiting_human before the first workflow node runs.
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).
		Update("status", TicketStatusWaitingHuman).Error; err != nil {
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
		if err := writeFormBindings(tx, params.TicketID, token.ScopeID, params.StartFormSchema, params.StartFormData, "form:start", ""); err != nil {
			slog.Warn("failed to write start form bindings", "ticketID", params.TicketID, "error", err)
			if fve, ok := err.(*FormValidationError); ok {
				e.recordTimeline(tx, params.TicketID, nil, params.RequesterID, "form_validation_failed",
					fmt.Sprintf("开始表单验证失败: %s", fve.Error()))
			}
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

	now := time.Now()
	if completedAssignment, completed, err := completePendingAssignment(tx, e.resolver, activity.ID, params.OperatorID, params.Outcome, now, params.OperatorPositionIDs, params.OperatorDepartmentIDs, params.OperatorOrgScopeReady); err != nil {
		if errors.Is(err, ErrNoActiveAssignment) && activityBecameInactive(tx, params.ActivityID) {
			return ErrActivityNotActive
		}
		return err
	} else if completed && completedAssignment != nil {
		if completedAssignment.DelegatedFrom != nil {
			if err := tx.Model(&assignmentModel{}).
				Where("id = ? AND status = ?", *completedAssignment.DelegatedFrom, "delegated").
				Updates(map[string]any{"status": "pending", "is_current": true}).Error; err != nil {
				return err
			}
			e.recordTimeline(tx, params.TicketID, &params.ActivityID, 0, "delegate_return",
				"委派任务已完成，工单已回归原处理人")
			return nil
		}
	}

	updates := map[string]any{
		"status":             humanOrCompletedActivityStatus(activity.ActivityType, params.Outcome),
		"transition_outcome": params.Outcome,
		"finished_at":        now,
	}
	if params.Opinion != "" {
		updates["decision_reasoning"] = params.Opinion
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
		if err := writeFormBindings(tx, params.TicketID, token.ScopeID, activity.FormSchema, string(params.Result), source, activity.NodeID); err != nil {
			slog.Warn("failed to write form bindings on progress", "ticketID", params.TicketID, "activityID", params.ActivityID, "error", err)
			if fve, ok := err.(*FormValidationError); ok {
				e.recordTimeline(tx, params.TicketID, &params.ActivityID, params.OperatorID, "form_validation_failed",
					fmt.Sprintf("表单验证失败: %s", fve.Error()))
			}
		}
	}

	// Record timeline
	msg := fmt.Sprintf("节点 [%s] 完成，结果: %s", nodeLabel(currentNode), params.Outcome)
	if params.Opinion != "" {
		msg = fmt.Sprintf("%s，处理意见: %s", msg, params.Opinion)
	}
	e.recordTimelineWithReasoning(tx, params.TicketID, &params.ActivityID, params.OperatorID, "activity_completed", msg, params.Opinion)

	// Cancel any suspended boundary tokens (⑤b itsm-boundary-events)
	cancelBoundaryTokens(tx, &token)

	// Reload activity to get the final transition_outcome recorded by the activity handler.
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
		Update("status", ActivityCancelled).Error; err != nil {
		return err
	}

	// Update ticket
	now := time.Now()
	if err := tx.Model(&ticketModel{}).Where("id = ?", params.TicketID).Updates(map[string]any{
		"status":      ticketCancelStatus(params.EventType),
		"outcome":     ticketCancelOutcome(params.EventType),
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

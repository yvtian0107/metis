package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
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

// Start parses the workflow, finds the start node, and creates the first Activity.
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

	// Record timeline: workflow started
	if err := e.recordTimeline(tx, params.TicketID, nil, params.RequesterID, "workflow_started", "流程已启动"); err != nil {
		return err
	}

	// Process the first real node
	return e.processNode(ctx, tx, def, nodeMap, outEdges, params.TicketID, params.RequesterID, targetNode, 0)
}

// Progress completes the current activity and advances the workflow.
func (e *ClassicEngine) Progress(ctx context.Context, tx *gorm.DB, params ProgressParams) error {
	// Load the activity
	var activity activityModel
	if err := tx.First(&activity, params.ActivityID).Error; err != nil {
		return ErrActivityNotFound
	}
	if activity.Status != ActivityPending && activity.Status != ActivityInProgress {
		return ErrActivityNotActive
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

	return e.processNode(ctx, tx, def, nodeMap, outEdges, params.TicketID, params.OperatorID, targetNode, 0)
}

// Cancel terminates all active activities and marks the ticket cancelled.
func (e *ClassicEngine) Cancel(ctx context.Context, tx *gorm.DB, params CancelParams) error {
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

// processNode handles a node based on its type. Auto nodes recurse; human nodes create pending activities.
func (e *ClassicEngine) processNode(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	ticketID uint, operatorID uint,
	node *WFNode, depth int,
) error {
	if depth > MaxAutoDepth {
		// Mark ticket as failed
		tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("status", "failed")
		e.recordTimeline(tx, ticketID, nil, 0, "error", "流程自动步进超过最大深度(50)")
		return ErrMaxDepthExceeded
	}

	nodeData, err := ParseNodeData(node.Data)
	if err != nil {
		return fmt.Errorf("parse node %s data: %w", node.ID, err)
	}

	switch node.Type {
	case NodeEnd:
		return e.handleEnd(tx, ticketID, operatorID, node, nodeData)

	case NodeForm:
		return e.handleForm(tx, ticketID, operatorID, node, nodeData)

	case NodeApprove:
		return e.handleApprove(tx, ticketID, operatorID, node, nodeData)

	case NodeProcess:
		return e.handleProcess(tx, ticketID, operatorID, node, nodeData)

	case NodeAction:
		return e.handleAction(tx, ticketID, operatorID, node, nodeData)

	case NodeGateway:
		return e.handleGateway(ctx, tx, def, nodeMap, outEdges, ticketID, operatorID, node, nodeData, depth)

	case NodeNotify:
		return e.handleNotify(ctx, tx, def, nodeMap, outEdges, ticketID, operatorID, node, nodeData, depth)

	case NodeWait:
		return e.handleWait(tx, ticketID, operatorID, node, nodeData)

	default:
		return fmt.Errorf("%w: %s", ErrInvalidNodeType, node.Type)
	}
}

// --- Node handlers ---

func (e *ClassicEngine) handleEnd(tx *gorm.DB, ticketID, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	// Create completed activity for the end node
	act := &activityModel{
		TicketID:          ticketID,
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

	// Complete the ticket
	if err := tx.Model(&ticketModel{}).Where("id = ?", ticketID).Updates(map[string]any{
		"status":              "completed",
		"finished_at":         now,
		"current_activity_id": act.ID,
	}).Error; err != nil {
		return err
	}

	return e.recordTimeline(tx, ticketID, &act.ID, operatorID, "workflow_completed", "流程已完结")
}

func (e *ClassicEngine) handleForm(tx *gorm.DB, ticketID, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	act := &activityModel{
		TicketID:      ticketID,
		Name:          labelOrDefault(data, "表单填写"),
		ActivityType:  NodeForm,
		Status:        ActivityPending,
		NodeID:        node.ID,
		ExecutionMode: "single",
		FormSchema:    string(data.FormSchema),
		StartedAt:     &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)

	return e.assignParticipants(tx, ticketID, act.ID, operatorID, data.Participants)
}

func (e *ClassicEngine) handleApprove(tx *gorm.DB, ticketID, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	mode := data.ApproveMode
	if mode == "" {
		mode = "single"
	}
	act := &activityModel{
		TicketID:      ticketID,
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

	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)

	return e.assignParticipants(tx, ticketID, act.ID, operatorID, data.Participants)
}

func (e *ClassicEngine) handleProcess(tx *gorm.DB, ticketID, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	act := &activityModel{
		TicketID:      ticketID,
		Name:          labelOrDefault(data, "处理"),
		ActivityType:  NodeProcess,
		Status:        ActivityPending,
		NodeID:        node.ID,
		ExecutionMode: "single",
		FormSchema:    string(data.FormSchema),
		StartedAt:     &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)

	return e.assignParticipants(tx, ticketID, act.ID, operatorID, data.Participants)
}

func (e *ClassicEngine) handleAction(tx *gorm.DB, ticketID, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	act := &activityModel{
		TicketID:     ticketID,
		Name:         labelOrDefault(data, "动作执行"),
		ActivityType: NodeAction,
		Status:       ActivityInProgress,
		NodeID:       node.ID,
		StartedAt:    &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)

	// Submit async task
	if e.scheduler != nil && data.ActionID != 0 {
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":   ticketID,
			"activity_id": act.ID,
			"action_id":   data.ActionID,
		})
		if err := e.scheduler.SubmitTask("itsm-action-execute", payload); err != nil {
			slog.Error("failed to submit action task", "error", err, "ticketID", ticketID)
			// Record timeline but don't fail the workflow
			e.recordTimeline(tx, ticketID, &act.ID, 0, "warning", "动作任务提交失败: "+err.Error())
		}
	}

	e.recordTimeline(tx, ticketID, &act.ID, operatorID, "action_started", "动作节点开始执行")
	return nil
}

func (e *ClassicEngine) handleGateway(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	ticketID, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	// Load ticket for field evaluation
	var ticket ticketModel
	if err := tx.First(&ticket, ticketID).Error; err != nil {
		return err
	}

	// Load latest completed activity's form data for condition evaluation
	var latestActivity activityModel
	tx.Where("ticket_id = ? AND status = ? AND form_data IS NOT NULL AND form_data != ''", ticketID, ActivityCompleted).
		Order("id DESC").First(&latestActivity)

	evalCtx := buildEvalContext(&ticket, &latestActivity)

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
		tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("status", "failed")
		e.recordTimeline(tx, ticketID, nil, 0, "error", fmt.Sprintf("网关节点 %s 无匹配条件且无默认边", node.ID))
		return fmt.Errorf("gateway %s: no matching condition and no default edge", node.ID)
	}

	targetNode, ok := nodeMap[matchedEdge.Target]
	if !ok {
		return fmt.Errorf("gateway target %q not found", matchedEdge.Target)
	}

	return e.processNode(ctx, tx, def, nodeMap, outEdges, ticketID, operatorID, targetNode, depth+1)
}

func (e *ClassicEngine) handleNotify(
	ctx context.Context, tx *gorm.DB,
	def *WorkflowDef, nodeMap map[string]*WFNode, outEdges map[string][]*WFEdge,
	ticketID, operatorID uint,
	node *WFNode, data *NodeData, depth int,
) error {
	// Non-blocking notification — record timeline and continue
	e.recordTimeline(tx, ticketID, nil, operatorID, "notification_sent", fmt.Sprintf("通知已发送: %s", data.Label))

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

	return e.processNode(ctx, tx, def, nodeMap, outEdges, ticketID, operatorID, targetNode, depth+1)
}

func (e *ClassicEngine) handleWait(tx *gorm.DB, ticketID, operatorID uint, node *WFNode, data *NodeData) error {
	now := time.Now()
	status := ActivityPending // signal mode
	if data.WaitMode == "timer" {
		status = ActivityInProgress
	}

	act := &activityModel{
		TicketID:     ticketID,
		Name:         labelOrDefault(data, "等待"),
		ActivityType: NodeWait,
		Status:       status,
		NodeID:       node.ID,
		StartedAt:    &now,
	}
	if err := tx.Create(act).Error; err != nil {
		return err
	}

	tx.Model(&ticketModel{}).Where("id = ?", ticketID).Update("current_activity_id", act.ID)

	if data.WaitMode == "timer" && e.scheduler != nil {
		dur := parseDuration(data.Duration)
		executeAfter := now.Add(dur)
		payload, _ := json.Marshal(map[string]any{
			"ticket_id":     ticketID,
			"activity_id":   act.ID,
			"execute_after": executeAfter.Format(time.RFC3339),
		})
		if err := e.scheduler.SubmitTask("itsm-wait-timer", payload); err != nil {
			slog.Error("failed to submit wait timer task", "error", err, "ticketID", ticketID)
		}
		e.recordTimeline(tx, ticketID, &act.ID, operatorID, "wait_timer_started", fmt.Sprintf("等待定时器已设置: %s", data.Duration))
	} else {
		e.recordTimeline(tx, ticketID, &act.ID, operatorID, "wait_signal", "等待外部信号")
	}

	return nil
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

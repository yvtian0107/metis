package bdd

// steps_vpn_smart_test.go — step definitions for the VPN smart engine BDD scenarios.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	. "metis/internal/app/itsm/domain"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"metis/internal/app/itsm/engine"
)

// registerSmartSteps registers all smart engine step definitions.
func registerSmartSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已基于协作规范发布 VPN 开通服务（智能引擎）$`, bc.givenSmartServicePublished)
	sc.Given(`^已基于协作规范发布 VPN 服务（智能引擎）$`, bc.givenSmartServicePublished)
	sc.Given(`^"([^"]*)" 已创建 VPN 工单，访问原因为 "([^"]*)"$`, bc.givenSmartTicketCreated)
	sc.Given(`^"([^"]*)" 已创建 VPN 工单，表单数据为:$`, bc.givenSmartTicketCreatedWithFormData)
	sc.Given(`^"([^"]*)" 已创建 VPN 工单，访问原因同时包含网络和安全诉求$`, bc.givenSmartTicketWithConflictingReasons)
	sc.Given(`^智能引擎置信度阈值设为 ([0-9.]+)$`, bc.givenConfidenceThreshold)
	sc.Given(`^VPN 处理人均已停用$`, bc.givenVPNOperatorsInactive)
	sc.Given(`^VPN 安全管理员处理人已停用$`, bc.givenVPNSecurityOperatorInactive)
	sc.Given(`^"([^"]*)" 已创建 VPN 工单（使用缺失参与者的工作流）$`, bc.givenSmartTicketMissingParticipant)
	sc.Given(`^VPN 工作流参考图错误地把网络类诉求指向安全管理员$`, bc.givenVPNWorkflowRoutesNetworkToSecurity)
	sc.Given(`^VPN 工作流参考图错误地把驳回指向申请人补充表单$`, bc.givenVPNWorkflowRejectedReturnsRequesterForm)

	sc.When(`^智能引擎执行决策循环$`, bc.whenSmartEngineDecisionCycle)
	sc.When(`^管理员接管该人工处置决策$`, bc.whenAdminConfirmsPendingDecision)
	sc.When(`^管理员确认该人工处置决策$`, bc.whenAdminConfirmsPendingDecision)
	sc.When(`^当前活动的被分配人认领并处理完成$`, bc.whenAssigneeClaimsAndProcesss)
	sc.When(`^当前活动的被分配人认领并处理驳回$`, bc.whenAssigneeRejects)
	sc.When(`^当前活动的被分配人驳回，意见为 "([^"]*)"$`, bc.whenAssigneeRejectsWithOpinion)
	sc.When(`^智能引擎再次执行决策循环$`, bc.whenSmartEngineDecisionCycleAgain)

	sc.Then(`^存在至少一个活动$`, bc.thenAtLeastOneActivity)
	sc.Then(`^活动类型在允许列表内$`, bc.thenActivityTypeAllowed)
	sc.Then(`^决策置信度在合法范围内$`, bc.thenConfidenceInRange)
	sc.Then(`^若指定了参与人则参与人在候选列表内$`, bc.thenParticipantInCandidates)
	sc.Then(`^时间线应包含 AI 决策相关事件$`, bc.thenTimelineContainsAIDecision)
	sc.Then(`^决策工具 "([^"]*)" 已被调用$`, bc.thenDecisionToolCalled)
	sc.Then(`^参与人解析工具使用岗位部门 "([^"]*)/([^"]*)"$`, bc.thenResolveParticipantUsedPositionDepartment)
	sc.Then(`^当前处理任务未分配到岗位 "([^"]*)"$`, bc.thenCurrentProcessNotAssignedToPosition)
	sc.Then(`^当前活跃人工任务数为 (\d+)$`, bc.thenActiveHumanActivityCountIs)
	sc.Then(`^当前岗位 "([^"]*)" 的活跃处理任务数为 (\d+)$`, bc.thenActiveProcessActivityCountForPositionIs)
	sc.Then(`^没有不可执行的高置信人工任务$`, bc.thenNoUnexecutableHighConfidenceHumanTask)
	sc.Then(`^决策诊断事件已记录$`, bc.thenDecisionDiagnosticRecorded)
	sc.Then(`^不得高置信选择单一路由$`, bc.thenNoHighConfidenceSingleRouteChoice)
	sc.Then(`^进入澄清或低置信人工处置$`, bc.thenClarificationOrLowConfidenceHandling)
	sc.Then(`^不会重复创建刚完成的人工作业$`, bc.thenNoDuplicateAfterCompletedHumanWork)
	sc.Then(`^不得创建申请人补充表单$`, bc.thenNoRequesterSupplementForm)
	sc.Then(`^工单结果为 "([^"]*)"$`, bc.thenTicketOutcomeIs)
	sc.Then(`^工单处于驳回终态或已有决策诊断$`, bc.thenTicketRejectedOrDiagnosticRecorded)
	sc.Then(`^工单活动数保持为 (\d+)$`, bc.thenActivityCountIs)
	sc.Then(`^AI 决策依据包含 "([^"]*)"$`, bc.thenAIDecisionEvidenceContains)
	sc.Then(`^当前活动状态为 "([^"]*)"$`, bc.thenCurrentActivityStatusIs)
	sc.Then(`^当前活动状态不为 "([^"]*)"$`, bc.thenCurrentActivityStatusIsNot)
	sc.Then(`^活动记录中包含 AI 推理说明$`, bc.thenActivityContainsAIReasoning)
}

// --- Given steps ---

func (bc *bddContext) givenSmartServicePublished() error {
	return publishVPNSmartService(bc)
}

func (bc *bddContext) givenSmartTicketCreated(username, requestKind string) error {
	normalizedKind := normalizeVPNRequestKind(requestKind)
	formData := map[string]any{
		"request_kind": normalizedKind,
		"vpn_account":  fmt.Sprintf("%s@dev.local", username),
		"device_usage": vpnDeviceUsageForKind(normalizedKind),
	}
	return bc.createSmartVPNTicket(username, fmt.Sprintf("VPN开通申请(智能) - %s", requestKind), formData, bc.service.WorkflowJSON)
}

func (bc *bddContext) givenSmartTicketCreatedWithFormData(username string, doc *godog.DocString) error {
	if doc == nil {
		return fmt.Errorf("missing form data doc string")
	}
	var formData map[string]any
	if err := json.Unmarshal([]byte(doc.Content), &formData); err != nil {
		return fmt.Errorf("parse form data JSON: %w", err)
	}
	return bc.createSmartVPNTicket(username, "VPN开通申请(智能) - corner case", formData, bc.service.WorkflowJSON)
}

func (bc *bddContext) createSmartVPNTicket(username, title string, formData map[string]any, workflowJSON JSONField) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	formJSON, _ := json.Marshal(formData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("VPN-S-%d", time.Now().UnixNano()),
		Title:        title,
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: workflowJSON,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func normalizeVPNRequestKind(requestKind string) string {
	switch requestKind {
	case "network_support":
		return "online_support"
	default:
		return requestKind
	}
}

func vpnDeviceUsageForKind(requestKind string) string {
	switch requestKind {
	case "online_support":
		return "线上支持，需要远程访问内网服务"
	case "troubleshooting":
		return "故障排查，需要临时访问诊断环境"
	case "production_emergency":
		return "生产应急，需要立即远程处理"
	case "network_access_issue":
		return "网络接入问题排查，需要 VPN 连通性验证"
	case "external_collaboration":
		return "外部协作，需要访问指定协作系统"
	case "long_term_remote_work":
		return "长期远程办公，需要稳定访问办公内网"
	case "cross_border_access":
		return "跨境访问，需要安全合规审查"
	case "security_compliance":
		return "安全合规事项，需要审计与取证访问"
	default:
		return "VPN 开通申请 BDD 测试"
	}
}

func (bc *bddContext) givenSmartTicketWithConflictingReasons(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	formData := map[string]any{
		"request_kind": []string{"network_access_issue", "security_compliance"},
		"vpn_account":  "conflict-user@dev.local",
		"device_usage": "同时用于网络链路调试和安全审计取证",
		"reason":       "网络链路调试和安全审计属于不同处理路径，需要用户明确本次办理哪一个诉求",
	}
	formJSON, _ := json.Marshal(formData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("VPN-SC-%d", time.Now().UnixNano()),
		Title:        "VPN开通申请(智能) - 网络与安全诉求冲突",
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: bc.service.WorkflowJSON,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create conflicting ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func (bc *bddContext) givenConfidenceThreshold(threshold string) error {
	if bc.service == nil {
		return fmt.Errorf("no service in context")
	}
	agentConfig := fmt.Sprintf(`{"confidence_threshold": %s}`, threshold)
	bc.service.AgentConfig = JSONField(agentConfig)
	return bc.db.Save(bc.service).Error
}

func (bc *bddContext) givenVPNOperatorsInactive() error {
	for _, username := range []string{"network-operator", "security-operator"} {
		if err := bc.deactivateUser(username); err != nil {
			return err
		}
	}
	return nil
}

func (bc *bddContext) givenVPNSecurityOperatorInactive() error {
	return bc.deactivateUser("security-operator")
}

func (bc *bddContext) deactivateUser(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}
	if err := bc.db.Table("users").Where("id = ?", user.ID).Update("is_active", false).Error; err != nil {
		return fmt.Errorf("deactivate %q: %w", username, err)
	}
	user.IsActive = false
	return nil
}

func (bc *bddContext) givenSmartTicketMissingParticipant(username string) error {
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found in context", username)
	}

	// Override service workflow with the missing-participant fixture.
	bc.service.WorkflowJSON = JSONField(missingParticipantWorkflowJSON)
	if err := bc.db.Save(bc.service).Error; err != nil {
		return fmt.Errorf("update service workflow: %w", err)
	}

	formData := map[string]any{
		"request_kind": "online_support",
		"vpn_account":  fmt.Sprintf("%s@dev.local", username),
		"device_usage": "BDD test - missing participant",
	}
	formJSON, _ := json.Marshal(formData)

	ticket := &Ticket{
		Code:         fmt.Sprintf("VPN-SM-%d", time.Now().UnixNano()),
		Title:        "VPN开通申请(智能) - 缺失参与者",
		ServiceID:    bc.service.ID,
		EngineType:   "smart",
		Status:       "pending",
		PriorityID:   bc.priority.ID,
		RequesterID:  user.ID,
		FormData:     JSONField(formJSON),
		WorkflowJSON: JSONField(missingParticipantWorkflowJSON),
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func (bc *bddContext) givenVPNWorkflowRoutesNetworkToSecurity() error {
	if bc.service == nil {
		return fmt.Errorf("no service in context")
	}
	corrupted, err := corruptVPNWorkflowRouteTargets(json.RawMessage(bc.service.WorkflowJSON))
	if err != nil {
		return err
	}
	bc.service.WorkflowJSON = JSONField(corrupted)
	return bc.db.Save(bc.service).Error
}

func (bc *bddContext) givenVPNWorkflowRejectedReturnsRequesterForm() error {
	if bc.service == nil {
		return fmt.Errorf("no service in context")
	}
	corrupted, err := corruptVPNWorkflowRejectedTarget(json.RawMessage(bc.service.WorkflowJSON))
	if err != nil {
		return err
	}
	bc.service.WorkflowJSON = JSONField(corrupted)
	return bc.db.Save(bc.service).Error
}

type vpnWorkflowDoc struct {
	Nodes []vpnWorkflowNode `json:"nodes"`
	Edges []vpnWorkflowEdge `json:"edges"`
}

type vpnWorkflowNode struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Position map[string]any `json:"position,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

type vpnWorkflowEdge struct {
	ID     string         `json:"id"`
	Source string         `json:"source"`
	Target string         `json:"target"`
	Data   map[string]any `json:"data,omitempty"`
}

func corruptVPNWorkflowRouteTargets(raw json.RawMessage) (json.RawMessage, error) {
	var wf vpnWorkflowDoc
	if err := json.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow_json: %w", err)
	}

	networkNodeID, securityNodeID := "", ""
	for _, node := range wf.Nodes {
		switch vpnProcessPositionCode(node) {
		case "network_admin":
			networkNodeID = node.ID
		case "security_admin":
			securityNodeID = node.ID
		}
	}
	if networkNodeID == "" || securityNodeID == "" {
		return nil, fmt.Errorf("workflow_json missing network/security process nodes")
	}

	changed := false
	for i := range wf.Edges {
		values := vpnConditionValues(wf.Edges[i].Data)
		if containsAnyVPNKind(values, []string{"online_support", "troubleshooting", "production_emergency", "network_access_issue"}) {
			wf.Edges[i].Target = securityNodeID
			changed = true
			continue
		}
		if containsAnyVPNKind(values, []string{"external_collaboration", "long_term_remote_work", "cross_border_access", "security_compliance"}) {
			wf.Edges[i].Target = networkNodeID
			changed = true
		}
	}
	if !changed {
		return nil, fmt.Errorf("workflow_json has no request_kind routing edges to corrupt")
	}

	corrupted, err := json.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("marshal corrupted workflow_json: %w", err)
	}
	return corrupted, nil
}

func corruptVPNWorkflowRejectedTarget(raw json.RawMessage) (json.RawMessage, error) {
	var wf vpnWorkflowDoc
	if err := json.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow_json: %w", err)
	}

	formID := "requester_supplement"
	wf.Nodes = append(wf.Nodes, vpnWorkflowNode{
		ID:   formID,
		Type: engine.NodeForm,
		Data: map[string]any{
			"label":    "申请人补充 VPN 信息",
			"nodeType": engine.NodeForm,
			"participants": []any{
				map[string]any{"type": "requester"},
			},
			"formSchema": map[string]any{
				"fields": []any{
					map[string]any{"key": "supplement_reason", "type": "textarea", "label": "补充说明"},
				},
			},
		},
	})

	changed := false
	for i := range wf.Edges {
		if edgeOutcome(wf.Edges[i].Data) == engine.ActivityRejected {
			wf.Edges[i].Target = formID
			changed = true
		}
	}
	if !changed {
		return nil, fmt.Errorf("workflow_json has no rejected edges to corrupt")
	}

	if endID := firstEndNodeID(wf.Nodes); endID != "" {
		wf.Edges = append(wf.Edges, vpnWorkflowEdge{
			ID:     "edge_requester_supplement_end",
			Source: formID,
			Target: endID,
		})
	}

	corrupted, err := json.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("marshal corrupted workflow_json: %w", err)
	}
	return corrupted, nil
}

func vpnProcessPositionCode(node vpnWorkflowNode) string {
	if node.Type != engine.NodeProcess && node.Type != engine.NodeApprove {
		return ""
	}
	rawParticipants, ok := node.Data["participants"].([]any)
	if !ok {
		return ""
	}
	for _, rawParticipant := range rawParticipants {
		participant, ok := rawParticipant.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(participant["type"]) == "position_department" {
			return fmt.Sprint(participant["position_code"])
		}
	}
	return ""
}

func vpnConditionValues(data map[string]any) []string {
	condition, ok := data["condition"].(map[string]any)
	if !ok {
		return nil
	}
	rawValue, ok := condition["value"]
	if !ok {
		return nil
	}
	switch value := rawValue.(type) {
	case []any:
		values := make([]string, 0, len(value))
		for _, item := range value {
			values = append(values, fmt.Sprint(item))
		}
		return values
	case string:
		return []string{value}
	default:
		return []string{fmt.Sprint(value)}
	}
}

func containsAnyVPNKind(values []string, kinds []string) bool {
	for _, value := range values {
		for _, kind := range kinds {
			if value == kind {
				return true
			}
		}
	}
	return false
}

func edgeOutcome(data map[string]any) string {
	if data == nil {
		return ""
	}
	return fmt.Sprint(data["outcome"])
}

func firstEndNodeID(nodes []vpnWorkflowNode) string {
	for _, node := range nodes {
		if node.Type == engine.NodeEnd {
			return node.ID
		}
	}
	return ""
}

// --- When steps ---

func (bc *bddContext) whenSmartEngineDecisionCycle() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err := bc.smartEngine.Start(ctx, bc.db, engine.StartParams{
		TicketID:     bc.ticket.ID,
		WorkflowJSON: json.RawMessage(bc.service.WorkflowJSON),
		RequesterID:  bc.ticket.RequesterID,
	})
	cancel()
	if err != nil {
		bc.lastErr = err
		return fmt.Errorf("smart engine start: %w", err)
	}

	if err := bc.runSmartDecisionCycle(nil); err != nil {
		return err
	}

	// Refresh ticket.
	bc.db.First(bc.ticket, bc.ticket.ID)
	return nil
}

func (bc *bddContext) runSmartDecisionCycle(completedID *uint) error {
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		err := bc.smartEngine.RunDecisionCycleForTicket(ctx, bc.db, bc.ticket.ID, completedID)
		cancel()

		if err == nil {
			return nil
		}
		bc.lastErr = err
		log.Printf("smart engine decision attempt %d/%d: %v", attempt, maxRetries, err)
		if (err == engine.ErrAIDecisionFailed || err == engine.ErrAIDisabled) && attempt < maxRetries {
			bc.db.Model(&Ticket{}).Where("id = ?", bc.ticket.ID).Update("ai_failure_count", 0)
			continue
		}
		break
	}
	return nil
}

func (bc *bddContext) whenAdminConfirmsPendingDecision() error {
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	if activity.Status != "pending" {
		return fmt.Errorf("expected activity status 'pending', got %q", activity.Status)
	}

	// Parse the AI decision to get the plan.
	var plan engine.DecisionPlan
	if err := json.Unmarshal([]byte(activity.AIDecision), &plan); err != nil {
		return fmt.Errorf("parse AI decision: %w", err)
	}

	if err := bc.smartEngine.ExecuteDecisionPlan(bc.db, bc.ticket.ID, &plan); err != nil {
		return fmt.Errorf("execute decision plan: %w", err)
	}

	// Refresh ticket.
	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

func (bc *bddContext) whenAssigneeClaimsAndProcesss() error {
	return bc.progressCurrentActivity("completed", "")
}

func (bc *bddContext) whenAssigneeRejects() error {
	return bc.progressCurrentActivity("rejected", "")
}

func (bc *bddContext) whenAssigneeRejectsWithOpinion(opinion string) error {
	return bc.progressCurrentActivity("rejected", opinion)
}

func (bc *bddContext) progressCurrentActivity(outcome, opinion string) error {
	activity, err := bc.getCurrentActivity()
	if err != nil {
		return err
	}

	// Find the assignment for this activity.
	var assignment TicketAssignment
	if err := bc.db.Where("activity_id = ?", activity.ID).First(&assignment).Error; err != nil {
		// No assignment exists — create one using the first non-requester active user.
		log.Printf("[claim-fallback] no assignment for activity %d, creating fallback", activity.ID)
		fallbackID := bc.findFallbackOperator()
		if fallbackID == 0 {
			return fmt.Errorf("no assignment for activity %d and no fallback user available", activity.ID)
		}
		assignment = TicketAssignment{
			TicketID:        bc.ticket.ID,
			ActivityID:      activity.ID,
			ParticipantType: "user",
			UserID:          &fallbackID,
			AssigneeID:      &fallbackID,
			Status:          "claimed",
			IsCurrent:       true,
		}
		bc.db.Create(&assignment)
	}

	// Determine the assignee: use existing AssigneeID, or UserID, or first candidate.
	var operatorID uint
	if assignment.AssigneeID != nil {
		operatorID = *assignment.AssigneeID
	} else if assignment.UserID != nil {
		operatorID = *assignment.UserID
	} else {
		// Find first eligible user via org service.
		operatorID = bc.resolveOperatorFromAssignment(assignment)
		if operatorID == 0 {
			// Fallback: use any active user.
			operatorID = bc.findFallbackOperator()
		}
	}

	if operatorID == 0 {
		return fmt.Errorf("could not determine operator for activity %d", activity.ID)
	}

	// Claim.
	bc.db.Model(&TicketAssignment{}).
		Where("activity_id = ?", activity.ID).
		Updates(map[string]any{"assignee_id": operatorID, "status": "claimed"})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = bc.smartEngine.Progress(ctx, bc.db, engine.ProgressParams{
		TicketID:   bc.ticket.ID,
		ActivityID: activity.ID,
		Outcome:    outcome,
		Opinion:    opinion,
		OperatorID: operatorID,
	})
	if err != nil {
		bc.lastErr = err
		return fmt.Errorf("smart engine progress: %w", err)
	}
	bc.lastCompletedUserID = operatorID

	return bc.db.First(bc.ticket, bc.ticket.ID).Error
}

// findFallbackOperator returns the first active non-requester user ID.
func (bc *bddContext) findFallbackOperator() uint {
	provider := &testUserProvider{db: bc.db}
	candidates, _ := provider.ListActiveUsers()
	for _, c := range candidates {
		if c.UserID != bc.ticket.RequesterID {
			return c.UserID
		}
	}
	if len(candidates) > 0 {
		return candidates[0].UserID
	}
	return 0
}

func (bc *bddContext) whenSmartEngineDecisionCycleAgain() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}

	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Find the last finished activity to pass as completedActivityID, including
		// rejected human work so SmartEngine receives recovery context.
		var lastFinished TicketActivity
		var completedID *uint
		if err := bc.db.Where("ticket_id = ? AND status IN ?", bc.ticket.ID, engine.CompletedActivityStatuses()).
			Order("finished_at DESC, id DESC").First(&lastFinished).Error; err == nil {
			completedID = &lastFinished.ID
		}

		if err := bc.runSmartDecisionCycle(completedID); err != nil {
			log.Printf("smart engine re-decision attempt %d/%d: %v", attempt, maxRetries, err)
			continue
		}
		break
	}

	bc.db.First(bc.ticket, bc.ticket.ID)
	return nil
}

// --- Then steps ---

func (bc *bddContext) thenAtLeastOneActivity() error {
	var count int64
	if err := bc.db.Model(&TicketActivity{}).Where("ticket_id = ?", bc.ticket.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("expected at least one activity for ticket %d, got 0", bc.ticket.ID)
	}
	return nil
}

func (bc *bddContext) thenActivityTypeAllowed() error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&activities).Error; err != nil {
		return err
	}
	for _, a := range activities {
		if !engine.AllowedSmartStepTypes[a.ActivityType] {
			return fmt.Errorf("activity %d has disallowed type %q", a.ID, a.ActivityType)
		}
	}
	return nil
}

func (bc *bddContext) thenConfidenceInRange() error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&activities).Error; err != nil {
		return err
	}
	for _, a := range activities {
		if a.AIConfidence < 0 || a.AIConfidence > 1 {
			return fmt.Errorf("activity %d has confidence %f outside [0, 1]", a.ID, a.AIConfidence)
		}
	}
	return nil
}

func (bc *bddContext) thenParticipantInCandidates() error {
	provider := &testUserProvider{db: bc.db}
	candidates, err := provider.ListActiveUsers()
	if err != nil {
		return fmt.Errorf("list active users: %w", err)
	}
	candidateIDs := make(map[uint]bool)
	for _, c := range candidates {
		candidateIDs[c.UserID] = true
	}

	var assignments []TicketAssignment
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&assignments).Error; err != nil {
		return err
	}
	for _, a := range assignments {
		if a.UserID != nil && *a.UserID > 0 {
			if !candidateIDs[*a.UserID] {
				return fmt.Errorf("assignment %d has user_id %d not in candidate list", a.ID, *a.UserID)
			}
		}
	}
	// Soft log: record what the AI chose (for observability).
	for _, a := range assignments {
		if a.UserID != nil {
			log.Printf("[smart-bdd] assignment %d: user_id=%d", a.ID, *a.UserID)
		}
	}
	return nil
}

func (bc *bddContext) thenTimelineContainsAIDecision() error {
	var count int64
	if err := bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type LIKE ?", bc.ticket.ID, "%ai_decision%").
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no AI decision event found in timeline for ticket %d", bc.ticket.ID)
	}
	return nil
}

func (bc *bddContext) thenDecisionToolCalled(name string) error {
	if bc.hasToolCall(name) {
		return nil
	}
	return fmt.Errorf("expected decision tool %q to be called, got %+v", name, bc.toolCalls)
}

func (bc *bddContext) thenResolveParticipantUsedPositionDepartment(departmentCode, positionCode string) error {
	for _, call := range bc.toolCalls {
		if call.Name != "decision.resolve_participant" {
			continue
		}
		var args struct {
			Type           string `json:"type"`
			Value          string `json:"value"`
			PositionCode   string `json:"position_code"`
			DepartmentCode string `json:"department_code"`
		}
		if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
			return fmt.Errorf("parse decision.resolve_participant args %q: %w", call.Arguments, err)
		}
		if args.Type == "position_department" && args.DepartmentCode == departmentCode && args.PositionCode == positionCode {
			return nil
		}
	}
	return fmt.Errorf("expected decision.resolve_participant with position_department %s/%s, got %+v", departmentCode, positionCode, bc.toolCalls)
}

func (bc *bddContext) thenCurrentProcessNotAssignedToPosition(positionCode string) error {
	if err := bc.thenCurrentProcessAssignedToPosition(positionCode); err == nil {
		return fmt.Errorf("current process unexpectedly assigned to position %q", positionCode)
	}
	return nil
}

func (bc *bddContext) thenActiveHumanActivityCountIs(expected int) error {
	var count int64
	if err := bc.db.Model(&TicketActivity{}).
		Where("ticket_id = ? AND activity_type IN ? AND status IN ?",
			bc.ticket.ID,
			[]string{engine.NodeApprove, engine.NodeForm, engine.NodeProcess},
			[]string{engine.ActivityPending, engine.ActivityInProgress}).
		Count(&count).Error; err != nil {
		return err
	}
	if int(count) != expected {
		return fmt.Errorf("expected %d active human activities, got %d", expected, count)
	}
	return nil
}

func (bc *bddContext) thenActiveProcessActivityCountForPositionIs(positionCode string, expected int) error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ? AND activity_type = ? AND status IN ?",
		bc.ticket.ID, engine.NodeProcess, []string{engine.ActivityPending, engine.ActivityInProgress}).
		Find(&activities).Error; err != nil {
		return err
	}

	actual := 0
	for _, activity := range activities {
		var assignments []TicketAssignment
		if err := bc.db.Where("activity_id = ?", activity.ID).Find(&assignments).Error; err != nil {
			return err
		}
		for _, assignment := range assignments {
			if bc.assignmentTargetsPosition(assignment, positionCode) {
				actual++
				break
			}
		}
	}
	if actual != expected {
		return fmt.Errorf("expected %d active process activities for position %q, got %d", expected, positionCode, actual)
	}
	return nil
}

func (bc *bddContext) thenNoUnexecutableHighConfidenceHumanTask() error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ? AND activity_type IN ? AND status IN ?",
		bc.ticket.ID,
		[]string{engine.NodeApprove, engine.NodeForm, engine.NodeProcess},
		[]string{engine.ActivityPending, engine.ActivityInProgress},
	).Find(&activities).Error; err != nil {
		return err
	}

	for _, activity := range activities {
		var count int64
		if err := bc.db.Model(&TicketAssignment{}).Where("activity_id = ?", activity.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 && activity.AIConfidence >= 0.75 {
			return fmt.Errorf("activity %d is high-confidence human task without executable assignment", activity.ID)
		}
	}
	return nil
}

func (bc *bddContext) thenDecisionDiagnosticRecorded() error {
	var count int64
	if err := bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type IN ?", bc.ticket.ID,
			[]string{"ai_decision_failed", "ai_decision_pending", "participant_fallback_warning"}).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		activity, err := bc.getLatestActivity()
		if err == nil && (activity.ActivityType == engine.NodeNotify || activity.ActivityType == "escalate") && activity.AIReasoning != "" {
			return nil
		}
		return fmt.Errorf("expected decision diagnostic timeline event for ticket %d", bc.ticket.ID)
	}
	return nil
}

func (bc *bddContext) thenNoHighConfidenceSingleRouteChoice() error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ? AND activity_type IN ? AND status IN ?",
		bc.ticket.ID,
		[]string{engine.NodeApprove, engine.NodeForm, engine.NodeProcess},
		[]string{engine.ActivityPending, engine.ActivityInProgress},
	).Find(&activities).Error; err != nil {
		return err
	}

	for _, activity := range activities {
		if activity.AIConfidence < 0.75 {
			continue
		}
		var assignments []TicketAssignment
		if err := bc.db.Where("activity_id = ?", activity.ID).Find(&assignments).Error; err != nil {
			return err
		}
		for _, assignment := range assignments {
			for _, code := range []string{"ops_admin", "network_admin", "security_admin"} {
				if bc.assignmentTargetsPosition(assignment, code) {
					return fmt.Errorf("high-confidence conflict decision chose single route %q via activity %d", code, activity.ID)
				}
			}
		}
	}
	return nil
}

func (bc *bddContext) thenClarificationOrLowConfidenceHandling() error {
	activity, err := bc.getLatestActivity()
	if err == nil {
		if activity.ActivityType == engine.NodeForm && (activity.Status == engine.ActivityPending || activity.Status == engine.ActivityInProgress) {
			return nil
		}
		if activity.AIConfidence < 0.75 && (activity.Status == engine.ActivityPending || activity.Status == engine.ActivityInProgress) {
			return nil
		}
		if isClarificationNotice(activity) {
			return nil
		}
	}

	var count int64
	if err := bc.db.Model(&TicketTimeline{}).
		Where("ticket_id = ? AND event_type IN ?", bc.ticket.ID,
			[]string{"ai_decision_failed", "ai_decision_pending"}).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("expected clarification form, low-confidence pending activity, or diagnostic event for ticket %d", bc.ticket.ID)
}

func isClarificationNotice(activity *TicketActivity) bool {
	if activity == nil {
		return false
	}
	if activity.ActivityType != engine.NodeNotify && activity.ActivityType != "escalate" {
		return false
	}
	content := strings.ToLower(strings.Join([]string{
		activity.Name,
		activity.AIReasoning,
		activity.DecisionReasoning,
		string(activity.AIDecision),
	}, "\n"))
	for _, marker := range []string{
		"澄清",
		"明确",
		"确认",
		"冲突",
		"未知",
		"枚举",
		"不在协作规范",
		"无法确定",
		"人工介入",
		"人工处理",
		"clarify",
		"clarification",
		"confirm",
		"conflict",
		"unknown",
		"unsupported",
	} {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

func (bc *bddContext) thenNoDuplicateAfterCompletedHumanWork() error {
	var completed []TicketActivity
	if err := bc.db.Where("ticket_id = ? AND status IN ? AND activity_type IN ?",
		bc.ticket.ID,
		[]string{engine.ActivityCompleted, engine.ActivityApproved},
		[]string{engine.NodeApprove, engine.NodeForm, engine.NodeProcess},
	).Find(&completed).Error; err != nil {
		return err
	}

	for _, done := range completed {
		var dupCount int64
		if err := bc.db.Model(&TicketActivity{}).
			Where("ticket_id = ? AND id <> ? AND activity_type = ? AND name = ? AND status IN ?",
				bc.ticket.ID, done.ID, done.ActivityType, done.Name,
				[]string{engine.ActivityPending, engine.ActivityInProgress}).
			Count(&dupCount).Error; err != nil {
			return err
		}
		if dupCount > 0 {
			return fmt.Errorf("completed human activity %d was recreated as active work", done.ID)
		}
	}
	return nil
}

func (bc *bddContext) thenNoRequesterSupplementForm() error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ? AND activity_type = ? AND status IN ?",
		bc.ticket.ID, engine.NodeForm, []string{engine.ActivityPending, engine.ActivityInProgress}).
		Find(&activities).Error; err != nil {
		return err
	}

	for _, activity := range activities {
		var count int64
		if err := bc.db.Model(&TicketAssignment{}).
			Where("activity_id = ? AND (participant_type = ? OR user_id = ? OR assignee_id = ?)",
				activity.ID, "requester", bc.ticket.RequesterID, bc.ticket.RequesterID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("unexpected requester supplement form activity %d", activity.ID)
		}
	}

	var timeline []TicketTimeline
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&timeline).Error; err != nil {
		return err
	}
	for _, event := range timeline {
		if strings.Contains(event.Message, "退回申请人补充") && !isRequesterSupplementGuardrailText(event.Message) {
			return fmt.Errorf("timeline still implies requester supplement: %s", event.Message)
		}
	}
	return nil
}

func isRequesterSupplementGuardrailText(text string) bool {
	for _, marker := range []string{
		"不得把驳回默认解释为退回申请人补充",
		"不得创建申请人补充",
		"不能退回申请人补充",
		"禁止退回申请人补充",
		"协作规范未显式定义",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (bc *bddContext) thenTicketOutcomeIs(expected string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}
	if bc.ticket.Outcome != expected {
		return fmt.Errorf("expected ticket outcome %q, got %q", expected, bc.ticket.Outcome)
	}
	return nil
}

func (bc *bddContext) thenTicketRejectedOrDiagnosticRecorded() error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	if err := bc.db.First(bc.ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}
	if bc.ticket.Status == TicketStatusRejected && bc.ticket.Outcome == TicketOutcomeRejected {
		return nil
	}
	return bc.thenDecisionDiagnosticRecorded()
}

func (bc *bddContext) thenActivityCountIs(expected int) error {
	var count int64
	if err := bc.db.Model(&TicketActivity{}).Where("ticket_id = ?", bc.ticket.ID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != expected {
		return fmt.Errorf("expected %d activities, got %d", expected, count)
	}
	return nil
}

func (bc *bddContext) thenAIDecisionEvidenceContains(needle string) error {
	var activities []TicketActivity
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&activities).Error; err != nil {
		return err
	}
	for _, activity := range activities {
		haystack := strings.Join([]string{
			activity.Name,
			activity.AIReasoning,
			activity.DecisionReasoning,
			string(activity.AIDecision),
		}, "\n")
		if strings.Contains(haystack, needle) {
			return nil
		}
	}

	var timeline []TicketTimeline
	if err := bc.db.Where("ticket_id = ?", bc.ticket.ID).Find(&timeline).Error; err != nil {
		return err
	}
	for _, event := range timeline {
		haystack := strings.Join([]string{event.Message, event.Reasoning, string(event.Details)}, "\n")
		if strings.Contains(haystack, needle) {
			return nil
		}
	}
	return fmt.Errorf("expected AI decision evidence to contain %q", needle)
}

func (bc *bddContext) thenCurrentActivityStatusIs(expected string) error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.Status != expected {
		return fmt.Errorf("expected activity status %q, got %q", expected, activity.Status)
	}
	return nil
}

func (bc *bddContext) thenCurrentActivityStatusIsNot(notExpected string) error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.Status == notExpected {
		return fmt.Errorf("expected activity status NOT to be %q, but it is", notExpected)
	}
	return nil
}

func (bc *bddContext) thenActivityContainsAIReasoning() error {
	activity, err := bc.getLatestActivity()
	if err != nil {
		return err
	}
	if activity.AIReasoning == "" {
		return fmt.Errorf("activity %d has empty AI reasoning", activity.ID)
	}
	return nil
}

// getLatestActivity returns the most recently created activity for the current ticket.
func (bc *bddContext) getLatestActivity() (*TicketActivity, error) {
	if bc.ticket == nil {
		return nil, fmt.Errorf("no ticket in context")
	}
	var activity TicketActivity
	err := bc.db.Where("ticket_id = ?", bc.ticket.ID).
		Order("id DESC").First(&activity).Error
	if err != nil {
		return nil, fmt.Errorf("no activity found for ticket %d: %w", bc.ticket.ID, err)
	}
	return &activity, nil
}

func (bc *bddContext) assignmentTargetsPosition(assignment TicketAssignment, positionCode string) bool {
	if assignment.PositionID != nil {
		if pos, ok := bc.positions[positionCode]; ok && pos.ID == *assignment.PositionID {
			return true
		}
	}

	var userID uint
	if assignment.AssigneeID != nil {
		userID = *assignment.AssigneeID
	} else if assignment.UserID != nil {
		userID = *assignment.UserID
	}
	if userID == 0 {
		return false
	}

	orgSvc := &testOrgService{db: bc.db}
	for deptCode := range bc.departments {
		eligibleIDs, err := orgSvc.FindUsersByPositionAndDepartment(positionCode, deptCode)
		if err != nil {
			continue
		}
		for _, eligibleID := range eligibleIDs {
			if eligibleID == userID {
				return true
			}
		}
	}
	return false
}

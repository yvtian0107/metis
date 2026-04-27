package bdd

import (
	"context"
	"encoding/json"
	"fmt"
	. "metis/internal/app/itsm/domain"
	"time"

	"github.com/cucumber/godog"

	ai "metis/internal/app/ai/runtime"
	"metis/internal/app/itsm/engine"
)

const bddSLAAssuranceAgentPrompt = `你是 SLA 保障岗，负责监督智能 ITSM 工单的 SLA 风险并在规则命中时触发升级动作。

操作必须按顺序执行：
1. 调用 sla.risk_queue 读取风险队列。
2. 对候选工单调用 sla.ticket_context 读取上下文。
3. 调用 sla.escalation_rules 读取已命中升级规则。
4. 规则允许时调用 sla.trigger_escalation 触发升级动作。

不得跳过工具调用直接回答。`

func registerSLAAssuranceSteps(sc *godog.ScenarioContext, bc *bddContext) {
	sc.Given(`^已发布带 SLA 的智能服务和 SLA 保障岗$`, bc.givenPublishedSLAServiceAndAssuranceAgent)
	sc.Given(`^存在响应 SLA 已超时且命中 "([^"]*)" 升级规则的工单$`, bc.givenResponseSLABreachedTicketWithRule)
	sc.When(`^执行 SLA 保障扫描$`, bc.whenRunSLAAssuranceScan)
	sc.Then(`^SLA 保障岗已调用工具 "([^"]*)"$`, bc.thenSLAAssuranceToolCalled)
	sc.Then(`^工单已转派给 "([^"]*)"$`, bc.thenTicketAssignedToUser)
	sc.Then(`^工单优先级为 "([^"]*)"$`, bc.thenTicketPriorityIs)
}

func (bc *bddContext) givenPublishedSLAServiceAndAssuranceAgent() error {
	catalog := &ServiceCatalog{Name: "SLA 保障测试目录", Code: "sla-assurance", IsActive: true}
	if err := bc.db.Create(catalog).Error; err != nil {
		return fmt.Errorf("create catalog: %w", err)
	}

	normal := &Priority{Name: "普通", Code: "normal", Value: 3, Color: "#52c41a", IsActive: true}
	if err := bc.db.Create(normal).Error; err != nil {
		return fmt.Errorf("create normal priority: %w", err)
	}
	bc.priority = normal

	urgent := &Priority{Name: "紧急", Code: "urgent", Value: 1, Color: "#f5222d", IsActive: true}
	if err := bc.db.Create(urgent).Error; err != nil {
		return fmt.Errorf("create urgent priority: %w", err)
	}

	sla := &SLATemplate{
		Name:              "BDD 响应 SLA",
		Code:              "bdd-response-sla",
		ResponseMinutes:   1,
		ResolutionMinutes: 60,
		IsActive:          true,
	}
	if err := bc.db.Create(sla).Error; err != nil {
		return fmt.Errorf("create sla template: %w", err)
	}

	decisionAgent := &ai.Agent{
		Name:         "BDD 流程决策智能体",
		Type:         ai.AgentTypeAssistant,
		IsActive:     true,
		Visibility:   "private",
		Strategy:     ai.AgentStrategyReact,
		SystemPrompt: decisionAgentSystemPrompt,
		Temperature:  0.2,
		MaxTokens:    4096,
		MaxTurns:     8,
	}
	if err := bc.db.Create(decisionAgent).Error; err != nil {
		return fmt.Errorf("create decision agent: %w", err)
	}

	slaAgent := &ai.Agent{
		Name:         "BDD SLA 保障智能体",
		Type:         ai.AgentTypeAssistant,
		IsActive:     true,
		Visibility:   "private",
		Strategy:     ai.AgentStrategyReact,
		SystemPrompt: bddSLAAssuranceAgentPrompt,
		Temperature:  0.1,
		MaxTokens:    4096,
		MaxTurns:     8,
	}
	if err := bc.db.Create(slaAgent).Error; err != nil {
		return fmt.Errorf("create sla assurance agent: %w", err)
	}
	bc.slaAssuranceAgentID = slaAgent.ID

	svc := &ServiceDefinition{
		Name:              "SLA 保障智能服务",
		Code:              "sla-assurance-smart-service",
		CatalogID:         catalog.ID,
		EngineType:        "smart",
		SLAID:             &sla.ID,
		CollaborationSpec: "SLA 保障 BDD 使用的智能服务，工单由 SLA 保障岗扫描并按规则升级。",
		AgentID:           &decisionAgent.ID,
		IsActive:          true,
	}
	if err := bc.db.Create(svc).Error; err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	bc.service = svc
	return nil
}

func (bc *bddContext) givenResponseSLABreachedTicketWithRule(actionType string) error {
	if bc.service == nil || bc.service.SLAID == nil {
		return fmt.Errorf("SLA service is not prepared")
	}
	requester, ok := bc.users["申请人"]
	if !ok {
		return fmt.Errorf("申请人 not found")
	}
	current, ok := bc.users["当前处理人"]
	if !ok {
		return fmt.Errorf("当前处理人 not found")
	}

	targetConfig, err := bc.slaTargetConfig(actionType)
	if err != nil {
		return err
	}
	rule := &EscalationRule{
		SLAID:        *bc.service.SLAID,
		TriggerType:  "response_timeout",
		Level:        1,
		WaitMinutes:  0,
		ActionType:   actionType,
		TargetConfig: JSONField(targetConfig),
		IsActive:     true,
	}
	if err := bc.db.Create(rule).Error; err != nil {
		return fmt.Errorf("create escalation rule: %w", err)
	}

	now := time.Now()
	responseDeadline := now.Add(-5 * time.Minute)
	resolutionDeadline := now.Add(55 * time.Minute)
	ticket := &Ticket{
		Code:                  fmt.Sprintf("SLA-BDD-%d", now.UnixNano()),
		Title:                 "生产访问异常待处理",
		Description:           "SLA 保障岗 BDD 风险工单",
		ServiceID:             bc.service.ID,
		EngineType:            "smart",
		Status:                TicketStatusWaitingHuman,
		PriorityID:            bc.priority.ID,
		RequesterID:           requester.ID,
		AssigneeID:            &current.ID,
		Source:                TicketSourceAgent,
		FormData:              JSONField(`{"impact":"production"}`),
		SLAResponseDeadline:   &responseDeadline,
		SLAResolutionDeadline: &resolutionDeadline,
		SLAStatus:             SLAStatusOnTrack,
	}
	if err := bc.db.Create(ticket).Error; err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	bc.ticket = ticket
	return nil
}

func (bc *bddContext) slaTargetConfig(actionType string) ([]byte, error) {
	switch actionType {
	case "notify":
		current, ok := bc.users["当前处理人"]
		if !ok {
			return nil, fmt.Errorf("当前处理人 not found")
		}
		return json.Marshal(map[string]any{
			"recipients": []map[string]any{{"type": "user", "value": fmt.Sprintf("%d", current.ID)}},
			"channelId":  1,
		})
	case "reassign":
		target, ok := bc.users["升级处理人"]
		if !ok {
			return nil, fmt.Errorf("升级处理人 not found")
		}
		return json.Marshal(map[string]any{
			"assigneeCandidates": []map[string]any{{"type": "user", "value": fmt.Sprintf("%d", target.ID)}},
		})
	case "escalate_priority":
		var urgent Priority
		if err := bc.db.Where("code = ?", "urgent").First(&urgent).Error; err != nil {
			return nil, fmt.Errorf("load urgent priority: %w", err)
		}
		return json.Marshal(map[string]any{"priorityId": urgent.ID})
	default:
		return nil, fmt.Errorf("unsupported SLA action type %q", actionType)
	}
}

func (bc *bddContext) whenRunSLAAssuranceScan() error {
	bc.toolCalls = nil
	executor := &testDecisionExecutor{db: bc.db, llmCfg: bc.llmCfg, recordToolCall: bc.recordToolCall}
	handler := engine.HandleSLACheck(bc.db, &bddConfigProvider{bc: bc}, executor, engine.NewParticipantResolver(nil), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	if err := handler(ctx, nil); err != nil {
		bc.lastErr = err
		return err
	}
	return nil
}

func (bc *bddContext) thenSLAAssuranceToolCalled(name string) error {
	if bc.hasToolCall(name) {
		return nil
	}
	return fmt.Errorf("expected SLA assurance tool %q to be called, got %+v", name, bc.toolCalls)
}

func (bc *bddContext) thenTicketAssignedToUser(username string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	user, ok := bc.usersByName[username]
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	var ticket Ticket
	if err := bc.db.First(&ticket, bc.ticket.ID).Error; err != nil {
		return fmt.Errorf("refresh ticket: %w", err)
	}
	if ticket.AssigneeID == nil || *ticket.AssigneeID != user.ID {
		actual := uint(0)
		if ticket.AssigneeID != nil {
			actual = *ticket.AssigneeID
		}
		return fmt.Errorf("expected assignee %s(%d), got %d", username, user.ID, actual)
	}
	return nil
}

func (bc *bddContext) thenTicketPriorityIs(priorityCode string) error {
	if bc.ticket == nil {
		return fmt.Errorf("no ticket in context")
	}
	var priority Priority
	if err := bc.db.Table("itsm_priorities").
		Joins("JOIN itsm_tickets ON itsm_tickets.priority_id = itsm_priorities.id").
		Where("itsm_tickets.id = ?", bc.ticket.ID).
		First(&priority).Error; err != nil {
		return fmt.Errorf("load ticket priority: %w", err)
	}
	if priority.Code != priorityCode {
		return fmt.Errorf("expected ticket priority %q, got %q", priorityCode, priority.Code)
	}
	return nil
}

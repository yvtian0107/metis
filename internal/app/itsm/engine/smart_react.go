package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"metis/internal/llm"
)

// agenticDecision runs a ReAct tool-calling loop to produce a DecisionPlan.
// The agent queries context on-demand via decision tools instead of receiving
// all information upfront, enabling scalable multi-step reasoning.
func (e *SmartEngine) agenticDecision(ctx context.Context, tx *gorm.DB, ticketID uint, svc *serviceModel) (*DecisionPlan, error) {
	agentCfg, err := e.agentProvider.GetAgentConfigByCode("itsm.decision")
	if err != nil {
		return nil, fmt.Errorf("get decision agent config: %w", err)
	}

	// Build initial seed messages
	var decisionMode string
	if e.configProvider != nil {
		decisionMode = e.configProvider.DecisionMode()
	}
	systemMsg, userMsg, err := e.buildInitialSeed(tx, ticketID, svc, agentCfg, decisionMode)
	if err != nil {
		return nil, fmt.Errorf("build initial seed: %w", err)
	}

	// Create LLM client
	client, err := llm.NewClient(agentCfg.Protocol, agentCfg.BaseURL, agentCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	// Prepare tool context
	toolCtx := &decisionToolContext{
		tx:                tx,
		ticketID:          ticketID,
		serviceID:         svc.ID,
		knowledgeSearcher: e.knowledgeSearcher,
		resolver:          e.resolver,
	}

	// Build tool definitions and handler map
	tools := allDecisionTools()
	toolDefs := make([]llm.ToolDef, len(tools))
	handlerMap := make(map[string]func(*decisionToolContext, json.RawMessage) (json.RawMessage, error))
	for i, t := range tools {
		toolDefs[i] = t.Def
		handlerMap[t.Def.Name] = t.Handler
	}

	// Prepare conversation messages
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemMsg},
		{Role: llm.RoleUser, Content: userMsg},
	}

	temp := float32(agentCfg.Temperature)
	maxTokens := agentCfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// ReAct loop
	for turn := 0; turn < DecisionToolMaxTurns; turn++ {
		resp, err := client.Chat(ctx, llm.ChatRequest{
			Model:       agentCfg.Model,
			Messages:    messages,
			Tools:       toolDefs,
			MaxTokens:   maxTokens,
			Temperature: &temp,
		})
		if err != nil {
			return nil, fmt.Errorf("llm chat (turn %d): %w", turn, err)
		}

		// No tool calls → final answer
		if len(resp.ToolCalls) == 0 {
			slog.Info("agentic decision completed", "ticketID", ticketID, "turns", turn+1,
				"inputTokens", resp.Usage.InputTokens, "outputTokens", resp.Usage.OutputTokens)
			return parseDecisionPlan(resp.Content)
		}

		// Append assistant message with tool calls
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append results
		for _, tc := range resp.ToolCalls {
			result := executeDecisionTool(toolCtx, handlerMap, tc)
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    result,
				ToolCallID: tc.ID,
			})
			slog.Debug("decision tool called",
				"ticketID", ticketID, "turn", turn, "tool", tc.Name)
		}
	}

	return nil, fmt.Errorf("决策循环超过最大轮数 (%d)，未产生最终决策", DecisionToolMaxTurns)
}

// executeDecisionTool dispatches a tool call to the appropriate handler.
func executeDecisionTool(
	ctx *decisionToolContext,
	handlers map[string]func(*decisionToolContext, json.RawMessage) (json.RawMessage, error),
	tc llm.ToolCall,
) string {
	handler, ok := handlers[tc.Name]
	if !ok {
		result, _ := toolError(fmt.Sprintf("未知工具: %s", tc.Name))
		return string(result)
	}

	result, err := handler(ctx, json.RawMessage(tc.Arguments))
	if err != nil {
		errResult, _ := toolError(fmt.Sprintf("工具执行错误: %v", err))
		return string(errResult)
	}

	return string(result)
}

// buildInitialSeed constructs the system and user messages for the agentic decision.
// The seed is intentionally lightweight — the agent queries detailed context via tools.
func (e *SmartEngine) buildInitialSeed(tx *gorm.DB, ticketID uint, svc *serviceModel, agentCfg *SmartAgentConfig, decisionMode string) (string, string, error) {
	// --- System message ---
	systemMsg := buildAgenticSystemPrompt(svc.CollaborationSpec, agentCfg.SystemPrompt, decisionMode)

	// --- User message: lightweight ticket snapshot + policy constraints ---
	var ticket struct {
		Code        string
		Title       string
		Description string
		Status      string
		Source      string
		PriorityID  uint
	}
	if err := tx.Table("itsm_tickets").Where("id = ?", ticketID).
		Select("code, title, description, status, source, priority_id").
		First(&ticket).Error; err != nil {
		return "", "", fmt.Errorf("ticket not found: %w", err)
	}

	var priorityName string
	if ticket.PriorityID > 0 {
		var p struct{ Name string }
		if err := tx.Table("itsm_priorities").Where("id = ?", ticket.PriorityID).
			Select("name").First(&p).Error; err == nil {
			priorityName = p.Name
		}
	}

	// Determine allowed step types based on current status
	allowedSteps := []string{"approve", "process", "action", "notify", "form", "complete", "escalate"}
	switch ticket.Status {
	case "completed", "cancelled", "failed":
		allowedSteps = []string{}
	}

	seed := map[string]any{
		"ticket": map[string]any{
			"code":        ticket.Code,
			"title":       ticket.Title,
			"description": ticket.Description,
			"status":      ticket.Status,
			"source":      ticket.Source,
			"priority":    priorityName,
		},
		"service": map[string]any{
			"name":        svc.Name,
			"description": svc.Description,
		},
		"policy": map[string]any{
			"allowed_step_types": allowedSteps,
			"current_status":     ticket.Status,
		},
	}
	seedJSON, _ := json.MarshalIndent(seed, "", "  ")
	userMsg := fmt.Sprintf("## 工单信息与策略约束\n\n```json\n%s\n```\n\n请使用可用工具收集所需信息，然后输出最终决策。", seedJSON)

	return systemMsg, userMsg, nil
}

// buildAgenticSystemPrompt constructs the system prompt for the agentic decision agent.
func buildAgenticSystemPrompt(collaborationSpec, agentSystemPrompt, decisionMode string) string {
	prompt := ""
	if collaborationSpec != "" {
		prompt += "## 服务处理规范\n\n" + collaborationSpec + "\n\n---\n\n"
	}
	if agentSystemPrompt != "" {
		prompt += "## 角色定义\n\n" + agentSystemPrompt + "\n\n---\n\n"
	}
	// DecisionMode prompt injection
	switch decisionMode {
	case "ai_only":
		prompt += "## 决策策略\n\n始终使用 AI 推理决定下一步，不依赖预定义路径。\n\n---\n\n"
	default: // "direct_first" or empty
		prompt += "## 决策策略\n\n优先走确定路径（workflow_hints），无法确定时使用 AI 推理。\n\n---\n\n"
	}
	prompt += agenticToolGuidance
	prompt += "\n\n---\n\n"
	prompt += agenticOutputFormat
	return prompt
}

const agenticToolGuidance = `## 工具使用指引

你可以通过以下工具按需查询信息来辅助决策：

- **decision.ticket_context** — 获取工单完整上下文（表单数据、SLA、活动历史、当前指派）
- **decision.knowledge_search** — 搜索服务关联知识库
- **decision.resolve_participant** — 按类型解析参与人（user/position/department/position_department/requester_manager）
- **decision.user_workload** — 查询用户当前工单负载
- **decision.similar_history** — 查询同服务历史工单的处理模式
- **decision.sla_status** — 查询 SLA 状态和紧急程度
- **decision.list_actions** — 查询服务可用的自动化动作

### 推荐推理步骤

1. 先用 decision.ticket_context 了解完整上下文
2. 如需查阅处理规范，使用 decision.knowledge_search
3. 确定下一步类型后，用 decision.resolve_participant 解析指派人
4. 可选：用 decision.user_workload 做负载均衡，用 decision.similar_history 参考历史模式
5. 最终输出决策 JSON（不调用任何工具）`

const agenticOutputFormat = "## 输出要求\n\n" +
	"当你完成信息收集和推理后，直接输出以下 JSON 格式的决策（不要再调用任何工具）：\n\n" +
	"```json\n" +
	"{\n" +
	"  \"next_step_type\": \"process|approve|action|notify|form|complete|escalate\",\n" +
	"  \"activities\": [\n" +
	"    {\n" +
	"      \"type\": \"process|approve|action|notify|form\",\n" +
	"      \"participant_type\": \"user\",\n" +
	"      \"participant_id\": 42,\n" +
	"      \"action_id\": null,\n" +
	"      \"instructions\": \"操作指引\"\n" +
	"    }\n" +
	"  ],\n" +
	"  \"reasoning\": \"决策推理过程...\",\n" +
	"  \"confidence\": 0.85\n" +
	"}\n" +
	"```\n\n" +
	"字段说明：\n" +
	"- next_step_type: 下一步类型。\"complete\" 表示流程可以结束。\n" +
	"- activities: 需要创建的活动列表。\n" +
	"- reasoning: 你的推理过程（会展示给管理员审核）。\n" +
	"- confidence: 决策信心（0.0-1.0）。越高表示越确信。"

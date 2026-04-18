package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"metis/internal/llm"
)

// agenticDecision runs a ReAct tool-calling loop to produce a DecisionPlan.
// The agent queries context on-demand via decision tools instead of receiving
// all information upfront, enabling scalable multi-step reasoning.
func (e *SmartEngine) agenticDecision(ctx context.Context, tx *gorm.DB, ticketID uint, svc *serviceModel) (*DecisionPlan, error) {
	// Get decision agent ID from engine config
	var agentID uint
	if e.configProvider != nil {
		agentID = e.configProvider.DecisionAgentID()
	}
	if agentID == 0 {
		return nil, fmt.Errorf("决策智能体未配置")
	}

	agentCfg, err := e.agentProvider.GetAgentConfig(agentID)
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
		actionExecutor:    e.actionExecutor,
	}

	// Parse knowledge base IDs from service definition
	if svc.KnowledgeBaseIDs != "" {
		var kbIDs []uint
		if err := json.Unmarshal([]byte(svc.KnowledgeBaseIDs), &kbIDs); err == nil {
			toolCtx.knowledgeBaseIDs = kbIDs
		}
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

	var tempPtr *float32
	if agentCfg.Temperature != 0 {
		temp := float32(agentCfg.Temperature)
		tempPtr = &temp
	}
	maxTokens := agentCfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// ReAct loop
	for turn := 0; turn < DecisionToolMaxTurns; turn++ {
		chatReq := llm.ChatRequest{
			Model:       agentCfg.Model,
			Messages:    messages,
			Tools:       toolDefs,
			MaxTokens:   maxTokens,
			Temperature: tempPtr,
		}
		// NOTE: Do NOT set ResponseFormat during the ReAct loop.
		// json_object mode can cause the LLM to skip tool calls and output
		// JSON directly, breaking the tool-calling flow. The extractJSON
		// pipeline already handles parsing from free-form LLM output.
		resp, err := client.Chat(ctx, chatReq)
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
	systemMsg := buildAgenticSystemPrompt(svc.CollaborationSpec, agentCfg.SystemPrompt, decisionMode, svc.WorkflowJSON)

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
func buildAgenticSystemPrompt(collaborationSpec, agentSystemPrompt, decisionMode, workflowJSON string) string {
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
		hints := extractWorkflowHints(workflowJSON)
		if hints != "" {
			prompt += "## 决策策略\n\n优先参考以下工作流路径来决定下一步，无法确定时使用 AI 推理。\n\n"
			prompt += "## 工作流参考路径\n\n" + hints + "\n\n---\n\n"
		} else {
			// No workflow hints available — degrade to ai_only behavior
			slog.Warn("direct_first mode but no workflow hints available, degrading to ai_only")
			prompt += "## 决策策略\n\n始终使用 AI 推理决定下一步，不依赖预定义路径。\n\n---\n\n"
		}
	}
	prompt += agenticToolGuidance
	prompt += "\n\n---\n\n"
	prompt += agenticOutputFormat
	return prompt
}

const agenticToolGuidance = `## 工具使用指引

你可以通过以下工具按需查询信息来辅助决策：

- **decision.ticket_context** — 获取工单完整上下文（表单数据、SLA、活动历史、当前指派、已执行动作、并签组状态）
- **decision.knowledge_search** — 搜索服务关联知识库
- **decision.resolve_participant** — 按类型解析参与人（user/position/department/position_department/requester_manager）
- **decision.user_workload** — 查询用户当前工单负载
- **decision.similar_history** — 查询同服务历史工单的处理模式
- **decision.sla_status** — 查询 SLA 状态和紧急程度
- **decision.list_actions** — 查询服务可用的自动化动作
- **decision.execute_action** — 同步执行服务配置的自动化动作（webhook），返回执行结果。在决策推理过程中直接触发自动化操作并观察结果。

### 推荐推理步骤

1. 先用 decision.ticket_context 了解完整上下文（注意 activity_history、executed_actions 和 parallel_groups）
2. 用 decision.list_actions 查看是否有可用的自动化动作
3. 如需查阅处理规范，使用 decision.knowledge_search
4. 如果协作规范要求执行某个动作（如预检、放行等），使用 decision.execute_action 同步执行并获取结果，而非输出 type 为 "action" 的活动
5. 如果需要多角色并行审批（并签），设置 execution_mode 为 "parallel"，在 activities 中列出所有并行角色
6. 如果需要人工审批，用 decision.resolve_participant 解析指派人
7. 可选：用 decision.user_workload 做负载均衡，用 decision.similar_history 参考历史模式
8. 最终输出决策 JSON（不调用任何工具）

### 完成判断

当 decision.ticket_context 返回的 all_actions_completed 为 true，表示服务配置的所有自动化动作均已成功执行。此时请对照协作规范判断流程是否应当结束——如果规范说"动作完成后结束"，你必须输出 next_step_type 为 "complete"，不要再创建新活动。`

const agenticOutputFormat = "## 输出要求\n\n" +
	"当你完成信息收集和推理后，直接输出以下 JSON 格式的决策（不要再调用任何工具）：\n\n" +
	"```json\n" +
	"{\n" +
	"  \"next_step_type\": \"process|approve|action|notify|form|complete|escalate\",\n" +
	"  \"execution_mode\": \"single|parallel\",\n" +
	"  \"activities\": [\n" +
	"    {\n" +
	"      \"type\": \"process|approve|action|notify|form\",\n" +
	"      \"participant_type\": \"user|position_department\",\n" +
	"      \"participant_id\": 42,\n" +
	"      \"position_code\": \"db_admin\",\n" +
	"      \"department_code\": \"it\",\n" +
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
	"- execution_mode: 执行模式。\"single\"（默认）为串行，\"parallel\" 为并签模式——activities 中的多个活动将并行等待处理，全部完成后才推进到下一步。当协作规范要求多角色并行审批时使用 \"parallel\"。\n" +
	"- activities: 需要创建的活动列表。\n" +
	"- participant_type: \"user\" 需填 participant_id；\"position_department\" 需填 position_code + department_code。\n" +
	"- position_code / department_code: 当 participant_type 为 position_department 时，填写岗位编码和部门编码。\n" +
	"- action_id: 当 type 为 \"action\" 时，填写 decision.list_actions 返回的 action id。\n" +
	"- reasoning: 你的推理过程（会展示给管理员审核）。\n" +
	"- confidence: 决策信心（0.0-1.0）。越高表示越确信。"

// extractWorkflowHints extracts a structured step summary from WorkflowJSON
// for injection into the system prompt in direct_first mode.
func extractWorkflowHints(workflowJSON string) string {
	if workflowJSON == "" {
		return ""
	}

	var wf struct {
		Nodes []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Data struct {
				Label        string `json:"label"`
				Participants []struct {
					Type           string `json:"type"`
					Value          string `json:"value"`
					PositionCode   string `json:"position_code"`
					DepartmentCode string `json:"department_code"`
				} `json:"participants"`
				ApproveMode      string `json:"approve_mode"`
				GatewayDirection string `json:"gateway_direction"`
				ActionID         *uint  `json:"action_id"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
			Target string `json:"target"`
			Data   struct {
				Outcome   string `json:"outcome"`
				Default   bool   `json:"default"`
				Condition *struct {
					Field    string `json:"field"`
					Operator string `json:"operator"`
					Value    any    `json:"value"`
				} `json:"condition"`
			} `json:"data"`
		} `json:"edges"`
	}

	if err := json.Unmarshal([]byte(workflowJSON), &wf); err != nil {
		return ""
	}

	if len(wf.Nodes) == 0 {
		return ""
	}

	// Build node map and adjacency list
	nodeMap := make(map[string]int) // id → index
	for i, n := range wf.Nodes {
		nodeMap[n.ID] = i
	}

	outEdges := make(map[string][]int) // source → edge indices
	for i, e := range wf.Edges {
		outEdges[e.Source] = append(outEdges[e.Source], i)
	}

	// Walk from start node to build hints
	var startID string
	for _, n := range wf.Nodes {
		if n.Type == "start" {
			startID = n.ID
			break
		}
	}
	if startID == "" {
		return ""
	}

	var hints []string
	step := 1
	visited := make(map[string]bool)
	queue := []string{startID}

	for len(queue) > 0 && step <= 20 { // cap at 20 steps
		nodeID := queue[0]
		queue = queue[1:]

		if visited[nodeID] {
			continue
		}
		visited[nodeID] = true

		idx, ok := nodeMap[nodeID]
		if !ok {
			continue
		}
		node := wf.Nodes[idx]

		// Skip start/end nodes in the hints
		if node.Type == "start" {
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
			continue
		}
		if node.Type == "end" {
			hints = append(hints, fmt.Sprintf("%d. **结束流程**", step))
			step++
			continue
		}

		// Build step description
		label := node.Data.Label
		if label == "" {
			label = node.Type
		}

		var desc string
		switch node.Type {
		case "exclusive":
			// Gateway: show branches
			desc = fmt.Sprintf("%d. **条件分支** [%s]", step, label)
			for _, ei := range outEdges[nodeID] {
				edge := wf.Edges[ei]
				if edge.Data.Condition != nil {
					desc += fmt.Sprintf("\n   - 当 %s %s %v → ", edge.Data.Condition.Field, edge.Data.Condition.Operator, edge.Data.Condition.Value)
					if ti, ok := nodeMap[edge.Target]; ok {
						tl := wf.Nodes[ti].Data.Label
						if tl == "" {
							tl = wf.Nodes[ti].Type
						}
						desc += tl
					}
				} else if edge.Data.Default {
					desc += "\n   - 默认 → "
					if ti, ok := nodeMap[edge.Target]; ok {
						tl := wf.Nodes[ti].Data.Label
						if tl == "" {
							tl = wf.Nodes[ti].Type
						}
						desc += tl
					}
				}
				queue = append(queue, edge.Target)
			}
		case "parallel", "inclusive":
			if node.Data.GatewayDirection == "fork" {
				desc = fmt.Sprintf("%d. **并行处理** [%s]（以下步骤同时进行）", step, label)
			} else {
				desc = fmt.Sprintf("%d. **汇聚等待** [%s]（等待所有并行分支完成）", step, label)
			}
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "approve":
			participant := describeParticipants(node.Data.Participants)
			mode := ""
			if node.Data.ApproveMode == "parallel" {
				mode = "（并签）"
			}
			desc = fmt.Sprintf("%d. **审批** [%s] — %s%s", step, label, participant, mode)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "process":
			participant := describeParticipants(node.Data.Participants)
			desc = fmt.Sprintf("%d. **处理** [%s] — %s", step, label, participant)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "action":
			desc = fmt.Sprintf("%d. **自动动作** [%s]", step, label)
			if node.Data.ActionID != nil {
				desc += fmt.Sprintf("（action_id: %d）", *node.Data.ActionID)
			}
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "form":
			participant := describeParticipants(node.Data.Participants)
			desc = fmt.Sprintf("%d. **表单采集** [%s] — %s", step, label, participant)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "notify":
			desc = fmt.Sprintf("%d. **通知** [%s]", step, label)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		case "wait":
			desc = fmt.Sprintf("%d. **等待** [%s]", step, label)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		default:
			desc = fmt.Sprintf("%d. **%s** [%s]", step, node.Type, label)
			for _, ei := range outEdges[nodeID] {
				queue = append(queue, wf.Edges[ei].Target)
			}
		}

		hints = append(hints, desc)
		step++
	}

	if len(hints) == 0 {
		return ""
	}

	return strings.Join(hints, "\n")
}

// describeParticipants returns a human-readable description of participant assignments.
func describeParticipants(participants []struct {
	Type           string `json:"type"`
	Value          string `json:"value"`
	PositionCode   string `json:"position_code"`
	DepartmentCode string `json:"department_code"`
}) string {
	if len(participants) == 0 {
		return "待指定"
	}

	var parts []string
	for _, p := range participants {
		switch p.Type {
		case "user":
			parts = append(parts, fmt.Sprintf("用户(%s)", p.Value))
		case "position":
			parts = append(parts, fmt.Sprintf("岗位(%s)", p.Value))
		case "department":
			parts = append(parts, fmt.Sprintf("部门(%s)", p.Value))
		case "position_department":
			parts = append(parts, fmt.Sprintf("岗位(%s)@部门(%s)", p.PositionCode, p.DepartmentCode))
		case "requester_manager":
			parts = append(parts, "申请人主管")
		default:
			parts = append(parts, p.Type)
		}
	}
	return strings.Join(parts, ", ")
}

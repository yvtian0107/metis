package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"metis/internal/llm"
)

// decisionToolDef defines a decision domain tool with its LLM definition and handler.
type decisionToolDef struct {
	Def     llm.ToolDef
	Handler func(ctx *decisionToolContext, args json.RawMessage) (json.RawMessage, error)
}

// decisionToolContext holds the shared context for all decision tool executions.
type decisionToolContext struct {
	ctx                 context.Context
	data                DecisionDataProvider
	ticketID            uint
	serviceID           uint
	workflowJSON        string
	collaborationSpec   string
	knowledgeSearcher   KnowledgeSearcher
	resolver            *ParticipantResolver
	knowledgeBaseIDs    []uint
	actionExecutor      *ActionExecutor
	completedActivityID *uint
	configProvider      EngineConfigProvider
}

// allDecisionTools returns the complete set of decision domain tools.
func allDecisionTools() []decisionToolDef {
	return []decisionToolDef{
		toolTicketContext(),
		toolKnowledgeSearch(),
		toolResolveParticipant(),
		toolUserWorkload(),
		toolSimilarHistory(),
		toolSLAStatus(),
		toolListActions(),
		toolExecuteAction(),
	}
}

// buildDecisionToolDefs extracts llm.ToolDef list from all decision tools.
func buildDecisionToolDefs() []llm.ToolDef {
	tools := allDecisionTools()
	defs := make([]llm.ToolDef, len(tools))
	for i, t := range tools {
		defs[i] = t.Def
	}
	return defs
}

// DecisionToolDefs exposes the persisted builtin decision tool definitions for seed sync.
func DecisionToolDefs() []llm.ToolDef {
	return buildDecisionToolDefs()
}

// --- Tool: decision.ticket_context ---

func toolTicketContext() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.ticket_context",
			Description: "查询工单的完整上下文信息，包括表单数据、SLA 状态、活动历史和当前指派",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		Handler: func(ctx *decisionToolContext, _ json.RawMessage) (json.RawMessage, error) {
			ticket, err := ctx.data.GetTicketContext(ctx.ticketID)
			if err != nil {
				return toolError("工单不存在")
			}

			result := map[string]any{
				"code":        ticket.Code,
				"title":       ticket.Title,
				"description": ticket.Description,
				"status":      ticket.Status,
				"outcome":     ticket.Outcome,
				"source":      ticket.Source,
				"is_terminal": isTerminalTicketStatus(ticket.Status),
			}

			// Form data
			if ticket.FormData != "" {
				result["form_data"] = json.RawMessage(ticket.FormData)
			}

			// SLA status
			now := time.Now()
			if ticket.SLAResponseDeadline != nil || ticket.SLAResolutionDeadline != nil {
				sla := map[string]any{}
				if ticket.SLAResponseDeadline != nil {
					sla["response_remaining_seconds"] = int64(ticket.SLAResponseDeadline.Sub(now).Seconds())
				}
				if ticket.SLAResolutionDeadline != nil {
					sla["resolution_remaining_seconds"] = int64(ticket.SLAResolutionDeadline.Sub(now).Seconds())
				}
				result["sla_status"] = sla
			}

			// Activity history
			activities, _ := ctx.data.GetDecisionHistory(ctx.ticketID)

			var history []map[string]any
			var completedRequirements []map[string]any
			for _, a := range activities {
				assignments, _ := ctx.data.GetActivityAssignments(a.ID)
				entry := activityFactMap(&a, assignments)
				history = append(history, entry)
				if a.Status == ActivityCompleted && isHumanActivityType(a.ActivityType) {
					satisfied := isPositiveActivityOutcome(a.TransitionOutcome)
					completedRequirements = append(completedRequirements, map[string]any{
						"type":                       a.ActivityType,
						"name":                       a.Name,
						"node_id":                    a.NodeID,
						"outcome":                    a.TransitionOutcome,
						"operator_opinion":           a.DecisionReasoning,
						"participants":               assignmentFacts(assignments),
						"satisfied":                  satisfied,
						"requires_recovery_decision": !satisfied,
					})
				}
			}
			result["activity_history"] = history
			result["completed_requirements"] = completedRequirements

			currentActivities, _ := ctx.data.GetCurrentActivities(ctx.ticketID)
			var current []map[string]any
			currentNodeID := ""
			currentActivityName := ""
			for _, a := range currentActivities {
				if currentNodeID == "" {
					currentNodeID = a.NodeID
					currentActivityName = a.Name
				}
				current = append(current, map[string]any{
					"id":                a.ID,
					"name":              a.Name,
					"type":              a.ActivityType,
					"node_id":           a.NodeID,
					"status":            a.Status,
					"execution_mode":    a.ExecutionMode,
					"activity_group_id": a.ActivityGroupID,
					"ai_confidence":     a.AIConfidence,
				})
			}
			result["current_activities"] = current

			formData := map[string]any{}
			if ticket.FormData != "" {
				_ = json.Unmarshal([]byte(ticket.FormData), &formData)
			}
			var completedActivity *activityModel
			if ctx.completedActivityID != nil && *ctx.completedActivityID > 0 {
				if completed, err := ctx.data.GetActivityByID(ctx.ticketID, *ctx.completedActivityID); err == nil {
					assignments, _ := ctx.data.GetActivityAssignments(completed.ID)
					result["completed_activity"] = activityFactMap(completed, assignments)
					completedActivity = completed
				}
			}
			if workflowCtx := buildWorkflowContext(ctx.workflowJSON, ctx.collaborationSpec, formData, currentNodeID, currentActivityName, completedActivity); workflowCtx != nil {
				result["workflow_context"] = workflowCtx
				for _, key := range []string{"selected_branch", "active_branch_contract", "current_branch_node_id", "allowed_next_branch_nodes", "completion_contract", "branch_reasoning_basis"} {
					if value, ok := workflowCtx[key]; ok {
						result[key] = value
					}
				}
			}

			// Executed actions — shows which service actions have been successfully run
			execs, _ := ctx.data.GetExecutedActions(ctx.ticketID)
			totalActions, _ := ctx.data.CountActiveServiceActions(ctx.serviceID)
			actionProgress := map[string]any{
				"total":         totalActions,
				"executed":      len(execs),
				"all_completed": totalActions > 0 && int64(len(execs)) >= totalActions,
			}
			result["action_progress"] = actionProgress

			if len(execs) > 0 {
				var execNames []string
				for _, e := range execs {
					execNames = append(execNames, e.ActionName)
				}
				result["executed_actions"] = execNames

				// Check if all service actions have been executed
				if actionProgress["all_completed"] == true {
					result["all_actions_completed"] = true
				}
			}

			// Current assignment
			assignment, err := ctx.data.GetCurrentAssignment(ctx.ticketID)
			if err == nil && assignment != nil {
				result["current_assignment"] = map[string]any{
					"assignee_id":   assignment.AssigneeID,
					"assignee_name": assignment.AssigneeName,
				}
			}

			// Parallel groups — active countersign groups with progress
			groups, _ := ctx.data.GetParallelGroups(ctx.ticketID)

			var activeGroups []map[string]any
			for _, g := range groups {
				if g.Total-g.Completed > 0 {
					// Collect pending activity names
					pendingNames, _ := ctx.data.GetPendingActivityNames(ctx.ticketID, g.ActivityGroupID)

					activeGroups = append(activeGroups, map[string]any{
						"group_id":           g.ActivityGroupID,
						"total":              g.Total,
						"completed":          g.Completed,
						"pending_activities": pendingNames,
					})
				}
			}
			if len(activeGroups) > 0 {
				result["parallel_groups"] = activeGroups
			}

			return json.Marshal(result)
		},
	}
}

func activityFactMap(a *activityModel, assignments []ActivityAssignmentInfo) map[string]any {
	entry := map[string]any{
		"id":      a.ID,
		"type":    a.ActivityType,
		"name":    a.Name,
		"status":  a.Status,
		"outcome": a.TransitionOutcome,
	}
	if a.NodeID != "" {
		entry["node_id"] = a.NodeID
	}
	if a.FinishedAt != nil {
		entry["completed_at"] = a.FinishedAt.Format(time.RFC3339)
	}
	if a.Status == ActivityCompleted && isHumanActivityType(a.ActivityType) {
		satisfied := isPositiveActivityOutcome(a.TransitionOutcome)
		entry["satisfied"] = satisfied
		if !satisfied {
			entry["requires_recovery_decision"] = true
			entry["recovery_reason"] = "人工节点已驳回；下一轮必须基于协作规范和 workflow_json 解释驳回后的恢复路径，不得无新证据重复创建同一处理任务。"
		}
	}
	if a.AIReasoning != "" {
		entry["ai_reasoning"] = a.AIReasoning
	}
	if a.DecisionReasoning != "" {
		entry["decision_reasoning"] = a.DecisionReasoning
		entry["operator_opinion"] = a.DecisionReasoning
	}
	if a.AIDecision != "" {
		var decision any
		if err := json.Unmarshal([]byte(a.AIDecision), &decision); err == nil {
			entry["source_decision"] = decision
		}
	}
	if a.FormData != "" {
		entry["form_data"] = json.RawMessage(a.FormData)
	}
	facts := assignmentFacts(assignments)
	if len(facts) > 0 {
		entry["participants"] = facts
	}
	return entry
}

func assignmentFacts(assignments []ActivityAssignmentInfo) []map[string]any {
	facts := make([]map[string]any, 0, len(assignments))
	for _, a := range assignments {
		fact := map[string]any{
			"participant_type": a.ParticipantType,
			"status":           a.Status,
		}
		if a.UserID != nil {
			fact["user_id"] = *a.UserID
		}
		if a.AssigneeID != nil {
			fact["assignee_id"] = *a.AssigneeID
		}
		if a.PositionID != nil {
			fact["position_id"] = *a.PositionID
		}
		if a.DepartmentID != nil {
			fact["department_id"] = *a.DepartmentID
		}
		if a.FinishedAt != nil {
			fact["finished_at"] = a.FinishedAt.Format(time.RFC3339)
		}
		facts = append(facts, fact)
	}
	return facts
}

func isHumanActivityType(activityType string) bool {
	switch activityType {
	case NodeApprove, NodeProcess, NodeForm:
		return true
	default:
		return false
	}
}

func isPositiveActivityOutcome(outcome string) bool {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "", "approve", "approved", "complete", "completed", "process", "processed", "submit", "submitted", "success", "passed":
		return true
	default:
		return false
	}
}

func isTerminalTicketStatus(status string) bool {
	return IsTerminalTicketStatus(status)
}

// --- Tool: decision.knowledge_search ---

func toolKnowledgeSearch() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.knowledge_search",
			Description: "搜索服务关联的知识库，返回与查询相关的知识片段",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索查询文本"},
					"limit": map[string]any{"type": "integer", "description": "返回结果数量上限（默认 3）"},
				},
				"required": []string{"query"},
			},
		},
		Handler: func(ctx *decisionToolContext, args json.RawMessage) (json.RawMessage, error) {
			var params struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return toolError(fmt.Sprintf("参数格式错误: %v", err))
			}
			if params.Limit <= 0 {
				params.Limit = 3
			}

			if ctx.knowledgeSearcher == nil {
				return json.Marshal(map[string]any{
					"results": []any{},
					"count":   0,
					"message": "知识搜索不可用",
				})
			}

			if len(ctx.knowledgeBaseIDs) == 0 {
				return json.Marshal(map[string]any{
					"results": []any{},
					"count":   0,
				})
			}

			results, err := ctx.knowledgeSearcher.Search(ctx.knowledgeBaseIDs, params.Query, params.Limit)
			if err != nil {
				return toolError(fmt.Sprintf("知识搜索失败: %v", err))
			}

			items := make([]map[string]any, len(results))
			for i, r := range results {
				items[i] = map[string]any{
					"title":   r.Title,
					"content": r.Content,
					"score":   r.Score,
				}
			}
			return json.Marshal(map[string]any{
				"results": items,
				"count":   len(items),
			})
		},
	}
}

// --- Tool: decision.resolve_participant ---

func toolResolveParticipant() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.resolve_participant",
			Description: "按参与人类型解析出具体用户。支持 requester/user/position/department/position_department/requester_manager",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type":            map[string]any{"type": "string", "description": "参与人类型: requester|user|position|department|position_department|requester_manager"},
					"value":           map[string]any{"type": "string", "description": "类型相关值（user类型为user_id或username, position类型为position_code等）"},
					"position_code":   map[string]any{"type": "string", "description": "岗位代码（position_department类型时必填）"},
					"department_code": map[string]any{"type": "string", "description": "部门代码（position_department类型时必填）"},
				},
				"required": []string{"type"},
			},
		},
		Handler: func(ctx *decisionToolContext, args json.RawMessage) (json.RawMessage, error) {
			if ctx.resolver == nil {
				return toolError("参与人解析器不可用")
			}

			userIDs, err := ctx.data.ResolveForTool(ctx.resolver, ctx.ticketID, args)
			if err != nil {
				return toolError(fmt.Sprintf("参与人解析失败: %v", err))
			}

			// Enrich with user details
			var candidates []map[string]any
			for _, uid := range userIDs {
				user, err := ctx.data.GetUserBasicInfo(uid)
				if err != nil {
					continue
				}
				if !user.IsActive {
					continue
				}
				candidates = append(candidates, map[string]any{
					"user_id": user.ID,
					"name":    user.Username,
				})
			}

			status := "resolved"
			if len(candidates) == 0 {
				status = "no_candidates"
			}
			return json.Marshal(map[string]any{
				"ok":         len(candidates) > 0,
				"status":     status,
				"candidates": candidates,
				"count":      len(candidates),
			})
		},
	}
}

// --- Tool: decision.user_workload ---

func toolUserWorkload() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.user_workload",
			Description: "查询指定用户当前的工单负载信息（待处理活动数），帮助做出负载均衡的指派决策",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user_id": map[string]any{"type": "integer", "description": "用户 ID"},
				},
				"required": []string{"user_id"},
			},
		},
		Handler: func(ctx *decisionToolContext, args json.RawMessage) (json.RawMessage, error) {
			var params struct {
				UserID uint `json:"user_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return toolError(fmt.Sprintf("参数格式错误: %v", err))
			}

			user, err := ctx.data.GetUserBasicInfo(params.UserID)
			if err != nil {
				return toolError("用户不存在")
			}

			// Count pending activities assigned to this user
			pendingCount, _ := ctx.data.CountUserPendingActivities(params.UserID)

			return json.Marshal(map[string]any{
				"user_id":            user.ID,
				"name":               user.Username,
				"is_active":          user.IsActive,
				"pending_activities": pendingCount,
			})
		},
	}
}

// --- Tool: decision.similar_history ---

func toolSimilarHistory() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.similar_history",
			Description: "查询同一服务下已完成工单的处理模式，提供历史参考（平均耗时、常见处理人等）",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "返回工单数量上限（默认 5）"},
				},
			},
		},
		Handler: func(ctx *decisionToolContext, args json.RawMessage) (json.RawMessage, error) {
			var params struct {
				Limit int `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return toolError(fmt.Sprintf("参数格式错误: %v", err))
			}
			if params.Limit <= 0 {
				defaultLimit := 5
				if ctx.configProvider != nil {
					defaultLimit = ctx.configProvider.SimilarHistoryLimit()
					if defaultLimit <= 0 {
						defaultLimit = 5
					}
				}
				params.Limit = defaultLimit
			}

			rows, _ := ctx.data.GetSimilarHistory(ctx.serviceID, ctx.ticketID, params.Limit)

			var tickets []map[string]any
			var totalHours float64
			for _, r := range rows {
				entry := map[string]any{
					"code":   r.Code,
					"title":  r.Title,
					"status": r.Status,
				}

				if r.FinishedAt != nil {
					hours := r.FinishedAt.Sub(r.CreatedAt).Hours()
					entry["resolution_duration_hours"] = math.Round(hours*10) / 10
					totalHours += hours
				}

				// Count activities
				actCount, _ := ctx.data.CountTicketActivities(r.ID)
				entry["activity_count"] = actCount

				tickets = append(tickets, entry)
			}

			// Aggregate stats
			totalCount, _ := ctx.data.CountCompletedTickets(ctx.serviceID)

			avgHours := 0.0
			if len(rows) > 0 {
				avgHours = math.Round(totalHours/float64(len(rows))*10) / 10
			}

			return json.Marshal(map[string]any{
				"tickets": tickets,
				"stats": map[string]any{
					"avg_resolution_hours": avgHours,
					"total_count":          totalCount,
				},
			})
		},
	}
}

// --- Tool: decision.sla_status ---

func toolSLAStatus() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.sla_status",
			Description: "查询工单的 SLA 状态和紧急程度评估",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		Handler: func(ctx *decisionToolContext, _ json.RawMessage) (json.RawMessage, error) {
			ticket, err := ctx.data.GetSLAData(ctx.ticketID)
			if err != nil {
				return toolError("工单不存在")
			}

			if ticket.SLAResponseDeadline == nil && ticket.SLAResolutionDeadline == nil {
				return json.Marshal(map[string]any{
					"has_sla": false,
					"urgency": "normal",
				})
			}

			// Resolve thresholds from config or use defaults
			criticalThreshold := int64(1800)
			warningThreshold := int64(3600)
			if ctx.configProvider != nil {
				criticalThreshold = int64(ctx.configProvider.SLACriticalThresholdSeconds())
				warningThreshold = int64(ctx.configProvider.SLAWarningThresholdSeconds())
			}

			now := time.Now()
			result := map[string]any{
				"has_sla":    true,
				"sla_status": ticket.SLAStatus,
			}

			urgency := "normal"
			if ticket.SLAResponseDeadline != nil {
				remaining := int64(ticket.SLAResponseDeadline.Sub(now).Seconds())
				result["response_remaining_seconds"] = remaining
				if remaining < 0 {
					urgency = "breached"
				} else if remaining < criticalThreshold {
					urgency = "critical"
				} else if remaining < warningThreshold {
					urgency = "warning"
				}
			}
			if ticket.SLAResolutionDeadline != nil {
				remaining := int64(ticket.SLAResolutionDeadline.Sub(now).Seconds())
				result["resolution_remaining_seconds"] = remaining
				if remaining < 0 && urgency != "breached" {
					urgency = "breached"
				}
			}
			result["urgency"] = urgency

			return json.Marshal(result)
		},
	}
}

// --- Tool: decision.list_actions ---

func toolListActions() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.list_actions",
			Description: "列出当前服务可用的自动化动作（ServiceAction）",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		Handler: func(ctx *decisionToolContext, _ json.RawMessage) (json.RawMessage, error) {
			actions, _ := ctx.data.ListActiveServiceActions(ctx.serviceID)

			items := make([]map[string]any, len(actions))
			for i, a := range actions {
				items[i] = map[string]any{
					"id":          a.ID,
					"code":        a.Code,
					"name":        a.Name,
					"description": a.Description,
				}
			}
			return json.Marshal(map[string]any{
				"actions": items,
				"count":   len(items),
			})
		},
	}
}

// --- Tool: decision.execute_action ---

func toolExecuteAction() decisionToolDef {
	return decisionToolDef{
		Def: llm.ToolDef{
			Name:        "decision.execute_action",
			Description: "同步执行服务配置的自动化动作（webhook），返回执行结果。用于在决策推理过程中触发自动化操作并观察结果。",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action_id": map[string]any{"type": "integer", "description": "要执行的动作 ID（从 decision.list_actions 获取）"},
				},
				"required": []string{"action_id"},
			},
		},
		Handler: func(ctx *decisionToolContext, args json.RawMessage) (json.RawMessage, error) {
			var params struct {
				ActionID uint `json:"action_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return toolError(fmt.Sprintf("参数格式错误: %v", err))
			}

			if ctx.actionExecutor == nil {
				return toolError("动作执行器不可用")
			}

			// Verify action exists and is active
			action, err := ctx.data.GetServiceAction(params.ActionID, ctx.serviceID)
			if err != nil {
				return toolError("动作不存在")
			}
			if !action.IsActive {
				return toolError("动作已停用")
			}

			// Idempotency: check if this action was already successfully executed for this ticket
			execs, _ := ctx.data.GetExecutedActions(ctx.ticketID)
			for _, e := range execs {
				if e.ActionCode == action.Code && e.Status == "success" {
					return json.Marshal(map[string]any{
						"success":     true,
						"action_name": action.Name,
						"action_code": action.Code,
						"cached":      true,
						"message":     fmt.Sprintf("动作 [%s] 已执行成功（缓存结果）", action.Name),
					})
				}
			}

			// Use a child context derived from the decision context with a timeout.
			// The ActionExecutor internally loads the action config and applies per-request
			// timeouts, but we bound the overall execution (including retries) with this context.
			// Default overall timeout: action timeout * (retries+1) with a floor of 30s.
			execCtx, cancel := context.WithTimeout(ctx.ctx, 120*time.Second)
			defer cancel()

			err = ctx.actionExecutor.Execute(
				execCtx,
				ctx.ticketID, 0, params.ActionID,
			)

			if err != nil {
				return json.Marshal(map[string]any{
					"success":     false,
					"action_name": action.Name,
					"action_code": action.Code,
					"error":       err.Error(),
				})
			}

			return json.Marshal(map[string]any{
				"success":     true,
				"action_name": action.Name,
				"action_code": action.Code,
				"message":     fmt.Sprintf("动作 [%s] 执行成功", action.Name),
			})
		},
	}
}

// --- Helpers ---

func toolError(msg string) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"error":   true,
		"message": msg,
	})
}

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"metis/internal/app"
	"metis/internal/llm"
)

// decisionExecutor implements app.AIDecisionExecutor using the AI App's
// agent configuration and LLM client infrastructure.
type decisionExecutor struct {
	gateway *AgentGateway
}

// NewDecisionExecutor creates a DecisionExecutor backed by the AgentGateway.
func NewDecisionExecutor(gw *AgentGateway) app.AIDecisionExecutor {
	return &decisionExecutor{gateway: gw}
}

const defaultDecisionMaxTurns = 8

func (e *decisionExecutor) Execute(ctx context.Context, agentID uint, req app.AIDecisionRequest) (*app.AIDecisionResponse, error) {
	// Resolve agent config via gateway
	agentCfg, err := e.gateway.GetAgentConfig(agentID)
	if err != nil {
		return nil, fmt.Errorf("get decision agent config: %w", err)
	}

	// Create LLM client
	client, err := llm.NewClient(agentCfg.Protocol, agentCfg.BaseURL, agentCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("create llm client: %w", err)
	}

	// Compose system prompt: agent's own system prompt + domain prompt
	systemPrompt := ""
	if agentCfg.SystemPrompt != "" {
		systemPrompt = "## 角色定义\n\n" + agentCfg.SystemPrompt + "\n\n---\n\n"
	}
	systemPrompt += req.SystemPrompt

	// Convert tool defs
	toolDefs := make([]llm.ToolDef, len(req.Tools))
	for i, t := range req.Tools {
		toolDefs[i] = llm.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		}
	}

	// Prepare messages
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: req.UserMessage},
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

	maxTurns := req.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultDecisionMaxTurns
	}

	// ReAct loop
	var totalInputTokens, totalOutputTokens int
	for turn := 0; turn < maxTurns; turn++ {
		slog.Info("decision executor: starting LLM turn", "agentID", agentID, "turn", turn+1, "maxTurns", maxTurns, "model", agentCfg.Model)
		chatReq := llm.ChatRequest{
			Model:       agentCfg.Model,
			Messages:    messages,
			Tools:       toolDefs,
			MaxTokens:   maxTokens,
			Temperature: tempPtr,
		}

		resp, err := client.Chat(ctx, chatReq)
		if err != nil {
			return nil, fmt.Errorf("llm chat (turn %d): %w", turn, err)
		}
		slog.Info("decision executor: completed LLM turn", "agentID", agentID, "turn", turn+1, "toolCalls", len(resp.ToolCalls))

		totalInputTokens += resp.Usage.InputTokens
		totalOutputTokens += resp.Usage.OutputTokens

		// No tool calls → final answer
		if len(resp.ToolCalls) == 0 {
			return &app.AIDecisionResponse{
				Content:      resp.Content,
				InputTokens:  totalInputTokens,
				OutputTokens: totalOutputTokens,
				Turns:        turn + 1,
			}, nil
		}

		// Append assistant message with tool calls
		messages = append(messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute tool calls via the provided handler
		for _, tc := range resp.ToolCalls {
			start := time.Now()
			result, err := req.ToolHandler(tc.Name, json.RawMessage(tc.Arguments))
			elapsed := time.Since(start)

			// Build log attrs: always include tool name and duration, plus caller metadata
			attrs := []any{
				"tool", tc.Name,
				"durationMs", elapsed.Milliseconds(),
			}
			for k, v := range req.Metadata {
				attrs = append(attrs, k, v)
			}

			var content string
			if err != nil {
				content = fmt.Sprintf(`{"error": "%s"}`, err.Error())
				slog.Warn("decision-tool: error", append(attrs, "error", err.Error())...)
			} else {
				content = string(result)
				slog.Info("decision-tool: call", append(attrs, "ok", true)...)
			}
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    content,
				ToolCallID: tc.ID,
			})
		}
	}

	return nil, fmt.Errorf("决策循环超过最大轮数 (%d)，未产生最终决策", maxTurns)
}

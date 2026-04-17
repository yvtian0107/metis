package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"metis/internal/llm"
)

// ReactExecutor implements the ReAct (Reason + Act) loop.
type ReactExecutor struct {
	llmClient    llm.Client
	toolExecutor ToolExecutor
}

func NewReactExecutor(client llm.Client, toolExec ToolExecutor) *ReactExecutor {
	return &ReactExecutor{
		llmClient:    client,
		toolExecutor: toolExec,
	}
}

func (e *ReactExecutor) Execute(ctx context.Context, req ExecuteRequest) (<-chan Event, error) {
	ch := make(chan Event, 64)

	go func() {
		defer close(ch)

		var seq atomic.Int32
		emit := func(evt Event) {
			evt.Sequence = int(seq.Add(1))
			select {
			case ch <- evt:
				return
			default:
			}
			select {
			case ch <- evt:
			case <-ctx.Done():
			}
		}

		messages := buildLLMMessages(req)
		maxTurns := req.MaxTurns
		if maxTurns <= 0 {
			maxTurns = 10
		}

		tools := buildLLMTools(req.Tools)
		var totalInput, totalOutput int

		for turn := 1; turn <= maxTurns; turn++ {
			select {
			case <-ctx.Done():
				emit(Event{Type: EventTypeCancelled, Message: "cancelled by user"})
				return
			default:
			}

			emit(Event{Type: EventTypeLLMStart, Turn: turn, Model: req.AgentConfig.ModelName})

			chatReq := llm.ChatRequest{
				Model:       req.AgentConfig.ModelName,
				Messages:    messages,
				Tools:       tools,
				MaxTokens:   req.AgentConfig.MaxTokens,
				Temperature: req.AgentConfig.Temperature,
			}

			streamCh, err := e.llmClient.ChatStream(ctx, chatReq)
			if err != nil {
				emit(Event{Type: EventTypeError, Message: fmt.Sprintf("LLM call failed: %v", err)})
				return
			}

			var assistantContent string
			var toolCalls []llm.ToolCall
			var usage llm.Usage

			for evt := range streamCh {
				switch evt.Type {
				case "content_delta":
					assistantContent += evt.Content
					emit(Event{Type: EventTypeContentDelta, Text: evt.Content})
				case "tool_call":
					if evt.ToolCall != nil {
						toolCalls = append(toolCalls, *evt.ToolCall)
					}
				case "done":
					if evt.Usage != nil {
						usage = *evt.Usage
					}
				case "error":
					emit(Event{Type: EventTypeError, Message: evt.Error})
					return
				}
			}

			totalInput += usage.InputTokens
			totalOutput += usage.OutputTokens

			// If no tool calls, we're done
			if len(toolCalls) == 0 {
				// Check for memory extraction in the content
				e.extractMemoryUpdates(assistantContent, emit)

				emit(Event{
					Type:         EventTypeDone,
					TotalTurns:   turn,
					InputTokens:  totalInput,
					OutputTokens: totalOutput,
				})
				return
			}

			// Add assistant message with tool calls
			messages = append(messages, llm.Message{
				Role:      llm.RoleAssistant,
				Content:   assistantContent,
				ToolCalls: toolCalls,
			})

			// Execute each tool call
			for _, tc := range toolCalls {
				emit(Event{
					Type:       EventTypeToolCall,
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					ToolArgs:   json.RawMessage(tc.Arguments),
				})

				start := time.Now()
				result, execErr := e.toolExecutor.ExecuteTool(ctx, ToolCall{
					ID:   tc.ID,
					Name: tc.Name,
					Args: json.RawMessage(tc.Arguments),
				})
				durationMs := int(time.Since(start).Milliseconds())

				output := result.Output
				if execErr != nil {
					output = fmt.Sprintf("Error: %v", execErr)
				}

				emit(Event{
					Type:       EventTypeToolResult,
					ToolCallID: tc.ID,
					ToolOutput: output,
					DurationMs: durationMs,
				})

				// Add tool result to messages
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Content:    output,
					ToolCallID: tc.ID,
				})
			}
			// Continue loop — next turn will send messages with tool results back to LLM
		}

		emit(Event{
			Type:    EventTypeError,
			Message: fmt.Sprintf("max turns (%d) exceeded", maxTurns),
		})
	}()

	return ch, nil
}

// buildLLMMessages converts ExecuteRequest messages to llm.Message format,
// prepending the system prompt.
func buildLLMMessages(req ExecuteRequest) []llm.Message {
	msgs := make([]llm.Message, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		msgs = append(msgs, llm.Message{Role: llm.RoleSystem, Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, llm.Message{
			Role:    m.Role,
			Content: m.Content,
			Images:  m.Images,
		})
	}
	return msgs
}

// buildLLMTools converts ToolDefinitions to llm.ToolDef format.
func buildLLMTools(tools []ToolDefinition) []llm.ToolDef {
	if len(tools) == 0 {
		return nil
	}
	defs := make([]llm.ToolDef, len(tools))
	for i, t := range tools {
		var params any
		if len(t.Parameters) > 0 {
			_ = json.Unmarshal(t.Parameters, &params)
		}
		defs[i] = llm.ToolDef{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		}
	}
	return defs
}

// extractMemoryUpdates looks for memory extraction markers in content.
// In practice, the system prompt instructs the LLM to output <memory> tags.
func (e *ReactExecutor) extractMemoryUpdates(content string, emit func(Event)) {
	// Simple pattern: LLM is instructed to emit JSON memory blocks
	// This is a placeholder — full implementation will use structured output
	// or a dedicated memory extraction prompt.
	_ = content
	_ = emit
}

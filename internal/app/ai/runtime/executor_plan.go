package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"metis/internal/app"
	"metis/internal/llm"
)

// PlanAndExecuteExecutor implements the Plan-and-Execute strategy:
// Phase 1: Call LLM to generate a plan
// Phase 2: Execute each step using a ReAct sub-loop
type PlanAndExecuteExecutor struct {
	llmClient    llm.Client
	toolExecutor ToolExecutor
}

func NewPlanAndExecuteExecutor(client llm.Client, toolExec ToolExecutor) *PlanAndExecuteExecutor {
	return &PlanAndExecuteExecutor{
		llmClient:    client,
		toolExecutor: toolExec,
	}
}

const planningPromptSuffix = `

Before executing, first create a plan. Output a JSON array of steps:
[{"index": 1, "description": "Step description"}, ...]

Output ONLY the JSON array, no other text.`

const defaultStepTurnBudget = 5

func (e *PlanAndExecuteExecutor) Execute(ctx context.Context, req ExecuteRequest) (<-chan Event, error) {
	ch := make(chan Event, 64)

	go func() {
		defer close(ch)

		var seq atomic.Int32
		emit := func(evt Event) {
			evt.Sequence = int(seq.Add(1))
			select {
			case ch <- evt:
			case <-ctx.Done():
			}
		}

		// Phase 1: Generate plan
		select {
		case <-ctx.Done():
			emit(stoppedEvent(ctx.Err(), "planning"))
			return
		default:
		}

		planMessages := buildLLMMessages(req)
		if len(planMessages) > 0 {
			last := &planMessages[len(planMessages)-1]
			last.Content += planningPromptSuffix
		}

		slog.Info("plan executor: starting planning LLM call", "model", req.AgentConfig.ModelName)
		planResp, err := chatWithTimeout(ctx, e.llmClient, llm.ChatRequest{
			Model:       req.AgentConfig.ModelName,
			Messages:    planMessages,
			MaxTokens:   req.AgentConfig.MaxTokens,
			Temperature: req.AgentConfig.Temperature,
		})
		if err != nil {
			if ctx.Err() != nil {
				emit(stoppedEvent(ctx.Err(), "planning"))
			} else {
				emit(Event{Type: EventTypeError, Message: llmCallErrorMessage("planning", err)})
			}
			return
		}
		slog.Info("plan executor: completed planning LLM call", "model", req.AgentConfig.ModelName)

		steps := parsePlanSteps(planResp.Content)
		if len(steps) == 0 {
			emit(Event{Type: EventTypeError, Message: "failed to parse plan from LLM response"})
			return
		}

		emit(Event{Type: EventTypePlan, Steps: steps})

		var totalInput, totalOutput int
		totalInput += planResp.Usage.InputTokens
		totalOutput += planResp.Usage.OutputTokens

		// Phase 2: Execute each step
		messages := buildLLMMessages(req)
		tools := buildLLMTools(req.Tools)
		originalUserMessage := latestLLMUserMessage(messages)
		priorStepSummaries := make([]llm.Message, 0, len(steps))

		for _, step := range steps {
			select {
			case <-ctx.Done():
				emit(stoppedEvent(ctx.Err(), "LLM stream"))
				return
			default:
			}

			emit(Event{Type: EventTypeStepStart, StepIndex: step.Index, Description: step.Description})

			// Add step instruction to messages
			stepMsg := llm.Message{
				Role:    llm.RoleUser,
				Content: fmt.Sprintf("Execute step %d: %s", step.Index, step.Description),
			}
			stepMessages := append(append([]llm.Message{}, messages...), priorStepSummaries...)
			stepMessages = append(stepMessages, stepMsg)

			// Run a mini ReAct loop for this step (max 5 turns per step)
			var stepContent strings.Builder
			var stepToolOutputs []string
			stepCompleted := false
			for turn := 0; turn < defaultStepTurnBudget; turn++ {
				slog.Info("plan executor: starting step LLM turn", "step", step.Index, "turn", turn+1, "model", req.AgentConfig.ModelName)
				streamCh, turnCtx, turnCancel, err := openChatStreamWithTimeout(ctx, e.llmClient, llm.ChatRequest{
					Model:       req.AgentConfig.ModelName,
					Messages:    stepMessages,
					Tools:       tools,
					MaxTokens:   req.AgentConfig.MaxTokens,
					Temperature: req.AgentConfig.Temperature,
				})
				if err != nil {
					if ctx.Err() != nil {
						emit(stoppedEvent(ctx.Err(), "LLM stream"))
					} else {
						emit(Event{Type: EventTypeError, Message: fmt.Sprintf("step %d %s", step.Index, llmCallErrorMessage("LLM stream", err))})
					}
					return
				}
				slog.Info("plan executor: LLM stream established", "step", step.Index, "turn", turn+1)

				var content string
				var toolCalls []llm.ToolCall

				streamDone := false
				for !streamDone {
					select {
					case evt, ok := <-streamCh:
						if !ok {
							if ctx.Err() != nil {
								turnCancel()
								emit(stoppedEvent(ctx.Err(), "LLM stream"))
								return
							}
							if turnCtx.Err() != nil {
								turnCancel()
								emit(Event{Type: EventTypeError, Message: fmt.Sprintf("step %d %s", step.Index, llmCallErrorMessage("LLM stream", turnCtx.Err()))})
								return
							}
							streamDone = true
							break
						}
						switch evt.Type {
						case "content_delta":
							content += evt.Content
							stepContent.WriteString(evt.Content)
							emit(Event{Type: EventTypeContentDelta, Text: evt.Content})
						case "tool_call":
							if evt.ToolCall != nil {
								toolCalls = append(toolCalls, *evt.ToolCall)
							}
						case "done":
							if evt.Usage != nil {
								totalInput += evt.Usage.InputTokens
								totalOutput += evt.Usage.OutputTokens
							}
						case "error":
							turnCancel()
							slog.Warn("plan executor: LLM stream returned error", "step", step.Index, "turn", turn+1, "error", evt.Error)
							emit(Event{Type: EventTypeError, Message: evt.Error})
							return
						}
					case <-ctx.Done():
						turnCancel()
						emit(stoppedEvent(ctx.Err(), "LLM stream"))
						return
					case <-turnCtx.Done():
						turnCancel()
						slog.Warn("plan executor: LLM stream timed out", "step", step.Index, "turn", turn+1, "error", turnCtx.Err())
						emit(Event{Type: EventTypeError, Message: fmt.Sprintf("step %d %s", step.Index, llmCallErrorMessage("LLM stream", turnCtx.Err()))})
						return
					}
				}
				turnCancel()
				slog.Info("plan executor: completed step LLM turn", "step", step.Index, "turn", turn+1, "toolCalls", len(toolCalls))

				if len(toolCalls) == 0 {
					stepCompleted = true
					break // Step complete
				}

				stepMessages = append(stepMessages, llm.Message{
					Role: llm.RoleAssistant, Content: content, ToolCalls: toolCalls,
				})

				for _, tc := range toolCalls {
					emit(Event{Type: EventTypeToolCall, ToolCallID: tc.ID, ToolName: tc.Name, ToolArgs: json.RawMessage(tc.Arguments)})

					start := time.Now()
					toolCtx := context.WithValue(ctx, app.UserMessageKey, originalUserMessage)
					result, execErr := e.toolExecutor.ExecuteTool(toolCtx, ToolCall{ID: tc.ID, Name: tc.Name, Args: json.RawMessage(tc.Arguments)})
					durationMs := result.DurationMs
					if durationMs == 0 {
						durationMs = int(time.Since(start).Milliseconds())
					}
					output := result.Output
					isError := result.IsError
					if execErr != nil {
						output = fmt.Sprintf("Error: %v", execErr)
						isError = true
					}
					if output == "" {
						output = "{}"
					}
					emit(Event{Type: EventTypeToolResult, ToolCallID: tc.ID, ToolOutput: output, DurationMs: durationMs, ToolIsError: isError})
					stepToolOutputs = append(stepToolOutputs, fmt.Sprintf("%s => %s", tc.Name, output))

					stepMessages = append(stepMessages, llm.Message{Role: llm.RoleTool, Content: output, ToolCallID: tc.ID})
				}
			}
			if !stepCompleted {
				emit(Event{Type: EventTypeError, Message: fmt.Sprintf("step %d exceeded turn budget (%d)", step.Index, defaultStepTurnBudget)})
				return
			}
			emit(Event{Type: EventTypeStepDone, StepIndex: step.Index})
			priorStepSummaries = append(priorStepSummaries, llm.Message{
				Role:    llm.RoleAssistant,
				Content: buildStepSummary(step, stepContent.String(), stepToolOutputs),
			})
		}

		emit(Event{
			Type:         EventTypeDone,
			TotalTurns:   len(steps),
			InputTokens:  totalInput,
			OutputTokens: totalOutput,
		})
	}()

	return ch, nil
}

func buildStepSummary(step PlanStep, content string, toolOutputs []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Completed step %d: %s", step.Index, step.Description))
	content = strings.TrimSpace(content)
	if content != "" {
		sb.WriteString("\nOutput:\n")
		sb.WriteString(content)
	}
	if len(toolOutputs) > 0 {
		sb.WriteString("\nTool results:\n")
		for _, output := range toolOutputs {
			sb.WriteString("- ")
			sb.WriteString(output)
			sb.WriteByte('\n')
		}
	}
	return strings.TrimSpace(sb.String())
}

func parsePlanSteps(content string) []PlanStep {
	content = strings.TrimSpace(content)
	// Try to find JSON array in the content
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	jsonStr := content[start : end+1]

	var steps []PlanStep
	if err := json.Unmarshal([]byte(jsonStr), &steps); err != nil {
		return nil
	}
	return steps
}

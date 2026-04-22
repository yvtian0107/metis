package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"metis/internal/llm"
)

func collectEvents(ch <-chan Event) []Event {
	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}
	return events
}

func TestReactExecutor_DirectContent(t *testing.T) {
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "content_delta", Content: "Hello "},
		{Type: "content_delta", Content: "world"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 5, OutputTokens: 2}},
	}, nil)

	exec := NewReactExecutor(mockLLM, newMockToolExecutor())
	req := ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Hi"}},
	}

	ch, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}
	if events[0].Type != EventTypeLLMStart {
		t.Errorf("first event: expected %q, got %q", EventTypeLLMStart, events[0].Type)
	}
	if events[len(events)-1].Type != EventTypeDone {
		t.Errorf("last event: expected %q, got %q", EventTypeDone, events[len(events)-1].Type)
	}
}

func TestBuildLLMMessages_MapsToolTranscriptFields(t *testing.T) {
	req := ExecuteRequest{
		SystemPrompt: "system",
		Messages: []ExecuteMessage{
			{Role: MessageRoleUser, Content: "查 VPN"},
			{
				Role: MessageRoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "itsm.service_match", Arguments: `{"query":"VPN"}`},
				},
			},
			{Role: llm.RoleTool, Content: `{"selected_service_id":5}`, ToolCallID: "call_1"},
		},
	}

	messages := buildLLMMessages(req)

	if len(messages) != 4 {
		t.Fatalf("expected system + 3 messages, got %+v", messages)
	}
	if len(messages[2].ToolCalls) != 1 || messages[2].ToolCalls[0].Name != "itsm.service_match" {
		t.Fatalf("expected tool calls to be preserved, got %+v", messages[2])
	}
	if messages[3].Role != llm.RoleTool || messages[3].ToolCallID != "call_1" {
		t.Fatalf("expected tool result to preserve call id, got %+v", messages[3])
	}
}

func TestReactExecutor_ToolCallRoundTrip(t *testing.T) {
	mockExec := newMockToolExecutor()
	mockExec.SetResult("search", ToolResult{Output: "result from search"})

	// First turn: tool call; second turn: final content
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{"q":"x"}`}},
		{Type: "done", Usage: &llm.Usage{InputTokens: 3, OutputTokens: 5}},
		{Type: "content_delta", Content: "Done"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 8, OutputTokens: 1}},
	}, nil)

	exec := NewReactExecutor(mockLLM, mockExec)
	req := ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Search x"}},
		Tools:    []ToolDefinition{{Name: "search", Parameters: []byte(`{"type":"object"}`)}},
		MaxTurns: 10,
	}

	ch, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)

	var hasToolCall, hasToolResult, hasDone bool
	for _, evt := range events {
		switch evt.Type {
		case EventTypeToolCall:
			hasToolCall = true
			if evt.ToolName != "search" {
				t.Errorf("toolName: expected search, got %s", evt.ToolName)
			}
		case EventTypeToolResult:
			hasToolResult = true
			if !strings.Contains(evt.ToolOutput, "result from search") {
				t.Errorf("toolOutput: expected search result, got %s", evt.ToolOutput)
			}
		case EventTypeDone:
			hasDone = true
		}
	}
	if !hasToolCall {
		t.Error("expected tool_call event")
	}
	if !hasToolResult {
		t.Error("expected tool_result event")
	}
	if !hasDone {
		t.Error("expected done event")
	}
}

func TestReactExecutor_UnknownToolReturnsToolResult(t *testing.T) {
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "missing_tool", Arguments: `{}`}},
		{Type: "done", Usage: &llm.Usage{}},
		{Type: "content_delta", Content: "Handled"},
		{Type: "done", Usage: &llm.Usage{}},
	}, nil)

	exec := NewReactExecutor(mockLLM, NewCompositeToolExecutor(nil, 0, 0))
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Use a tool"}},
		Tools:    []ToolDefinition{{Name: "missing_tool"}},
		MaxTurns: 2,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	var found bool
	for _, evt := range events {
		if evt.Type == EventTypeToolResult {
			found = true
			if !strings.Contains(evt.ToolOutput, "unknown tool: missing_tool") {
				t.Fatalf("expected unknown tool output, got %q", evt.ToolOutput)
			}
		}
	}
	if !found {
		t.Fatalf("expected tool_result event, got %#v", events)
	}
}

func TestReactExecutor_ToolExecutionErrorReturnsToolResult(t *testing.T) {
	mockExec := newMockToolExecutor()
	mockExec.SetError("search", errors.New("backend offline"))
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{}`}},
		{Type: "done", Usage: &llm.Usage{}},
		{Type: "content_delta", Content: "Handled"},
		{Type: "done", Usage: &llm.Usage{}},
	}, nil)

	exec := NewReactExecutor(mockLLM, mockExec)
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Search"}},
		Tools:    []ToolDefinition{{Name: "search"}},
		MaxTurns: 2,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	var found bool
	for _, evt := range events {
		if evt.Type == EventTypeToolResult {
			found = true
			if !strings.Contains(evt.ToolOutput, "Error: backend offline") {
				t.Fatalf("expected execution error output, got %q", evt.ToolOutput)
			}
		}
	}
	if !found {
		t.Fatalf("expected tool_result event, got %#v", events)
	}
}

func TestReactExecutor_MaxTurnsExceeded(t *testing.T) {
	// Always return a tool call
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{}`}},
		{Type: "done", Usage: &llm.Usage{}},
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_2", Name: "search", Arguments: `{}`}},
		{Type: "done", Usage: &llm.Usage{}},
	}, nil)

	exec := NewReactExecutor(mockLLM, newMockToolExecutor())
	req := ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Test"}},
		Tools:    []ToolDefinition{{Name: "search"}},
		MaxTurns: 1,
	}

	ch, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	last := events[len(events)-1]
	if last.Type != EventTypeError {
		t.Fatalf("expected error event, got %q", last.Type)
	}
	if !strings.Contains(last.Message, "max turns (1) exceeded") {
		t.Errorf("expected max turns exceeded, got %q", last.Message)
	}
}

func TestReactExecutor_Cancelled(t *testing.T) {
	mockLLM := newMockLLMClient([]llm.StreamEvent{}, nil)
	exec := NewReactExecutor(mockLLM, newMockToolExecutor())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch, err := exec.Execute(ctx, ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	found := false
	for _, evt := range events {
		if evt.Type == EventTypeCancelled {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected cancelled event, got %v", events)
	}
}

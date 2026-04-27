package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"metis/internal/llm"
)

type controlledStreamLLMClient struct {
	streams []chan llm.StreamEvent
	next    int
}

func newControlledStreamLLMClient(turns int) *controlledStreamLLMClient {
	streams := make([]chan llm.StreamEvent, turns)
	for i := range streams {
		streams[i] = make(chan llm.StreamEvent, 8)
	}
	return &controlledStreamLLMClient{streams: streams}
}

func (c *controlledStreamLLMClient) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, llm.ErrNotSupported
}

func (c *controlledStreamLLMClient) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	if c.next >= len(c.streams) {
		return nil, errors.New("unexpected chat stream request")
	}
	stream := c.streams[c.next]
	c.next++
	return stream, nil
}

func (c *controlledStreamLLMClient) Embedding(context.Context, llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

type blockingStreamStartLLMClient struct{}

func (c *blockingStreamStartLLMClient) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, llm.ErrNotSupported
}

func (c *blockingStreamStartLLMClient) ChatStream(ctx context.Context, _ llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (c *blockingStreamStartLLMClient) Embedding(context.Context, llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

func collectEvents(ch <-chan Event) []Event {
	var events []Event
	for evt := range ch {
		events = append(events, evt)
	}
	return events
}

func waitEvent(t *testing.T, ch <-chan Event) Event {
	t.Helper()
	select {
	case evt, ok := <-ch:
		if !ok {
			t.Fatal("event channel closed")
		}
		return evt
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for executor event")
	}
	return Event{}
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

func TestReactExecutor_ChatStreamStartTimeoutEmitsError(t *testing.T) {
	previous := llmTurnTimeout
	llmTurnTimeout = 20 * time.Millisecond
	t.Cleanup(func() { llmTurnTimeout = previous })

	exec := NewReactExecutor(&blockingStreamStartLLMClient{}, newMockToolExecutor())
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	if len(events) < 2 {
		t.Fatalf("expected start and error events, got %+v", events)
	}
	if events[0].Type != EventTypeLLMStart {
		t.Fatalf("expected first event %q, got %+v", EventTypeLLMStart, events[0])
	}
	last := events[len(events)-1]
	if last.Type != EventTypeError {
		t.Fatalf("expected final event %q, got %+v", EventTypeError, last)
	}
	if !strings.Contains(last.Message, "timed out") {
		t.Fatalf("expected timeout message, got %q", last.Message)
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

func TestReactExecutor_EmitsToolCallBeforeLLMTurnDone(t *testing.T) {
	mockLLM := newControlledStreamLLMClient(2)
	mockExec := newMockToolExecutor()
	mockExec.SetResult("search", ToolResult{Output: "result from search"})

	exec := NewReactExecutor(mockLLM, mockExec)
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Search x"}},
		Tools:    []ToolDefinition{{Name: "search", Parameters: []byte(`{"type":"object"}`)}},
		MaxTurns: 2,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if evt := waitEvent(t, ch); evt.Type != EventTypeLLMStart {
		t.Fatalf("expected first event to be LLM start, got %+v", evt)
	}

	mockLLM.streams[0] <- llm.StreamEvent{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{"q":"x"}`}}
	toolCall := waitEvent(t, ch)
	if toolCall.Type != EventTypeToolCall || toolCall.ToolCallID != "call_1" || toolCall.ToolName != "search" {
		t.Fatalf("expected live tool call before LLM turn done, got %+v", toolCall)
	}

	select {
	case evt := <-ch:
		t.Fatalf("did not expect tool execution before LLM turn done, got %+v", evt)
	case <-time.After(25 * time.Millisecond):
	}

	mockLLM.streams[0] <- llm.StreamEvent{Type: "done", Usage: &llm.Usage{InputTokens: 3, OutputTokens: 5}}
	close(mockLLM.streams[0])

	if evt := waitEvent(t, ch); evt.Type != EventTypeToolResult || !strings.Contains(evt.ToolOutput, "result from search") {
		t.Fatalf("expected tool result after LLM turn done, got %+v", evt)
	}
	if evt := waitEvent(t, ch); evt.Type != EventTypeLLMStart {
		t.Fatalf("expected second turn LLM start, got %+v", evt)
	}

	mockLLM.streams[1] <- llm.StreamEvent{Type: "content_delta", Content: "Done"}
	if evt := waitEvent(t, ch); evt.Type != EventTypeContentDelta || evt.Text != "Done" {
		t.Fatalf("expected final content delta, got %+v", evt)
	}
	mockLLM.streams[1] <- llm.StreamEvent{Type: "done", Usage: &llm.Usage{InputTokens: 8, OutputTokens: 1}}
	close(mockLLM.streams[1])

	if evt := waitEvent(t, ch); evt.Type != EventTypeDone {
		t.Fatalf("expected done event, got %+v", evt)
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

func TestReactExecutor_ITSMDraftPrepareSurfacesUseStableID(t *testing.T) {
	mockExec := newMockToolExecutor()
	mockExec.SetResult("itsm.service_load", ToolResult{Output: `{"service_id":5,"name":"VPN 开通申请","engine_type":"smart"}`})
	mockExec.SetResult("itsm.draft_prepare", ToolResult{Output: `{
		"ok": true,
		"ready_for_confirmation": true,
		"service_id": 5,
		"service_name": "VPN 开通申请",
		"service_engine_type": "smart",
		"draft_version": 2,
		"summary": "申请 VPN",
		"form_data": {"vpn_account": "wenhaowu@dev.com"},
		"form_schema": {"version": 1, "fields": [{"key": "vpn_account", "type": "text", "label": "VPN账号"}]}
	}`})

	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_load", Name: "itsm.service_load", Arguments: `{"service_id":5}`}},
		{Type: "done", Usage: &llm.Usage{}},
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_draft", Name: "itsm.draft_prepare", Arguments: `{"service_id":5}`}},
		{Type: "done", Usage: &llm.Usage{}},
		{Type: "content_delta", Content: "请确认草稿。"},
		{Type: "done", Usage: &llm.Usage{}},
	}, nil)

	exec := NewReactExecutor(mockLLM, mockExec)
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "申请 VPN"}},
		Tools: []ToolDefinition{
			{Name: "itsm.service_load"},
			{Name: "itsm.draft_prepare"},
		},
		MaxTurns: 3,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	var surfaces []Event
	toolCallCounts := map[string]int{}
	for _, evt := range events {
		if evt.Type == EventTypeToolCall {
			toolCallCounts[evt.ToolCallID]++
		}
		if evt.Type == EventTypeUISurface && evt.SurfaceType == itsmDraftFormSurfaceType {
			surfaces = append(surfaces, evt)
		}
	}
	if toolCallCounts["call_draft"] != 1 {
		t.Fatalf("expected one draft_prepare tool call event, got %d in %+v", toolCallCounts["call_draft"], events)
	}
	if len(surfaces) != 2 {
		t.Fatalf("expected loading and ready draft surfaces, got %+v", surfaces)
	}
	if surfaces[0].SurfaceID != surfaces[1].SurfaceID || surfaces[0].SurfaceID != "itsm-draft-form-call_draft" {
		t.Fatalf("expected stable draft surface id, got %q and %q", surfaces[0].SurfaceID, surfaces[1].SurfaceID)
	}

	var loadingPayload, readyPayload map[string]any
	if err := json.Unmarshal(surfaces[0].SurfaceData, &loadingPayload); err != nil {
		t.Fatalf("unmarshal loading surface: %v", err)
	}
	if err := json.Unmarshal(surfaces[1].SurfaceData, &readyPayload); err != nil {
		t.Fatalf("unmarshal ready surface: %v", err)
	}
	if loadingPayload["status"] != "loading" || readyPayload["status"] != "ready" {
		t.Fatalf("expected loading then ready surfaces, got %#v then %#v", loadingPayload, readyPayload)
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

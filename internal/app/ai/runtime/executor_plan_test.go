package runtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"metis/internal/llm"
)

type scriptedPlanLLMClient struct {
	mu             sync.Mutex
	chatResponse   *llm.ChatResponse
	chatErr        error
	streams        [][]llm.StreamEvent
	streamErrs     []error
	chatRequests   []llm.ChatRequest
	streamRequests []llm.ChatRequest
}

func (m *scriptedPlanLLMClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatRequests = append(m.chatRequests, req)
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	if m.chatResponse == nil {
		return &llm.ChatResponse{}, nil
	}
	return m.chatResponse, nil
}

func (m *scriptedPlanLLMClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	m.mu.Lock()
	callIndex := len(m.streamRequests)
	m.streamRequests = append(m.streamRequests, req)
	var events []llm.StreamEvent
	if callIndex < len(m.streams) {
		events = m.streams[callIndex]
	}
	var err error
	if callIndex < len(m.streamErrs) {
		err = m.streamErrs[callIndex]
	}
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}

	ch := make(chan llm.StreamEvent, len(events))
	for _, evt := range events {
		ch <- evt
	}
	close(ch)
	return ch, nil
}

func (m *scriptedPlanLLMClient) Embedding(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

func (m *scriptedPlanLLMClient) streamRequest(index int) llm.ChatRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index >= len(m.streamRequests) {
		return llm.ChatRequest{}
	}
	return m.streamRequests[index]
}

func TestPlanAndExecuteExecutor_EmitsStepDoneAndCarriesPriorStepSummary(t *testing.T) {
	mockLLM := &scriptedPlanLLMClient{
		chatResponse: &llm.ChatResponse{
			Content: `[{"index":1,"description":"first"},{"index":2,"description":"second"}]`,
			Usage:   llm.Usage{InputTokens: 7, OutputTokens: 3},
		},
		streams: [][]llm.StreamEvent{
			{
				{Type: "content_delta", Content: "step one output"},
				{Type: "done", Usage: &llm.Usage{InputTokens: 4, OutputTokens: 2}},
			},
			{
				{Type: "content_delta", Content: "step two output"},
				{Type: "done", Usage: &llm.Usage{InputTokens: 5, OutputTokens: 2}},
			},
		},
	}

	exec := NewPlanAndExecuteExecutor(mockLLM, newMockToolExecutor())
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Do it"}},
		AgentConfig: AgentExecuteConfig{
			ModelName: "test-model",
			MaxTokens: 128,
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	var stepDone []int
	for _, evt := range events {
		if evt.Type == EventTypeStepDone {
			stepDone = append(stepDone, evt.StepIndex)
		}
	}
	if len(stepDone) != 2 || stepDone[0] != 1 || stepDone[1] != 2 {
		t.Fatalf("expected step_done for steps 1 and 2, got %#v from %#v", stepDone, events)
	}
	if events[len(events)-1].Type != EventTypeDone {
		t.Fatalf("expected final done event, got %#v", events[len(events)-1])
	}

	secondReq := mockLLM.streamRequest(1)
	var hasPriorSummary bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleAssistant &&
			strings.Contains(msg.Content, "Completed step 1: first") &&
			strings.Contains(msg.Content, "step one output") {
			hasPriorSummary = true
			break
		}
	}
	if !hasPriorSummary {
		t.Fatalf("expected second step request to include prior step summary, got %#v", secondReq.Messages)
	}
}

func TestPlanAndExecuteExecutor_ToolExecutionErrorBecomesToolResult(t *testing.T) {
	mockLLM := &scriptedPlanLLMClient{
		chatResponse: &llm.ChatResponse{
			Content: `[{"index":1,"description":"search"}]`,
		},
		streams: [][]llm.StreamEvent{
			{
				{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{}`}},
				{Type: "done", Usage: &llm.Usage{}},
			},
			{
				{Type: "content_delta", Content: "recovered"},
				{Type: "done", Usage: &llm.Usage{}},
			},
		},
	}
	mockExec := newMockToolExecutor()
	mockExec.SetError("search", errors.New("backend offline"))

	exec := NewPlanAndExecuteExecutor(mockLLM, mockExec)
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Search"}},
		Tools:    []ToolDefinition{{Name: "search", Parameters: []byte(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	var foundToolResult, foundStepDone bool
	for _, evt := range events {
		switch evt.Type {
		case EventTypeToolResult:
			foundToolResult = true
			if !strings.Contains(evt.ToolOutput, "Error: backend offline") {
				t.Fatalf("expected tool error output, got %q", evt.ToolOutput)
			}
		case EventTypeStepDone:
			foundStepDone = true
		}
	}
	if !foundToolResult || !foundStepDone {
		t.Fatalf("expected tool_result and step_done events, got %#v", events)
	}

	secondTurn := mockLLM.streamRequest(1)
	var hasToolMessage bool
	for _, msg := range secondTurn.Messages {
		if msg.Role == llm.RoleTool && strings.Contains(msg.Content, "Error: backend offline") {
			hasToolMessage = true
			break
		}
	}
	if !hasToolMessage {
		t.Fatalf("expected second turn to include tool error message, got %#v", secondTurn.Messages)
	}
}

func TestPlanAndExecuteExecutor_StepTurnBudgetExceeded(t *testing.T) {
	streams := make([][]llm.StreamEvent, 0, defaultStepTurnBudget)
	for i := 0; i < defaultStepTurnBudget; i++ {
		streams = append(streams, []llm.StreamEvent{
			{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "search", Arguments: `{}`}},
			{Type: "done", Usage: &llm.Usage{}},
		})
	}
	mockLLM := &scriptedPlanLLMClient{
		chatResponse: &llm.ChatResponse{
			Content: `[{"index":1,"description":"loop"}]`,
		},
		streams: streams,
	}

	exec := NewPlanAndExecuteExecutor(mockLLM, newMockToolExecutor())
	ch, err := exec.Execute(context.Background(), ExecuteRequest{
		Messages: []ExecuteMessage{{Role: MessageRoleUser, Content: "Loop"}},
		Tools:    []ToolDefinition{{Name: "search"}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	events := collectEvents(ch)
	last := events[len(events)-1]
	if last.Type != EventTypeError {
		t.Fatalf("expected final error event, got %#v", last)
	}
	if !strings.Contains(last.Message, "step 1 exceeded turn budget") {
		t.Fatalf("expected turn budget error, got %q", last.Message)
	}
}

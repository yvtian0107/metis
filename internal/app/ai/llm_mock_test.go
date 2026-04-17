package ai

import (
	"context"
	"sync"

	"metis/internal/llm"
)

// mockLLMClient is a test double for llm.Client that emits a programmed sequence of events.
type mockLLMClient struct {
	events []llm.StreamEvent
	err    error
	mu     sync.Mutex
	cursor int
}

func newMockLLMClient(events []llm.StreamEvent, err error) *mockLLMClient {
	return &mockLLMClient{events: events, err: err}
}

func (m *mockLLMClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, llm.ErrNotSupported
}

func (m *mockLLMClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan llm.StreamEvent)
	go func() {
		defer close(ch)
		for m.cursor < len(m.events) {
			select {
			case ch <- m.events[m.cursor]:
				m.mu.Lock()
				m.cursor++
				m.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (m *mockLLMClient) Embedding(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

// mockToolExecutor is a test double for ToolExecutor.
type mockToolExecutor struct {
	results map[string]ToolResult
	errs    map[string]error
}

func newMockToolExecutor() *mockToolExecutor {
	return &mockToolExecutor{
		results: make(map[string]ToolResult),
		errs:    make(map[string]error),
	}
}

func (m *mockToolExecutor) SetResult(name string, result ToolResult) {
	m.results[name] = result
}

func (m *mockToolExecutor) SetError(name string, err error) {
	m.errs[name] = err
}

func (m *mockToolExecutor) ExecuteTool(ctx context.Context, call ToolCall) (ToolResult, error) {
	if err, ok := m.errs[call.Name]; ok {
		return ToolResult{}, err
	}
	if res, ok := m.results[call.Name]; ok {
		return res, nil
	}
	return ToolResult{Output: "mock-default"}, nil
}

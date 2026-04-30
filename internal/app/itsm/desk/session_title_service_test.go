package desk

import (
	"context"
	"errors"
	"testing"

	"metis/internal/app/itsm/config"
	"metis/internal/llm"
)

type fakeTitleConfigProvider struct {
	intakeAgentID uint
	cfg           config.LLMEngineRuntimeConfig
	err           error
}

func (f fakeTitleConfigProvider) IntakeAgentID() uint { return f.intakeAgentID }
func (f fakeTitleConfigProvider) SessionTitleRuntimeConfig() (config.LLMEngineRuntimeConfig, error) {
	if f.err != nil {
		return config.LLMEngineRuntimeConfig{}, f.err
	}
	return f.cfg, nil
}

type fakeTitleLLMClient struct {
	responses []llm.ChatResponse
	errs      []error
	calls     int
}

func (f *fakeTitleLLMClient) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls++
	idx := f.calls - 1
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.responses) {
		resp := f.responses[idx]
		return &resp, nil
	}
	return &llm.ChatResponse{}, nil
}

func (*fakeTitleLLMClient) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return nil, llm.ErrNotSupported
}

func (*fakeTitleLLMClient) Embedding(context.Context, llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

func TestSessionTitleServiceSkipWhenNotIntakeAgent(t *testing.T) {
	svc := &SessionTitleService{
		configSvc: fakeTitleConfigProvider{intakeAgentID: 42},
		llmClientFactory: func(string, string, string) (llm.Client, error) {
			t.Fatalf("should not build llm client when agent is not intake")
			return nil, nil
		},
	}

	title, handled, err := svc.Generate(context.Background(), 1, 1, 7, "我想申请VPN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected not handled")
	}
	if title != "" {
		t.Fatalf("expected empty title, got %q", title)
	}
}

func TestSessionTitleServiceGenerate(t *testing.T) {
	svc := &SessionTitleService{
		configSvc: fakeTitleConfigProvider{
			intakeAgentID: 7,
			cfg: config.LLMEngineRuntimeConfig{
				Model:          "gpt-test",
				Protocol:       llm.ProtocolOpenAI,
				BaseURL:        "https://example.test",
				APIKey:         "test",
				Temperature:    0.2,
				MaxTokens:      96,
				MaxRetries:     1,
				TimeoutSeconds: 30,
				SystemPrompt:   "system",
			},
		},
		llmClientFactory: func(string, string, string) (llm.Client, error) {
			return &fakeTitleLLMClient{responses: []llm.ChatResponse{{Content: "\n\"VPN 线上支持申请\"\n"}}}, nil
		},
	}

	title, handled, err := svc.Generate(context.Background(), 1, 1, 7, "我想申请VPN，线上支持用")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if title != "VPN 线上支持申请" {
		t.Fatalf("unexpected title: %q", title)
	}
}

func TestSessionTitleServiceRetriesChatErrorsAndEmptyTitle(t *testing.T) {
	client := &fakeTitleLLMClient{
		errs: []error{context.DeadlineExceeded, nil, nil},
		responses: []llm.ChatResponse{
			{},
			{Content: "   "},
			{Content: "VPN 线上支持申请"},
		},
	}
	svc := &SessionTitleService{
		configSvc: fakeTitleConfigProvider{
			intakeAgentID: 7,
			cfg: config.LLMEngineRuntimeConfig{
				Model:          "gpt-test",
				Protocol:       llm.ProtocolOpenAI,
				BaseURL:        "https://example.test",
				APIKey:         "test",
				Temperature:    0.2,
				MaxTokens:      96,
				MaxRetries:     2,
				TimeoutSeconds: 30,
				SystemPrompt:   "system",
			},
		},
		llmClientFactory: func(string, string, string) (llm.Client, error) {
			return client, nil
		},
	}

	title, handled, err := svc.Generate(context.Background(), 1, 1, 7, "我想申请VPN，线上支持用")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if title != "VPN 线上支持申请" {
		t.Fatalf("unexpected title: %q", title)
	}
	if client.calls != 3 {
		t.Fatalf("expected three attempts, got %d", client.calls)
	}
}

func TestSessionTitleServiceGenerateError(t *testing.T) {
	svc := &SessionTitleService{
		configSvc: fakeTitleConfigProvider{
			intakeAgentID: 7,
			err:           errors.New("missing model"),
		},
	}

	title, handled, err := svc.Generate(context.Background(), 1, 1, 7, "我想申请VPN")
	if !handled {
		t.Fatalf("expected handled when intake agent")
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	if title != "" {
		t.Fatalf("expected empty title, got %q", title)
	}
}

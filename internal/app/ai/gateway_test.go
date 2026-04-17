package ai

import (
	"context"
	"io"
	"testing"
	"time"

	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/llm"
)

func newGatewayForTest(t *testing.T, db *gorm.DB, mockLLM llm.Client) *AgentGateway {
	t.Helper()
	agentRepo := &AgentRepo{db: &database.DB{DB: db}}
	modelRepo := &ModelRepo{db: &database.DB{DB: db}}
	providerRepo := &ProviderRepo{db: &database.DB{DB: db}}
	sessionRepo := &SessionRepo{db: &database.DB{DB: db}}
	memoryRepo := &MemoryRepo{db: &database.DB{DB: db}}

	agentSvc := &AgentService{repo: agentRepo}
	sessionSvc := &SessionService{repo: sessionRepo, agentSvc: agentSvc}
	memorySvc := &MemoryService{repo: memoryRepo}

	return &AgentGateway{
		agentSvc:              agentSvc,
		sessionSvc:            sessionSvc,
		memorySvc:             memorySvc,
		agentRepo:             agentRepo,
		modelRepo:             modelRepo,
		providerRepo:          providerRepo,
		encKey:                newTestEncryptionKey(t),
		toolRegistries:        []ToolHandlerRegistry{NewGeneralToolRegistry(nil, nil)},
		executions:            make(map[uint]context.CancelFunc),
		testLLMClientOverride: mockLLM,
	}
}

func TestGateway_BuildToolDefinitions_FiltersInactive(t *testing.T) {
	db := setupTestDB(t)
	gw := newGatewayForTest(t, db, nil)
	agentRepo := gw.agentRepo

	// Create agent
	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentRepo.Create(agent)

	// Seed tools
	activeTool := &Tool{Name: "active", Toolkit: "general", DisplayName: "Active", IsActive: true}
	inactiveTool := &Tool{Name: "inactive", Toolkit: "general", DisplayName: "Inactive", IsActive: false}
	_ = agentRepo.db.Create(activeTool).Error
	_ = agentRepo.db.Create(inactiveTool).Error
	// Work around GORM default:true behavior for bool zero values
	_ = agentRepo.db.Model(inactiveTool).Update("is_active", false).Error

	// Bind both
	_ = agentRepo.ReplaceToolBindings(agent.ID, []uint{activeTool.ID, inactiveTool.ID})

	defs, err := gw.buildToolDefinitions(agent.ID)
	if err != nil {
		t.Fatalf("buildToolDefinitions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 active tool definition, got %d", len(defs))
	}
	if defs[0].Name != "active" {
		t.Errorf("expected active tool, got %q", defs[0].Name)
	}
}

func TestGateway_BuildToolDefinitions_IncludesMCP(t *testing.T) {
	db := setupTestDB(t)
	gw := newGatewayForTest(t, db, nil)
	agentRepo := gw.agentRepo

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentRepo.Create(agent)

	mcp := &MCPServer{Name: "MyServer", Transport: MCPTransportSSE, URL: "https://example.com", IsActive: true}
	_ = agentRepo.db.Create(mcp).Error
	_ = agentRepo.ReplaceMCPServerBindings(agent.ID, []uint{mcp.ID})

	defs, err := gw.buildToolDefinitions(agent.ID)
	if err != nil {
		t.Fatalf("buildToolDefinitions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 mcp definition, got %d", len(defs))
	}
	if defs[0].Type != "mcp" {
		t.Errorf("expected type mcp, got %q", defs[0].Type)
	}
	if defs[0].Name != "mcp_MyServer" {
		t.Errorf("expected name mcp_MyServer, got %q", defs[0].Name)
	}
}

func TestGateway_SystemPromptAssembly(t *testing.T) {
	agent := &Agent{
		SystemPrompt: "You are a helpful assistant.",
		Instructions: "Be concise.",
	}
	memoryBlock := "User prefers JSON output."

	prompt := buildSystemPrompt(agent, memoryBlock)
	expected := "You are a helpful assistant.\n\nBe concise.\n\nUser prefers JSON output."
	if prompt != expected {
		t.Errorf("expected %q, got %q", expected, prompt)
	}
}

func TestGateway_Run_CompletesSession(t *testing.T) {
	db := setupTestDB(t)
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "content_delta", Content: "Hello!"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 2, OutputTokens: 2}},
	}, nil)
	gw := newGatewayForTest(t, db, mockLLM)

	// Seed agent
	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, CreatedBy: 1}
	_ = gw.agentSvc.Create(agent)

	session, _ := gw.sessionSvc.Create(agent.ID, 1)

	reader, err := gw.Run(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = drainReader(reader)

	// Allow goroutine to finish
	time.Sleep(100 * time.Millisecond)

	loaded, _ := gw.sessionSvc.Get(session.ID)
	if loaded.Status != SessionStatusCompleted {
		t.Errorf("status: expected %q, got %q", SessionStatusCompleted, loaded.Status)
	}

	messages, _ := gw.sessionSvc.GetMessages(session.ID)
	var foundAssistant bool
	for _, m := range messages {
		if m.Role == MessageRoleAssistant && m.Content == "Hello!" {
			foundAssistant = true
		}
	}
	if !foundAssistant {
		t.Errorf("expected assistant message 'Hello!', got %v", messages)
	}
}

func TestGateway_Run_ErrorSession(t *testing.T) {
	db := setupTestDB(t)
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "error", Error: "model failure"},
	}, nil)
	gw := newGatewayForTest(t, db, mockLLM)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, CreatedBy: 1}
	_ = gw.agentSvc.Create(agent)
	session, _ := gw.sessionSvc.Create(agent.ID, 1)

	reader, err := gw.Run(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = drainReader(reader)
	time.Sleep(100 * time.Millisecond)

	loaded, _ := gw.sessionSvc.Get(session.ID)
	if loaded.Status != SessionStatusError {
		t.Errorf("status: expected %q, got %q", SessionStatusError, loaded.Status)
	}
}

func TestGateway_Run_CancelledSession(t *testing.T) {
	db := setupTestDB(t)
	// Use a mock that blocks until cancelled so we can cancel mid-run
	mockLLM := &blockingMockLLMClient{}
	gw := newGatewayForTest(t, db, mockLLM)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, CreatedBy: 1}
	_ = gw.agentSvc.Create(agent)
	session, _ := gw.sessionSvc.Create(agent.ID, 1)

	ctx, cancel := context.WithCancel(context.Background())
	reader, err := gw.Run(ctx, session.ID)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Cancel quickly
	cancel()
	_ = drainReader(reader)
	time.Sleep(100 * time.Millisecond)

	loaded, _ := gw.sessionSvc.Get(session.ID)
	if loaded.Status != SessionStatusCancelled {
		t.Errorf("status: expected %q, got %q", SessionStatusCancelled, loaded.Status)
	}
}

// blockingMockLLMClient blocks ChatStream until the context is cancelled.
type blockingMockLLMClient struct{}

func (m *blockingMockLLMClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, llm.ErrNotSupported
}

func (m *blockingMockLLMClient) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

func (m *blockingMockLLMClient) Embedding(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrNotSupported
}

func drainReader(r io.Reader) string {
	buf, _ := io.ReadAll(r)
	return string(buf)
}

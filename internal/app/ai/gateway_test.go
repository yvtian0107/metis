package ai

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"gorm.io/gorm"

	"metis/internal/database"
	"metis/internal/llm"
	"metis/internal/model"
)

type recordingStreamEncoder struct {
	encoded []Event
	closed  bool
}

func (e *recordingStreamEncoder) Encode(evt Event) error {
	e.encoded = append(e.encoded, evt)
	return nil
}

func (e *recordingStreamEncoder) Close() error {
	e.closed = true
	return nil
}

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
		agentSvc:       agentSvc,
		sessionSvc:     sessionSvc,
		memorySvc:      memorySvc,
		agentRepo:      agentRepo,
		modelRepo:      modelRepo,
		providerRepo:   providerRepo,
		mcpClient:      &fakeMCPRuntimeClient{},
		encKey:         newTestEncryptionKey(t),
		toolRegistries: []ToolHandlerRegistry{NewGeneralToolRegistry(nil, nil)},
		executions:     make(map[uint]context.CancelFunc),
		streamEncoderFactory: func(w io.Writer) StreamEncoder {
			return NewUIMessageStreamEncoder(w)
		},
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
	activeTool := &Tool{Name: "general.current_time", Toolkit: "general", DisplayName: "Active", IsActive: true}
	inactiveTool := &Tool{Name: "system.current_user_profile", Toolkit: "general", DisplayName: "Inactive", IsActive: false}
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
	if defs[0].Name != "general.current_time" {
		t.Errorf("expected general.current_time tool, got %q", defs[0].Name)
	}
}

func TestGateway_AssistantRuntime_IncludesDiscoveredMCP(t *testing.T) {
	db := setupTestDB(t)
	gw := newGatewayForTest(t, db, nil)
	agentRepo := gw.agentRepo

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, CreatedBy: 1}
	_ = agentRepo.Create(agent)

	mcp := &MCPServer{Name: "MyServer", Transport: MCPTransportSSE, URL: "https://example.com", IsActive: true}
	_ = agentRepo.db.Create(mcp).Error
	_ = agentRepo.ReplaceMCPServerBindings(agent.ID, []uint{mcp.ID})
	gw.mcpClient = &fakeMCPRuntimeClient{toolsByServer: map[uint][]MCPRuntimeTool{
		mcp.ID: {{Name: "search", Description: "Search", Parameters: []byte(`{"type":"object"}`)}},
	}}

	runtime, err := gw.buildAssistantRuntime(context.Background(), agent, &AgentSession{AgentID: agent.ID, UserID: 1}, nil, "")
	if err != nil {
		t.Fatalf("buildAssistantRuntime: %v", err)
	}
	if len(runtime.Tools) != 1 {
		t.Fatalf("expected 1 mcp definition, got %d", len(runtime.Tools))
	}
	if runtime.Tools[0].Type != "mcp" {
		t.Errorf("expected type mcp, got %q", runtime.Tools[0].Type)
	}
	if runtime.Tools[0].Name != "mcp__MyServer__search" {
		t.Errorf("expected discovered MCP tool name, got %q", runtime.Tools[0].Name)
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

func TestBuildExecuteMessagesFromSessionMessages_ReplaysCompletedToolTranscript(t *testing.T) {
	messages := []SessionMessage{
		{Role: MessageRoleUser, Content: "查一下 VPN 服务", Sequence: 1},
		{
			Role:     MessageRoleToolCall,
			Metadata: model.JSONText([]byte(`{"tool_call_id":"call_1","tool_name":"itsm.service_match","tool_args":{"query":"申请VPN"},"status":"running"}`)),
			Sequence: 2,
		},
		{
			Role:     MessageRoleToolResult,
			Content:  `{"selected_service_id":5,"next_required_tool":"itsm.service_load"}`,
			Metadata: model.JSONText([]byte(`{"tool_call_id":"call_1","status":"completed"}`)),
			Sequence: 3,
		},
		{Role: MessageRoleAssistant, Content: "已匹配到 VPN 服务。", Sequence: 4},
	}

	execMessages := buildExecuteMessagesFromSessionMessages(messages)

	if len(execMessages) != 4 {
		t.Fatalf("expected user, assistant tool_call, tool result, assistant messages; got %+v", execMessages)
	}
	if execMessages[1].Role != MessageRoleAssistant || len(execMessages[1].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call message, got %+v", execMessages[1])
	}
	call := execMessages[1].ToolCalls[0]
	if call.ID != "call_1" || call.Name != "itsm.service_match" || call.Arguments != `{"query":"申请VPN"}` {
		t.Fatalf("unexpected tool call replay: %+v", call)
	}
	if execMessages[2].Role != llm.RoleTool || execMessages[2].ToolCallID != "call_1" {
		t.Fatalf("expected tool result message with call id, got %+v", execMessages[2])
	}
	if execMessages[2].Content != `{"selected_service_id":5,"next_required_tool":"itsm.service_load"}` {
		t.Fatalf("unexpected tool output: %s", execMessages[2].Content)
	}
}

func TestBuildExecuteMessagesFromSessionMessages_SkipsIncompleteToolTranscript(t *testing.T) {
	messages := []SessionMessage{
		{Role: MessageRoleUser, Content: "继续", Sequence: 1},
		{
			Role:     MessageRoleToolCall,
			Metadata: model.JSONText([]byte(`{"tool_call_id":"call_missing","tool_name":"itsm.service_load","tool_args":{"service_id":5},"status":"running"}`)),
			Sequence: 2,
		},
		{
			Role:     MessageRoleToolResult,
			Content:  `{"ok":true}`,
			Metadata: model.JSONText([]byte(`{"tool_call_id":"orphan","status":"completed"}`)),
			Sequence: 3,
		},
		{Role: MessageRoleAssistant, Content: "请稍后。", Sequence: 4},
	}

	execMessages := buildExecuteMessagesFromSessionMessages(messages)

	if len(execMessages) != 2 {
		t.Fatalf("expected only user and assistant messages, got %+v", execMessages)
	}
	if execMessages[0].Role != MessageRoleUser || execMessages[1].Role != MessageRoleAssistant {
		t.Fatalf("unexpected messages after skipping incomplete transcript: %+v", execMessages)
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

	reader, err := gw.Run(context.Background(), session.ID, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = drainReader(reader)

	// Allow goroutine to finish
	time.Sleep(100 * time.Millisecond)

	loaded, _ := gw.sessionSvc.GetOwned(session.ID, 1)
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

func TestGateway_Run_ReplaysStoredToolTranscriptToLLM(t *testing.T) {
	db := setupTestDB(t)
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "content_delta", Content: "继续处理"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 4, OutputTokens: 2}},
	}, nil)
	gw := newGatewayForTest(t, db, mockLLM)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, CreatedBy: 1}
	_ = gw.agentSvc.Create(agent)
	session, _ := gw.sessionSvc.Create(agent.ID, 1)

	_, _ = gw.sessionSvc.StoreMessage(session.ID, MessageRoleUser, "我要申请VPN", nil, 0)
	_, _ = gw.sessionSvc.StoreMessage(
		session.ID,
		MessageRoleToolCall,
		"",
		model.JSONText([]byte(`{"tool_call_id":"call_1","tool_name":"itsm.service_match","tool_args":{"query":"我要申请VPN"},"status":"running"}`)),
		0,
	)
	_, _ = gw.sessionSvc.StoreMessage(
		session.ID,
		MessageRoleToolResult,
		`{"selected_service_id":5,"next_required_tool":"itsm.service_load"}`,
		model.JSONText([]byte(`{"tool_call_id":"call_1","status":"completed"}`)),
		0,
	)
	_, _ = gw.sessionSvc.StoreMessage(session.ID, MessageRoleUser, "是的", nil, 0)

	reader, err := gw.Run(context.Background(), session.ID, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = drainReader(reader)
	time.Sleep(100 * time.Millisecond)

	requests := mockLLM.Requests()
	if len(requests) == 0 {
		t.Fatalf("expected LLM request")
	}
	var foundAssistantToolCall, foundToolResult bool
	for _, msg := range requests[0].Messages {
		if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) == 1 && msg.ToolCalls[0].ID == "call_1" {
			foundAssistantToolCall = true
		}
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call_1" && msg.Content == `{"selected_service_id":5,"next_required_tool":"itsm.service_load"}` {
			foundToolResult = true
		}
	}
	if !foundAssistantToolCall || !foundToolResult {
		t.Fatalf("expected stored tool transcript in LLM request, got %+v", requests[0].Messages)
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

	reader, err := gw.Run(context.Background(), session.ID, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = drainReader(reader)
	time.Sleep(100 * time.Millisecond)

	loaded, _ := gw.sessionSvc.GetOwned(session.ID, 1)
	if loaded.Status != SessionStatusError {
		t.Errorf("status: expected %q, got %q", SessionStatusError, loaded.Status)
	}
}

func TestGateway_Run_StoresToolMetadata(t *testing.T) {
	db := setupTestDB(t)
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "tool_call", ToolCall: &llm.ToolCall{ID: "call_1", Name: "general.current_time", Arguments: `{"timezone":"Asia/Shanghai"}`}},
		{Type: "done", Usage: &llm.Usage{InputTokens: 3, OutputTokens: 2}},
		{Type: "content_delta", Content: "Done"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 8, OutputTokens: 1}},
	}, nil)
	gw := newGatewayForTest(t, db, mockLLM)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, CreatedBy: 1}
	_ = gw.agentSvc.Create(agent)
	session, _ := gw.sessionSvc.Create(agent.ID, 1)

	reader, err := gw.Run(context.Background(), session.ID, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = drainReader(reader)
	time.Sleep(100 * time.Millisecond)

	messages, _ := gw.sessionSvc.GetMessages(session.ID)
	var foundCall, foundResult bool
	for _, msg := range messages {
		var meta map[string]any
		if len(msg.Metadata) > 0 {
			if err := json.Unmarshal(msg.Metadata, &meta); err != nil {
				t.Fatalf("metadata json: %v", err)
			}
		}
		switch msg.Role {
		case MessageRoleToolCall:
			foundCall = true
			if meta["tool_call_id"] != "call_1" || meta["tool_name"] != "general.current_time" || meta["status"] != "running" {
				t.Fatalf("unexpected tool_call metadata: %#v", meta)
			}
		case MessageRoleToolResult:
			foundResult = true
			if meta["tool_call_id"] != "call_1" || meta["status"] != "completed" {
				t.Fatalf("unexpected tool_result metadata: %#v", meta)
			}
		}
	}
	if !foundCall || !foundResult {
		t.Fatalf("expected tool_call and tool_result messages, got %#v", messages)
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
	reader, err := gw.Run(ctx, session.ID, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Cancel quickly
	cancel()
	_ = drainReader(reader)
	time.Sleep(100 * time.Millisecond)

	loaded, _ := gw.sessionSvc.GetOwned(session.ID, 1)
	if loaded.Status != SessionStatusCancelled {
		t.Errorf("status: expected %q, got %q", SessionStatusCancelled, loaded.Status)
	}
}

func TestGateway_Run_CrossUserSessionReturnsNotFound(t *testing.T) {
	db := setupTestDB(t)
	gw := newGatewayForTest(t, db, nil)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, CreatedBy: 1, Visibility: AgentVisibilityTeam}
	_ = gw.agentSvc.Create(agent)
	session, _ := gw.sessionSvc.Create(agent.ID, 1)

	if _, err := gw.Run(context.Background(), session.ID, 2); err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestGateway_Run_UsesConfiguredEncoderFactory(t *testing.T) {
	db := setupTestDB(t)
	mockLLM := newMockLLMClient([]llm.StreamEvent{
		{Type: "content_delta", Content: "Hello!"},
		{Type: "done", Usage: &llm.Usage{InputTokens: 2, OutputTokens: 2}},
	}, nil)
	gw := newGatewayForTest(t, db, mockLLM)

	modelID := uint(1)
	agent := &Agent{Name: "Agent", Type: AgentTypeAssistant, ModelID: &modelID, Strategy: AgentStrategyReact, CreatedBy: 1, Visibility: AgentVisibilityTeam}
	_ = gw.agentSvc.Create(agent)
	session, _ := gw.sessionSvc.Create(agent.ID, 1)

	recorder := &recordingStreamEncoder{}
	gw.streamEncoderFactory = func(w io.Writer) StreamEncoder {
		return recorder
	}

	reader, err := gw.Run(context.Background(), session.ID, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	_ = drainReader(reader)
	time.Sleep(100 * time.Millisecond)

	if len(recorder.encoded) == 0 {
		t.Fatal("expected encoder to receive events")
	}
	if !recorder.closed {
		t.Fatal("expected encoder to be closed")
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

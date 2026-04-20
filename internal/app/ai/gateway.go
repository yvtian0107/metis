package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/samber/do/v2"

	"metis/internal/app"
	"metis/internal/llm"
	"metis/internal/pkg/crypto"
)

// AgentGateway orchestrates the full agent execution flow.
type AgentGateway struct {
	agentSvc     *AgentService
	sessionSvc   *SessionService
	memorySvc    *MemoryService
	agentRepo    *AgentRepo
	modelRepo    *ModelRepo
	providerRepo *ProviderRepo
	encKey       crypto.EncryptionKey

	// Tool registries for building CompositeToolExecutor per session
	toolRegistries []ToolHandlerRegistry

	// Active execution contexts, keyed by session ID
	mu         sync.Mutex
	executions map[uint]context.CancelFunc

	// testLLMClientOverride is used in tests to inject a mock LLM client.
	testLLMClientOverride llm.Client
	streamEncoderFactory  StreamEncoderFactory
}

func NewAgentGateway(i do.Injector) (*AgentGateway, error) {
	return &AgentGateway{
		agentSvc:       do.MustInvoke[*AgentService](i),
		sessionSvc:     do.MustInvoke[*SessionService](i),
		memorySvc:      do.MustInvoke[*MemoryService](i),
		agentRepo:      do.MustInvoke[*AgentRepo](i),
		modelRepo:      do.MustInvoke[*ModelRepo](i),
		providerRepo:   do.MustInvoke[*ProviderRepo](i),
		encKey:         do.MustInvoke[crypto.EncryptionKey](i),
		toolRegistries: collectToolRegistries(i),
		executions:     make(map[uint]context.CancelFunc),
		streamEncoderFactory: func(w io.Writer) StreamEncoder {
			return NewUIMessageStreamEncoder(w)
		},
	}, nil
}

// Run executes an agent for a given session. Returns a UI Message Stream reader.
func (gw *AgentGateway) Run(ctx context.Context, sessionID, userID uint) (io.ReadCloser, error) {
	session, err := gw.sessionSvc.GetOwned(sessionID, userID)
	if err != nil {
		return nil, err
	}

	agent, err := gw.agentSvc.Get(session.AgentID)
	if err != nil {
		return nil, err
	}

	if !agent.IsActive {
		return nil, errors.New("agent is inactive")
	}

	// Load message history
	messages, err := gw.sessionSvc.GetMessages(sessionID)
	if err != nil {
		return nil, fmt.Errorf("load messages: %w", err)
	}

	// Build system prompt with memory injection
	memoryBlock, _ := gw.memorySvc.FormatForPrompt(agent.ID, session.UserID)
	systemPrompt := buildSystemPrompt(agent, memoryBlock)

	// Convert messages to ExecuteMessage format
	execMessages := make([]ExecuteMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == MessageRoleUser || m.Role == MessageRoleAssistant {
			var images []string
			if len(m.Metadata) > 0 {
				var meta struct {
					Images []string `json:"images"`
				}
				_ = json.Unmarshal(m.Metadata, &meta)
				images = meta.Images
			}
			execMessages = append(execMessages, ExecuteMessage{
				Role:    m.Role,
				Content: m.Content,
				Images:  images,
			})
		}
	}

	// Build tool definitions from bindings
	tools, err := gw.buildToolDefinitions(agent.ID)
	if err != nil {
		return nil, fmt.Errorf("build tools: %w", err)
	}

	// Fetch model info for ModelName
	var modelID uint
	var modelName string
	if agent.ModelID != nil {
		modelID = *agent.ModelID
		model, err := gw.modelRepo.FindByID(*agent.ModelID)
		if err == nil {
			modelName = model.ModelID
		}
	}

	// Convert temperature to *float32 (nil if 0 to avoid sending it for models that don't support it)
	var tempPtr *float32
	if agent.Temperature != 0 {
		temp := float32(agent.Temperature)
		tempPtr = &temp
	}

	execReq := ExecuteRequest{
		SessionID: sessionID,
		AgentConfig: AgentExecuteConfig{
			Type:          agent.Type,
			Strategy:      agent.Strategy,
			ModelID:       modelID,
			ModelName:     modelName,
			Temperature:   tempPtr,
			MaxTokens:     agent.MaxTokens,
			Runtime:       agent.Runtime,
			RuntimeConfig: json.RawMessage(agent.RuntimeConfig),
			ExecMode:      agent.ExecMode,
			NodeID:        agent.NodeID,
			Workspace:     agent.Workspace,
			Instructions:  agent.Instructions,
		},
		Messages:     execMessages,
		SystemPrompt: systemPrompt,
		Tools:        tools,
		MaxTurns:     agent.MaxTurns,
	}

	// Select executor
	executor, err := gw.selectExecutor(agent, sessionID, session.UserID)
	if err != nil {
		return nil, err
	}

	// Create cancellable context
	execCtx, cancel := context.WithCancel(ctx)
	gw.mu.Lock()
	gw.executions[sessionID] = cancel
	gw.mu.Unlock()

	// Execute
	eventCh, err := executor.Execute(execCtx, execReq)
	if err != nil {
		cancel()
		gw.mu.Lock()
		delete(gw.executions, sessionID)
		gw.mu.Unlock()
		return nil, err
	}

	// Set up UI Message Stream pipe
	pr, pw := io.Pipe()
	encoder := gw.streamEncoderFactory(pw)

	go func() {
		defer func() {
			gw.mu.Lock()
			delete(gw.executions, sessionID)
			gw.mu.Unlock()
		}()
		defer encoder.Close()
		defer pw.Close()

		var assistantContent string

		for {
			select {
			case evt, ok := <-eventCh:
				if !ok {
					return
				}
				if err := encoder.Encode(evt); err != nil {
					pw.CloseWithError(err)
					return
				}

				// Post-process: store results
				switch evt.Type {
				case EventTypeContentDelta:
					assistantContent += evt.Text

				case EventTypeToolCall:
					meta, _ := json.Marshal(map[string]any{
						"tool_name": evt.ToolName,
						"tool_args": evt.ToolArgs,
					})
					_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleToolCall, "", meta, 0)

				case EventTypeToolResult:
					meta, _ := json.Marshal(map[string]any{
						"tool_call_id": evt.ToolCallID,
						"duration_ms":  evt.DurationMs,
					})
					_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleToolResult, evt.ToolOutput, meta, 0)

				case EventTypeMemoryUpdate:
					_ = gw.memorySvc.Upsert(&AgentMemory{
						AgentID: session.AgentID,
						UserID:  session.UserID,
						Key:     evt.MemoryKey,
						Content: evt.MemoryContent,
						Source:  MemorySourceAgentGenerated,
					})

				case EventTypeDone:
					if assistantContent != "" {
						_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleAssistant, assistantContent, nil, evt.OutputTokens)
					}
					_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusCompleted)

				case EventTypeError:
					if assistantContent != "" {
						_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleAssistant, assistantContent, nil, 0)
					}
					_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusError)

				case EventTypeCancelled:
					if assistantContent != "" {
						_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleAssistant, assistantContent, nil, 0)
					}
					_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusCancelled)
				}

			case <-execCtx.Done():
				if assistantContent != "" {
					_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleAssistant, assistantContent, nil, 0)
				}
				_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusCancelled)
				pw.CloseWithError(context.Canceled)
				return
			}
		}
	}()

	return pr, nil
}

// Cancel cancels an active execution for the given session.
func (gw *AgentGateway) Cancel(sessionID, userID uint) {
	if _, err := gw.sessionSvc.GetOwned(sessionID, userID); err != nil {
		return
	}
	gw.mu.Lock()
	cancel, ok := gw.executions[sessionID]
	gw.mu.Unlock()
	if ok {
		cancel()
	}
}

func (gw *AgentGateway) selectExecutor(agent *Agent, sessionID, userID uint) (Executor, error) {
	switch agent.Type {
	case AgentTypeAssistant:
		client, err := gw.buildLLMClient(agent)
		if err != nil {
			return nil, fmt.Errorf("build LLM client: %w", err)
		}
		toolExec := NewCompositeToolExecutor(gw.toolRegistries, sessionID, userID)
		switch agent.Strategy {
		case AgentStrategyPlanAndExecute:
			return NewPlanAndExecuteExecutor(client, toolExec), nil
		default:
			return NewReactExecutor(client, toolExec), nil
		}

	case AgentTypeCoding:
		switch agent.ExecMode {
		case AgentExecModeRemote:
			return NewRemoteCodingExecutor(), nil
		default:
			return NewLocalCodingExecutor(), nil
		}

	default:
		return nil, fmt.Errorf("unsupported agent type: %s", agent.Type)
	}
}

func buildSystemPrompt(agent *Agent, memoryBlock string) string {
	prompt := agent.SystemPrompt
	if agent.Instructions != "" {
		prompt += "\n\n" + agent.Instructions
	}
	if memoryBlock != "" {
		prompt += "\n\n" + memoryBlock
	}
	return prompt
}

func (gw *AgentGateway) buildLLMClient(agent *Agent) (llm.Client, error) {
	if gw.testLLMClientOverride != nil {
		return gw.testLLMClientOverride, nil
	}
	if agent.ModelID == nil {
		return nil, errors.New("agent has no model configured")
	}
	model, err := gw.modelRepo.FindByID(*agent.ModelID)
	if err != nil {
		return nil, fmt.Errorf("model not found: %w", err)
	}
	provider, err := gw.providerRepo.FindByID(model.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}
	apiKey, err := decryptAPIKey(provider.APIKeyEncrypted, gw.encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt API key: %w", err)
	}
	return llm.NewClient(provider.Protocol, provider.BaseURL, apiKey)
}

func (gw *AgentGateway) buildToolDefinitions(agentID uint) ([]ToolDefinition, error) {
	var defs []ToolDefinition

	// Builtin tools
	toolIDs, err := gw.agentRepo.GetToolIDs(agentID)
	if err != nil {
		return nil, err
	}
	for _, tid := range toolIDs {
		var tool Tool
		if err := gw.agentRepo.db.First(&tool, tid).Error; err != nil {
			slog.Warn("tool not found for binding", "toolId", tid)
			continue
		}
		if !tool.IsActive {
			continue
		}
		defs = append(defs, ToolDefinition{
			Type:        "builtin",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  json.RawMessage(tool.ParametersSchema),
			SourceID:    tool.ID,
		})
	}

	// MCP servers — add all tools from each server
	mcpIDs, err := gw.agentRepo.GetMCPServerIDs(agentID)
	if err != nil {
		return nil, err
	}
	for _, mid := range mcpIDs {
		var mcp MCPServer
		if err := gw.agentRepo.db.First(&mcp, mid).Error; err != nil {
			continue
		}
		if !mcp.IsActive {
			continue
		}
		// MCP tools are discovered at runtime from the server
		// For now, register the server as a single meta-tool
		defs = append(defs, ToolDefinition{
			Type:        "mcp",
			Name:        "mcp_" + mcp.Name,
			Description: mcp.Description,
			SourceID:    mcp.ID,
		})
	}

	return defs, nil
}

// --- app.AIAgentProvider implementation ---

// GetAgentConfig returns agent configuration by ID for external consumers (e.g. ITSM SmartEngine).
func (gw *AgentGateway) GetAgentConfig(agentID uint) (*app.AIAgentConfig, error) {
	agent, err := gw.agentSvc.Get(agentID)
	if err != nil {
		return nil, err
	}
	return gw.buildAgentConfig(agent)
}

func (gw *AgentGateway) buildAgentConfig(agent *Agent) (*app.AIAgentConfig, error) {
	cfg := &app.AIAgentConfig{
		Name:         agent.Name,
		SystemPrompt: agent.SystemPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
	}
	if agent.ModelID == nil {
		return cfg, nil
	}
	m, err := gw.modelRepo.FindByID(*agent.ModelID)
	if err != nil {
		return nil, fmt.Errorf("model not found: %w", err)
	}
	cfg.Model = m.ModelID
	provider, err := gw.providerRepo.FindByID(m.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}
	cfg.Protocol = provider.Protocol
	cfg.BaseURL = provider.BaseURL
	apiKey, err := decryptAPIKey(provider.APIKeyEncrypted, gw.encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt API key: %w", err)
	}
	cfg.APIKey = apiKey
	return cfg, nil
}

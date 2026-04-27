package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/samber/do/v2"

	"metis/internal/app"
	"metis/internal/llm"
	"metis/internal/pkg/crypto"
)

const streamHeartbeatInterval = 15 * time.Second

// AgentGateway orchestrates the full agent execution flow.
type AgentGateway struct {
	agentSvc          *AgentService
	sessionSvc        *SessionService
	memorySvc         *MemoryService
	agentRepo         *AgentRepo
	modelRepo         *ModelRepo
	providerRepo      *ProviderRepo
	knowledgeSearcher *KnowledgeSearchService
	mcpClient         MCPRuntimeClient
	encKey            crypto.EncryptionKey

	// Tool registries for building CompositeToolExecutor per session
	toolRegistries          []ToolHandlerRegistry
	runtimeContextProviders []app.AgentRuntimeContextProvider

	// Active execution contexts, keyed by session ID
	mu         sync.Mutex
	executions map[uint]context.CancelFunc

	// testLLMClientOverride is used in tests to inject a mock LLM client.
	testLLMClientOverride llm.Client
	streamEncoderFactory  StreamEncoderFactory
}

func NewAgentGateway(i do.Injector) (*AgentGateway, error) {
	return &AgentGateway{
		agentSvc:                do.MustInvoke[*AgentService](i),
		sessionSvc:              do.MustInvoke[*SessionService](i),
		memorySvc:               do.MustInvoke[*MemoryService](i),
		agentRepo:               do.MustInvoke[*AgentRepo](i),
		modelRepo:               do.MustInvoke[*ModelRepo](i),
		providerRepo:            do.MustInvoke[*ProviderRepo](i),
		knowledgeSearcher:       do.MustInvoke[*KnowledgeSearchService](i),
		mcpClient:               do.MustInvoke[MCPRuntimeClient](i),
		encKey:                  do.MustInvoke[crypto.EncryptionKey](i),
		toolRegistries:          collectToolRegistries(i),
		runtimeContextProviders: collectRuntimeContextProviders(),
		executions:              make(map[uint]context.CancelFunc),
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

	execMessages := buildExecuteMessagesFromSessionMessages(messages)

	var runtime *assistantRuntimeAssembly
	if agent.Type == AgentTypeAssistant {
		runtime, err = gw.buildAssistantRuntime(ctx, agent, session, execMessages, systemPrompt)
		if err != nil {
			return nil, fmt.Errorf("build assistant runtime: %w", err)
		}
		systemPrompt = runtime.SystemPrompt
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
		MaxTurns:     agent.MaxTurns,
	}
	if runtime != nil {
		execReq.Tools = runtime.Tools
	}

	// Select executor
	var runtimeRegistries []ToolHandlerRegistry
	if runtime != nil {
		runtimeRegistries = runtime.ToolRegistries
	}
	executor, err := gw.selectExecutor(agent, sessionID, session.UserID, runtimeRegistries)
	if err != nil {
		return nil, err
	}

	// Create cancellable context with a backend hard timeout. User/client
	// cancellation still propagates through the parent context.
	execCtx, cancel := context.WithTimeout(ctx, agentExecutionTimeout)
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
		var closeErr error
		defer func() {
			gw.mu.Lock()
			delete(gw.executions, sessionID)
			gw.mu.Unlock()
		}()
		defer func() {
			if closeErr != nil {
				_ = pw.CloseWithError(closeErr)
				return
			}
			if err := encoder.Close(); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()

		var assistantContent string
		finalized := false
		idleTimer := time.NewTimer(streamHeartbeatInterval)
		defer idleTimer.Stop()

		resetIdleTimer := func() {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(streamHeartbeatInterval)
		}

		persistPartialAssistant := func(tokens int) {
			if assistantContent != "" {
				_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleAssistant, assistantContent, nil, tokens)
				assistantContent = ""
			}
		}

		finalizeCancelled := func() {
			if finalized {
				return
			}
			finalized = true
			persistPartialAssistant(0)
			_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusCancelled)
		}
		finalizeError := func() {
			if finalized {
				return
			}
			finalized = true
			persistPartialAssistant(0)
			_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusError)
		}
		encodeExecutionStopped := func() error {
			if ctx.Err() != nil || errors.Is(execCtx.Err(), context.Canceled) {
				finalizeCancelled()
				return encoder.Encode(Event{Type: EventTypeCancelled, Message: execCtx.Err().Error()})
			}
			finalizeError()
			return encoder.Encode(Event{Type: EventTypeError, Message: fmt.Sprintf("agent execution timed out after %s", agentExecutionTimeout)})
		}

		for {
			select {
			case evt, ok := <-eventCh:
				if !ok {
					if execCtx.Err() != nil {
						if err := encodeExecutionStopped(); err != nil {
							closeErr = err
							return
						}
					}
					return
				}
				if err := encoder.Encode(evt); err != nil {
					closeErr = err
					return
				}
				resetIdleTimer()

				// Post-process: store results
				switch evt.Type {
				case EventTypeContentDelta:
					assistantContent += evt.Text

				case EventTypeToolCall:
					meta, _ := json.Marshal(map[string]any{
						"tool_call_id": evt.ToolCallID,
						"tool_name":    evt.ToolName,
						"tool_args":    evt.ToolArgs,
						"status":       "running",
					})
					_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleToolCall, "", meta, 0)

				case EventTypeToolResult:
					status := "completed"
					if evt.ToolIsError {
						status = "error"
					}
					meta, _ := json.Marshal(map[string]any{
						"tool_call_id": evt.ToolCallID,
						"duration_ms":  evt.DurationMs,
						"status":       status,
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

				case EventTypeUISurface:
					var payload any
					if len(evt.SurfaceData) > 0 {
						_ = json.Unmarshal(evt.SurfaceData, &payload)
					}
					meta, _ := json.Marshal(map[string]any{
						"ui_surface": map[string]any{
							"surfaceId":   evt.SurfaceID,
							"surfaceType": evt.SurfaceType,
							"payload":     payload,
						},
					})
					_, _ = gw.sessionSvc.StoreMessage(sessionID, MessageRoleAssistant, "", meta, 0)

				case EventTypeDone:
					finalized = true
					persistPartialAssistant(evt.OutputTokens)
					_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusCompleted)

				case EventTypeError:
					finalized = true
					persistPartialAssistant(0)
					_ = gw.sessionSvc.UpdateStatus(sessionID, SessionStatusError)

				case EventTypeCancelled:
					finalizeCancelled()
				}

			case <-execCtx.Done():
				if err := encodeExecutionStopped(); err != nil {
					closeErr = err
					return
				}
				return

			case <-idleTimer.C:
				if hb, ok := encoder.(heartbeatStreamEncoder); ok {
					if err := hb.Heartbeat(); err != nil {
						closeErr = err
						return
					}
				}
				idleTimer.Reset(streamHeartbeatInterval)
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

func (gw *AgentGateway) selectExecutor(agent *Agent, sessionID, userID uint, runtimeRegistries []ToolHandlerRegistry) (Executor, error) {
	switch agent.Type {
	case AgentTypeAssistant:
		client, err := gw.buildLLMClient(agent)
		if err != nil {
			return nil, fmt.Errorf("build LLM client: %w", err)
		}
		if len(runtimeRegistries) == 0 {
			runtimeRegistries = gw.toolRegistries
		}
		toolExec := NewCompositeToolExecutor(runtimeRegistries, sessionID, userID)
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

type storedToolCallMeta struct {
	ToolCallID string          `json:"tool_call_id"`
	ToolName   string          `json:"tool_name"`
	ToolArgs   json.RawMessage `json:"tool_args"`
}

type storedToolResultMeta struct {
	ToolCallID string `json:"tool_call_id"`
}

func buildExecuteMessagesFromSessionMessages(messages []SessionMessage) []ExecuteMessage {
	execMessages := make([]ExecuteMessage, 0, len(messages))
	for i := 0; i < len(messages); {
		m := messages[i]
		switch m.Role {
		case MessageRoleUser, MessageRoleAssistant:
			execMessages = append(execMessages, executeMessageFromChatMessage(m))
			i++
		case MessageRoleToolCall:
			next, transcript := buildToolTranscript(messages, i)
			execMessages = append(execMessages, transcript...)
			i = next
		default:
			i++
		}
	}
	return execMessages
}

func executeMessageFromChatMessage(m SessionMessage) ExecuteMessage {
	var images []string
	if len(m.Metadata) > 0 {
		var meta struct {
			Images []string `json:"images"`
		}
		_ = json.Unmarshal(m.Metadata, &meta)
		images = meta.Images
	}
	return ExecuteMessage{
		Role:    m.Role,
		Content: m.Content,
		Images:  images,
	}
}

func buildToolTranscript(messages []SessionMessage, start int) (int, []ExecuteMessage) {
	type callWithResult struct {
		call   llm.ToolCall
		output string
	}

	var calls []llm.ToolCall
	i := start
	for i < len(messages) && messages[i].Role == MessageRoleToolCall {
		if call, ok := parseStoredToolCall(messages[i]); ok {
			calls = append(calls, call)
		}
		i++
	}

	results := make(map[string]string)
	for i < len(messages) && messages[i].Role == MessageRoleToolResult {
		if id := parseStoredToolResultID(messages[i]); id != "" {
			results[id] = messages[i].Content
		}
		i++
	}

	completed := make([]callWithResult, 0, len(calls))
	for _, call := range calls {
		output, ok := results[call.ID]
		if !ok {
			continue
		}
		completed = append(completed, callWithResult{call: call, output: output})
	}
	if len(completed) == 0 {
		return i, nil
	}

	toolCalls := make([]llm.ToolCall, len(completed))
	transcript := make([]ExecuteMessage, 0, len(completed)+1)
	for idx, item := range completed {
		toolCalls[idx] = item.call
	}
	transcript = append(transcript, ExecuteMessage{
		Role:      MessageRoleAssistant,
		ToolCalls: toolCalls,
	})
	for _, item := range completed {
		transcript = append(transcript, ExecuteMessage{
			Role:       llm.RoleTool,
			Content:    item.output,
			ToolCallID: item.call.ID,
		})
	}
	return i, transcript
}

func parseStoredToolCall(m SessionMessage) (llm.ToolCall, bool) {
	var meta storedToolCallMeta
	if len(m.Metadata) == 0 || json.Unmarshal(m.Metadata, &meta) != nil {
		return llm.ToolCall{}, false
	}
	if meta.ToolCallID == "" || meta.ToolName == "" {
		return llm.ToolCall{}, false
	}
	args := "{}"
	if len(meta.ToolArgs) > 0 {
		args = string(meta.ToolArgs)
	}
	return llm.ToolCall{ID: meta.ToolCallID, Name: meta.ToolName, Arguments: args}, true
}

func parseStoredToolResultID(m SessionMessage) string {
	var meta storedToolResultMeta
	if len(m.Metadata) == 0 || json.Unmarshal(m.Metadata, &meta) != nil {
		return ""
	}
	return meta.ToolCallID
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
	assembly := &assistantRuntimeAssembly{ToolRegistries: append([]ToolHandlerRegistry(nil), gw.toolRegistries...)}
	if err := gw.addBuiltinRuntimeTools(agentID, assembly, map[string]struct{}{}); err != nil {
		return nil, err
	}
	return assembly.Tools, nil
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

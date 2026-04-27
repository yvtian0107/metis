package runtime

import (
	"context"
	"encoding/json"

	"metis/internal/llm"
)

// Executor is the unified interface for all agent execution strategies.
type Executor interface {
	Execute(ctx context.Context, req ExecuteRequest) (<-chan Event, error)
}

// ExecuteRequest contains the fully assembled context for execution.
type ExecuteRequest struct {
	SessionID    uint
	AgentConfig  AgentExecuteConfig
	Messages     []ExecuteMessage
	SystemPrompt string
	Tools        []ToolDefinition
	MaxTurns     int
}

// AgentExecuteConfig holds agent-specific config needed during execution.
type AgentExecuteConfig struct {
	Type          string
	Strategy      string
	ModelID       uint
	ModelName     string   // Actual model identifier (e.g., "claude-opus-4-20250514")
	Temperature   *float32 // nil means don't send temperature (for models that don't support it)
	MaxTokens     int
	Runtime       string
	RuntimeConfig json.RawMessage
	ExecMode      string
	NodeID        *uint
	Workspace     string
	Instructions  string
}

// ExecuteMessage represents a message in the conversation history.
type ExecuteMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	Images     []string       `json:"images,omitempty"` // base64 encoded image URLs (data:image/...)
	ToolCalls  []llm.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

// ToolDefinition describes a tool available to the agent.
type ToolDefinition struct {
	Type        string          `json:"type"` // "builtin", "mcp", "skill"
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	SourceID    uint            `json:"sourceId"` // Tool/MCPServer/Skill ID
}

// ToolExecutor dispatches tool calls to the appropriate handler.
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, call ToolCall) (ToolResult, error)
}

// ToolCall represents a tool invocation from the LLM.
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	ID         string `json:"id"`
	Output     string `json:"output"`
	IsError    bool   `json:"isError"`
	DurationMs int    `json:"durationMs"`
}

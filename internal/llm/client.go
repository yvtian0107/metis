package llm

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotSupported is returned when an operation is not supported by the protocol.
var ErrNotSupported = errors.New("llm: operation not supported by this provider")

// Protocols
const (
	ProtocolOpenAI    = "openai"
	ProtocolAnthropic = "anthropic"
)

// Message roles
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// MessageContentPart represents a single part of a message content (text or image).
type MessageContentPart struct {
	Type     string `json:"type"` // "text" or "image_url"
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"` // base64 data URL or http URL
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"` // For simple text messages
	Images     []string   `json:"images,omitempty"` // base64 data URLs or http URLs for multimodal
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool/function call from the model.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef defines a tool available to the model.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ResponseFormat controls structured output from the model.
type ResponseFormat struct {
	Type   string `json:"type"`             // "json_object" or "json_schema"
	Schema any    `json:"schema,omitempty"` // JSON Schema object, only for "json_schema"
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Tools          []ToolDef       `json:"tools,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float32        `json:"temperature,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ChatResponse represents a non-streaming chat completion response.
type ChatResponse struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Usage      Usage      `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent is emitted during streaming chat completion.
type StreamEvent struct {
	// Type: "content_delta", "tool_call", "done", "error"
	Type      string    `json:"type"`
	Content   string    `json:"content,omitempty"`
	ToolCall  *ToolCall `json:"tool_call,omitempty"`
	Usage     *Usage    `json:"usage,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// EmbeddingRequest represents an embedding request.
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse represents an embedding response.
type EmbeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Usage      struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Client is the unified LLM client interface.
type Client interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	Embedding(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}

// NewClient creates a Client for the given protocol.
func NewClient(protocol, baseURL, apiKey string) (Client, error) {
	switch protocol {
	case ProtocolOpenAI:
		return newOpenAIClient(baseURL, apiKey), nil
	case ProtocolAnthropic:
		return newAnthropicClient(baseURL, apiKey), nil
	default:
		return nil, fmt.Errorf("llm: unsupported protocol %q", protocol)
	}
}

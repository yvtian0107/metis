# Capability: ai-llm-client

## Purpose
Provides a unified LLM client interface that abstracts over multiple provider protocols (OpenAI, Anthropic), supporting chat completion, streaming, and embedding operations.

## Requirements

### Requirement: Unified LLM client interface
The system SHALL provide a unified LLM client at `internal/llm/` that abstracts over multiple provider protocols. The client interface SHALL support Chat, ChatStream, and Embedding operations.

#### Scenario: Create client from provider config
- **WHEN** a caller provides protocol, base_url, and api_key
- **THEN** `llm.NewClient(protocol, baseURL, apiKey)` returns the appropriate protocol implementation

#### Scenario: OpenAI protocol client
- **WHEN** client is created with protocol `openai`
- **THEN** it uses `sashabaranov/go-openai` library for all API calls

#### Scenario: Anthropic protocol client
- **WHEN** client is created with protocol `anthropic`
- **THEN** it uses `anthropics/anthropic-sdk-go` library for all API calls

### Requirement: Chat completion
The system SHALL support non-streaming chat completion via the unified client.

#### Scenario: Send chat request
- **WHEN** caller invokes `Chat(ctx, ChatRequest{Model, Messages, Tools, MaxTokens, Temperature})`
- **THEN** the client sends the request to the provider and returns `ChatResponse{Content, ToolCalls, Usage{InputTokens, OutputTokens}}`

#### Scenario: Chat with tool calls
- **WHEN** the LLM response includes tool calls
- **THEN** `ChatResponse.ToolCalls` contains the parsed tool call list with ID, name, and arguments

### Requirement: Streaming chat completion
The system SHALL support streaming chat completion that returns events via a Go channel. The `StreamEvent` schema SHALL remain stable so that the Agent Gateway can translate each event into Vercel AI SDK Data Stream lines without information loss. Event types from `ChatStream` SHALL include: `content_delta` (text fragment), `tool_call` (complete tool call object), `done` (with usage stats), and `error`.

#### Scenario: Stream chat request
- **WHEN** caller invokes `ChatStream(ctx, ChatRequest)`
- **THEN** the client returns a `<-chan StreamEvent` that emits content deltas, complete tool calls, and a final done event with usage stats

#### Scenario: Cancel streaming
- **WHEN** caller cancels the context during streaming
- **THEN** the stream channel is closed and the underlying HTTP connection is terminated

#### Scenario: Gateway translation compatibility
- **WHEN** `ChatStream` emits a `tool_call` event
- **THEN** the event SHALL include a valid `ToolCall` struct with `ID`, `Name`, and `Arguments` (JSON string) so that the Gateway can encode it as a Data Stream tool-invocation chunk

### Requirement: Embedding
The system SHALL support text embedding via the unified client.

#### Scenario: Generate embeddings
- **WHEN** caller invokes `Embedding(ctx, EmbeddingRequest{Model, Input[]string})`
- **THEN** the client returns `EmbeddingResponse{Embeddings[][]float32, Usage{TotalTokens}}`

#### Scenario: Embedding not supported by Anthropic
- **WHEN** caller invokes Embedding on an Anthropic protocol client
- **THEN** the client returns an `ErrNotSupported` error

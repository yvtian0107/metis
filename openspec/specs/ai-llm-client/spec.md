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

### Requirement: LLM 输出 JSON 提取与修复公共函数
系统 SHALL 在 `internal/llm/json.go` 提供公共函数 `ExtractJSON(content string) string`，统一处理 LLM 输出中的 JSON 提取与修复。该函数 SHALL 依次执行：剥离 markdown code fence（```json ... ```）、TrimSpace、`json.Valid()` 快速校验、`jsonrepair.Repair()` 修复常见 LLM 输出问题（trailing comma、single quotes、截断 JSON、缺失闭合括号、注释）。

#### Scenario: 提取 markdown 包裹的 JSON
- **WHEN** 输入为 `"分析如下：\n```json\n{\"key\": \"value\"}\n```\n补充说明"`
- **THEN** ExtractJSON SHALL 返回 `{"key": "value"}`

#### Scenario: 修复 trailing comma
- **WHEN** 输入为 `{"items": ["a", "b",], "count": 2,}`
- **THEN** ExtractJSON SHALL 返回合法 JSON `{"items": ["a", "b"], "count": 2}`

#### Scenario: 修复截断 JSON
- **WHEN** 输入为 `{"key": "value", "nested": {"a": 1`（LLM 输出被截断）
- **THEN** ExtractJSON SHALL 尽力修复并返回闭合的 JSON

#### Scenario: 合法 JSON 直接返回
- **WHEN** 输入为合法 JSON 字符串
- **THEN** ExtractJSON SHALL 跳过 repair 直接返回

### Requirement: ChatRequest 支持 ResponseFormat
`llm.ChatRequest` SHALL 新增 `ResponseFormat *ResponseFormat` 可选字段。`ResponseFormat` 结构包含 `Type string`（"json_object" 或 "json_schema"）和 `Schema any`（JSON Schema 对象，仅 json_schema 时使用）。

#### Scenario: OpenAI 驱动翻译 json_schema
- **WHEN** ChatRequest 携带 `ResponseFormat{Type: "json_schema", Schema: <schema>}`
- **AND** 使用 OpenAI 协议客户端
- **THEN** 客户端 SHALL 在 API 请求中设置 `response_format: {type: "json_schema", json_schema: {name: "response", schema: <schema>}}`

#### Scenario: OpenAI 驱动翻译 json_object
- **WHEN** ChatRequest 携带 `ResponseFormat{Type: "json_object"}`
- **AND** 使用 OpenAI 协议客户端
- **THEN** 客户端 SHALL 在 API 请求中设置 `response_format: {type: "json_object"}`

#### Scenario: Anthropic 驱动处理 json_object
- **WHEN** ChatRequest 携带 `ResponseFormat{Type: "json_object"}`
- **AND** 使用 Anthropic 协议客户端
- **THEN** 客户端 SHALL 在 system prompt 末尾追加 JSON 约束提示词
- **AND** SHALL 在 messages 末尾追加 assistant prefill `{"` 引导 JSON 输出

#### Scenario: ResponseFormat 为 nil 时行为不变
- **WHEN** ChatRequest 的 ResponseFormat 为 nil
- **THEN** 客户端行为 SHALL 与当前完全一致，不做任何额外处理

### Requirement: Chat completion
The system SHALL support non-streaming chat completion via the unified client.

#### Scenario: Send chat request
- **WHEN** caller invokes `Chat(ctx, ChatRequest{Model, Messages, Tools, MaxTokens, Temperature})`
- **THEN** the client sends the request to the provider and returns `ChatResponse{Content, ToolCalls, Usage{InputTokens, OutputTokens}}`

#### Scenario: Chat with tool calls
- **WHEN** the LLM response includes tool calls
- **THEN** `ChatResponse.ToolCalls` contains the parsed tool call list with ID, name, and arguments

#### Scenario: Chat with ResponseFormat
- **WHEN** caller invokes `Chat(ctx, ChatRequest{..., ResponseFormat: &ResponseFormat{Type: "json_object"}})`
- **AND** LLM returns text content (no tool calls)
- **THEN** `ChatResponse.Content` SHALL contain valid JSON

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

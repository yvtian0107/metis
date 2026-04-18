## ADDED Requirements

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

## MODIFIED Requirements

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

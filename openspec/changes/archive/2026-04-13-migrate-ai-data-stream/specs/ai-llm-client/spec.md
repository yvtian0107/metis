## MODIFIED Requirements

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

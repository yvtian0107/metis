## ADDED Requirements

### Requirement: Data Stream line format
The system SHALL encode all SSE events for agent streaming as Vercel AI SDK Data Stream lines. Each line SHALL be a Server-Sent Event `data:` field whose value is a Data Stream chunk. A chunk starts with a type prefix (`0:` for text delta, `1:` for reasoning, `2:` for data, `8:` for error, `d:` for finish message, `e:` for custom event), followed by a JSON payload, and ends with a newline.

#### Scenario: Text delta encoding
- **WHEN** the backend emits a `content_delta` with text `"hello"`
- **THEN** the SSE line SHALL be `data: 0:"hello"\n\n`

#### Scenario: Tool call start encoding
- **WHEN** the backend emits a `tool_call` event with name `"search_knowledge"` and arguments `{"query":"x"}`
- **THEN** the SSE line SHALL be a Data Stream chunk that represents a tool call invocation start with a JSON payload containing `toolCallId`, `toolName`, and `args`

#### Scenario: Finish message encoding
- **WHEN** an execution completes successfully
- **THEN** the backend SHALL emit a finish message chunk (`d:`) containing `finishReason:"stop"`, `usage` with `promptTokens` and `completionTokens`, and any optional `totalTurns`

### Requirement: SSE flush behavior
The streaming endpoint SHALL flush each Data Stream line to the network immediately after writing it to the response body. The response SHALL set headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`.

#### Scenario: Immediate flush per event
- **WHEN** the gateway writes a Data Stream line to the HTTP response writer
- **THEN** it SHALL call `http.Flusher.Flush()` before proceeding to the next event

#### Scenario: Connection headers
- **WHEN** a client opens `GET /api/v1/ai/sessions/:sid/stream`
- **THEN** the response headers SHALL include `Content-Type: text/event-stream` and `X-Accel-Buffering: no`

### Requirement: Protocol translation layer
The system SHALL provide a translation layer in the Agent Gateway that converts internal `Event` structures to Data Stream lines without changing the internal executor event schema.

#### Scenario: Gateway translation
- **WHEN** an executor emits an internal `Event{Type:"content_delta", Text:"..."}`
- **THEN** the gateway SHALL translate it to a Data Stream text-delta chunk before writing to the SSE stream

#### Scenario: Sidecar events remain internal
- **WHEN** a remote sidecar sends custom NDJSON events to `POST /api/v1/ai/sessions/:sid/events`
- **THEN** the gateway SHALL first parse them into internal `Event` structures, then apply the same Data Stream translation before forwarding to the browser

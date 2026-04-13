## MODIFIED Requirements

### Requirement: SSE streaming
The system SHALL provide `GET /api/v1/ai/sessions/:sid/stream` as an SSE endpoint. The response body SHALL consist of Vercel AI SDK Data Stream lines (Server-Sent Events with `data:` fields). Each line SHALL represent a standard AI SDK stream chunk: text-delta, reasoning, tool-invocation, error, or finish-message. The endpoint SHALL set `Cache-Control: no-cache` and flush each line immediately.

#### Scenario: SSE connection
- **WHEN** user opens SSE connection to `/api/v1/ai/sessions/:sid/stream`
- **THEN** system SHALL stream Data Stream lines from the current or next execution in real-time with immediate flush

#### Scenario: SSE reconnection
- **WHEN** SSE connection drops and client reconnects with Last-Event-ID header
- **THEN** system MAY ignore the header and start a new stream (reconnection replay is not required in this version)

#### Scenario: No active execution
- **WHEN** user opens SSE and no execution is running
- **THEN** system SHALL keep the connection open and wait for the next execution, then begin streaming Data Stream lines

### Requirement: Send message and trigger execution
`POST /api/v1/ai/sessions/:sid/messages` SHALL accept user message content, store it, and trigger agent execution. The response SHALL be the stored message. The actual agent response is delivered via the Data Stream SSE endpoint.

#### Scenario: Send message to running session
- **WHEN** user sends a message while a previous execution is still running
- **THEN** system SHALL return 409 conflict

#### Scenario: Send message
- **WHEN** user sends `POST /api/v1/ai/sessions/:sid/messages` with content
- **THEN** system SHALL store the message, trigger Gateway execution, and return 202 with the message object

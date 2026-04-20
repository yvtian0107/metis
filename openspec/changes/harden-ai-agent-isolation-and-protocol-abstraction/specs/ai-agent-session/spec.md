## MODIFIED Requirements

### Requirement: Session CRUD API
The system SHALL provide REST endpoints under `/api/v1/ai/sessions` with JWT auth:
- `POST /` — create session (body: agent_id)
- `GET /` — list user's sessions (with pagination, agent_id filter)
- `GET /:sid` — get session with full message history
- `DELETE /:sid` — soft-delete session (only owner or admin)

All session detail and mutation routes SHALL enforce session ownership before loading history, changing state, or deleting records. Requests for a session not owned by the current user SHALL behave the same as a missing session.

#### Scenario: List sessions filtered by agent
- **WHEN** user requests `GET /api/v1/ai/sessions?agent_id=xxx`
- **THEN** system SHALL return only sessions for that agent belonging to the current user

#### Scenario: Get session messages
- **WHEN** user requests `GET /api/v1/ai/sessions/:sid`
- **THEN** system SHALL return session metadata and all messages ordered by sequence

#### Scenario: Cross-user session detail behaves as not found
- **WHEN** a user requests `GET /api/v1/ai/sessions/:sid` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

#### Scenario: Cross-user session delete behaves as not found
- **WHEN** a user requests `DELETE /api/v1/ai/sessions/:sid` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

### Requirement: Send message and trigger execution
`POST /api/v1/ai/sessions/:sid/messages` SHALL accept user message content, store it, and trigger agent execution. The response SHALL be the stored message. The actual agent response is delivered via the Data Stream SSE endpoint.

The session used for message sending SHALL belong to the current user before the message is stored or execution is triggered.

#### Scenario: Send message to running session
- **WHEN** user sends a message while a previous execution is still running
- **THEN** system SHALL return 409 conflict

#### Scenario: Send message
- **WHEN** user sends `POST /api/v1/ai/sessions/:sid/messages` with content
- **THEN** system SHALL store the message, trigger Gateway execution, and return 202 with the message object

#### Scenario: Cross-user send message behaves as not found
- **WHEN** a user sends `POST /api/v1/ai/sessions/:sid/messages` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

### Requirement: SSE streaming
The system SHALL provide `GET /api/v1/ai/sessions/:sid/stream` as an SSE endpoint. The response body SHALL consist of Vercel AI SDK Data Stream lines (Server-Sent Events with `data:` fields). Each line SHALL represent a standard AI SDK stream chunk: text-delta, reasoning, tool-invocation, error, or finish-message. The endpoint SHALL set `Cache-Control: no-cache` and flush each line immediately.

The stream endpoint SHALL verify that the session belongs to the current user before opening or resuming execution.

#### Scenario: SSE connection
- **WHEN** user opens SSE connection to `/api/v1/ai/sessions/:sid/stream`
- **THEN** system SHALL stream Data Stream lines from the current or next execution in real-time with immediate flush

#### Scenario: SSE reconnection
- **WHEN** SSE connection drops and client reconnects with Last-Event-ID header
- **THEN** system MAY ignore the header and start a new stream (reconnection replay is not required in this version)

#### Scenario: No active execution
- **WHEN** user opens SSE and no execution is running
- **THEN** system SHALL keep the connection open and wait for the next execution, then begin streaming Data Stream lines

#### Scenario: Cross-user stream behaves as not found
- **WHEN** a user opens `/api/v1/ai/sessions/:sid/stream` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

## ADDED Requirements

### Requirement: Session ownership applies to all mutation endpoints
All session mutation endpoints, including update, message edit, cancel, continue, and image upload, SHALL require that the target session belongs to the current user before any state change or side effect occurs.

#### Scenario: Cross-user session update behaves as not found
- **WHEN** a user requests `PUT /api/v1/ai/sessions/:sid` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

#### Scenario: Cross-user message edit behaves as not found
- **WHEN** a user requests `PUT /api/v1/ai/sessions/:sid/messages/:mid` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

#### Scenario: Cross-user cancel behaves as not found
- **WHEN** a user requests `POST /api/v1/ai/sessions/:sid/cancel` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

#### Scenario: Cross-user continue behaves as not found
- **WHEN** a user requests `POST /api/v1/ai/sessions/:sid/continue` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

#### Scenario: Cross-user image upload behaves as not found
- **WHEN** a user requests `POST /api/v1/ai/sessions/:sid/images` for a session owned by another user
- **THEN** system SHALL return the same not-found result used for a missing session

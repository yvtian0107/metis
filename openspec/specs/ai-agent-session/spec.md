# ai-agent-session Specification

## Purpose
TBD - created by archiving change ai-agent-runtime. Update Purpose after archive.
## Requirements
### Requirement: Session lifecycle
The system SHALL support Agent Sessions with states: `running`, `completed`, `cancelled`, `error`. A session is created when a user starts a conversation with an Agent. Sessions belong to a (agent_id, user_id) pair.

#### Scenario: Create session
- **WHEN** user sends the first message to an Agent
- **THEN** system SHALL create a new session with status `running` and store the user message

#### Scenario: Session completion
- **WHEN** the last executor response completes without error
- **THEN** system SHALL set session status to `completed`

#### Scenario: Explicit session creation
- **WHEN** user calls `POST /api/v1/ai/sessions` with agent_id
- **THEN** system SHALL create an empty session ready for messages

### Requirement: Message storage
The system SHALL persist all messages within a session. Each message SHALL have: session_id (FK), role (`user` | `assistant` | `tool_call` | `tool_result`), content (text), metadata (JSON), token_count (int), sequence (int, auto-increment within session), created_at.

#### Scenario: Store user message
- **WHEN** user sends a message via `POST /api/v1/ai/sessions/:sid/messages`
- **THEN** system SHALL store the message with role `user` and the next sequence number

#### Scenario: Store assistant response
- **WHEN** executor emits `content_delta` events followed by `done`
- **THEN** system SHALL concatenate content deltas and store as a single message with role `assistant`

#### Scenario: Store tool interactions
- **WHEN** executor emits `tool_call` and `tool_result` events
- **THEN** system SHALL store each as separate messages with roles `tool_call` and `tool_result`, with metadata containing tool_name, tool_args, output, and duration_ms

### Requirement: Session CRUD API
The system SHALL provide REST endpoints under `/api/v1/ai/sessions` with JWT auth:
- `POST /` — create session (body: agent_id)
- `GET /` — list user's sessions (with pagination, agent_id filter)
- `GET /:sid` — get session with full message history
- `DELETE /:sid` — soft-delete session (only owner or admin)

#### Scenario: List sessions filtered by agent
- **WHEN** user requests `GET /api/v1/ai/sessions?agent_id=xxx`
- **THEN** system SHALL return only sessions for that agent belonging to the current user

#### Scenario: Get session messages
- **WHEN** user requests `GET /api/v1/ai/sessions/:sid`
- **THEN** system SHALL return session metadata and all messages ordered by sequence

### Requirement: Send message and trigger execution
`POST /api/v1/ai/sessions/:sid/messages` SHALL accept user message content, store it, and trigger agent execution. The response SHALL be the stored message. The actual agent response is delivered via the Data Stream SSE endpoint.

#### Scenario: Send message to running session
- **WHEN** user sends a message while a previous execution is still running
- **THEN** system SHALL return 409 conflict

#### Scenario: Send message
- **WHEN** user sends `POST /api/v1/ai/sessions/:sid/messages` with content
- **THEN** system SHALL store the message, trigger Gateway execution, and return 202 with the message object

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

### Requirement: Cancel execution
`POST /api/v1/ai/sessions/:sid/cancel` SHALL interrupt any running execution for the session.

#### Scenario: Cancel running execution
- **WHEN** user cancels while executor is running
- **THEN** system SHALL signal the executor to stop, emit a `cancelled` event, store partial results, and set session status to `cancelled`

#### Scenario: Cancel when not running
- **WHEN** user cancels but no execution is in progress
- **THEN** system SHALL return 200 (idempotent, no-op)

### Requirement: Context window management
Before dispatching to an executor, the Gateway SHALL assemble the full context with token budget allocation: (1) system prompt + instructions + memory, (2) tool definitions, (3) knowledge context, (4) message history (oldest truncated first). Total tokens SHALL NOT exceed the model's context_window.

#### Scenario: History truncation
- **WHEN** message history exceeds the remaining token budget after allocating system/tools/knowledge
- **THEN** system SHALL truncate from the oldest messages, preserving the most recent messages

#### Scenario: Empty history
- **WHEN** session has no prior messages (first message)
- **THEN** system SHALL send only the system prompt + instructions + memory + user message


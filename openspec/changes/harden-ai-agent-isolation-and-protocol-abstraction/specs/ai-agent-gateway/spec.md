## MODIFIED Requirements

### Requirement: Gateway request orchestration
The Agent Gateway SHALL be the single entry point for all agent execution. Upon receiving a user message, the Gateway SHALL: (1) validate the session and agent, (2) store the user message, (3) load message history with truncation, (4) load user memories for this agent, (5) query bound knowledge bases for relevant context, (6) assemble the full ExecuteRequest, (7) dispatch to the appropriate Executor, (8) consume the event stream, (9) store results to DB, (10) encode events into the configured stream protocol, (11) forward lines to browser via flushed SSE.

The Gateway SHALL resolve the session through an ownership-aware access path before reading history, loading memories, dispatching execution, or applying cancellation.

#### Scenario: Full orchestration flow
- **WHEN** user sends a message to a session
- **THEN** Gateway SHALL execute all steps in order and stream Data Stream lines to the browser in real-time

#### Scenario: Agent not found or inactive
- **WHEN** session references an agent that is deleted or inactive
- **THEN** Gateway SHALL return 404 and not attempt execution

#### Scenario: Cross-user session execution behaves as not found
- **WHEN** Gateway is asked to run a session owned by another user
- **THEN** it SHALL return the same not-found result used for a missing session and SHALL NOT load messages, memories, or start an executor

### Requirement: Unified event protocol
All executors SHALL produce events conforming to the unified event schema. Event types: `llm_start`, `content_delta`, `tool_call`, `tool_result`, `plan`, `step_start`, `done`, `cancelled`, `error`. Each event SHALL carry a monotonically increasing sequence number within the execution. The Gateway SHALL translate these internal events into the configured external stream protocol before sending them to the client via SSE.

The default configured external stream protocol for the current chat endpoint SHALL remain the Vercel AI SDK Data Stream / UI Message Stream format used by the existing frontend.

#### Scenario: Event ordering
- **WHEN** executor produces events
- **THEN** each event SHALL have a sequence number greater than the previous event

#### Scenario: Done event completeness
- **WHEN** execution completes normally
- **THEN** the `done` event SHALL include total_turns, input_tokens, and output_tokens, and the configured encoder SHALL emit the matching terminal stream chunk

#### Scenario: Default chat stream remains Vercel compatible
- **WHEN** the standard chat SSE endpoint encodes Gateway events
- **THEN** the response SHALL remain compatible with the existing Vercel UI stream consumer used by the frontend

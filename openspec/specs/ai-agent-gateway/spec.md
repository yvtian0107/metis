# ai-agent-gateway Specification

## Purpose
TBD - created by archiving change ai-agent-runtime. Update Purpose after archive.
## Requirements
### Requirement: Gateway request orchestration
The Agent Gateway SHALL be the single entry point for all agent execution. Upon receiving a user message, the Gateway SHALL: (1) validate the session and agent, (2) store the user message, (3) load message history with truncation, (4) load user memories for this agent, (5) query bound knowledge bases for relevant context, (6) assemble the full ExecuteRequest, (7) dispatch to the appropriate Executor, (8) consume the event stream, (9) store results to DB, (10) translate events to Data Stream lines, (11) forward lines to browser via flushed SSE.

#### Scenario: Full orchestration flow
- **WHEN** user sends a message to a session
- **THEN** Gateway SHALL execute all steps in order and stream Data Stream lines to the browser in real-time

#### Scenario: Agent not found or inactive
- **WHEN** session references an agent that is deleted or inactive
- **THEN** Gateway SHALL return 404 and not attempt execution

### Requirement: Executor dispatch by agent type
The Gateway SHALL select the Executor based on agent configuration:
- `type=assistant, strategy=react` → ReactExecutor
- `type=assistant, strategy=plan_and_execute` → PlanAndExecuteExecutor
- `type=coding, exec_mode=local` → LocalCodingExecutor
- `type=coding, exec_mode=remote` → RemoteCodingExecutor

All executors SHALL implement the same interface: `Execute(ctx, req) (<-chan Event, error)`.

#### Scenario: Assistant with react strategy
- **WHEN** Gateway dispatches for an assistant agent with strategy `react`
- **THEN** ReactExecutor SHALL be used

#### Scenario: Coding agent with local mode
- **WHEN** Gateway dispatches for a coding agent with exec_mode `local`
- **THEN** LocalCodingExecutor SHALL be used

### Requirement: ReactExecutor
The ReactExecutor SHALL implement the ReAct loop: (1) call LLM via ai-llm-client ChatStream, (2) if response contains tool calls, execute each tool (builtin Tool / MCP / Skill), emit tool_call + tool_result events, add results to messages, loop back to step 1, (3) if response is text-only, emit content_delta events, emit done event, exit loop. The loop SHALL NOT exceed the agent's max_turns setting. All emitted events SHALL be translatable by the Gateway into Data Stream chunks.

#### Scenario: Simple text response
- **WHEN** LLM returns text without tool calls
- **THEN** executor SHALL emit content_delta events and a done event with token counts, and Gateway SHALL translate them to Data Stream text-delta and finish-message chunks

#### Scenario: Tool call loop
- **WHEN** LLM returns a tool call for "search_knowledge"
- **THEN** executor SHALL emit tool_call event, execute the tool, emit tool_result event, then call LLM again with the result, and Gateway SHALL translate both to Data Stream tool-invocation chunks

#### Scenario: Max turns exceeded
- **WHEN** LLM loop reaches max_turns without completing
- **THEN** executor SHALL emit an error event with message "max turns exceeded" and stop

### Requirement: PlanAndExecuteExecutor
The PlanAndExecuteExecutor SHALL implement two-phase execution: (1) Plan phase: call LLM with a planning system prompt, parse response into numbered steps, emit a `plan` event, (2) Execute phase: for each step, call LLM with step context + tool access, execute tool calls as needed, emit `step_start` + content/tool events per step, (3) emit done event after all steps complete. Plan and step events SHALL be encoded as custom Data Stream annotations or forwarded through a side channel, and text/tool events within steps SHALL follow standard Data Stream translation.

#### Scenario: Plan generation
- **WHEN** executor starts
- **THEN** it SHALL first call LLM to generate a plan and emit a `plan` event with the step list

#### Scenario: Step execution with tools
- **WHEN** executing a step that requires tool usage
- **THEN** executor SHALL emit `step_start`, then execute the ReAct loop within that step's scope

### Requirement: LocalCodingExecutor
The LocalCodingExecutor SHALL spawn the configured coding tool (claude-code / codex / opencode / aider) as a subprocess on the Server machine. It SHALL: (1) set working directory to the agent's workspace path, (2) inject instructions via the tool's native mechanism, (3) pipe the user's message as input, (4) parse stdout/stderr into unified Event format, (5) manage subprocess lifecycle (start, monitor, kill on cancel).

#### Scenario: Spawn claude-code
- **WHEN** agent runtime is `claude-code`
- **THEN** executor SHALL spawn `claude` CLI with appropriate flags, workspace as cwd, and stream output as events

#### Scenario: Subprocess crash
- **WHEN** the subprocess exits with non-zero code
- **THEN** executor SHALL emit an error event with the stderr content

#### Scenario: Cancel local execution
- **WHEN** user cancels and subprocess is running
- **THEN** executor SHALL send SIGTERM to the subprocess, wait briefly, then SIGKILL if needed

### Requirement: RemoteCodingExecutor
The RemoteCodingExecutor SHALL delegate execution to a remote Node via the existing Sidecar SSE infrastructure. It SHALL: (1) send a `run` command to the node via NodeHub SSE with the full execution payload, (2) receive events from the sidecar via `POST /api/v1/ai/sessions/:sid/events` (Node Token auth), (3) forward events to the Gateway event channel.

#### Scenario: Dispatch to remote node
- **WHEN** agent exec_mode is `remote` and node is online
- **THEN** executor SHALL send a run command via SSE and listen for events from the sidecar

#### Scenario: Remote node offline
- **WHEN** the bound node is offline
- **THEN** executor SHALL emit an error event with message "node offline" without queueing

#### Scenario: Cancel remote execution
- **WHEN** user cancels a remote execution
- **THEN** executor SHALL send a `cancel` command via NodeHub SSE to the node

### Requirement: Unified event protocol
All executors SHALL produce events conforming to the unified event schema. Event types: `llm_start`, `content_delta`, `tool_call`, `tool_result`, `plan`, `step_start`, `done`, `cancelled`, `error`. Each event SHALL carry a monotonically increasing sequence number within the execution. The Gateway SHALL translate these internal events into Vercel AI SDK Data Stream lines before sending them to the client via SSE.

#### Scenario: Event ordering
- **WHEN** executor produces events
- **THEN** each event SHALL have a sequence number greater than the previous event

#### Scenario: Done event completeness
- **WHEN** execution completes normally
- **THEN** the `done` event SHALL include total_turns, input_tokens, and output_tokens, and the Gateway SHALL encode it as a Data Stream finish message (`d:`)

#### Scenario: Data Stream translation
- **WHEN** the Gateway forwards an event to the browser
- **THEN** the SSE payload SHALL be a valid Data Stream line and NOT a raw JSON object inside `data:`

### Requirement: Tool execution
The Gateway SHALL support executing three types of tools during agent runs:
- **Builtin Tool**: invoke the tool's handler function directly within the Server process
- **MCP Server**: send tool call to the registered MCP server (SSE or STDIO transport) and await result
- **Skill**: if endpoint skill, call the skill's endpoint; if prompt-only skill, inject into next LLM call

#### Scenario: Execute builtin tool
- **WHEN** LLM emits a tool_call for a registered builtin tool
- **THEN** system SHALL execute the tool handler and return the result

#### Scenario: Execute MCP tool
- **WHEN** LLM emits a tool_call matching a tool from a bound MCP server
- **THEN** system SHALL forward the call to the MCP server and return the result

#### Scenario: Unknown tool call
- **WHEN** LLM emits a tool_call for an unregistered tool name
- **THEN** system SHALL return a tool_result with error "tool not found" and let the LLM retry

### Requirement: Sidecar event ingestion endpoint
The system SHALL provide `POST /api/v1/ai/sessions/:sid/events` with Node Token auth for sidecar to upload NDJSON event streams from remote coding executions.

#### Scenario: Sidecar uploads events
- **WHEN** sidecar POSTs NDJSON event stream for a session
- **THEN** system SHALL parse events and feed them into the Gateway's event channel for that session

#### Scenario: Invalid session
- **WHEN** sidecar POSTs events for a non-existent or non-running session
- **THEN** system SHALL return 404


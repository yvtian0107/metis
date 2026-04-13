## MODIFIED Requirements

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

### Requirement: Gateway request orchestration
The Agent Gateway SHALL be the single entry point for all agent execution. Upon receiving a user message, the Gateway SHALL: (1) validate the session and agent, (2) store the user message, (3) load message history with truncation, (4) load user memories for this agent, (5) query bound knowledge bases for relevant context, (6) assemble the full ExecuteRequest, (7) dispatch to the appropriate Executor, (8) consume the event stream, (9) store results to DB, (10) translate events to Data Stream lines, (11) forward lines to browser via flushed SSE.

#### Scenario: Full orchestration flow
- **WHEN** user sends a message to a session
- **THEN** Gateway SHALL execute all steps in order and stream Data Stream lines to the browser in real-time

#### Scenario: Agent not found or inactive
- **WHEN** session references an agent that is deleted or inactive
- **THEN** Gateway SHALL return 404 and not attempt execution

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

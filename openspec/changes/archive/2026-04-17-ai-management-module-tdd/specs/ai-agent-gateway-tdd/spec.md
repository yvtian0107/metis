## ADDED Requirements

### Requirement: Agent gateway and executor test infrastructure
The system SHALL provide reusable mocks for `llm.Client` and `ToolExecutor` to enable deterministic unit testing of `ReactExecutor`, `PlanAndExecuteExecutor`, and `AgentGateway` without network calls.

#### Scenario: Mock LLM client available
- **WHEN** a gateway test needs a deterministic LLM response
- **THEN** a mock implementation of `llm.Client` can be programmed with a sequence of `ChatEvent`s

#### Scenario: Mock tool executor available
- **WHEN** an executor test needs a deterministic tool result
- **THEN** a mock implementation of `ToolExecutor` can return pre-configured outputs per tool name

### Requirement: Test ReAct executor loop
The unit test suite SHALL verify the core ReAct execution flow: content generation, tool calls, result integration, and turn limits.

#### Scenario: Direct content without tool calls
- **WHEN** the mock LLM returns a single content delta and then done
- **THEN** `ReactExecutor.Execute` emits `LLMStart`, `ContentDelta`, and `Done` events

#### Scenario: Single tool call round-trip
- **WHEN** the mock LLM returns a tool call in turn 1, and a final content delta in turn 2 after receiving the tool result
- **THEN** the executor emits `ToolCall`, `ToolResult`, and final `Done` events, and the total turns are 2

#### Scenario: Max turns exceeded
- **WHEN** the mock LLM returns a tool call on every turn and maxTurns=2
- **THEN** after turn 2 the executor emits an `Error` event with "max turns (2) exceeded"

#### Scenario: Cancellation during execution
- **WHEN** the parent context is cancelled mid-stream
- **THEN** the executor emits a `Cancelled` event and stops

### Requirement: Test AgentGateway orchestration
The unit test suite SHALL verify that `AgentGateway.Run` assembles the execution context correctly and persists outcomes.

#### Scenario: Build tool definitions filters inactive bindings
- **WHEN** an agent is bound to an active tool and an inactive tool
- **THEN** `buildToolDefinitions` returns only the active tool

#### Scenario: Build tool definitions includes MCP servers
- **WHEN** an agent is bound to an active MCP server
- **THEN** `buildToolDefinitions` includes a definition of type="mcp" with the server name

#### Scenario: System prompt assembly
- **WHEN** `Run` is called for an agent with `SystemPrompt`, `Instructions`, and memories
- **THEN** the assembled system prompt concatenates all three parts in order

#### Scenario: Session status transitions to completed
- **WHEN** a gateway run finishes with a `Done` event
- **THEN** the session status is updated to "completed" and an assistant message is stored

#### Scenario: Session status transitions to error
- **WHEN** a gateway run encounters an `Error` event
- **THEN** the session status is updated to "error" and any partial assistant content is stored

#### Scenario: Session status transitions to cancelled
- **WHEN** a gateway run is cancelled via `Cancel`
- **THEN** the session status is updated to "cancelled" and any partial assistant content is stored

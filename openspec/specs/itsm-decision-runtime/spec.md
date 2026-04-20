## Purpose

ITSM decision runtime -- shared decision execution abstraction between SmartEngine and the AI app implementation.

## Requirements

### Requirement: DecisionExecutor interface definition
The engine package SHALL define a `DecisionExecutor` interface that accepts a `DecisionRequest` (system prompt, user message, tool definitions, tool handler closure, model config, max turns) and returns the final LLM content string synchronously.

#### Scenario: SmartEngine calls DecisionExecutor for agentic decision
- **WHEN** SmartEngine runs a decision cycle for a ticket
- **THEN** it SHALL delegate to the injected `DecisionExecutor` implementation instead of managing its own ReAct loop

#### Scenario: DecisionExecutor receives tool handler closure
- **WHEN** SmartEngine prepares a `DecisionRequest`
- **THEN** the `ToolHandler` field SHALL be a closure that wraps the decision tools with the current ticket's context (ticketID, serviceID, DB connection)

### Requirement: AI App provides DecisionExecutor implementation
The AI App SHALL provide a `DecisionExecutor` implementation that uses `llm.Client.Chat` (synchronous, non-streaming) to run a ReAct tool-calling loop.

#### Scenario: DecisionExecutor resolves LLM config from agentID
- **WHEN** `Execute(ctx, agentID, req)` is called
- **THEN** the implementation SHALL look up the agent's model and provider to create an `llm.Client`, without exposing API keys to the caller

#### Scenario: DecisionExecutor ReAct loop
- **WHEN** the LLM returns tool calls
- **THEN** the implementation SHALL call `req.ToolHandler` for each tool call, append results to messages, and loop until the LLM produces a final response without tool calls or max turns is exceeded

#### Scenario: DecisionExecutor returns error on max turns exceeded
- **WHEN** the ReAct loop reaches `req.MaxTurns` without a final answer
- **THEN** it SHALL return an error

### Requirement: DecisionExecutor injected via SmartEngine constructor
The `NewSmartEngine` function SHALL accept a `DecisionExecutor` parameter. When nil (AI App not installed), `SmartEngine.IsAvailable()` SHALL return false.

#### Scenario: SmartEngine without DecisionExecutor
- **WHEN** `DecisionExecutor` is nil
- **THEN** `Start()` and `Progress()` SHALL return `ErrSmartEngineUnavailable`

#### Scenario: Test injection
- **WHEN** a test creates SmartEngine with a mock DecisionExecutor
- **THEN** the mock SHALL be called instead of any real LLM client

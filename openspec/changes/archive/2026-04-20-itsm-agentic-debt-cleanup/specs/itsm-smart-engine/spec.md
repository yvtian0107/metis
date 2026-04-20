## MODIFIED Requirements

### Requirement: SmartEngine decision cycle
The SmartEngine SHALL delegate its agentic decision logic to the injected `DecisionExecutor` interface instead of directly managing an LLM client and ReAct loop internally. The `AgentProvider` interface SHALL no longer need to expose API keys — the `DecisionExecutor` resolves LLM configuration internally via agentID.

#### Scenario: Decision cycle delegates to DecisionExecutor
- **WHEN** `runDecisionCycle` is called for a ticket
- **THEN** the engine SHALL build the seed messages (system + user), wrap decision tools as a `ToolHandler` closure, and call `DecisionExecutor.Execute(ctx, agentID, req)`

#### Scenario: Decision plan parsing unchanged
- **WHEN** `DecisionExecutor` returns the final LLM content
- **THEN** the engine SHALL parse it as a `DecisionPlan` using the existing `parseDecisionPlan` function

#### Scenario: Timeout handled by context
- **WHEN** the decision timeout expires
- **THEN** the context cancellation SHALL propagate to `DecisionExecutor`, which SHALL return a context error

### Requirement: SmartEngine constructor signature update
`NewSmartEngine` SHALL replace the `AgentProvider` parameter with `DecisionExecutor`. The `AgentProvider` interface SHALL be removed from the engine package.

#### Scenario: Constructor with DecisionExecutor
- **WHEN** creating a SmartEngine with a non-nil `DecisionExecutor`
- **THEN** `IsAvailable()` SHALL return true

#### Scenario: Constructor without DecisionExecutor
- **WHEN** creating a SmartEngine with nil `DecisionExecutor`
- **THEN** `IsAvailable()` SHALL return false

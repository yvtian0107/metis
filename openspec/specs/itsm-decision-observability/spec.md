## Purpose

ITSM SmartEngine 决策循环的结构化日志与可观测性 -- 决策入口/跳过、工具调用、计划结果、执行结果的 Info/Warn 级别日志，以及 ticketID 贯穿追踪。

## Requirements

### Requirement: Decision cycle entry logging
SmartEngine.runDecisionCycle() SHALL log an Info-level message at entry that includes the ticketID, trigger reason, service name, agent ID, and decision mode.

#### Scenario: Decision cycle starts on initial ticket submission
- **WHEN** a smart-progress task triggers runDecisionCycle with triggerReason="initial_decision"
- **THEN** the system SHALL output an Info log with message prefix "decision-cycle: starting", containing ticketID, triggerReason="initial_decision", serviceID, agentID, and decisionMode

#### Scenario: Decision cycle starts after activity completion
- **WHEN** a smart-progress task triggers runDecisionCycle with triggerReason="activity_completed" and a completedActivityID
- **THEN** the system SHALL output an Info log containing ticketID, triggerReason="activity_completed", completedActivityID, serviceID, agentID, and decisionMode

### Requirement: Decision cycle skipped logging
SmartEngine.runDecisionCycle() SHALL log when the decision cycle is skipped due to terminal state, AI disabled, or active activities — with the specific reason.

#### Scenario: Ticket is in terminal state
- **WHEN** runDecisionCycle finds the ticket in status "completed", "cancelled", or "failed"
- **THEN** the system SHALL output an Info log with message prefix "decision-cycle: skipped" containing ticketID and reason="terminal_state"

#### Scenario: AI failure count exceeded
- **WHEN** runDecisionCycle finds ticket.AIFailureCount >= MaxAIFailureCount
- **THEN** the system SHALL output a Warn log with message prefix "decision-cycle: skipped" containing ticketID, reason="ai_disabled", and failureCount

### Requirement: Tool call structured logging at Info level
Every decision tool invocation within the DecisionExecutor ReAct loop SHALL be logged at Info level with the tool name, execution duration, and success/error status. The ticketID MUST be present in every log entry.

#### Scenario: Successful tool call
- **WHEN** a tool call to "decision.ticket_context" completes successfully in 12ms
- **THEN** the system SHALL output an Info log with message prefix "decision-tool: call", containing ticketID, tool="decision.ticket_context", durationMs=12, and ok=true

#### Scenario: Tool call returns error
- **WHEN** a tool call to "decision.resolve_participant" returns an error
- **THEN** the system SHALL output a Warn log with message prefix "decision-tool: error", containing ticketID, tool="decision.resolve_participant", durationMs, ok=false, and the error message

#### Scenario: Unknown tool name
- **WHEN** the LLM requests a tool name not in the handler map
- **THEN** the system SHALL output a Warn log with message prefix "decision-tool: unknown", containing ticketID and the unknown tool name

### Requirement: Tool call wrapper in SmartEngine
The tool logging SHALL be implemented as a wrapper around the toolHandler closure in SmartEngine.agenticDecision(), so that all current and future decision tools are automatically covered without per-tool modifications.

#### Scenario: New tool added without logging code
- **WHEN** a developer adds a 9th decision tool to allDecisionTools() in smart_tools.go
- **THEN** the new tool's calls SHALL automatically appear in logs without any additional logging code, because the wrapper covers all tools dispatched through the handler map

### Requirement: Decision plan result logging
After agenticDecision() returns a DecisionPlan, SmartEngine SHALL log the plan's key fields at Info level.

#### Scenario: High-confidence decision plan
- **WHEN** agenticDecision returns a plan with next_step_type="process", confidence=0.92, 1 activity, execution_mode="single"
- **THEN** the system SHALL output an Info log with message prefix "decision-cycle: plan", containing ticketID, nextStepType="process", confidence=0.92, activityCount=1, executionMode="single"

#### Scenario: Low-confidence decision plan
- **WHEN** agenticDecision returns a plan with confidence below the threshold
- **THEN** the system SHALL output an Info log with the plan summary and an additional Info log with message prefix "decision-cycle: low-confidence" indicating the plan will be pended for manual handling

#### Scenario: Decision plan validation failure
- **WHEN** validateDecisionPlan returns an error
- **THEN** the system SHALL output a Warn log with message prefix "decision-cycle: validation-failed", containing ticketID and the validation error message

### Requirement: Plan execution result logging
After executeDecisionPlan creates activities and assignments, the system SHALL log what was created.

#### Scenario: Single activity created with user assignment
- **WHEN** executeSinglePlan creates an activity of type "process" and assigns it to user ID 7
- **THEN** the system SHALL output an Info log with message prefix "decision-cycle: executed", containing ticketID, activityType="process", activityID, assigneeID=7, executionMode="single"

#### Scenario: Parallel activities created
- **WHEN** executeParallelPlan creates 3 parallel activities in a group
- **THEN** the system SHALL output an Info log with message prefix "decision-cycle: executed", containing ticketID, activityCount=3, executionMode="parallel", groupID

#### Scenario: Complete decision executed
- **WHEN** handleComplete marks the ticket as completed
- **THEN** the system SHALL output an Info log with message prefix "decision-cycle: completed", containing ticketID

### Requirement: TicketID in all decision log entries
Every slog call added by this change SHALL include a structured "ticketID" attribute, enabling `grep ticketID=<id>` to retrieve the full decision chain for a single ticket.

#### Scenario: Grep for a specific ticket
- **WHEN** an operator runs `grep ticketID=42` against the application log output
- **THEN** the output SHALL include all decision-cycle, decision-tool, and plan execution log entries for ticket 42, in chronological order

### Requirement: AIDecisionRequest metadata pass-through
app.AIDecisionRequest SHALL support a Metadata field (map[string]any) that callers can use to pass contextual attributes (e.g., ticketID) into the DecisionExecutor. The DecisionExecutor SHALL include these attributes in all tool-dispatch log entries.

#### Scenario: SmartEngine passes ticketID via metadata
- **WHEN** SmartEngine calls decisionExecutor.Execute with Metadata containing ticketID=42
- **THEN** the DecisionExecutor SHALL include ticketID=42 in the "decision-tool: call" and "decision-tool: error" log entries

#### Scenario: Metadata is nil
- **WHEN** a caller invokes DecisionExecutor with nil Metadata
- **THEN** the DecisionExecutor SHALL log tool calls without additional metadata attributes (no panic, no error)

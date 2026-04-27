## ADDED Requirements

### Requirement: 直接调度路径保持 Tools 合同
Decision tools SHALL remain available and behaviorally identical when SmartEngine is launched by direct goroutine dispatch. Tool registration, tool names, input/output schemas, and validation expectations SHALL NOT differ from the scheduler-launched decision path.

#### Scenario: 决策上下文工具可用
- **WHEN** SmartEngine starts from direct decision dispatch
- **THEN** the decision agent SHALL be able to call `decision.ticket_context`
- **AND** the tool response SHALL include ticket status, activity history, completed activity facts, workflow_json, and workflow_context

#### Scenario: 参与人解析工具可用
- **WHEN** SmartEngine starts from direct decision dispatch and needs to create a human activity
- **THEN** the decision agent SHALL be able to call `decision.resolve_participant`
- **AND** participant resolution SHALL use the same org resolver behavior as the previous path

#### Scenario: 动作工具可用
- **WHEN** SmartEngine starts from direct decision dispatch and needs to execute an action
- **THEN** the decision agent SHALL be able to call `decision.list_actions` and `decision.execute_action`
- **AND** action execution SHALL continue through the existing action execution infrastructure

### Requirement: Rejected context remains explicit
When direct dispatch follows a rejected human activity, decision tools SHALL expose rejected facts as first-class context, including rejected activity id, node id, outcome, operator opinion, satisfied=false, and requires_recovery_decision=true.

#### Scenario: 驳回后上下文可见
- **WHEN** a user rejects a human activity and direct dispatch starts SmartEngine
- **THEN** `decision.ticket_context` SHALL expose the rejected activity as completed_activity
- **AND** completed_activity SHALL include outcome=`rejected`
- **AND** completed_activity SHALL include requires_recovery_decision=true

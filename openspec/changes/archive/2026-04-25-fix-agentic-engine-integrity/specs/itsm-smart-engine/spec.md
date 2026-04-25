## ADDED Requirements

### Requirement: ensureContinuation called from Start
SmartEngine.Start() SHALL call ensureContinuation(tx, ticket, 0) after setting ticket status to "in_progress" and recording the timeline event. The completedActivityID=0 indicates this is the initial decision trigger. ensureContinuation's existing gate logic (terminal check, circuit breaker) SHALL apply.

#### Scenario: Start triggers initial decision cycle
- **WHEN** SmartEngine.Start() is called for a new ticket
- **THEN** after setting status to in_progress, ensureContinuation is called with completedActivityID=0, which submits an itsm-smart-progress async task

#### Scenario: Start with AI disabled does not trigger decision
- **WHEN** SmartEngine.Start() is called for a ticket where ai_failure_count >= MaxAIFailureCount
- **THEN** ensureContinuation's circuit breaker gate prevents task submission

### Requirement: ensureContinuation called from Cancel
SmartEngine.Cancel() SHALL call ensureContinuation(tx, ticket, 0) after setting ticket status to "cancelled". This allows the decision cycle to perform cleanup if needed. If the ticket is already terminal, ensureContinuation's terminal gate SHALL prevent further processing.

#### Scenario: Cancel triggers cleanup decision
- **WHEN** SmartEngine.Cancel() is called on an in_progress ticket
- **THEN** after setting status to cancelled, ensureContinuation is called

#### Scenario: Cancel on terminal ticket is a no-op
- **WHEN** SmartEngine.Cancel() is called on an already cancelled ticket
- **THEN** ensureContinuation's terminal state gate returns immediately without submitting a task

### Requirement: handleComplete writes NodeID
SmartEngine.handleComplete() SHALL set the terminal activity's NodeID to the end node's ID from workflow_json. If workflow_json is unavailable or no end node is found, NodeID SHALL default to empty string.

#### Scenario: Complete activity gets end node ID
- **WHEN** handleComplete creates a "流程完结" activity and workflow_json has an end node with id "end_1"
- **THEN** the activity's NodeID is set to "end_1"

#### Scenario: Complete without workflow_json
- **WHEN** handleComplete creates a terminal activity and workflow_json is not available
- **THEN** the activity's NodeID is set to empty string

### Requirement: ExecutionMode validation on parse
parseDecisionPlan() SHALL validate that execution_mode is one of: empty string, "single", or "parallel". If any other value is present, it SHALL log a warning and default to empty string (single mode behavior).

#### Scenario: Valid execution_mode accepted
- **WHEN** parseDecisionPlan parses a plan with execution_mode="parallel"
- **THEN** DecisionPlan.ExecutionMode is set to "parallel"

#### Scenario: Invalid execution_mode defaults to empty
- **WHEN** parseDecisionPlan parses a plan with execution_mode="batch"
- **THEN** DecisionPlan.ExecutionMode is set to "" and a warning is logged

#### Scenario: Missing execution_mode backward compatible
- **WHEN** parseDecisionPlan parses a plan without execution_mode field
- **THEN** DecisionPlan.ExecutionMode is set to "" (existing behavior preserved)

## MODIFIED Requirements

### Requirement: SmartEngine continuation trigger points
SmartEngine SHALL trigger near-real-time continuation at all state change boundaries. Trigger points: (1) Manual activity completion → submit itsm-smart-progress immediately. (2) Action completion → submit itsm-smart-progress immediately. (3) AI decision approval → apply decision + submit itsm-smart-progress. (4) AI decision rejection → record reason + submit itsm-smart-progress. (5) Start() → call ensureContinuation for initial decision cycle. (6) Cancel() → call ensureContinuation for cleanup. Before progressing, re-check current state to prevent duplicate submissions.

#### Scenario: 人工审批完成后近实时续跑
- **WHEN** 一个人工审批活动被完成
- **THEN** 系统立即提交 itsm-smart-progress 异步任务

#### Scenario: action 完成后近实时续跑
- **WHEN** 一个 action 活动执行成功
- **THEN** 系统立即提交 itsm-smart-progress 异步任务

#### Scenario: AI 决策确认后近实时续跑
- **WHEN** 一个 status=pending_approval 的 AI 活动被授权用户确认
- **THEN** 系统应用决策结果并立即提交 itsm-smart-progress 异步任务

#### Scenario: AI 决策拒绝后近实时续跑
- **WHEN** 一个 status=pending_approval 的 AI 活动被拒绝
- **THEN** 系统记录拒绝原因并立即提交 itsm-smart-progress 异步任务

#### Scenario: Start 触发首次决策循环
- **WHEN** SmartEngine.Start() 设置工单状态为 in_progress
- **THEN** 系统通过 ensureContinuation 提交首次 itsm-smart-progress 异步任务

#### Scenario: Cancel 触发清理决策
- **WHEN** SmartEngine.Cancel() 设置工单状态为 cancelled
- **THEN** 系统通过 ensureContinuation 触发清理流程

#### Scenario: 并发触发续跑不重复推进状态
- **WHEN** 两个触发源几乎同时触发续跑
- **THEN** 系统 re-check 当前状态确保不重复创建/完成同一步骤

### Requirement: DecisionPlan 并签字段
DecisionPlan SHALL include an ExecutionMode field parsed from the AI response's execution_mode. Valid values are: empty string (default single mode), "single", or "parallel". Invalid values SHALL be defaulted to empty string with a warning logged. When ExecutionMode is "parallel", all activities in the plan are created simultaneously and run concurrently. The cosign gate SHALL check that ALL parallel activities are completed before triggering ensureContinuation.

#### Scenario: 并签活动全部完成后续跑
- **WHEN** DecisionPlan.ExecutionMode == "parallel" 且所有活动均已完成
- **THEN** ensureContinuation 检测到全部完成，提交下一轮 itsm-smart-progress

#### Scenario: 并签活动部分完成不续跑
- **WHEN** DecisionPlan.ExecutionMode == "parallel" 且仅部分活动完成
- **THEN** ensureContinuation 检测到尚有未完成活动，不提交新任务

#### Scenario: 无效 ExecutionMode 回退为单活动模式
- **WHEN** parseDecisionPlan 解析到 execution_mode="batch"
- **THEN** ExecutionMode 被设为 ""，记录警告日志，按单活动模式执行

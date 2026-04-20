## Purpose

ITSM SmartEngine 核心引擎 -- ReAct 决策循环、DecisionPlan 解析与执行。

## Requirements

### Requirement: SmartEngine decision cycle
The SmartEngine SHALL delegate its agentic decision logic to the injected `DecisionExecutor` interface instead of directly managing an LLM client and ReAct loop internally. The `AgentProvider` interface SHALL no longer need to expose API keys because the `DecisionExecutor` resolves LLM configuration internally via `agentID`.

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

### Requirement: DecisionPlan 并签字段
DecisionPlan 结构 SHALL 新增 `ExecutionMode string` 字段（JSON key: `execution_mode`）。合法值为 `""` / `"single"` / `"parallel"`。空值或 `"single"` 表示串行（现有行为），`"parallel"` 表示并签。

#### Scenario: 解析含 execution_mode 的 DecisionPlan
- **WHEN** Agent 输出的 JSON 包含 `"execution_mode": "parallel"`
- **THEN** `parseDecisionPlan()` SHALL 正确解析该字段到 `DecisionPlan.ExecutionMode`

#### Scenario: execution_mode 缺失向后兼容
- **WHEN** Agent 输出的 JSON 不包含 `execution_mode` 字段
- **THEN** `parseDecisionPlan()` SHALL 将 `ExecutionMode` 默认为空字符串，等同 `"single"`

### Requirement: executeDecisionPlan 并签分支
`executeDecisionPlan()` SHALL 在 `ExecutionMode == "parallel"` 时创建并签活动组，而非逐个覆盖 `current_activity_id`。并行计划中的 action 类型活动 SHALL 被正确调度执行。

#### Scenario: parallel 模式创建活动组
- **WHEN** `executeDecisionPlan()` 处理 `ExecutionMode == "parallel"` 的 DecisionPlan
- **THEN** SHALL 生成一个 UUID 作为 `activity_group_id`
- **AND** SHALL 为 activities 中的每个条目创建独立 TicketActivity，设置相同的 `activity_group_id`
- **AND** SHALL 将工单 `current_activity_id` 设为组内第一个 activity ID

#### Scenario: parallel 模式中的 action activity 被调度
- **WHEN** `executeDecisionPlan()` 处理 parallel 计划且 activities 中包含 action 类型
- **THEN** SHALL 为每个 action activity 提交 `itsm-action-execute` 异步任务
- **AND** action activity 的初始状态 SHALL 为 `in_progress`

#### Scenario: single 模式只创建第一个 activity
- **WHEN** `executeDecisionPlan()` 处理 `ExecutionMode` 为空或 `"single"` 的 DecisionPlan 且 activities 包含多个条目
- **THEN** SHALL 只创建第一个 activity 并设为 current
- **AND** SHALL 不预先创建后续 activity 记录
- **AND** 后续步骤 SHALL 由下一轮决策循环根据最新工单上下文重新决定

### Requirement: SmartEngine continuation trigger points
SmartEngine SHALL 在真正完成一个 smart 活动边界时近实时提交 `itsm-smart-progress` 续跑任务，而不是依赖轮询式推进。触发点至少包括：人工审批/处理完成、action 活动完成、AI `pending_approval` 决策被确认、AI `pending_approval` 决策被拒绝。

#### Scenario: 人工审批完成后近实时续跑
- **WHEN** smart 工单的当前人工活动完成并提交结果
- **THEN** 系统 SHALL 在该完成事务成功后提交 `itsm-smart-progress` 任务
- **AND** 下一轮决策 SHALL 无需等待周期性扫描才开始

#### Scenario: action 完成后近实时续跑
- **WHEN** smart 工单的 action 活动执行完成
- **THEN** 系统 SHALL 在 action 完成后提交 `itsm-smart-progress` 任务
- **AND** 引擎 SHALL 基于 action 结果进入下一轮决策

#### Scenario: AI 决策确认后近实时续跑
- **WHEN** status=`pending_approval` 的 AI 活动被授权用户确认
- **THEN** 系统 SHALL 应用该决策并提交 `itsm-smart-progress` 任务

#### Scenario: AI 决策拒绝后近实时续跑
- **WHEN** status=`pending_approval` 的 AI 活动被授权用户拒绝
- **THEN** 系统 SHALL 记录拒绝结果与理由并提交 `itsm-smart-progress` 任务
- **AND** 下一轮决策 SHALL 在包含拒绝上下文的前提下重新运行

#### Scenario: 并发触发续跑不重复推进状态
- **WHEN** 同一 smart 工单因接近同时的完成事件多次提交 `itsm-smart-progress`
- **THEN** 引擎 SHALL 在进入决策前重新检查当前 ticket/activity 状态
- **AND** 重复提交 SHALL 不得导致同一逻辑步骤被重复创建或重复完成

### Requirement: Signal 按引擎类型分派
`ticket_service.Signal()` SHALL 根据工单的 `engine_type` 分派到正确的引擎，而非硬编码 classic engine。

#### Scenario: Smart engine 工单收到 Signal
- **WHEN** Signal 被调用且工单 `engine_type` 为 smart
- **THEN** SHALL 调用 `smartEngine.Progress()` 而非 `classicEngine.Progress()`

#### Scenario: Classic engine 工单收到 Signal
- **WHEN** Signal 被调用且工单 `engine_type` 为 classic
- **THEN** SHALL 调用 `classicEngine.Progress()`（保持现有行为）

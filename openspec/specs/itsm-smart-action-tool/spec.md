## Purpose

ITSM SmartEngine decision.execute_action 决策工具 -- 允许 AI Agent 在 ReAct 循环中同步执行服务配置的自动化动作（ServiceAction）并获取执行结果。

## Requirements

### Requirement: decision.execute_action 决策工具
SmartEngine SHALL 提供 `decision.execute_action` 决策工具，允许 AI Agent 在 ReAct 循环中同步执行服务配置的自动化动作（ServiceAction）并获取执行结果。

#### Scenario: Agent 同步执行 action 并获取结果
- **WHEN** Agent 在 ReAct 循环中调用 `decision.execute_action`，传入 `action_id` 参数
- **THEN** 工具 SHALL 加载 ServiceAction 配置，执行 HTTP webhook 调用
- **AND** 工具 SHALL 等待 webhook 返回结果（受 ActionConfig.Timeout 限制）
- **AND** 工具 SHALL 将执行结果（status, response_body）作为 tool result 返回给 Agent
- **AND** Agent SHALL 在同一推理链中看到结果并继续决策

#### Scenario: Action 执行成功记录
- **WHEN** `decision.execute_action` 成功执行 webhook
- **THEN** 工具 SHALL 创建 `TicketActionExecution` 记录（status=success, response_body 存储）
- **AND** tool result SHALL 包含 `{"success": true, "action_name": "...", "response": ...}`

#### Scenario: Action 执行失败
- **WHEN** `decision.execute_action` 执行 webhook 返回非 2xx 或超时
- **THEN** 工具 SHALL 创建 `TicketActionExecution` 记录（status=failed）
- **AND** tool result SHALL 包含 `{"success": false, "error": "..."}`
- **AND** Agent SHALL 看到失败结果并自行决策下一步

#### Scenario: Action 不存在或不可用
- **WHEN** Agent 调用 `decision.execute_action` 且 `action_id` 对应的 ServiceAction 不存在或 `is_active=false`
- **THEN** 工具 SHALL 返回错误 `{"error": true, "message": "动作不存在或已停用"}`

### Requirement: Action 执行复用现有基础设施
`decision.execute_action` 工具 SHALL 复用现有 `ActionExecutor` 的 webhook 调用逻辑，不重复实现 HTTP 调用、模板变量替换、重试等能力。

#### Scenario: 模板变量替换
- **WHEN** 执行 action 时 webhook 请求体包含 `{{ticket.code}}` 模板变量
- **THEN** 工具 SHALL 使用现有 `replaceTemplateVars()` 进行替换

#### Scenario: 超时控制
- **WHEN** 执行 action 时
- **THEN** 工具 SHALL 使用 ActionConfig 中配置的 Timeout 值控制 HTTP 调用超时

### Requirement: decisionToolContext 扩展
`decisionToolContext` SHALL 新增 `actionExecutor` 字段（`*ActionExecutor` 类型），用于 `decision.execute_action` 工具执行 webhook。

#### Scenario: actionExecutor 注入
- **WHEN** ReAct 循环初始化 decisionToolContext
- **THEN** SHALL 将 SmartEngine 持有的 ActionExecutor 实例注入 toolCtx.actionExecutor

### Requirement: BDD 验证 Action 元调用场景
系统 SHALL 提供 LLM 驱动的 BDD 场景验证 Agent 的 action 元调用能力。

#### Scenario: Agent 自主调用触发器并完成流程
- **WHEN** 服务配置了自动化动作（如 db_backup_execute）且协作规范要求执行动作
- **THEN** Agent SHALL 在 ReAct 循环中主动调用 `decision.execute_action`
- **AND** Agent SHALL 看到执行结果后判断流程是否完成
- **AND** 整个流程 SHALL 在最少的决策循环内完成（2 轮而非 4 轮）

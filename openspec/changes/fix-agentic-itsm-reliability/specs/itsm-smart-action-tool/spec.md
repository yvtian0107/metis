## MODIFIED Requirements

### Requirement: decision.execute_action 决策工具
SmartEngine SHALL 提供 `decision.execute_action` 决策工具，允许 AI Agent 在 ReAct 循环中同步执行服务配置的自动化动作（ServiceAction）并获取执行结果。工具 SHALL 使用带超时的 context 执行 webhook 调用，并提供幂等保护。

#### Scenario: Agent 同步执行 action 并获取结果
- **WHEN** Agent 在 ReAct 循环中调用 `decision.execute_action`，传入 `action_id` 参数
- **THEN** 工具 SHALL 加载 ServiceAction 配置，使用从决策上下文派生的子 context（带 ActionConfig.Timeout 超时）执行 HTTP webhook 调用
- **AND** 工具 SHALL 等待 webhook 返回结果
- **AND** 工具 SHALL 将执行结果（status, response_body）作为 tool result 返回给 Agent
- **AND** Agent SHALL 在同一推理链中看到结果并继续决策

#### Scenario: 幂等保护 — action 已成功执行
- **WHEN** Agent 调用 `decision.execute_action` 且该 action_id 在当前工单已有 status=success 的 TicketActionExecution 记录
- **THEN** 工具 SHALL 直接返回缓存的成功结果 `{"success": true, "action_name": "...", "response": ..., "cached": true}`
- **AND** 不再重复执行 webhook

#### Scenario: 幂等保护 — action 之前执行失败
- **WHEN** Agent 调用 `decision.execute_action` 且该 action_id 在当前工单仅有 status=failed 的 TicketActionExecution 记录
- **THEN** 工具 SHALL 正常重新执行 webhook（允许重试失败的 action）

#### Scenario: Context 超时控制
- **WHEN** 执行 action 的 webhook 调用超过 ActionConfig.Timeout
- **THEN** 工具 SHALL 因 context 超时取消请求
- **AND** 返回 `{"success": false, "error": "执行超时"}`

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

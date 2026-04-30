## MODIFIED Requirements

### Requirement: 服务台会话状态管理
AgentSession 状态模型 SHALL 扩展为对话式提单状态机，支持 `missing_fields`、`asked_fields`、`min_decision_ready` 等字段，以驱动增量追问与最小可决策提交。系统 MUST 在每轮工具调用后更新并持久化状态，且不得重复追问已确认字段。

#### Scenario: 追问缺失字段并推进状态
- **WHEN** 用户输入请求后存在关键字段缺失
- **THEN** 系统 SHALL 在会话状态记录 missing_fields 并发起下一轮追问

#### Scenario: 达到最小可决策条件
- **WHEN** 会话状态满足 min_decision_ready=true
- **THEN** 工具链 SHALL 允许进入 draft_confirm 与 ticket_create

#### Scenario: 已确认字段不重复追问
- **WHEN** 字段已存在于 asked_fields 且已确认
- **THEN** 后续工具调用 SHALL 跳过该字段追问

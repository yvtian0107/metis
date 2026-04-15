## ADDED Requirements

### Requirement: Activity token_id 外键
`itsm_ticket_activities` 表 SHALL 新增 `token_id` 字段（uint, nullable, INDEX）。新创建的 Activity SHALL 始终设置 token_id。token_id 关联到 `itsm_execution_tokens.id`。

#### Scenario: 人工节点创建 Activity 绑定 token
- **WHEN** processNode 处理 form/approve/process/wait 节点时创建 Activity
- **THEN** Activity 的 token_id SHALL 设置为当前 token 的 ID

#### Scenario: 自动节点创建 Activity 绑定 token
- **WHEN** processNode 处理 action 节点时创建 Activity
- **THEN** Activity 的 token_id SHALL 设置为当前 token 的 ID

#### Scenario: End 节点创建 Activity 绑定 token
- **WHEN** processNode 处理 end 节点时创建完成 Activity
- **THEN** Activity 的 token_id SHALL 设置为当前 token 的 ID

---

### Requirement: Activity 双 FK 查询
Activity SHALL 同时保留 `ticket_id` 和 `token_id` 两个外键。`ticket_id` 用于"查询工单所有 Activity"的高频查询场景，`token_id` 用于"查询某 token 关联的 Activity"精确绑定场景。

#### Scenario: 按工单查询所有 Activity
- **WHEN** 工单详情页请求 Activity 列表
- **THEN** 系统通过 `WHERE ticket_id = ?` 查询，不需要 JOIN token 表

#### Scenario: 按 token 查询关联 Activity
- **WHEN** 引擎需要查找某 token 的当前 Activity
- **THEN** 系统通过 `WHERE token_id = ? AND status IN ('pending', 'in_progress')` 查询

---

### Requirement: Progress 加载 token
ClassicEngine.Progress() SHALL 在完成 Activity 后，通过 `activity.token_id` 加载关联的 ExecutionToken，然后基于 token 推进到下一节点。

#### Scenario: Progress 使用 token 推进
- **WHEN** 处理人提交 Activity 的 outcome
- **THEN** 系统加载 activity.token_id 对应的 token，使用 token 调用 processNode 推进

#### Scenario: Activity 无 token_id 时报错
- **WHEN** Progress 处理的 Activity 没有 token_id（理论上不应发生）
- **THEN** 系统返回错误 "activity has no associated execution token"

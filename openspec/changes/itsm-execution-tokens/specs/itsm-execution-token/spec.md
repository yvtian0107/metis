## ADDED Requirements

### Requirement: ExecutionToken 模型
系统 SHALL 提供 `ExecutionToken` 模型（表名 `itsm_execution_tokens`），字段包含：`id`(PK), `ticket_id`(FK, NOT NULL, INDEX), `parent_token_id`(自引用 FK, nullable), `node_id`(VARCHAR(64), 当前所在节点 ID), `status`(VARCHAR(16), NOT NULL), `token_type`(VARCHAR(16), NOT NULL), `scope_id`(VARCHAR(64), NOT NULL, default "root"), `created_at`, `updated_at`。模型 SHALL 嵌入 `model.BaseModel`。

#### Scenario: Root token 创建
- **WHEN** ClassicEngine.Start() 被调用
- **THEN** 系统创建一个 ExecutionToken，ticket_id 为当前工单 ID，parent_token_id 为 nil，token_type 为 "main"，status 为 "active"，scope_id 为 "root"

#### Scenario: Token 节点推进
- **WHEN** processNode 推进到新节点
- **THEN** 系统更新 token 的 node_id 为新节点的 ID

#### Scenario: Token 完成
- **WHEN** token 到达 end 节点
- **THEN** token 的 status SHALL 更新为 "completed"

#### Scenario: Token 取消
- **WHEN** 工单被取消
- **THEN** 所有 status 为 "active" 或 "waiting" 的 token SHALL 更新为 "cancelled"

---

### Requirement: Token 状态机
ExecutionToken 的 status 字段 SHALL 遵循以下状态机：初始状态为 `active`。合法转换为：`active` → `completed`（正常完成），`active` → `waiting`（fork 出子 token，④ 中实现），`active` → `cancelled`（取消），`waiting` → `completed`（所有子 token 完成，④ 中实现），`waiting` → `cancelled`（取消）。`suspended` 状态 SHALL 定义常量但本 change 不使用（预留给 ⑤ 边界事件）。

#### Scenario: active → completed 转换
- **WHEN** token 到达 end 节点且正常完结
- **THEN** token.status 从 "active" 更新为 "completed"

#### Scenario: active → cancelled 转换
- **WHEN** 工单被取消
- **THEN** 所有 active 状态的 token.status 更新为 "cancelled"

#### Scenario: 非法状态转换
- **WHEN** 尝试将 "completed" 状态的 token 更新为 "active"
- **THEN** 系统 SHALL 拒绝该操作（已完成的 token 不可复活）

---

### Requirement: Token 状态和类型常量
系统 SHALL 在 `engine.go` 中定义以下常量：

Token 状态：`TokenActive = "active"`, `TokenWaiting = "waiting"`, `TokenCompleted = "completed"`, `TokenCancelled = "cancelled"`, `TokenSuspended = "suspended"`。

Token 类型：`TokenMain = "main"`, `TokenParallel = "parallel"`, `TokenSubprocess = "subprocess"`, `TokenMultiInstance = "multi_instance"`, `TokenBoundary = "boundary"`。

本 change 只使用 `TokenActive`、`TokenCompleted`、`TokenCancelled` 状态和 `TokenMain` 类型。

#### Scenario: 常量可用
- **WHEN** 引擎代码引用 token 状态或类型
- **THEN** 使用预定义常量而非字符串字面量

#### Scenario: 预留常量注释
- **WHEN** 开发者查看 suspended/parallel/subprocess 等预留常量
- **THEN** 注释 SHALL 明确标注这些常量属于哪个后续 change（④ 或 ⑤）

---

### Requirement: Token 树查询
系统 SHALL 提供按 ticket_id 查询活跃 token 的方法。查询 SHALL 使用复合索引 `(ticket_id, status)` 提高效率。

#### Scenario: 查询工单活跃 token
- **WHEN** 引擎需要查找某工单的所有活跃 token
- **THEN** 系统返回该工单所有 status="active" 的 ExecutionToken 列表

#### Scenario: 查询 root token
- **WHEN** 引擎需要获取工单的主 token
- **THEN** 系统返回该工单 parent_token_id 为 nil 且 token_type="main" 的 ExecutionToken

---

### Requirement: Token 与变量作用域联动
ExecutionToken 的 `scope_id` 字段 SHALL 与 `itsm_process_variables` 表的 `scope_id` 一致。Root token 的 scope_id 为 "root"。子流程 token（⑤ 中实现）将使用独立的 scope_id 实现变量隔离。

#### Scenario: Root token 变量作用域
- **WHEN** root token 执行过程中写入变量
- **THEN** 变量的 scope_id 为 "root"，与 token.scope_id 一致

#### Scenario: Token scope_id 传递
- **WHEN** processNode 中调用 writeFormBindings
- **THEN** 使用当前 token 的 scope_id 作为变量的 scope_id，而非硬编码 "root"

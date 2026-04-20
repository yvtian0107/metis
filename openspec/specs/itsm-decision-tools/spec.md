## Purpose

ITSM SmartEngine 决策域工具集 -- 提供 8 个决策域工具供 ReAct 循环中的 Agent 按需调用。

## Requirements

### Requirement: 决策域工具集定义
SmartEngine SHALL 提供一组决策域工具，供决策 Agent 在 ReAct 循环中按需调用。工具定义（名称、描述、参数 JSON Schema）在 `smart_tools.go` 中硬编码注册，不通过 `ai_agent_tools` 表动态绑定。每个工具 SHALL 返回 JSON 格式结果。

#### Scenario: 工具集包含 8 个决策工具
- **WHEN** SmartEngine 初始化构建工具列表
- **THEN** 工具集 SHALL 包含以下 8 个工具：`decision.ticket_context`、`decision.knowledge_search`、`decision.resolve_participant`、`decision.user_workload`、`decision.similar_history`、`decision.sla_status`、`decision.list_actions`、`decision.execute_action`

#### Scenario: 工具定义转换为 llm.ToolDef
- **WHEN** ReAct 循环构建 `llm.ChatRequest`
- **THEN** 每个决策工具 SHALL 被转换为 `llm.ToolDef{Name, Description, Parameters}` 格式传入 `ChatRequest.Tools`

### Requirement: decision.ticket_context 工具
该工具 SHALL 返回工单的完整上下文信息，包括表单数据、SLA 状态、活动历史和并签组状态。这是初始 seed 的补充，Agent 需要详细信息时调用。

参数：无（工具执行时从 ReAct 循环上下文获取 ticketID）

返回字段：
- `form_data`: 完整表单 JSON
- `description`: 工单详细描述
- `sla_status`: SLA 剩余时间（response_remaining_seconds, resolution_remaining_seconds），无 SLA 时为 null
- `activity_history`: 已完成活动列表（type, name, outcome, completed_at, ai_reasoning）
- `current_assignment`: 当前指派信息（assignee_id, assignee_name），无指派时为 null
- `executed_actions`: 已成功执行的动作名称列表
- `all_actions_completed`: 布尔值，所有服务动作是否全部执行完毕
- `parallel_groups`: 当前活跃的并签组状态（group_id, total, completed, pending_activities）

#### Scenario: 查询含并签组的工单上下文
- **WHEN** Agent 调用 `decision.ticket_context` 且工单有一个活跃的并签组（2 活动，1 已完成）
- **THEN** 返回结果 SHALL 包含 `parallel_groups` 字段，其中 `total=2, completed=1, pending_activities` 列出未完成活动

#### Scenario: 查询无并签组的工单上下文
- **WHEN** Agent 调用 `decision.ticket_context` 且工单无活跃并签组
- **THEN** 返回结果 SHALL NOT 包含 `parallel_groups` 字段（或为空数组）

#### Scenario: 查询含 SLA 的工单上下文
- **WHEN** Agent 调用 `decision.ticket_context` 且工单关联了 SLA 模板
- **THEN** 返回结果 SHALL 包含 `sla_status` 字段，其中 `response_remaining_seconds` 和 `resolution_remaining_seconds` 为距当前时间的剩余秒数

#### Scenario: 查询无 SLA 的工单上下文
- **WHEN** Agent 调用 `decision.ticket_context` 且工单未关联 SLA
- **THEN** 返回结果的 `sla_status` SHALL 为 null

#### Scenario: 查询含活动历史的工单
- **WHEN** Agent 调用 `decision.ticket_context` 且工单有 3 个已完成活动
- **THEN** 返回结果的 `activity_history` SHALL 包含 3 条记录，按完成时间升序排列

### Requirement: decision.knowledge_search 工具
该工具 SHALL 搜索服务关联的知识库，返回与查询相关的知识片段。复用现有 `KnowledgeSearcher` 接口。工具 SHALL 从 ServiceDefinition 的 `knowledge_base_ids` 字段获取关联知识库 ID 列表，传递给 KnowledgeSearcher 进行搜索。

参数：
- `query` (string, required): 搜索查询文本
- `limit` (integer, optional, default 3): 返回结果数量上限

返回字段：
- `results`: 知识结果数组（title, content, score）
- `count`: 实际返回数量

#### Scenario: 搜索有结果
- **WHEN** Agent 调用 `decision.knowledge_search` 且服务关联的知识库中有匹配内容
- **THEN** 返回结果 SHALL 包含按 score 降序排列的知识片段，每项含 title、content 摘要和相关度 score

#### Scenario: KnowledgeSearcher 不可用
- **WHEN** Agent 调用 `decision.knowledge_search` 但 KnowledgeSearcher 为 nil（AI App 知识模块未启用）
- **THEN** 工具 SHALL 返回 `{"results": [], "count": 0, "message": "知识搜索不可用"}`，不视为错误

#### Scenario: 服务无关联知识库
- **WHEN** Agent 调用 `decision.knowledge_search` 但当前服务的 `knowledge_base_ids` 为空或 NULL
- **THEN** 工具 SHALL 返回空结果 `{"results": [], "count": 0}`

#### Scenario: 知识库 ID 部分失效
- **WHEN** Agent 调用 `decision.knowledge_search` 且 `knowledge_base_ids` 中包含已删除的知识库 ID
- **THEN** 工具 SHALL 忽略不存在的 KB ID，仅搜索仍存在的知识库，返回有效结果

### Requirement: decision.resolve_participant 工具
该工具 SHALL 按参与人类型解析出具体用户，复用现有 `ParticipantResolver` 和 `OrgService` 接口。

参数：
- `type` (string, required): 参与人类型，枚举 `"user" | "position" | "department" | "position_department" | "requester_manager"`
- `value` (string, optional): 类型相关值（user 类型为 user_id，position 类型为 position_code 等）
- `position_code` (string, optional): 岗位代码（position_department 类型时必填）
- `department_code` (string, optional): 部门代码（position_department 类型时必填）

返回字段：
- `candidates`: 匹配的用户列表（user_id, name, department, position）
- `count`: 匹配数量

#### Scenario: 按岗位解析参与人
- **WHEN** Agent 调用 `decision.resolve_participant` 且 type="position", value="it_manager"
- **THEN** 工具 SHALL 通过 OrgService 查询该岗位下的活跃用户列表

#### Scenario: 解析申请人主管
- **WHEN** Agent 调用 `decision.resolve_participant` 且 type="requester_manager"
- **THEN** 工具 SHALL 通过 OrgService 查询工单申请人的直属主管

#### Scenario: Org App 不可用
- **WHEN** Agent 调用 `decision.resolve_participant` 但 OrgService 为 nil
- **THEN** 工具 SHALL 返回错误信息 `"组织架构模块未安装，无法按岗位/部门解析参与人"`

#### Scenario: 直接指定用户
- **WHEN** Agent 调用 `decision.resolve_participant` 且 type="user", value="42"
- **THEN** 工具 SHALL 查询该用户是否存在且活跃，返回用户信息

### Requirement: decision.user_workload 工具
该工具 SHALL 查询指定用户当前的工单负载信息，帮助 Agent 做出负载均衡的指派决策。

参数：
- `user_id` (integer, required): 用户 ID

返回字段：
- `user_id`: 用户 ID
- `name`: 用户姓名
- `pending_activities`: 待处理活动数量（status=pending 或 in_progress）
- `is_active`: 用户是否活跃

#### Scenario: 查询有待办的用户
- **WHEN** Agent 调用 `decision.user_workload` 且该用户有 5 个未完成活动
- **THEN** 返回结果 SHALL 包含 `pending_activities: 5`

#### Scenario: 查询不存在的用户
- **WHEN** Agent 调用 `decision.user_workload` 且 user_id 对应的用户不存在
- **THEN** 工具 SHALL 返回错误信息 `"用户不存在"`

### Requirement: decision.similar_history 工具
该工具 SHALL 查询同一服务下已完成工单的处理模式，为 Agent 提供历史参考。

参数：
- `limit` (integer, optional, default 5): 返回工单数量上限

返回字段：
- `tickets`: 历史工单摘要数组（code, title, status, resolution_duration_hours, activity_count, assignee_names）
- `stats`: 聚合统计（avg_resolution_hours, total_count）

#### Scenario: 查询有历史的服务
- **WHEN** Agent 调用 `decision.similar_history` 且该服务有 20 张已完成工单
- **THEN** 返回结果 SHALL 包含最近 5 张工单的摘要和聚合统计

#### Scenario: 查询无历史的新服务
- **WHEN** Agent 调用 `decision.similar_history` 且该服务尚无已完成工单
- **THEN** 返回结果 SHALL 为 `{"tickets": [], "stats": {"avg_resolution_hours": 0, "total_count": 0}}`

### Requirement: decision.sla_status 工具
该工具 SHALL 返回工单的 SLA 状态和紧急程度评估。

参数：无

返回字段：
- `has_sla`: 是否有 SLA 约束
- `response_remaining_seconds`: 响应时限剩余秒数（负数表示已违约）
- `resolution_remaining_seconds`: 解决时限剩余秒数
- `urgency`: 紧急程度评估（"normal" | "warning" | "critical" | "breached"）
- `sla_status`: 当前 SLA 状态字段值

#### Scenario: SLA 即将违约
- **WHEN** Agent 调用 `decision.sla_status` 且响应时限剩余不足 30 分钟
- **THEN** 返回结果的 `urgency` SHALL 为 `"critical"`

#### Scenario: SLA 已违约
- **WHEN** Agent 调用 `decision.sla_status` 且响应时限已过
- **THEN** 返回结果的 `response_remaining_seconds` SHALL 为负数，`urgency` 为 `"breached"`

#### Scenario: 无 SLA 的工单
- **WHEN** Agent 调用 `decision.sla_status` 且工单未关联 SLA
- **THEN** 返回结果 SHALL 为 `{"has_sla": false, "urgency": "normal"}`

### Requirement: decision.list_actions 工具
该工具 SHALL 列出当前服务可用的自动化动作（ServiceAction）。

参数：无

返回字段：
- `actions`: 动作列表（id, code, name, description）
- `count`: 动作数量

#### Scenario: 服务有可用动作
- **WHEN** Agent 调用 `decision.list_actions` 且服务配置了 3 个活跃的 ServiceAction
- **THEN** 返回结果 SHALL 包含 3 个动作的 id、code、name、description

#### Scenario: 服务无可用动作
- **WHEN** Agent 调用 `decision.list_actions` 且服务未配置任何 ServiceAction
- **THEN** 返回结果 SHALL 为 `{"actions": [], "count": 0}`

### Requirement: Decision tools data access layer
All 8 decision tools SHALL access data through Repository interfaces or dedicated query methods instead of raw `tx.Table()` queries. The `decisionToolContext` struct SHALL hold Repository references instead of a bare `*gorm.DB`.

#### Scenario: ticket_context tool uses Repository
- **WHEN** `decision.ticket_context` is called
- **THEN** it SHALL use `TicketRepo` and `ActivityRepo` methods instead of raw table queries for ticket data, activity history, executed actions, assignments, and parallel groups

#### Scenario: resolve_participant tool uses Repository
- **WHEN** `decision.resolve_participant` is called
- **THEN** it SHALL use `ParticipantResolver` and user lookup via Repository instead of `tx.Table("users")`

#### Scenario: similar_history tool uses Repository
- **WHEN** `decision.similar_history` is called
- **THEN** it SHALL use `TicketRepo.ListCompleted()` or equivalent instead of raw table queries

#### Scenario: Missing Repository methods added
- **WHEN** a decision tool requires a query not currently on the Repository
- **THEN** a new method SHALL be added to the appropriate Repository (e.g., `TicketRepo.GetContextForDecision`, `ActivityRepo.ListCompletedByTicket`)

### Requirement: 工具执行的事务一致性
所有决策工具 SHALL 通过 SmartEngine 在当前决策事务内构建的 Repository 或查询接口执行查询，确保决策期间的数据一致性。

#### Scenario: 工具查询在事务内
- **WHEN** ReAct 循环中 Agent 连续调用多个工具
- **THEN** 所有工具查询 SHALL 使用同一个决策事务上下文，保证读取到一致的数据快照

### Requirement: Decision tools receive context via ToolHandler closure
Decision tools SHALL receive ticket-specific context (ticketID, serviceID, repositories) through the `ToolHandler` closure provided by SmartEngine, rather than through a shared mutable `decisionToolContext` struct.

#### Scenario: Tool handler closure captures ticket context
- **WHEN** SmartEngine builds the `ToolHandler` for a `DecisionRequest`
- **THEN** the closure SHALL capture the current ticket's repositories and IDs, and dispatch to the appropriate tool handler function

### Requirement: 工具错误返回格式
工具执行失败时 SHALL 返回结构化错误 JSON 而非抛出异常，让 Agent 能够理解错误并继续推理。工具参数解析失败时 SHALL 返回明确的参数错误信息。

#### Scenario: 工具执行失败
- **WHEN** 工具查询数据库出错
- **THEN** 工具 SHALL 返回 `{"error": true, "message": "具体错误描述"}`，ReAct 循环将此作为 tool result 追加到消息中

#### Scenario: 工具参数 JSON 解析失败
- **WHEN** Agent 传入的参数 JSON 格式错误（无法 unmarshal）
- **THEN** 工具 SHALL 返回 `{"error": true, "message": "参数格式错误: <具体解析错误>"}`
- **AND** 不得静默使用零值参数

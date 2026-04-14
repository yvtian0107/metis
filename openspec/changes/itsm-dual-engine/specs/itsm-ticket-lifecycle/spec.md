## ADDED Requirements

### Requirement: Ticket 数据模型
系统 SHALL 提供 Ticket 模型，代表一个 ITSM 工单实例。内嵌 BaseModel。字段包括：code（唯一工单编号，系统自动生成，格式 "INC-YYYYMMDD-XXXXX"）、service_id（FK 关联 ServiceDefinition）、engine_type（引擎类型，冗余存储 "classic" | "smart"）、status（工单状态枚举）、priority（优先级："low" | "medium" | "high" | "urgent"）、summary（工单摘要）、form_data（JSON，表单数据）、requester_id（FK 关联申请人 User）、assignee_id（FK 关联当前处理人 User，可为空）、current_activity_id（FK 关联当前活动）、source（来源："catalog" | "agent"）、agent_session_id（智能入口时的 Agent 会话 ID，可为空）、sla_deadline_response（SLA 响应时限 deadline）、sla_deadline_resolve（SLA 解决时限 deadline）、finished_at（完成时间，可为空）。

#### Scenario: 表自动迁移
- **WHEN** ITSM App 启动并执行 AutoMigrate
- **THEN** 系统 SHALL 创建 tickets 表，code 字段有唯一索引，service_id、requester_id、status 有普通索引

#### Scenario: 工单编号自动生成
- **WHEN** 创建新工单
- **THEN** 系统 SHALL 自动生成唯一的 code，格式为 "INC-YYYYMMDD-XXXXX"（XXXXX 为当日递增序号）

#### Scenario: ToResponse 脱敏
- **WHEN** API 返回 Ticket 数据
- **THEN** 系统 SHALL 通过 ToResponse() 方法返回，关联的 requester 和 assignee 仅包含 ID、name、avatar

### Requirement: TicketActivity 数据模型
系统 SHALL 提供 TicketActivity 模型，代表工单流程中的一个活动步骤。内嵌 BaseModel。字段包括：ticket_id（FK 关联 Ticket）、activity_type（活动类型："form" | "approve" | "process" | "action" | "notify" | "wait" | "gateway"）、status（活动状态："pending" | "in_progress" | "completed" | "failed" | "cancelled"）、node_id（经典模式下对应 workflow_json 中的节点 ID，可为空）、ai_decision（JSON，智能模式下 Agent 的 DecisionPlan）、ai_reasoning（文本，AI 决策推理过程）、ai_confidence（浮点数，AI 决策信心分数）、overridden_by（FK 关联覆盖决策的用户 ID，可为空）、form_schema（JSON，该步骤的表单 Schema）、form_data（JSON，该步骤的表单提交数据）、started_at（开始时间）、finished_at（完成时间，可为空）。

#### Scenario: 表自动迁移
- **WHEN** ITSM App 启动并执行 AutoMigrate
- **THEN** 系统 SHALL 创建 ticket_activities 表，ticket_id 有索引

#### Scenario: 经典模式 Activity 记录 node_id
- **WHEN** 经典引擎创建 Activity
- **THEN** Activity 的 node_id SHALL 对应 workflow_json 中的节点 ID，ai_decision/ai_reasoning/ai_confidence 为空

#### Scenario: 智能模式 Activity 记录 AI 决策
- **WHEN** 智能引擎创建 Activity
- **THEN** Activity 的 ai_decision SHALL 存储 DecisionPlan JSON，ai_reasoning 存储推理文本，ai_confidence 存储信心分数，node_id 为空

### Requirement: TicketAssignment 数据模型
系统 SHALL 提供 TicketAssignment 模型，代表活动中的参与人分配记录。内嵌 BaseModel。字段包括：ticket_id（FK 关联 Ticket）、activity_id（FK 关联 TicketActivity）、participant_type（参与人类型："user" | "position" | "department" | "requester_manager"）、user_id（直接指定的用户 ID，可为空）、position_id（职位 ID，可为空）、department_id（部门 ID，可为空）、assignee_id（实际处理人 FK 关联 User，认领后填写）、status（分配状态："pending" | "claimed" | "completed" | "skipped"）、sequence（串行审批时的顺序号）、is_current（串行审批时是否为当前激活项）、claimed_at（认领时间，可为空）、finished_at（完成时间，可为空）。

#### Scenario: 表自动迁移
- **WHEN** ITSM App 启动并执行 AutoMigrate
- **THEN** 系统 SHALL 创建 ticket_assignments 表，ticket_id 和 activity_id 有联合索引

#### Scenario: 单人审批的 Assignment
- **WHEN** 经典引擎创建单人审批 Activity
- **THEN** 系统 SHALL 创建一条 Assignment 记录，assignee_id 直接设为配置的审批人

#### Scenario: 并行审批的 Assignment
- **WHEN** 经典引擎创建并行审批 Activity
- **THEN** 系统 SHALL 为每个参与人创建 Assignment 记录，所有记录的 is_current 均为 true

### Requirement: TicketTimeline 数据模型
系统 SHALL 提供 TicketTimeline 模型，记录工单生命周期中的所有事件。内嵌 BaseModel。字段包括：ticket_id（FK 关联 Ticket）、activity_id（FK 关联 TicketActivity，可为空）、operator_id（FK 关联操作人 User，系统操作时为空）、event_type（事件类型："created" | "assigned" | "claimed" | "submitted" | "approved" | "rejected" | "action_executed" | "escalated" | "overridden" | "cancelled" | "completed" | "comment"）、message（事件描述文本）、details（JSON，事件详细数据）、reasoning（AI 推理记录，可为空）。

#### Scenario: 表自动迁移
- **WHEN** ITSM App 启动并执行 AutoMigrate
- **THEN** 系统 SHALL 创建 ticket_timelines 表，ticket_id 有索引，按 created_at 降序排列

#### Scenario: 工单创建事件
- **WHEN** 新工单创建成功
- **THEN** 系统 SHALL 自动写入一条 event_type 为 "created" 的 Timeline 记录

#### Scenario: AI 决策事件
- **WHEN** 智能引擎完成一次 AI 决策
- **THEN** 系统 SHALL 写入 Timeline 记录，event_type 为对应操作类型，reasoning 字段记录 AI 推理过程

### Requirement: TicketActionExecution 数据模型
系统 SHALL 提供 TicketActionExecution 模型，记录 ServiceAction 的每次执行。内嵌 BaseModel。字段包括：ticket_id（FK 关联 Ticket）、activity_id（FK 关联 TicketActivity）、service_action_id（FK 关联 ServiceAction）、status（执行状态："pending" | "running" | "success" | "failed"）、request_payload（JSON，实际发送的请求内容）、response_payload（JSON，收到的响应内容）、failure_reason（失败原因，可为空）、retry_count（已重试次数）。

#### Scenario: 表自动迁移
- **WHEN** ITSM App 启动并执行 AutoMigrate
- **THEN** 系统 SHALL 创建 ticket_action_executions 表

#### Scenario: 记录成功执行
- **WHEN** ServiceAction HTTP 调用返回 2xx
- **THEN** 系统 SHALL 写入 status 为 "success" 的记录，response_payload 存储响应内容

#### Scenario: 记录失败执行
- **WHEN** ServiceAction HTTP 调用失败
- **THEN** 系统 SHALL 写入 status 为 "failed" 的记录，failure_reason 存储错误信息

### Requirement: 工单状态枚举
Ticket 的 status 字段 SHALL 使用以下枚举值：pending（待处理，刚创建）、in_progress（处理中，有人认领或流程推进中）、waiting_action（等待动作执行结果）、waiting_approval（等待审批或人工确认 AI 决策）、completed（已完成）、failed（失败）、cancelled（已取消）。

#### Scenario: 初始状态
- **WHEN** 工单创建成功
- **THEN** 工单 status SHALL 为 "pending"

#### Scenario: 认领后状态变更
- **WHEN** 处理人认领工单
- **THEN** 工单 status SHALL 变更为 "in_progress"

#### Scenario: 进入审批状态
- **WHEN** 引擎创建审批类型的 Activity
- **THEN** 工单 status SHALL 变更为 "waiting_approval"

#### Scenario: 进入动作等待状态
- **WHEN** 引擎触发 ServiceAction 执行
- **THEN** 工单 status SHALL 变更为 "waiting_action"

#### Scenario: 完成状态
- **WHEN** 引擎到达结束节点或 Agent 决定完成
- **THEN** 工单 status SHALL 变更为 "completed"，finished_at 记录当前时间

### Requirement: 经典入口工单创建
系统 SHALL 支持用户通过服务目录选择经典服务并提交表单来创建工单。

#### Scenario: 表单提交创建工单
- **WHEN** 用户请求 `POST /api/v1/itsm/tickets` 并传入 service_id（经典服务）、summary、form_data
- **THEN** 系统 SHALL 创建 Ticket（source="catalog", engine_type="classic"），计算 SLA deadline，调用 ClassicEngine.Start() 启动流程

#### Scenario: 表单数据校验
- **WHEN** 用户提交的 form_data 不符合服务定义的 form_schema
- **THEN** 系统 SHALL 返回 400 错误，包含字段级校验错误详情

#### Scenario: SLA deadline 计算
- **WHEN** 工单创建时关联服务的 sla_response_hours 为 4、sla_resolve_hours 为 24
- **THEN** 系统 SHALL 设置 sla_deadline_response 为当前时间 +4 小时，sla_deadline_resolve 为当前时间 +24 小时

### Requirement: 智能入口工单创建
系统 SHALL 支持 AI Agent 通过工具调用创建工单。Agent 在对话中识别用户意图后，调用 `itsm.create_ticket` 工具创建工单。

#### Scenario: Agent 工具调用创建工单
- **WHEN** Agent 调用 `itsm.create_ticket` 工具并传入 service_id（智能服务）、summary、form_data
- **THEN** 系统 SHALL 创建 Ticket（source="agent", engine_type="smart", agent_session_id 为当前会话 ID），调用 SmartEngine.Start()

#### Scenario: Agent 自动填充表单
- **WHEN** Agent 从对话上下文中提取了用户信息
- **THEN** Agent SHALL 自动填充 form_data 中可推断的字段，减少用户手动输入

### Requirement: 工单流转
Activity 完成后，系统 SHALL 调用对应引擎的 Progress() 方法推进流程到下一步。

#### Scenario: 经典流程流转
- **WHEN** 经典工单的当前 Activity 状态变为 "completed"
- **THEN** 系统 SHALL 调用 ClassicEngine.Progress()，根据 outcome 和边的定义创建下一个 Activity

#### Scenario: 智能流程流转
- **WHEN** 智能工单的当前 Activity 状态变为 "completed"
- **THEN** 系统 SHALL 调用 SmartEngine.Progress()，触发新一轮 AI 决策循环

#### Scenario: 流转失败回滚
- **WHEN** Engine.Progress() 执行过程中发生错误
- **THEN** 系统 SHALL 将当前 Activity 标记为 "failed"，工单状态设为 "failed"，记录错误到 Timeline

### Requirement: 工单取消
系统 SHALL 支持申请人或管理员取消工单。

#### Scenario: 申请人取消
- **WHEN** 申请人请求 `POST /api/v1/itsm/tickets/:id/cancel` 且工单 status 不为 "completed" 或 "cancelled"
- **THEN** 系统 SHALL 调用 Engine.Cancel()，工单状态更新为 "cancelled"

#### Scenario: 管理员取消
- **WHEN** 管理员请求 `POST /api/v1/itsm/tickets/:id/cancel`
- **THEN** 系统 SHALL 调用 Engine.Cancel()，无论工单状态如何（已完成除外）

#### Scenario: 已完成工单不可取消
- **WHEN** 用户尝试取消 status 为 "completed" 的工单
- **THEN** 系统 SHALL 返回 400 错误，提示已完成工单不可取消

### Requirement: 派单机制
系统 SHALL 根据引擎类型采用不同的派单策略。

#### Scenario: 经典模式派单
- **WHEN** 经典引擎创建包含 participants 配置的 Activity
- **THEN** 系统 SHALL 根据参与人规则解析候选人，创建 TicketAssignment 记录

#### Scenario: 智能模式派单
- **WHEN** 智能引擎的 DecisionPlan 中指定了参与人
- **THEN** 系统 SHALL 按 Agent 决策创建 TicketAssignment 记录

#### Scenario: 无候选人告警
- **WHEN** 参与人规则解析后无可用候选人
- **THEN** 系统 SHALL 将 Activity 记录为 "pending" 状态并发送通知告警管理员

### Requirement: 工单认领
系统 SHALL 支持处理人从待办列表中认领分配给自己的工单。

#### Scenario: 认领 Assignment
- **WHEN** 处理人请求 `POST /api/v1/itsm/tickets/:id/claim`
- **THEN** 系统 SHALL 将匹配的 TicketAssignment 的 status 更新为 "claimed"，assignee_id 设为当前用户，claimed_at 设为当前时间

#### Scenario: 已被认领的 Assignment
- **WHEN** 处理人尝试认领已被其他人认领的 Assignment
- **THEN** 系统 SHALL 返回 409 错误，提示该工单已被他人认领

#### Scenario: 认领后工单状态变更
- **WHEN** 工单的某个 Assignment 被认领且工单当前 status 为 "pending"
- **THEN** 工单 status SHALL 变更为 "in_progress"

### Requirement: 工单列表与搜索
系统 SHALL 提供工单列表 API，支持分页、筛选和关键词搜索。

#### Scenario: 工单列表查询
- **WHEN** 用户请求 `GET /api/v1/itsm/tickets` 并可选传入 status、priority、service_id、engine_type、requester_id、assignee_id、keyword、page、page_size
- **THEN** 系统 SHALL 返回分页的工单列表，包含关联的服务名称、申请人名称、处理人名称

#### Scenario: 关键词搜索
- **WHEN** 用户传入 keyword 参数
- **THEN** 系统 SHALL 按 code 和 summary 字段模糊匹配

#### Scenario: 默认排序
- **WHEN** 用户未指定排序参数
- **THEN** 系统 SHALL 按 created_at 降序排列

### Requirement: 工单详情
系统 SHALL 提供工单详情 API 和页面，展示完整的工单信息。

#### Scenario: 工单详情 API
- **WHEN** 用户请求 `GET /api/v1/itsm/tickets/:id`
- **THEN** 系统 SHALL 返回工单完整信息，包含服务定义、所有 Activity 列表、当前 Assignment 列表、Timeline 事件列表

#### Scenario: 工单详情页面
- **WHEN** 用户访问 `/itsm/tickets/:id`
- **THEN** 系统 SHALL 展示工单详情页，包含：工单基本信息卡片、时间线面板、流程图面板（经典/智能各自渲染）、当前步骤的操作按钮

#### Scenario: 操作按钮渲染
- **WHEN** 用户是当前 Activity 的参与人且 Assignment status 为 "claimed"
- **THEN** 系统 SHALL 根据 activity_type 展示对应操作按钮（提交表单/审批通过/审批驳回/完成处理）

### Requirement: 我的工单视图
系统 SHALL 提供"我的工单"视图，展示当前用户作为申请人提交的所有工单。

#### Scenario: 我的工单列表
- **WHEN** 用户请求 `GET /api/v1/itsm/tickets?requester_id=me`（me 为特殊值，解析为当前用户）
- **THEN** 系统 SHALL 返回当前用户作为 requester_id 的所有工单

#### Scenario: 我的工单页面
- **WHEN** 用户访问 `/itsm/my-tickets`
- **THEN** 系统 SHALL 展示当前用户提交的工单列表，支持按状态筛选

### Requirement: 我的待办视图
系统 SHALL 提供"我的待办"视图，展示当前用户需要处理的所有 Assignment。

#### Scenario: 我的待办列表
- **WHEN** 用户请求 `GET /api/v1/itsm/tickets/my-todo`
- **THEN** 系统 SHALL 返回当前用户有 pending 或 claimed 状态的 TicketAssignment 关联的工单列表

#### Scenario: 我的待办页面
- **WHEN** 用户访问 `/itsm/my-todo`
- **THEN** 系统 SHALL 展示待办工单列表，区分"待认领"和"处理中"两个分组

#### Scenario: 待办徽标提示
- **WHEN** 用户有未认领的待办项
- **THEN** 系统 SHALL 在导航菜单的"我的待办"入口显示未读数量徽标

### Requirement: API 路由注册
ITSM App 的工单相关 API SHALL 注册在 `/api/v1/itsm/tickets/*` 前缀下，使用 JWT + Casbin 中间件保护。

#### Scenario: 工单 CRUD API
- **WHEN** 请求发送到 `/api/v1/itsm/tickets/*`
- **THEN** 系统 SHALL 路由到 TicketHandler 处理，支持创建、查询、详情、取消、认领、流转操作

#### Scenario: 工单操作 API
- **WHEN** 请求发送到 `/api/v1/itsm/tickets/:id/submit`、`/approve`、`/reject`、`/claim`、`/cancel`、`/signal`
- **THEN** 系统 SHALL 路由到对应的操作处理方法，校验用户权限后执行

#### Scenario: 我的待办 API
- **WHEN** 请求发送到 `/api/v1/itsm/tickets/my-todo`
- **THEN** 系统 SHALL 查询当前用户的待办 Assignment 列表并返回

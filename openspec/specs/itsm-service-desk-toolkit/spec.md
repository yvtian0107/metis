## Purpose

ITSM 服务台工具链，为 IT 服务台智能体提供完整的提单引导流程工具集。工具按 match -> confirm -> load -> draft -> confirm -> create 的状态机流程设计，通过 AgentSession.State 管理多轮对话状态。
## Requirements
### Requirement: itsm.service_match 工具
系统 SHALL 注册 `itsm.service_match` 工具（toolkit: "itsm"），用于根据用户自然语言描述匹配 0-3 个候选服务。匹配基于关键词权重评分，返回置信度和是否需要用户确认。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "用户描述的需求（自然语言）" }
  },
  "required": ["query"]
}
```

**返回结构**:
```json
{
  "query": "申请VPN",
  "matches": [
    { "id": 1, "name": "VPN 接入申请", "catalog_path": "账号与权限/网络接入", "description": "...", "score": 0.85, "reason": "名称关键词匹配" }
  ],
  "confirmation_required": false,
  "selected_service_id": 1
}
```

#### Scenario: 单个高置信匹配
- **WHEN** Agent 调用 itsm.service_match，输入 query 匹配到 1 个服务且 score >= 0.8
- **THEN** 系统 SHALL 返回该服务，`confirmation_required=false`，`selected_service_id` 设为该服务 ID

#### Scenario: 多个候选需确认
- **WHEN** Agent 调用 itsm.service_match，匹配到 2-3 个候选且最高分与次高分差距 < 0.1
- **THEN** 系统 SHALL 返回所有候选，`confirmation_required=true`，`selected_service_id=null`

#### Scenario: 无匹配
- **WHEN** Agent 调用 itsm.service_match，无任何服务匹配
- **THEN** 系统 SHALL 返回空 matches 列表，`confirmation_required=false`

#### Scenario: 匹配结果写入会话状态
- **WHEN** itsm.service_match 返回结果
- **THEN** 系统 SHALL 将候选列表、top_match_service_id、confirmation_required 写入 AgentSession.State，stage 更新为 `candidates_ready`

### Requirement: itsm.service_confirm 工具
系统 SHALL 注册 `itsm.service_confirm` 工具，用于在多候选场景下锁定用户选择的服务。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "service_id": { "type": "integer", "description": "用户选择的服务 ID（必须在 service_match 返回的候选列表中）" }
  },
  "required": ["service_id"]
}
```

#### Scenario: 确认有效候选
- **WHEN** Agent 调用 itsm.service_confirm，service_id 在当前会话候选列表中
- **THEN** 系统 SHALL 锁定该服务，更新 Session.State 的 confirmed_service_id，stage 更新为 `service_selected`

#### Scenario: 确认无效候选
- **WHEN** Agent 调用 itsm.service_confirm，service_id 不在当前会话候选列表中
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "该服务不在当前候选列表中"}`

#### Scenario: 无候选状态下调用
- **WHEN** Agent 调用 itsm.service_confirm，但会话 stage 不是 `candidates_ready`
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "请先调用 service_match 获取候选服务"}`

### Requirement: itsm.service_load 工具
系统 SHALL 注册 `itsm.service_load` 工具，用于加载指定服务的完整信息（协作规范、表单定义、动作配置、路由提示）。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "service_id": { "type": "integer", "description": "服务定义 ID" }
  },
  "required": ["service_id"]
}
```

**返回结构**:
```json
{
  "ok": true,
  "service_id": 1,
  "name": "VPN 接入申请",
  "collaboration_spec": "## 协作规范\n...",
  "form_fields": [ { "key": "vpn_type", "label": "VPN 类型", "type": "select", "required": true, "options": [] } ],
  "actions": [ { "id": 1, "code": "check_vpn", "name": "VPN 检查" } ],
  "routing_field_hint": { "field_key": "vpn_type", "option_route_map": { "临时远程": "network_admin", "长期远程": "security_admin" } }
}
```

#### Scenario: 加载已确认的服务
- **WHEN** Agent 调用 itsm.service_load，service_id 与 Session.State 中的 confirmed_service_id 或 selected_service_id 一致
- **THEN** 系统 SHALL 返回服务完整信息，更新 Session.State 的 loaded_service_id 和 fields_hash，stage 更新为 `service_loaded`

#### Scenario: 需确认但未确认时加载
- **WHEN** Agent 调用 itsm.service_load，Session.State 的 confirmation_required=true 但 confirmed_service_id 为空
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "请先调用 service_confirm 确认服务选择"}`

#### Scenario: 提取路由字段提示
- **WHEN** 服务的 workflow_json 包含基于表单字段的条件分支
- **THEN** 系统 SHALL 解析 workflow_json 提取 routing_field_hint，包含 field_key 和 option_route_map

### Requirement: itsm.new_request 工具
系统 SHALL 注册 `itsm.new_request` 工具，用于在同一会话内重置上下文以开始新的工单申请。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {}
}
```

#### Scenario: 重置会话状态
- **WHEN** Agent 调用 itsm.new_request
- **THEN** 系统 SHALL 清空 Session.State 中的所有服务台状态（候选列表、已选服务、草稿数据等），stage 重置为 `idle`

#### Scenario: 无状态时调用
- **WHEN** Agent 调用 itsm.new_request，但会话尚无服务台状态
- **THEN** 系统 SHALL 返回 `{"ok": true, "message": "已就绪，请描述您的需求"}`

### Requirement: itsm.draft_prepare 工具
系统 SHALL 注册 `itsm.draft_prepare` 工具，用于在向用户展示草稿前登记当前版本并校验表单字段。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "summary": { "type": "string", "description": "工单摘要" },
    "form_data": { "type": "object", "description": "表单字段键值对（必须是完整表单，不能只传增量）" }
  },
  "required": ["summary", "form_data"]
}
```

**返回结构**:
```json
{
  "ok": true,
  "draft_version": 2,
  "summary": "申请VPN临时接入",
  "form_data": { "vpn_type": "临时远程", "duration": "2026-04-16 20:00:00~22:00:00" },
  "warnings": []
}
```

#### Scenario: 成功登记草稿
- **WHEN** Agent 调用 itsm.draft_prepare，传入完整的 summary 和 form_data，服务已加载
- **THEN** 系统 SHALL 校验 form_data 中的必填字段、选项值合法性，自增 draft_version，更新 Session.State，stage 更新为 `awaiting_confirmation`

#### Scenario: 必填字段缺失
- **WHEN** form_data 中缺少服务表单定义的必填字段
- **THEN** 系统 SHALL 在 warnings 中返回缺失字段列表 `[{"type": "missing_required", "field_key": "vpn_type", "field_label": "VPN 类型"}]`

#### Scenario: 无效选项值
- **WHEN** form_data 中某个 select 字段的值不在选项列表中
- **THEN** 系统 SHALL 在 warnings 中返回 `[{"type": "invalid_option", "field_key": "vpn_type", "value": "xxx", "valid_options": ["临时远程", "长期远程"]}]`

#### Scenario: 单选字段传入多值且为路由字段
- **WHEN** form_data 中某个单选字段传入了逗号分隔的多个值
- **AND** 该字段是 RoutingFieldHint.FieldKey（路由决策字段）
- **THEN** 系统 SHALL 在 warnings 中返回 `multivalue_on_single_field` 类型警告，附带 `resolved_values` 数组，每个元素包含 `value`（原始值）和 `route`（该值在 OptionRouteMap 中对应的路由分支标签）
- **AND** 若某个值不在 OptionRouteMap 中，`route` SHALL 为空字符串

#### Scenario: 单选字段传入多值但非路由字段
- **WHEN** form_data 中某个单选字段传入了逗号分隔的多个值
- **AND** 该字段不是路由决策字段
- **THEN** 系统 SHALL 在 warnings 中返回 `multivalue_on_single_field` 类型警告，不包含 `resolved_values`

#### Scenario: 服务未加载时调用
- **WHEN** Agent 调用 itsm.draft_prepare 但 Session.State 中无 loaded_service_id
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "请先调用 service_load 加载服务详情"}`

#### Scenario: 草稿内容变更自增版本
- **WHEN** 本次传入的 summary 或 form_data 与上一次 draft_prepare 不同
- **THEN** 系统 SHALL 自增 draft_version，将 confirmed_draft_version 清空（需要重新确认）

### Requirement: itsm.draft_confirm 工具
系统 SHALL 注册 `itsm.draft_confirm` 工具，用于在用户明确确认草稿后标记确认状态。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {}
}
```

#### Scenario: 确认当前草稿
- **WHEN** Agent 调用 itsm.draft_confirm，Session.State 处于 `awaiting_confirmation` stage
- **THEN** 系统 SHALL 将 confirmed_draft_version 设为当前 draft_version，返回 `{"ok": true, "draft_version": 2, "confirmed_draft_version": 2}`

#### Scenario: 非等待确认状态
- **WHEN** Agent 调用 itsm.draft_confirm，Session.State 不处于 `awaiting_confirmation`
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "请先调用 draft_prepare 生成草稿"}`

#### Scenario: 表单定义变更检测
- **WHEN** Agent 调用 itsm.draft_confirm，但服务的表单定义已发生变更（fields_hash 不匹配）
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "服务表单定义已变更，请重新调用 service_load 和 draft_prepare"}`

### Requirement: itsm.validate_participants 工具
系统 SHALL 提供参与人预检能力（通过 `ValidateParticipants` 方法），在提单前校验工作流中各人工节点的参与人是否可解析。该方法 SHALL 复用 `engine.ParticipantResolver` 而非独立解析 workflow_json。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "service_id": { "type": "integer", "description": "服务定义 ID" },
    "form_data": { "type": "object", "description": "表单数据（用于确定路由分支）" }
  },
  "required": ["service_id", "form_data"]
}
```

#### Scenario: ValidateParticipants 复用 ParticipantResolver
- **WHEN** `ValidateParticipants` 被调用，workflow_json 中有一个 process 节点配置了 `position_department` 类型参与人
- **THEN** 方法 SHALL 通过 `ParticipantResolver.Resolve()` 验证该参与人是否可解析，而非自行查询 org 表

#### Scenario: requester 类型在提单前跳过
- **WHEN** `ValidateParticipants` 遍历到 requester 类型参与人
- **THEN** 方法 SHALL 跳过该参与人的验证（提单前不知道 requester 是谁）

#### Scenario: position_department 参与人无可用人员
- **WHEN** `ValidateParticipants` 验证一个 position_department 参与人，`ParticipantResolver.Resolve()` 返回空用户列表
- **THEN** 方法 SHALL 返回 `ParticipantValidation{OK: false, FailureReason: "岗位[xxx]+部门[xxx] 下无可用人员", NodeLabel: "节点名"}`

#### Scenario: user 类型参与人不存在
- **WHEN** `ValidateParticipants` 验证一个 user 类型参与人，`ParticipantResolver.Resolve()` 返回空列表或错误
- **THEN** 方法 SHALL 返回 `ParticipantValidation{OK: false, FailureReason: "指定用户不存在或已停用"}`

#### Scenario: form 节点参与人也被校验
- **WHEN** `ValidateParticipants` 遍历 workflow_json 中的节点
- **THEN** 方法 SHALL 同时校验 process 和 form 类型节点的参与人（当前仅校验 process）

#### Scenario: Org App 不可用时跳过岗位校验
- **WHEN** `ValidateParticipants` 执行时 `ParticipantResolver` 的 orgResolver 为 nil
- **THEN** position/position_department 类型参与人的校验 SHALL 被跳过，不视为错误

#### Scenario: 消除独立 workflow 解析逻辑
- **WHEN** `ValidateParticipants` 执行
- **THEN** 方法 SHALL NOT 包含独立的 workflow JSON 解析和参与人结构体定义（如 `workflowParticipant`），SHALL 复用 `engine.ParseWorkflowDef` 和 `engine.Participant` 类型

### Requirement: itsm.ticket_create 工具（增强版）
系统 SHALL 注册增强版 `itsm.ticket_create` 工具，带有前置条件检查，必须经过完整的 match->load->draft->confirm 流程后才能创建工单。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "service_id": { "type": "integer", "description": "服务定义 ID" },
    "summary": { "type": "string", "description": "工单摘要" },
    "form_data": { "type": "object", "description": "完整表单数据" }
  },
  "required": ["service_id", "summary"]
}
```

#### Scenario: 前置条件满足时创建
- **WHEN** Agent 调用 itsm.ticket_create，Session.State 满足：loaded_service_id == service_id 且 confirmed_draft_version == draft_version
- **THEN** 系统 SHALL 创建工单，source 设为 `"agent"`，关联 agent_session_id，返回 `{"ok": true, "ticket_id": 123, "ticket_code": "ITSM-20260416-0001", "status": "pending"}`

#### Scenario: 草稿未确认
- **WHEN** Agent 调用 itsm.ticket_create，Session.State 的 confirmed_draft_version 为空或与 draft_version 不一致
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "请先调用 draft_confirm 确认草稿"}`

#### Scenario: 服务未加载
- **WHEN** Agent 调用 itsm.ticket_create，Session.State 的 loaded_service_id 为空
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "请先调用 service_load 加载服务详情"}`

#### Scenario: 创建后重置状态
- **WHEN** 工单创建成功
- **THEN** 系统 SHALL 重置 Session.State 到 idle stage，为下一次提单做准备

### Requirement: Agent ticket creation uses TicketService
The `Operator.CreateTicket` method SHALL delegate to a `TicketCreator` interface implemented by `TicketService`, instead of directly inserting into the database. This ensures Agent-created tickets receive the same processing as UI-created tickets.

#### Scenario: Agent-created ticket starts workflow engine
- **WHEN** an Agent creates a ticket via `itsm.ticket_create` tool
- **THEN** the ticket SHALL have its workflow engine started (classic or smart), SLA deadlines calculated, and timeline events recorded, identical to a ticket created via the HTTP API

#### Scenario: Agent-created ticket has SLA deadlines
- **WHEN** an Agent creates a ticket for a service with an SLA template
- **THEN** the ticket SHALL have `sla_response_deadline` and `sla_resolution_deadline` populated

#### Scenario: Agent-created ticket records timeline
- **WHEN** an Agent creates a ticket
- **THEN** a `ticket_created` timeline event SHALL be recorded

### Requirement: itsm.my_tickets 工具
系统 SHALL 注册 `itsm.my_tickets` 工具，查询当前用户的进行中工单列表，包含撤回资格判断。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "status": { "type": "string", "description": "状态筛选（可选）" }
  }
}
```

#### Scenario: 查询我的工单
- **WHEN** Agent 调用 itsm.my_tickets
- **THEN** 系统 SHALL 返回当前用户的非终态工单列表，每项包含 ticket_id、ticket_code、summary、status、service_name、created_at、can_withdraw

#### Scenario: can_withdraw 判断
- **WHEN** 工单状态为 pending 且无已完成的 activity
- **THEN** 该工单的 can_withdraw SHALL 为 true

### Requirement: itsm.ticket_withdraw 工具
系统 SHALL 注册 `itsm.ticket_withdraw` 工具，用于撤回尚未处理的工单。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "ticket_code": { "type": "string", "description": "工单编号" },
    "reason": { "type": "string", "description": "撤回原因（可选）" }
  },
  "required": ["ticket_code"]
}
```

#### Scenario: 成功撤回
- **WHEN** Agent 调用 itsm.ticket_withdraw，当前用户是工单提交人且工单尚未被处理
- **THEN** 系统 SHALL 取消工单，返回 `{"ok": true, "ticket_code": "ITSM-20260416-0001"}`

#### Scenario: 已处理不可撤回
- **WHEN** Agent 调用 itsm.ticket_withdraw，工单已有已完成的 activity
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "工单已进入处理流程，无法撤回"}`

#### Scenario: 非提交人不可撤回
- **WHEN** Agent 调用 itsm.ticket_withdraw，当前用户不是工单提交人
- **THEN** 系统 SHALL 返回错误 `{"ok": false, "error": "仅工单提交人可撤回"}`

### Requirement: 服务台会话状态管理
AgentSession 状态模型 SHALL 扩展为对话式提单状态机，支持 `missing_fields`、`asked_fields`、`min_decision_ready` 等字段，以驱动增量追问与最小可决策提交。系统 MUST 在每轮工具调用后更新并持久化状态，且不得重复追问已确认字段。

**状态结构扩展**:
```json
{
  "stage": "idle|candidates_ready|service_selected|service_loaded|awaiting_confirmation|confirmed",
  "candidate_service_ids": [1, 2],
  "top_match_service_id": 1,
  "confirmed_service_id": null,
  "loaded_service_id": null,
  "confirmation_required": false,
  "draft_summary": "",
  "draft_form_data": {},
  "draft_version": 0,
  "confirmed_draft_version": 0,
  "fields_hash": "",
  "missing_fields": [],
  "asked_fields": [],
  "min_decision_ready": false
}
```

#### Scenario: 追问缺失字段并推进状态
- **WHEN** 用户输入请求后存在关键字段缺失
- **THEN** 系统 SHALL 在会话状态记录 missing_fields 并发起下一轮追问

#### Scenario: 达到最小可决策条件
- **WHEN** 会话状态满足 min_decision_ready=true
- **THEN** 工具链 SHALL 允许进入 draft_confirm 与 ticket_create

#### Scenario: 已确认字段不重复追问
- **WHEN** 字段已存在于 asked_fields 且已确认
- **THEN** 后续工具调用 SHALL 跳过该字段追问

#### Scenario: 状态随工具调用自动推进
- **WHEN** 工具按顺序调用 service_match -> service_confirm -> service_load -> draft_prepare -> draft_confirm -> ticket_create
- **THEN** Session.State 的 stage SHALL 依次推进：idle -> candidates_ready -> service_selected -> service_loaded -> awaiting_confirmation -> confirmed -> idle

#### Scenario: 状态持久化
- **WHEN** 工具更新 Session.State
- **THEN** 系统 SHALL 将 State 序列化为 JSON 并持久化到 AgentSession 记录

#### Scenario: 工具前置条件校验
- **WHEN** 任何工具被调用
- **THEN** 系统 SHALL 从 Session.State 中读取当前 stage，校验是否满足该工具的前置条件

### Requirement: ServiceDeskState explicit state machine
The `ServiceDeskState` stage transitions SHALL be validated by a centralized transition table. Invalid transitions SHALL return an error.

#### Scenario: Valid stage transition
- **WHEN** the state is `candidates_ready` and a transition to `service_selected` is requested
- **THEN** the transition SHALL succeed

#### Scenario: Invalid stage transition
- **WHEN** the state is `idle` and a transition to `confirmed` is requested
- **THEN** the transition SHALL return an error indicating the invalid transition

#### Scenario: Reset to idle always allowed
- **WHEN** a transition to `idle` is requested from any stage
- **THEN** the transition SHALL succeed (reset via `itsm.new_request`)

### Requirement: System prompt seed synchronization
The `SeedAgents` function SHALL update the system prompt of preset agents on every `seed.Sync()` invocation, matching agents by their `code` field. User-created agents (without a preset code) SHALL NOT be affected.

#### Scenario: Prompt updated on restart
- **WHEN** the application restarts and `seed.Sync()` runs
- **THEN** preset agents (matched by code) SHALL have their `system_prompt` updated to the latest version defined in code

#### Scenario: Non-preset agents unaffected
- **WHEN** a user creates a custom agent without a preset code
- **THEN** `seed.Sync()` SHALL NOT modify that agent's system prompt

### Requirement: Typed context keys for session ID
The `ai_session_id` context key SHALL use a typed `contextKey` type instead of a plain string, defined in a shared location accessible to both AI App and ITSM tools.

#### Scenario: Session ID injection uses typed key
- **WHEN** `CompositeToolExecutor` injects `ai_session_id` into context
- **THEN** it SHALL use the typed context key

#### Scenario: ITSM tool handlers read typed key
- **WHEN** ITSM tool handlers read `ai_session_id` from context
- **THEN** they SHALL use the same typed context key

### Requirement: ITSM Service Desk uses shared Chat Workspace
The ITSM Service Desk frontend SHALL use the shared Chat Workspace for the service intake conversation. It SHALL not provide a separate composer, message list, scroll manager, stop button, header, or data surface parser outside Chat Workspace configuration and registered ITSM renderers.

#### Scenario: Service desk conversation opens
- **WHEN** a user opens ITSM Service Desk with a configured service intake agent
- **THEN** the conversation SHALL render with the shared Chat Workspace shell
- **AND** the header SHALL identify the service intake agent using the shared AgentSwitcher pattern

#### Scenario: Service desk sends image context
- **WHEN** a user includes an image in an ITSM Service Desk message
- **THEN** the image SHALL be uploaded and sent through the shared Agent Session image message flow
- **AND** the service desk agent SHALL receive the message in the same session context as text-only messages

### Requirement: ITSM draft form surface renderer
The ITSM Service Desk frontend SHALL register an `itsm.draft_form` surface renderer with Chat Workspace. The renderer SHALL support draft loading, editable confirmation form, inline submit error, submitted state, ticket code display, and workspace invalidation after successful submission.

#### Scenario: Draft loading surface
- **WHEN** the stream contains an `itsm.draft_form` surface with loading status
- **THEN** the registered renderer SHALL show a draft preparation state inside the assistant response area

#### Scenario: Editable draft confirmation
- **WHEN** the stream contains an `itsm.draft_form` surface with a valid form schema
- **THEN** the registered renderer SHALL display the service form as an editable confirmation UI
- **AND** submitting it SHALL call the ITSM draft submit API with the current draft version and form data

#### Scenario: Submitted ticket result
- **WHEN** the draft submit API returns a submitted ticket result
- **THEN** the registered renderer SHALL show the submitted state and ticket code in the assistant response area
- **AND** the service desk workspace SHALL refresh affected session and ticket queries

### Requirement: Service desk not-on-duty state remains business-specific
When no service intake agent is configured, ITSM Service Desk SHALL use a business-specific not-on-duty state that leads the user to Smart Staffing configuration. This state MAY be injected into Chat Workspace as an empty or unavailable state but SHALL use shared chat visual language.

#### Scenario: No intake agent configured
- **WHEN** the service intake post has no configured agent
- **THEN** ITSM Service Desk SHALL show an unavailable state that clearly explains the service intake agent is not configured
- **AND** the primary action SHALL lead to Smart Staffing configuration


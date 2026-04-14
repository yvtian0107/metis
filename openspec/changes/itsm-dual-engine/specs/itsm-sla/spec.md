## ADDED Requirements

### Requirement: SLATemplate 数据模型
系统 SHALL 维护 sla_templates 表，包含以下字段：name（模板名称，唯一）、code（模板编码，唯一）、response_time（响应时间，分钟）、resolution_time（解决时间，分钟）、is_active（是否启用）、description（描述）。嵌入 BaseModel 提供 id、created_at、updated_at、deleted_at。

#### Scenario: 创建 SLA 模板
- **WHEN** 管理员请求 POST /api/v1/itsm/sla/templates，body 包含 name="标准服务"、code="standard"、response_time=30、resolution_time=480
- **THEN** 系统 SHALL 创建 SLA 模板记录并返回模板 ID

#### Scenario: 模板编码唯一性校验
- **WHEN** 管理员尝试创建与已有模板相同 code 的 SLA 模板
- **THEN** 系统 SHALL 返回 409 错误，提示编码已存在

#### Scenario: 禁用 SLA 模板
- **WHEN** 管理员将某 SLA 模板的 is_active 设为 false
- **THEN** 该模板 SHALL 不再可被新服务绑定，但已绑定的服务不受影响

### Requirement: SLA 模板 CRUD API
系统 SHALL 提供完整的 SLA 模板管理 API，API 前缀为 /api/v1/itsm/sla/templates。

#### Scenario: 列表查询
- **WHEN** 管理员请求 GET /api/v1/itsm/sla/templates，传入 page、pageSize、keyword 参数
- **THEN** 系统 SHALL 返回分页结果，包含模板名称、编码、响应时间、解决时间、启用状态

#### Scenario: 单条查询
- **WHEN** 管理员请求 GET /api/v1/itsm/sla/templates/:id
- **THEN** 系统 SHALL 返回模板详情，包含关联的升级策略列表

#### Scenario: 更新模板
- **WHEN** 管理员请求 PUT /api/v1/itsm/sla/templates/:id，修改 response_time 和 resolution_time
- **THEN** 系统 SHALL 更新模板，已有工单的 deadline 不受影响（仅新工单使用新值）

#### Scenario: 删除模板
- **WHEN** 管理员请求 DELETE /api/v1/itsm/sla/templates/:id，且该模板已被服务绑定
- **THEN** 系统 SHALL 返回 400 错误，提示模板正在使用中不可删除

### Requirement: EscalationRule 升级策略模型
系统 SHALL 维护 escalation_rules 表，包含以下字段：sla_template_id（关联的 SLA 模板）、trigger_type（触发条件：response_timeout / resolution_timeout）、trigger_minutes（触发时间，相对于 deadline 的提前/延后分钟数）、action_type（升级动作：notify / reassign / escalate_priority）、action_config（动作配置，JSON）、sort_order（排序，同模板同触发类型下的执行顺序）。嵌入 BaseModel。

#### Scenario: 创建响应超时通知策略
- **WHEN** 管理员为 SLA 模板创建升级策略，trigger_type="response_timeout"、trigger_minutes=0、action_type="notify"、action_config 包含 channel_id 和 notify_targets
- **THEN** 系统 SHALL 创建升级策略记录，表示响应时间到期时通过指定通道通知指定人员

#### Scenario: 创建解决超时改派策略
- **WHEN** 管理员创建升级策略，trigger_type="resolution_timeout"、trigger_minutes=0、action_type="reassign"、action_config 包含 target_user_id 或 target_role
- **THEN** 系统 SHALL 创建升级策略记录，表示解决时间到期时自动改派给指定处理人或角色

#### Scenario: 创建优先级提升策略
- **WHEN** 管理员创建升级策略，trigger_type="resolution_timeout"、trigger_minutes=30、action_type="escalate_priority"
- **THEN** 系统 SHALL 创建升级策略记录，表示解决时间超期 30 分钟后自动提升工单优先级

### Requirement: 升级策略 CRUD API
系统 SHALL 提供升级策略管理 API，API 前缀为 /api/v1/itsm/sla/templates/:templateId/rules。

#### Scenario: 查询模板的升级策略
- **WHEN** 管理员请求 GET /api/v1/itsm/sla/templates/:templateId/rules
- **THEN** 系统 SHALL 返回该模板下的全部升级策略，按 trigger_type 和 sort_order 排序

#### Scenario: 批量保存升级策略
- **WHEN** 管理员请求 PUT /api/v1/itsm/sla/templates/:templateId/rules，body 包含完整的策略列表
- **THEN** 系统 SHALL 全量替换该模板的升级策略（删除旧的，创建新的）

### Requirement: 工单创建时计算 SLA deadline
系统 SHALL 在工单创建时，根据服务绑定的 SLA 模板自动计算 response_deadline 和 resolution_deadline。

#### Scenario: 服务绑定了 SLA 模板
- **WHEN** 用户创建工单，所选服务绑定了 response_time=30、resolution_time=480 的 SLA 模板
- **THEN** 系统 SHALL 设置工单的 response_deadline 为当前时间 +30 分钟，resolution_deadline 为当前时间 +480 分钟，sla_status 为 "on_track"

#### Scenario: 服务未绑定 SLA 模板
- **WHEN** 用户创建工单，所选服务未绑定 SLA 模板
- **THEN** 系统 SHALL 不设置 deadline 字段，sla_status 保持为空

#### Scenario: 智能服务创建的工单同样适用 SLA
- **WHEN** Agent 通过 itsm.create_ticket 工具创建智能服务工单，该服务绑定了 SLA 模板
- **THEN** 系统 SHALL 同样计算并设置 response_deadline 和 resolution_deadline

### Requirement: SLA 检查定时任务
系统 SHALL 注册一个 Scheduler 定时任务 `itsm-sla-check`（cron: `* * * * *`，每分钟执行），扫描所有未完结且有 SLA deadline 的工单，检查是否超时并触发升级。

#### Scenario: 检查响应超时
- **WHEN** 定时任务执行，发现工单的 response_deadline 已过期且 sla_status 为 "on_track"，且工单尚未被认领（无 assignee 或 assignee 未确认）
- **THEN** 系统 SHALL 将 sla_status 更新为 "breached_response"，并触发该 SLA 模板中 trigger_type="response_timeout" 的升级策略

#### Scenario: 检查解决超时
- **WHEN** 定时任务执行，发现工单的 resolution_deadline 已过期且工单状态不为 "completed" / "cancelled"
- **THEN** 系统 SHALL 将 sla_status 更新为 "breached_resolution"，并触发该 SLA 模板中 trigger_type="resolution_timeout" 的升级策略

#### Scenario: 已完结工单跳过检查
- **WHEN** 定时任务执行，工单状态为 "completed" 或 "cancelled"
- **THEN** 系统 SHALL 跳过该工单的 SLA 检查

#### Scenario: 无 SLA 工单跳过检查
- **WHEN** 定时任务执行，工单的 response_deadline 和 resolution_deadline 均为空
- **THEN** 系统 SHALL 跳过该工单

### Requirement: 响应超时升级逻辑
系统 SHALL 在工单首次派单后，如果在 response_deadline 前无人认领，触发响应超时升级。

#### Scenario: 派单后及时认领
- **WHEN** 工单被派给处理人，处理人在 response_deadline 前确认认领（状态从 pending → in_progress）
- **THEN** 系统 SHALL 不触发响应超时升级，sla_status 保持 "on_track"

#### Scenario: 派单后未及时认领
- **WHEN** 工单被派给处理人，但在 response_deadline 过期时处理人仍未认领
- **THEN** 系统 SHALL 触发响应超时升级，执行配置的升级动作

### Requirement: 解决超时升级逻辑
系统 SHALL 在工单创建后，如果在 resolution_deadline 前未完结，触发解决超时升级。

#### Scenario: 在 deadline 前完结
- **WHEN** 工单在 resolution_deadline 前状态变为 "completed"
- **THEN** 系统 SHALL 不触发解决超时升级，sla_status 保持 "on_track"

#### Scenario: 超过 deadline 未完结
- **WHEN** 工单的 resolution_deadline 已过，且工单仍在处理中
- **THEN** 系统 SHALL 触发解决超时升级，按 sort_order 依次执行配置的升级动作

### Requirement: 升级动作执行
系统 SHALL 支持三种升级动作的执行：通知（notify）、自动改派（reassign）、提升优先级（escalate_priority）。

#### Scenario: 执行通知动作
- **WHEN** 升级策略的 action_type 为 "notify"，action_config 包含 channel_id 和 notify_targets（用户 ID 列表）
- **THEN** 系统 SHALL 通过 Channel 模块向指定人员发送通知，通知内容包含工单编号、标题、超时类型、当前处理人

#### Scenario: 执行自动改派动作
- **WHEN** 升级策略的 action_type 为 "reassign"，action_config 包含 target_user_id
- **THEN** 系统 SHALL 将工单的当前处理人改为目标用户，并在工单时间线中记录改派事件

#### Scenario: 执行优先级提升动作
- **WHEN** 升级策略的 action_type 为 "escalate_priority"
- **THEN** 系统 SHALL 将工单优先级提升一级（如 P3→P2），并在时间线中记录提升事件；若已是最高优先级（P0），SHALL 不再提升

#### Scenario: 通知通道不可用
- **WHEN** 执行通知动作时，指定的 Channel 不存在或已禁用
- **THEN** 系统 SHALL 记录升级执行失败日志，但不阻塞后续升级动作的执行

### Requirement: SLA 状态追踪
系统 SHALL 在工单上维护 sla_status 字段，可选值为 on_track / breached_response / breached_resolution，反映当前 SLA 遵守情况。

#### Scenario: 新工单默认状态
- **WHEN** 工单创建且绑定了 SLA
- **THEN** sla_status SHALL 为 "on_track"

#### Scenario: 响应超时后状态变更
- **WHEN** 工单触发响应超时
- **THEN** sla_status SHALL 更新为 "breached_response"

#### Scenario: 解决超时后状态变更
- **WHEN** 工单触发解决超时
- **THEN** sla_status SHALL 更新为 "breached_resolution"

#### Scenario: SLA 状态不可回退
- **WHEN** 工单 sla_status 已为 "breached_resolution"
- **THEN** 即使后续工单被完结，sla_status SHALL 不回退为 "on_track"

### Requirement: SLA 统一适用于经典和智能工单
SLA 引擎 SHALL 对经典引擎和智能引擎创建的工单统一适用，不区分引擎类型。

#### Scenario: 经典工单 SLA 检查
- **WHEN** 经典引擎创建的工单绑定了 SLA 模板
- **THEN** 定时任务 SHALL 正常检查该工单的 SLA 状态并触发升级

#### Scenario: 智能工单 SLA 检查
- **WHEN** 智能引擎创建的工单绑定了 SLA 模板
- **THEN** 定时任务 SHALL 正常检查该工单的 SLA 状态并触发升级

### Requirement: 权限控制
SLA 模板和升级策略管理 SHALL 受 Casbin RBAC 保护，默认仅 admin 和 itsm_admin 角色可管理。

#### Scenario: itsm_admin 管理 SLA 模板
- **WHEN** itsm_admin 角色用户请求 SLA 模板相关 API
- **THEN** Casbin SHALL 放行

#### Scenario: 普通用户访问 SLA 管理
- **WHEN** user 角色用户请求 SLA 模板管理 API
- **THEN** Casbin SHALL 拒绝并返回 403

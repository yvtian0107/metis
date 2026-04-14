## ADDED Requirements

### Requirement: Priority 数据模型
系统 SHALL 维护 itsm_priorities 表，包含以下字段：name（优先级名称）、code（编码，P0~P4，唯一）、value（数值，越小越紧急）、color（颜色，Hex 值）、description（描述）、default_response_time（默认响应时间，分钟）、default_resolution_time（默认解决时间，分钟）、is_active（是否启用）。嵌入 BaseModel 提供 id、created_at、updated_at、deleted_at。

#### Scenario: 查询优先级列表
- **WHEN** 用户请求 GET /api/v1/itsm/priorities
- **THEN** 系统 SHALL 返回全部已启用的优先级定义，按 value 升序排列

#### Scenario: 优先级编码唯一
- **WHEN** 管理员尝试创建与已有 code 相同的优先级
- **THEN** 系统 SHALL 返回 409 错误，提示编码已存在

### Requirement: 故障分级定义
系统 SHALL 通过 Seed 数据预置 P0~P4 五个默认优先级定义，覆盖从紧急到咨询的完整分级。

#### Scenario: Seed 创建 P0 紧急级别
- **WHEN** ITSM App 首次执行 Seed
- **THEN** 系统 SHALL 创建 P0 优先级：name="紧急"、code="P0"、value=0、color="#DC2626"、description="全局影响，核心业务中断"、default_response_time=5、default_resolution_time=60

#### Scenario: Seed 创建 P1 高级别
- **WHEN** ITSM App 首次执行 Seed
- **THEN** 系统 SHALL 创建 P1 优先级：name="高"、code="P1"、value=1、color="#EA580C"、description="部门影响，重要功能不可用"、default_response_time=15、default_resolution_time=240

#### Scenario: Seed 创建 P2 中级别
- **WHEN** ITSM App 首次执行 Seed
- **THEN** 系统 SHALL 创建 P2 优先级：name="中"、code="P2"、value=2、color="#CA8A04"、description="多人影响，功能受限但有替代方案"、default_response_time=30、default_resolution_time=480

#### Scenario: Seed 创建 P3 低级别
- **WHEN** ITSM App 首次执行 Seed
- **THEN** 系统 SHALL 创建 P3 优先级：name="低"、code="P3"、value=3、color="#2563EB"、description="个人影响，非紧急功能异常"、default_response_time=60、default_resolution_time=1440

#### Scenario: Seed 创建 P4 最低级别
- **WHEN** ITSM App 首次执行 Seed
- **THEN** 系统 SHALL 创建 P4 优先级：name="最低"、code="P4"、value=4、color="#6B7280"、description="咨询类，信息查询或建议"、default_response_time=120、default_resolution_time=2880

#### Scenario: Seed 幂等执行
- **WHEN** ITSM App 再次执行 Seed，P0~P4 优先级已存在
- **THEN** 系统 SHALL 不创建重复记录，保持现有数据不变

### Requirement: 优先级 CRUD API
系统 SHALL 提供优先级管理 API，API 前缀为 /api/v1/itsm/priorities。

#### Scenario: 创建自定义优先级
- **WHEN** 管理员请求 POST /api/v1/itsm/priorities，body 包含 name、code、value、color
- **THEN** 系统 SHALL 创建优先级记录并返回 ID

#### Scenario: 更新优先级
- **WHEN** 管理员请求 PUT /api/v1/itsm/priorities/:id，修改 name 和 color
- **THEN** 系统 SHALL 更新优先级字段，code 和 value 不可变更

#### Scenario: 删除优先级
- **WHEN** 管理员请求 DELETE /api/v1/itsm/priorities/:id，且有工单引用该优先级
- **THEN** 系统 SHALL 返回 400 错误，提示优先级正在使用中不可删除

#### Scenario: 禁用优先级
- **WHEN** 管理员请求 PUT /api/v1/itsm/priorities/:id/toggle，将 is_active 设为 false
- **THEN** 系统 SHALL 禁用该优先级，新工单不可选用，已有工单不受影响

### Requirement: 故障升级链配置
系统 SHALL 维护 itsm_escalation_chains 表，按优先级配置多级升级（一级、二级、三级），每级指定通知对象和等待时间。字段包含：priority_id（关联优先级）、level（升级级别，1/2/3）、wait_minutes（本级等待时间，分钟）、notify_targets（通知对象，JSON 数组，包含 user_id 或 role_code）、notify_channel_id（通知通道 ID）。嵌入 BaseModel。

#### Scenario: 配置 P0 三级升级链
- **WHEN** 管理员为 P0 优先级配置升级链：一级（wait_minutes=5，通知值班工程师）、二级（wait_minutes=15，通知技术经理）、三级（wait_minutes=30，通知 CTO）
- **THEN** 系统 SHALL 创建三条升级链记录，level 分别为 1、2、3

#### Scenario: 查询升级链配置
- **WHEN** 管理员请求 GET /api/v1/itsm/priorities/:priorityId/escalation-chain
- **THEN** 系统 SHALL 返回该优先级的全部升级链配置，按 level 升序排列

#### Scenario: 批量保存升级链
- **WHEN** 管理员请求 PUT /api/v1/itsm/priorities/:priorityId/escalation-chain，body 包含完整的升级链列表
- **THEN** 系统 SHALL 全量替换该优先级的升级链配置

### Requirement: 故障自动通知
系统 SHALL 在 P0 或 P1 故障工单创建时，自动触发升级链第一级的通知。

#### Scenario: P0 工单创建自动通知
- **WHEN** 创建优先级为 P0 的工单
- **THEN** 系统 SHALL 立即触发 P0 升级链 level=1 的通知动作，通过配置的 Channel 通知指定对象

#### Scenario: P1 工单创建自动通知
- **WHEN** 创建优先级为 P1 的工单
- **THEN** 系统 SHALL 立即触发 P1 升级链 level=1 的通知动作

#### Scenario: P2 及以下工单不自动通知
- **WHEN** 创建优先级为 P2、P3 或 P4 的工单
- **THEN** 系统 SHALL 不触发自动通知，升级链仅在 SLA 超时时触发

#### Scenario: 升级链未配置
- **WHEN** 创建 P0 工单，但 P0 优先级未配置升级链
- **THEN** 系统 SHALL 跳过自动通知，记录警告日志

### Requirement: 故障升级定时检查
系统 SHALL 注册 Scheduler 定时任务 `itsm-escalation-check`（cron: `* * * * *`，每分钟执行），检查 P0/P1 工单是否需要触发下一级升级。

#### Scenario: 一级超时触发二级
- **WHEN** P0 工单创建超过一级的 wait_minutes 且仍未解决
- **THEN** 系统 SHALL 触发二级升级链的通知动作

#### Scenario: 二级超时触发三级
- **WHEN** P0 工单创建超过一级 + 二级的累计 wait_minutes 且仍未解决
- **THEN** 系统 SHALL 触发三级升级链的通知动作

#### Scenario: 工单已解决不再升级
- **WHEN** P0 工单在一级等待时间内被解决（状态变为 completed）
- **THEN** 系统 SHALL 停止该工单的升级检查，不触发后续级别通知

### Requirement: 故障关联（主工单/子工单）
系统 SHALL 支持多个工单关联为同一故障事件，通过主工单和子工单的方式管理。工单表增加 parent_ticket_id 字段（可选，外键关联自身）。

#### Scenario: 关联子工单
- **WHEN** 处理人请求 POST /api/v1/itsm/incidents/:ticketId/link，body 包含 child_ticket_id
- **THEN** 系统 SHALL 将 child_ticket_id 的 parent_ticket_id 设置为当前工单 ID，在两个工单的时间线中记录关联事件

#### Scenario: 查询关联工单
- **WHEN** 用户请求 GET /api/v1/itsm/incidents/:ticketId/linked
- **THEN** 系统 SHALL 返回该工单的全部子工单列表，包含 ticket_code、summary、status、priority

#### Scenario: 取消关联
- **WHEN** 处理人请求 DELETE /api/v1/itsm/incidents/:ticketId/link/:childTicketId
- **THEN** 系统 SHALL 清除子工单的 parent_ticket_id，在两个工单的时间线中记录取消关联事件

#### Scenario: 禁止循环关联
- **WHEN** 处理人尝试将主工单关联为另一个工单的子工单（形成环路）
- **THEN** 系统 SHALL 返回 400 错误，提示不允许循环关联

#### Scenario: 主工单完结联动
- **WHEN** 主工单状态变为 "completed"
- **THEN** 系统 SHALL 不自动完结子工单，但 SHALL 在子工单时间线中提示"关联的主工单已完结"

### Requirement: 故障复盘
系统 SHALL 支持在工单完结后创建复盘记录，包含根因分析和改进措施。系统 SHALL 维护 itsm_post_mortems 表，包含以下字段：ticket_id（关联工单，唯一）、root_cause（根因分析，富文本）、impact_summary（影响范围描述）、timeline_summary（故障时间线描述）、action_items（改进措施，JSON 数组，每项包含 description、assignee_id、due_date、status）、created_by（创建人）。嵌入 BaseModel。

#### Scenario: 创建复盘记录
- **WHEN** 处理人请求 POST /api/v1/itsm/incidents/:ticketId/post-mortem，body 包含 root_cause、impact_summary、timeline_summary、action_items
- **THEN** 系统 SHALL 创建复盘记录，在工单时间线中记录"已创建复盘"事件

#### Scenario: 工单未完结时不可创建
- **WHEN** 处理人尝试为状态非 "completed" 的工单创建复盘
- **THEN** 系统 SHALL 返回 400 错误，提示工单尚未完结

#### Scenario: 查询复盘记录
- **WHEN** 用户请求 GET /api/v1/itsm/incidents/:ticketId/post-mortem
- **THEN** 系统 SHALL 返回该工单的复盘记录详情，包含 action_items 及各项的完成状态

#### Scenario: 更新复盘记录
- **WHEN** 处理人请求 PUT /api/v1/itsm/incidents/:ticketId/post-mortem，修改 root_cause 或 action_items
- **THEN** 系统 SHALL 更新复盘记录

#### Scenario: 更新改进措施状态
- **WHEN** 责任人请求 PUT /api/v1/itsm/incidents/:ticketId/post-mortem/actions/:index，将某条改进措施的 status 更新为 "completed"
- **THEN** 系统 SHALL 更新该改进措施的状态

### Requirement: 故障生命周期与标准工单一致
故障工单 SHALL 使用与标准工单完全一致的生命周期（创建→流转→派单→处理→完结），但优先级影响 SLA deadline 和升级速度。

#### Scenario: P0 工单使用更紧迫的 SLA
- **WHEN** 创建 P0 工单，服务绑定的 SLA 模板未覆盖优先级默认值
- **THEN** 系统 SHALL 使用 P0 优先级的 default_response_time（5 分钟）和 default_resolution_time（60 分钟）作为 fallback

#### Scenario: SLA 模板优先于优先级默认值
- **WHEN** 创建 P0 工单，服务绑定的 SLA 模板定义了 response_time=10
- **THEN** 系统 SHALL 使用 SLA 模板的 response_time=10 而非 P0 默认的 5 分钟

#### Scenario: 高优先级工单升级更快
- **WHEN** P0 工单和 P3 工单同时触发 SLA 超时
- **THEN** P0 工单 SHALL 按其升级链配置的更短等待时间触发升级，P3 工单按其自身配置触发

### Requirement: Seed 数据初始化
ITSM App 的 Seed SHALL 初始化默认优先级定义（P0~P4），以及 Casbin 策略。

#### Scenario: 首次 Seed 创建优先级
- **WHEN** ITSM App 首次运行 Seed
- **THEN** 系统 SHALL 创建 P0~P4 五个默认优先级记录

#### Scenario: Seed 幂等检查
- **WHEN** ITSM App 再次运行 Seed，优先级已存在
- **THEN** 系统 SHALL 通过 code 字段做幂等检查，不创建重复记录

#### Scenario: Seed 创建 Casbin 策略
- **WHEN** ITSM App 运行 Seed
- **THEN** 系统 SHALL 为 itsm_admin 角色创建 /api/v1/itsm/priorities/* 和 /api/v1/itsm/incidents/* 的完整 CRUD 策略

### Requirement: 权限控制
优先级管理和故障管理 API SHALL 受 Casbin RBAC 保护。

#### Scenario: itsm_admin 管理优先级
- **WHEN** itsm_admin 角色用户请求优先级管理 API（创建、更新、删除）
- **THEN** Casbin SHALL 放行

#### Scenario: 普通用户只读优先级
- **WHEN** 普通用户请求 GET /api/v1/itsm/priorities
- **THEN** Casbin SHALL 放行（优先级列表为公开只读数据）

#### Scenario: 普通用户修改优先级
- **WHEN** 普通用户请求 POST/PUT/DELETE /api/v1/itsm/priorities/*
- **THEN** Casbin SHALL 拒绝并返回 403

#### Scenario: 处理人管理故障关联和复盘
- **WHEN** 工单处理人请求故障关联或复盘相关 API
- **THEN** Casbin SHALL 放行（处理人有权管理自己处理的工单的故障信息）

### Requirement: ITSM App 向 AI App 注册 Builtin Tool
ITSM App SHALL 在 Seed 阶段向 `ai_tools` 表注册 10 个 ITSM 专用 Builtin Tool（替换原有 6 个）。AI App 不存在时 SHALL 静默跳过注册，不影响 ITSM 经典功能。Seed 逻辑 SHALL 先清理不再使用的旧工具记录（itsm.search_services、itsm.query_ticket、itsm.cancel_ticket、itsm.add_comment）。

#### Scenario: AI App 存在时注册工具
- **WHEN** ITSM App 启动，ai_tools 表可用
- **THEN** 系统 SHALL 注册 10 个 ITSM Builtin Tool：itsm.service_match、itsm.service_confirm、itsm.service_load、itsm.new_request、itsm.draft_prepare、itsm.draft_confirm、itsm.validate_participants、itsm.ticket_create、itsm.my_tickets、itsm.ticket_withdraw

#### Scenario: 清理旧工具
- **WHEN** ITSM App 启动，ai_tools 表中存在已废弃的工具（itsm.search_services、itsm.query_ticket、itsm.cancel_ticket、itsm.add_comment）
- **THEN** 系统 SHALL 删除这些旧工具记录及其 ai_agent_tools 绑定关系

#### Scenario: AI App 不存在时静默跳过
- **WHEN** ITSM App 启动，ai_tools 表不存在
- **THEN** 系统 SHALL 静默跳过工具注册，仅输出 info 级别日志

#### Scenario: 工具注册幂等
- **WHEN** ITSM App 重启，ai_tools 表中已存在同名工具
- **THEN** 系统 SHALL 更新已有工具的 description 和 parameters_schema，而非创建重复记录

### Requirement: 工具 inputSchema 定义
每个 ITSM Builtin Tool 和通用工具 MUST 提供符合 JSON Schema 规范的 inputSchema。

#### Scenario: Agent 按 Schema 调用
- **WHEN** Agent 根据工具的 inputSchema 构造函数调用参数
- **THEN** 系统 SHALL 按 Schema 校验输入，校验失败返回明确的错误信息

#### Scenario: Schema 包含 description
- **WHEN** AI App 读取工具的 inputSchema
- **THEN** 每个参数 SHALL 包含 `description` 字段，描述语言为中文

### Requirement: 工具执行权限验证
每个 ITSM 工具和通用工具执行时 MUST 验证调用者权限，通过 Agent Session 关联的 user_id 确定当前操作者身份。

#### Scenario: 有效会话用户
- **WHEN** Agent 工具被调用，当前 Agent Session 关联了有效的 user_id
- **THEN** 系统 SHALL 以该 user_id 的身份执行操作

#### Scenario: 无会话用户
- **WHEN** Agent 工具被调用，当前 Agent Session 未关联 user_id
- **THEN** 系统 SHALL 以系统身份执行操作

### Requirement: IT 服务台 Agent 预置定义
ITSM App 的 Seed 数据 SHALL 包含一个"IT 服务台智能体"Agent 预置定义。Seed 逻辑 SHALL 先删除旧的"IT 服务台"、"ITSM 流程决策"、"ITSM 处理协助"三个预置智能体。Seed 更新时 SHALL 保留管理员自定义的 `system_prompt`。

#### Scenario: 清理旧智能体
- **WHEN** ITSM App 执行 Seed
- **THEN** 系统 SHALL 删除名为"IT 服务台"、"ITSM 流程决策"、"ITSM 处理协助"的旧智能体及其工具绑定

#### Scenario: Seed 创建 IT 服务台智能体
- **WHEN** ITSM App 执行 Seed 且 AI App 可用
- **THEN** 系统 SHALL 创建"IT 服务台智能体"：
  - type: assistant, strategy: react
  - visibility: public
  - temperature: 0.3, max_tokens: 4096, max_turns: 20
  - system_prompt: 复刻 bklite 的 19 条约束版服务台 prompt
  - 绑定 ITSM 工具: itsm.service_match、itsm.service_confirm、itsm.service_load、itsm.new_request、itsm.draft_prepare、itsm.draft_confirm、itsm.validate_participants、itsm.ticket_create、itsm.my_tickets、itsm.ticket_withdraw
  - 绑定通用工具: general.current_time、system.current_user_profile、organization.org_context

#### Scenario: Seed 幂等 - 不覆盖已自定义的 prompt
- **WHEN** ITSM App 重复执行 Seed，同名智能体已存在
- **THEN** 系统 SHALL 跳过 `system_prompt` 的更新
- **AND** 系统 SHALL 同步更新工具绑定（确保新增工具被绑定）
- **AND** 系统 SHALL 记录 info 日志 "智能体已存在，跳过 prompt 更新"

#### Scenario: Tool binding 同步更新
- **WHEN** ITSM App 执行 Seed 且智能体已存在且 ToolNames 中包含新增工具
- **THEN** 系统 SHALL 将新增工具绑定到已有智能体
- **AND** 系统 SHALL 移除不再需要的旧工具绑定

### Requirement: 流程决策 Agent 预置定义
ITSM App 的 Seed 数据 SHALL 包含一个"流程决策智能体"Agent 预置定义，复刻 bklite 的决策原则版 prompt。Seed 更新时 SHALL 保留管理员自定义的 `system_prompt`。

#### Scenario: Seed 创建流程决策智能体
- **WHEN** ITSM App 执行 Seed 且 AI App 可用
- **THEN** 系统 SHALL 创建"流程决策智能体"：
  - type: assistant, strategy: react
  - visibility: private
  - temperature: 0.2, max_tokens: 2048, max_turns: 1
  - system_prompt: 复刻 bklite 的 4 条决策原则 + 4 条严格约束版 prompt
  - 不绑定任何工具（SmartEngine 内部使用）

#### Scenario: Seed 幂等 - 不覆盖已自定义的 prompt
- **WHEN** ITSM App 重复执行 Seed，同名智能体已存在
- **THEN** 系统 SHALL 跳过 `system_prompt` 的更新

### Requirement: draft hash 确定性序列化
`hashFormData()` SHALL 使用确定性的 JSON 序列化方式，确保相同内容始终产生相同的 hash 值。

#### Scenario: map 键顺序不影响 hash
- **WHEN** 对 `{"b": 1, "a": 2}` 和 `{"a": 2, "b": 1}` 分别计算 hash
- **THEN** 两次 hash 结果 SHALL 相同

#### Scenario: 实现方式
- **WHEN** 计算 form data hash
- **THEN** SHALL 先对 map keys 排序，再按排序后的顺序序列化为 JSON bytes 进行 hash

### Requirement: draft_confirm 字段变更错误恢复规则

生产 prompt 的严格约束区块 SHALL 新增一条规则：当 `itsm.draft_confirm` 返回含 "字段已变更" 的错误时，Agent MUST 重新调用 `itsm.service_load` 获取最新表单定义，再根据新定义调用 `itsm.draft_prepare` 重新准备草稿；若新增了必填字段，向用户追问后再继续。

#### Scenario: prompt 包含恢复规则
- **WHEN** 查看 IT 服务台智能体的 system_prompt
- **THEN** SHALL 包含对 draft_confirm 字段变更错误的恢复指引
- **AND** 指引 Agent 重新 service_load → draft_prepare

#### Scenario: Agent 遵循恢复规则
- **WHEN** Agent 调用 draft_confirm 收到 "服务表单字段已变更" 错误
- **THEN** Agent SHALL 在 ReAct 循环内重新调用 service_load
- **AND** 随后调用 draft_prepare 重新准备草稿

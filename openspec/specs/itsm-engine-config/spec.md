## Purpose

定义 ITSM 的「智能岗位」与「引擎设置」双入口配置模型。智能岗位只承载以 Agent 身份执行任务的岗位编制；引擎设置只承载 ITSM 领域内非 Agent 的参考路径生成、异常兜底与审计运行参数。

## Requirements

### Requirement: ITSM 智能岗位配置 API

系统 SHALL 提供智能岗位配置 API `GET /api/v1/itsm/smart-staffing/config` 和 `PUT /api/v1/itsm/smart-staffing/config`，统一读写 Agentic ITSM 的岗位编制配置。API 受 JWT + Casbin 权限保护。

响应结构：
```json
{
  "posts": {
    "intake": {
      "agentId": 5,
      "agentName": "IT 服务台智能体"
    },
    "decision": {
      "agentId": 6,
      "agentName": "流程决策智能体",
      "mode": "direct_first"
    },
    "slaAssurance": {
      "agentId": 7,
      "agentName": "SLA 保障智能体"
    }
  },
  "health": {
    "items": [
      { "key": "intake", "label": "服务受理岗", "status": "pass", "message": "服务受理岗已上岗" },
      { "key": "decision", "label": "流程决策岗", "status": "pass", "message": "流程决策岗已上岗" },
      { "key": "slaAssurance", "label": "SLA 保障岗", "status": "pass", "message": "SLA 保障岗已上岗" }
    ]
  }
}
```

请求结构（PUT）：
```json
{
  "posts": {
    "intake": { "agentId": 5 },
    "decision": { "agentId": 6, "mode": "direct_first" },
    "slaAssurance": { "agentId": 7 }
  }
}
```

#### Scenario: 读取智能岗位配置
- **WHEN** 管理员调用 `GET /api/v1/itsm/smart-staffing/config`
- **THEN** 系统 SHALL 读取 `itsm.smart_ticket.intake.agent_id`、`itsm.smart_ticket.decision.agent_id`、`itsm.smart_ticket.sla_assurance.agent_id` 和 `itsm.smart_ticket.decision.mode`
- **AND** 系统 SHALL 查询对应 Agent 名称
- **AND** 系统 SHALL 返回服务受理岗、流程决策岗、SLA 保障岗的健康状态

#### Scenario: 保存智能岗位配置
- **WHEN** 管理员调用 `PUT /api/v1/itsm/smart-staffing/config`
- **THEN** 系统 SHALL 更新 `itsm.smart_ticket.intake.agent_id`、`itsm.smart_ticket.decision.agent_id`、`itsm.smart_ticket.sla_assurance.agent_id` 和 `itsm.smart_ticket.decision.mode`

#### Scenario: Agent 不存在时读取
- **WHEN** 读取配置且 SystemConfig 中的 agent_id 对应 Agent 不存在或已被删除
- **THEN** 系统 SHALL 保留对应 agentId 并返回空 agentName
- **AND** 健康状态 SHALL 标记对应岗位不可用

#### Scenario: 无效 agent_id
- **WHEN** 保存配置时提交的 agentId 对应 Agent 不存在或 is_active=false
- **THEN** 系统 SHALL 返回 400 错误 "智能体不存在或已停用"

#### Scenario: 无效决策模式
- **WHEN** 保存配置时提交的 decision.mode 不是 `direct_first` 或 `ai_only`
- **THEN** 系统 SHALL 返回 400 错误 "ITSM 配置无效"

#### Scenario: 智能岗位健康检查
- **WHEN** 系统构建智能岗位健康状态
- **THEN** 服务受理岗 SHALL 校验 Agent 是否启用、是否配置模型、是否绑定 `itsm.service_match`、`itsm.service_load`、`itsm.draft_prepare`、`itsm.draft_confirm`、`itsm.validate_participants`、`itsm.ticket_create`
- **AND** 流程决策岗 SHALL 校验 Agent 是否启用、是否配置模型、是否绑定 `decision.ticket_context`、`decision.resolve_participant`、`decision.sla_status`、`decision.list_actions`、`decision.execute_action`
- **AND** SLA 保障岗 SHALL 校验 Agent 是否启用、是否配置模型、是否绑定 `sla.risk_queue`、`sla.ticket_context`、`sla.escalation_rules`、`sla.trigger_escalation`、`sla.write_timeline`
- **AND** 服务受理岗 SHALL 校验 `itsm.service_match` 的 AI Tool 运行时已配置

### Requirement: ITSM 引擎设置 API

系统 SHALL 提供引擎设置 API `GET /api/v1/itsm/engine-settings/config` 和 `PUT /api/v1/itsm/engine-settings/config`，统一读写 ITSM 领域内非 Agent 的参考路径生成、异常兜底与审计配置。API 受 JWT + Casbin 权限保护。

响应结构：
```json
{
  "runtime": {
    "pathBuilder": {
      "modelId": 1,
      "providerId": 1,
      "providerName": "DeepSeek",
      "modelName": "deepseek-v3",
      "temperature": 0.3,
      "maxRetries": 3,
      "timeoutSeconds": 120
    },
    "guard": {
      "auditLevel": "full",
      "fallbackAssignee": 0
    }
  },
  "health": {
    "items": [
      { "key": "pathBuilder", "label": "参考路径生成", "status": "pass", "message": "参考路径生成已就绪" },
      { "key": "guard", "label": "异常兜底与审计", "status": "warn", "message": "未指定兜底处理人，异常时只能进入人工处置队列" }
    ]
  }
}
```

请求结构（PUT）：
```json
{
  "runtime": {
    "pathBuilder": {
      "modelId": 1,
      "temperature": 0.3,
      "maxRetries": 3,
      "timeoutSeconds": 120
    },
    "guard": {
      "auditLevel": "full",
      "fallbackAssignee": 0
    }
  }
}
```

#### Scenario: 读取引擎设置
- **WHEN** 管理员调用 `GET /api/v1/itsm/engine-settings/config`
- **THEN** 系统 SHALL 读取 `itsm.smart_ticket.path.model_id`、`itsm.smart_ticket.path.temperature`、`itsm.smart_ticket.path.max_retries`、`itsm.smart_ticket.path.timeout_seconds`、`itsm.smart_ticket.guard.audit_level` 和 `itsm.smart_ticket.guard.fallback_assignee`
- **AND** 系统 SHALL 根据 modelId 返回 providerId、providerName 和 modelName
- **AND** 系统 SHALL 返回参考路径生成、异常兜底与审计的健康状态

#### Scenario: 保存引擎设置
- **WHEN** 管理员调用 `PUT /api/v1/itsm/engine-settings/config`
- **THEN** 系统 SHALL 更新 `itsm.smart_ticket.path.model_id`、`itsm.smart_ticket.path.temperature`、`itsm.smart_ticket.path.max_retries`、`itsm.smart_ticket.path.timeout_seconds`、`itsm.smart_ticket.guard.audit_level` 和 `itsm.smart_ticket.guard.fallback_assignee`

#### Scenario: 无效 model_id
- **WHEN** 保存配置时提交的 pathBuilder.modelId 对应 AIModel 不存在、已停用，或其 Provider 不存在、已停用
- **THEN** 系统 SHALL 返回 400 错误 "模型不存在或已停用"

#### Scenario: 无效参考路径生成参数
- **WHEN** 保存配置时 pathBuilder.temperature 不在 0 到 1 之间
- **THEN** 系统 SHALL 返回 400 错误 "ITSM 配置无效"
- **WHEN** pathBuilder.maxRetries 小于 0 或大于 10
- **THEN** 系统 SHALL 返回 400 错误 "ITSM 配置无效"
- **WHEN** pathBuilder.timeoutSeconds 小于 10 或大于 300
- **THEN** 系统 SHALL 返回 400 错误 "ITSM 配置无效"

#### Scenario: 无效审计级别
- **WHEN** 保存配置时 guard.auditLevel 不是 `full`、`summary` 或 `off`
- **THEN** 系统 SHALL 返回 400 错误 "ITSM 配置无效"

#### Scenario: 保存无效 fallbackAssignee
- **WHEN** 保存配置时 guard.fallbackAssignee 对应用户不存在或 is_active=false
- **THEN** 系统 SHALL 返回 400 错误 "兜底处理人不存在或已停用"

#### Scenario: fallbackAssignee 为 0 时清除配置
- **WHEN** 保存配置时 guard.fallbackAssignee 为 0
- **THEN** 系统 SHALL 将 `itsm.smart_ticket.guard.fallback_assignee` 设为 "0"，表示未配置兜底处理人

#### Scenario: 参考路径生成健康检查
- **WHEN** 系统构建参考路径生成健康状态
- **THEN** 系统 SHALL 校验 pathBuilder.modelId 对应模型和 Provider 均存在且启用
- **AND** 系统 SHALL 校验 pathBuilder.timeoutSeconds 大于 0
- **AND** 系统 SHALL 校验 pathBuilder.maxRetries 不小于 0

#### Scenario: 异常兜底与审计健康检查
- **WHEN** 系统构建异常兜底与审计健康状态
- **THEN** guard.fallbackAssignee 为 0 时 SHALL 返回 warn
- **AND** guard.fallbackAssignee 对应用户不存在或停用时 SHALL 返回 fail
- **AND** guard.fallbackAssignee 对应用户有效时 SHALL 返回 pass

### Requirement: ITSM 智能岗位前端页面

系统 SHALL 在 ITSM 模块侧边栏系统配置分组提供「智能岗位」菜单项（路由 `/itsm/smart-staffing`），页面只展示真正以 Agent 身份执行任务、绑定工具并产生操作轨迹的岗位。

#### Scenario: 页面加载
- **WHEN** 管理员进入 `/itsm/smart-staffing`
- **THEN** 系统 SHALL 调用 `GET /api/v1/itsm/smart-staffing/config` 加载岗位配置
- **AND** 系统 SHALL 调用 AI Agent 列表接口加载可选 assistant Agent

#### Scenario: 岗位配置
- **WHEN** 页面加载完成
- **THEN** 系统 SHALL 展示服务受理岗、流程决策岗、SLA 保障岗
- **AND** 服务受理岗 SHALL 只配置上岗 Agent
- **AND** 流程决策岗 SHALL 配置上岗 Agent 和决策模式
- **AND** SLA 保障岗 SHALL 只配置上岗 Agent

#### Scenario: 决策模式选择
- **WHEN** 管理员在流程决策岗选择决策模式
- **THEN** 系统 SHALL 提供两个选项：「优先确定路径，回退 AI」（direct_first）和「始终使用 AI 决策」（ai_only）

#### Scenario: 保存智能岗位
- **WHEN** 管理员修改岗位配置并保存
- **THEN** 系统 SHALL 调用 `PUT /api/v1/itsm/smart-staffing/config` 提交完整岗位配置

### Requirement: ITSM 引擎设置前端页面

系统 SHALL 在 ITSM 模块侧边栏系统配置分组提供「引擎设置」菜单项（路由 `/itsm/engine-settings`），页面只展示 ITSM 领域内非 Agent 的参考路径生成、异常兜底与审计配置。

#### Scenario: 页面加载
- **WHEN** 管理员进入 `/itsm/engine-settings`
- **THEN** 系统 SHALL 调用 `GET /api/v1/itsm/engine-settings/config` 加载引擎设置
- **AND** 系统 SHALL 调用 AI Provider 和模型接口加载参考路径生成的可选模型
- **AND** 系统 SHALL 调用用户列表接口加载可选兜底处理人

#### Scenario: 参考路径生成配置
- **WHEN** 页面加载完成
- **THEN** 系统 SHALL 展示「参考路径生成」配置区块
- **AND** 区块 SHALL 提供 Provider、模型、温度、重试和超时配置
- **AND** Provider 变更时 SHALL 重新加载该 Provider 下的模型，并清空不属于新 Provider 的已选模型

#### Scenario: 异常兜底与审计配置
- **WHEN** 页面加载完成
- **THEN** 系统 SHALL 展示「异常兜底与审计」配置区块
- **AND** 区块 SHALL 提供审计级别和兜底处理人配置
- **AND** 审计级别 SHALL 提供三个选项：「完整推理记录」（full）、「仅摘要」（summary）、「关闭」（off）

#### Scenario: 保存引擎设置
- **WHEN** 管理员修改引擎设置并保存
- **THEN** 系统 SHALL 调用 `PUT /api/v1/itsm/engine-settings/config` 提交完整引擎设置

#### Scenario: 服务匹配运行时不属于引擎设置
- **WHEN** 管理员需要配置 `itsm.service_match` 的 Provider、模型、温度、最大 Token 或超时
- **THEN** 系统 SHALL 要求管理员前往 AI Tools 的 `itsm.service_match` 工具详情 Sheet 配置
- **AND** ITSM 引擎设置页面 SHALL NOT 重复展示 `itsm.service_match` 的运行时参数

### Requirement: ITSM Seed 引擎配置

ITSM App 的 Seed SHALL 在首次安装和后续启动时确保智能岗位和引擎设置相关的 SystemConfig 默认值存在，并清理旧引擎配置模型。

#### Scenario: 首次安装写入智能岗位默认 agent_id
- **WHEN** ITSM Seed 运行且智能岗位 agent_id 配置不存在
- **THEN** Seed SHALL 查询 code=`itsm.servicedesk` 的 Agent ID 写入 `itsm.smart_ticket.intake.agent_id`
- **AND** Seed SHALL 查询 code=`itsm.decision` 的 Agent ID 写入 `itsm.smart_ticket.decision.agent_id`
- **AND** Seed SHALL 查询 code=`itsm.sla_assurance` 的 Agent ID 写入 `itsm.smart_ticket.sla_assurance.agent_id`
- **AND** preset Agent 不存在时 SHALL 写入 "0"

#### Scenario: 首次安装写入引擎设置默认值
- **WHEN** ITSM Seed 运行且对应 SystemConfig 不存在
- **THEN** Seed SHALL 写入 `itsm.smart_ticket.decision.mode`=`direct_first`
- **AND** Seed SHALL 写入 `itsm.smart_ticket.path.model_id`=`0`
- **AND** Seed SHALL 写入 `itsm.smart_ticket.path.temperature`=`0.3`
- **AND** Seed SHALL 写入 `itsm.smart_ticket.path.max_retries`=`3`
- **AND** Seed SHALL 写入 `itsm.smart_ticket.path.timeout_seconds`=`120`
- **AND** Seed SHALL 写入 `itsm.smart_ticket.guard.audit_level`=`full`
- **AND** Seed SHALL 写入 `itsm.smart_ticket.guard.fallback_assignee`=`0`

#### Scenario: seed-dev 默认兜底处理人为 admin
- **WHEN** 开发环境 `seed-dev` 已创建或更新 admin 用户
- **AND** `itsm.smart_ticket.guard.fallback_assignee` 不存在、为空或为 "0"
- **THEN** seed-dev SHALL 将 `itsm.smart_ticket.guard.fallback_assignee` 写为 admin 用户 ID
- **AND** seed-dev SHALL NOT 覆盖已经配置为其他非零用户 ID 的兜底处理人

#### Scenario: 迁移旧 SystemConfig key
- **WHEN** ITSM Seed 运行且新 key 不存在但旧 `itsm.engine.*` key 存在
- **THEN** Seed SHALL 将旧值迁移到对应的 `itsm.smart_ticket.*` key

#### Scenario: 迁移旧 internal path builder Agent
- **WHEN** ITSM Seed 运行且存在 code 为 `itsm.path_builder` 或 `itsm.generator` 的旧 internal Agent
- **THEN** Seed SHALL 将其 model_id 和 temperature 迁移到 `itsm.smart_ticket.path.model_id` 和 `itsm.smart_ticket.path.temperature`

#### Scenario: 初始化服务匹配 Tool 运行时
- **WHEN** ITSM Seed 运行且 `itsm.service_match` Tool 运行时为空或为默认空配置
- **THEN** Seed SHALL 可以从 code=`itsm.servicedesk` 的 Agent 迁移 model_id、temperature 和 max_tokens 到 `itsm.service_match` Tool runtime_config

#### Scenario: 删除旧配置
- **WHEN** ITSM Seed 完成迁移
- **THEN** Seed SHALL 删除旧 `itsm.engine.*` SystemConfig
- **AND** Seed SHALL 删除旧 `itsm.smart_ticket.service_matcher.*` SystemConfig
- **AND** Seed SHALL 删除 code 为 `itsm.path_builder` 或 `itsm.generator` 的旧 Agent

#### Scenario: 后续启动不覆盖已有配置
- **WHEN** ITSM Seed 运行且新 SystemConfig 已存在
- **THEN** Seed SHALL 跳过已存在的配置，不覆盖用户自定义值

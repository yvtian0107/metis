### Requirement: ITSM 引擎配置聚合 API

系统 SHALL 提供聚合 API `GET /api/v1/itsm/engine/config` 和 `PUT /api/v1/itsm/engine/config`，统一读写 ITSM 引擎的全部配置。API 受 JWT + Casbin 权限保护。

响应结构：
```json
{
  "generator": {
    "modelId": 1,
    "providerId": 1,
    "providerName": "DeepSeek",
    "modelName": "deepseek-v3",
    "temperature": 0.3
  },
  "servicedesk": {
    "agentId": 5,
    "agentName": "IT 服务台智能体"
  },
  "decision": {
    "agentId": 6,
    "agentName": "流程决策智能体",
    "decisionMode": "direct_first"
  },
  "general": {
    "maxRetries": 3,
    "timeoutSeconds": 30,
    "reasoningLog": "full",
    "fallbackAssignee": 0
  }
}
```

请求结构（PUT）：
```json
{
  "generator": { "modelId": 1, "temperature": 0.3 },
  "servicedesk": { "agentId": 5 },
  "decision": { "agentId": 6, "decisionMode": "direct_first" },
  "general": { "maxRetries": 3, "timeoutSeconds": 30, "reasoningLog": "full", "fallbackAssignee": 0 }
}
```

#### Scenario: 读取引擎配置
- **WHEN** 管理员调用 `GET /api/v1/itsm/engine/config`
- **THEN** 系统 SHALL 读取 `itsm.generator`（internal agent）的配置（model_id 关联的 provider_id、providerName、modelName、temperature），读取 SystemConfig 中 `itsm.engine.servicedesk.agent_id` 和 `itsm.engine.decision.agent_id` 查询对应 Agent 的 name，以及 SystemConfig 中 `itsm.engine.*` 前缀的运维参数，合并为统一的 JSON 结构返回

#### Scenario: 保存引擎配置
- **WHEN** 管理员调用 `PUT /api/v1/itsm/engine/config` 提交完整配置
- **THEN** 系统 SHALL 更新 `itsm.generator` Agent 的 model_id 和 temperature，更新 SystemConfig 中的 `itsm.engine.servicedesk.agent_id`、`itsm.engine.decision.agent_id`、`itsm.engine.decision.decision_mode`、`itsm.engine.general.max_retries`、`itsm.engine.general.timeout_seconds`、`itsm.engine.general.reasoning_log`、`itsm.engine.general.fallback_assignee`

#### Scenario: Agent 不存在时读取
- **WHEN** 读取配置且 SystemConfig 中的 agent_id 对应的 Agent 不存在或已被删除
- **THEN** 系统 SHALL 在对应区块返回 agentId=0、agentName=""，前端据此展示未配置状态

#### Scenario: 无效 agent_id
- **WHEN** 保存配置时提交的 servicedesk 或 decision 的 agentId 对应的 Agent 不存在或 is_active=false
- **THEN** 系统 SHALL 返回 400 错误 "智能体不存在或已停用"

#### Scenario: 无效 model_id
- **WHEN** 保存配置时提交的 generator modelId 对应的 AIModel 不存在或已停用
- **THEN** 系统 SHALL 返回 400 错误 "模型不存在或已停用"

#### Scenario: 保存无效 fallback_assignee
- **WHEN** 保存配置时提交的 `fallback_assignee` 用户 ID 对应的用户不存在或 `is_active=false`
- **THEN** 系统 SHALL 返回 400 错误 "兜底处理人不存在或已停用"

#### Scenario: fallback_assignee 为 0 时清除配置
- **WHEN** 保存配置时 `fallback_assignee` 为 0
- **THEN** 系统 SHALL 将 `itsm.engine.general.fallback_assignee` 设为 "0"，表示未配置兜底处理人

### Requirement: ITSM 引擎配置前端页面

系统 SHALL 在 ITSM 模块侧边栏提供「引擎配置」菜单项（路由 `/itsm/engine-config`），页面展示四个配置区块：解析引擎、服务台智能体、决策智能体、通用设置。

#### Scenario: 页面加载
- **WHEN** 管理员进入 `/itsm/engine-config` 页面
- **THEN** 系统 SHALL 调用 `GET /api/v1/itsm/engine/config` 加载配置，调用 `GET /api/v1/ai/providers` 加载 Provider 列表（用于 generator），调用 `GET /api/v1/ai/agents` 加载 Agent 列表（用于 servicedesk/decision 的下拉选择及预览信息）

#### Scenario: 两列布局 — 智能体卡片
- **WHEN** 页面加载完成且视口宽度 >= md 断点
- **THEN** 系统 SHALL 将服务台智能体和决策智能体两个配置卡片在同一行以 `grid grid-cols-1 md:grid-cols-2` 两列布局展示，解析引擎和通用设置保持全宽单列

#### Scenario: 服务台智能体配置卡片
- **WHEN** 页面加载完成
- **THEN** 系统 SHALL 展示"服务台智能体"配置卡片，包含 Agent 下拉选择器（列表来自 AI 智能体，筛选 type=assistant 且 is_active=true），描述为"IT 服务台接单引导流程所使用的智能体"

#### Scenario: 决策智能体配置卡片
- **WHEN** 页面加载完成
- **THEN** 系统 SHALL 展示"决策智能体"配置卡片，包含 Agent 下拉选择器（同上筛选条件）、决策模式选择，描述为"工单运行时流程决策所使用的智能体"

#### Scenario: 智能体预览信息
- **WHEN** 服务台或决策卡片中已选择一个智能体
- **THEN** 系统 SHALL 在下拉选择器下方展示该智能体的摘要信息，包含策略（strategy）、温度（temperature）、最大轮次（maxTurns），以 `text-xs text-muted-foreground` 样式单行展示，字段间以 `·` 分隔

#### Scenario: 智能体预览 — 未选择时
- **WHEN** 服务台或决策卡片中未选择智能体（agentId 为 0）
- **THEN** 系统 SHALL 不展示预览信息

#### Scenario: 配置状态指示器 — 已配置
- **WHEN** 卡片对应的配置已正确设置（智能体 agentId > 0 且该智能体存在于 agent 列表且 isActive=true；生成引擎 modelId > 0）
- **THEN** 系统 SHALL 在卡片标题右侧展示 `bg-green-500` 圆点（h-2 w-2 rounded-full）及"已配置"文字标签

#### Scenario: 配置状态指示器 — 未配置
- **WHEN** 卡片对应的配置未设置（agentId 或 modelId 为 0）
- **THEN** 系统 SHALL 在卡片标题右侧展示 `bg-gray-400` 圆点及"未配置"文字标签

#### Scenario: 配置状态指示器 — 异常
- **WHEN** 卡片已配置的智能体在 agent 列表中不存在或 isActive=false
- **THEN** 系统 SHALL 在卡片标题右侧展示 `bg-red-500` 圆点及"异常"文字标签

#### Scenario: Provider-Model 联动（仅 Generator）
- **WHEN** 管理员在解析引擎区块选择 AI 服务商（Provider）
- **THEN** 系统 SHALL 调用 `GET /api/v1/ai/providers/:id/models` 加载该 Provider 下的模型列表，填充模型下拉框。若之前已选的模型属于新 Provider 则保留，否则清空模型选择

#### Scenario: 保存配置
- **WHEN** 管理员修改配置并点击「保存」
- **THEN** 系统 SHALL 调用 `PUT /api/v1/itsm/engine/config` 提交全部配置，成功后显示成功提示

#### Scenario: 未配置 Agent 引导
- **WHEN** 页面加载时 Agent 列表为空（系统未添加任何智能体）
- **THEN** 系统 SHALL 在 servicedesk/decision 卡片展示引导提示："请先在 AI 模块添加智能体"，并提供跳转链接

#### Scenario: 决策模式选择
- **WHEN** 管理员在决策智能体区块选择决策模式
- **THEN** 系统 SHALL 提供两个选项：「优先确定路径，回退 AI」（direct_first）和「始终使用 AI 决策」（ai_only），使用 Select 组件

#### Scenario: 推理日志级别选择
- **WHEN** 管理员在通用设置区块选择推理日志级别
- **THEN** 系统 SHALL 提供三个选项：「完整推理记录」（full）、「仅摘要」（summary）、「关闭」（off），使用 Select 组件

### Requirement: ITSM Seed Preset Agent 带 Code

ITSM App 的 `tools/provider.go` SeedAgents SHALL 在创建 preset agent 时写入 `code` 字段，使引擎配置服务能通过 code 统一管理。

#### Scenario: 首次安装创建带 code 的 preset agent
- **WHEN** SeedAgents 运行且数据库中不存在名为"IT 服务台智能体"的 Agent
- **THEN** Seed SHALL 创建 Agent 记录并设置 code=`itsm.servicedesk`

#### Scenario: 首次安装创建决策智能体
- **WHEN** SeedAgents 运行且数据库中不存在名为"流程决策智能体"的 Agent
- **THEN** Seed SHALL 创建 Agent 记录并设置 code=`itsm.decision`

#### Scenario: 已有 agent 补写 code
- **WHEN** SeedAgents 运行且数据库中已存在名为"IT 服务台智能体"的 Agent 但 code 为空
- **THEN** Seed SHALL 更新该 Agent 的 code 为 `itsm.servicedesk`

#### Scenario: 已有决策智能体补写 code
- **WHEN** SeedAgents 运行且数据库中已存在名为"流程决策智能体"的 Agent 但 code 为空
- **THEN** Seed SHALL 更新该 Agent 的 code 为 `itsm.decision`

### Requirement: ITSM Seed 移除 itsm.runtime Internal Agent

ITSM App 的 `seed.go` seedEngineConfig SHALL 不再创建 `itsm.runtime` internal agent。`itsm.generator` 保持不变。

#### Scenario: seedEngineConfig 仅创建 generator
- **WHEN** seedEngineConfig 运行
- **THEN** Seed SHALL 仅检查并创建 code=`itsm.generator` 的 internal agent，不再处理 `itsm.runtime`

#### Scenario: SystemConfig 默认值 key 调整
- **WHEN** seedEngineConfig 写入默认 SystemConfig
- **THEN** Seed SHALL 将 `itsm.engine.runtime.decision_mode` 改为 `itsm.engine.decision.decision_mode`

### Requirement: ITSM 引擎配置菜单注册

ITSM App SHALL 在 Seed 阶段注册「引擎配置」菜单项和对应的权限策略。

#### Scenario: 菜单注册
- **WHEN** ITSM Seed 运行
- **THEN** Seed SHALL 在 ITSM 菜单组下添加「引擎配置」菜单项，路由为 `/itsm/engine-config`，排序在现有菜单末尾

#### Scenario: 权限策略注册
- **WHEN** ITSM Seed 运行
- **THEN** Seed SHALL 通过 Casbin 注册 `GET /api/v1/itsm/engine/config` 和 `PUT /api/v1/itsm/engine/config` 的权限策略，关联到 ITSM 管理角色

### Requirement: ITSM Seed 引擎默认配置

ITSM App 的 Seed SHALL 在首次安装和后续启动时确保引擎相关的 Agent 记录和 SystemConfig 默认值存在。

#### Scenario: 首次安装创建 internal Agent
- **WHEN** ITSM Seed 运行且数据库中不存在 code 为 `itsm.generator` 的 Agent
- **THEN** Seed SHALL 创建 Agent 记录：code=`itsm.generator`、name="ITSM 工作流解析"、type=`internal`、temperature=0.3、system_prompt=内置解析提示词、model_id=0（未绑定）

#### Scenario: 首次安装写入 SystemConfig 默认值
- **WHEN** ITSM Seed 运行且 SystemConfig 中不存在 key `itsm.engine.decision.decision_mode`
- **THEN** Seed SHALL 写入以下默认值：`itsm.engine.decision.decision_mode`=`direct_first`、`itsm.engine.general.max_retries`=`3`、`itsm.engine.general.timeout_seconds`=`30`、`itsm.engine.general.reasoning_log`=`full`

#### Scenario: 后续启动不覆盖已有配置
- **WHEN** ITSM Seed 运行且 Agent 和 SystemConfig 已存在
- **THEN** Seed SHALL 跳过已存在的记录，不覆盖用户自定义的配置值

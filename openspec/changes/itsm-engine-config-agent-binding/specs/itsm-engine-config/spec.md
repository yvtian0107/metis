## MODIFIED Requirements

### Requirement: ITSM 引擎配置聚合 API

系统 SHALL 提供聚合 API `GET /api/v1/itsm/engine/config` 和 `PUT /api/v1/itsm/engine/config`，统一读写 ITSM 引擎的全部配置。API 受 JWT + Casbin 权限保护。

响应/请求结构：
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
    "modelId": 3,
    "providerId": 1,
    "providerName": "DeepSeek",
    "modelName": "deepseek-v3",
    "temperature": 0.3
  },
  "decision": {
    "modelId": 2,
    "providerId": 1,
    "providerName": "DeepSeek",
    "modelName": "deepseek-r1",
    "temperature": 0.2,
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

#### Scenario: 读取引擎配置
- **WHEN** 管理员调用 `GET /api/v1/itsm/engine/config`
- **THEN** 系统 SHALL 读取 `itsm.generator`（internal agent）、`itsm.servicedesk`（preset agent）、`itsm.decision`（preset agent）三个 Agent 的配置（model_id 关联的 provider_id、providerName、modelName、temperature），以及 SystemConfig 中 `itsm.engine.*` 前缀的运维参数，合并为统一的 JSON 结构返回

#### Scenario: 保存引擎配置
- **WHEN** 管理员调用 `PUT /api/v1/itsm/engine/config` 提交完整配置
- **THEN** 系统 SHALL 更新 `itsm.generator` Agent 的 model_id 和 temperature，更新 `itsm.servicedesk` Agent 的 model_id 和 temperature，更新 `itsm.decision` Agent 的 model_id 和 temperature，更新 SystemConfig 中的 `itsm.engine.decision.decision_mode`、`itsm.engine.general.max_retries`、`itsm.engine.general.timeout_seconds`、`itsm.engine.general.reasoning_log`

#### Scenario: Agent 未绑定模型时读取
- **WHEN** 读取配置且某个 Agent 的 model_id 为空（0 或 null）
- **THEN** 系统 SHALL 在对应区块返回 modelId=0、providerId=0、providerName=""、modelName=""，前端据此展示未配置状态

#### Scenario: 无效 model_id
- **WHEN** 保存配置时提交的 model_id 对应的 AIModel 不存在或已停用
- **THEN** 系统 SHALL 返回 400 错误 "模型不存在或已停用"

### Requirement: ITSM 引擎配置前端页面

系统 SHALL 在 ITSM 模块侧边栏提供「引擎配置」菜单项（路由 `/itsm/engine-config`），页面展示四个配置区块：解析引擎、服务台智能体、决策智能体、通用设置。

#### Scenario: 页面加载
- **WHEN** 管理员进入 `/itsm/engine-config` 页面
- **THEN** 系统 SHALL 调用 `GET /api/v1/itsm/engine/config` 加载配置，并调用 `GET /api/v1/ai/providers` 加载 Provider 列表填充下拉框

#### Scenario: 服务台智能体配置卡片
- **WHEN** 页面加载完成
- **THEN** 系统 SHALL 展示"服务台智能体"配置卡片，包含 Provider 选择、Model 选择、Temperature 滑块，描述为"IT 服务台接单引导流程所使用的 LLM 配置"

#### Scenario: 决策智能体配置卡片
- **WHEN** 页面加载完成
- **THEN** 系统 SHALL 展示"决策智能体"配置卡片（原"决策引擎"），包含 Provider 选择、Model 选择、Temperature 滑块、决策模式选择，描述为"工单运行时流程决策所使用的 LLM 配置"

#### Scenario: Provider-Model 联动
- **WHEN** 管理员在某个配置区块选择 AI 服务商（Provider）
- **THEN** 系统 SHALL 调用 `GET /api/v1/ai/providers/:id/models`（或等效 API）加载该 Provider 下的模型列表，填充模型下拉框。若之前已选的模型属于新 Provider 则保留，否则清空模型选择

#### Scenario: 保存配置
- **WHEN** 管理员修改配置并点击「保存」
- **THEN** 系统 SHALL 调用 `PUT /api/v1/itsm/engine/config` 提交全部配置，成功后显示成功提示

#### Scenario: 未配置 Provider 引导
- **WHEN** 页面加载时 Provider 列表为空（系统未添加任何 AI 服务商）
- **THEN** 系统 SHALL 展示引导提示："请先在 AI 模块添加服务商"，并提供跳转链接

#### Scenario: 决策模式选择
- **WHEN** 管理员在决策智能体区块选择决策模式
- **THEN** 系统 SHALL 提供两个选项：「优先确定路径，回退 AI」（direct_first）和「始终使用 AI 决策」（ai_only），使用 Select 组件

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

## REMOVED Requirements

### Requirement: 首次安装创建运行时 Agent
**Reason**: `itsm.runtime` internal agent 被移除，决策能力由 `itsm.decision` preset agent 承担
**Migration**: 引擎配置服务改为读写 code=`itsm.decision` 的 preset agent

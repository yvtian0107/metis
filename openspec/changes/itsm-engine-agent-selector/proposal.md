## Why

引擎配置页面中「服务台智能体」和「决策智能体」的配置当前使用 Provider → Model 选择器，这与实际语义不符——它们应该选择 AI 管理中已配置好的 **Agent**（自带 model、system prompt、tools、策略），而非裸选 LLM Provider/Model。

## What Changes

- **服务台智能体和决策智能体改为 Agent 选择器**：前端从 Provider+Model+Temperature 三控件改为单个 Agent 下拉选择，列表来自 AI 管理中的智能体
- **存储方式变更**：不再修改 preset agent 的 model_id/temperature，改为在 SystemConfig 中存储选中的 agent_id（`itsm.engine.servicedesk.agent_id`、`itsm.engine.decision.agent_id`）
- **API 契约变更**：GET/PUT 的 `servicedesk` 和 `decision` 区块字段从 modelId/providerId/providerName/modelName/temperature 改为 agentId/agentName
- **SmartEngine 运行时变更**：`agenticDecision()` 从 `GetAgentConfigByCode("itsm.decision")` 改为读取 SystemConfig 中配置的 agent_id 调用 `GetAgentConfig(agentID)`
- **Seed 默认值**：首次安装时 SystemConfig 指向 seed 创建的 preset agent ID
- **Generator 不变**：生成器引擎仍使用 Provider → Model 选择

## Capabilities

### New Capabilities

（无新增）

### Modified Capabilities

- `itsm-engine-config`: servicedesk/decision 区块从 Provider+Model 选择改为 Agent 选择，API 契约变更，存储方式从 agent 记录改为 SystemConfig
- `itsm-smart-engine`: 决策智能体获取方式从 `GetAgentConfigByCode` 硬编码改为从 SystemConfig 读取 agent_id

## Impact

- **后端**: `engine_config_service.go`（Get/Update 逻辑重写 servicedesk/decision 部分）、`engine/smart_react.go`（决策 agent 获取方式）、`seed.go`（写入默认 agent_id）
- **前端**: `pages/engine-config/index.tsx`（servicedesk/decision 卡片 UI 重写）
- **API**: GET/PUT `/api/v1/itsm/engine/config` 的 servicedesk/decision 区块字段变更（**BREAKING**）

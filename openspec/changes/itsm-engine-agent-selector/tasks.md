## 1. 后端 Service 层

- [x] 1.1 `engine_config_service.go`: 修改 `EngineConfig` 结构体，servicedesk/decision 区块从 `EngineAgentConfig`（modelId/providerId/providerName/modelName/temperature）改为 `EngineAgentSelector`（agentId/agentName）
- [x] 1.2 `engine_config_service.go`: 修改 `GetConfig()` — servicedesk/decision 从 `readAgentConfig(code)` 改为读取 SystemConfig `itsm.engine.servicedesk.agent_id` / `itsm.engine.decision.agent_id`，查询 Agent 记录获取 name；agent 不存在时返回 agentId=0、agentName=""
- [x] 1.3 `engine_config_service.go`: 修改 `UpdateConfigRequest` 结构体，servicedesk/decision 从 modelId/temperature 改为 agentId
- [x] 1.4 `engine_config_service.go`: 修改 `UpdateConfig()` — servicedesk/decision 校验 agent 存在且 is_active=true，然后写入 SystemConfig agent_id；移除对 `updateAgentConfig` 的调用
- [x] 1.5 `engine_config_service.go`: 新增 sentinel error `ErrAgentNotFound`，handler 中映射为 400

## 2. 后端 EngineConfigProvider 接口

- [x] 2.1 `engine_config_service.go`: `EngineConfigProvider` 接口新增 `DecisionAgentID() uint` 方法，从 SystemConfig 读取 `itsm.engine.decision.agent_id`

## 3. SmartEngine 运行时

- [x] 3.1 `engine/smart_react.go`: `agenticDecision()` 从 `e.agentProvider.GetAgentConfigByCode("itsm.decision")` 改为 `e.configProvider.DecisionAgentID()` → `e.agentProvider.GetAgentConfig(agentID)`，agent_id 为 0 时返回错误 "决策智能体未配置"

## 4. Seed 默认值

- [x] 4.1 `seed.go`: `seedEngineConfig()` 新增写入 `itsm.engine.servicedesk.agent_id` 和 `itsm.engine.decision.agent_id` 默认值，查询 preset agent 的 ID（code=`itsm.servicedesk` / `itsm.decision`），不存在则写 "0"

## 5. 前端

- [x] 5.1 `pages/engine-config/index.tsx`: 新增 Agent 列表查询（`GET /api/v1/ai/agents`），筛选 type=assistant 且 is_active=true
- [x] 5.2 `pages/engine-config/index.tsx`: servicedesk 卡片从 LLMFields（Provider+Model+Temperature）改为 Agent 下拉选择器
- [x] 5.3 `pages/engine-config/index.tsx`: decision 卡片从 LLMFields 改为 Agent 下拉选择器 + 决策模式 Select
- [x] 5.4 `pages/engine-config/index.tsx`: 表单 state 中 servicedesk/decision 从 providerId/modelId/temperature 改为 agentId
- [x] 5.5 `pages/engine-config/index.tsx`: Agent 列表为空时展示引导提示 "请先在 AI 模块添加智能体"

## 6. Spec 同步

- [x] 6.1 运行 `/opsx:sync` 将 delta specs 合并到 main specs

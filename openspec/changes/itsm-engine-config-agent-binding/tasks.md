## 1. Preset Agent 加 Code

- [x] 1.1 `tools/provider.go`: `presetAgent` 结构体增加 `Code string` 字段
- [x] 1.2 `tools/provider.go`: "IT 服务台智能体" seed 时设置 code=`itsm.servicedesk`，"流程决策智能体"设置 code=`itsm.decision`
- [x] 1.3 `tools/provider.go`: `SeedAgents` 的 upsert 逻辑调整——已存在 agent（按 name 匹配）更新时补写 code 字段；新建时写入 code

## 2. 移除 itsm.runtime Internal Agent

- [x] 2.1 `seed.go`: `seedEngineConfig` 中移除 `itsm.runtime` agent 的 seed 逻辑（保留 `itsm.generator`）
- [x] 2.2 `seed.go`: 移除 `itsmRuntimeSystemPrompt` 常量
- [x] 2.3 `seed.go`: SystemConfig 默认值 key 从 `itsm.engine.runtime.decision_mode` 改为 `itsm.engine.decision.decision_mode`

## 3. 引擎配置服务重构

- [x] 3.1 `engine_config_service.go`: `EngineConfig` 结构体将 `Runtime` 改为 `Decision EngineDecisionConfig`，新增 `Servicedesk EngineAgentConfig`
- [x] 3.2 `engine_config_service.go`: `GetConfig` 读取 `itsm.servicedesk` 和 `itsm.decision` 替代 `itsm.runtime`
- [x] 3.3 `engine_config_service.go`: `UpdateConfigRequest` 结构体同步调整（runtime → decision，新增 servicedesk）
- [x] 3.4 `engine_config_service.go`: `UpdateConfig` 方法更新逻辑适配新结构
- [x] 3.5 `engine_config_service.go`: SystemConfig key 读写从 `itsm.engine.runtime.decision_mode` 改为 `itsm.engine.decision.decision_mode`

## 4. SmartEngine AgentProvider 适配

- [x] 4.1 `engine/smart.go`: AgentProvider 从按 `itsm.runtime` 查找改为按 `itsm.decision` 查找完整 agent，使用其 model_id 和 temperature 发起 ReAct 执行
- [x] 4.2 `engine/smart.go`: decisionMode 提示词注入逻辑读取 `itsm.engine.decision.decision_mode`

## 5. 前端引擎配置页面

- [x] 5.1 `locales/zh-CN.json` + `locales/en.json`: `runtimeTitle` 改为"决策智能体"/"Decision Agent"，`runtimeDesc` 更新描述；新增 `servicedeskTitle`/`servicedeskDesc`
- [x] 5.2 `api.ts`: `EngineConfig` / `EngineConfigUpdate` 类型调整（runtime → decision，新增 servicedesk）
- [x] 5.3 `pages/engine-config/index.tsx`: 新增"服务台智能体"配置卡片（LLMFields），位于解析引擎和决策智能体之间
- [x] 5.4 `pages/engine-config/index.tsx`: runtime 状态变量和逻辑改为 decision，新增 servicedesk 状态变量

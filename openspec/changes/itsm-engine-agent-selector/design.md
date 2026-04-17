## Context

引擎配置页面（`/itsm/engine-config`）当前为 servicedesk 和 decision 两个角色提供 Provider → Model → Temperature 三级选择器，本质是在直接修改 preset agent 记录的 `model_id` 和 `temperature`。这与 AI 管理中已有完整 Agent 管理（含 model、prompt、tools、策略）的设计矛盾——管理员应选择一个已配置好的 Agent，而非在引擎配置中重复配置 LLM 参数。

当前涉及文件：
- `engine_config_service.go`：`readAgentConfig()` 通过 agent code 查 model_id → join provider；`updateAgentConfig()` 直接更新 agent 的 model_id/temperature
- `engine/smart_react.go:18`：`GetAgentConfigByCode("itsm.decision")` 硬编码获取决策 agent
- `pages/engine-config/index.tsx`：`LLMFields` 组件渲染 Provider/Model/Temperature 控件
- `seed.go`：`seedEngineConfig()` 创建 `itsm.generator` internal agent 并写入 SystemConfig 默认值

## Goals / Non-Goals

**Goals:**
- servicedesk 和 decision 配置改为从 AI 智能体列表中选择一个 Agent
- 选中的 agent_id 存入 SystemConfig，不再修改 agent 记录本身
- SmartEngine 运行时从 SystemConfig 读取配置的 agent_id
- Seed 时自动将默认 agent_id 指向 preset agent

**Non-Goals:**
- 不改 generator（生成器引擎），保持 Provider → Model 选择
- 不改 Agent 模型本身的字段结构
- 不改 AI App 的 Agent CRUD API

## Decisions

### 1. 存储：SystemConfig 而非 Agent 记录

agent_id 存入 SystemConfig（`itsm.engine.servicedesk.agent_id`、`itsm.engine.decision.agent_id`），而非在 agent 表上加 engine 绑定字段。

**理由**：SystemConfig 是 ITSM 引擎配置的统一存储位置（`itsm.engine.*`），与 general 设置（maxRetries、timeout 等）保持一致。Agent 表属于 AI App，不应被 ITSM 的配置需求侵入。

### 2. API 响应包含 agentName 用于展示

GET 返回 `agentId` + `agentName`，前端用 agentName 做展示，不需要额外查询。

```json
{
  "servicedesk": { "agentId": 5, "agentName": "IT 服务台智能体" },
  "decision": { "agentId": 6, "agentName": "流程决策智能体", "decisionMode": "direct_first" }
}
```

PUT 只需传 `agentId`：
```json
{
  "servicedesk": { "agentId": 5 },
  "decision": { "agentId": 6, "decisionMode": "direct_first" }
}
```

### 3. SmartEngine 通过 EngineConfigProvider 获取 agent_id

给 `EngineConfigProvider` 接口新增 `DecisionAgentID() uint` 方法。`agenticDecision()` 调用此方法获取 agent_id，再用已有的 `GetAgentConfig(agentID)` 获取完整配置。

**备选方案**：直接在 `agenticDecision` 中读 SystemConfig → 增加了对 SystemConfig 的直接依赖，破坏了接口抽象。

### 4. 前端 Agent 下拉数据源

调用 `GET /api/v1/ai/agents`（AI App 已有的 agent 列表 API），筛选 `type=assistant` 且 `is_active=true` 的 agent 展示在下拉列表中。

### 5. Seed 默认值写入时机

`seedEngineConfig()` 在写入 SystemConfig 默认值时，需要先确保 preset agents 已创建（`SeedAgents()` 先于 `seedEngineConfig()` 执行）。Seed 查询 preset agent 的 ID 写入 SystemConfig。如果 preset agent 不存在（被删除），则默认值为 0（未配置）。

## Risks / Trade-offs

- **[Preset agent 被删除]** → agent_id 指向不存在的记录。缓解：GET 配置时校验 agent 是否存在，不存在则返回 agentId=0、agentName=""，前端展示未配置状态。
- **[API Breaking Change]** → servicedesk/decision 区块字段完全变化。缓解：前后端同步修改，无外部消费者。
- **[Seed 执行顺序依赖]** → SystemConfig 默认 agent_id 依赖 preset agent 先创建。缓解：ITSM seed 中 `SeedAgents()` 已在 `seedEngineConfig()` 之前调用（`tools/provider.go` 的 SeedAgents 在 `seed.go` 的 Seed 函数中先被调用）。

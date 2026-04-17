## Context

ITSM 引擎配置页面管理智能引擎的 LLM 配置。当前架构存在两层 agent：

1. **Internal agent**（`seed.go` → `seedEngineConfig`）：`itsm.generator`（解析引擎）和 `itsm.runtime`（运行时决策），纯 LLM 配置，无工具绑定
2. **Preset agent**（`tools/provider.go` → `SeedAgents`）："IT 服务台智能体"和"流程决策智能体"，完整 assistant agent，有工具绑定和 ReAct 策略

问题在于：`itsm.runtime` 是一个多余的中间层——SmartEngine 真正需要的是"流程决策智能体"的 LLM 配置来发起 ReAct 执行。同时"IT 服务台智能体"的 LLM 配置没有统一管理入口。

## Goals / Non-Goals

**Goals:**
- 消除 `itsm.runtime` internal agent，让引擎配置直接管理真正干活的 preset agent
- 给 preset agent 加 `code` 字段，使引擎配置服务能统一通过 code 定位
- 引擎配置页面新增服务台智能体配置卡片，改名决策引擎为决策智能体
- SmartEngine 改为从 `itsm.decision` 读取 LLM 配置

**Non-Goals:**
- 不修改 preset agent 的 system prompt、工具绑定或 ReAct 策略
- 不做已有数据库数据迁移（用户手动清库）
- 不修改服务定义层面的 agent_id 绑定逻辑

## Decisions

### D1: preset agent 增加 code 字段

`tools/provider.go` 的 `presetAgent` 结构体增加 `Code string` 字段。seed 时写入 `ai_agents.code`。

- `IT 服务台智能体` → `code: "itsm.servicedesk"`
- `流程决策智能体` → `code: "itsm.decision"`

**理由**：引擎配置服务已有成熟的 `readAgentConfig(code)` / `updateAgentConfig(code)` 模式，复用该模式最简单。

### D2: 移除 itsm.runtime seed，保留 itsm.generator

`seedEngineConfig()` 中移除 `itsm.runtime` agent 的 seed 逻辑。`itsm.generator` 保留不变——它是纯 LLM 解析器，没有对应的完整 agent，继续作为 internal agent 存在。

### D3: API 结构调整

```
旧结构：{ generator, runtime, general }
新结构：{ generator, servicedesk, decision, general }
```

- `runtime` 整个移除
- `decision` 替代 `runtime`，对应 `itsm.decision`（流程决策智能体）
- `servicedesk` 新增，对应 `itsm.servicedesk`（IT 服务台智能体）
- `decisionMode` 从 `runtime` 移入 `decision`

Go 结构体映射：
```
EngineConfig {
  Generator   EngineAgentConfig    // code=itsm.generator
  Servicedesk EngineAgentConfig    // code=itsm.servicedesk  ← 新增
  Decision    EngineDecisionConfig // code=itsm.decision     ← 替代 runtime
  General     EngineGeneralConfig  // 不变
}
```

### D4: SmartEngine AgentProvider 适配

SmartEngine 通过 `AgentProvider` 接口获取 agent 配置。改为按 `code=itsm.decision` 查找完整 agent，用其 `model_id` 和 `temperature` 发起 ReAct 执行。`decisionMode` 仍从 SystemConfig 读取，作为提示词注入。

### D5: SeedAgents 幂等策略

`SeedAgents` 当前按 `name` 查重。增加 `code` 后，对已存在的 agent（按 name 匹配到）追加 `code` 字段更新，确保已有数据库升级时也能填入 code。

## Risks / Trade-offs

- **BREAKING API 变更** → 前后端需同步发布。由于 ITSM 是内部模块且没有外部消费者，风险可控
- **已有数据库中的 preset agent 无 code** → `SeedAgents` 的 upsert 逻辑需在更新已有记录时补写 code 字段
- **`itsm.runtime` 残留数据** → 不做自动清理，用户手动删库。如果残留不影响功能（引擎配置不再读取它）

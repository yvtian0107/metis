## Why

智能引擎当前 BDD 只覆盖了 7 种决策类型中的 2 种（approve/complete），且引擎错误处理、熔断机制、取消流程、拒绝决策等关键分支完全未测。现有场景依赖真实 LLM，既慢又不确定——无法稳定触达特定边界路径。

通过 `ExecuteConfirmedPlan` + crafted `DecisionPlan`，可以绕过 LLM 直接注入决策，确定性地覆盖引擎执行层的所有分支。

## What Changes

- 创建 `features/vpn_smart_engine_deterministic.feature`：10+ 确定性 Scenario，覆盖引擎执行层全路径
- 创建 `steps_vpn_smart_deterministic_test.go`：确定性步骤实现，通过 crafted DecisionPlan 驱动
- 注册新步骤到 `bdd_test.go`

## Capabilities

### Modified Capabilities
- `itsm-bdd-infrastructure`: 增加智能引擎确定性覆盖场景

## Scope

### Tier 1 — 核心引擎逻辑
1. 多种决策类型执行：process / action / notify / form / escalate（通过 crafted plan）
2. AI 决策失败 → failure count 递增 + timeline 记录
3. 连续 3 次失败 → AI disabled 熔断
4. Cancel 智能引擎工单（取消活跃审批活动）

### Tier 2 — 决策质量保障
5. 低置信度 → 人工拒绝 → 不执行 + rejected timeline
6. 兜底用户无效（inactive）→ warning timeline
7. complete 类型直接决策 → 工单完结

### 不做
- LLM 工具链深度测试（knowledge_search / sla_status / similar_history）
- Service desk 对话层覆盖
- 真实 LLM 决策质量验证

## Impact

- `internal/app/itsm/features/vpn_smart_engine_deterministic.feature` (new)
- `internal/app/itsm/steps_vpn_smart_deterministic_test.go` (new)
- `internal/app/itsm/bdd_test.go` (mod — register new steps)

## Dependencies

- smart-engine-fallback-assignee (已完成，提供 tryFallbackAssignment + EngineConfigProvider)

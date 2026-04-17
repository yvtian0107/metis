## Context

智能引擎 `executeDecisionPlan`（smart.go:382）在创建 Activity 后，仅当 `ParticipantID != nil && > 0` 时创建 assignment。AI Agent 可能因为 `toolResolveParticipant` 返回空 candidates 而无法指定有效参与者，导致 Activity 创建了但无人认领。

引擎配置已有 `EngineGeneralConfig`（存储在 `SystemConfig` K/V 表，`itsm.engine.general.*` 前缀），目前包含 `MaxRetries`、`TimeoutSeconds`、`ReasoningLog` 三个字段。

## Goals / Non-Goals

**Goals:**
- 在 `EngineGeneralConfig` 中新增 `FallbackAssignee`（用户 ID），管理员可配置
- 智能引擎 `executeDecisionPlan` 执行时，若 activity 需要参与者但 AI 未指定有效人选，自动替换为兜底处理人
- Timeline 明确标记"参与者缺失，已转派兜底处理人"
- BDD 场景验证兜底行为

**Non-Goals:**
- 不修改经典引擎（ClassicEngine）的参与者处理
- 不修改 `ParticipantResolver`（resolver 层）
- 不改变 `toolResolveParticipant` 的工具返回行为（仍返回空 candidates）
- 不做前端引擎配置页面的 UI 改动（本轮只做后端 + BDD）

## Decisions

### D1: 兜底检查位置 — `executeDecisionPlan` 内部

**选择**: 在 `executeDecisionPlan` 中，创建 assignment 前检查 `ParticipantID`。
**备选**: (A) 在 `validateDecisionPlan` 校验时拒绝空参与者 → 触发 failure count；(B) 在 resolver 层自动注入。
**理由**: A 会导致本可救回的工单进入失败流程；B 对经典引擎有副作用且对 AI 不透明。在执行层兜底最精准——只影响"需要人但 AI 没给有效人"的情况。

### D2: 判定"需要参与者"的条件

需要参与者的 activity type: `approve`、`process`、`form`。`action` 不需要（HTTP webhook），`notify`/`complete`/`escalate` 也不需要。
当 `da.Type` 在这三类中且 `ParticipantID` 为 nil 或 0 时触发兜底。

### D3: 配置存储 — 复用 `SystemConfig` K/V

**键名**: `itsm.engine.general.fallback_assignee`，值为用户 ID 字符串（"0" 或空 = 未配置）。
**理由**: 完全复用现有 `EngineConfigService` 的读写模式（`getConfigInt`/`setConfigValue`），无需加表加列。

### D4: SmartEngine 读取配置的方式

SmartEngine 不直接依赖 `EngineConfigService`（那是 service 层），而是通过新增的 `ConfigProvider` 接口获取 fallback 配置。
在 IOC 注入时将 `EngineConfigService` 适配为该接口传入 `NewSmartEngine`。

```go
type EngineConfigProvider interface {
    FallbackAssigneeID() uint
}
```

### D5: 兜底 Timeline 事件

使用新的事件类型 `participant_fallback`，message 格式："参与者缺失，已转派兜底处理人（{username}）"。

## Risks / Trade-offs

- [兜底人不存在或未激活] → `executeDecisionPlan` 在创建 assignment 前检查 user 是否 active，不 active 则 skip 兜底并记录 warning timeline
- [配置值为 0（未配置）时参与者缺失] → 保持原有行为：activity 创建但无 assignment，等人工介入
- [并发多个 Activity 都缺参与者] → 全部转给同一个兜底人是可接受的，admin 可以自行改派

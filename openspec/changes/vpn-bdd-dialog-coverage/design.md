## Context

SmartEngine 的 `runDecisionCycle` 有 5 条出口路径、`executeDecisionPlan` 处理 7 种 activity type、`handleDecisionFailure` 有熔断逻辑、`Cancel` 有独立流程。当前 BDD 仅通过真实 LLM 覆盖了 approve + complete 两条路径。

关键入口：
- `SmartEngine.ExecuteConfirmedPlan(tx, ticketID, plan)` — 跳过 LLM + 校验，直接执行 plan
- `SmartEngine.Start(ctx, tx, params)` — 完整流程（含 LLM）
- `SmartEngine.Progress(ctx, tx, params)` — 完成活动后触发下一轮
- `SmartEngine.Cancel(ctx, tx, params)` — 取消流程

已有模式参考：`steps_vpn_participant_test.go` 中用 `ExecuteConfirmedPlan` + crafted plan 实现了确定性测试。

## Goals / Non-Goals

**Goals:**
- 确定性覆盖 SmartEngine 执行层的全部分支（不依赖 LLM）
- 覆盖 7 种决策类型的 activity 创建
- 覆盖错误处理（failure count、熔断、拒绝）
- 覆盖 Cancel 流程
- 覆盖 fallback 无效用户 warning

**Non-Goals:**
- 不测 LLM 决策质量（已有 agentic scenario 覆盖）
- 不测 agentic ReAct 工具调用链
- 不测 service desk 对话层
- 不增加 agentic (真 LLM) scenario

## Decisions

### D1: 全部用 ExecuteConfirmedPlan 驱动

**选择**: 通过 crafted `DecisionPlan` 直接调用 `ExecuteConfirmedPlan`，不经过 `runDecisionCycle`。

**替代方案**: mock AgentProvider 返回预设 plan → 走完整 `runDecisionCycle`。

**理由**: `ExecuteConfirmedPlan` 直接调 `executeDecisionPlan` / `handleComplete`，覆盖了执行层全路径。mock AgentProvider 更重但额外覆盖的只是 validate 逻辑（已有 agentic 场景间接覆盖）。对于 failure/disabled 路径需要直接操作 `ai_failure_count` 字段。

### D2: 单独 feature 文件 + steps 文件

**选择**: `vpn_smart_engine_deterministic.feature` + `steps_vpn_smart_deterministic_test.go`

**理由**: 与现有 agentic 场景（`vpn_smart_flow.feature`）明确区分。确定性场景不需要 LLM env vars，可以跳过 LLM 依赖独立运行（但当前 TestBDD 统一 gate 了 LLM config，保持一致即可）。

### D3: 共享 Background + 复用现有 bddContext

**选择**: 与 smart_flow 共享 Background（系统初始化 + 参与人 + 协作规范 + 发布智能服务），在 When 步骤中直接构造 plan。

**理由**: 需要 service/priority/user 等基础数据，复用 Background 最简洁。

### D4: failure/disabled 路径通过直接操作 DB 模拟

**选择**: 直接 `UPDATE itsm_tickets SET ai_failure_count = N` 然后调 `runDecisionCycle`（需要 mock agentProvider 返回 error），或直接验证 `handleDecisionFailure` 的 DB 效果。

**实际做法**: 用一个 `failingAgentProvider`（`GetAgentConfig` 返回有效 config，但 LLM 端点指向不存在地址）或直接 set failure count 到 3 后调 `RunDecisionCycleForTicket`。

更简洁：直接设 `ai_failure_count = 3` 后调 `RunDecisionCycleForTicket`，验证返回 `ErrAIDisabled` + timeline 记录。

### D5: Cancel 通过 SmartEngine.Cancel 直接调用

直接调用，验证活动状态变 cancelled、工单状态变 cancelled、timeline 记录。

## Risks / Trade-offs

- **[Risk] ExecuteConfirmedPlan 跳过了 validateDecisionPlan** → 可接受，agentic 场景已间接覆盖校验；本轮目标是执行层分支
- **[Risk] action 类型的 scheduler.SubmitTask 在测试中是 noop** → noopSubmitter 已存在，验证 activity 创建即可
- **[Risk] TestBDD 统一要求 LLM env vars，但确定性场景不需要 LLM** → 保持现状，不拆分 test suite；未来可考虑 tag 分离

## Context

服务台 Agent 对话流程中，`draft_confirm` 会校验 FieldsHash 是否与 `service_load` 时一致。如果管理员在用户对话期间修改了服务表单字段（增删字段、改选项等），`draft_confirm` 返回 "服务表单字段已变更，请重新调用 service_load" 错误。

当前状态：
- handler 层的 FieldsHash 校验逻辑已完备（`handlers.go` L557）
- 生产 prompt（`provider.go` 19 条规则）**没有**关于 `draft_confirm` 失败后恢复的指导
- dialog BDD 测试框架（`steps_vpn_dialog_validation_test.go`）只支持单轮对话、只记录 toolCalls 不记录 toolResults
- 无任何测试覆盖 Agent 面对工具报错后的自愈行为

## Goals / Non-Goals

**Goals:**
- 生产 prompt 增加 `draft_confirm` 字段变更错误的恢复规则，使 Agent 能在 ReAct 循环内自愈
- BDD 测试验证 Agent 在 `draft_confirm` 返回字段变更错误后，自动重新 `service_load` → `draft_prepare`
- 扩展 dialog 测试框架，增加 toolResults 记录（调试用）和工具调用计数断言

**Non-Goals:**
- 不测 `ticket_create` 的 DraftVersion 不匹配（handler 层 unit test 已覆盖，不属于 agentic 恢复）
- 不做多轮对话框架（单轮 ReAct 循环内自愈即可）
- 不测非路由字段的 multivalue 场景（与现有 dialog validation 重叠度高）

## Decisions

### 1. 注入点：StateStore.SaveState 装饰器

**选择**: 在 `memStateStore` 之上包装一个 `mutatingStateStore`，当 state.Stage 变为 `"awaiting_confirmation"` 时触发 DB 表单变更。

**备选方案**:
- *Operator 层拦截*: 在 `LoadService` 调用后修改 DB。但 `service_load` handler 和 `draft_confirm` handler 都调用 `LoadService`，需要计数来区分，不够精确。
- *多轮对话*: 第一轮跑到 draft_prepare 后停下，手动改 DB，第二轮继续。需要大幅重构测试框架，成本过高。

**理由**: StateStore 的 stage 转换是精确的语义边界——`"awaiting_confirmation"` 只在 `draft_prepare` 成功后出现，不会误触发。

### 2. 表单变更类型：新增 optional 字段

**选择**: `vpnFormSchemaV2` 在原有 4 个字段基础上增加一个 `remark`（备注）optional 字段。

**理由**: 改变 FieldsHash 但不破坏已收集的 form_data，Agent 可以在同一个 ReAct 循环内无障碍完成重试，无需向用户追问新信息。

### 3. 断言策略：调用计数为主，toolResults 为辅

**选择**: 主断言用 `service_load ≥ 2` + `draft_prepare ≥ 2`（稳定、不依赖消息文本）。`toolResults` 记录用于失败调试，不作为 pass/fail 判定。

**理由**: LLM 测试天然非确定性，断言越多越脆。调用计数是可靠的行为信号——如果 Agent 重新 load 和 prepare 了，说明它处理了错误。

### 4. 恢复规则写入生产 prompt

**选择**: 在 `provider.go` 的严格约束区块增加一条恢复规则，测试 prompt 同步加入。

**备选**: 只在测试 prompt 加。但管理员改表单字段是真实生产场景，Agent 必须具备恢复能力。

## Risks / Trade-offs

- **LLM 非确定性** → 即使 prompt 明确写了恢复策略，Agent 可能偶尔不遵循。缓解：测试用 temperature=0.2，且断言指标选择调用计数（最鲁棒的信号）。
- **mutatingStateStore 与内部 stage 耦合** → 如果 draft_prepare handler 改了 stage 名（如从 `"awaiting_confirmation"` 改名），测试会静默失败（不触发变更）。缓解：测试中额外断言 `draft_confirm` 曾被调用，确保流程确实走到了关键点。

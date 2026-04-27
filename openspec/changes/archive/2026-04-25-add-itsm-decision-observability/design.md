## Context

ITSM SmartEngine 的 AI 决策链路跨两个包：

1. **AI App 层** (`internal/app/ai/runtime/decision_executor.go`)：DecisionExecutor 实现 ReAct 循环，调 LLM、dispatch tool calls
2. **ITSM Engine 层** (`internal/app/itsm/engine/smart.go` + `smart_tools.go`)：构建 domain context、注册 8 个决策 tool、校验和执行 Decision Plan

当前日志覆盖极度不均：HTTP 请求层有完整 OTel trace，但决策内部一个完整周期只输出 3 行 Info（turn start/end + decision completed）。Tool 调用仅有 Debug 级别一行（生产不可见），tool 错误被静默包装为 JSON，Decision Plan 内容和执行结果只写数据库。

实际部署环境不一定接入 OTel，运维排查手段可能只有 `docker logs`。方案必须在纯 slog → stderr 条件下即可工作。

## Goals / Non-Goals

**Goals:**

- 运维通过 `grep ticketID=42` 即可串联一个工单的完整决策链路（从触发到执行）
- 每个 tool 调用的名称、耗时、成功/失败在 Info 级别可见
- Tool 错误不再被静默吞掉——Warn 级别输出后继续正常流程
- Decision Plan 关键字段（next_step_type、confidence、activities 数量、execution_mode）在控制台可见
- Plan 执行结果（创建了什么 activity、分给了谁）在控制台可见
- 零外部依赖，纯 slog；OTel 接入时 trace_id/span_id 由现有 traceHandler 自动注入

**Non-Goals:**

- 不引入新的日志库或框架（不加 zap/zerolog）
- 不新建数据库表或修改现有表结构
- 不实现日志级别运行时调节（可作为后续改进）
- 不实现 OTel Span 级别的决策瀑布图（那是 Level 2 的事）
- 不记录 tool 参数/结果的完整 JSON（避免日志膨胀和敏感数据泄露），只记摘要
- 不修改前端

## Decisions

### D1: 在 toolHandler 闭包层统一包装，而非在每个 tool handler 内部加日志

**选择**：在 `smart.go:agenticDecision()` 构建 `toolHandler` 闭包时，用一个 logging wrapper 包裹 `handlerMap[name](toolCtx, args)`，统一记录 tool 名称、耗时、错误。

**替代方案**：在 `smart_tools.go` 每个 handler 函数入口/出口各加 slog 调用。

**理由**：
- 8 个 tool handler，逐个加是重复劳动且容易遗漏
- 闭包包装是单点修改，天然覆盖未来新增 tool
- tool handler 内部保持纯业务逻辑，不混入 observability concern
- 耗时计算只需在 wrapper 层 `time.Now()` / `time.Since()`

### D2: Tool 错误同时记日志 + 继续返回 JSON 给 LLM

**选择**：在 decision_executor.go 的 tool 错误路径增加 `slog.Warn`，但仍把 error JSON 返回给 LLM（不改变现有行为）。

**替代方案**：tool 错误时中断 ReAct 循环。

**理由**：
- LLM 能自行处理 tool 错误（换策略、降 confidence），当前行为设计合理
- 问题不是 LLM 看到了错误，而是运维看不到——补上 Warn 即可
- 不改变决策流程的行为，降低风险

### D3: 所有新增日志强制携带 ticketID 结构化字段

**选择**：每条日志都包含 `"ticketID", ticketID` 属性。DecisionExecutor 层通过扩展 request 参数或闭包捕获获取 ticketID。

**替代方案**：用 context.Value 传递 ticketID。

**理由**：
- 显式参数比 context 隐式传递更清晰、更不容易漏
- DecisionExecutor 的 AIDecisionRequest 可以增加一个 `Metadata map[string]any` 字段用于携带调用方上下文（ticketID、serviceID 等），日志时展开
- `grep ticketID=42` 是运维的核心排查手段，不能让任何一条日志漏掉

### D4: Decision Plan 只记摘要，不记完整 JSON

**选择**：Plan 日志记录 `next_step_type`、`confidence`、`len(activities)`、`execution_mode`，不记完整 activities 数组和 reasoning 文本。

**替代方案**：完整 JSON 序列化输出。

**理由**：
- 完整 plan JSON 可达数 KB（reasoning 可能很长），会淹没日志
- 摘要字段足以判断"AI 做了什么类型的决定、信心多高、几个活动"
- 完整 plan 已存入 `activity.ai_decision` 数据库字段，需要深查时有据可查
- 如果后续需要完整输出，可通过日志级别控制（Debug 级别输出完整 JSON）

## Risks / Trade-offs

**[日志量增加] → 可控**
每个决策周期从 ~3 行增加到 ~10-20 行（取决于 tool 调用次数）。对于每分钟处理数十张工单的系统，这仍然是可接受的量级。且全部是 Info/Warn 级别，没有高频 Debug 输出。

**[tool 参数摘要可能丢失关键信息] → 按需 Debug**
为避免日志膨胀，tool 参数只记名称不记完整 JSON。如果需要深度排查，可通过将 slog 默认级别调到 Debug 获取完整参数——但这要求实现运行时级别调节，当前标记为 Non-Goal。

**[跨包修改] → 两个包，改动隔离**
需要同时修改 `internal/app/ai/runtime/decision_executor.go`（AI App）和 `internal/app/itsm/engine/smart.go`（ITSM Engine）。两个包通过 `app.AIDecisionRequest` 接口通信，修改范围清晰隔离。AIDecisionRequest 增加 Metadata 字段是向后兼容的（zero value 为 nil）。

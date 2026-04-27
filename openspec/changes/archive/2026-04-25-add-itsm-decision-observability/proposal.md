## Why

ITSM Agent 的决策链路（从 smart-progress 任务触发到 Decision Plan 执行完毕）在生产环境中几乎是黑箱。当前控制台一个完整决策周期只输出 3 行 Info 日志（turn 开始/结束 + decision completed），tool 调用完全不可见（唯一一行在 Debug 级别被过滤）、tool 错误被静默包装成 JSON 喂给 LLM 后丢弃、决策 plan 和执行结果只写数据库不输出控制台。实际部署可能不接 OTel，运维唯一手段是看 `docker logs` / `journalctl`，无法判断 AI 为什么做了某个决策、tool 是否报错、工单为什么卡住。

## What Changes

- 在 DecisionExecutor ReAct 循环中将 tool 调用从 Debug 提升为结构化 Info 日志，记录 tool 名称、耗时，tool error 单独输出 Warn
- 在 ITSM SmartEngine 决策链路的关键节点补充结构化日志：决策周期入口上下文、agenticDecision 调用参数、Decision Plan 内容（next_step_type / confidence / activities 摘要）、plan 校验结果、plan 执行结果
- 在 smart_tools.go 的 tool handler 包装层统一记录每次 tool 调用的名称、参数摘要、结果摘要、耗时和错误
- 所有日志强制携带 `ticketID` 字段，确保 `grep ticketID=123` 可串联完整链路
- 纯 slog 实现，零外部依赖；OTel 接入时 trace_id/span_id 由现有 traceHandler 自动注入

## Capabilities

### New Capabilities

- `itsm-decision-observability`: ITSM Agent 决策链路的结构化日志覆盖——涵盖 DecisionExecutor tool dispatch、SmartEngine 决策周期全流程、smart_tools handler 执行三层，确保纯控制台输出即可完成决策排查

### Modified Capabilities

（无）

## Impact

- **后端代码**：`internal/app/ai/runtime/decision_executor.go`（tool dispatch 日志）、`internal/app/itsm/engine/smart.go`（决策周期日志）、`internal/app/itsm/engine/smart_tools.go`（tool handler 包装日志）
- **日志量**：每个决策周期从 3 行增加到约 10-20 行（取决于 tool 调用次数），均为 Info/Warn 级别
- **性能**：slog 写 stderr 开销可忽略；tool handler 增加 `time.Now()` 计时，纳秒级
- **兼容性**：无 API 变更、无数据库变更、无前端变更；纯追加日志，不影响现有行为

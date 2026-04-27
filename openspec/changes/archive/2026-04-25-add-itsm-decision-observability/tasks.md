## 1. AIDecisionRequest Metadata 扩展

- [x] 1.1 在 `internal/app/app.go` 的 `AIDecisionRequest` 结构体中添加 `Metadata map[string]any` 字段
- [x] 1.2 在 `internal/app/itsm/engine/smart.go:agenticDecision()` 构建 `AIDecisionRequest` 时填入 `Metadata: map[string]any{"ticketID": ticketID, "serviceID": svc.ID}`

## 2. DecisionExecutor Tool Dispatch 日志

- [x] 2.1 在 `internal/app/ai/runtime/decision_executor.go` 的 tool 调用循环中，将 `slog.Debug` 替换为结构化 Info 日志：记录 tool 名称、耗时（`time.Since`）、ok 状态，并从 `req.Metadata` 展开上下文字段
- [x] 2.2 在 tool 错误路径（`err != nil`）增加 `slog.Warn("decision-tool: error", ...)` 输出 tool 名称、错误信息、ticketID

## 3. SmartEngine 决策周期入口/出口日志

- [x] 3.1 在 `smart.go:runDecisionCycle()` 入口增加 Info 日志 "decision-cycle: starting"，包含 ticketID、triggerReason、serviceID、agentID、decisionMode
- [x] 3.2 在 `runDecisionCycle()` 的 terminal state / AI disabled 分支增加日志 "decision-cycle: skipped"，包含 ticketID 和具体原因
- [x] 3.3 在 `agenticDecision()` 返回后、`validateDecisionPlan()` 前，增加 Info 日志 "decision-cycle: plan"，记录 nextStepType、confidence、activityCount、executionMode
- [x] 3.4 在 `validateDecisionPlan()` 失败路径增加 Warn 日志 "decision-cycle: validation-failed"，包含 ticketID 和校验错误

## 4. SmartEngine Tool Handler 包装层日志

- [x] 4.1 在 `smart.go:agenticDecision()` 的 toolHandler 闭包内增加 logging wrapper：在调用 `handlerMap[name](toolCtx, args)` 前后计时，调用后输出 Info 日志 "decision-tool: call"，错误时输出 Warn "decision-tool: error"
- [x] 4.2 在 handler 未找到（unknown tool）时输出 Warn "decision-tool: unknown"

## 5. Plan 执行结果日志

- [x] 5.1 在 `smart.go:executeSinglePlan()` 创建 activity 和 assignment 后增加 Info 日志 "decision-cycle: executed"，包含 ticketID、activityType、activityID、assigneeID、executionMode="single"
- [x] 5.2 在 `smart.go:executeParallelPlan()` 完成循环后增加 Info 日志 "decision-cycle: executed"，包含 ticketID、activityCount、executionMode="parallel"、groupID
- [x] 5.3 在 `smart.go:handleComplete()` 增加 Info 日志 "decision-cycle: completed"，包含 ticketID
- [x] 5.4 在 `smart.go:pendManualHandlingPlan()` 增加 Info 日志 "decision-cycle: low-confidence"，包含 ticketID、confidence

## 6. 验证

- [x] 6.1 运行 `go build -tags dev ./cmd/server` 确认编译通过
- [x] 6.2 运行 `go test ./internal/app/itsm/engine/...` 确认现有测试通过
- [x] 6.3 运行 `go test ./internal/app/ai/runtime/...` 确认现有测试通过

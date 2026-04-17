## Why

ITSM 服务台智能体的工具链只完成了"定义层"：10 个 ITSM 工具和 3 个通用工具的 handler 函数、参数 schema、seed 数据都已就绪。但"执行层"完全缺失——没有 `ServiceDeskOperator` 具体实现（MatchServices/LoadService/CreateTicket 等方法体为空壳接口）、没有 `StateStore` 持久化实现、没有 `ToolExecutor` 调度层将 Agent Session 的工具调用路由到正确的 handler。`gateway.go:269` 留着 `TODO: inject real ToolExecutor`。智能体发起工具调用时会直接 panic 或返回 nil。

## What Changes

- 新增 `ServiceDeskOperator` 具体实现，将 ITSM 工具 handler 的 6 个方法桥接到已有 ITSM 服务（ServiceDefService、TicketService、FormDefService、ParticipantResolver）
- 新增 `StateStore` 实现，基于 `AgentSession.State` JSON 字段持久化服务台会话状态
- 新增 `CompositeToolExecutor`，按工具名前缀分发到 GeneralToolRegistry / ITSM Registry / 其他 App Registry
- 在 AI App 和 ITSM App 的 IOC 容器中完成注册与注入，替换 gateway.go 的 TODO
- 补齐 ITSM seed 中通用工具的 Agent 绑定（确认 3 个 general tools 绑定正确）

## Capabilities

### New Capabilities
- `itsm-tool-executor`: ServiceDeskOperator 实现 + StateStore 实现 + ITSM 工具链到 ITSM 业务服务的完整桥接
- `ai-tool-dispatch`: CompositeToolExecutor — Agent 运行时工具调用统一分发层

### Modified Capabilities
- `itsm-agent-tools`: Seed 绑定补齐验证，确保 13 个工具全部正确绑定

## Impact

- `internal/app/itsm/tools/` — 新增 operator.go、state_store.go
- `internal/app/ai/` — 新增 tool_executor.go（CompositeToolExecutor）
- `internal/app/ai/gateway.go` — 替换 TODO，注入真实 ToolExecutor
- `internal/app/ai/app.go` — IOC 注册 CompositeToolExecutor
- `internal/app/itsm/app.go` — IOC 注册 ServiceDeskOperator、StateStore、Registry，暴露给 AI App
- `internal/app/itsm/tools/provider.go` — seed 绑定验证/修正

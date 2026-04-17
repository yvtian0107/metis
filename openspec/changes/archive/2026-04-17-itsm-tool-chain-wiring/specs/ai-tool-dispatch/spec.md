## Purpose

Agent 运行时工具调用统一分发层 — CompositeToolExecutor 将工具调用路由到正确的 handler registry。

## Requirements

### Requirement: CompositeToolExecutor 实现 ToolExecutor 接口
CompositeToolExecutor SHALL 实现 `ToolExecutor` 接口的 `ExecuteTool(ctx, ToolCall) (ToolResult, error)` 方法。

#### Scenario: 路由到 GeneralToolRegistry
- **WHEN** ToolCall.Name = "general.current_time"
- **THEN** 系统 SHALL 将调用路由到 GeneralToolRegistry

#### Scenario: 路由到 ITSM Registry
- **WHEN** ToolCall.Name = "itsm.service_match"
- **THEN** 系统 SHALL 将调用路由到 ITSM tools.Registry

#### Scenario: 未知工具
- **WHEN** ToolCall.Name 不匹配任何已注册 registry
- **THEN** 系统 SHALL 返回 ToolResult{IsError: true, Output: "unknown tool: xxx"}

### Requirement: session_id 注入上下文
CompositeToolExecutor SHALL 在调用 registry.Execute 前，将 sessionID 注入 context。

#### Scenario: ITSM 工具获取 sessionID
- **WHEN** ITSM 工具 handler 从 context 中读取 SessionIDKey
- **THEN** SHALL 获取到正确的 AgentSession ID

### Requirement: ToolHandlerRegistry 接口
AI App SHALL 定义 `ToolHandlerRegistry` 接口供各 App 实现。

```go
type ToolHandlerRegistry interface {
    HasTool(name string) bool
    Execute(ctx context.Context, toolName string, userID uint, args json.RawMessage) (json.RawMessage, error)
}
```

#### Scenario: GeneralToolRegistry 已实现
- **GIVEN** GeneralToolRegistry 已有 HasTool 和 Execute 方法
- **THEN** GeneralToolRegistry 自然满足 ToolHandlerRegistry 接口

### Requirement: Gateway 注入
AgentGateway SHALL 在创建 Executor 时注入 CompositeToolExecutor，传入 sessionID 和 userID。

#### Scenario: selectExecutor 使用真实 ToolExecutor
- **WHEN** gateway.selectExecutor 为 assistant 类型 agent 创建 executor
- **THEN** 传入的 ToolExecutor SHALL 为 CompositeToolExecutor 实例，非 nil

### Requirement: 跨 App Registry 收集
AI App SHALL 通过 IOC 收集所有实现了 ToolRegistryProvider 接口的 App 的 registry。

#### Scenario: ITSM App 注册 registry
- **WHEN** ITSM App 已安装
- **THEN** CompositeToolExecutor 的 registries 列表 SHALL 包含 ITSM Registry

#### Scenario: ITSM App 未安装
- **WHEN** ITSM App 未安装
- **THEN** CompositeToolExecutor 仍可正常工作，仅包含 GeneralToolRegistry

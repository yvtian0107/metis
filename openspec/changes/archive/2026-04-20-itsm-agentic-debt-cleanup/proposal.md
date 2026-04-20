## Why

ITSM Agentic 子系统存在 8 项核心技术债务，其中最严重的是：SmartEngine 内部手写了独立的 ReAct 循环和工具系统，与 AI Runtime 的 ReactExecutor 完全重复；Operator 工具层绕过 TicketService 直接写数据库，导致 Agent 创建的工单不启动工作流引擎、不计算 SLA、不记录 timeline——这是一个功能性 bug。这些债务已导致双重维护负担、Agent 工单行为不一致、决策过程不可观测等实际问题，需要立即修复。

## What Changes

### Bug 修复
- **修复 Agent 工单不启动引擎**：`tools/operator.go` 的 `CreateTicket` 改为调用 `TicketService.Create()`，消除两条工单创建路径的行为差异（SLA、引擎启动、timeline）
- **修复 ServiceDeskState stage 转换未集中管理**：将散布在各 handler 中的 stage 转换逻辑抽取为显式状态机

### 架构重构
- **SmartEngine 决策循环统一到 AI Runtime**：删除 `smart_react.go` 手写的 ReAct 循环，改为通过 AI App 的 `AgentGateway` 执行决策 Agent，决策工具注册为标准 `ToolHandlerRegistry`
- **决策工具使用 Repository 层**：`smart_tools.go` 中 8 个工具的 raw `tx.Table()` 查询改为调用已有的 Repository 方法
- **Context key 类型化**：`"ai_session_id"` string key 改为 typed context key，消除隐式耦合

### 代码组织
- **拆分 classic.go**：57KB 单文件按职责拆分为 node executor、token manager、activity lifecycle 等模块
- **System prompt 外置化**：服务台智能体和决策智能体的 system prompt 从 Go string literal 移到 `seed.Sync()` 可更新的数据库记录，支持不重新编译即可更新

## Capabilities

### New Capabilities
- `itsm-decision-runtime`: SmartEngine 决策循环通过 AI Runtime AgentGateway 执行的集成方式，包括 session 创建、工具注册、事件回传

### Modified Capabilities
- `itsm-smart-engine`: 引擎不再自建 LLM client 和 ReAct 循环，改为委托 AI Runtime 执行
- `itsm-smart-react`: 删除独立 ReAct 实现，由 itsm-decision-runtime 取代
- `itsm-decision-tools`: 决策工具从 engine 内部闭包改为标准 ToolHandlerRegistry，数据访问改走 Repository
- `itsm-service-desk-toolkit`: Operator.CreateTicket 改为调用 TicketService，状态机逻辑集中化
- `itsm-classic-engine`: 单文件拆分为多模块，不改变外部行为

## Impact

- **Backend**: `internal/app/itsm/engine/` 下 smart*.go 重大重构；classic.go 拆分；`tools/` 下 operator.go、handlers.go 修改
- **AI App 接口**: 需要 AgentGateway 暴露同步执行接口（当前仅 SSE streaming），或 SmartEngine 通过 internal session 消费
- **数据库**: 无 schema 变更，但 `ai_agent_sessions` 表会新增 SmartEngine 决策用的 internal session 记录
- **API**: 无外部 API 变更
- **测试**: 所有现有 BDD 测试和单元测试必须继续通过；SmartEngine 相关测试需要 mock AgentGateway

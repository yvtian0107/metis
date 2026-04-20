## Context

ITSM Agentic 子系统存在两个并行的 AI 执行路径：

1. **AI Runtime 路径**（用于服务台交互）：`AgentGateway` → `ReactExecutor` → `CompositeToolExecutor` → ITSM ToolHandlerRegistry
2. **SmartEngine 内部路径**（用于流程决策）：`agenticDecision()` → 手写 ReAct 循环 → `decisionToolContext` dispatch

两条路径的 ReAct 循环逻辑几乎一致（LLM 调用 → 解析 tool calls → 执行 → 回传结果），但实现完全独立。SmartEngine 的实现还绕过了 Repository 层直接查表，Operator 工具层绕过 TicketService 直接写数据库导致 Agent 创建的工单不启动引擎。

### 现有测试基础设施

- 38 个测试文件，含 14 个 BDD feature 和完整的单元测试
- SmartEngine BDD 测试通过 `testAgentProvider`（返回 LLM config from env vars）和 `noopSubmitter` 驱动
- 确定性测试通过 `ExecuteConfirmedPlan` 绕过 LLM，直接验证 plan 执行逻辑
- LLM 集成测试需要 `LLM_TEST_*` 环境变量，缺失时 skip

## Goals / Non-Goals

**Goals:**
- 修复 Agent 工单不启动工作流引擎的 bug（D2/D6）
- SmartEngine 决策循环不再手写 ReAct loop，委托给可注入的执行器（D1）
- 决策工具数据访问走 Repository 层（D4）
- classic.go 按职责拆分为多文件（D5）
- System prompt 可通过 seed.Sync() 更新，不需要重编译（D3）
- ServiceDeskState 状态转换集中管理（D7）
- Context key 类型化（D8）
- **所有现有测试保持通过**

**Non-Goals:**
- 不合并服务台工具和决策工具为同一个 registry（它们服务不同场景）
- 不添加决策过程的 streaming/SSE 支持（未来可通过 DecisionExecutor 实现增强）
- 不修改前端代码
- 不变更数据库 schema
- 不变更外部 API

## Decisions

### D1: DecisionExecutor 接口隔离 SmartEngine 与 AI Runtime

**选择**: 在 engine 包定义 `DecisionExecutor` 接口，AI App 提供实现。

```
engine 包定义:
┌──────────────────────────────────────────────────┐
│ type DecisionExecutor interface {                 │
│   Execute(ctx, DecisionRequest) (string, error)  │
│ }                                                │
│                                                  │
│ type DecisionRequest struct {                    │
│   SystemPrompt string                            │
│   UserMessage  string                            │
│   Tools        []llm.ToolDef                     │
│   ToolHandler  func(name, args) (result, error)  │
│   Model, Temperature, MaxTokens, MaxTurns        │
│ }                                                │
└──────────────────────────────────────────────────┘

AI App 实现 (内部):
┌──────────────────────────────────────────────────┐
│ type decisionExecutorImpl struct {                │
│   llmClient llm.Client                           │
│ }                                                │
│                                                  │
│ func (d *impl) Execute(ctx, req) (string, error) │
│   // 使用 llm.Client.Chat (非 streaming)         │
│   // ReAct loop: call LLM → tool calls →         │
│   //   dispatch via req.ToolHandler → loop       │
│   // 返回最终 content string                      │
│ }                                                │
└──────────────────────────────────────────────────┘
```

**原因**:
- engine 包不依赖 ai 包，保持现有的接口隔离模式
- `ToolHandler` 是闭包，SmartEngine 在每次决策前绑定当前 ticket 上下文，无需改变决策工具签名
- AI App 的实现内部使用 `llm.Client.Chat`（同步），不需要 streaming
- 测试可以注入 mock DecisionExecutor，不需要真实 LLM

**替代方案（否决）**:
- 让 SmartEngine 直接使用 `ai.ReactExecutor`：引入 engine → ai 的直接包依赖，违反插件架构
- 让 SmartEngine 创建 AgentSession 并调用 Gateway.Run()：需要 session 管理 + stream→sync 转换，过度复杂

### D1-impl: 删除 smart_react.go，重写 agenticDecision

删除 `smart_react.go` 中 `agenticDecision()` 的手写 ReAct 循环。新实现：

```go
func (e *SmartEngine) agenticDecision(ctx, tx, ticketID, svc) (*DecisionPlan, error) {
    // 1. 构建 system/user messages（复用现有 buildInitialSeed）
    // 2. 构建 toolHandler 闭包（包裹 decisionToolContext）
    // 3. 调用 e.decisionExecutor.Execute(ctx, DecisionRequest{...})
    // 4. parseDecisionPlan(content)
}
```

`SmartEngine` 新增 `decisionExecutor DecisionExecutor` 字段，通过 `NewSmartEngine()` 注入。

### D1-impl: AI App 提供 DecisionExecutor

在 `internal/app/ai/` 中添加 `decision_executor.go`：

```go
type decisionExecutorImpl struct{}

func (d *decisionExecutorImpl) Execute(ctx, req) (string, error) {
    client, err := llm.NewClient(...)  // 从 req 中获取
    // ReAct loop (从 smart_react.go 提取，简化)
    messages := []{system, user}
    for turn := 0; turn < req.MaxTurns; turn++ {
        resp := client.Chat(ctx, chatReq)
        if no tool calls → return resp.Content, nil
        for each tool call → req.ToolHandler(name, args)
        append results to messages
    }
}
```

但这里有个问题：`DecisionExecutor` 的实现需要 `llm.Client`，而 client 的创建需要 protocol/baseURL/apiKey。目前这些信息通过 `AgentProvider.GetAgentConfig()` 获取，并在 `agenticDecision` 内部创建 client。

**方案调整**: `DecisionExecutor` 接口由 AI App gateway 提供工厂方法，SmartEngine 传入 agentID：

```go
// engine 包
type DecisionExecutor interface {
    Execute(ctx context.Context, agentID uint, req DecisionRequest) (string, error)
}
```

AI App 的实现通过 agentID 查找 model/provider，创建 client，执行 ReAct loop。这样 SmartEngine 不需要自己管理 LLM client 创建。同时 AI App 的 `GetAgentConfig` 接口中不再需要暴露 APIKey 等敏感信息。

### D2/D6: Operator 通过 TicketCreator 接口调用 TicketService

**选择**: 定义 `TicketCreator` 接口，TicketService 实现它。

```go
// tools 包
type TicketCreator interface {
    CreateFromAgent(ctx context.Context, req AgentTicketRequest) (*AgentTicketResult, error)
}

type AgentTicketRequest struct {
    RequesterID uint
    ServiceID   uint
    Summary     string
    FormData    map[string]any
    SessionID   uint
}
```

**原因**: Operator 当前直接写数据库，遗漏了 SLA 计算、引擎启动、timeline 记录。通过接口调用 TicketService.Create() 复用完整业务逻辑。

**实现**: TicketService 新增 `CreateFromAgent` 方法，内部调用已有的 `Create()` 逻辑，但 source 设为 "agent"，session_id 绑定。Operator.CreateTicket 删除所有直接 DB 操作。

### D3: System Prompt seed.Sync() 可更新

**选择**: SeedAgents 改为 upsert 模式（当前仅在 agent 不存在时创建）。

```go
// 修改 SeedAgents:
// - 若 agent 存在且 system_prompt 与代码中定义不同 → 更新
// - 用 agent.code 做匹配（不依赖 name，name 可能被用户修改）
```

**原因**: 当前改 prompt 需要改 Go 代码 + 重新编译 + 重新部署。upsert 后，每次 `seed.Sync()` 自动将 prompt 更新到最新版本。用户自定义的修改会被覆盖——这是期望行为，因为 ITSM 预置智能体的 prompt 是产品行为的一部分。

### D4: 决策工具使用 Repository

**选择**: 将 `decisionToolContext` 中的 `*gorm.DB` 替换为具体的 Repository 引用。

```go
type decisionToolContext struct {
    ticketRepo    TicketQueryRepo   // 新接口，子集
    activityRepo  ActivityQueryRepo
    assignmentRepo AssignmentQueryRepo
    // ... 其他查询接口
    knowledgeSearcher KnowledgeSearcher
    resolver          *ParticipantResolver
    actionExecutor    *ActionExecutor
}
```

**原因**: 8 个决策工具中有约 20 处 `tx.Table()` raw 查询，而 Repository 层已经提供了相同功能。使用 Repository：
- 消除 table name 硬编码
- 复用 soft delete/join 逻辑
- 利于测试 mock

**注意**: 部分查询在 Repository 中不存在（如 ticket context 的 executed_actions join 查询），需要新增 Repository 方法。

### D5: classic.go 拆分方案

```
classic.go (57KB)
  ↓ 拆分为
classic_core.go     ← Start/Progress/Cancel + 图遍历 + 入口
classic_nodes.go    ← 各节点类型处理 (processNode, processApprove, processAction...)
classic_activity.go ← Activity 创建/更新/查询
classic_token.go    ← Token 树操作 (create, complete, fork, join)
classic_notify.go   ← 通知分发
classic_helpers.go  ← 辅助函数 (type aliases, JSON helpers)
```

**原则**: 纯文件重组，不改变任何函数签名或行为。所有函数留在 `engine` 包内，仅改变文件归属。

### D7: ServiceDeskState 显式状态机

```go
// 当前状态 → 允许的目标状态
var validTransitions = map[string][]string{
    "idle":                  {"candidates_ready"},
    "candidates_ready":      {"service_selected", "idle"},
    "service_selected":      {"service_loaded", "idle"},
    "service_loaded":        {"awaiting_confirmation", "service_loaded", "idle"},
    "awaiting_confirmation": {"confirmed", "service_loaded", "idle"},
    "confirmed":             {"idle"},
}

func (s *ServiceDeskState) TransitionTo(next string) error {
    allowed := validTransitions[s.Stage]
    if !slices.Contains(allowed, next) {
        return fmt.Errorf("invalid transition: %s → %s", s.Stage, next)
    }
    s.Stage = next
    return nil
}
```

### D8: Typed Context Keys

```go
// internal/app/itsm/tools/context.go
type contextKey string
const (
    SessionIDKey contextKey = "ai_session_id"
)
```

对应修改 AI App 的 `tool_executor.go` 中注入 session ID 的代码，使用相同的 typed key。为了避免包依赖，将 key 类型定义在 `internal/app/` 下的共享位置。

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| D1: DecisionExecutor 改变了 SmartEngine 的 mock 点，BDD 测试需要适配 | 确定性测试不受影响（直接调用 ExecuteConfirmedPlan）；LLM 集成测试改为 mock DecisionExecutor 返回预设 content |
| D1: AI App 的 DecisionExecutor 实现内部有自己的 ReAct loop，仍有一定重复 | 该 loop 是简化版（同步 + 无 streaming），且与 ReactExecutor 共享 llm.Client；未来可提取公共函数 |
| D2: Operator → TicketService 引入了 tools → service 层的依赖 | 通过 TicketCreator 接口隔离，不直接依赖 service 实现 |
| D3: seed.Sync() 更新 prompt 会覆盖用户自定义修改 | 预置智能体的 prompt 是产品行为，不应被用户修改；若需自定义，复制为新 agent |
| D5: classic.go 拆分可能引入临时编译错误 | 一次性拆分 + 立即 `go build` 验证；不改变任何函数签名 |

## Implementation Order

```
Phase 1 (低风险，修 bug)
  D2/D6: Operator → TicketCreator 接口
  D8: Typed context keys
  D7: State machine

Phase 2 (中风险，重构)
  D4: 决策工具 → Repository
  D5: classic.go 拆分
  D3: Prompt seed.Sync() 更新

Phase 3 (高风险，架构)
  D1: DecisionExecutor 接口 + AI App 实现
  D1: 删除 smart_react.go，重写 agenticDecision
  测试适配
```

每个 Phase 完成后跑全量测试确认无回归。

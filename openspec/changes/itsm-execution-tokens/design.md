## Context

当前 ClassicEngine 使用 `ticket.current_activity_id` 单指针模型追踪执行位置。整个引擎的核心循环是 `processNode()` 同步递归——遇到自动节点（gateway/action/notify）递归推进，遇到人工节点（form/approve/process/wait）创建 Activity 后停止。

这个模型在排他分支（exclusive gateway）场景下工作良好，但不支持：
- **并行分支**：fork 出多条同时活跃的路径
- **子流程**：独立的执行上下文，有自己的变量作用域
- **边界事件**：附着在任务上的并行监听器

本 change 引入 ExecutionToken 模型，为后续 ④⑤⑥⑦ 打下基础，但**本身只实现单 token 顺序执行**，确保 token 基础设施正确后再逐步扩展。

### 当前引擎调用链

```
TicketService.Create()
  → ClassicEngine.Start(tx, StartParams)
    → parseWorkflow → findStart → processNode(targetNode, depth=0)
      → handleForm/Approve/Process/Wait: 创建 Activity, 更新 ticket.current_activity_id

TicketService.Progress()
  → ClassicEngine.Progress(tx, ProgressParams)
    → 加载 activity → 完成 activity → matchEdge → processNode(targetNode, depth=0)
      → handleGateway: buildEvalContext → evaluateCondition → processNode(matchedTarget, depth+1)
```

关键约束：
- `processNode` 是同步递归，在单次 DB 事务内完成
- `ticket.current_activity_id` 是唯一的执行位置指针
- Activity 通过 `ticket_id` 直接关联工单，无 token 概念
- `IsAutoNode()` 判断 gateway/action/notify 为自动节点
- `NodeGateway = "gateway"` 是唯一的网关类型

## Goals / Non-Goals

**Goals:**
- 引入 ExecutionToken 模型（`itsm_execution_tokens` 表），支持 token 树结构
- 定义 token 状态机（active/waiting/completed/cancelled/suspended）
- Activity 新增 `token_id` FK，绑定到执行路径
- 将 `gateway` 节点类型重命名为 `exclusive`，同时注册 `parallel`、`inclusive` 类型常量（执行逻辑在 ④ 中实现）
- 重构 ClassicEngine 核心循环为 token-based：Start 创建 root token，processNode 基于 token 推进
- ValidateWorkflow 增强：识别新节点类型，对未实现类型给出友好提示
- `ticket.current_activity_id` 保留，语义改为"最近活跃 token 的当前活动"（便于列表页展示）

**Non-Goals:**
- 并行网关的 fork/join 执行逻辑（属于 ④ itsm-gateway-parallel）
- 子流程节点的实际执行（属于 ⑤ itsm-advanced-nodes）
- 边界事件（timer/error boundary）的实际执行（属于 ⑤）
- Script 节点执行（属于 ⑤）
- 前端变更（token 对终端用户不可见）
- 数据迁移（开发阶段，可删库重建）

## Decisions

### D1: Token 状态机

```
  ┌──────────┐
  │  active   │──── 正常完成 ───▶ completed
  └────┬──────┘
       │ fork (④中实现)
       ▼
  ┌──────────┐
  │ waiting   │──── 所有子token完成 ───▶ completed
  └──────────┘

  任何非 completed 状态 ──── cancel ───▶ cancelled

  suspended: 仅定义常量，实际使用在 ⑤ 中（边界事件挂起恢复）
```

**状态定义：**
- `active`：token 正在执行，有且仅有一个活跃 Activity
- `waiting`：token 已 fork 出子 token，等待子 token 全部完成后 join（④ 实现）
- `completed`：token 执行完毕
- `cancelled`：token 被取消（级联或显式）
- `suspended`：仅定义常量，本 change 不使用

**替代方案考虑：** 不定义 `suspended`，等 ⑤ 再加。**选择定义但不实现**——提前注册常量避免后续改动 token 状态机。

### D2: Token 类型

```go
const (
    TokenMain          = "main"           // root token，每个工单有且仅有一个
    TokenParallel      = "parallel"       // 并行网关 fork 出的子 token（④）
    TokenSubprocess    = "subprocess"     // 子流程 token（⑤）
    TokenMultiInstance = "multi_instance" // 多实例 token（⑤）
    TokenBoundary      = "boundary"       // 边界事件 token（⑤）
)
```

本 change 只使用 `main` 类型。其余类型仅定义常量。

### D3: ExecutionToken 模型

```
itsm_execution_tokens
├── id (PK)
├── ticket_id (FK, index)
├── parent_token_id (self-ref, nullable, 树结构)
├── node_id (当前所在节点 ID, string)
├── status (active/waiting/completed/cancelled/suspended)
├── token_type (main/parallel/subprocess/multi_instance/boundary)
├── scope_id (变量作用域, default "root", 与 process_variables 联动)
├── created_at
├── updated_at
```

**关键索引：** `idx_ticket_status` (ticket_id, status) — 查询某工单所有活跃 token。

### D4: Activity 双 FK 策略

Activity 新增 `token_id` 字段：

```go
type activityModel struct {
    // ... existing fields
    TokenID  *uint `gorm:"column:token_id;index"` // nullable for backward compat during migration
}
```

**为什么保留 `ticket_id`**：Activity 的大量查询是"查询某工单的所有 Activity"，保留 `ticket_id` 避免 JOIN token 表。`token_id` 用于精确绑定执行路径。

**替代方案：** 只用 token_id，通过 token.ticket_id 反查。**选择双 FK**——查询性能优先，冗余可控。

### D5: gateway → exclusive 重命名

**破坏性变更**：

```go
// Before
NodeGateway = "gateway"

// After
NodeExclusive = "exclusive"
NodeParallel  = "parallel"   // 注册常量，④ 实现执行逻辑
NodeInclusive = "inclusive"   // 注册常量，④ 实现执行逻辑
```

同步更新：
- `ValidNodeTypes` map
- `IsAutoNode()` 函数
- `handleGateway` → `handleExclusive`
- `condition.go` 中的引用
- Validator 中的 gateway 校验规则
- 前端 workflow editor 中的节点类型常量

**理由**：开发阶段，无生产数据需要迁移。早期重命名比后期兼容层成本低。

### D6: processNode 签名变更

```go
// Before
func (e *ClassicEngine) processNode(
    ctx, tx, def, nodeMap, outEdges,
    ticketID, operatorID uint,
    node *WFNode, depth int,
) error

// After
func (e *ClassicEngine) processNode(
    ctx, tx, def, nodeMap, outEdges,
    token *executionTokenModel, operatorID uint,
    node *WFNode, depth int,
) error
```

所有 handler（handleForm/Approve/Process/Action/Wait/End/Exclusive/Notify）签名同步变更，从 `ticketID uint` 改为 `token *executionTokenModel`，通过 `token.TicketID` 获取工单 ID。

**保持同步递归模式**：本 change 不引入异步 token 调度器。processNode 仍然在单次事务内同步递归。异步 token 调度在 ④（并行网关需要）时再引入。

### D7: 新节点类型常量注册

仅注册常量和 ValidNodeTypes，不实现执行逻辑：

```go
// Gateway 族
NodeExclusive = "exclusive"  // 本 change 实现
NodeParallel  = "parallel"   // ④ 实现
NodeInclusive = "inclusive"   // ④ 实现

// 高级节点 — ⑤ 实现
NodeScript    = "script"
NodeSubprocess = "subprocess"
NodeTimer      = "timer"      // 中间定时事件
NodeSignal     = "signal"     // 中间信号事件

// 边界事件 — ⑤ 实现
NodeBTimer    = "b_timer"     // 边界定时事件
NodeBError    = "b_error"     // 边界错误事件
```

Validator 对未实现类型输出友好提示而非报错（"节点类型 X 已注册但执行逻辑尚未实现"）。

### D8: ClassicEngine 重构流程

**Start 流程变更：**
```
Before: Start → findStartNode → processNode(ticketID, targetNode)
After:  Start → findStartNode → createRootToken(ticketID) → processNode(token, targetNode)
```

**Progress 流程变更：**
```
Before: Progress → loadActivity → completeActivity → matchEdge → processNode(ticketID, targetNode)
After:  Progress → loadActivity → loadToken(activity.TokenID) → completeActivity → matchEdge → processNode(token, targetNode)
```

**Cancel 流程变更：**
```
Before: Cancel → 批量更新 activities.status → 更新 ticket.status
After:  Cancel → 查找所有活跃 token → 递归取消 token + activities → 更新 ticket.status
```

## Risks / Trade-offs

**[R1] 单 token 阶段引入 token 表增加复杂度** → 本 change 结束后行为与之前完全一致（单 token 顺序执行），但每次操作多一次 token 表读写。权衡：提前付出 ~5% 的性能开销，换取 ④⑤ 的平滑扩展。

**[R2] gateway → exclusive 破坏性重命名** → 所有现有 workflow_json 中的 `"type": "gateway"` 节点将无法被识别。Mitigation: 开发阶段删库重建，前端编辑器同步更新节点类型。

**[R3] Activity.token_id nullable** → 为了平滑过渡，token_id 设为 nullable。但本 change 实现后所有新 Activity 都会有 token_id。Mitigation: 后续可考虑加 NOT NULL 约束（当确认无旧数据时）。

**[R4] processNode 同步递归 + token 表写入** → 每次递归都更新 token.node_id，增加事务内写操作。Mitigation: 自动节点（exclusive/action/notify）递归深度受 MaxAutoDepth=50 限制。

**[R5] suspended 状态未使用可能造成混淆** → 开发者可能误用。Mitigation: 注释明确标注 "reserved for ⑤, do not use in ③④"。

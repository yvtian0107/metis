## Context

当前工单撤回存在两个问题：

1. **逻辑缺失**：`tools/operator.WithdrawTicket` 仅检查 "是否有已完成 activity"，不检查认领状态。审批人认领后（`claimed_at IS NOT NULL`）申请人仍可撤回，违反业务预期。
2. **架构绕行**：`WithdrawTicket` 使用原始 SQL 操作，不经过 `engine.Cancel`，导致 execution tokens、activities、assignments 未被正确清理。

需要一个正式的 `TicketService.Withdraw` 方法，BDD 场景先行定义正确行为，再驱动代码修复。

## Goals / Non-Goals

**Goals:**
- BDD 场景覆盖撤回的 4 个核心场景（成功、认领阻止、非申请人拒绝、时间线记录）
- 新增 `TicketService.Withdraw`，统一撤回语义
- `tools/operator.WithdrawTicket` 改为委托 service 方法，消除原始 SQL

**Non-Goals:**
- 不修改 `engine.Cancel` 本身的清理逻辑
- 不新增 `engine.Withdraw` 方法（复用 Cancel 即可）
- 不涉及前端 UI 撤回按钮

## Decisions

### D1: Withdraw 检查 claimed_at 而非 activity status

**选择**: 查询 `TicketAssignment` 表中 `claimed_at IS NOT NULL` 作为认领判定

**理由**: `Claim()` 方法同时设置 `assignee_id` 和 `claimed_at`，但 `claimed_at` 是最明确的时间语义信号——有人在某个时间点认领了。`assignee_id` 也可能通过其他路径被设置（如自动分配），而 `claimed_at` 只有主动认领才会写入。

**替代方案**: 检查 `assignee_id IS NOT NULL` —— 被否决，因为分配 ≠ 认领。

### D2: 复用 engine.Cancel，Withdraw 层记录 timeline

**选择**: `TicketService.Withdraw` 调用 `engine.Cancel(CancelParams)` 完成 token/activity/assignment/ticket 清理，然后自行写入 event_type="withdrawn" 的 timeline 记录。

**理由**: 引擎清理逻辑完全相同（取消 tokens、activities、assignments、更新 ticket status）。唯一区别是 timeline 事件语义。在 service 层区分即可，不需要给引擎增加复杂度。

**实现**: `engine.Cancel` 内部已写 event_type="ticket_cancelled" 的 timeline。Withdraw 调用后再追加一条 event_type="withdrawn" 记录，包含撤回原因。或者修改 `CancelParams` 增加 `EventType` 字段让引擎写正确的事件。后者更干净——选择后者。

### D3: tools/operator.WithdrawTicket 委托 service

**选择**: `Operator` 持有 `TicketService` 引用（或通过接口），`WithdrawTicket` 解析 ticket_code → ticket_id 后调用 `svc.Withdraw()`。

**理由**: 消除原始 SQL 重复逻辑，单一真相源。tool handler 只负责参数解析和格式化返回。

## Risks / Trade-offs

- **engine.Cancel timeline 会多写一条 "ticket_cancelled"** → 通过给 CancelParams 增加 EventType/Message 字段解决，让 Cancel 写 "withdrawn" 而非 "ticket_cancelled"
- **Operator 依赖 TicketService 需要 IOC 注入调整** → Operator 构造函数已接受 `*gorm.DB`，改为额外接受 service 或通过闭包传入

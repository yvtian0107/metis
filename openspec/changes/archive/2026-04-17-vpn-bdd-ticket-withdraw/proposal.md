## Why

验证申请人可以在工单尚未被处理前撤回 VPN 开通申请，以及各种撤回失败场景的正确处理。

当前撤回逻辑仅存在于 `tools/operator.WithdrawTicket`，使用原始 SQL，不经过引擎清理，且不检查认领状态。BDD 驱动修复：新增 `TicketService.Withdraw` 方法，统一撤回语义。

参考来源：bklite-cloud `tests/bdd/itsm/features/vpn_ticket_withdraw.feature`

## What Changes

### BDD 测试（新增）
- `features/vpn_ticket_withdraw.feature`：4 个 Scenario
  - 无人认领时成功撤回
  - 已被审批人认领后撤回失败
  - 非申请人撤回失败
  - 撤回原因记录在时间线
- `steps_vpn_withdraw_test.go`：撤回步骤实现 + 时间线断言

### 业务逻辑（BDD 驱动修复）
- `ticket_service.go`：新增 `Withdraw(ticketID, reason, operatorID)` 方法
  - 校验 requester_id == operatorID
  - 校验无 claimed_at IS NOT NULL 的 assignment
  - 委托 engine.Cancel 执行清理，timeline event_type 为 "withdrawn"
- `ticket_handler.go`：新增 Withdraw handler
- `tools/operator.go`：`WithdrawTicket` 改为调用 `TicketService.Withdraw`
- 新增 sentinel errors：`ErrNotRequester`, `ErrTicketClaimed`

## Capabilities

### Modified Capabilities
- `itsm-bdd-infrastructure`: 增加工单撤回步骤和时间线断言

## Impact

- `internal/app/itsm/features/vpn_ticket_withdraw.feature` (new)
- `internal/app/itsm/steps_vpn_withdraw_test.go` (new)
- `internal/app/itsm/ticket_service.go` (modified — Withdraw method)
- `internal/app/itsm/ticket_handler.go` (modified — Withdraw handler)
- `internal/app/itsm/tools/operator.go` (modified — delegate to service)
- `internal/app/itsm/tools/handlers.go` (modified — use service)

## Dependencies

- vpn-bdd-main-flow (已完成) — 复用工单创建和流转步骤

## 1. BDD Feature 文件

- [x] 1.1 创建 `features/vpn_ticket_withdraw.feature`：Background（参与人 + 协作规范 + 发布经典引擎服务）+ 4 个 Scenario（成功撤回、认领后失败、非申请人失败、时间线记录）

## 2. 业务逻辑（BDD 驱动）

- [x] 2.1 `engine/classic.go`：`CancelParams` 增加 `EventType` 和 `Message` 可选字段，Cancel 方法优先使用这些字段写 timeline（默认仍为 "ticket_cancelled"）
- [x] 2.2 `ticket_service.go`：新增 `ErrNotRequester` 和 `ErrTicketClaimed` sentinel errors
- [x] 2.3 `ticket_service.go`：新增 `Withdraw(ticketID uint, reason string, operatorID uint) (*Ticket, error)` 方法，校验 requester_id、claimed_at、调用 engine.Cancel（EventType="withdrawn"）
- [x] 2.4 `ticket_handler.go`：新增 `Withdraw` handler，路由注册 `PUT /tickets/:id/withdraw`
- [x] 2.5 `tools/operator.go`：`WithdrawTicket` 改为通过 `TicketService.Withdraw` 委托执行，移除原始 SQL 逻辑

## 3. BDD Steps 实现

- [x] 3.1 创建 `steps_vpn_withdraw_test.go`：实现 `registerWithdrawSteps(sc, bc)`
- [x] 3.2 实现 When 步骤 `^"([^"]*)" 撤回工单，原因为 "([^"]*)"$`：调用 `TicketService.Withdraw`，失败时存入 `bc.lastErr`
- [x] 3.3 实现 When 步骤 `^"([^"]*)" 认领当前工单$`：查 TicketAssignment 设置 assignee_id + claimed_at
- [x] 3.4 实现 Then 步骤 `^操作失败$`：断言 `bc.lastErr != nil`
- [x] 3.5 实现 Then 步骤 `^时间线包含 "([^"]*)"$`：查 TicketTimeline 断言存在 message 包含指定文本的记录
- [x] 3.6 实现 Then 步骤 `^时间线包含撤回记录$`：查 TicketTimeline 断言存在 event_type="withdrawn" 的记录

## 4. 注册与验证

- [x] 4.1 `bdd_test.go`：`initializeScenario` 调用 `registerWithdrawSteps(sc, bc)`
- [x] 4.2 运行 `go test ./internal/app/itsm/ -run TestBDD -v` 验证 4 个 withdraw scenarios 全部 green

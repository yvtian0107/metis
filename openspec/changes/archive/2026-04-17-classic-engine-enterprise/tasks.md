## 1. Scheduler ErrNotReady 支持

- [x] 1.1 在 `internal/scheduler/` 中定义 `ErrNotReady` 哨兵错误
- [x] 1.2 修改 scheduler 异步任务 handler 的 error handling：识别 `errors.Is(err, ErrNotReady)` 时保留任务不标记完成
- [x] 1.3 单元测试：handler 返回 ErrNotReady 时任务保留在队列中

## 2. 并发安全 — 行级锁

- [x] 2.1 `classic.go:Progress()` — activity 和 token 的 `tx.First()` 加 `clause.Locking{Strength: "UPDATE"}`
- [x] 2.2 `classic.go:tryCompleteJoin()` — parent token 查询加 FOR UPDATE，Count 改为锁后执行
- [x] 2.3 `tasks.go:HandleWaitTimer` — activity 状态检查加 FOR UPDATE
- [x] 2.4 `tasks.go:HandleBoundaryTimer` — boundary token 和 host activity 查询加 FOR UPDATE
- [x] 2.5 BDD 测试：并发 Progress 同一 activity（验证只有一个成功）

## 3. 定时器可靠性

- [x] 3.1 `tasks.go:HandleWaitTimer` — 未到期时返回 `scheduler.ErrNotReady` 替代 nil
- [x] 3.2 `tasks.go:HandleBoundaryTimer` — 未到期时返回 `scheduler.ErrNotReady` 替代 nil
- [x] 3.3 单元测试：timer 未到期返回 ErrNotReady，到期正常执行

## 4. 多人审批

- [x] 4.1 新增 `classic.go:progressApproval()` 私有方法，封装 parallel/sequential/single 分支
- [x] 4.2 实现 parallel（会签）逻辑：完成当前 assignment，Count 未完成 assignments，全部完成才 complete activity；任一 reject 立即 complete
- [x] 4.3 实现 sequential（依次）逻辑：完成当前 is_current assignment，推进 is_current 到下一个；最后一个完成时 complete activity
- [x] 4.4 `Progress()` 中对 approve 类型 activity 调用 `progressApproval()` 替代直接完成
- [x] 4.5 BDD 测试：parallel 全部通过、parallel 一人拒绝、sequential 链式推进、single 模式不变

## 5. 通知集成

- [x] 5.1 在 `engine/` 包定义 `NotificationSender` 接口：`Send(ctx, channelID uint, subject, body string, recipientIDs []uint) error`
- [x] 5.2 `ClassicEngine` 结构体增加 `notifier NotificationSender` 可选字段
- [x] 5.3 `handleNotify()` 调用 `notifier.Send()`，失败仅记 timeline warning
- [x] 5.4 实现模板变量替换：`{{ticket.code}}`、`{{ticket.status}}`、`{{activity.name}}`、`{{var.xxx}}`
- [x] 5.5 `app.go` 中从 IOC 解析 `service.MessageChannelService` 并适配为 NotificationSender 注入
- [x] 5.6 单元测试：mock NotificationSender 验证调用参数和失败降级

## 6. 条件表达式扩展

- [x] 6.1 `condition.go:evaluateCondition()` 新增操作符：`in`、`not_in`、`is_empty`、`is_not_empty`、`between`、`matches`
- [x] 6.2 `GatewayCondition` 结构体新增 `Logic string` 和 `Conditions []GatewayCondition` 字段
- [x] 6.3 `evaluateCondition()` 支持递归求值复合条件（Logic="and"/"or"）
- [x] 6.4 `validator.go` 校验复合条件的 Logic 值合法性
- [x] 6.5 单元测试：每个新操作符 + 复合 AND/OR + 嵌套复合 + 向后兼容单条件

## 7. SLA 主动监控

- [x] 7.1 Ticket 模型新增 `sla_paused_at *time.Time` 字段，AutoMigrate 注册
- [x] 7.2 实现 `itsm-sla-check` cron 任务 handler：扫描 in_progress 且非 paused 且 sla_status 非终态的工单
- [x] 7.3 breach 检测逻辑：比较 response_deadline/resolution_deadline 与当前时间，更新 sla_status
- [x] 7.4 升级规则匹配与执行：加载 EscalationRule，按 trigger_type 匹配，执行 notify/reassign/escalate_priority
- [x] 7.5 `app.go` 注册 `itsm-sla-check` 为 cron 任务（60 秒间隔）
- [x] 7.6 SLA pause API：`PUT /api/v1/itsm/tickets/:id/sla/pause`，设置 sla_paused_at
- [x] 7.7 SLA resume API：`PUT /api/v1/itsm/tickets/:id/sla/resume`，计算暂停时长并延长 deadline
- [x] 7.8 单元测试：breach 检测、escalation 执行、pause/resume deadline 计算

## 8. 任务转办/委派/抢单

- [x] 8.1 Assignment 模型新增字段：`delegated_from *uint`、`transfer_from *uint`；新增状态常量
- [x] 8.2 `ticket_service.go` 新增 `Transfer()` 方法：创建新 assignment + 标记原 transferred + 更新 assignee
- [x] 8.3 `ticket_service.go` 新增 `Delegate()` 方法：创建新 assignment(delegated_from) + 标记原 delegated
- [x] 8.4 `ticket_service.go` 新增 `Claim()` 方法：标记调用者 claimed + 其他 claimed_by_other + 更新 assignee
- [x] 8.5 委派自动回归：Progress 完成 assignment 时检查 delegated_from，若非空则创建回归 assignment
- [x] 8.6 `ticket_handler.go` 新增三个 API 端点路由注册
- [x] 8.7 单元测试：transfer 流转、delegate 回归、claim 排他、权限校验

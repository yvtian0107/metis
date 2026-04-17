## Context

ITSM 经典引擎（ClassicEngine）基于 token 树 + 递归 processNode 实现 BPMN 图遍历。当前实现在单用户串行操作下行为正确，但在企业级多用户并发场景下存在竞态条件、审批模式未完整实现、通知空实现、SLA 无主动监控等问题。

现状：
- `Progress()` 读取 activity/token 时无行级锁，并发可双重推进
- `handleApprove()` 创建 activity 并设 execution_mode，但 Progress 不区分审批模式
- Notify 节点仅写 timeline，未对接通知渠道
- wait-timer/boundary-timer 返回 nil 可能导致 scheduler 标记任务完成
- SLA 只在创建时计算 deadline，无周期检查和升级
- 无转办/委派/抢单操作

## Goals / Non-Goals

**Goals:**
- 消除 Progress/Join 的并发竞态，保证多用户操作安全
- 多人审批模式（会签/依次/单人）实际生效
- Notify 节点对接 Kernel MessageChannel 发送真实通知
- 定时器任务在 scheduler 层面可靠（未到期不消费）
- SLA 主动监控 + 升级规则自动执行
- 支持转办/委派/抢单三种任务流转操作
- 条件表达式支持更多操作符和复合条件

**Non-Goals:**
- 不引入分布式锁（当前单实例 SQLite/PostgreSQL，行级锁足够）
- 不做 Call Activity（可复用子流程），保留嵌入式 subprocess
- 不做中间信号/定时事件（NodeTimer/NodeSignal）
- 不做业务日历（工作时间/节假日排除），SLA 仍基于 wall-clock
- 不做工作流版本管理
- 不重构 processNode 的递归模型

## Decisions

### D1: 并发锁策略 — 悲观锁（FOR UPDATE）

**选择**: 在 Progress() 中对 activity 和 token 查询加 `clause.Locking{Strength: "UPDATE"}`。

**替代方案**: 乐观锁（version 列 + CAS）。
**选择原因**: 悲观锁实现简单，ITSM 工单操作频率低（不是高并发热点），锁持有时间短（单个 Progress 事务），SQLite WAL 模式和 PostgreSQL 都支持。乐观锁需要改模型 + 重试循环，复杂度高但收益小。

**影响范围**:
- `classic.go:Progress()` — activity 和 token 的 First() 加 FOR UPDATE
- `classic.go:tryCompleteJoin()` — Count() 改为先 FOR UPDATE 锁 parent token
- `tasks.go:HandleWaitTimer/HandleBoundaryTimer` — activity 状态检查加锁

### D2: 定时器可靠性 — 引入 ErrNotReady 哨兵错误

**选择**: 定义 `scheduler.ErrNotReady` 哨兵错误。Timer handler 在未到期时返回此错误，scheduler 识别后不标记完成、保留任务等待下次轮询。

**替代方案**: 改用 delay queue（到期时间前不出队）。
**选择原因**: 当前 scheduler 是轮询模式（每 3s 拉取），改为 delay queue 需要重构 scheduler 核心。引入哨兵错误是最小侵入方案，只需 scheduler 在 error handling 中识别 `errors.Is(err, ErrNotReady)` 即可。

### D3: 多人审批 — Activity 级 counting + Assignment 状态推进

**选择**:
- **会签 (parallel)**: Progress 时不直接完成 activity，而是完成当前 assignment 并检查是否所有 assignment 都已完成。全部完成后才 complete activity 并推进。任一 reject 立即推进为 reject outcome。
- **依次 (sequential)**: Progress 时完成当前 assignment（is_current=true），如果还有后续 assignment 则推进 is_current 到下一个，不推进 activity。最后一个完成后推进 activity。

**影响**: Progress() 在遇到 approve 类型 activity 时需要分支处理。新增 `progressApproval()` 私有方法封装多人逻辑。

### D4: 通知集成 — 通过 IOC 解耦

**选择**: engine 包定义 `NotificationSender` 接口，由 app.go 从 IOC 容器解析 `service.MessageChannelService` 并适配注入。Notify 节点调用 `sender.Send(channelID, templateVars)` 发送通知。

**替代方案**: engine 直接依赖 `service.MessageChannelService`。
**选择原因**: engine 包不应直接依赖 service 包（避免循环依赖），通过接口解耦符合现有架构模式（与 OrgService/TaskSubmitter 一致）。

### D5: SLA 监控 — 周期任务 + 升级规则

**选择**:
- 新增 `itsm-sla-check` cron 任务（每 60 秒执行）
- 扫描 `itsm_tickets` 中 status=in_progress 且 sla_status != breached_resolution 的工单
- 检测 response_deadline/resolution_deadline 是否已过期
- 匹配 EscalationRule，执行升级动作（notify/reassign/escalate_priority）
- SLA 暂停/恢复：ticket 新增 `sla_paused_at` 字段，暂停时记录时间，恢复时将 deadline 向后推移暂停时长

### D6: 转办/委派/抢单 — Assignment 状态机扩展

**选择**:
- **Transfer（转办）**: 创建新 assignment（status=pending），原 assignment 标记 transferred，更新 ticket.assignee_id
- **Delegate（委派）**: 创建新 assignment（status=pending, delegated_from=原 assignee），原 assignment 标记 delegated。委派人完成后自动回到原 assignee
- **Claim（抢单）**: Activity 的 assignment 中，第一个调用 claim 的用户获得排他处理权，其他 assignment 标记 claimed_by_other

Assignment 新增字段: `delegated_from *uint`, `transfer_from *uint`
Assignment 新增状态: `transferred`, `delegated`, `claimed`, `claimed_by_other`

### D7: 条件表达式扩展 — 扩展操作符 + 复合条件

**选择**:
- `evaluateCondition()` 新增操作符: `in`, `not_in`, `is_empty`, `is_not_empty`, `between`, `matches`
- `GatewayCondition` 新增 `Logic` 字段（"and"/"or"），支持 `Conditions []GatewayCondition` 子条件
- 当 `Logic` 非空时递归求值子条件；为空时按原逻辑走单条件

## Risks / Trade-offs

| 风险 | 影响 | 缓解 |
|------|------|------|
| FOR UPDATE 在 SQLite 上降级为表锁 | 高并发写入时性能下降 | SQLite 本身是单写模型，ITSM 操作频率低，实际无影响；PostgreSQL 下是真正行级锁 |
| 会签审批新增 counting 逻辑增加 Progress 复杂度 | 审批路径 bug 风险 | BDD 测试覆盖 single/parallel/sequential 三种模式 |
| ErrNotReady 需要 scheduler 配合识别 | 若 scheduler 不识别则退化为重试 | scheduler 改动很小（一个 errors.Is 判断），风险可控 |
| SLA check 每分钟全表扫描 | 工单量大时性能 | 加索引 (status, sla_status, response_deadline)；万级工单量下无问题 |
| 通知发送失败不应阻塞流程 | 通知丢失 | Send 错误只记 timeline warning，不 block processNode |

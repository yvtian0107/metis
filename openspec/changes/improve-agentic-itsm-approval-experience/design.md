## Context

当前 ITSM 工单使用 `Ticket.status` 表达生命周期，使用 `TicketActivity.status=completed` 搭配 `transition_outcome=approved/rejected` 表达人工结果，前端再把 `completed` 翻译为“已完成”。这个模型对系统内部够用，但对审批用户不够真实：同意、驳回、撤回、取消和智能体决策中都被压扁成少数技术状态。

SmartEngine 的推进链路目前通过 `ensureContinuation()` 提交 `itsm-smart-progress` 异步任务，再由 scheduler poller/worker 执行。这个机制适合恢复和定时任务，不适合作为审批提交后的主路径。审批提交成功后，产品上应立即承认人工动作并进入决策中；技术上应在事务提交后直接启动决策 goroutine，前端刷新只用于观察，不用于驱动流程。

约束：
- 不做向后兼容别名，不保留旧状态展示语义。
- 保持 Agentic 架构，决策智能体继续使用 Tools、`workflow_json`、`workflow_context`、prompt 和 validation。
- SQLite 本地模式下避免事务内启动长耗时 AI 或再次占用 root DB 导致单连接自锁。

## Goals / Non-Goals

**Goals:**
- 建立产品语义优先的工单状态模型，让用户能直接看出“待我审批 / 已同意决策中 / 已驳回决策中 / 自动执行中 / 已通过 / 已驳回 / 已撤回”。
- 让人工活动状态直接表达人工结果：`approved` 或 `rejected`，不再把所有人工完成都叫 `completed`。
- 审批/驳回事务提交成功后立即启动 SmartEngine 决策 goroutine，消除对 scheduler worker 轮询的主路径依赖。
- 让列表、详情、历史工单和时间线使用同一套业务状态和终态结果。
- 保留 60 秒前端自动刷新和 smart recovery 作为兜底，而不是流程推进机制。

**Non-Goals:**
- 不把 SmartEngine 改成手写确定性执行器。
- 不保留旧 `completed`/`cancelled` 用户展示兼容层。
- 不引入 WebSocket/SSE 实时推送；本次用主动刷新和 60 秒兜底。
- 不重做 AI Agent Tools 本身的业务能力，只验证直接调度路径下 Tools 可用。

## Decisions

### 1. Ticket.status 采用产品状态，Ticket.outcome 表达终态结果

`Ticket.status` 直接服务用户理解和列表筛选，包含：
- `submitted`
- `waiting_human`
- `approved_decisioning`
- `rejected_decisioning`
- `decisioning`
- `executing_action`
- `completed`
- `rejected`
- `withdrawn`
- `cancelled`
- `failed`

新增 `Ticket.outcome` 用于终态结果，值包含 `approved`、`rejected`、`fulfilled`、`withdrawn`、`cancelled`、`failed`。非终态为空。

替代方案：保留旧 `status`，新增 `displayStatus`。拒绝该方案，因为它会继续让后端真实状态和用户感知状态分裂，留下技术债。

### 2. Human Activity.status 直接落 `approved` / `rejected`

人工节点（approve/form/process）的结果状态直接使用 `approved` 或 `rejected`；非人工动作节点仍可使用 `completed` 表示动作执行完成。`transition_outcome` 保留为流程出边和决策上下文的 outcome 字段，但不再作为用户理解人工结果的唯一来源。

替代方案：继续 `status=completed` + `transition_outcome=approved/rejected`。拒绝该方案，因为历史列表和 Badge 必须继续到处推导，问题源头没有消失。

### 3. 审批后直接 goroutine 调度 SmartEngine

审批/驳回 API 在 DB 事务内只做短事务写入：
- 锁定工单和活动
- 写 activity status
- 写 assignment status
- 写 ticket status 为 `approved_decisioning` 或 `rejected_decisioning`
- 写 timeline

事务提交成功后调用调度器接口启动 goroutine。goroutine 使用 fresh DB session 和独立 context timeout 调用 `RunDecisionCycleForTicket()`。goroutine 必须 recover panic，并把错误写入 timeline / `ai_failure_count` / 结构化日志。

替代方案：继续提交 `itsm-smart-progress` task 等 scheduler poller。拒绝作为主路径，因为它把用户动作后的推进延迟交给后台轮询，产品体感和因果链都不对。

### 4. Scheduler 保留为恢复和延迟任务基础设施

`itsm-smart-recovery` 继续周期扫描孤儿智能工单，但恢复时调用新的 direct decision dispatcher。`itsm-action-execute`、wait timer、boundary timer 这类天然异步/延迟任务可以继续使用 scheduler；如果 action 完成后需要继续决策，也通过 direct dispatcher 进入 SmartEngine。

### 5. 前端状态视图从 Ticket.status / outcome 生成

前端不再靠 `smartState=ai_reasoning` 覆盖旧状态，而是直接展示后端返回的产品状态和终态结果。详情页和列表页都提供刷新按钮；自动刷新间隔统一为 60 秒，只用于兜底观察。

## Risks / Trade-offs

- 状态枚举破坏性变更 → 通过一次性迁移、集中常量和测试覆盖解决；不保留旧状态别名。
- goroutine 中 AI 调用失败不可返回给原 HTTP 请求 → 通过 timeline、`ai_failure_count`、ticket status 和日志呈现失败，并由 recovery 扫描兜底。
- 事务后 goroutine 可能在进程退出时丢失 → recovery 每 10 分钟扫描 `decisioning` 且无活动的孤儿工单并重新调度。
- SQLite 单连接下 goroutine 与请求事务竞争 → goroutine 必须在 commit 后启动，且使用新的 DB session；测试必须覆盖单连接模式。
- 旧数据状态迁移不完整会污染列表筛选 → 迁移脚本必须覆盖 ticket、activity、assignment 和 timeline 派生 outcome 的常见历史数据。

## Migration Plan

1. 引入新状态常量和 `outcome` 字段，更新模型迁移。
2. 编写一次性迁移：旧 `pending/in_progress/waiting_action/completed/failed/cancelled` 映射到新状态；结合 timeline `withdrawn` 和最后人工活动 outcome 派生 `outcome`。
3. 更新后端所有终态判断、列表查询、历史查询和 BuildResponse。
4. 引入 direct decision dispatcher，并把 SmartEngine continuation 主路径切过去。
5. 更新前端状态 Badge、筛选项、审批列表、详情页轮询。
6. 跑后端单元测试、SQLite 单连接回归、前端 lint/build 和 seed-dev。

## Open Questions

- `form` / `process` 人工节点是否统一允许 `rejected`，还是仅 approve/process 支持驳回；实现前需按现有 UI 操作面确认。
- 自动 action 执行中是否统一展示 `executing_action`，还是在多个并行动作时增加更具体的计数摘要；本次先以状态和 nextStepSummary 表达。

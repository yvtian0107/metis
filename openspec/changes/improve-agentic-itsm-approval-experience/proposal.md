## Why

Agentic ITSM 的审批体验现在把生命周期、人工决策结果和智能体运行阶段混在同一个 `completed/cancelled` 展示里，用户无法分辨已同意、已驳回、已撤回还是后台仍在决策。审批/驳回后也不应靠列表刷新或 scheduler 轮询来推动流程，系统必须在业务提交成功后立即承认人工结果并主动启动决策智能体。

## What Changes

- **BREAKING**: 重定义 ITSM 工单与活动状态语义，移除用户可见的泛化 `completed` 表达，人工活动使用 `approved` / `rejected` 作为真实业务状态。
- **BREAKING**: 工单主状态改为用户可理解的产品状态，包含 `decisioning`、`approved_decisioning`、`rejected_decisioning`、`executing_action`、`withdrawn` 等状态，不再把“决策中”藏在 smartState 辅助字段里。
- 审批/驳回提交成功后，当前人工活动立即落为已同意或已驳回，工单立即进入对应决策中状态，并记录清晰时间线。
- 智能决策推进从 scheduler worker 轮询改为事务提交后的 goroutine 主动执行；scheduler 仅保留恢复、定时器、非即时兜底类任务。
- 决策智能体继续使用现有 Tools、workflow_json、workflow_context 和 validation 机制，不回退为手写确定性执行器。
- 列表、详情、历史工单、我的待办统一展示新的业务状态，并提供手动刷新；自动刷新仅作为 60 秒兜底观察，不负责推动流程。
- 历史列表和详情能清楚区分“已通过 / 已驳回 / 已撤回 / 已取消 / 失败”等结果。

## Capabilities

### New Capabilities

- `itsm-ticket-status-model`: 定义 Agentic ITSM 面向用户的工单状态、活动状态和终态结果模型。
- `itsm-smart-direct-decision-dispatch`: 定义审批/动作完成后事务提交成功即由 goroutine 直接触发 SmartEngine 决策循环的运行语义。

### Modified Capabilities

- `itsm-ticket-lifecycle`: 更新 Ticket、TicketActivity、TicketAssignment 状态枚举和终态判断。
- `itsm-smart-continuation`: 将 smart continuation 从提交 `itsm-smart-progress` scheduler 任务调整为直接调度决策 goroutine，并保留恢复兜底。
- `itsm-approval-ui`: 审批提交后立即展示已同意/已驳回并进入决策中，刷新降为 60 秒兜底。
- `itsm-ticket-list-views`: 列表和历史视图统一展示业务状态，增加刷新入口和清晰终态结果。
- `itsm-smart-recovery`: 恢复任务继续扫描孤儿智能工单，但恢复后进入新的直接决策调度入口。
- `itsm-decision-tools`: 保持决策智能体 Tools 契约，确保直接调度路径下 Tools 仍按原有能力运行。

## Impact

- 后端：`internal/app/itsm/domain/model_ticket.go`、`internal/app/itsm/ticket/*`、`internal/app/itsm/engine/smart.go`、`internal/app/itsm/engine/tasks.go`、`internal/app/itsm/app.go`。
- 前端：`web/src/apps/itsm/api.ts`、`components/ticket-status*`、`pages/tickets/*`、`pages/tickets/approvals/*`、`locales/*`。
- 数据库：状态枚举语义发生破坏性变更；需要一次性迁移旧状态，不保留兼容别名。
- 运行时：即时 SmartEngine 决策改由 goroutine 驱动，必须有 panic recover、timeout、fresh DB session 和结构化 timeline/log。
- 测试：需要覆盖 SQLite 单连接事务后调度、审批/驳回状态展示、直接决策触发、Tools 可用性、列表刷新和历史结果区分。

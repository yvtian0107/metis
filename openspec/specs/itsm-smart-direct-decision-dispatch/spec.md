# itsm-smart-direct-decision-dispatch Specification

## Purpose
TBD - created by archiving change improve-agentic-itsm-approval-experience. Update Purpose after archive.
## Requirements
### Requirement: 事务后直接调度智能决策
系统 SHALL 在审批、驳回或智能动作完成的数据库事务提交成功后，直接启动 goroutine 执行 SmartEngine 决策循环。该 goroutine SHALL 使用独立 context timeout、fresh DB session、panic recover 和结构化日志。主路径 SHALL NOT 依赖 `itsm-smart-progress` scheduler worker 轮询。

#### Scenario: 审批事务提交后立即启动决策
- **WHEN** 用户同意智能工单的人工活动且事务提交成功
- **THEN** 系统 SHALL 在事务外启动 goroutine
- **AND** goroutine SHALL 调用 SmartEngine 决策循环
- **AND** 决策循环 SHALL 接收 completedActivityID 和 triggerReason=`activity_approved`

#### Scenario: 驳回事务提交后立即启动决策
- **WHEN** 用户驳回智能工单的人工活动且事务提交成功
- **THEN** 系统 SHALL 在事务外启动 goroutine
- **AND** goroutine SHALL 调用 SmartEngine 决策循环
- **AND** 决策循环 SHALL 接收 completedActivityID 和 triggerReason=`activity_rejected`

#### Scenario: 事务失败不启动决策
- **WHEN** 审批或驳回事务回滚
- **THEN** 系统 SHALL NOT 启动决策 goroutine
- **AND** 工单状态和活动状态 SHALL 保持回滚后的数据库真实状态

### Requirement: 决策 goroutine 错误可观测
直接决策 goroutine SHALL 捕获 panic 和 error，并将结果写入 timeline、AI failure count 和结构化日志。若 AI 决策失败，系统 SHALL 按现有 SmartEngine 失败策略进入可恢复状态。

#### Scenario: 决策 goroutine panic
- **WHEN** 决策 goroutine 执行时发生 panic
- **THEN** 系统 SHALL recover panic
- **AND** 写入 timeline 说明智能决策异常
- **AND** 记录结构化 error log

#### Scenario: 决策超时
- **WHEN** 决策 goroutine 超过服务配置的决策超时时间
- **THEN** 系统 SHALL 取消本轮 context
- **AND** SmartEngine SHALL 记录 AI 决策失败
- **AND** 工单 SHALL 可被 smart recovery 后续扫描恢复

### Requirement: Tools 沿用 Agentic 决策链
直接调度路径 SHALL 继续调用决策智能体，并允许智能体使用现有 decision tools、workflow_json、workflow_context 和 validation。系统 MUST NOT 将智能决策替换为手写固定流程执行器。

#### Scenario: 直接调度路径下 Tools 可用
- **WHEN** 直接决策 goroutine 启动一轮 SmartEngine 决策
- **THEN** 决策智能体 SHALL 能调用 `decision.ticket_context`、`decision.resolve_participant`、`decision.sla_status`、`decision.list_actions`、`decision.execute_action`
- **AND** 决策计划 SHALL 继续经过现有 validation


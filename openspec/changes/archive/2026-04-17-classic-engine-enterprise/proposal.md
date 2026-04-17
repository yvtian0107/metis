## Why

ITSM 经典引擎（ClassicEngine）的核心执行模型（token 树 + 递归推进）架构正确，但深度审查发现多个阻塞企业级上线的缺陷：并发竞态（无行级锁）、多人审批形同虚设、通知节点空实现、定时器可能丢失、SLA 无主动监控、无任务转办/委派/抢单、条件表达式能力不足。这些问题在多用户高并发的真实 ITSM 运营中会导致流程错乱和 SLA 失控。

## What Changes

### 并发安全（P0）
- Activity/Token 查询增加 `FOR UPDATE` 行级锁，消除 Progress() 竞态
- Parallel join counting 加锁，防止双重激活 parent token

### 定时器可靠性（P0）
- 修复 wait-timer / boundary-timer 返回语义，确保未到期任务不被 scheduler 标记 completed
- 引入显式 "not ready" 错误码让 scheduler 保留任务

### 多人审批（P0）
- 实现 parallel（会签）模式：所有参与人审批完成后才推进
- 实现 sequential（依次）模式：按 sequence 顺序逐个推进 is_current
- single 模式行为不变（第一人操作即推进）

### 通知集成（P0）
- Notify 节点对接 Kernel MessageChannel，支持邮件和站内信
- 支持通知模板变量替换（工单字段 + 流程变量）

### SLA 主动监控（P1）
- 新增 `itsm-sla-check` 周期任务，扫描即将到期和已超期工单
- 接入 EscalationRule 模型，超期自动触发升级动作
- 支持 SLA 暂停/恢复（工单挂起时暂停 SLA 时钟）

### 任务委派/转办/抢单（P1）
- Activity 层增加 transfer（转办）、delegate（委派）、claim（抢单）API
- Assignment 状态机扩展，支持转办历史追溯

### 条件表达式扩展（P1）
- 新增操作符：`in`、`not_in`、`is_empty`、`is_not_empty`、`between`、`matches`
- 支持 AND/OR 复合条件组（ConditionGroup）

## Capabilities

### New Capabilities
- `itsm-concurrency-safety`: 经典引擎并发锁机制，覆盖 Progress/Join/Timer 的行级锁
- `itsm-multi-approval`: 多人审批模式（会签 parallel、依次 sequential、单人 single）
- `itsm-notification`: 通知节点对接 Kernel MessageChannel 发送实际通知
- `itsm-sla-monitor`: SLA 主动监控、升级规则执行、SLA 暂停/恢复
- `itsm-task-dispatch`: 任务转办、委派、抢单能力
- `itsm-condition-expression`: 扩展条件操作符和复合条件组

### Modified Capabilities
- `itsm-classic-engine`: Progress/Cancel 方法内部加锁；processNode 审批分支逻辑变更
- `itsm-execution-token`: Token 查询加锁语义变更；join 逻辑加锁
- `itsm-bpmn-nodes`: approve 节点执行逻辑变更（多人模式）；notify 节点从空实现变为实际发送
- `itsm-sla`: 从静态计算变为主动监控 + 暂停/恢复
- `itsm-approval-api`: 审批 API 适配多人模式（会签需全员完成、依次需按序推进）

## Impact

- **核心引擎**: `internal/app/itsm/engine/classic.go` — Progress/handleApprove/tryCompleteJoin 加锁 + 审批逻辑重构
- **任务调度**: `internal/app/itsm/engine/tasks.go` — timer handler 返回值语义修复；新增 sla-check 任务
- **条件求值**: `internal/app/itsm/engine/condition.go` — 新增操作符 + 复合条件
- **参与人解析**: `internal/app/itsm/engine/resolver.go` — 无变更
- **Service 层**: `internal/app/itsm/ticket_service.go` — 新增 Transfer/Delegate/Claim 方法；审批逻辑适配
- **Handler 层**: `internal/app/itsm/ticket_handler.go` — 新增转办/委派/抢单 API 端点
- **App 注册**: `internal/app/itsm/app.go` — 注册新路由 + 新 scheduler 任务
- **通知渠道**: `internal/channel/` — 被 notify 节点调用（只读依赖）
- **数据库**: itsm_ticket_assignments 表可能需新增字段（delegate_from、transfer_from）
- **API**: 新增 6 个端点（transfer/delegate/claim/approve-counted/sla-pause/sla-resume）

## 1. 状态模型与数据迁移

- [x] 1.1 重构 Ticket/TicketActivity/TicketAssignment 状态常量与终态判断，移除旧兼容语义
- [x] 1.2 新增并接入 Ticket.outcome 字段，统一终态结果写入规则
- [x] 1.3 编写一次性迁移脚本，将旧状态映射到新状态与 outcome
- [x] 1.4 为迁移脚本补充校验脚本与回归样本（ticket/activity/assignment/timeline）

## 2. 事件驱动主链路

- [x] 2.1 定义工单决策相关领域事件与 payload 契约（activity.decided/action.finished/decision.failed）
- [x] 2.2 落地提交后事件分发器（goroutine + timeout + panic recover + fresh DB session）
- [x] 2.3 将审批/驳回/动作完成路径统一改接事件驱动 direct dispatch
- [x] 2.4 保留 scheduler 任务但收敛为恢复兜底职责，移除其主流程推进角色

## 3. 决策说明卡能力

- [x] 3.1 设计并实现决策说明卡数据结构（basis/trigger/decision/next_step/human_override）
- [x] 3.2 在 SmartEngine 决策完成与降级路径写入 explanation snapshot
- [x] 3.3 在 ticket detail API/read model 返回说明卡并支持按活动追溯
- [x] 3.4 补充解释字段完整性校验与异常回退策略

## 4. 失败恢复编排

- [x] 4.1 定义恢复动作模型（retry/handoff_human/withdraw）与可执行状态矩阵
- [x] 4.2 实现恢复动作权限校验、幂等键、防重窗口与执行器
- [x] 4.3 打通恢复动作时间线与审计日志记录
- [x] 4.4 在 ticket API 返回 recoveryActions 合同并覆盖测试

## 5. 列表与审批体验统一

- [x] 5.1 统一列表/详情/历史/审批视图的状态展示合同（status/statusLabel/statusTone/outcome）
- [x] 5.2 将审批提交交互统一为即时反馈模型并接入状态纠正
- [x] 5.3 提供统一手动刷新入口，保持筛选/分页/关键词上下文不丢失
- [x] 5.4 将自动刷新降级为纯观察兜底，确认不触发流程推进

## 6. 对话式提单升级

- [x] 6.1 扩展 ServiceDeskState，加入 missing_fields/asked_fields/min_decision_ready
- [x] 6.2 实现缺失字段检测与增量追问策略（最小可决策集）
- [x] 6.3 将 draft_prepare/draft_confirm/ticket_create 接入新状态机前置校验
- [x] 6.4 补充“已确认字段不重复追问”的会话一致性测试

## 7. 决策质量观测

- [x] 7.1 定义核心指标口径与计算窗口（approval/rejection/retry/latency/recovery_success）
- [x] 7.2 实现按服务与部门维度的指标聚合查询
- [x] 7.3 提供运营看板 API 与基础前端展示
- [x] 7.4 建立指标版本治理规则并补充口径一致性测试

## 8. 验证与发布

- [x] 8.1 运行后端回归：`go test ./internal/app/itsm/...` 与关键迁移测试
- [x] 8.2 运行编译与启动验证：`go build -tags dev ./cmd/server`、`go run -tags dev ./cmd/server seed-dev`
- [x] 8.3 运行前端验证：`cd web && bun run lint && bun run build`
- [x] 8.4 端到端走查：提单、审批、驳回、决策说明、失败恢复、列表一致性与观测指标

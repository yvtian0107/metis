## 1. 状态模型与迁移

- [x] 1.1 更新 `Ticket`、`TicketActivity`、`TicketAssignment` 状态常量，新增 `Ticket.outcome` 和状态展示 DTO 字段
- [x] 1.2 实现旧数据一次性迁移：从旧 ticket status、最后人工活动 outcome、timeline `withdrawn/ticket_cancelled/workflow_completed` 派生新 status/outcome
- [x] 1.3 更新终态判断、活跃状态判断、可撤回/可取消判断，移除旧 `completed/cancelled` 泛化语义
- [x] 1.4 更新人工活动完成逻辑：approved/rejected 直接写入 activity status、assignment status 和 transition_outcome
- [x] 1.5 为状态迁移和终态判断补充后端单元测试

## 2. 直接决策调度

- [x] 2.1 设计并实现 SmartEngine direct decision dispatcher，支持事务提交后 goroutine、timeout、panic recover、fresh DB session
- [x] 2.2 将审批/驳回 Progress 路径改为事务内写人工结果和决策中状态，事务提交后触发 direct dispatcher
- [x] 2.3 将 smart action 完成后的续推路径接入 direct dispatcher
- [x] 2.4 将 `ensureContinuation` 主路径从提交 `itsm-smart-progress` scheduler task 改为注册 direct dispatch
- [x] 2.5 保留并调整 smart recovery，使其扫描 decisioning 孤儿工单后调用 direct dispatcher
- [x] 2.6 为 SQLite 单连接模式补充回归测试，验证事务提交后调度不自锁、不等待 scheduler poller

## 3. 决策智能体与 Tools 验证

- [x] 3.1 验证 direct dispatch 路径下 `decision.ticket_context` 能读取新 status/outcome、人工结果和 workflow_context
- [x] 3.2 验证驳回后 rejected context 仍包含 satisfied=false、requires_recovery_decision=true 和 operator opinion
- [x] 3.3 验证 direct dispatch 路径下 `decision.resolve_participant`、`decision.sla_status`、`decision.list_actions`、`decision.execute_action` 可正常运行
- [x] 3.4 更新 SmartEngine continuation / rejected recovery / action execution 相关测试断言

## 4. 后端 API 与列表查询

- [x] 4.1 更新 `TicketResponse` / `TicketMonitorItem` / approval list 响应，返回状态展示合同字段
- [x] 4.2 更新审批待办、审批历史、我的工单、监控列表查询，使用新状态枚举和 outcome
- [x] 4.3 更新历史列表逻辑，确保已通过、已驳回、已撤回、已取消、失败可区分展示和筛选
- [x] 4.4 更新 Casbin/API 不变性检查，确认无新增接口被权限拦截
- [x] 4.5 补充 repository/service/handler 测试覆盖状态展示、筛选和历史结果区分

## 5. 前端体验

- [x] 5.1 更新 `web/src/apps/itsm/api.ts` 类型，接入 status/outcome/statusLabel/statusTone 等字段
- [x] 5.2 重写 ITSM 状态 Badge 和筛选项，移除用户可见的泛化“已完成”
- [x] 5.3 更新工单详情审批提交乐观态：同意显示“已同意，决策中”，驳回显示“已驳回，决策中”
- [x] 5.4 审批待办、审批历史、我的工单、监控列表增加刷新按钮，并保持当前筛选/分页/关键词
- [x] 5.5 将详情页和列表页自动刷新兜底统一调整为 60 秒，确保刷新不触发流程推进
- [x] 5.6 更新 zh-CN/en 文案，覆盖待审批、已同意决策中、已驳回决策中、自动执行中、已通过、已驳回、已撤回、已取消、失败

## 6. 验证

- [x] 6.1 运行 `go test ./internal/app/itsm/...`
- [x] 6.2 运行 `go build -tags dev ./cmd/server`
- [x] 6.3 运行 `go run -tags dev ./cmd/server seed-dev`
- [ ] 6.4 运行 `cd web && bun run lint && bun run build`
- [ ] 6.5 用 `make dev` 验证 Smart 工单：提交、同意、驳回、撤回、动作执行和历史列表展示
- [ ] 6.6 验证审批/驳回后无需整页刷新即可看到“已同意/已驳回，决策中”，且决策智能体立即开始调用 Tools

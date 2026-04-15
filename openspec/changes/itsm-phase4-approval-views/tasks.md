## 1. 后端：审批列表查询

- [x] 1.1 `ticket_repository.go` 新增 `ListApprovals(userID uint, positionIDs []uint, deptIDs []uint, page, pageSize int)` 方法：JOIN TicketAssignment + TicketActivity + Ticket，筛选 `activity_type="approve"` 且 `status IN ("pending","in_progress")`，按 Priority 排序
- [x] 1.2 `ticket_repository.go` 新增 `CountApprovals(userID uint, positionIDs []uint, deptIDs []uint)` 方法：返回待审批数量
- [x] 1.3 `ticket_service.go` 新增 `Approvals(userID, page, pageSize)` 和 `ApprovalCount(userID)` 方法：通过 IOC 可选获取 Org App 解析用户的 positionIDs/deptIDs，调用 repository
- [x] 1.4 `ticket_handler.go` 新增 `Approvals` handler（`GET /itsm/tickets/approvals`）：返回审批列表（含 Ticket 摘要 + Activity 详情 + SLA 信息）
- [x] 1.5 `ticket_handler.go` 新增 `ApprovalCount` handler（`GET /itsm/tickets/approvals/count`）：返回 `{"count": N}`

## 2. 后端：审批动作

- [x] 2.1 `ticket_handler.go` 新增 `ApproveActivity` handler（`POST /itsm/tickets/:id/activities/:aid/approve`）：验证当前用户是分配的审批人，设置 TransitionOutcome="approve"，调用 WorkflowEngine.Progress()，记录 Timeline
- [x] 2.2 `ticket_handler.go` 新增 `DenyActivity` handler（`POST /itsm/tickets/:id/activities/:aid/deny`）：同上但 TransitionOutcome="reject"，接受可选 reason 参数，记录 Timeline
- [x] 2.3 `ticket_service.go` 新增 `ApproveActivity(ticketID, activityID, userID)` 和 `DenyActivity(ticketID, activityID, userID, reason)` 方法：包含 assignment 验证 + engine progress 调用 + timeline 写入

## 3. 路由与权限

- [x] 3.1 `app.go` Routes() 注册新路由：`GET /tickets/approvals`、`GET /tickets/approvals/count`（放在 `:id` 路由前）、`POST /tickets/:id/activities/:aid/approve`、`POST /tickets/:id/activities/:aid/deny`
- [x] 3.2 `seed.go` 新增 Casbin 策略：为标准角色（itsm_user, itsm_agent, itsm_admin）添加 approvals/count GET 和 approve/deny POST 策略
- [x] 3.3 `seed.go` 新增"我的审批"菜单项：在工单中心菜单组下添加，permission 标识为 `itsm:tickets:approvals`

## 4. 前端：API 层

- [x] 4.1 `api.ts` 新增 `ApprovalItem` 类型（ticket 摘要 + activity 详情 + assignment 信息 + SLA 字段）
- [x] 4.2 `api.ts` 新增 `fetchApprovals(page, pageSize)`、`fetchApprovalCount()`、`approveActivity(ticketId, activityId)`、`denyActivity(ticketId, activityId, reason?)` 函数

## 5. 前端：我的审批页面

- [x] 5.1 新建 `pages/tickets/approvals/index.tsx`：审批列表页，Table 展示（工单编号可点击跳转、标题、服务、优先级 badge、SLA 状态 badge、SLA 剩余时间、活动名称、创建时间）+ 分页
- [x] 5.2 审批列表行操作：每行"通过"按钮 + "驳回"按钮（驳回弹出 Popover 输入原因），操作后刷新列表 + toast 提示
- [x] 5.3 `module.ts` 注册 `/itsm/tickets/approvals` 路由

## 6. 前端：SLA 展示增强

- [x] 6.1 新建 `components/sla-badge.tsx`：SLA 状态 badge 组件（on_track=绿色, breached_response=橙色, breached_resolution=红色）+ 剩余时间计算（距 deadline 的相对时间）
- [x] 6.2 修改 `pages/tickets/mine/index.tsx`：表格新增 SLA 列，使用 SLA badge 组件
- [x] 6.3 修改 `pages/tickets/todo/index.tsx`：表格新增 SLA 列，使用 SLA badge 组件
- [x] 6.4 修改 `pages/tickets/history/index.tsx`：表格新增 SLA 列（展示最终状态，非剩余时间）

## 7. i18n 与验证

- [x] 7.1 更新 `locales/zh-CN.json` 和 `locales/en.json`：新增审批相关翻译 key（approval.title, approval.empty, approval.approve, approval.deny, approval.reason, sla.onTrack, sla.breached, sla.remaining 等）
- [x] 7.2 Go build 验证 `go build -tags dev ./cmd/server/` + 前端 lint `cd web && bun run lint`

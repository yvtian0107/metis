## Tasks

### NodeData & 常量更新
- [ ] 1. NodeData 扩展：在 `workflow.go` 的 NodeData struct 新增 `AttachedTo string` 和 `Interrupting bool` 字段
- [ ] 2. 引擎常量更新：从 `engine.go` 的 UnimplementedNodeTypes 中移除 NodeBTimer 和 NodeBError

### 辅助函数：构建 boundaryMap
- [ ] 3. 在 `workflow.go` 中新增 `BuildBoundaryMap(def *WorkflowDef) map[string][]*WFNode` 函数：扫描所有 b_timer/b_error 节点，按 attached_to 分组返回 map[hostNodeID][]*WFNode

### attachBoundaryEvents 实现
- [ ] 4. 在 `classic.go` 中新增 `attachBoundaryEvents(tx, def, nodeMap, outEdges, token, node)` 方法：调用 BuildBoundaryMap 获取当前节点的 boundary 节点列表，为每个 b_timer 创建 suspended boundary token（parent_token_id=宿主 token, token_type=boundary），提交 itsm-boundary-timer 调度任务，记录 Timeline
- [ ] 5. handleForm 末尾调用 attachBoundaryEvents
- [ ] 6. handleApprove 末尾调用 attachBoundaryEvents
- [ ] 7. handleProcess 末尾调用 attachBoundaryEvents

### cancelBoundaryTokens 实现
- [ ] 8. 在 `classic.go` 中新增 `cancelBoundaryTokens(tx, token)` 辅助函数：将 parent_token_id=token.ID 且 token_type="boundary" 且 status="suspended" 的 token 全部标记为 cancelled
- [ ] 9. 在 `Progress()` 方法中，完成 activity 后、matchEdge 之前调用 cancelBoundaryTokens

### tryCompleteJoin 修改
- [ ] 10. 修改 tryCompleteJoin 中 remaining 计数查询，增加 `AND token_type != 'boundary'` 过滤条件
- [ ] 11. 修改 handleEnd 中 child token 的 remaining 计数查询，增加同样的 boundary 过滤条件

### HandleBoundaryTimer 调度任务
- [ ] 12. 在 `tasks.go` 新增 BoundaryTimerPayload struct（ticket_id, boundary_token_id, boundary_node_id, execute_after）
- [ ] 13. 在 `tasks.go` 新增 HandleBoundaryTimer 函数：解析 payload → 检查 execute_after → 加载 boundary token → 状态非 suspended 则 skip → 开始事务 → 取消宿主 activity → 取消宿主 token → 激活 boundary token → 加载 workflow → buildMaps → 从 b_timer 出边 processNode

### HandleActionExecute 的 b_error 拦截
- [ ] 14. 在 `classic.go` 新增 `triggerBoundaryError(ctx, tx, def, nodeMap, outEdges, ticketID, activityID, hostToken, bErrorNode)` 方法：取消宿主 activity/token，创建 active boundary token，从 b_error 出边 processNode
- [ ] 15. 修改 `tasks.go` 的 HandleActionExecute：当 outcome="failed" 时，加载 workflow JSON 和 nodeMap，查找 b_error boundary 节点，有则调用 triggerBoundaryError，无则走原有 Progress 路径

### Validator 增强
- [ ] 16. 在 `validator.go` 新增 boundary 节点校验规则：attached_to 必填且引用存在的节点、b_timer 目标为人工节点、b_error 目标为 action 节点、boundary 有且仅有一条出边、boundary 无入边、b_timer 必须有 duration

### ITSM App 注册
- [ ] 17. 在 ITSM App 的 Tasks() 中注册 `itsm-boundary-timer` 调度任务（async 类型），handler 为 HandleBoundaryTimer

### 编译验证
- [ ] 18. 运行 `go build -tags dev ./cmd/server/` 确认无编译错误

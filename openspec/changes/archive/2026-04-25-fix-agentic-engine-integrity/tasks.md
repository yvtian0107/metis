## 1. 后端表单验证激活（P0）

- [x] 1.1 在 `variable_writer.go` 的 `writeFormBindings()` 入口调用 `form.ValidateFormData(schema, data)`，验证失败时返回 error 且不写入任何流程变量
- [x] 1.2 在 `writeFormBindings()` 调用方（activity 完成路径）处理验证错误：记录 `form_validation_failed` timeline 事件，活动完成不阻塞但表单数据不传播
- [x] 1.3 为 `writeFormBindings` 新增 `currentNodeID` 参数，供权限校验使用（先传入，P1 任务中激活校验逻辑）
- [x] 1.4 编写单元测试：合法数据通过 → 变量写入；非法数据 → 原子拒绝 + error 返回；部分非法 → 全部拒绝

## 2. ensureContinuation 补齐（P0）

- [x] 2.1 在 `SmartEngine.Start()` 中，设置 `status=in_progress` 并记录 timeline 后，调用 `ensureContinuation(tx, ticket, 0)` 触发首次决策循环
- [x] 2.2 在 `SmartEngine.Cancel()` 中，设置 `status=cancelled` 并记录 timeline 后，调用 `ensureContinuation(tx, ticket, 0)` 触发清理
- [x] 2.3 编写测试：Start → 验证 itsm-smart-progress 任务被提交；Cancel → 验证 ensureContinuation 被调用；AI disabled 时 Start 不提交任务

## 3. 工作流拓扑验证（P0）

- [x] 3.1 `ValidationError` 结构新增 `Level` 字段（`"blocking"` | `"warning"`），现有结构/节点校验设为 blocking，formSchema 引用校验设为 warning
- [x] 3.2 实现 `detectCycles(nodes, edges)` — DFS 染色法环路检测，返回 blocking ValidationError 含环路路径描述
- [x] 3.3 实现 `detectDeadEnds(nodes, edges)` — 从 end 节点反向 BFS，未被访问的非 start 节点标记为死端，返回 blocking ValidationError
- [x] 3.4 实现 `validateParticipantTypes(nodes)` — 白名单校验（user, position, department, position_department, requester, requester_manager），返回 blocking ValidationError
- [x] 3.5 在 `ValidateWorkflow()` 中依次调用三个新检查函数，合并到返回的 errors 列表
- [x] 3.6 修改 `workflow_generate_service.go`：保存前检查 errors，任何 Level="blocking" → 不保存，返回 HTTP 400
- [x] 3.7 编写单元测试：无环通过、直接环检测、间接环检测、死端检测、参与者类型校验、blocking/warning 分级

## 4. 并行会签收敛超时（P1）

- [x] 4.1 `EngineConfigProvider` 接口新增 `ParallelConvergenceTimeout() time.Duration` 方法（默认 72h）
- [x] 4.2 在 `ensureContinuation` 的并行组检查中增加超时检测：读取 SLA deadline → config → 168h 兜底，比对组内最早活动创建时间
- [x] 4.3 实现超时处理：标记未完成兄弟活动为 `cancelled`（reason="convergence_timeout"），记录 `parallel_convergence_timeout` timeline 事件
- [x] 4.4 超时处理后调用 `ensureContinuation` 继续推进，seed 中包含超时信息
- [x] 4.5 编写测试：组内超时 → 取消 pending 活动 + 保留已完成结果 + 触发下一决策；未超时 → 正常等待

## 5. 表单字段权限后端校验（P1）

- [x] 5.1 在 `writeFormBindings()` 中，对每个字段检查 `field.Permissions[currentNodeID]`：readonly/hidden → 跳过写入 + warning log；无 permissions 或无 nodeID 条目 → 默认 editable
- [x] 5.2 编写测试：editable 正常写入；readonly 跳过 + log；hidden 跳过 + log；无 permissions 向后兼容

## 6. 恢复机制周期化（P1）

- [x] 6.1 将 `itsm-smart-recovery` 任务的 schedule 从 `@reboot` 改为 `@every 10m`
- [x] 6.2 在 `HandleSmartRecovery` 中增加内存 map `lastRecoverySubmissions`（ticketID→timestamp），10 分钟内已提交的 ticketID 跳过
- [x] 6.3 每次运行前清理 map 中超过 10 分钟的旧条目
- [x] 6.4 编写测试：首次运行提交恢复；10 分钟内重复运行跳过；10 分钟后重新提交

## 7. user_picker / dept_picker 组件（P1）

- [x] 7.1 实现 `UserPicker` 组件：Combobox + `GET /api/v1/users?keyword=xxx&limit=10` 搜索，300ms debounce，值存 ID，展示名称
- [x] 7.2 实现 `DeptPicker` 组件：TreeSelect + `GET /api/v1/org/departments/tree`，值存 ID，展示部门名称
- [x] 7.3 两个组件的 view/readonly 模式：通过 ID 反查名称展示
- [x] 7.4 两个组件的 fallback：API 不可用时降级为 text Input + warning tooltip
- [x] 7.5 在 `field-renderers.tsx` 中替换 user_picker 和 dept_picker 的纯文本 Input 为新组件

## 8. SmartEngine 次级修复（P2）

- [x] 8.1 `decision.sla_status` 工具：SLA 紧急度阈值从 `EngineConfigProvider` 读取（critical_threshold_seconds 默认 1800，warning_threshold_seconds 默认 3600）
- [x] 8.2 `decision.similar_history` 工具：limit 从 `EngineConfigProvider.SimilarHistoryLimit()` 读取（默认 5）
- [x] 8.3 `parseDecisionPlan()` 中校验 `execution_mode` 只接受 `""` / `"single"` / `"parallel"`，非法值 warn + 默认空
- [x] 8.4 `handleComplete()` 创建终态活动时写入 end 节点 NodeID（从 workflow_json 查找 activity_kind=end）
- [x] 8.5 `buildInitialSeed` 中 `approved_next_step.instruction` 增加 `"应遵循此路径继续推进"` 约束，与 rejected 路径对称

## 9. FormDesigner 权限编辑器（P2）

- [x] 9.1 `FormDesigner` 组件 props 新增可选 `workflowNodes` 数组
- [x] 9.2 在 field property editor 中增加 "节点权限" section，仅在 `workflowNodes` 存在时显示
- [x] 9.3 实现权限编辑 UI：每个节点一行 dropdown（editable/readonly/hidden），变更实时更新 field.permissions map
- [x] 9.4 在 workflow editor 的 form-binding-picker 中传入 workflowNodes 到 FormDesigner

## 10. 验证与集成

- [x] 10.1 后端完整构建通过：`go build -tags dev ./cmd/server`
- [x] 10.2 后端测试通过：`go test ./internal/app/itsm/...`
- [x] 10.3 前端 lint + 构建通过：`cd web && bun run lint && bun run build`
- [ ] 10.4 端到端验证：通过 `make dev` 启动，创建 Smart 工单，验证首次决策循环自动触发

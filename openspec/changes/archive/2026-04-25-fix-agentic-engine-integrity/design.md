## Context

Agentic ITSM 引擎核心链路已基本实现：8 个决策工具、DecisionExecutor 抽象、上下文对称性、NodeID 绑定、并行会签、恢复机制、服务台工具链、流程生成、表单引擎、SLA 监控。但深度审计发现 7 个致命或高危缺陷，均源于同一模式 — 把 Agentic 系统当传统 CRUD 实现，缺少"验证→兜底→自愈"闭环。

当前状态：
- `form.ValidateFormData()` 已实现但在工作流引擎中是死代码
- `ensureContinuation()` 在 Progress/Reject/Confirm/Action 路径调用，但 Start/Cancel 遗漏
- `ValidateWorkflow()` 返回 errors 但不阻止持久化
- 并行会签无收敛超时，`ensureContinuation` 无限等待
- 表单字段 permissions 前端执行、后端不校验
- `itsm-smart-recovery` 只 @reboot 运行一次
- user_picker/dept_picker 是纯文本 Input

## Goals / Non-Goals

**Goals:**

- 激活后端表单验证，确保 Agent 决策上下文中的表单数据合法
- 所有状态变更路径都驱动 `ensureContinuation`，无死角
- 工作流生成的拓扑缺陷被阻塞而非静默接受
- 并行会签有时间感知，超时可升级或取消
- 表单字段权限在后端强制执行
- 恢复机制持续运行，非一次性
- user_picker/dept_picker 是真正的选择器组件

**Non-Goals:**

- 不改变 DecisionExecutor 接口（已稳定）
- 不改变 8 个决策工具的工具名称和参数 schema（向后兼容）
- 不引入新的数据库表（只改行为）
- 不重构 ClassicEngine（本次只修 SmartEngine 链路）
- 不实现 rich_text markdown 渲染（独立 change）
- 不修复 Draft 提交竞态（需要 DB unique constraint，独立 change）
- 不做条件可见性循环依赖检测（独立 change）

## Decisions

### D1：后端表单验证的集成点

**选择**：在 `variable_writer.go` 的 `writeFormBindings()` 入口处调用 `form.ValidateFormData()`

**替代方案**：
- A) 在 Activity 完成的 Handler 层调用 — 太远离数据写入点，中间可能被绕过
- B) 在 Engine 的 Progress() 里调用 — 耦合了引擎和表单验证逻辑
- C) 在 variable_writer 里调用（选择此方案） — 最近数据写入点，无法绕过，且 writer 已有 schema 上下文

**理由**：variable_writer 是"最后一道门"，所有流程变量写入都经过这里。在这里拦截确保无论哪条路径触发写入，验证都会执行。验证失败时返回 error，调用方决定如何处理（记录 timeline + 拒绝写入）。

### D2：ensureContinuation 在 Start() 中的触发方式

**选择**：`Start()` 在设置状态为 in_progress 后，直接调用 `ensureContinuation(tx, ticket, 0)`（completedActivityID=0 表示初始决策）

**替代方案**：
- A) Start() 内部直接提交 itsm-smart-progress 任务 — 绕过 ensureContinuation 的门控逻辑（terminal check、circuit breaker）
- B) 通过 ensureContinuation 触发（选择此方案） — 复用所有现有门控，统一入口

**理由**：ensureContinuation 已经封装了 terminal state check、circuit breaker、countersign convergence 等所有门控。Start 应该走同一条路，而不是旁路。

### D3：工作流拓扑验证策略

**选择**：在 `ValidateWorkflow()` 中新增三个拓扑检查函数，返回 `blocking` 级别的 ValidationError

**实现方案**：
- `detectCycles(nodes, edges)` — DFS 染色法，O(V+E)
- `detectDeadEnds(nodes, edges)` — 从 end 节点反向 BFS，未被访问的非 start 节点是死端
- `validateParticipantTypes(nodes)` — 白名单校验：user, position, department, position_department, requester, requester_manager

**错误分级**：
- ValidationError 新增 `Level` 字段：`"blocking"` | `"warning"`
- 现有校验（结构/节点类型）全部为 blocking
- formSchema 引用校验保持 warning
- 新增拓扑校验全部为 blocking

**持久化阻塞逻辑**：
- `workflow_generate_service.go` 在保存前检查：如果 errors 中有任何 `Level=="blocking"`，不保存，返回 400
- 仅 warning 的场景正常保存

### D4：并行收敛超时机制

**选择**：在 `ensureContinuation` 的并行组检查中增加超时检测，触发时提交 `itsm-smart-timeout` 异步任务

**超时来源**（优先级递减）：
1. 工单的 SLA resolution_deadline（如果有 SLA）
2. `EngineConfigProvider.ParallelConvergenceTimeout()`（全局配置，默认 72h）
3. 兜底 168h（7天）

**超时后行为**：
- 标记超时的兄弟活动为 `cancelled`，reason = "convergence_timeout"
- 记录 timeline 事件
- 调用 ensureContinuation 继续推进（已完成的活动结果有效）

**理由**：超时不等于失败 — 已完成的审批结果应保留，只取消仍在等待的活动。这保持了并行审批的"尽力而为"语义。

### D5：权限后端校验架构

**选择**：`variable_writer.go` 在写入每个字段时检查 `field.Permissions[currentNodeID]`

**逻辑**：
- 如果 permissions map 不存在或无当前 nodeID 条目 → 默认 editable（向后兼容）
- 如果 permission == "readonly" 或 "hidden" → 跳过该字段写入，不报错（静默忽略）
- 需要 variable_writer 接收 currentNodeID 参数（新增）

**理由**：静默忽略而非报错，因为前端已经做了 UI 层面的限制。后端校验是安全兜底，不应影响正常用户体验。但应记录 warning log 以便审计。

### D6：恢复机制周期化

**选择**：`itsm-smart-recovery` 从 `@reboot` 改为 `@every 10m`

**替代方案**：
- A) 保持 @reboot 但增加独立的周期扫描任务 — 两个任务做类似事情，维护负担
- B) 直接改调度频率（选择此方案） — 简单，复用现有逻辑

**幂等保护**：现有逻辑已经跳过有活跃活动的工单，重复扫描是安全的。增加一个 `lastRecoveryAt` 内存标记避免 10 分钟内重复提交相同 ticketID。

### D7：user_picker / dept_picker 组件设计

**选择**：
- user_picker → Combobox + 防抖搜索 API `GET /api/v1/users?keyword=xxx&limit=10`
- dept_picker → TreeSelect + 组织树 API `GET /api/v1/org/departments/tree`

**共同约束**：
- 值存储为 ID（number），不存储名称
- 展示时通过 ID 反查名称（前端缓存）
- readonly/view 模式显示名称文本
- Org App 不可用时 fallback 为 Input + 手动输入 ID（带 warning 提示）

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| 后端表单验证可能拒绝历史上已存入的"脏数据" | 验证只在新写入时触发，不追溯历史数据 |
| ensureContinuation 在 Start() 中同步调用可能阻塞创建请求 | ensureContinuation 提交的是异步任务，不阻塞 |
| 拓扑验证的 DFS/BFS 对大图可能慢 | 实际工作流节点数 < 100，O(V+E) 可忽略 |
| 并行超时 72h 默认值可能不适合所有场景 | 通过 EngineConfigProvider 可配置 |
| 权限静默忽略可能让管理员误以为写入成功 | 记录 warning log + timeline 事件 |
| 10 分钟恢复间隔可能过频 | 幂等 + lastRecoveryAt 内存去重，实际开销极小 |
| user_picker 搜索 API 在大量用户时可能慢 | limit=10 + 防抖 300ms，只查活跃用户 |

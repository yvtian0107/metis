## Context

ITSM 经典引擎已支持 11 种节点类型的图遍历执行，其中人工节点（form/approve/process）创建 pending Activity 后停止，等待人工操作完成后 Progress 推进。当前缺少"附着"在人工节点上的超时/错误事件机制。

已有相关基础设施：
- `wait` 节点的 `HandleWaitTimer` 已实现调度器定时 + 惰性状态检查模式
- `HandleActionExecute` 处理 action 节点的异步 HTTP 调用和失败重试
- `executionTokenModel` 已预留 `TokenSuspended` 状态和 `TokenBoundary` 类型
- `NodeBTimer` / `NodeBError` 已注册为合法节点类型（当前在 UnimplementedNodeTypes 中）

## Goals / Non-Goals

**Goals:**
- 实现 Boundary Timer Event（b_timer）：附着在人工节点上的超时事件（interrupting 模式）
- 实现 Boundary Error Event（b_error）：附着在 action 节点上的错误捕获事件
- 支持多个 boundary event 附着同一宿主节点（多级超时升级）
- 在宿主正常完成时自动清理所有 boundary token

**Non-Goals:**
- Non-interrupting 模式（并行分支）— 后续优化
- 前端 UI（⑥ itsm-bpmn-designer 中实现）
- 并行分支中 boundary event 的 join 语义优化 — 初版文档标注限制

## Decisions

### D1: Boundary Event 数据模型 — 独立节点 + attachedTo
**选择**: b_timer / b_error 作为 workflow graph 中的独立节点，通过 `data.attached_to` 字段关联宿主节点

**理由**:
- 引擎已注册 `NodeBTimer` / `NodeBError` 为独立节点类型
- 独立节点拥有自己的出边，ReactFlow 可自然渲染边界事件和连线
- 与 BPMN 标准一致：boundary event 是附着在宿主上的独立元素
- 校验逻辑（出边、入边）天然兼容

**备选**: 嵌入宿主 NodeData 的 `boundaryEvents` 数组 — 简单但与已注册节点类型矛盾，targetNodeId 是隐式跳转无边

### D2: NodeData 新增字段
**选择**: 新增 `AttachedTo string`（宿主节点 ID）和 `Interrupting bool`（是否中断宿主）

**理由**: b_timer 需要 attached_to + interrupting + duration（已有字段）。b_error 只需 attached_to（error 始终 interrupting）。最小扩展。

### D3: Boundary Token 关联方式 — 复用 parent_token_id
**选择**: boundary token 的 `parent_token_id` 指向宿主 token，`token_type` 设为 "boundary"

**理由**:
- 无 schema 变更（不需新增列）
- tryCompleteJoin 已有 token_type 字段可过滤：`AND token_type != 'boundary'`
- 语义清晰：boundary token 的生命周期由宿主 token 决定

**备选**: 新增 `host_token_id` 列 — 语义更精确但增加 schema 复杂度，两种"父"关系并存易混淆

### D4: b_error 拦截点 — HandleActionExecute 中
**选择**: 在 `HandleActionExecute` 任务 handler 中，当 outcome="failed" 时先检查 b_error，有则调用 `triggerBoundaryError()`，无则走现有 Progress 路径

**理由**:
- 零侵入 Progress 核心路径
- b_error 仅 action 节点产生，在 action 专属 handler 中处理符合单一职责
- Progress 保持纯粹的"完成当前 activity + 沿出边前进"语义

**备选**: 在 Progress 内部拦截 — 集中处理但修改核心流转逻辑，增加所有流转的检查成本

### D5: Scheduler Task 取消方式 — 惰性检查
**选择**: boundary timer 到期后检查 boundary token 状态，已被 cancelled（宿主完成）则静默 skip

**理由**: 与现有 `HandleWaitTimer` 完全一致的模式。一次空轮询的代价远低于维护 task ID 关联和显式取消机制。

### D6: 多个 Boundary Event 附着同一节点 — 支持
**选择**: 一个宿主节点可附着多个 b_timer（如 24h 通知 + 48h 升级）

**理由**: ITSM 多级超时升级是刚需。实现代价为零（循环创建多个 boundary token + timer task）。

### D7: tryCompleteJoin 的 Boundary 排除
**选择**: 在 tryCompleteJoin 的 remaining 查询中增加 `AND token_type != 'boundary'`

**理由**: boundary token 被激活后状态变为 active，如果不排除会被错误计入 parallel join 的兄弟计数。显式排除最安全。

### D8: attachBoundaryEvents 调用方式 — 每个 handler 中调用
**选择**: handleForm / handleApprove / handleProcess 末尾各自调用 `attachBoundaryEvents()`

**理由**: 调用点明确，attachment 发生在 activity 和 assignment 都已创建之后。三处调用点是可接受的代价。

### D9: processNode 中 b_timer/b_error 处理 — 不处理
**选择**: b_timer / b_error 节点没有入边，不会被 processNode 正常路由到。boundary 触发时直接从 b_node 的出边查找 target，调用 processNode(target)。

**理由**: 无需在 switch 中添加 case。仅从 UnimplementedNodeTypes 移除即可。

### D10: 并行分支中 Boundary 行为 — 初版简单模式
**选择**: boundary token 的 parent 始终指向宿主 token。文档标注：并行分支内的 boundary event 可能导致 join 提前触发。

**理由**: 大多数 ITSM 场景中 boundary event 附着在主流程的人工节点上（非并行分支内），初始版本覆盖 90% 场景即可。

## Risks / Trade-offs

**[并行分支 + boundary]** → 如果 boundary 在并行分支中触发，宿主 token 被 cancelled 后 join 可能提前触发。Mitigation: 文档标注限制，后续版本实现 boundary token 替换宿主成为 fork 子 token 的语义。

**[多 boundary timer 竞争]** → 多个 b_timer 同时到期时，先触发的会 cancel 宿主，后触发的惰性检查发现已取消而 skip。这是正确行为（interrupting 语义：第一个赢）。

**[HandleActionExecute 加载 workflow JSON]** → b_error 检查需要在 task handler 中加载 workflow JSON 和 buildMaps，有一定开销。Mitigation: 仅在 outcome="failed" 时才加载，成功路径零开销。

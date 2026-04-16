## Why

ITSM 中几乎所有审批和处理任务都需要超时机制——"审批超时自动升级"、"处理超时通知主管"、"action 执行失败走错误处理分支"。当前引擎只有独立的 wait 节点（流程中的一个步骤），无法将超时/错误事件**附着**在其他节点上。

BPMN Boundary Event 解决这个问题：它不是流程中的独立步骤，而是附着在宿主节点上的事件监听器。当事件触发时，可以中断宿主（interrupting）或并行分支（non-interrupting）。

本 change 实现两种 Boundary Event：
- **Boundary Timer**（b_timer）：附着在 UserTask（form/approve/process）上的超时事件
- **Boundary Error**（b_error）：附着在 ServiceTask（action）上的错误捕获事件

## What Changes

### Boundary Timer Event
- **NodeData 扩展**：`boundaryEvents: [{id, type:"b_timer", interrupting: bool, duration, targetNodeId}]`
- **attachBoundaryEvents 辅助函数**：在 handleForm/handleApprove/handleProcess 创建 activity 后调用
- **执行逻辑**：
  - 创建 UserTask activity 时，为每个 b_timer 创建 boundary token（status=suspended）
  - 注册调度器定时任务 `itsm-boundary-timer`
  - 超时触发（interrupting=true）：取消宿主 activity + 宿主 token，激活 boundary token 走 targetNode
  - 宿主正常完成时：取消所有关联的 boundary token + 定时任务

### Boundary Error Event
- **action 失败路径增强**：action webhook 执行失败（超过重试次数）时，先检查宿主节点是否有 b_error boundary event
- **有 b_error**：激活 error boundary token，走错误处理分支（而非标记工单失败）
- **无 b_error**：保持现有行为（走 failed 出边或标记异常）

### Scope
- 仅实现 **interrupting** 模式。non-interrupting（并行分支）标记为后续优化
- 不含前端 UI（⑥ 中实现）

## Capabilities

### New Capabilities
- `itsm-boundary-timer`: 边界定时器事件（interrupting）
- `itsm-boundary-error`: 边界错误事件（ServiceTask 失败处理）

### Modified Capabilities
- `itsm-classic-engine`: handleForm/Approve/Process 集成 boundary 注册；processNode 中 b_timer/b_error 不再返回"未实现"
- `itsm-action-execute`: action 失败时检查并触发 boundary error

## Impact

- **后端**：`engine/classic.go` 新增 attachBoundaryEvents + boundary timer handler (~120 行)；`engine/tasks.go` 新增 itsm-boundary-timer 调度任务 (~50 行)；修改 HandleActionExecute 增加 b_error 检查 (~30 行)；`engine/validator.go` boundary 配置校验 (~40 行)
- **前端**：无改动
- **依赖**：③ itsm-execution-tokens（token suspended/boundary 状态）

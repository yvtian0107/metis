## ADDED Requirements

### Requirement: Boundary Timer 事件注册
当 processNode 到达人工节点（form/approve/process）时，系统 SHALL 扫描 workflow 中所有 `type="b_timer"` 且 `data.attached_to` 指向该节点 ID 的 boundary 节点。对每个匹配的 b_timer 节点，系统 SHALL 创建一个 boundary token（token_type="boundary", status="suspended", parent_token_id=宿主 token ID），并提交 `itsm-boundary-timer` 调度器异步任务（payload 含 ticket_id, boundary_token_id, boundary_node_id, execute_after）。

#### Scenario: 单个 b_timer 附着审批节点
- **WHEN** 流程到达 approve 节点，workflow 中有一个 b_timer 节点 attached_to 指向该 approve
- **THEN** 系统创建一个 boundary token（suspended），提交 itsm-boundary-timer 任务（execute_after = 当前时间 + b_timer 的 duration），记录 Timeline "边界定时器已设置: {duration}"

#### Scenario: 多个 b_timer 附着同一节点
- **WHEN** 流程到达 form 节点，workflow 中有两个 b_timer（24h 和 48h）attached_to 指向该 form
- **THEN** 系统创建两个独立的 boundary token，各自提交 itsm-boundary-timer 任务

#### Scenario: 无 b_timer 附着
- **WHEN** 流程到达 approve 节点，workflow 中无 b_timer attached_to 指向该节点
- **THEN** 系统不创建 boundary token，不提交定时任务（行为与变更前一致）

---

### Requirement: Boundary Timer 超时触发
当 `itsm-boundary-timer` 调度任务到期执行时，系统 SHALL 检查 boundary token 状态。若仍为 suspended（宿主尚未完成），系统 SHALL 执行 interrupting 逻辑：取消宿主 activity、取消宿主 token、激活 boundary token，然后从 b_timer 节点的出边继续流程推进。

#### Scenario: 超时触发（interrupting）
- **WHEN** itsm-boundary-timer 任务到期，boundary token 状态为 suspended
- **THEN** 系统将宿主 activity 标记为 cancelled，宿主 token 标记为 cancelled，boundary token 状态设为 active，从 b_timer 节点的出边找到目标节点，调用 processNode 继续流程，记录 Timeline "审批超时，流程已转向边界路径"

#### Scenario: 宿主已完成时 timer 到期
- **WHEN** itsm-boundary-timer 任务到期，但 boundary token 状态已为 cancelled（宿主正常完成时已清理）
- **THEN** 系统静默跳过，不执行任何操作

#### Scenario: timer 未到期
- **WHEN** itsm-boundary-timer 任务被轮询但当前时间 < execute_after
- **THEN** 系统跳过本次执行，等待下次轮询

---

### Requirement: 宿主完成时清理 Boundary Token
当人工节点的 Activity 正常完成（通过 Progress）时，系统 SHALL 将该 token 下所有 boundary 子 token（token_type="boundary", status="suspended"）标记为 cancelled。

#### Scenario: 审批完成后清理 boundary token
- **WHEN** 审批人完成审批，Progress 推进流程
- **THEN** 所有附着在该审批节点的 boundary token 状态设为 cancelled，关联的 itsm-boundary-timer 任务到期后惰性跳过

#### Scenario: 无 boundary token 时 Progress 行为不变
- **WHEN** 人工节点完成 Progress，该 token 下无 boundary 子 token
- **THEN** Progress 行为与变更前完全一致

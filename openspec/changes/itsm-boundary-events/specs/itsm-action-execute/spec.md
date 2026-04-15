## MODIFIED Requirements

### Requirement: Scheduler 异步任务注册
ITSM App SHALL 注册以下 Scheduler 异步任务：`itsm-action-execute`（执行动作节点的 HTTP 调用）、`itsm-wait-timer`（等待节点定时唤醒）和 `itsm-boundary-timer`（边界定时器超时触发）。三个任务均为 async 类型。

`itsm-action-execute` 在 action 失败时 SHALL 先检查宿主节点是否附着有 `b_error` boundary 节点。若有，SHALL 调用 `triggerBoundaryError()` 执行错误边界逻辑（取消宿主 activity/token、创建 boundary token、从 b_error 出边继续流程），而非调用 Progress(outcome="failed")。若无 b_error，保持现有行为不变。

#### Scenario: itsm-action-execute 任务执行
- **WHEN** Scheduler 轮询到 itsm-action-execute 任务
- **THEN** 任务执行器读取 payload（ticket_id, activity_id, action_id），发起 HTTP 请求，根据结果处理后续

#### Scenario: action 成功
- **WHEN** HTTP 请求返回成功
- **THEN** 调用 Progress(outcome="success")（行为不变）

#### Scenario: action 失败且有 b_error
- **WHEN** HTTP 请求重试耗尽失败，宿主 action 节点附着有 b_error 节点
- **THEN** 调用 triggerBoundaryError()，不调用 Progress(outcome="failed")

#### Scenario: action 失败且无 b_error
- **WHEN** HTTP 请求重试耗尽失败，宿主 action 节点无 b_error 附着
- **THEN** 调用 Progress(outcome="failed")（行为不变）

#### Scenario: itsm-wait-timer 任务执行
- **WHEN** Scheduler 轮询到 itsm-wait-timer 任务，且当前时间 >= payload.execute_after
- **THEN** 任务执行器调用 TicketService.Progress()，outcome="timeout"

#### Scenario: itsm-boundary-timer 任务执行
- **WHEN** Scheduler 轮询到 itsm-boundary-timer 任务，且当前时间 >= payload.execute_after
- **THEN** 任务执行器检查 boundary token 状态，若为 suspended 则执行 interrupting 逻辑，若非 suspended 则静默跳过

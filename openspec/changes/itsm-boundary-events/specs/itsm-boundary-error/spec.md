## ADDED Requirements

### Requirement: Boundary Error 事件触发
当 action 节点的 HTTP 调用重试耗尽最终失败时，HandleActionExecute SHALL 检查该 action 节点是否有 `type="b_error"` 且 `data.attached_to` 指向该节点的 boundary 节点。若有，系统 SHALL 执行 boundary error 逻辑而非调用 Progress(outcome="failed")。

#### Scenario: action 失败且有 b_error 附着
- **WHEN** action 节点 HTTP 调用重试耗尽失败，workflow 中存在 b_error 节点 attached_to 指向该 action
- **THEN** 系统取消宿主 activity（status=cancelled），取消宿主 token（status=cancelled），创建 boundary token（token_type="boundary", status="active"），从 b_error 节点的出边找到目标节点调用 processNode 继续流程，记录 Timeline "动作执行失败，已触发错误边界事件"

#### Scenario: action 失败且无 b_error 附着
- **WHEN** action 节点 HTTP 调用失败，workflow 中无 b_error 节点 attached_to 指向该 action
- **THEN** 系统调用 Progress(outcome="failed")（现有行为不变）

#### Scenario: action 成功
- **WHEN** action 节点 HTTP 调用成功
- **THEN** 系统调用 Progress(outcome="success")（行为不变，不检查 b_error）

---

### Requirement: Boundary Error Token 生命周期
b_error 的 boundary token 与 b_timer 不同：b_error 的 boundary token 不在宿主 activity 创建时预创建，而是在 action 实际失败时**按需创建**（status=active）。

#### Scenario: b_error token 按需创建
- **WHEN** action 失败触发 b_error
- **THEN** boundary token 在触发时创建（不是在 action activity 创建时），直接以 active 状态创建

#### Scenario: action 成功不创建 b_error token
- **WHEN** action 节点成功完成
- **THEN** 不创建任何 b_error boundary token

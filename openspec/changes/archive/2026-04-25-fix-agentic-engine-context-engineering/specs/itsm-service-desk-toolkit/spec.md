## MODIFIED Requirements

### Requirement: itsm.validate_participants 工具
系统 SHALL 提供参与人预检能力（通过 `ValidateParticipants` 方法），在提单前校验工作流中各人工节点的参与人是否可解析。该方法 SHALL 复用 `engine.ParticipantResolver` 而非独立解析 workflow_json。

#### Scenario: ValidateParticipants 复用 ParticipantResolver
- **WHEN** `ValidateParticipants` 被调用，workflow_json 中有一个 process 节点配置了 `position_department` 类型参与人
- **THEN** 方法 SHALL 通过 `ParticipantResolver.Resolve()` 验证该参与人是否可解析，而非自行查询 org 表

#### Scenario: requester 类型在提单前跳过
- **WHEN** `ValidateParticipants` 遍历到 requester 类型参与人
- **THEN** 方法 SHALL 跳过该参与人的验证（提单前不知道 requester 是谁）

#### Scenario: position_department 参与人无可用人员
- **WHEN** `ValidateParticipants` 验证一个 position_department 参与人，`ParticipantResolver.Resolve()` 返回空用户列表
- **THEN** 方法 SHALL 返回 `ParticipantValidation{OK: false, FailureReason: "岗位[xxx]+部门[xxx] 下无可用人员", NodeLabel: "节点名"}`

#### Scenario: user 类型参与人不存在
- **WHEN** `ValidateParticipants` 验证一个 user 类型参与人，`ParticipantResolver.Resolve()` 返回空列表或错误
- **THEN** 方法 SHALL 返回 `ParticipantValidation{OK: false, FailureReason: "指定用户不存在或已停用"}`

#### Scenario: form 节点参与人也被校验
- **WHEN** `ValidateParticipants` 遍历 workflow_json 中的节点
- **THEN** 方法 SHALL 同时校验 process 和 form 类型节点的参与人（当前仅校验 process）

#### Scenario: Org App 不可用时跳过岗位校验
- **WHEN** `ValidateParticipants` 执行时 `ParticipantResolver` 的 orgResolver 为 nil
- **THEN** position/position_department 类型参与人的校验 SHALL 被跳过，不视为错误

#### Scenario: 消除独立 workflow 解析逻辑
- **WHEN** `ValidateParticipants` 执行
- **THEN** 方法 SHALL NOT 包含独立的 workflow JSON 解析和参与人结构体定义（如 `workflowParticipant`），SHALL 复用 `engine.ParseWorkflowDef` 和 `engine.Participant` 类型

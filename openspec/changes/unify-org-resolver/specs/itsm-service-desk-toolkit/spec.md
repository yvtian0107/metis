## MODIFIED Requirements

### Requirement: itsm.validate_participants 工具
系统 SHALL 注册 `itsm.validate_participants` 工具，用于在创建工单前预检审批参与者是否可达。**参与者可达性检查 SHALL 通过 `app.OrgResolver` 接口完成，不得直接查询 org 领域的数据库表（departments、positions、user_positions）。** 当 OrgResolver 为 nil（Org App 未安装）时，岗位/部门类型的检查 SHALL 跳过并返回 ok=true。

**inputSchema**:
```json
{
  "type": "object",
  "properties": {
    "service_id": { "type": "integer", "description": "服务定义 ID" },
    "form_data": { "type": "object", "description": "表单数据（用于确定路由分支）" }
  },
  "required": ["service_id", "form_data"]
}
```

#### Scenario: 参与者可达
- **WHEN** Agent 调用 itsm.validate_participants，所有审批节点的参与者都能解析到有效用户
- **THEN** 系统 SHALL 返回 `{"ok": true}`

#### Scenario: 参与者不可达
- **WHEN** Agent 调用 itsm.validate_participants，某审批节点的岗位+部门无法解析到有效用户
- **THEN** 系统 SHALL 返回 `{"ok": false, "failure_reason": "岗位[网络管理员]+部门[IT] 下无可用人员", "node_label": "网络审批", "guidance": "请联系 IT 管理员补充人员配置后再提单"}`

#### Scenario: position 类型通过 OrgResolver 检查
- **WHEN** 工作流节点的 participantType 为 "position" 且 positionCode 为 "network_admin"
- **THEN** 系统 SHALL 调用 `OrgResolver.FindUsersByPositionCode("network_admin")`，若返回空列表则报告不可达

#### Scenario: position+department 类型通过 OrgResolver 检查
- **WHEN** 工作流节点的 participantType 为 "position" 且同时指定了 positionCode 和 departmentCode
- **THEN** 系统 SHALL 调用 `OrgResolver.FindUsersByPositionAndDepartment(posCode, deptCode)`，若返回空列表则报告不可达

#### Scenario: Org App 未安装时跳过组织检查
- **WHEN** OrgResolver 为 nil 且工作流节点包含 position 类型的参与者
- **THEN** 系统 SHALL 跳过该节点的检查（视为可达），返回 `{"ok": true}`

#### Scenario: user 类型直接查内核 users 表
- **WHEN** 工作流节点的 participantType 为 "user" 且指定了 userId
- **THEN** 系统 SHALL 直接查询 `users` 表检查用户是否存在且 `is_active=true`（此为内核表，非 org 领域）

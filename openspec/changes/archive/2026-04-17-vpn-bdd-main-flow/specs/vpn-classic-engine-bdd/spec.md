## ADDED Requirements

### Requirement: Classic engine routes network support to network_admin

经典引擎 SHALL 根据排他网关条件将 `request_kind=network_support` 的 VPN 申请路由到 `it/network_admin` 岗位审批。

#### Scenario: 网络支持类请求路由到网络管理员并审批完成
- **WHEN** 申请人提交 VPN 申请，访问原因为 "network_support"
- **THEN** 工单状态为 "in_progress"
- **THEN** 当前活动类型为 "approve"
- **THEN** 当前活动分配给 network-operator 所属的 it/network_admin
- **THEN** 当前活动未分配给 security-operator
- **WHEN** network-operator 认领并审批通过
- **THEN** 工单状态为 "completed"

### Requirement: Classic engine routes security compliance to security_admin

经典引擎 SHALL 根据排他网关条件将 `request_kind=external_collaboration` 的 VPN 申请路由到 `it/security_admin` 岗位审批。

#### Scenario: 安全合规类请求路由到安全管理员并审批完成
- **WHEN** 申请人提交 VPN 申请，访问原因为 "external_collaboration"
- **THEN** 工单状态为 "in_progress"
- **THEN** 当前活动类型为 "approve"
- **THEN** 当前活动分配给 security-operator 所属的 it/security_admin
- **THEN** 当前活动未分配给 network-operator
- **WHEN** security-operator 认领并审批通过
- **THEN** 工单状态为 "completed"

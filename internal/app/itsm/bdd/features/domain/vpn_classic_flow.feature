Feature: VPN 开通申请 — 经典引擎流程

  经典引擎根据排他网关条件将 VPN 申请路由到对应岗位处理。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 开通服务（经典引擎）

  Scenario: 网络支持类请求路由到网络管理员并处理完成
    When "vpn-requester" 提交 VPN 申请，访问原因为 "network_support"
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前活动分配给 "network-operator" 所属的 it/network_admin
    And 当前活动未分配给 "security-operator"
    When "network-operator" 认领并处理完成当前工单
    Then 工单状态为 "completed"

  Scenario: 安全合规类请求路由到安全管理员并处理完成
    When "vpn-requester" 提交 VPN 申请，访问原因为 "external_collaboration"
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前活动分配给 "security-operator" 所属的 it/security_admin
    And 当前活动未分配给 "network-operator"
    When "security-operator" 认领并处理完成当前工单
    Then 工单状态为 "completed"

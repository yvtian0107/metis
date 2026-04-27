Feature: VPN 开通申请 — 工单撤回

  申请人在工单尚未被认领前可以撤回申请。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 开通服务（经典引擎）

  Scenario: 无人认领时成功撤回
    When "vpn-requester" 提交 VPN 申请，访问原因为 "network_support"
    And "vpn-requester" 撤回工单，原因为 "不需要了"
    Then 工单状态为 "cancelled"
    And 时间线包含撤回记录

  Scenario: 已被处理人认领后撤回失败
    When "vpn-requester" 提交 VPN 申请，访问原因为 "network_support"
    And "network-operator" 认领当前工单
    And "vpn-requester" 撤回工单，原因为 "不需要了"
    Then 操作失败
    And 工单状态不为 "cancelled"

  Scenario: 非申请人撤回失败
    When "vpn-requester" 提交 VPN 申请，访问原因为 "network_support"
    And "network-operator" 撤回工单，原因为 "误提"
    Then 操作失败
    And 工单状态不为 "cancelled"

  Scenario: 撤回原因记录在时间线
    When "vpn-requester" 提交 VPN 申请，访问原因为 "network_support"
    And "vpn-requester" 撤回工单，原因为 "项目取消"
    Then 工单状态为 "cancelled"
    And 时间线包含 "项目取消"

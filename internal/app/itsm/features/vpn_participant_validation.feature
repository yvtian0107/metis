Feature: VPN 开通申请 — 参与者校验

  验证智能引擎在参与者缺失时使用兜底处理人安全兜底，参与者完整时正确路由。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 开通服务（智能引擎）

  Scenario: 配置兜底人后缺失参与者决策自动转派
    Given 引擎已配置兜底处理人为 "network-operator"
    And "vpn-requester" 已创建 VPN 工单（使用缺失参与者的工作流）
    When 引擎执行无参与者的处理决策
    Then 工单分配人为兜底处理人
    And 时间线包含参与者兜底事件


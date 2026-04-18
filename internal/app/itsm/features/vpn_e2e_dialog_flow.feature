@bdd @itsm @llm @e2e
Feature: VPN E2E 对话流程
  从服务台对话到创单到引擎触发的端到端验证

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名        | 部门     | 岗位       |
      | 申请人     | vpn-requester | it       | staff      |
      | 网络管理   | net-admin     | it       | net_admin  |
      | 安全管理   | sec-admin     | security | sec_admin  |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 服务（智能引擎）

  @network_support
  Scenario: 网络支持完整对话到创单
    Given 服务台收到用户 "vpn-requester" 的对话
    When 用户说 "我需要开通VPN，用于远程办公，访问开发环境。我的部门是IT部门。"
    Then 服务台识别出服务为 "vpn"
    And 表单数据包含访问原因
    When 服务台创建工单
    Then 工单状态为 "in_progress"
    And 智能引擎已触发决策循环

  @security_compliance
  Scenario: 安全合规完整对话到创单
    Given 服务台收到用户 "vpn-requester" 的对话
    When 用户说 "我需要VPN访问权限，原因是安全审计合规检查，需要访问生产环境安全日志。"
    Then 服务台识别出服务为 "vpn"
    When 服务台创建工单
    Then 工单状态为 "in_progress"
    And 智能引擎已触发决策循环

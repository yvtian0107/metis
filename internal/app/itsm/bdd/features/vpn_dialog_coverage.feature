@bdd @itsm @dialog_coverage
Feature: VPN 对话模式覆盖
  验证 6 种对话模式下的服务台行为

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名        | 部门     | 岗位       |
      | 申请人     | vpn-requester | it       | staff      |
      | 网络管理   | net-admin     | it       | net_admin  |
    And 已定义 VPN 开通申请协作规范
    And 已发布 VPN 对话测试服务

  Scenario Outline: 对话模式 - <模式名称>
    Given 服务台收到用户 "vpn-requester" 的对话
    When 用户按 "<模式名称>" 模式发起对话
    Then 服务台最终动作为 "<预期动作>"

    Examples:
      | 模式名称                     | 预期动作           |
      | complete_direct              | create_ticket      |
      | colloquial_complete          | create_ticket      |
      | multi_turn_fill_details      | create_ticket      |
      | full_info_hold               | hold_for_review    |
      | ambiguous_incomplete_hold    | request_more_info  |
      | multi_turn_hold              | hold_for_review    |

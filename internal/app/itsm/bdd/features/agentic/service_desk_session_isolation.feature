@bdd @itsm @session_isolation
Feature: 服务台会话隔离
  验证连续请求之间的状态隔离和会话重置

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名        | 部门 | 岗位       |
      | 申请人     | vpn-requester | it   | staff      |
      | 网络管理   | net-admin     | it   | net_admin  |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 服务（智能引擎）

  Scenario: 连续请求状态隔离
    Given 服务台收到用户 "vpn-requester" 的对话
    When 用户说 "我需要开通VPN，远程办公用。"
    And 服务台创建工单
    Then 工单状态为 "waiting_human"
    When 服务台发起新会话
    And 用户说 "我需要另一个VPN申请，用于访问测试环境。"
    And 服务台创建工单
    Then 新工单与前一张工单不同
    And 两张工单互不关联

  Scenario: new_request 重置会话状态
    Given 服务台收到用户 "vpn-requester" 的对话
    When 用户说 "我需要开通VPN"
    And 用户发送 "new_request" 重置指令
    Then 服务台会话状态已重置
    When 用户说 "重新开始，我要申请VPN用于安全审计"
    Then 服务台识别出新的请求意图

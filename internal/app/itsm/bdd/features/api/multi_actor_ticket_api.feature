@bdd @itsm @api
Feature: API 级多角色待办处理
  API BDD 通过统一 Actor、登录和 HTTP client 验证真实 ITSM API 合同。

  Background:
    Given API BDD 已初始化
    And API BDD 已准备默认 Actor
    And API BDD 存在一张分配给 "网络管理员" 的智能工单

  Scenario: 只有当前处理人能看到并认领待办
    When 以 "安全管理员" 身份查询待办列表
    Then API 响应状态应为 200
    And 待办列表不应包含当前工单
    When 以 "安全管理员" 身份认领当前待办
    Then API 响应状态应为 403
    When 以 "网络管理员" 身份查询待办列表
    Then API 响应状态应为 200
    And 待办列表应包含当前工单
    When 以 "网络管理员" 身份认领当前待办
    Then API 响应状态应为 200
    And 工单状态应为 "waiting_human"

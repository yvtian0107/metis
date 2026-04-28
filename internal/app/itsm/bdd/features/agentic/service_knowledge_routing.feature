@bdd @itsm @llm @knowledge_routing
Feature: 知识驱动路由
  验证知识库搜索结果影响服务路由决策

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名        | 部门     | 岗位       |
      | 申请人     | vpn-requester | it       | staff      |
      | 网络管理   | net-admin     | it       | net_admin  |
      | 安全管理   | sec-admin     | security | sec_admin  |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 服务（智能引擎）

  Scenario: 知识命中路由 - 有匹配知识时优先参考
    Given 服务定义关联了知识库
    And 知识库包含 VPN 配置指南
    When 智能引擎执行决策循环
    Then 决策工具调用包含 "decision.knowledge_search"

  Scenario: 知识未命中 - 走默认路由
    Given 服务定义未关联知识库
    When 智能引擎执行决策循环
    Then 决策正常完成
    And 知识搜索返回空结果

  Scenario: 知识库不可用 - 降级处理
    Given 知识搜索服务不可用
    When 智能引擎执行决策循环
    Then 决策正常完成且不依赖知识结果

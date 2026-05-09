@llm
Feature: 高风险变更协同申请（Boss）— 多类别主路径

  验证三种申请类别（访问授权、应急支持、多模块多明细生产变更）均能完成
  "总部首级岗位 -> 信息部运维岗位 -> completed"的两级串行完整流程。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名              | 部门         | 岗位            |
      | 申请人甲   | boss-requester-1    | -            | -               |
      | 申请人乙   | boss-requester-2    | -            | -               |
      | 首级处理人 | boss-head-reviewer  | headquarters | serial_reviewer |
      | 二级处理人 | ops-handler         | it           | ops_admin       |
    And 已定义高风险变更协同申请协作规范
    And 已基于协作规范发布高风险变更协同申请服务（智能引擎）

  # BS-002 ---------------------------------------------------------------
  Scenario: 访问授权主路径两级串签完成
    Given "boss-requester-1" 已创建高风险变更扩展工单，场景为 "access-grant-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前处理任务分配到岗位部门 "it/ops_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  # BS-003 ---------------------------------------------------------------
  Scenario: 应急支持主路径两级串签完成
    Given "boss-requester-1" 已创建高风险变更扩展工单，场景为 "emergency-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前处理任务分配到岗位部门 "it/ops_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 工单的表单数据中包含完整的 change_items 明细表格

  # BS-004 ---------------------------------------------------------------
  Scenario: 多模块多明细主路径草稿和工单完整保留
    Given "boss-requester-1" 已创建高风险变更扩展工单，场景为 "multi-module-1"
    Then 工单的表单数据中包含完整的 change_items 明细表格
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 工单的表单数据中包含完整的 change_items 明细表格
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前处理任务分配到岗位部门 "it/ops_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 工单的表单数据中包含完整的 change_items 明细表格

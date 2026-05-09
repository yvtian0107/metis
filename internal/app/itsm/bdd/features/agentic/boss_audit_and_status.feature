@llm
Feature: 高风险变更协同申请（Boss）— 状态流转与审计闭环

  验证工单状态流转的正确性、管理员取消、表单持久化和时间线完整性，
  确保"完成"不是页面假象，而是状态、活动、时间线、表单全部闭环。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名             | 部门         | 岗位            |
      | 申请人     | boss-requester-1   | -            | -               |
      | 首级处理人 | boss-head-reviewer | headquarters | serial_reviewer |
      | 二级处理人 | ops-handler        | it           | ops_admin       |
    And 已定义高风险变更协同申请协作规范
    And 已基于协作规范发布高风险变更协同申请服务（智能引擎）

  # BS-402 ---------------------------------------------------------------
  Scenario: 首关通过后工单仍处于进行中而非完成
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    When 当前活动的被分配人认领并处理完成
    Then 工单状态不为 "completed"

  # BS-405 ---------------------------------------------------------------
  Scenario: 次关驳回后工单不进入 completed
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前处理任务分配到岗位部门 "it/ops_admin"
    When 当前活动的被分配人认领并处理驳回
    And 智能引擎再次执行决策循环
    Then 工单状态不为 "completed"

  # BS-408 ---------------------------------------------------------------
  Scenario: 管理员取消进行中工单所有活动终止
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    When 管理员取消当前工单，原因为 "变更计划临时取消"
    Then 工单状态为 "cancelled"
    And 当前岗位部门 "headquarters/serial_reviewer" 的活跃处理任务数为 0

  # BS-501 ---------------------------------------------------------------
  Scenario: 工单表单数据完整保留所有基础字段
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    Then 工单表单数据包含字段 "subject,request_category,risk_level,change_window,impact_scope,rollback_required,impact_modules,change_items"

  # BS-503 ---------------------------------------------------------------
  Scenario: 全流程完成后时间线包含关键节点
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 时间线至少包含 4 个事件

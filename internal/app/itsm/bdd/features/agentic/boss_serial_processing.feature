Feature: 高风险变更协同申请（Boss）— 两级串行处理

  智能引擎编排"总部首级岗位处理→信息部运维岗位处理→完成"流程，验证两级岗位串行、复杂表单保留、处理隔离和并行工单隔离。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份                 | 用户名              | 部门 | 岗位       |
      | 申请人甲             | boss-requester-1    | -    | -          |
      | 申请人乙             | boss-requester-2    | -    | -          |
      | 首级处理人           | boss-head-reviewer  | headquarters | serial_reviewer |
      | 二级处理人           | ops-handler         | it           | ops_admin       |
    And 已定义高风险变更协同申请协作规范
    And 已基于协作规范发布高风险变更协同申请服务（智能引擎）

  Scenario: 完整串行处理——总部首级岗位处理→信息部运维岗位处理→完成
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前处理任务分配到岗位部门 "it/ops_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  Scenario: 处理隔离——二级处理人无法操作首级处理，首级处理人无法认领二级处理
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    And "ops-handler" 认领当前工单应失败
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前处理任务分配到岗位部门 "it/ops_admin"
    And "boss-head-reviewer" 认领当前工单应失败
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  Scenario: 复杂表单——change_items 明细表格跨工单完整保留
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    Then 工单的表单数据中包含完整的 change_items 明细表格
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 工单的表单数据中包含完整的 change_items 明细表格
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 工单的表单数据中包含完整的 change_items 明细表格

  Scenario: 申请人不能认领任意关的工单
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    And 申请人不能认领当前工单

  Scenario: 并行工单——两张串行处理工单的处理指派完全隔离
    Given "boss-requester-1" 已创建高风险变更工单 "A"，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    Given "boss-requester-2" 已创建高风险变更工单 "B"，场景为 "requester-2"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 工单 "A" 的处理记录与工单 "B" 完全隔离

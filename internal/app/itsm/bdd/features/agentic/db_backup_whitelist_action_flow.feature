Feature: 数据库备份白名单临时放行 — 智能引擎 Action 元调用
  智能引擎使用 decision.execute_action 工具在 ReAct 循环内同步执行预检和放行动作，
  验证 Action 元调用模型下的"预检→DBA处理→放行→完成"全流程。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份                 | 用户名              | 部门 | 岗位       |
      | 申请人甲             | db-requester-1      | -    | -          |
      | 申请人乙             | db-requester-2      | -    | -          |
      | 数据库管理员处理人   | dba-operator        | it   | db_admin   |
      | 运维管理员处理人     | ops-operator        | it   | ops_admin  |
    And 已定义数据库备份白名单临时放行协作规范
    And 已基于协作规范发布数据库备份白名单放行服务（智能引擎）

  Scenario: 完整流程——Agent 元调用预检、DBA处理、元调用放行、完成
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "in_progress"
    And 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作已为当前工单触发
    And 工单状态为 "completed"

  Scenario: 权限校验——运维管理员无法认领DBA处理且放行动作未提前触发
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    And 当前处理任务仅对 "dba-operator" 可见
    And "ops-operator" 认领当前工单应失败
    And 放行动作未为当前工单触发
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作已为当前工单触发
    And 工单状态为 "completed"

  Scenario: 并行工单——两张工单各自独立触发动作且记录隔离
    Given "db-requester-1" 已创建数据库备份白名单放行工单 "A"，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作已为当前工单触发
    And 工单状态为 "completed"
    Given "db-requester-2" 已创建数据库备份白名单放行工单 "B"，场景为 "requester-2"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作已为当前工单触发
    And 工单状态为 "completed"
    And 工单 "A" 的动作记录与工单 "B" 完全隔离

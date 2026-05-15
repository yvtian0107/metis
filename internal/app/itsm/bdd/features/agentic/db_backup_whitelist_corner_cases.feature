@deterministic @itsm @db_backup
Feature: 数据库备份白名单临时放行 — 生产语义边界
  数据库备份白名单临时放行是生产高风险动作，
  智能引擎必须在预检、DBA 处理、放行和终态之间保持严格边界。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份                 | 用户名              | 部门 | 岗位       |
      | 申请人甲             | db-requester-1      | -    | -          |
      | 申请人乙             | db-requester-2      | -    | -          |
      | 数据库管理员处理人   | dba-operator        | it   | db_admin   |
      | 运维管理员处理人     | ops-operator        | it   | ops_admin  |
    And 已定义数据库备份白名单临时放行协作规范
    And 已基于静态工作流发布数据库备份白名单放行服务（智能引擎）

  Scenario: 预检和放行动作必须携带完整生产风险上下文
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 预检动作请求包含完整风险上下文
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作已为当前工单触发
    And 放行动作请求包含完整放行上下文
    And 工单状态为 "completed"

  Scenario Outline: 缺失或模糊放行窗口时核心层不得触发任何动作
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "<case>"
    When 智能引擎执行决策循环
    Then 预检动作未为当前工单触发
    And 放行动作未为当前工单触发
    And 当前工单未完成且未履约
    And 决策诊断事件已记录

    Examples:
      | case             |
      | missing-window   |
      | ambiguous-window |

  Scenario: DBA 驳回后不得触发放行或退回申请人补充
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人驳回，意见为 "备份窗口未获生产变更授权"
    And 智能引擎再次执行决策循环
    Then 放行动作未为当前工单触发
    And 不得创建申请人补充表单
    And 工单处于驳回终态或已有决策诊断

  Scenario: 放行动作失败时不得完成，恢复后只成功放行一次
    Given 放行接收端临时失败
    And "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "apply-failure"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 放行动作执行失败
    And 当前工单未完成且未履约
    When 放行接收端恢复成功
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作成功记录数为 1
    And 放行动作失败记录数至少为 1
    And 工单状态为 "completed"

  Scenario: completed 后再次决策不得新增活动或重复触发动作
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作已为当前工单触发
    And 工单状态为 "completed"
    And 记录当前工单活动数与动作请求数
    When 智能引擎再次执行决策循环
    Then 当前工单活动数与动作请求数保持不变

  # DBW-204 ---------------------------------------------------------------
  Scenario: 预检失败时不得进入 DBA 处理也不得触发放行
    Given 预检接收端临时失败
    And "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "precheck-failure"
    When 智能引擎执行决策循环
    Then 预检动作执行失败
    And 放行动作未为当前工单触发
    And 当前工单未完成且未履约

  # DBW-509 ---------------------------------------------------------------
  Scenario: 放行动作失败记录中可追溯故障原因与响应内容
    Given 放行接收端临时失败
    And "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "apply-failure"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 放行动作执行失败
    And 放行动作失败记录包含完整故障信息
    And 当前工单未完成且未履约

  # TICK-00109 回归 ---------------------------------------------------------
  Scenario: 管理员指派接管 DBA 步骤后守卫仍正确识别完成并触发放行
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 管理员将当前活动指派给 "dba-operator"
    And 被指派人认领并处理完成当前活动
    And 智能引擎执行决策循环直到工单完成
    Then 放行动作已为当前工单触发
    And 工单状态为 "completed"

  # DBW-407 ---------------------------------------------------------------
  Scenario: 管理员取消进行中工单后所有活动终止
    Given "db-requester-1" 已创建数据库备份白名单放行工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 预检动作已为当前工单触发
    And 当前处理任务分配到岗位 "db_admin"
    When 管理员取消当前工单，原因为 "维护窗口临时取消"
    Then 工单状态为 "cancelled"
    And 当前工单所有活动均已取消

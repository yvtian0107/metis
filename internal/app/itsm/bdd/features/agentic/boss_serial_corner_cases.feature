@llm
Feature: 高风险变更协同申请（Boss）— Agentic 边界场景

  用真实 LLM + 真实 SmartEngine 工具链压测 Boss 高风险变更协同申请的边界语义。
  协作规范是事实源，workflow_json 是辅助背景；首级总部岗位、二级信息部运维岗位和完成顺序必须稳定。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份       | 用户名             | 部门         | 岗位            |
      | 申请人     | boss-requester-1   | -            | -               |
      | 首级处理人 | boss-head-reviewer | headquarters | serial_reviewer |
      | 二级处理人 | ops-handler        | it           | ops_admin       |
    And 已定义高风险变更协同申请协作规范
    And 已基于协作规范发布高风险变更协同申请服务（智能引擎）

  Scenario: 首级岗位解析优先
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    And 当前处理任务未分配到岗位部门 "it/ops_admin"
    And 参与人解析工具使用岗位部门 "headquarters/serial_reviewer"

  Scenario: 二级岗位串行门禁
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 当前岗位部门 "headquarters/serial_reviewer" 的活跃处理任务数为 1
    And 当前岗位部门 "it/ops_admin" 的活跃处理任务数为 0
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前岗位部门 "headquarters/serial_reviewer" 的活跃处理任务数为 0
    And 当前岗位部门 "it/ops_admin" 的活跃处理任务数为 1

  Scenario: workflow_json 错误首级岗位不得覆盖协作规范
    Given 高风险变更工作流参考图错误地把首级岗位改成 "it/ops_admin"
    And "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    And 当前处理任务未分配到岗位部门 "it/ops_admin"
    And AI 决策依据包含 "协作规范"

  Scenario: workflow_json 旧固定用户不得覆盖协作规范
    Given 高风险变更工作流参考图错误地把首级岗位改成旧固定用户 "serial-reviewer"
    And "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    And 当前处理任务未分配到岗位部门 "it/ops_admin"
    And AI 决策依据包含 "协作规范"

  Scenario: 首级参与人不可解析时不得 fallback 到二级
    Given 高风险变更岗位 "headquarters/serial_reviewer" 处理人已停用
    And "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 当前岗位部门 "it/ops_admin" 的活跃处理任务数为 0
    And 没有不可执行的高置信人工任务
    And 决策诊断事件已记录

  Scenario: 已有首级待处理任务时再次决策不得重复创建
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前岗位部门 "headquarters/serial_reviewer" 的活跃处理任务数为 1
    When 智能引擎再次执行决策循环
    Then 当前活跃人工任务数为 1
    And 当前岗位部门 "headquarters/serial_reviewer" 的活跃处理任务数为 1

  Scenario: completed 终态再次决策不得新增活动或改写结果
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 工单结果为 "fulfilled"
    And 工单活动数保持为 3
    When 智能引擎再次执行决策循环
    Then 工单状态为 "completed"
    And 工单结果为 "fulfilled"
    And 工单活动数保持为 3

  Scenario: rejected 不得默认退回申请人补充
    Given 高风险变更工作流参考图错误地把驳回指向申请人补充表单
    And "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    When 当前活动的被分配人驳回，意见为 "高风险变更信息不符合协作规范"
    And 智能引擎再次执行决策循环
    Then 不得创建申请人补充表单
    And 工单处于驳回终态或已有决策诊断

  Scenario: 次关已存在待处理任务时再次决策不得重复创建
    Given "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 当前处理任务分配到岗位部门 "it/ops_admin"
    And 当前岗位部门 "it/ops_admin" 的活跃处理任务数为 1
    When 智能引擎再次执行决策循环
    Then 当前活跃人工任务数为 1
    And 当前岗位部门 "it/ops_admin" 的活跃处理任务数为 1

  Scenario: 次关岗位无人时不得静默卡死
    Given 次关岗位 "it/ops_admin" 处理人已停用
    And "boss-requester-1" 已创建高风险变更工单，场景为 "requester-1"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎再次执行决策循环
    Then 工单状态不为 "failed"
    And 当前岗位部门 "it/ops_admin" 的活跃处理任务数为 0
    And 决策诊断事件已记录

  Scenario: 复杂表单异常枚举不驱动乱路由
    Given "boss-requester-1" 已创建高风险变更工单，表单数据为:
      """
      {"subject":"支付网关紧急变更","request_category":"prod_change","risk_level":"critical","expected_finish_time":"2026-05-01 12:00","change_window":["2026-04-30 22:00","2026-05-01 02:00"],"impact_scope":"支付核心链路","rollback_required":"required","impact_modules":["gateway"],"resource_items":[{"system_name":"legacy-payment","resource_account":"legacy-admin","permission_level":"read_write","target_operation":"旧字段命名混杂"}]}
      """
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 当前处理任务分配到岗位部门 "headquarters/serial_reviewer"
    And 当前岗位部门 "it/ops_admin" 的活跃处理任务数为 0

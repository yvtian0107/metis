Feature: 多角色并行处理后汇聚 — 智能引擎
  验证 Smart Engine 支持多角色并行处理处理：AI Agent 输出 execution_mode: "parallel" 后
  创建并行活动组，全部完成后汇聚触发下一轮决策循环。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份         | 用户名                | 部门 | 岗位            |
      | 申请人       | countersign-requester | -    | -               |
      | 并行处理处理人A  | countersign-netadmin  | it   | network_admin   |
      | 并行处理处理人B  | countersign-secadmin  | it   | security_admin  |
      | 最终处理人   | countersign-opsadmin  | it   | ops_admin       |
    And 已定义多角色并行处理协作规范
    And 已基于协作规范发布多角色并行处理服务（智能引擎）

  @bdd @itsm @countersign
  Scenario: 全部并行处理处理后汇聚推进到最终处理并完成
    Given "countersign-requester" 已创建并行处理工单，场景为 "standard"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 应存在一个并行处理活动组，包含 2 个并行活动
    When 并行处理组中岗位 "network_admin" 的处理人认领并处理完成
    Then 并行处理组仍有未完成活动，不应触发下一步
    When 并行处理组中岗位 "security_admin" 的处理人认领并处理完成
    Then 并行处理组全部完成，应触发下一轮决策
    When 智能引擎再次执行决策循环
    Then 当前活动类型为 "process"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  @bdd @itsm @countersign
  Scenario: 部分并行处理完成不得提前创建后续活动
    Given "countersign-requester" 已创建并行处理工单，场景为 "standard"
    When 智能引擎执行决策循环
    Then 应存在一个并行处理活动组，包含 2 个并行活动
    When 并行处理组中岗位 "network_admin" 的处理人认领并处理完成
    Then 并行处理组仍有未完成活动，不应触发下一步
    And 不应存在分配给岗位 "ops_admin" 的待处理活动

@deterministic @itsm @parallel_approval
Feature: 多角色并签申请 — 暂停恢复、驳回、并发收敛与权限隔离（确定性覆盖）

  通过静态工作流绕过 LLM 生成，用确定性方式验证以下补充行为：
  - 单签岗位无人时工单暂停（suspended）并可由 SmartRecovery 自动恢复
  - 并签阶段岗位无人时工单保持 waiting_human 且监控可见待处理
  - 并签任一人驳回导致工单拒绝，不进入终审
  - 终审驳回导致工单拒绝
  - 并发收敛幂等：两人同时通过只生成一个终审活动
  - 非并签角色无法处理并签阶段活动
  - 审批顺序不影响收敛结果

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份         | 用户名        | 部门 | 岗位           |
      | 申请人       | pa-requester  | -    | -              |
      | 并签审批人A  | pa-netadmin   | it   | network_admin  |
      | 并签审批人B  | pa-secadmin   | it   | security_admin |
      | 最终审批人   | pa-opsadmin   | it   | ops_admin      |
    And 已基于静态并签工作流发布多角色申请服务（智能引擎）

  # MR-306: 单签岗位无活跃成员 → suspended
  Scenario: BDD-NEW-1 单签岗位无人时并签收敛后工单进入暂停
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    And 岗位 "ops_admin" 当前没有活跃成员
    When 执行确定性并签审批决策
    And 并签审批组中岗位 "network_admin" 的审批人认领并审批通过
    And 并签审批组中岗位 "security_admin" 的审批人认领并审批通过
    And 执行确定性单签审批决策，岗位为 "ops_admin"
    Then 工单状态为 "suspended"
    And 时间线包含 "approver_missing_suspended" 类型事件

  # MR-308: 组织修复后 suspended 工单自动恢复
  Scenario: BDD-NEW-2 岗位成员修复后 suspended 工单自动恢复到 waiting_human
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    And 岗位 "ops_admin" 当前没有活跃成员
    When 执行确定性并签审批决策
    And 并签审批组中岗位 "network_admin" 的审批人认领并审批通过
    And 并签审批组中岗位 "security_admin" 的审批人认领并审批通过
    And 执行确定性单签审批决策，岗位为 "ops_admin"
    Then 工单状态为 "suspended"
    When 向岗位 "ops_admin" 添加活跃成员 "pa-opsadmin"
    And 执行 SmartRecovery 周期任务
    Then 工单状态为 "waiting_human"
    And 时间线包含 "participant_recovered" 类型事件

  # MR-307: 并签阶段岗位无人 → waiting_human（非 suspended，监控可感知）
  Scenario: BDD-NEW-3 并签阶段岗位无人时工单进入 waiting_human 待人工干预
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    And 岗位 "network_admin" 当前没有活跃成员
    When 执行确定性并签审批决策
    Then 工单状态为 "waiting_human"
    And 应存在一个并签审批活动组，包含 2 个并行活动
    And 并签审批组仍有未完成活动，不应触发下一步

  # MR-401: 并签阶段一人驳回 → rejected_decisioning，不进入终审
  # 收敛逻辑：全部并签完成后若任一为驳回则进入 rejected_decisioning；终审活动不应被创建。
  # 注：执行确定性 complete 决策会先将状态重置为 in_progress，破坏 rejected_decisioning 上下文，
  # 因此此处仅验证中间状态和 ops_admin 无活动，不再调用 complete 决策。
  Scenario: BDD-NEW-4 并签阶段任一审批人驳回则并签组收敛为拒绝状态
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    When 执行确定性并签审批决策
    And 并签审批组中岗位 "network_admin" 的审批人认领并审批驳回
    And 并签审批组中岗位 "security_admin" 的审批人认领并审批通过
    # 两人均已完成，并签组收敛；因有驳回 → rejected_decisioning
    Then 工单状态为 "rejected_decisioning"
    And 不应存在分配给岗位 "ops_admin" 的待处理审批活动

  # MR-402: 并签全通过后终审驳回 → rejected
  Scenario: BDD-NEW-5 并签全部通过后终审驳回工单应为 rejected
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    When 执行确定性并签审批决策
    And 并签审批组中岗位 "network_admin" 的审批人认领并审批通过
    And 并签审批组中岗位 "security_admin" 的审批人认领并审批通过
    And 执行确定性单签审批决策，岗位为 "ops_admin"
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并审批驳回
    And 执行确定性 complete 决策
    Then 工单状态为 "rejected"

  # MR-204: 并发收敛幂等 → 只生成一个终审活动
  Scenario: BDD-NEW-6 两人同时完成并签时只生成一个终审活动
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    When 执行确定性并签审批决策
    And 并签审批组两岗位审批人模拟并发通过
    Then 并签审批组全部完成，应触发下一轮决策
    When 执行确定性单签审批决策，岗位为 "ops_admin"
    Then 有且只有一个分配给岗位 "ops_admin" 的待处理审批活动

  # MR-302: 非并签角色在并签阶段无可执行活动
  Scenario: BDD-NEW-7 并签阶段非并签角色无待处理活动
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    When 执行确定性并签审批决策
    Then 应存在一个并签审批活动组，包含 2 个并行活动
    And 不应存在分配给岗位 "ops_admin" 的待处理审批活动
    When 并签审批组中岗位 "network_admin" 的审批人认领并审批通过
    Then 并签审批组仍有未完成活动，不应触发下一步
    And 不应存在分配给岗位 "ops_admin" 的待处理审批活动

  # MR-003: 先安全管理员后网络管理员通过，收敛结果相同
  Scenario: BDD-NEW-8 安全管理员先通过后网络管理员通过同样完成收敛
    Given "pa-requester" 已创建并签申请工单，场景为 "standard"
    When 执行确定性并签审批决策
    Then 应存在一个并签审批活动组，包含 2 个并行活动
    When 并签审批组中岗位 "security_admin" 的审批人认领并审批通过
    Then 并签审批组仍有未完成活动，不应触发下一步
    When 并签审批组中岗位 "network_admin" 的审批人认领并审批通过
    Then 并签审批组全部完成，应触发下一轮决策
    When 执行确定性单签审批决策，岗位为 "ops_admin"
    Then 工单状态为 "waiting_human"
    When 当前活动的被分配人认领并审批通过
    # 终审通过后进入 approved_decisioning；确定性执行 complete 决策完成工单
    And 执行确定性完成决策
    Then 工单状态为 "completed"

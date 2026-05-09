Feature: 生产服务器临时访问申请 — 智能引擎分支决策

  智能引擎根据访问目的将生产服务器临时访问申请路由到对应岗位处理。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份             | 用户名               | 部门 | 岗位           |
      | 申请人           | ops-access-requester | -    | -              |
      | 运维管理员处理人 | ops-operator         | it   | ops_admin      |
      | 网络管理员处理人 | network-operator     | it   | network_admin  |
      | 安全管理员处理人 | security-operator    | it   | security_admin |
    And 已定义生产服务器临时访问申请协作规范
    And 已基于协作规范发布生产服务器临时访问服务（智能引擎）

  Scenario: 生产故障排查访问路由到运维管理员并处理完成
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "ops"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "ops_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  Scenario: 网络链路诊断访问路由到网络管理员并处理完成
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "network"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "network_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  Scenario: 安全审计取证访问路由到安全管理员并处理完成
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "security"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "security_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  Scenario: 模糊描述下的边界语义判定路由到安全管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "boundary_security"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "security_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  @routing-synonyms
  Scenario: 运维同义词写法路由到运维管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "ops_synonym"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "ops_admin"

  @routing-synonyms
  Scenario: 运维词序变体路由到运维管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "ops_reordered"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "ops_admin"

  @routing-synonyms
  Scenario: 运维长句描述路由到运维管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "ops_long_sentence"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "ops_admin"

  @routing-synonyms
  Scenario: 网络组合描述路由到网络管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "network_combined"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "network_admin"

  @routing-synonyms
  Scenario: 安全组合描述路由到安全管理员
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "security_combined"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "security_admin"

  @routing-synonyms
  Scenario: 未分类事项走默认安全兜底
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "default_security"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "security_admin"

  Scenario: 运维分支处理的责任边界验证
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "ops"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前活动类型为 "process"
    And 当前处理任务分配到岗位 "ops_admin"
    And 当前处理任务仅对 "ops-operator" 可见
    And "network-operator" 认领当前工单应失败
    And "security-operator" 处理当前工单应失败
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"

  Scenario: 安全分支驳回后不得改派到其他业务分支
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "security"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "security_admin"
    When 当前活动的被分配人驳回，意见为 "安全条件不满足"
    And 智能引擎再次执行决策循环
    Then 工单状态为 "rejected"

  Scenario: 网络分支驳回后不得改派到其他业务分支
    Given "ops-access-requester" 已创建生产服务器访问工单，场景为 "network"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "network_admin"
    When 当前活动的被分配人驳回，意见为 "网络条件不满足"
    And 智能引擎再次执行决策循环
    Then 工单状态为 "rejected"

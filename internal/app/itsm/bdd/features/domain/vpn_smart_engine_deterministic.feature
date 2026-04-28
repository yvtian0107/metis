Feature: VPN 开通申请 — 智能引擎确定性覆盖

  使用 crafted DecisionPlan 确定性验证智能引擎执行层的全部分支，不依赖 LLM 决策。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已定义 VPN 开通申请协作规范
    And 已基于静态工作流发布 VPN 开通服务（智能引擎）

  Scenario: process 类型决策创建处理活动并指派参与者
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 执行确定性决策 type="process" 参与者为 "network-operator"
    Then 最新活动类型为 "process" 且状态为 "pending"
    And 最新活动存在指派记录
    And 工单分配人为 "network-operator"
    And 时间线包含 "ai_decision_executed" 类型事件

  Scenario: action 类型决策创建自动动作活动
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 执行无参与者的确定性决策 type="action"
    Then 最新活动类型为 "action" 且状态为 "in_progress"
    And 最新活动无指派记录
    And 时间线包含 "ai_decision_executed" 类型事件

  Scenario: notify 类型决策创建通知活动
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 执行无参与者的确定性决策 type="notify"
    Then 最新活动类型为 "notify" 且状态为 "in_progress"
    And 时间线包含 "ai_decision_executed" 类型事件

  Scenario: form 类型决策创建表单填写活动并指派参与者
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 执行确定性决策 type="form" 参与者为 "network-operator"
    Then 最新活动类型为 "form" 且状态为 "pending"
    And 最新活动存在指派记录
    And 工单分配人为 "network-operator"

  Scenario: escalate 类型决策创建升级活动
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 执行无参与者的确定性决策 type="escalate"
    Then 最新活动类型为 "escalate" 且状态为 "in_progress"
    And 时间线包含 "ai_decision_executed" 类型事件

  Scenario: complete 类型决策直接完结工单
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 执行确定性 complete 决策
    Then 工单状态为 "completed"
    And 最新活动类型为 "complete" 且状态为 "completed"
    And 时间线包含 "workflow_completed" 类型事件

  Scenario: AI 熔断后取消执行新决策
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 工单 AI 失败次数设为上限并尝试决策
    Then 操作失败
    And 时间线包含 "ai_disabled" 类型事件

  Scenario: Cancel 取消有活跃处理活动的智能工单
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 执行确定性决策 type="process" 参与者为 "network-operator"
    And 取消智能工单，原因为 "测试取消"
    Then 工单状态为 "cancelled"
    And 所有活动和指派均已取消
    And 时间线包含 "ticket_cancelled" 类型事件

  Scenario: 低置信度决策被管理员取消后不执行
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_support"
    When 创建确定性人工处置决策 type="process"
    Then 当前活动状态为 "pending"
    When 管理员取消当前人工处置决策
    Then 当前活动状态为 "cancelled"
    And 工单状态不为 "completed"

  Scenario: 兜底用户已停用时记录 warning 而非分配
    Given 引擎已配置兜底处理人为已停用用户
    And "vpn-requester" 已创建 VPN 工单（使用缺失参与者的工作流）
    When 引擎执行无参与者的处理决策
    Then 最新活动无指派记录
    And 时间线包含 "participant_fallback_warning" 类型事件

@llm
Feature: VPN 开通申请 — Agentic 引擎残酷核验

  用真实 LLM + 真实 SmartEngine 工具链拷问 VPN 开通申请。
  协作规范是事实源，workflow_json 只是辅助背景；Agent 必须基于工具事实做可执行决策。

  Background:
    Given 已完成系统初始化
    And 已准备好以下参与人、岗位与职责
      | 身份               | 用户名             | 部门 | 岗位           |
      | 申请人             | vpn-requester      | -    | -              |
      | 网络管理员处理人   | network-operator   | it   | network_admin  |
      | 安全管理员处理人   | security-operator  | it   | security_admin |
    And 已定义 VPN 开通申请协作规范
    And 已基于协作规范发布 VPN 开通服务（智能引擎）

  Scenario: 网络类诉求必须基于工具事实路由到网络管理员
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_access_issue"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "network_admin"
    And 当前处理任务未分配到岗位 "security_admin"
    And 决策工具 "decision.ticket_context" 已被调用
    And 决策工具 "decision.resolve_participant" 已被调用

  Scenario: 安全合规诉求不得误派网络管理员
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "security_compliance"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "security_admin"
    And 当前处理任务未分配到岗位 "network_admin"
    And 决策工具 "decision.ticket_context" 已被调用
    And 决策工具 "decision.resolve_participant" 已被调用

  Scenario: 网络与安全诉求冲突时不得拍脑袋选择单一路由
    Given "vpn-requester" 已创建 VPN 工单，访问原因同时包含网络和安全诉求
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 决策工具 "decision.ticket_context" 已被调用
    And 不得高置信选择单一路由
    And 进入澄清或低置信人工处置

  Scenario: 参与人不可解析时不得创建不可执行的高置信人工任务
    Given "vpn-requester" 已创建 VPN 工单（使用缺失参与者的工作流）
    When 智能引擎执行决策循环
    Then 工单状态不为 "failed"
    And 没有不可执行的高置信人工任务
    And 决策诊断事件已记录

  Scenario: 人工处理完成后必须结束流程而不是重复创建同一处理任务
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_access_issue"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "network_admin"
    When 当前活动的被分配人认领并处理完成
    And 智能引擎执行决策循环直到工单完成
    Then 工单状态为 "completed"
    And 不会重复创建刚完成的人工作业

  Scenario: 人工驳回后不得默认退回申请人补充
    Given "vpn-requester" 已创建 VPN 工单，访问原因为 "network_access_issue"
    When 智能引擎执行决策循环
    Then 工单状态为 "waiting_human"
    And 当前处理任务分配到岗位 "network_admin"
    When 当前活动的被分配人驳回，意见为 "访问理由不符合 VPN 开通规范"
    And 智能引擎再次执行决策循环
    Then 不得创建申请人补充表单
